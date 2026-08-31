package domain

import "errors"

var (
	ErrNotFound         = errors.New("not_found")
	ErrConflict         = errors.New("conflict")
	ErrInvalid          = errors.New("invalid")
	ErrOutOfStock       = errors.New("out_of_stock")
	ErrProvider         = errors.New("provider_error")
	ErrProviderTimeout  = errors.New("provider_timeout")
	ErrAlreadyDelivered = errors.New("already_delivered")
)
