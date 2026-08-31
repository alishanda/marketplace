package handler

import (
	"net/http"
	"strconv"
	"time"

	"marketplace/internal/domain"
	"marketplace/internal/provider"
	"marketplace/internal/service"
)

type API struct {
	catalog   *service.Catalog
	orders    *service.Orders
	payments  *service.Payment
	delivery  *service.Delivery
	reconcile *service.Reconcile
	recovery  *service.Recovery
	stubA     *provider.Stub
	stubB     *provider.Stub
	inventory *inventoryAdmin
}

type inventoryAdmin struct {
	restock func(sku string, codes []string) error
}

func New(
	catalog *service.Catalog,
	orders *service.Orders,
	payments *service.Payment,
	delivery *service.Delivery,
	reconcile *service.Reconcile,
	recovery *service.Recovery,
	stubA, stubB *provider.Stub,
	restock func(sku string, codes []string) error,
) *API {
	return &API{
		catalog:   catalog,
		orders:    orders,
		payments:  payments,
		delivery:  delivery,
		reconcile: reconcile,
		recovery:  recovery,
		stubA:     stubA,
		stubB:     stubB,
		inventory: &inventoryAdmin{restock: restock},
	}
}

func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /swagger", a.swaggerUI)
	mux.HandleFunc("GET /swagger/", a.swaggerUI)
	mux.HandleFunc("GET /openapi.yaml", a.openapiSpec)
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("GET /api/catalog", a.catalogList)
	mux.HandleFunc("GET /api/catalog/{sku}", a.catalogGet)
	mux.HandleFunc("POST /api/orders", a.createOrder)
	mux.HandleFunc("GET /api/orders/{id}", a.getOrder)
	mux.HandleFunc("POST /api/orders/{id}/pay", a.simulatePay)
	mux.HandleFunc("POST /webhook/payment", a.paymentWebhook)
	mux.HandleFunc("POST /internal/providers/a/issue", a.stubA.ServeHTTP)
	mux.HandleFunc("POST /internal/providers/b/issue", a.stubB.ServeHTTP)
	mux.HandleFunc("GET /admin/reconcile", a.reconcileReport)
	mux.HandleFunc("POST /admin/orders/{id}/retry", a.retryDelivery)
	mux.HandleFunc("POST /admin/recovery/tick", a.recoveryTick)
	mux.HandleFunc("POST /admin/providers/config", a.providerConfig)
	mux.HandleFunc("GET /admin/providers/config", a.providerConfigGet)
	mux.HandleFunc("POST /admin/inventory/restock", a.restock)
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) catalogList(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	featured := r.URL.Query().Get("featured") == "1"
	var (
		items []domain.Product
		err   error
	)
	if featured {
		items, err = a.catalog.Featured(r.Context())
	} else {
		items, err = a.catalog.Storefront(r.Context(), limit, offset)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"products": items, "count": len(items)})
}

func (a *API) catalogGet(w http.ResponseWriter, r *http.Request) {
	p, err := a.catalog.Get(r.Context(), r.PathValue("sku"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *API) createOrder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID  string `json:"id"`
		SKU string `json:"sku"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	order, err := a.orders.Create(r.Context(), service.CreateOrderInput{ID: in.ID, SKU: in.SKU})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (a *API) getOrder(w http.ResponseWriter, r *http.Request) {
	order, err := a.orders.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (a *API) simulatePay(w http.ResponseWriter, r *http.Request) {
	order, err := a.orders.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	ev := domain.PaymentEvent{
		EventID:   domain.NewID("evt_"),
		OrderID:   order.ID,
		Status:    domain.PaymentPaid,
		Amount:    order.Amount,
		Currency:  order.Currency,
		CreatedAt: time.Now().UTC(),
	}
	if err := a.payments.HandleWebhook(r.Context(), ev); err != nil {
		writeError(w, err)
		return
	}
	updated, err := a.orders.Get(r.Context(), order.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) paymentWebhook(w http.ResponseWriter, r *http.Request) {
	var in struct {
		EventID   string `json:"event_id"`
		OrderID   string `json:"order_id"`
		Status    string `json:"status"`
		Amount    int    `json:"amount"`
		Currency  string `json:"currency"`
		CreatedAt string `json:"created_at"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	ts := time.Now().UTC()
	if in.CreatedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, in.CreatedAt); err == nil {
			ts = parsed
		}
	}
	err := a.payments.HandleWebhook(r.Context(), domain.PaymentEvent{
		EventID:   in.EventID,
		OrderID:   in.OrderID,
		Status:    in.Status,
		Amount:    in.Amount,
		Currency:  in.Currency,
		CreatedAt: ts,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

func (a *API) reconcileReport(w http.ResponseWriter, r *http.Request) {
	report, err := a.reconcile.Report(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (a *API) retryDelivery(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.delivery.Deliver(r.Context(), id); err != nil && err != domain.ErrInvalid {
		writeError(w, err)
		return
	}
	order, err := a.orders.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (a *API) recoveryTick(w http.ResponseWriter, r *http.Request) {
	a.recovery.Tick(r.Context())
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) providerConfig(w http.ResponseWriter, r *http.Request) {
	var in struct {
		A domain.ProviderRates `json:"a"`
		B domain.ProviderRates `json:"b"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	a.stubA.SetRates(in.A.ErrorRate, in.A.TimeoutRate)
	a.stubB.SetRates(in.B.ErrorRate, in.B.TimeoutRate)
	writeJSON(w, http.StatusOK, map[string]any{"a": a.stubA.Rates(), "b": a.stubB.Rates()})
}

func (a *API) providerConfigGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"a": a.stubA.Rates(), "b": a.stubB.Rates()})
}

func (a *API) restock(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SKU   string   `json:"sku"`
		Codes []string `json:"codes"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	if err := a.inventory.restock(in.SKU, in.Codes); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
