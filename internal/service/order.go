package service

import (
	"context"
	"log/slog"
	"time"

	"marketplace/internal/domain"
	"marketplace/internal/repository"
)

type Orders struct {
	products *repository.ProductRepo
	orders   *repository.OrderRepo
	payments *Payment
}

func NewOrders(products *repository.ProductRepo, orders *repository.OrderRepo, payments *Payment) *Orders {
	return &Orders{products: products, orders: orders, payments: payments}
}

type CreateOrderInput struct {
	ID  string
	SKU string
}

func (s *Orders) Create(ctx context.Context, in CreateOrderInput) (domain.Order, error) {
	if in.SKU == "" {
		return domain.Order{}, domain.ErrInvalid
	}
	product, err := s.products.Get(ctx, in.SKU)
	if err != nil {
		return domain.Order{}, err
	}
	now := time.Now().UTC()
	id := in.ID
	if id == "" {
		id = domain.NewID("ord_")
	}
	if existing, err := s.orders.Get(ctx, id); err == nil {
		return existing, nil
	}
	order := domain.Order{
		ID:        id,
		SKU:       product.SKU,
		Amount:    product.Price,
		Currency:  product.Currency,
		Status:    domain.StatusCreated,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.orders.Create(ctx, order); err != nil {
		if current, getErr := s.orders.Get(ctx, id); getErr == nil {
			return current, nil
		}
		return domain.Order{}, err
	}
	slog.Info("order_created", "order_id", order.ID, "sku", order.SKU, "amount", order.Amount)
	if err := s.payments.ApplyPending(ctx, order.ID); err != nil {
		slog.Error("apply_pending_payments", "order_id", order.ID, "err", err)
	}
	return s.orders.Get(ctx, order.ID)
}

func (s *Orders) Get(ctx context.Context, id string) (domain.Order, error) {
	if id == "" {
		return domain.Order{}, domain.ErrInvalid
	}
	return s.orders.Get(ctx, id)
}
