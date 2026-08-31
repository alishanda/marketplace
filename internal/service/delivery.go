package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"marketplace/internal/domain"
	"marketplace/internal/provider"
	"marketplace/internal/repository"

	"github.com/jackc/pgx/v5"
)

type Issuer interface {
	Name() string
	Issue(ctx context.Context, req domain.IssueRequest) (domain.IssueResponse, error)
}

type Delivery struct {
	db       *repository.DB
	orders   *repository.OrderRepo
	ledger   *repository.LedgerRepo
	attempts *repository.DeliveryRepo
	primary  Issuer
	fallback Issuer
	retries  int
	backoff  time.Duration
}

func NewDelivery(db *repository.DB, orders *repository.OrderRepo, ledger *repository.LedgerRepo, attempts *repository.DeliveryRepo, primary, fallback Issuer, retries int, backoff time.Duration) *Delivery {
	return &Delivery{
		db:       db,
		orders:   orders,
		ledger:   ledger,
		attempts: attempts,
		primary:  primary,
		fallback: fallback,
		retries:  retries,
		backoff:  backoff,
	}
}

func NewDeliveryFromClients(db *repository.DB, orders *repository.OrderRepo, ledger *repository.LedgerRepo, attempts *repository.DeliveryRepo, primary, fallback *provider.Client, retries int, backoff time.Duration) *Delivery {
	return NewDelivery(db, orders, ledger, attempts, primary, fallback, retries, backoff)
}

func (s *Delivery) Deliver(ctx context.Context, orderID string) error {
	order, err := s.begin(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status == domain.StatusDelivered {
		return nil
	}

	code, requestID, providerName, err := s.issueWithFallback(ctx, order)
	if errors.Is(err, domain.ErrOutOfStock) {
		return s.finish(ctx, order.ID, domain.StatusOutOfStock, nil, nil, nil)
	}
	if err != nil {
		return s.finish(ctx, order.ID, domain.StatusDeliveryFailed, nil, nil, nil)
	}
	return s.complete(ctx, order, code, requestID, providerName)
}

func (s *Delivery) begin(ctx context.Context, orderID string) (domain.Order, error) {
	var order domain.Order
	err := s.db.InTx(ctx, func(tx pgx.Tx) error {
		var err error
		order, err = s.orders.GetForUpdate(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if order.Status == domain.StatusDelivered {
			return nil
		}
		if !domain.CanDeliver(order.Status) {
			return domain.ErrInvalid
		}
		if order.Status != domain.StatusDelivering {
			order.Status = domain.StatusDelivering
			return s.orders.Update(ctx, tx, order)
		}
		return nil
	})
	return order, err
}

func (s *Delivery) issueWithFallback(ctx context.Context, order domain.Order) (code, requestID, providerName string, err error) {
	code, requestID, err = s.issue(ctx, s.primary, order, domain.RequestID(order.ID, domain.ProviderA))
	if err == nil {
		return code, requestID, s.primary.Name(), nil
	}
	if errors.Is(err, domain.ErrProviderTimeout) {
		slog.Warn("provider_timeout_exhausted", "order_id", order.ID, "provider", s.primary.Name(), "request_id", requestID)
		return "", requestID, s.primary.Name(), err
	}
	if errors.Is(err, domain.ErrOutOfStock) {
		slog.Info("provider_out_of_stock", "order_id", order.ID, "provider", s.primary.Name())
	} else {
		slog.Warn("provider_error", "order_id", order.ID, "provider", s.primary.Name(), "err", err)
	}

	code, requestID, err = s.issue(ctx, s.fallback, order, domain.RequestID(order.ID, domain.ProviderB))
	if err == nil {
		return code, requestID, s.fallback.Name(), nil
	}
	return "", requestID, s.fallback.Name(), err
}

func (s *Delivery) issue(ctx context.Context, issuer Issuer, order domain.Order, requestID string) (string, string, error) {
	req := domain.IssueRequest{RequestID: requestID, SKU: order.SKU, OrderID: order.ID}
	var last error
	for attempt := 1; attempt <= s.retries; attempt++ {
		slog.Info("delivery_attempt", "order_id", order.ID, "provider", issuer.Name(), "request_id", requestID, "attempt", attempt)
		resp, err := issuer.Issue(ctx, req)
		status := "error"
		var codePtr, reasonPtr *string
		if err == nil {
			status = "ok"
			codePtr = &resp.Code
			_ = s.attempts.Log(ctx, requestID, order.ID, issuer.Name(), order.SKU, status, codePtr, nil)
			slog.Info("delivery_ok", "order_id", order.ID, "provider", issuer.Name(), "request_id", requestID)
			return resp.Code, requestID, nil
		}
		reason := err.Error()
		reasonPtr = &reason
		if errors.Is(err, domain.ErrProviderTimeout) {
			status = "timeout"
			slog.Warn("provider_timeout", "order_id", order.ID, "provider", issuer.Name(), "request_id", requestID, "attempt", attempt)
		} else if errors.Is(err, domain.ErrOutOfStock) {
			_ = s.attempts.Log(ctx, requestID, order.ID, issuer.Name(), order.SKU, "out_of_stock", nil, reasonPtr)
			return "", requestID, err
		}
		_ = s.attempts.Log(ctx, requestID, order.ID, issuer.Name(), order.SKU, status, nil, reasonPtr)
		last = err
		if attempt < s.retries {
			sleep := s.backoff * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return "", requestID, ctx.Err()
			case <-time.After(sleep):
			}
		}
	}
	return "", requestID, last
}

func (s *Delivery) complete(ctx context.Context, order domain.Order, code, requestID, providerName string) error {
	return s.db.InTx(ctx, func(tx pgx.Tx) error {
		current, err := s.orders.GetForUpdate(ctx, tx, order.ID)
		if err != nil {
			return err
		}
		if current.Status == domain.StatusDelivered {
			return nil
		}
		current.Status = domain.StatusDelivered
		current.DeliveryCode = &code
		current.DeliveryRequestID = &requestID
		current.DeliveryProvider = &providerName
		if err := s.orders.Update(ctx, tx, current); err != nil {
			return err
		}
		if err := s.ledger.RecordPair(ctx, tx, current.ID, nil, domain.LedgerSettlement, current.Amount, domain.AccountEscrow, domain.AccountRevenue); err != nil {
			return err
		}
		slog.Info("order_delivered", "order_id", current.ID, "request_id", requestID, "provider", providerName)
		return nil
	})
}

func (s *Delivery) finish(ctx context.Context, orderID, status string, code, requestID, providerName *string) error {
	return s.db.InTx(ctx, func(tx pgx.Tx) error {
		order, err := s.orders.GetForUpdate(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if order.Status == domain.StatusDelivered {
			return nil
		}
		order.Status = status
		order.DeliveryCode = code
		order.DeliveryRequestID = requestID
		order.DeliveryProvider = providerName
		slog.Info("order_status", "order_id", order.ID, "status", status)
		return s.orders.Update(ctx, tx, order)
	})
}
