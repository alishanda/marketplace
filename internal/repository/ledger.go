package repository

import (
	"context"

	"marketplace/internal/domain"
)

type LedgerRepo struct {
	db *DB
}

func NewLedgerRepo(db *DB) *LedgerRepo {
	return &LedgerRepo{db: db}
}

func (r *LedgerRepo) RecordPair(ctx context.Context, q Execer, orderID string, eventID *string, entryType string, amount int, debitAccount, creditAccount string) error {
	_, err := q.Exec(ctx, `
		INSERT INTO ledger_entries (order_id, event_id, debit, credit, account, entry_type)
		VALUES ($1, $2, $3, 0, $4, $5)
		ON CONFLICT DO NOTHING
	`, orderID, eventID, amount, debitAccount, entryType)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `
		INSERT INTO ledger_entries (order_id, event_id, debit, credit, account, entry_type)
		VALUES ($1, $2, 0, $3, $4, $5)
		ON CONFLICT DO NOTHING
	`, orderID, eventID, amount, creditAccount, entryType)
	return err
}

func (r *LedgerRepo) Balance(ctx context.Context) (domain.LedgerBalance, error) {
	var b domain.LedgerBalance
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(debit), 0), COALESCE(SUM(credit), 0)
		FROM ledger_entries
	`).Scan(&b.Debit, &b.Credit)
	b.Balanced = b.Debit == b.Credit
	return b, err
}
