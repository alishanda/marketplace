package repository

import (
	"context"
	"time"

	"marketplace/internal/domain"
)

type PaymentRepo struct {
	db *DB
}

func NewPaymentRepo(db *DB) *PaymentRepo {
	return &PaymentRepo{db: db}
}

func (r *PaymentRepo) InsertIfNew(ctx context.Context, ev domain.PaymentEvent) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx, `
		INSERT INTO payment_events (event_id, order_id, status, amount, currency, event_time)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (event_id) DO NOTHING
	`, ev.EventID, ev.OrderID, ev.Status, ev.Amount, ev.Currency, ev.CreatedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *PaymentRepo) PendingForOrder(ctx context.Context, orderID string) ([]domain.PaymentEvent, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT event_id, order_id, status, amount, currency, event_time, processed_at
		FROM payment_events
		WHERE order_id = $1 AND processed_at IS NULL
		ORDER BY event_time, created_at
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectEvents(rows)
}

func (r *PaymentRepo) AllPending(ctx context.Context) ([]domain.PaymentEvent, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT event_id, order_id, status, amount, currency, event_time, processed_at
		FROM payment_events
		WHERE processed_at IS NULL
		ORDER BY created_at
		LIMIT 200
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectEvents(rows)
}

func (r *PaymentRepo) MarkProcessed(ctx context.Context, q Execer, eventID string) error {
	_, err := q.Exec(ctx, `
		UPDATE payment_events SET processed_at = $2 WHERE event_id = $1 AND processed_at IS NULL
	`, eventID, time.Now().UTC())
	return err
}

func collectEvents(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]domain.PaymentEvent, error) {
	out := make([]domain.PaymentEvent, 0)
	for rows.Next() {
		var ev domain.PaymentEvent
		if err := rows.Scan(&ev.EventID, &ev.OrderID, &ev.Status, &ev.Amount, &ev.Currency, &ev.CreatedAt, &ev.ProcessedAt); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}
