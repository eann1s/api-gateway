package config

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)


type Config struct {
	Listeners ListenersConfig `yaml:"listeners"`
	Observability ObservabilityConfig `yaml:"observability"`
	Defaults DefaultsConfig `yaml:"defaults"`
	Shutdown ShutdownConfig `yaml:"shutdown"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Redis RedisConfig `yaml:"redis"`
	Routes []RouteConfig `yaml:"routes"`
	UpstreamPools []UpstreamPoolConfig `yaml:"upstream_pools"`
}


type PublicListenerConfig struct {
	Addr string `yaml:"addr"`
}
type AdminListenerConfig struct {
	Addr string `yaml:"addr"`
}
type ListenersConfig struct {
	Public PublicListenerConfig
	Admin AdminListenerConfig
}


type LogsConfig struct {
	Level string `yaml:"level"`
}
type MetricsConfig struct {
	Enabled bool `yaml:"enabled"`
}
type ObservabilityConfig struct {
	Logs LogsConfig
	Metrics MetricsConfig
}


type TimeoutsConfig struct {
	Request time.Duration `yaml:"request"`
	UpstreamResponseHeader time.Duration `yaml:"upstream_response_header"`
}
type DefaultsConfig struct {
	Timeouts TimeoutsConfig
	BodyLimit ByteSize `yaml:"body_limit"`
}


type ShutdownConfig struct {
	Timeout time.Duration `yaml:"timeout"`
}


type RateLimitConfig struct {
	Capacity int64 `yaml:"capacity"`
	RefillRatePerSec int64 `yaml:"refill_rate_per_sec"`
	KeyPrefix string `yaml:"key_prefix"`
}
type RedisConfig struct {
	Addr string `yaml:"addr"`
	Password string `yaml:"password"`
	DB int `yaml:"db"`
}


type RouteConfig struct {
	ID string `yaml:"id"`
	Host string `yaml:"host"`
	PathPrefix string `yaml:"path_prefix"`
	UpstreamPool string `yaml:"upstream_pool"`
}


type UpstreamPoolConfig struct {
	ID string `yaml:"id"`
	Targets []string `yaml:"targets"`
}

type ByteSize int64
var ErrInvalidByteSize = errors.New("invalid size")
var ErrInvalidByteUnit = errors.New("invalid unit")

var sizeRe = regexp.MustCompile(`^\s*([0-9]+)\s*([A-Za-z]+)?\s*$`)
var multipliers = map[string]int64{
	"B": 1,
	"KB": 1024,
	"MB": 1024 * 1024,
	"GB": 1024 * 1024 * 1024,
	"TB": 1024 * 1024 * 1024 * 1024,
}

func ParseByteSize(s string) (ByteSize, error) {
	s = strings.TrimSpace(s)
	
	m := sizeRe.FindStringSubmatch(s)
	if m == nil {
		return ByteSize(0), fmt.Errorf("invalid size %q", s)
	}
	numPart := m[1]
	unitPart := m[2]
	unitPart = strings.ToUpper(strings.TrimSpace(unitPart))
	unit := normalizeUnit(unitPart)

	num, err := strconv.ParseInt(numPart, 10, 64)
	if err != nil {
		return ByteSize(0), err
	}
	if num <= 0 {
		return ByteSize(0), fmt.Errorf("%w: %q", ErrInvalidByteSize, s)
	}

	multiplier, ok := multipliers[unit]
	if !ok {
		return ByteSize(0), fmt.Errorf("%w: %q",ErrInvalidByteUnit, unit)
	}
	
	num, err = mulChecked(num, multiplier)
	if err != nil {
		return ByteSize(0), err
	}
	
	return ByteSize(num), nil
}

func mulChecked(n int64, mult int64) (int64, error) {
    if n <= 0 || mult <= 0 {
        return 0, fmt.Errorf("invalid operands")
    }
    if n > math.MaxInt64/mult {
        return 0, fmt.Errorf("size overflow: %d * %d", n, mult)
    }
    return n * mult, nil
}

func normalizeUnit(unit string) string {
	switch unit {
	case "", "B":
		return "B"
	case "K", "KB":
		return "KB"
	case "M", "MB":
		return "MB"
	case "G", "GB":
		return "GB"
	case "T", "TB":
		return "TB"
	default:
		return unit
	}
}

func (b *ByteSize) UnmarshalText(data []byte) error {
	size, err := ParseByteSize(string(data))
	if err != nil {
		return err
	}
	*b = size
	return nil
}

