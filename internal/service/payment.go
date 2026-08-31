package service

import (
	"context"
	"log/slog"
	"time"

	"marketplace/internal/domain"
	"marketplace/internal/queue"
	"marketplace/internal/repository"

	"github.com/jackc/pgx/v5"
)

type Payment struct {
	db     *repository.DB
	orders *repository.OrderRepo
	events *repository.PaymentRepo
	ledger *repository.LedgerRepo
	jobs   *queue.Delivery
}

func NewPayment(db *repository.DB, orders *repository.OrderRepo, events *repository.PaymentRepo, ledger *repository.LedgerRepo, jobs *queue.Delivery) *Payment {
	return &Payment{db: db, orders: orders, events: events, ledger: ledger, jobs: jobs}
}

func (s *Payment) HandleWebhook(ctx context.Context, ev domain.PaymentEvent) error {
	if ev.EventID == "" || ev.OrderID == "" {
		return domain.ErrInvalid
	}
	if ev.Status != domain.PaymentPaid && ev.Status != domain.PaymentFailed {
		return domain.ErrInvalid
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}

	inserted, err := s.events.InsertIfNew(ctx, ev)
	if err != nil {
		return err
	}
	if !inserted {
		slog.Info("payment_duplicate", "event_id", ev.EventID, "order_id", ev.OrderID)
		return nil
	}
	slog.Info("payment_received", "event_id", ev.EventID, "order_id", ev.OrderID, "status", ev.Status, "amount", ev.Amount)
	return s.apply(ctx, ev)
}

func (s *Payment) ApplyPending(ctx context.Context, orderID string) error {
	pending, err := s.events.PendingForOrder(ctx, orderID)
	if err != nil {
		return err
	}
	for _, ev := range pending {
		if err := s.apply(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

func (s *Payment) ApplyAllPending(ctx context.Context) error {
	pending, err := s.events.AllPending(ctx)
	if err != nil {
		return err
	}
	for _, ev := range pending {
		if err := s.apply(ctx, ev); err != nil {
			slog.Error("apply_pending_event", "event_id", ev.EventID, "err", err)
		}
	}
	return nil
}

func (s *Payment) apply(ctx context.Context, ev domain.PaymentEvent) error {
	var startDelivery bool
	err := s.db.InTx(ctx, func(tx pgx.Tx) error {
		order, err := s.orders.GetForUpdate(ctx, tx, ev.OrderID)
		if err != nil {
			return err
		}
		next, changed := transitionPayment(order.Status, ev.Status)
		if changed {
			order.Status = next
			if err := s.orders.Update(ctx, tx, order); err != nil {
				return err
			}
			if next == domain.StatusPaid {
				eventID := ev.EventID
				if err := s.ledger.RecordPair(ctx, tx, order.ID, &eventID, domain.LedgerPayment, order.Amount, domain.AccountCustomer, domain.AccountEscrow); err != nil {
					return err
				}
				startDelivery = true
			}
			slog.Info("order_status", "order_id", order.ID, "status", next, "event_id", ev.EventID)
		}
		return s.events.MarkProcessed(ctx, tx, ev.EventID)
	})
	if err == domain.ErrNotFound {
		slog.Info("payment_orphan", "event_id", ev.EventID, "order_id", ev.OrderID)
		return nil
	}
	if err != nil {
		return err
	}
	if startDelivery {
		s.jobs.Enqueue(ev.OrderID)
	}
	return nil
}

func transitionPayment(current, paymentStatus string) (string, bool) {
	if paymentStatus == domain.PaymentPaid && domain.CanAcceptPaid(current) {
		return domain.StatusPaid, true
	}
	if paymentStatus == domain.PaymentFailed && domain.CanAcceptFailed(current) {
		return domain.StatusPaymentFailed, true
	}
	return current, false
}
