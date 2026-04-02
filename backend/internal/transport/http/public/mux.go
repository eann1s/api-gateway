package http_public

import (
	"net/http"
)


func NewPublicMux(handlers *Handlers) *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("/", handlers.Root)
	return m
}
