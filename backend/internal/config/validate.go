package config

import (
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

func Validate(c *Config) []error {
	errors := []error{}

	_, portStr, err := net.SplitHostPort(c.Listeners.Public.Addr)
	if err != nil {
		errors = append(errors, fmt.Errorf("Invalid public listener address %q, err: %v", c.Listeners.Public.Addr, err))
	}
	if portStr == "" {
		errors = append(errors, fmt.Errorf("Invalid public listener address %q", c.Listeners.Public.Addr))
	}
	pubPort, err := strconv.Atoi(portStr)
	if err != nil || pubPort < 1 || pubPort > 65535 {
		errors = append(errors, fmt.Errorf("Public listener should be between 1 and 65535: %d", pubPort))
	}

	_, portStr, err = net.SplitHostPort(c.Listeners.Admin.Addr)
	if err != nil {
		errors = append(errors, fmt.Errorf("Invalid admin listener address %q, err: %v", c.Listeners.Public.Addr, err))
	}
	if portStr == "" {
		errors = append(errors, fmt.Errorf("Invalid admin listener address %q", c.Listeners.Public.Addr))
	}
	admPort, err := strconv.Atoi(portStr)
	if err != nil || admPort < 1 || admPort > 65535 {
		errors = append(errors, fmt.Errorf("Admin listener should be between 1 and 65535: %d", admPort))
	}

	if pubPort == admPort {
		errors = append(errors, fmt.Errorf("Public and admin listener should be different: %d / %d", pubPort, admPort))
	}

	if _, ok := logLevels[c.Observability.Logs.Level]; !ok {
		errors = append(errors, fmt.Errorf("Invalid log level value %q", c.Observability.Logs.Level))
	}

	if c.Defaults.Timeouts.Request <= 0 {
		errors = append(errors, fmt.Errorf("Invalid request timeout value %q", c.Defaults.Timeouts.Request))
	}
	if c.Defaults.Timeouts.UpstreamResponseHeader <= 0 {
		errors = append(errors, fmt.Errorf("Invalid upstream response header timeout value %q", c.Defaults.Timeouts.UpstreamResponseHeader))
	}

	if c.Defaults.BodyLimit <= 0 {
		errors = append(errors, fmt.Errorf("Invalid body limit value %q", c.Defaults.BodyLimit))
	}

	if c.Shutdown.Timeout <= 0 {
		errors = append(errors, fmt.Errorf("Invalid shutdown timeout value %q", c.Shutdown.Timeout))
	}
	return errors
}
