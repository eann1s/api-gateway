package http_admin

import "net/http"


func NewAdminMux(handlers *Handlers) *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("/healthz", handlers.Healthz)
	m.HandleFunc("/readyz", handlers.Readyz)
	m.HandleFunc("/metrics", handlers.Metrics)
	return m
}
