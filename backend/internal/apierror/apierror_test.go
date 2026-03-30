package apierror

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eann1s/rate-limiter/backend/internal/requestid"
)


func TestApierror_mapError(t *testing.T) {
	t.Parallel()
	
	tests := []struct{
		name string
		err error
		want *APIError
	} {
		{
			name: "Return api error when error is api error",
			err: &APIError{
				Status: http.StatusBadRequest,
				Code: "bad_request",
				Message: "Bad request",
			},
			want: &APIError{
				Status: http.StatusBadRequest,
				Code: "bad_request",
				Message: "Bad request",
			},
		},
		{
			name: "Return internal error when error is not api error",
			err: errors.New("unknown"),
			want: &APIError{
				Status: http.StatusInternalServerError,
				Code: "internal_error",
				Message: "Internal server error",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := mapError(tt.err)
			if got.Status != tt.want.Status || got.Code != tt.want.Code || got.Message != tt.want.Message {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestApierror_Write(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		err error
		requestID string
		wantStatus int
		wantCode string
		wantMessage string
		wantReqID string
	} {
		{
			name: "api error without request id",
			err: &APIError{
				Status: http.StatusBadRequest,
				Code: "bad_request",
				Message: "Bad request",
			},
			wantStatus: http.StatusBadRequest,
			wantCode: "bad_request",
			wantMessage: "Bad request",
		},
		{
			name: "api error with request id",
			err: &APIError{
				Status: http.StatusBadRequest,
				Code: "bad_request",
				Message: "Bad request",
			},
			requestID: "123",
			wantStatus: http.StatusBadRequest,
			wantCode: "bad_request",
			wantMessage: "Bad request",
			wantReqID: "123",
		},
		{
			name: "unknown error maps to internal error",
			err: errors.New("unknown"),
			wantStatus: http.StatusInternalServerError,
			wantCode: "internal_error",
			wantMessage: "Internal server error",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			if tt.requestID != "" {
				req = req.WithContext(requestid.WithContext(req.Context(), tt.requestID))
			}

			Write(w, req, tt.err)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			var got response
			err := json.NewDecoder(w.Body).Decode(&got)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", got.Status, tt.wantStatus)
			}
			if got.Code != tt.wantCode {
				t.Fatalf("code = %s, want %s", got.Code, tt.wantCode)
			}
			if got.Message != tt.wantMessage {
				t.Fatalf("message = %s, want %s", got.Message, tt.wantMessage)
			}
			if tt.requestID != "" && got.RequestID != tt.wantReqID {
				t.Fatalf("request_id = %s, want %s", got.RequestID, tt.wantReqID)
			}
			if tt.requestID == "" && got.RequestID != "" {
				t.Fatalf("request_id = %s, want %s", got.RequestID, tt.wantReqID)
			}
		})
	}
}
