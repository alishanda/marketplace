package service

import (
	"context"
	"log/slog"
	"time"

	"marketplace/internal/domain"
	"marketplace/internal/repository"
)

type Recovery struct {
	orders     *repository.OrderRepo
	payments   *Payment
	delivery   *Delivery
	stuckAfter time.Duration
}

func NewRecovery(orders *repository.OrderRepo, payments *Payment, delivery *Delivery, stuckAfter time.Duration) *Recovery {
	return &Recovery{orders: orders, payments: payments, delivery: delivery, stuckAfter: stuckAfter}
}

func (s *Recovery) Tick(ctx context.Context) {
	if err := s.payments.ApplyAllPending(ctx); err != nil {
		slog.Error("recovery_payments", "err", err)
	}
	cutoff := time.Now().UTC().Add(-s.stuckAfter)
	orders, err := s.orders.ListByStatus(ctx, []string{
		domain.StatusPaid,
		domain.StatusDelivering,
		domain.StatusOutOfStock,
		domain.StatusDeliveryFailed,
	}, cutoff)
	if err != nil {
		slog.Error("recovery_list", "err", err)
		return
	}
	for _, order := range orders {
		slog.Info("recovery_retry", "order_id", order.ID, "status", order.Status)
		if err := s.delivery.Deliver(ctx, order.ID); err != nil && err != domain.ErrInvalid {
			slog.Error("recovery_deliver", "order_id", order.ID, "err", err)
		}
	}
}
