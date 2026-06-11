package repository

import (
	"context"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateOrderParams struct {
	SessionID             uuid.UUID
	BranchID              int64
	PlacedByParticipantID int64
	IdempotencyKey        string
	TotalAmount           pgtype.Numeric
	OrderNumber           string
}

func (r *Repos) CreateOrder(ctx context.Context, p CreateOrderParams) (sqlc.Order, error) {
	return r.q.CreateOrder(ctx, sqlc.CreateOrderParams{
		SessionID:             p.SessionID,
		BranchID:              p.BranchID,
		PlacedByParticipantID: pgtype.Int8{Int64: p.PlacedByParticipantID, Valid: true},
		IdempotencyKey:        p.IdempotencyKey,
		TotalAmount:           p.TotalAmount,
		OrderNumber:           pgtype.Text{String: p.OrderNumber, Valid: p.OrderNumber != ""},
	})
}

func (r *Repos) NextOrderNumber(ctx context.Context, branchID int64, date string) (int32, error) {
	return r.q.NextOrderNumber(ctx, sqlc.NextOrderNumberParams{
		BranchID: branchID,
		Date:     date,
	})
}

func (r *Repos) CreateOrderItem(ctx context.Context, p sqlc.CreateOrderItemParams) (sqlc.OrderItem, error) {
	return r.q.CreateOrderItem(ctx, p)
}

func (r *Repos) GetOrderByID(ctx context.Context, id uuid.UUID) (sqlc.Order, error) {
	o, err := r.q.GetOrderByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Order{}, domain.ErrOrderNotFound
	}
	return o, err
}

func (r *Repos) GetOrderByIdempotencyKey(ctx context.Context, key string) (sqlc.Order, error) {
	o, err := r.q.GetOrderByIdempotencyKey(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Order{}, domain.ErrOrderNotFound
	}
	return o, err
}

func (r *Repos) ListOrdersForSession(ctx context.Context, sessionID uuid.UUID) ([]sqlc.Order, error) {
	return r.q.ListOrdersForSession(ctx, sessionID)
}

func (r *Repos) ListOrderItems(ctx context.Context, orderID uuid.UUID) ([]sqlc.OrderItem, error) {
	return r.q.ListOrderItems(ctx, orderID)
}

func (r *Repos) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status sqlc.OrderStatus) (sqlc.Order, error) {
	o, err := r.q.UpdateOrderStatus(ctx, sqlc.UpdateOrderStatusParams{
		ID:     id,
		Status: status,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Order{}, domain.ErrOrderNotFound
	}
	return o, err
}

func (r *Repos) ListActiveOrdersForBranch(ctx context.Context, branchID int64) ([]sqlc.ListActiveOrdersForBranchRow, error) {
	return r.q.ListActiveOrdersForBranch(ctx, branchID)
}
