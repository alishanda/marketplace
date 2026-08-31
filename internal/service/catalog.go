package service

import (
	"context"

	"marketplace/internal/domain"
	"marketplace/internal/repository"
)

type Catalog struct {
	products *repository.ProductRepo
}

func NewCatalog(products *repository.ProductRepo) *Catalog {
	return &Catalog{products: products}
}

func (s *Catalog) Storefront(ctx context.Context, limit, offset int) ([]domain.Product, error) {
	if limit <= 0 || limit > 200 {
		limit = 48
	}
	if offset < 0 {
		offset = 0
	}
	return s.products.Storefront(ctx, limit, offset)
}

func (s *Catalog) Get(ctx context.Context, sku string) (domain.Product, error) {
	if sku == "" {
		return domain.Product{}, domain.ErrInvalid
	}
	return s.products.Get(ctx, sku)
}

func (s *Catalog) All(ctx context.Context) ([]domain.Product, error) {
	return s.products.All(ctx)
}

func (s *Catalog) Featured(ctx context.Context) ([]domain.Product, error) {
	return s.products.Featured(ctx)
}
