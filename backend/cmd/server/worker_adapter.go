package main

import (
	"context"

	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/worker"
	"github.com/google/uuid"
)

// workerQuerier adapts *repository.Repos to satisfy the worker.Querier interface.
type workerQuerier struct {
	repos *repository.Repos
}

func (w *workerQuerier) ListExpiredSessions(ctx context.Context) ([]worker.ExpiredSession, error) {
	rows, err := w.repos.ListExpiredSessions(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]worker.ExpiredSession, 0, len(rows))
	for _, r := range rows {
		result = append(result, worker.ExpiredSession{
			ID:      r.ID,
			TableID: r.TableID,
		})
	}
	return result, nil
}

func (w *workerQuerier) AbandonStaleSession(ctx context.Context, id uuid.UUID, tableID int64) error {
	return w.repos.AbandonStaleSession(ctx, id, tableID)
}

func (w *workerQuerier) ListSessionsExpiringSoon(ctx context.Context) ([]worker.ExpiringSoonSession, error) {
	rows, err := w.repos.ListSessionsExpiringSoon(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]worker.ExpiringSoonSession, 0, len(rows))
	for _, r := range rows {
		result = append(result, worker.ExpiringSoonSession{
			ID:                    r.ID,
			CreatedAt:             r.CreatedAt,
			SessionTimeoutMinutes: r.SessionTimeoutMinutes,
		})
	}
	return result, nil
}

func (w *workerQuerier) MarkSessionWarned(ctx context.Context, id uuid.UUID) error {
	return w.repos.MarkSessionWarned(ctx, id)
}
