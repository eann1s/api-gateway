package proxy

import (
	"errors"
	"fmt"
	"io"
)

var (
	ErrRequestBodyTooLarge = errors.New("request body too large")
	ErrInvalidLimitValue   = errors.New("invalid limit")
	ErrInvalidInnerReader  = errors.New("invalid inner reader")
)

type limitedBody struct {
	inner     io.ReadCloser
	limit     int64
	bytesRead int64
}

func newLimitedBody(inner io.ReadCloser, limit int64) (*limitedBody, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("%w, limit should be > 0, received: %d", ErrInvalidLimitValue, limit)
	}
	if inner == nil {
		return nil, fmt.Errorf("%w, inner reader is required", ErrInvalidInnerReader)
	}

	return &limitedBody{
		inner: inner,
		limit: limit,
	}, nil
}

func (l *limitedBody) Read(p []byte) (n int, err error) {
	var probe [1]byte
	remaining := l.limit - l.bytesRead
	if remaining == 0 {
		n, err := l.inner.Read(probe[:])
		if n > 0 {
			return 0, ErrRequestBodyTooLarge
		}
		if err == nil {
			return n, io.EOF
		}
		return n, err
	}
	if remaining < 0 {
		return 0, ErrRequestBodyTooLarge
	}

	if int64(len(p)) > remaining {
		p = p[:int(remaining)+1]
	}
	n, err = l.inner.Read(p)
	l.bytesRead += int64(n)
	if int64(n) > remaining {
		return int(remaining), ErrRequestBodyTooLarge
	}
	return n, err
}

func (l *limitedBody) Close() error {
	return l.inner.Close()
}
