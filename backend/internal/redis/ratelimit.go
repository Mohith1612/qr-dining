package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// RateLimiter implements a fixed-window per-IP rate limiter backed by Redis.
// Key pattern: ratelimit:{ip}:{floor(unix_time/60)}
// The minute-aligned window auto-expires without a background cleaner.
type RateLimiter struct {
	client *goredis.Client
}

func NewRateLimiter(client *goredis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

// Allow checks whether the given IP is within its rate limit for the current minute window.
// Returns (allowed, remaining, error).
func (rl *RateLimiter) Allow(ctx context.Context, ip string, limitPerMinute int) (bool, int, error) {
	return rl.AllowWithPrefix(ctx, "", ip, limitPerMinute)
}

// AllowWithPrefix is like Allow but scopes the key to a named endpoint prefix,
// enabling tighter per-endpoint limits independent of the global limit.
func (rl *RateLimiter) AllowWithPrefix(ctx context.Context, prefix, ip string, limitPerMinute int) (bool, int, error) {
	window := time.Now().Unix() / 60
	var key string
	if prefix != "" {
		key = fmt.Sprintf("ratelimit:%s:%s:%d", prefix, ip, window)
	} else {
		key = fmt.Sprintf("ratelimit:%s:%d", ip, window)
	}

	pipe := rl.client.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, 90*time.Second) // slightly longer than a minute to avoid edge-case expiry
	if _, err := pipe.Exec(ctx); err != nil {
		// On Redis error, fail open — don't block legitimate requests.
		return true, limitPerMinute, nil
	}

	count := int(incrCmd.Val())
	if count > limitPerMinute {
		return false, 0, nil
	}
	return true, limitPerMinute - count, nil
}
