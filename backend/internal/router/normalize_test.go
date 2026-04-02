package router

import (
	"errors"
	"slices"
	"testing"
)


func TestLongestPrefixNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		rawPrefixes  []string
		wantPrefixes []string
		wantErr      error
	}{
		{
			name:         "trims spaces and keeps leading slash",
			rawPrefixes:  []string{"/api", " /api/v1  ", " /api/v1/users"},
			wantPrefixes: []string{"/api", "/api/v1", "/api/v1/users"},
		},
		{
			name:         "trims and dedupes",
			rawPrefixes:  []string{"/api", " /api  "},
			wantPrefixes: []string{"/api"},
		},
		{
			name:         "add leading slash",
			rawPrefixes:  []string{"api"},
			wantPrefixes: []string{"/api"},
		},
		{
			name:         "collapses duplicate slashes and removes trailing ones",
			rawPrefixes:  []string{"//api/v1//users//"},
			wantPrefixes: []string{"/api/v1/users"},
		},
		{
			name:         "keep root untouched",
			rawPrefixes:  []string{"/"},
			wantPrefixes: []string{"/"},
		},
		{
			name:         "resolves root from duplicate slashes",
			rawPrefixes:  []string{"////"},
			wantPrefixes: []string{"/"},
		},
		{
			name:         "ignore empty or blank strings",
			rawPrefixes:  []string{"", "   "},
			wantPrefixes: []string{},
			wantErr: ErrInvalidPrefix,
		},
		{
			name:         "errors on invalid chars",
			rawPrefixes:  []string{"/api?/v1#users"},
			wantPrefixes: []string{},
			wantErr:      ErrInvalidPrefix,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res, err := normalizePrefixes(tt.rawPrefixes)
			if !slices.Equal(res, tt.wantPrefixes) {
				t.Fatalf("want prefixes %+v, got %+v", tt.wantPrefixes, res)
			}
			if tt.wantErr == nil && err != nil {
				t.Fatalf("want no error, got %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("want error %v, got %v", tt.wantErr, err)
			}
		})
	}
}
