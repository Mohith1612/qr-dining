package repository

import (
	"context"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repos) ListExpiredSessions(ctx context.Context) ([]sqlc.ListExpiredSessionsRow, error) {
	return r.q.ListExpiredSessions(ctx)
}

func (r *Repos) ListStaleSessions(ctx context.Context, interval pgtype.Interval) ([]sqlc.ListExpiredSessionsRow, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, branch_id, table_id
FROM sessions
WHERE status = 'active'
  AND created_at < NOW() - $1::interval
ORDER BY created_at ASC
`, interval)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []sqlc.ListExpiredSessionsRow{}
	for rows.Next() {
		var i sqlc.ListExpiredSessionsRow
		if err := rows.Scan(&i.ID, &i.BranchID, &i.TableID); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (r *Repos) AbandonStaleSession(ctx context.Context, id uuid.UUID, tableID int64) error {
	return r.WithTx(ctx, func(tx *Repos) error {
		if err := tx.q.AbandonStaleSession(ctx, id); err != nil {
			return err
		}
		return tx.UpdateTableStatus(ctx, tableID, sqlc.TableStatusAvailable)
	})
}

func (r *Repos) ListSessionsExpiringSoon(ctx context.Context) ([]sqlc.ListSessionsExpiringSoonRow, error) {
	return r.q.ListSessionsExpiringSoon(ctx)
}

func (r *Repos) MarkSessionWarned(ctx context.Context, id uuid.UUID) error {
	return r.q.MarkSessionWarned(ctx, id)
}
