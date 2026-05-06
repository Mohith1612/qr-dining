package repository

import (
	"context"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repos) ListStaleSessions(ctx context.Context, olderThan pgtype.Interval) ([]sqlc.ListStaleSessionsRow, error) {
	return r.q.ListStaleSessions(ctx, olderThan)
}
