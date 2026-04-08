package proxy

import (
	"errors"
	"io"
	"strings"
	"testing"
)


func TestLimitedBody_NewLimitedBody(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		inner io.ReadCloser
		limit int64
		wantErr error
	} {
		{ 
			name: "success", 
			inner: io.NopCloser(strings.NewReader("1234567890")), 
			limit: 10 * 1024 * 1024,
			wantErr: nil,
		},
		{ 
			name: "invalid limit", 
			inner: io.NopCloser(strings.NewReader("1234567890")), 
			limit: 0,
			wantErr: ErrInvalidLimitValue,
		},
		{ 
			name: "invalid inner reader", 
			inner: nil,
			limit: 10 * 1024 * 1024,
			wantErr: ErrInvalidInnerReader,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := newLimitedBody(tt.inner, tt.limit)
			if tt.wantErr != nil && err == nil {
				t.Fatalf("expected error to be %v, got nil", tt.wantErr)
			}
			if err != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error to be %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestLimitedBody_Read(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		inner io.ReadCloser
		limit int64
		bufSize int
		wantBytesRead int
		wantErr error
	} {
		{ 
			name: "success", 
			inner: io.NopCloser(strings.NewReader("1234567890")), 
			limit: 10 * 1024 * 1024,
			bufSize: 10,
			wantBytesRead: 10,
			wantErr: io.EOF,
		},
		{ 
			name: "overflow", 
			inner: io.NopCloser(strings.NewReader("1234567890")), 
			limit: 5,
			bufSize: 10,
			wantBytesRead: 5,
			wantErr: ErrRequestBodyTooLarge,
		},
		{ 
			name: "exactly at limit", 
			inner: io.NopCloser(strings.NewReader("1234567890")), 
			limit: 10,
			bufSize: 10,
			wantBytesRead: 10,
			wantErr: io.EOF,
		},
		{ 
			name: "extra bytes later", 
			inner: io.NopCloser(strings.NewReader("1234567890")), 
			limit: 9,
			bufSize: 9,
			wantBytesRead: 9,
			wantErr: ErrRequestBodyTooLarge,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			buf := make([]byte, tt.bufSize)
			b, err := newLimitedBody(tt.inner, tt.limit)
			if err != nil {
				t.Fatal(err)
			}

			var bytesRead int
			for {
				n, err := b.Read(buf)
				bytesRead += n
				if err != nil {
					if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
						t.Errorf("expected error %v, got %v", tt.wantErr, err)
					}
					break
				}
			}
			if bytesRead != tt.wantBytesRead {
				t.Errorf("bytesRead = %v, want %v", bytesRead, tt.wantBytesRead)
			}
		})
	}
}

func TestLimitedBody_DelegatedClose(t *testing.T) {
	t.Parallel()

	b := &limitedBody{inner: io.NopCloser(strings.NewReader("1234567890"))}
	err := b.Close()
	if err != nil {
		t.Fatalf("expected error to be nil")
	}
}
