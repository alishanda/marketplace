package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"marketplace/internal/app"
	"marketplace/internal/config"
	"marketplace/internal/domain"
)

func testCfg() config.Config {
	cfg := config.Load()
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		cfg.DatabaseURL = u
	}
	cfg.HTTPAddr = "127.0.0.1:0"
	cfg.CatalogSeedExtra = 0
	cfg.ProviderTimeout = 400 * time.Millisecond
	cfg.ProviderAErrorRate = 0
	cfg.ProviderATimeoutRate = 0
	cfg.ProviderBErrorRate = 0
	cfg.ProviderBTimeoutRate = 0
	cfg.WorkerInterval = time.Second
	cfg.StuckAfter = 2 * time.Second
	return cfg
}

func startApp(t *testing.T) (*app.App, string) {
	t.Helper()
	ctx := context.Background()
	a, err := app.New(ctx, testCfg())
	if err != nil {
		t.Skipf("db unavailable: %v", err)
	}
	a.Start(ctx)
	t.Cleanup(func() {
		shut, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		a.Close(shut)
	})
	return a, a.URL()
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func readJSON(t *testing.T, res *http.Response, dst any) {
	t.Helper()
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if dst != nil {
		if err := json.Unmarshal(raw, dst); err != nil {
			t.Fatalf("decode %s: %s", err, raw)
		}
	}
}

func createOrder(t *testing.T, base, sku string, id string) domain.Order {
	t.Helper()
	payload := map[string]string{"sku": sku}
	if id != "" {
		payload["id"] = id
	}
	res := postJSON(t, base+"/api/orders", payload)
	var order domain.Order
	readJSON(t, res, &order)
	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
		t.Fatalf("create order: %d %+v", res.StatusCode, order)
	}
	return order
}

func getOrder(t *testing.T, base, id string) domain.Order {
	t.Helper()
	res, err := http.Get(base + "/api/orders/" + id)
	if err != nil {
		t.Fatal(err)
	}
	var order domain.Order
	readJSON(t, res, &order)
	return order
}

func webhook(t *testing.T, base string, ev map[string]any) int {
	t.Helper()
	code, err := postWebhook(base, ev)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func postWebhook(base string, ev map[string]any) (int, error) {
	raw, err := json.Marshal(ev)
	if err != nil {
		return 0, err
	}
	res, err := http.Post(base+"/webhook/payment", "application/json", bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	return res.StatusCode, nil
}

func waitStatus(t *testing.T, base, id string, want ...string) domain.Order {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		o := getOrder(t, base, id)
		for _, s := range want {
			if o.Status == s {
				return o
			}
		}
		time.Sleep(80 * time.Millisecond)
	}
	o := getOrder(t, base, id)
	t.Fatalf("order %s stuck in %s, want %v", id, o.Status, want)
	return o
}

func ensureStock(t *testing.T, base, sku string, n int) {
	t.Helper()
	codes := make([]string, n)
	for i := 0; i < n; i++ {
		codes[i] = domain.NewID("K")
	}
	res := postJSON(t, base+"/admin/inventory/restock", map[string]any{"sku": sku, "codes": codes})
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
}

func setRates(t *testing.T, base string, aErr, aTo, bErr, bTo float64) {
	t.Helper()
	res := postJSON(t, base+"/admin/providers/config", map[string]any{
		"a": map[string]float64{"error_rate": aErr, "timeout_rate": aTo},
		"b": map[string]float64{"error_rate": bErr, "timeout_rate": bTo},
	})
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
}

func TestRaceFiftyWebhooks(t *testing.T) {
	_, base := startApp(t)
	ensureStock(t, base, "STEAM-TOPUP-500", 2)
	order := createOrder(t, base, "STEAM-TOPUP-500", "")
	var codes int64
	var wg sync.WaitGroup
	errCh := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code, err := postWebhook(base, map[string]any{
				"event_id":   fmt.Sprintf("evt_race_%s_%d", order.ID, i),
				"order_id":   order.ID,
				"status":     "paid",
				"amount":     order.Amount,
				"currency":   "RUB",
				"created_at": "2025-01-01T12:00:00Z",
			})
			if err != nil {
				errCh <- err
				return
			}
			if code == http.StatusOK {
				atomic.AddInt64(&codes, 1)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	if codes != 50 {
		t.Fatalf("accepted %d/50", codes)
	}
	got := waitStatus(t, base, order.ID, domain.StatusDelivered)
	if got.DeliveryCode == nil || *got.DeliveryCode == "" {
		t.Fatal("empty code")
	}
	again := getOrder(t, base, order.ID)
	if again.DeliveryCode == nil || *again.DeliveryCode != *got.DeliveryCode {
		t.Fatalf("code changed %v -> %v", got.DeliveryCode, again.DeliveryCode)
	}
}

func TestDuplicateEventID(t *testing.T) {
	_, base := startApp(t)
	ensureStock(t, base, "KEY-CS2-PRIME", 2)
	order := createOrder(t, base, "KEY-CS2-PRIME", "")
	ev := map[string]any{
		"event_id":   "evt_same_" + order.ID,
		"order_id":   order.ID,
		"status":     "paid",
		"amount":     order.Amount,
		"currency":   "RUB",
		"created_at": "2025-01-01T12:00:00Z",
	}
	if webhook(t, base, ev) != http.StatusOK {
		t.Fatal("first webhook")
	}
	got := waitStatus(t, base, order.ID, domain.StatusDelivered)
	if webhook(t, base, ev) != http.StatusOK {
		t.Fatal("replay")
	}
	again := getOrder(t, base, order.ID)
	if again.Status != got.Status || *again.DeliveryCode != *got.DeliveryCode {
		t.Fatalf("replay mutated order: %+v", again)
	}
}

func TestWebhookBeforeOrder(t *testing.T) {
	_, base := startApp(t)
	ensureStock(t, base, "STEAM-TOPUP-1000", 2)
	id := domain.NewID("ord_")
	if webhook(t, base, map[string]any{
		"event_id":   "evt_early_" + id,
		"order_id":   id,
		"status":     "paid",
		"amount":     500,
		"currency":   "RUB",
		"created_at": "2025-01-01T12:00:00Z",
	}) != http.StatusOK {
		t.Fatal("early webhook")
	}
	order := createOrder(t, base, "STEAM-TOPUP-1000", id)
	if order.ID != id {
		t.Fatalf("id mismatch %s", order.ID)
	}
	got := waitStatus(t, base, id, domain.StatusDelivered, domain.StatusPaid, domain.StatusDelivering)
	if !domain.IsPaidLike(got.Status) && got.Status != domain.StatusDelivered {
		t.Fatalf("early pay not applied: %s", got.Status)
	}
	waitStatus(t, base, id, domain.StatusDelivered)
}

func TestTimeoutSameRequestID(t *testing.T) {
	_, base := startApp(t)
	ensureStock(t, base, "SUB-SPOTIFY-1M", 2)
	setRates(t, base, 0, 1, 1, 1)
	order := createOrder(t, base, "SUB-SPOTIFY-1M", "")
	webhook(t, base, map[string]any{
		"event_id":   "evt_to_" + order.ID,
		"order_id":   order.ID,
		"status":     "paid",
		"amount":     order.Amount,
		"currency":   "RUB",
		"created_at": "2025-01-01T12:00:00Z",
	})
	got := waitStatus(t, base, order.ID, domain.StatusDelivered)
	if got.DeliveryProvider == nil || *got.DeliveryProvider != domain.ProviderA {
		t.Fatalf("expected provider A after timeout retry, got %v", got.DeliveryProvider)
	}
	if got.DeliveryRequestID == nil || *got.DeliveryRequestID != domain.RequestID(order.ID, domain.ProviderA) {
		t.Fatalf("request_id %v", got.DeliveryRequestID)
	}
}

func TestFallbackToB(t *testing.T) {
	_, base := startApp(t)
	ensureStock(t, base, "GIFT-PSN-1000", 2)
	setRates(t, base, 1, 0, 0, 0)
	order := createOrder(t, base, "GIFT-PSN-1000", "")
	webhook(t, base, map[string]any{
		"event_id":   "evt_fb_" + order.ID,
		"order_id":   order.ID,
		"status":     "paid",
		"amount":     order.Amount,
		"currency":   "RUB",
		"created_at": "2025-01-01T12:00:00Z",
	})
	got := waitStatus(t, base, order.ID, domain.StatusDelivered)
	if got.DeliveryProvider == nil || *got.DeliveryProvider != domain.ProviderB {
		t.Fatalf("expected B, got %v", got.DeliveryProvider)
	}
}

func TestOutOfStockRecoverable(t *testing.T) {
	_, base := startApp(t)
	setRates(t, base, 0, 0, 0, 0)
	sku := "SUB-YT-3M"
	var empty domain.Order
	for i := 0; i < 40; i++ {
		o := createOrder(t, base, sku, "")
		webhook(t, base, map[string]any{
			"event_id":   fmt.Sprintf("evt_stock_%s_%d", o.ID, i),
			"order_id":   o.ID,
			"status":     "paid",
			"amount":     o.Amount,
			"currency":   "RUB",
			"created_at": "2025-01-01T12:00:00Z",
		})
		got := waitStatus(t, base, o.ID, domain.StatusDelivered, domain.StatusOutOfStock)
		if got.Status == domain.StatusOutOfStock {
			empty = got
			break
		}
	}
	if empty.ID == "" {
		t.Fatal("could not exhaust stock")
	}
	res := postJSON(t, base+"/admin/inventory/restock", map[string]any{
		"sku": sku,
		"codes": []string{
			domain.NewID("R"),
			domain.NewID("R"),
			domain.NewID("R"),
		},
	})
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	retry := postJSON(t, base+"/admin/orders/"+empty.ID+"/retry", map[string]any{})
	io.Copy(io.Discard, retry.Body)
	retry.Body.Close()
	got := waitStatus(t, base, empty.ID, domain.StatusDelivered)
	if got.DeliveryCode == nil {
		t.Fatal("recovery without code")
	}
}

func TestLedgerBalanced(t *testing.T) {
	_, base := startApp(t)
	ensureStock(t, base, "KEY-GTA5", 2)
	order := createOrder(t, base, "KEY-GTA5", "")
	webhook(t, base, map[string]any{
		"event_id":   "evt_led_" + order.ID,
		"order_id":   order.ID,
		"status":     "paid",
		"amount":     order.Amount,
		"currency":   "RUB",
		"created_at": "2025-01-01T12:00:00Z",
	})
	waitStatus(t, base, order.ID, domain.StatusDelivered)
	res, err := http.Get(base + "/admin/reconcile")
	if err != nil {
		t.Fatal(err)
	}
	var report domain.ReconcileReport
	readJSON(t, res, &report)
	if !report.Ledger.Balanced {
		t.Fatalf("ledger %d/%d", report.Ledger.Debit, report.Ledger.Credit)
	}
}
