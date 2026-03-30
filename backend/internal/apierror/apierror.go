package apierror

import (
	"encoding/json"
	"errors"
	"net/http"
	"github.com/eann1s/rate-limiter/backend/internal/requestid"
)


type APIError struct {
	Status int
	Code string
	Message string
	Err error
}

func (e *APIError) Error() string { return e.Message }

func (e *APIError) Unwrap() error { return e.Err }

type response struct {
	Status int `json:"status"`
	Code string `json:"code"`
	Message string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

var (
	ErrBadRequest = &APIError{
		Status: http.StatusBadRequest,
		Code: "bad_request",
		Message: "Bad request",
	}
	ErrNotFound = &APIError{
		Status: http.StatusNotFound,
		Code: "not_found",
		Message: "Not found",
	}
)

func mapError(err error) *APIError {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae
	}
	return &APIError{
		Status: http.StatusInternalServerError,
		Code: "internal_error",
		Message: "Internal server error",
		Err: err,
	}
}

func Write(w http.ResponseWriter, r *http.Request, err error) error {
	
	ae := mapError(err)

	resp := response{
		Status: ae.Status,
		Code: ae.Code,
		Message: ae.Message,
	}

	reqID, ok := requestid.FromContext(r.Context())
	if ok {
		resp.RequestID = reqID
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ae.Status)

	return json.NewEncoder(w).Encode(resp)
}
