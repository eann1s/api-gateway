package router

import (
	"errors"
	"fmt"
	"maps"
	"net"
	"regexp"
	"slices"
	"strings"
)

var prefixSegmentRe = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)
var hostRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$`)

var ErrInvalidPrefix = errors.New("invalid prefix")
var ErrInvalidHost = errors.New("invalid host")

func NormalizeAndValidateHost(host string) (string, error) {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return "", fmt.Errorf("%w: host cannot be empty", ErrInvalidHost)
	}
	res, _, err := net.SplitHostPort(host)
	if err == nil && res != "" {
		host = res
	}
	ok := hostRe.MatchString(host)
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrInvalidHost, host)
	}
	return host, nil
}

func normalizePrefix(prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", fmt.Errorf("%w: %q", ErrInvalidPrefix, prefix)
	}
	var sb strings.Builder

	rawParts := strings.Split(prefix, "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part != "" {
			if !prefixSegmentRe.MatchString(part) {
				return "", fmt.Errorf("%w: segment=%q, raw=%q", ErrInvalidPrefix, part, prefix)
			}
			parts = append(parts, part)
		}
	}

	for i, part := range parts {
		sb.WriteString(part)
		if i < len(parts)-1 {
			sb.WriteRune('/')
		}
	}

	if sb.String() == "" {
		return "/", nil
	}
	return "/" + sb.String(), nil
}

func normalizePrefixes(rawPrefixes []string) ([]string, error) {
	pset := make(map[string]struct{}, len(rawPrefixes))
	for _, raw := range rawPrefixes {
		normalized, err := normalizePrefix(raw)
		if err != nil {
			return nil, err
		}
		pset[normalized] = struct{}{}
	}
	res := slices.Collect(maps.Keys(pset))
	slices.Sort(res)
	return res, nil
}

