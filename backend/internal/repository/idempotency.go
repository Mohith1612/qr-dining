package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type IdempotencyScope struct {
	ScopeType string
	ScopeID   string
	ActorType string
	ActorID   string
	Key       string
}

// CreateIdempotencyKey reserves a key. The bool reports whether this caller got
// the reservation: false means a live row already exists and the caller should
// take the replay path. A row whose expires_at has passed is reclaimed as a
// fresh reservation rather than replayed — see sql/queries/idempotency.sql.
func (r *Repos) CreateIdempotencyKey(ctx context.Context, scope IdempotencyScope, requestHash string, expiresAt time.Time) (sqlc.IdempotencyKey, bool, error) {
	row, err := r.q.CreateIdempotencyKey(ctx, sqlc.CreateIdempotencyKeyParams{
		ScopeType:   scope.ScopeType,
		ScopeID:     scope.ScopeID,
		ActorType:   scope.ActorType,
		ActorID:     scope.ActorID,
		Key:         scope.Key,
		RequestHash: requestHash,
		ExpiresAt:   expiresAt,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.IdempotencyKey{}, false, nil
	}
	return row, true, err
}

// GetIdempotencyKey returns the live reservation for a scope+key. An expired
// row is treated as absent and yields pgx.ErrNoRows.
func (r *Repos) GetIdempotencyKey(ctx context.Context, scope IdempotencyScope) (sqlc.IdempotencyKey, error) {
	return r.q.GetIdempotencyKey(ctx, sqlc.GetIdempotencyKeyParams{
		ScopeType: scope.ScopeType,
		ScopeID:   scope.ScopeID,
		ActorType: scope.ActorType,
		ActorID:   scope.ActorID,
		Key:       scope.Key,
	})
}

func (r *Repos) CompleteIdempotencyKey(ctx context.Context, scope IdempotencyScope, resourceType, resourceID string) error {
	return r.q.CompleteIdempotencyKey(ctx, sqlc.CompleteIdempotencyKeyParams{
		ScopeType:            scope.ScopeType,
		ScopeID:              scope.ScopeID,
		ActorType:            scope.ActorType,
		ActorID:              scope.ActorID,
		Key:                  scope.Key,
		ResponseResourceType: pgtype.Text{String: resourceType, Valid: true},
		ResponseResourceID:   pgtype.Text{String: resourceID, Valid: true},
	})
}

func (r *Repos) FailIdempotencyKey(ctx context.Context, scope IdempotencyScope) error {
	return r.q.FailIdempotencyKey(ctx, sqlc.FailIdempotencyKeyParams{
		ScopeType: scope.ScopeType,
		ScopeID:   scope.ScopeID,
		ActorType: scope.ActorType,
		ActorID:   scope.ActorID,
		Key:       scope.Key,
	})
}

// DeleteExpiredIdempotencyKeys removes up to limit keys whose expiry passed
// before expiredBefore, returning how many it deleted. Callers loop until a
// batch comes back short.
func (r *Repos) DeleteExpiredIdempotencyKeys(ctx context.Context, expiredBefore time.Time, limit int32) (int64, error) {
	return r.q.DeleteExpiredIdempotencyKeys(ctx, sqlc.DeleteExpiredIdempotencyKeysParams{
		ExpiresAt: expiredBefore,
		Limit:     limit,
	})
}
