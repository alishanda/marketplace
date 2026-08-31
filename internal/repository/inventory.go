package repository

import (
	"context"
	"errors"

	"marketplace/internal/domain"

	"github.com/jackc/pgx/v5"
)

type InventoryRepo struct {
	db *DB
}

func NewInventoryRepo(db *DB) *InventoryRepo {
	return &InventoryRepo{db: db}
}

func (r *InventoryRepo) InsertKey(ctx context.Context, sku, code string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO inventory_keys (code, sku)
		VALUES ($1, $2)
		ON CONFLICT (code) DO NOTHING
	`, code, sku)
	return err
}

func (r *InventoryRepo) CodeByRequest(ctx context.Context, q Execer, requestID string) (string, error) {
	var code string
	err := q.QueryRow(ctx, `
		SELECT code FROM inventory_keys WHERE reserved_by_request_id = $1
	`, requestID).Scan(&code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return code, err
}

func (r *InventoryRepo) Reserve(ctx context.Context, q Execer, sku, orderID, requestID, provider string) (string, error) {
	code, err := r.CodeByRequest(ctx, q, requestID)
	if err == nil {
		return code, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return "", err
	}
	var reserved string
	err = q.QueryRow(ctx, `
		UPDATE inventory_keys
		SET reserved_by_request_id = $1,
		    issued_to_order_id = $2,
		    provider = $3
		WHERE id = (
			SELECT id FROM inventory_keys
			WHERE sku = $4 AND reserved_by_request_id IS NULL
			ORDER BY id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING code
	`, requestID, orderID, provider, sku).Scan(&reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrOutOfStock
	}
	return reserved, err
}

func (r *InventoryRepo) Available(ctx context.Context, sku string) (int, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM inventory_keys
		WHERE sku = $1 AND reserved_by_request_id IS NULL
	`, sku).Scan(&n)
	return n, err
}
