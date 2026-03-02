package config

import (
	"fmt"
)

var logLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

func Validate(c *Config) []error {
	errors := []error{}
	if c.Listeners.Public.Addr == "" {
		errors = append(errors, fmt.Errorf("Invalid public listener address %q", c.Listeners.Public.Addr))
	}
	if c.Listeners.Admin.Addr == "" {
		errors = append(errors, fmt.Errorf("Invalid admin listener address %q", c.Listeners.Admin.Addr))
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
