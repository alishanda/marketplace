package service

import (
	"context"
	"time"

	"marketplace/internal/domain"
	"marketplace/internal/repository"
)

type Reconcile struct {
	orders *repository.OrderRepo
	ledger *repository.LedgerRepo
}

func NewReconcile(orders *repository.OrderRepo, ledger *repository.LedgerRepo) *Reconcile {
	return &Reconcile{orders: orders, ledger: ledger}
}

func (s *Reconcile) Report(ctx context.Context) (domain.ReconcileReport, error) {
	var report domain.ReconcileReport
	var err error
	report.PaidNotDelivered, err = s.orders.PaidNotDelivered(ctx)
	if err != nil {
		return report, err
	}
	report.DeliveredNotPaid, err = s.orders.DeliveredWithoutPaidEvent(ctx)
	if err != nil {
		return report, err
	}
	report.StuckDelivering, err = s.orders.ListByStatus(ctx, []string{domain.StatusDelivering}, time.Now().UTC())
	if err != nil {
		return report, err
	}
	report.Ledger, err = s.ledger.Balance(ctx)
	return report, err
}
