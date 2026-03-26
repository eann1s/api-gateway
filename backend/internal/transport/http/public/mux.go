package http_public

import "net/http"


func NewPublicMux(handlers *Handlers) *http.ServeMux {
	m := http.NewServeMux()
	return m
}
