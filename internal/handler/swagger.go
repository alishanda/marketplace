package handler

import (
	"net/http"

	"marketplace/docs"
)

func (a *API) swaggerUI(w http.ResponseWriter, _ *http.Request) {
	raw, err := docs.FS.ReadFile("swagger.html")
	if err != nil {
		http.Error(w, "swagger ui missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(raw)
}

func (a *API) openapiSpec(w http.ResponseWriter, _ *http.Request) {
	raw, err := docs.FS.ReadFile("openapi.yaml")
	if err != nil {
		http.Error(w, "openapi missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(raw)
}
