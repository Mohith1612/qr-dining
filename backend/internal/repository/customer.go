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

func (r *Repos) UpsertCustomer(ctx context.Context, restaurantID int64, phone, displayName string) (sqlc.Customer, error) {
	return r.q.UpsertCustomer(ctx, sqlc.UpsertCustomerParams{
		RestaurantID: restaurantID,
		PhoneE164:    phone,
		DisplayName:  displayName,
	})
}

func (r *Repos) LinkSessionToCustomer(ctx context.Context, sessionID uuid.UUID, customerID int64) error {
	return r.q.LinkSessionToCustomer(ctx, sqlc.LinkSessionToCustomerParams{
		ID:         sessionID,
		CustomerID: pgtype.Int8{Int64: customerID, Valid: true},
	})
}

func (r *Repos) GetCustomerByPhone(ctx context.Context, restaurantID int64, phone string) (sqlc.Customer, error) {
	c, err := r.q.GetCustomerByPhone(ctx, sqlc.GetCustomerByPhoneParams{
		RestaurantID: restaurantID,
		PhoneE164:    phone,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Customer{}, domain.ErrCustomerNotFound
	}
	return c, err
}

func (r *Repos) GetCustomerSessionHistory(ctx context.Context, customerID int64) ([]sqlc.GetCustomerSessionHistoryRow, error) {
	return r.q.GetCustomerSessionHistory(ctx, pgtype.Int8{Int64: customerID, Valid: true})
}

func (r *Repos) DeleteCustomer(ctx context.Context, customerID, restaurantID int64) error {
	return r.q.DeleteCustomer(ctx, sqlc.DeleteCustomerParams{
		ID:           customerID,
		RestaurantID: restaurantID,
	})
}

func (r *Repos) SearchCustomersByPhone(ctx context.Context, restaurantID int64, prefix string) ([]sqlc.Customer, error) {
	return r.q.SearchCustomersByPhone(ctx, sqlc.SearchCustomersByPhoneParams{
		RestaurantID: restaurantID,
		Column2:      pgtype.Text{String: prefix, Valid: true},
	})
}
