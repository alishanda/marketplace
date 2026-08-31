package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"marketplace/internal/domain"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal"
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
	case errors.Is(err, domain.ErrInvalid):
		status = http.StatusBadRequest
		code = "invalid"
	case errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
		code = "conflict"
	case errors.Is(err, domain.ErrOutOfStock):
		status = http.StatusConflict
		code = "out_of_stock"
	}
	writeJSON(w, status, map[string]string{"error": code, "message": err.Error()})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return domain.ErrInvalid
	}
	return nil
}
