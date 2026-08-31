package domain

import (
	"crypto/rand"
	"encoding/hex"
)

func NewID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return prefix + hex.EncodeToString(buf)
}

func RequestID(orderID string, provider string) string {
	switch provider {
	case ProviderB:
		return "req_" + orderID + "-b"
	default:
		return "req_" + orderID + "-a"
	}
}
