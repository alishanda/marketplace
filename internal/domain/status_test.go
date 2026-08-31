package domain

import "testing"

func TestTransitions(t *testing.T) {
	if !CanAcceptPaid(StatusCreated) || !CanAcceptPaid(StatusPaymentFailed) {
		t.Fatal("paid should reopen created/failed")
	}
	if CanAcceptPaid(StatusDelivered) {
		t.Fatal("delivered is final")
	}
	if !CanDeliver(StatusPaid) || !CanDeliver(StatusOutOfStock) {
		t.Fatal("recoverable states must deliver")
	}
	if CanDeliver(StatusCreated) {
		t.Fatal("unpaid must not deliver")
	}
	if !IsFinal(StatusDelivered) || !IsFinal(StatusPaymentFailed) {
		t.Fatal("final flags")
	}
}
