package repository

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SessionMaintenanceResult struct {
	SessionID      uuid.UUID
	OrganizationID int64
	BranchID       int64
	TableID        int64
}

type SessionTableReconciliation struct {
	Action         string
	SessionID      uuid.UUID
	OrganizationID int64
	BranchID       int64
	TableID        int64
}

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

func (r *Repos) AbandonStaleSession(ctx context.Context, id uuid.UUID, tableID int64) (SessionMaintenanceResult, bool, error) {
	var result SessionMaintenanceResult
	abandoned := false
	err := r.WithTx(ctx, func(tx *Repos) error {
		row := tx.db.QueryRow(ctx, `
SELECT s.id, b.organization_id, s.branch_id, s.table_id, s.status
FROM sessions s
JOIN branches b ON b.id = s.branch_id
WHERE s.id = $1
FOR UPDATE OF s
`, id)
		var status sqlc.SessionStatus
		if err := row.Scan(&result.SessionID, &result.OrganizationID, &result.BranchID, &result.TableID, &status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if result.TableID != tableID || status != sqlc.SessionStatusActive {
			return nil
		}
		if err := tx.ExecRaw(ctx, `SELECT id FROM tables WHERE id = $1 FOR UPDATE`, tableID); err != nil {
			return err
		}
		if err := tx.q.AbandonStaleSession(ctx, id); err != nil {
			return err
		}
		if err := tx.UpdateTableStatus(ctx, tableID, sqlc.TableStatusAvailable); err != nil {
			return err
		}
		// Revoke guest credentials in the same transaction so a stored token can
		// never be used to act on a terminal session. Bumping credential_version
		// also invalidates outstanding tokens whose signature is otherwise valid.
		if err := tx.RevokeAllParticipants(ctx, id, "session_abandoned"); err != nil {
			return err
		}
		if err := tx.BumpAllParticipantCredentialVersions(ctx, id); err != nil {
			return err
		}
		abandoned = true
		return nil
	})
	return result, abandoned, err
}

func (r *Repos) ListSessionsExpiringSoon(ctx context.Context) ([]sqlc.ListSessionsExpiringSoonRow, error) {
	return r.q.ListSessionsExpiringSoon(ctx)
}

func (r *Repos) MarkSessionWarned(ctx context.Context, id uuid.UUID) error {
	return r.q.MarkSessionWarned(ctx, id)
}

func (r *Repos) ReconcileSessionTables(ctx context.Context) ([]SessionTableReconciliation, error) {
	actions := []SessionTableReconciliation{}
	err := r.WithTx(ctx, func(tx *Repos) error {
		rows, err := tx.db.Query(ctx, `
WITH affected AS (
    UPDATE tables t
    SET status = 'available'
    FROM branches b
    WHERE b.id = t.branch_id
      AND t.status = 'occupied'
      AND NOT EXISTS (
          SELECT 1 FROM sessions s
          WHERE s.table_id = t.id AND s.status = 'active'
      )
    RETURNING b.organization_id, t.branch_id, t.id
)
SELECT 'table_released'::text, '00000000-0000-0000-0000-000000000000'::uuid, organization_id, branch_id, id FROM affected
`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item SessionTableReconciliation
			if err := rows.Scan(&item.Action, &item.SessionID, &item.OrganizationID, &item.BranchID, &item.TableID); err != nil {
				return err
			}
			actions = append(actions, item)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		rows, err = tx.db.Query(ctx, `
WITH affected AS (
    UPDATE tables t
    SET status = 'occupied'
    FROM sessions s
    JOIN branches b ON b.id = s.branch_id
    WHERE s.table_id = t.id
      AND s.status = 'active'
      AND t.status = 'available'
    RETURNING s.id AS session_id, b.organization_id, t.branch_id, t.id AS table_id
)
SELECT 'table_occupied'::text, session_id, organization_id, branch_id, table_id FROM affected
`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item SessionTableReconciliation
			if err := rows.Scan(&item.Action, &item.SessionID, &item.OrganizationID, &item.BranchID, &item.TableID); err != nil {
				return err
			}
			actions = append(actions, item)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		rows, err = tx.db.Query(ctx, `
WITH ranked AS (
    SELECT s.id, s.branch_id, s.table_id, b.organization_id,
           ROW_NUMBER() OVER (
               PARTITION BY s.table_id
               ORDER BY (s.host_participant_id IS NOT NULL) DESC, s.created_at DESC, s.id DESC
           ) AS rn
    FROM sessions s
    JOIN branches b ON b.id = s.branch_id
    WHERE s.status = 'active'
),
affected AS (
    UPDATE sessions s
    SET status = 'abandoned', closed_at = NOW()
    FROM ranked r
    WHERE s.id = r.id AND r.rn > 1
    RETURNING s.id AS session_id, r.organization_id, s.branch_id, s.table_id
)
SELECT 'duplicate_abandoned'::text, session_id, organization_id, branch_id, table_id FROM affected
`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var item SessionTableReconciliation
			if err := rows.Scan(&item.Action, &item.SessionID, &item.OrganizationID, &item.BranchID, &item.TableID); err != nil {
				return err
			}
			actions = append(actions, item)
		}
		return rows.Err()
	})
	return actions, err
}
