package provider

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"marketplace/internal/domain"
	"marketplace/internal/repository"

	"github.com/jackc/pgx/v5"
)

type Stub struct {
	name        string
	inventory   *repository.InventoryRepo
	products    *repository.ProductRepo
	db          *repository.DB
	mu          sync.RWMutex
	errorRate   float64
	timeoutRate float64
	hangFor     time.Duration
}

func NewStub(name string, db *repository.DB, inventory *repository.InventoryRepo, products *repository.ProductRepo, errorRate, timeoutRate float64, hangFor time.Duration) *Stub {
	return &Stub{
		name:        name,
		db:          db,
		inventory:   inventory,
		products:    products,
		errorRate:   errorRate,
		timeoutRate: timeoutRate,
		hangFor:     hangFor,
	}
}

func (s *Stub) SetRates(errorRate, timeoutRate float64) {
	s.mu.Lock()
	s.errorRate = errorRate
	s.timeoutRate = timeoutRate
	s.mu.Unlock()
}

func (s *Stub) Rates() domain.ProviderRates {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return domain.ProviderRates{ErrorRate: s.errorRate, TimeoutRate: s.timeoutRate}
}

func (s *Stub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req domain.IssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RequestID == "" || req.SKU == "" || req.OrderID == "" {
		writeJSON(w, http.StatusBadRequest, domain.IssueResponse{Status: "error", Reason: "invalid_request"})
		return
	}

	existing, err := s.inventory.CodeByRequest(r.Context(), s.db.Pool, req.RequestID)
	if err == nil {
		writeJSON(w, http.StatusOK, domain.IssueResponse{Status: "ok", RequestID: req.RequestID, Code: existing})
		return
	}
	if !errors.Is(err, domain.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, domain.IssueResponse{Status: "error", Reason: "internal"})
		return
	}

	s.mu.RLock()
	errorRate := s.errorRate
	timeoutRate := s.timeoutRate
	hang := s.hangFor
	s.mu.RUnlock()

	roll := rand.Float64()
	if roll < errorRate {
		writeJSON(w, http.StatusBadGateway, domain.IssueResponse{Status: "error", Reason: "upstream"})
		return
	}

	code, err := s.reserve(r.Context(), req)
	if errors.Is(err, domain.ErrOutOfStock) {
		writeJSON(w, http.StatusConflict, domain.IssueResponse{Status: "error", Reason: "out_of_stock"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, domain.IssueResponse{Status: "error", Reason: "internal"})
		return
	}

	if roll < errorRate+timeoutRate {
		timer := time.NewTimer(hang)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
			return
		case <-timer.C:
			return
		}
	}

	writeJSON(w, http.StatusOK, domain.IssueResponse{
		Status:    "ok",
		RequestID: req.RequestID,
		Code:      code,
	})
}

func (s *Stub) reserve(ctx context.Context, req domain.IssueRequest) (string, error) {
	var code string
	err := s.db.InTx(ctx, func(tx pgx.Tx) error {
		var reserveErr error
		code, reserveErr = s.inventory.Reserve(ctx, tx, req.SKU, req.OrderID, req.RequestID, s.name)
		if reserveErr != nil {
			return reserveErr
		}
		return s.products.RefreshStock(ctx, tx, req.SKU)
	})
	return code, err
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
