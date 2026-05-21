package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Mohith1612/qr-dining/internal/repository"
	"github.com/Mohith1612/qr-dining/internal/worker"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// workerQuerier adapts *repository.Repos to satisfy the worker.Querier interface.
// The worker package uses a string interval (e.g. "2 hours"); we convert it to pgtype.Interval.
type workerQuerier struct {
	repos *repository.Repos
}

func (w *workerQuerier) ListStaleSessions(ctx context.Context, intervalStr string) ([]worker.StaleSession, error) {
	interval, err := parseIntervalString(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("parse interval %q: %w", intervalStr, err)
	}

	rows, err := w.repos.ListStaleSessions(ctx, interval)
	if err != nil {
		return nil, err
	}

	result := make([]worker.StaleSession, 0, len(rows))
	for _, r := range rows {
		result = append(result, worker.StaleSession{
			ID:      r.ID,
			TableID: r.TableID,
		})
	}
	return result, nil
}

func (w *workerQuerier) AbandonStaleSession(ctx context.Context, id uuid.UUID) error {
	return w.repos.AbandonStaleSession(ctx, id)
}

// parseIntervalString converts simple strings like "2 hours", "30 minutes" to pgtype.Interval.
// Supports "N hours" and "N minutes" formats only — sufficient for the stale session cleaner.
func parseIntervalString(s string) (pgtype.Interval, error) {
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return pgtype.Interval{}, fmt.Errorf("unsupported interval format: %q", s)
	}
	n, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return pgtype.Interval{}, err
	}
	var microseconds int64
	switch strings.ToLower(parts[1]) {
	case "hour", "hours":
		microseconds = n * 3_600_000_000
	case "minute", "minutes":
		microseconds = n * 60_000_000
	default:
		return pgtype.Interval{}, fmt.Errorf("unsupported unit: %s", parts[1])
	}
	return pgtype.Interval{Microseconds: microseconds, Valid: true}, nil
}
