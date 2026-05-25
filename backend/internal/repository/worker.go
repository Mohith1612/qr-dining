package repository

import (
	"context"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/google/uuid"
)

func (r *Repos) ListExpiredSessions(ctx context.Context) ([]sqlc.ListExpiredSessionsRow, error) {
	return r.q.ListExpiredSessions(ctx)
}

func (r *Repos) AbandonStaleSession(ctx context.Context, id uuid.UUID) error {
	return r.q.AbandonStaleSession(ctx, id)
}

func (r *Repos) ListSessionsExpiringSoon(ctx context.Context) ([]sqlc.ListSessionsExpiringSoonRow, error) {
	return r.q.ListSessionsExpiringSoon(ctx)
}

func (r *Repos) MarkSessionWarned(ctx context.Context, id uuid.UUID) error {
	return r.q.MarkSessionWarned(ctx, id)
}
