package repository

import (
	"context"
	"errors"

	"marketplace/internal/domain"

	"github.com/jackc/pgx/v5"
)

type ProductRepo struct {
	db *DB
}

func NewProductRepo(db *DB) *ProductRepo {
	return &ProductRepo{db: db}
}

func (r *ProductRepo) Upsert(ctx context.Context, p domain.Product) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO products (sku, name, type, price, currency, image, stock)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (sku) DO UPDATE SET
			name = EXCLUDED.name,
			type = EXCLUDED.type,
			price = EXCLUDED.price,
			currency = EXCLUDED.currency,
			image = EXCLUDED.image
	`, p.SKU, p.Name, p.Type, p.Price, p.Currency, p.Image, p.Stock)
	return err
}

func (r *ProductRepo) Get(ctx context.Context, sku string) (domain.Product, error) {
	return scanProduct(r.db.Pool.QueryRow(ctx, `
		SELECT sku, name, type, price, currency, image, stock
		FROM products WHERE sku = $1
	`, sku))
}

func (r *ProductRepo) Storefront(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT sku, name, type, price, currency, image, stock
		FROM products
		WHERE stock > 0
		ORDER BY type, sku
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectProducts(rows)
}

func (r *ProductRepo) All(ctx context.Context) ([]domain.Product, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT sku, name, type, price, currency, image, stock
		FROM products
		ORDER BY type, sku
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectProducts(rows)
}

func (r *ProductRepo) Featured(ctx context.Context) ([]domain.Product, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT sku, name, type, price, currency, image, stock
		FROM products
		WHERE sku NOT LIKE 'GEN-%'
		ORDER BY type, sku
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectProducts(rows)
}

func (r *ProductRepo) RefreshStock(ctx context.Context, q Execer, sku string) error {
	_, err := q.Exec(ctx, `
		UPDATE products
		SET stock = (
			SELECT COUNT(*) FROM inventory_keys
			WHERE sku = $1 AND reserved_by_request_id IS NULL
		)
		WHERE sku = $1
	`, sku)
	return err
}

func (r *ProductRepo) SyncStock(ctx context.Context, sku string) error {
	return r.RefreshStock(ctx, r.db.Pool, sku)
}

func scanProduct(row pgx.Row) (domain.Product, error) {
	var p domain.Product
	err := row.Scan(&p.SKU, &p.Name, &p.Type, &p.Price, &p.Currency, &p.Image, &p.Stock)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, domain.ErrNotFound
	}
	return p, err
}

func collectProducts(rows pgx.Rows) ([]domain.Product, error) {
	out := make([]domain.Product, 0)
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.SKU, &p.Name, &p.Type, &p.Price, &p.Currency, &p.Image, &p.Stock); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
