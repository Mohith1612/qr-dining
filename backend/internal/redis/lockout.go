package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// LockoutStore implements a sliding-window failure counter with a separate
// lockout TTL, backed by Redis. Used for brute-force protection on auth
// surfaces where a global rate limit is not enough — staff PIN attempts and
// platform admin passwords must lock the specific account after N failures
// independently of the per-IP throttle.
//
// Two Redis keys per identity:
//
//	lockout:counter:{scope}:{identity}  — INCR'd on each failure, TTL = window.
//	lockout:locked:{scope}:{identity}   — set when threshold is reached, TTL = lockoutDuration.
//
// The counter exists only to make the lockout decision; the locked key is the
// authoritative "currently locked out" signal. Once locked, the locked key's
// TTL is the source of truth for unlock time. The counter is wiped on
// successful login (via Reset) so a single mis-typed PIN early in the shift
// doesn't accumulate.
//
// ErrAuthLockedOut is returned both when threshold is just reached and when a
// subsequent attempt is made within the lockout window. Callers should not
// distinguish the two — they should always return the same 423 response so
// the attacker cannot probe for "just locked" vs "still locked".
var ErrAuthLockedOut = errors.New("auth attempts locked out")

type LockoutStore struct {
	client *goredis.Client
}

func NewLockoutStore(client *goredis.Client) *LockoutStore {
	return &LockoutStore{client: client}
}

type LockoutPolicy struct {
	Window         time.Duration // counter window
	MaxFailures    int           // threshold to trigger lockout
	LockoutTTL     time.Duration // how long the lockout persists
	FailOpenOnLoss bool          // if true, return allowed when Redis is unreachable
}

// CheckLocked returns nil when the identity is currently allowed to attempt,
// ErrAuthLockedOut when it is locked, and a Redis error otherwise. The policy
// controls fail-open vs fail-closed behavior on Redis loss.
func (s *LockoutStore) CheckLocked(ctx context.Context, scope, identity string, p LockoutPolicy) error {
	key := lockedKey(scope, identity)
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		if p.FailOpenOnLoss {
			return nil
		}
		return err
	}
	if exists > 0 {
		return ErrAuthLockedOut
	}
	return nil
}

// RecordFailure increments the failure counter. If the counter crosses
// MaxFailures the lockout key is set. Returns whether the identity is now
// locked.
func (s *LockoutStore) RecordFailure(ctx context.Context, scope, identity string, p LockoutPolicy) (bool, error) {
	counter := counterKey(scope, identity)
	pipe := s.client.Pipeline()
	incrCmd := pipe.Incr(ctx, counter)
	pipe.Expire(ctx, counter, p.Window)
	if _, err := pipe.Exec(ctx); err != nil {
		if p.FailOpenOnLoss {
			return false, nil
		}
		return false, err
	}
	if int(incrCmd.Val()) < p.MaxFailures {
		return false, nil
	}
	// Set the locked key with the lockout TTL. We do not wipe the counter —
	// it expires on its own and the locked key is the gate from now on.
	if err := s.client.Set(ctx, lockedKey(scope, identity), "1", p.LockoutTTL).Err(); err != nil {
		if p.FailOpenOnLoss {
			return true, nil
		}
		return true, err
	}
	return true, nil
}

// Reset clears the failure counter (but not an existing lockout). Called on
// every successful authentication so an honest user who mistypes once does
// not slowly accumulate strikes across a long shift.
func (s *LockoutStore) Reset(ctx context.Context, scope, identity string) {
	_ = s.client.Del(ctx, counterKey(scope, identity)).Err()
}

// Unlock clears the lockout immediately. Used by operator override and by
// tests; never expose to callers.
func (s *LockoutStore) Unlock(ctx context.Context, scope, identity string) error {
	pipe := s.client.Pipeline()
	pipe.Del(ctx, counterKey(scope, identity))
	pipe.Del(ctx, lockedKey(scope, identity))
	_, err := pipe.Exec(ctx)
	return err
}

// RemainingLockoutSeconds returns the seconds until the lock expires (zero if
// not locked). Useful for the Retry-After header so the caller can time their
// next attempt politely. A negative TTL indicates an unexpected state and is
// reported as zero.
func (s *LockoutStore) RemainingLockoutSeconds(ctx context.Context, scope, identity string) int {
	ttl, err := s.client.TTL(ctx, lockedKey(scope, identity)).Result()
	if err != nil || ttl <= 0 {
		return 0
	}
	return int(ttl.Seconds())
}

func counterKey(scope, identity string) string {
	return fmt.Sprintf("lockout:counter:%s:%s", scope, identity)
}

func lockedKey(scope, identity string) string {
	return fmt.Sprintf("lockout:locked:%s:%s", scope, identity)
}
