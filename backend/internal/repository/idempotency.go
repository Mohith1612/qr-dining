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
