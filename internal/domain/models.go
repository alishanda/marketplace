package domain

import "time"

type Product struct {
	SKU      string `json:"sku"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Price    int    `json:"price"`
	Currency string `json:"currency"`
	Image    string `json:"image"`
	Stock    int    `json:"stock"`
}

type Order struct {
	ID                string    `json:"id"`
	SKU               string    `json:"sku"`
	Amount            int       `json:"amount"`
	Currency          string    `json:"currency"`
	Status            string    `json:"status"`
	DeliveryCode      *string   `json:"delivery_code,omitempty"`
	DeliveryRequestID *string   `json:"delivery_request_id,omitempty"`
	DeliveryProvider  *string   `json:"delivery_provider,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type PaymentEvent struct {
	EventID     string     `json:"event_id"`
	OrderID     string     `json:"order_id"`
	Status      string     `json:"status"`
	Amount      int        `json:"amount"`
	Currency    string     `json:"currency"`
	CreatedAt   time.Time  `json:"created_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
}

type IssueRequest struct {
	RequestID string `json:"request_id"`
	SKU       string `json:"sku"`
	OrderID   string `json:"order_id"`
}

type IssueResponse struct {
	Status    string `json:"status"`
	RequestID string `json:"request_id,omitempty"`
	Code      string `json:"code,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type LedgerEntry struct {
	ID        int64
	OrderID   string
	EventID   *string
	Debit     int
	Credit    int
	Account   string
	EntryType string
	CreatedAt time.Time
}

type ReconcileReport struct {
	PaidNotDelivered []Order       `json:"paid_not_delivered"`
	DeliveredNotPaid []Order       `json:"delivered_not_paid"`
	Ledger           LedgerBalance `json:"ledger"`
	StuckDelivering  []Order       `json:"stuck_delivering"`
}

type LedgerBalance struct {
	Debit    int  `json:"debit"`
	Credit   int  `json:"credit"`
	Balanced bool `json:"balanced"`
}

type ProviderRates struct {
	ErrorRate   float64 `json:"error_rate"`
	TimeoutRate float64 `json:"timeout_rate"`
}
