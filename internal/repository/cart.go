package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Mohith1612/qr-dining/internal/db/sqlc"
	"github.com/Mohith1612/qr-dining/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *Repos) GetOrCreateCart(ctx context.Context, sessionID uuid.UUID, participantID int64) (sqlc.Cart, error) {
	return r.q.GetOrCreateCart(ctx, sqlc.GetOrCreateCartParams{
		SessionID:     sessionID,
		ParticipantID: pgtype.Int8{Int64: participantID, Valid: true},
	})
}

func (r *Repos) GetCartByID(ctx context.Context, id int64) (sqlc.Cart, error) {
	c, err := r.q.GetCartByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Cart{}, domain.ErrInternalError
	}
	return c, err
}

func (r *Repos) AddCartItem(ctx context.Context, cartID, menuItemID int64, qty int16, modifiers json.RawMessage, note string) (sqlc.CartItem, error) {
	return r.q.AddCartItem(ctx, sqlc.AddCartItemParams{
		CartID:                cartID,
		MenuItemID:            menuItemID,
		Quantity:              qty,
		SelectedModifiersJson: modifiers,
		Note:                  note,
	})
}

func (r *Repos) GetCartItem(ctx context.Context, id int64) (sqlc.CartItem, error) {
	item, err := r.q.GetCartItem(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.CartItem{}, domain.ErrCartItemNotFound
	}
	return item, err
}

func (r *Repos) RemoveCartItem(ctx context.Context, cartID, itemID int64) error {
	return r.q.RemoveCartItem(ctx, sqlc.RemoveCartItemParams{
		CartID: cartID,
		ID:     itemID,
	})
}

func (r *Repos) ListCartItems(ctx context.Context, cartID int64) ([]sqlc.ListCartItemsRow, error) {
	return r.q.ListCartItems(ctx, cartID)
}

func (r *Repos) UpdateCartItemQuantity(ctx context.Context, id int64, qty int16) (sqlc.CartItem, error) {
	return r.q.UpdateCartItemQuantity(ctx, sqlc.UpdateCartItemQuantityParams{
		ID:       id,
		Quantity: qty,
	})
}

func (r *Repos) ClearCart(ctx context.Context, cartID int64) error {
	return r.q.ClearCart(ctx, cartID)
}
