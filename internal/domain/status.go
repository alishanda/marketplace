package domain

const (
	StatusCreated        = "created"
	StatusPaid           = "paid"
	StatusDelivering     = "delivering"
	StatusDelivered      = "delivered"
	StatusPaymentFailed  = "payment_failed"
	StatusOutOfStock     = "out_of_stock"
	StatusDeliveryFailed = "delivery_failed"
)

const (
	PaymentPaid   = "paid"
	PaymentFailed = "failed"
)

const (
	ProviderA = "A"
	ProviderB = "B"
)

const (
	AccountCustomer = "customer"
	AccountEscrow   = "escrow"
	AccountRevenue  = "revenue"
)

const (
	LedgerPayment    = "payment"
	LedgerSettlement = "settlement"
)

func IsFinal(status string) bool {
	return status == StatusDelivered || status == StatusPaymentFailed
}

func IsPaidLike(status string) bool {
	switch status {
	case StatusPaid, StatusDelivering, StatusDelivered, StatusOutOfStock, StatusDeliveryFailed:
		return true
	default:
		return false
	}
}

func CanDeliver(status string) bool {
	switch status {
	case StatusPaid, StatusDelivering, StatusOutOfStock, StatusDeliveryFailed:
		return true
	default:
		return false
	}
}

func CanAcceptPaid(status string) bool {
	return status == StatusCreated || status == StatusPaymentFailed
}

func CanAcceptFailed(status string) bool {
	return status == StatusCreated
}
