package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
)

var logLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

var (
	ErrInvalidListenersConfig = errors.New("Invalid listeners config")
	ErrInvalidObservabilityConfig = errors.New("Invalid observability config")
	ErrInvalidDefaultsConfig = errors.New("Invalid defaults config")
	ErrInvalidShutdownConfig = errors.New("Invalid shutdown config")
	ErrInvalidRoutesConfig = errors.New("Invalid routes config")
)

func Validate(c *Config) []error {
	errors := []error{}

	_, portStr, err := net.SplitHostPort(c.Listeners.Public.Addr)
	if err != nil {
		errors = append(errors, fmt.Errorf("%w, invalid public listener address: %q, err: %v", ErrInvalidListenersConfig, c.Listeners.Public.Addr, err))
	}
	if portStr == "" {
		errors = append(errors, fmt.Errorf("%w, invalid public listener address %q", ErrInvalidListenersConfig, c.Listeners.Public.Addr))
	}
	pubPort, err := strconv.Atoi(portStr)
	if err != nil || pubPort < 1 || pubPort > 65535 {
		errors = append(errors, fmt.Errorf("%w, public listener should be between 1 and 65535: %d", ErrInvalidListenersConfig, pubPort))
	}

	_, portStr, err = net.SplitHostPort(c.Listeners.Admin.Addr)
	if err != nil {
		errors = append(errors, fmt.Errorf("%w, invalid admin listener address: %q, err: %v", ErrInvalidListenersConfig, c.Listeners.Admin.Addr, err))
	}
	if portStr == "" {
		errors = append(errors, fmt.Errorf("%w, invalid admin listener address: %q", ErrInvalidListenersConfig, c.Listeners.Admin.Addr))
	}
	admPort, err := strconv.Atoi(portStr)
	if err != nil || admPort < 1 || admPort > 65535 {
		errors = append(errors, fmt.Errorf("%w, admin listener should be between 1 and 65535: %d", ErrInvalidListenersConfig, admPort))
	}

	if pubPort == admPort {
		errors = append(errors, fmt.Errorf("%w, public and admin listener should be different: %d / %d", ErrInvalidListenersConfig, pubPort, admPort))
	}

	if _, ok := logLevels[c.Observability.Logs.Level]; !ok {
		errors = append(errors, fmt.Errorf("%w, invalid log level value %q", ErrInvalidObservabilityConfig, c.Observability.Logs.Level))
	}

	if c.Defaults.Timeouts.Request <= 0 {
		errors = append(errors, fmt.Errorf("%w, invalid request timeout value %q", ErrInvalidDefaultsConfig, c.Defaults.Timeouts.Request))
	}
	if c.Defaults.Timeouts.UpstreamResponseHeader <= 0 {
		errors = append(errors, fmt.Errorf("%w, invalid upstream response header timeout value %q", ErrInvalidDefaultsConfig, c.Defaults.Timeouts.UpstreamResponseHeader))
	}

	if c.Defaults.BodyLimit <= 0 {
		errors = append(errors, fmt.Errorf("%w, invalid body limit value %q", ErrInvalidDefaultsConfig, c.Defaults.BodyLimit))
	}

	if c.Shutdown.Timeout <= 0 {
		errors = append(errors, fmt.Errorf("%w, invalid shutdown timeout value %q", ErrInvalidShutdownConfig, c.Shutdown.Timeout))
	}

	if c.Routes == nil || len(c.Routes) == 0 {
		errors = append(errors, fmt.Errorf("%w, no routes defined", ErrInvalidRoutesConfig))
	}
	for _, r := range c.Routes {
		if r.ID == "" {
			errors = append(errors, fmt.Errorf("%w, invalid route id %q", ErrInvalidRoutesConfig, r.ID))
		}
		if r.Host == "" {
			errors = append(errors, fmt.Errorf("%w, invalid route host %q", ErrInvalidRoutesConfig, r.Host))
		}
		if r.PathPrefix == "" {
			errors = append(errors, fmt.Errorf("%w, invalid route path prefix %q", ErrInvalidRoutesConfig, r.PathPrefix))
		}
		if r.UpstreamPool == "" {
			errors = append(errors, fmt.Errorf("%w, invalid route upstream pool %q", ErrInvalidRoutesConfig, r.UpstreamPool))
		}
	}
	return errors
}
