package repository

import (
	"context"
)

type DeliveryRepo struct {
	db *DB
}

func NewDeliveryRepo(db *DB) *DeliveryRepo {
	return &DeliveryRepo{db: db}
}

func (r *DeliveryRepo) Log(ctx context.Context, requestID, orderID, provider, sku, status string, code, reason *string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO delivery_attempts (request_id, order_id, provider, sku, status, code, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, requestID, orderID, provider, sku, status, code, reason)
	return err
}
