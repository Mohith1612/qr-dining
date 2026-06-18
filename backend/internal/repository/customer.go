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

func (r *Repos) GetCustomerByID(ctx context.Context, customerID int64) (sqlc.Customer, error) {
	row := r.db.QueryRow(ctx, `SELECT id, restaurant_id, phone_e164, display_name, opted_in, opted_in_at, last_seen_at, visit_count, created_at FROM customers WHERE id = $1`, customerID)
	var c sqlc.Customer
	err := row.Scan(
		&c.ID,
		&c.RestaurantID,
		&c.PhoneE164,
		&c.DisplayName,
		&c.OptedIn,
		&c.OptedInAt,
		&c.LastSeenAt,
		&c.VisitCount,
		&c.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Customer{}, domain.ErrCustomerNotFound
	}
	return c, err
}

func (r *Repos) GetCustomerSessionHistory(ctx context.Context, customerID int64) ([]sqlc.GetCustomerSessionHistoryRow, error) {
	return r.q.GetCustomerSessionHistory(ctx, pgtype.Int8{Int64: customerID, Valid: true})
}

func (r *Repos) GetCustomerSessionHistoryScoped(ctx context.Context, customerID, restaurantID int64) ([]sqlc.GetCustomerSessionHistoryRow, error) {
	rows, err := r.db.Query(ctx, `
SELECT
    s.id AS session_id,
    s.created_at,
    s.closed_at,
    t.identifier AS table_identifier,
    COALESCE(SUM(o.total_amount), 0::numeric) AS total_spent
FROM sessions s
JOIN customers c ON c.id = s.customer_id
JOIN tables t ON t.id = s.table_id
LEFT JOIN orders o ON o.session_id = s.id AND o.status != 'cancelled'
WHERE s.customer_id = $1
  AND c.restaurant_id = $2
GROUP BY s.id, s.created_at, s.closed_at, t.identifier
ORDER BY s.created_at DESC
LIMIT 20
`, customerID, restaurantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []sqlc.GetCustomerSessionHistoryRow{}
	for rows.Next() {
		var i sqlc.GetCustomerSessionHistoryRow
		if err := rows.Scan(&i.SessionID, &i.CreatedAt, &i.ClosedAt, &i.TableIdentifier, &i.TotalSpent); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
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
