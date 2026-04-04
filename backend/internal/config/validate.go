package config

import (
	"errors"
	"fmt"
	"net"
	url "net/url"
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
	ErrInvalidUpstreamPoolsConfig = errors.New("Invalid upstream pools config")
)

func Validate(c *Config) []error {
	errs := []error{}

	_, portStr, err := net.SplitHostPort(c.Listeners.Public.Addr)
	if err != nil {
		errs = append(errs, fmt.Errorf("%w, invalid public listener address: %q, err: %v", ErrInvalidListenersConfig, c.Listeners.Public.Addr, err))
	}
	if portStr == "" {
		errs = append(errs, fmt.Errorf("%w, invalid public listener address %q", ErrInvalidListenersConfig, c.Listeners.Public.Addr))
	}
	pubPort, err := strconv.Atoi(portStr)
	if err != nil || pubPort < 1 || pubPort > 65535 {
		errs = append(errs, fmt.Errorf("%w, public listener should be between 1 and 65535: %d, err: %v", ErrInvalidListenersConfig, pubPort, err))
	}

	_, portStr, err = net.SplitHostPort(c.Listeners.Admin.Addr)
	if err != nil {
		errs = append(errs, fmt.Errorf("%w, invalid admin listener address: %q, err: %v", ErrInvalidListenersConfig, c.Listeners.Admin.Addr, err))
	}
	if portStr == "" {
		errs = append(errs, fmt.Errorf("%w, invalid admin listener address: %q", ErrInvalidListenersConfig, c.Listeners.Admin.Addr))
	}
	admPort, err := strconv.Atoi(portStr)
	if err != nil || admPort < 1 || admPort > 65535 {
		errs = append(errs, fmt.Errorf("%w, admin listener should be between 1 and 65535: %d, err: %v", ErrInvalidListenersConfig, admPort, err))
	}

	if pubPort == admPort {
		errs = append(errs, fmt.Errorf("%w, public and admin listener should be different: %d / %d", ErrInvalidListenersConfig, pubPort, admPort))
	}

	if _, ok := logLevels[c.Observability.Logs.Level]; !ok {
		errs = append(errs, fmt.Errorf("%w, invalid log level value %q", ErrInvalidObservabilityConfig, c.Observability.Logs.Level))
	}

	if c.Defaults.Timeouts.Request <= 0 {
		errs = append(errs, fmt.Errorf("%w, invalid request timeout value %q", ErrInvalidDefaultsConfig, c.Defaults.Timeouts.Request))
	}
	if c.Defaults.Timeouts.UpstreamResponseHeader <= 0 {
		errs = append(errs, fmt.Errorf("%w, invalid upstream response header timeout value %q", ErrInvalidDefaultsConfig, c.Defaults.Timeouts.UpstreamResponseHeader))
	}

	if c.Defaults.BodyLimit <= 0 {
		errs = append(errs, fmt.Errorf("%w, invalid body limit value %q", ErrInvalidDefaultsConfig, c.Defaults.BodyLimit))
	}

	if c.Shutdown.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("%w, invalid shutdown timeout value %q", ErrInvalidShutdownConfig, c.Shutdown.Timeout))
	}

	if c.Routes == nil || len(c.Routes) == 0 {
		errs = append(errs, fmt.Errorf("%w, no routes defined", ErrInvalidRoutesConfig))
	}
	for _, r := range c.Routes {
		if r.ID == "" {
			errs = append(errs, fmt.Errorf("%w, invalid route id %q", ErrInvalidRoutesConfig, r.ID))
		}
		if r.Host == "" {
			errs = append(errs, fmt.Errorf("%w, invalid route host %q", ErrInvalidRoutesConfig, r.Host))
		}
		if r.PathPrefix == "" {
			errs = append(errs, fmt.Errorf("%w, invalid route path prefix %q", ErrInvalidRoutesConfig, r.PathPrefix))
		}
		if r.UpstreamPool == "" {
			errs = append(errs, fmt.Errorf("%w, invalid route upstream pool %q", ErrInvalidRoutesConfig, r.UpstreamPool))
		}
	}

	if c.UpstreamPools == nil || len(c.UpstreamPools) == 0 {
		errs = append(errs, fmt.Errorf("%w, no upstream pools defined", ErrInvalidUpstreamPoolsConfig))
	}
	for _, p := range c.UpstreamPools {
		if p.ID == "" {
			errs = append(errs, fmt.Errorf("%w, upstream pool id cannot be empty %q", ErrInvalidUpstreamPoolsConfig, p.ID))
		}
		if p.Targets == nil || len(p.Targets) == 0 {
			errs = append(errs, fmt.Errorf("%w, upstream pool targets are required %q", ErrInvalidUpstreamPoolsConfig, p.Targets))
		}

		for _, t := range p.Targets {
			if t == "" {
				continue
			}
			u, err := url.Parse(t)
			if err != nil || u.Host == "" {
				errs = append(errs, fmt.Errorf("%w, invalid upstream pool target %q, err: %v", ErrInvalidUpstreamPoolsConfig, t, err))
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				errs = append(errs, fmt.Errorf("%w, invalid upstream pool target %q, scheme must be http or https", ErrInvalidUpstreamPoolsConfig, t))
			}
			if u.Path != "" {
				errs = append(errs, fmt.Errorf("%w, invalid upstream pool target %q, path must be empty and no trailing slash", ErrInvalidUpstreamPoolsConfig, t))
			}
			
		}
	}
	err = validateUpstreamPoolReferenceIntegrity(c)
	if err != nil {
		errs = append(errs, err)
	}
	return errs
}

func validateUpstreamPoolReferenceIntegrity(c *Config) error {
	upstreamPools := make(map[string]struct{}, len(c.UpstreamPools))
	for _, p := range c.UpstreamPools {
		upstreamPools[p.ID] = struct{}{}
	}

	for _, r := range c.Routes {
		if _, ok := upstreamPools[r.UpstreamPool]; !ok {
			return fmt.Errorf("%w, upstream pool %q is referenced in routes but not defined", ErrInvalidUpstreamPoolsConfig, r.UpstreamPool)
		}
	}
	return nil
}
