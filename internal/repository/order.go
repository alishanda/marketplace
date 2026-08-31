package repository

import (
	"context"
	"errors"
	"time"

	"marketplace/internal/domain"

	"github.com/jackc/pgx/v5"
)

type OrderRepo struct {
	db *DB
}

func NewOrderRepo(db *DB) *OrderRepo {
	return &OrderRepo{db: db}
}

func (r *OrderRepo) Create(ctx context.Context, o domain.Order) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO orders (id, sku, amount, currency, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, o.ID, o.SKU, o.Amount, o.Currency, o.Status, o.CreatedAt, o.UpdatedAt)
	return err
}

func (r *OrderRepo) Get(ctx context.Context, id string) (domain.Order, error) {
	return scanOrder(r.db.Pool.QueryRow(ctx, orderSelect+" WHERE id = $1", id))
}

func (r *OrderRepo) GetForUpdate(ctx context.Context, q Execer, id string) (domain.Order, error) {
	return scanOrder(q.QueryRow(ctx, orderSelect+" WHERE id = $1 FOR UPDATE", id))
}

func (r *OrderRepo) Update(ctx context.Context, q Execer, o domain.Order) error {
	_, err := q.Exec(ctx, `
		UPDATE orders
		SET status = $2,
		    delivery_code = $3,
		    delivery_request_id = $4,
		    delivery_provider = $5,
		    updated_at = $6
		WHERE id = $1
	`, o.ID, o.Status, o.DeliveryCode, o.DeliveryRequestID, o.DeliveryProvider, time.Now().UTC())
	return err
}

func (r *OrderRepo) ListByStatus(ctx context.Context, statuses []string, updatedBefore time.Time) ([]domain.Order, error) {
	rows, err := r.db.Pool.Query(ctx, orderSelect+`
		WHERE status = ANY($1) AND updated_at <= $2
		ORDER BY updated_at
		LIMIT 100
	`, statuses, updatedBefore)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectOrders(rows)
}

func (r *OrderRepo) PaidNotDelivered(ctx context.Context) ([]domain.Order, error) {
	rows, err := r.db.Pool.Query(ctx, orderSelect+`
		WHERE status IN ('paid', 'delivering', 'out_of_stock', 'delivery_failed')
		ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectOrders(rows)
}

func (r *OrderRepo) DeliveredWithoutPaidEvent(ctx context.Context) ([]domain.Order, error) {
	rows, err := r.db.Pool.Query(ctx, orderSelect+`
		WHERE status = 'delivered'
		  AND NOT EXISTS (
			SELECT 1 FROM payment_events pe
			WHERE pe.order_id = orders.id AND pe.status = 'paid'
		  )
		ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectOrders(rows)
}

const orderSelect = `
	SELECT id, sku, amount, currency, status, delivery_code, delivery_request_id, delivery_provider, created_at, updated_at
	FROM orders
`

func scanOrder(row pgx.Row) (domain.Order, error) {
	var o domain.Order
	err := row.Scan(
		&o.ID, &o.SKU, &o.Amount, &o.Currency, &o.Status,
		&o.DeliveryCode, &o.DeliveryRequestID, &o.DeliveryProvider,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, domain.ErrNotFound
	}
	return o, err
}

func collectOrders(rows pgx.Rows) ([]domain.Order, error) {
	out := make([]domain.Order, 0)
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(
			&o.ID, &o.SKU, &o.Amount, &o.Currency, &o.Status,
			&o.DeliveryCode, &o.DeliveryRequestID, &o.DeliveryProvider,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
