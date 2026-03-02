package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)


func Load(p string) (Config, error) {
	bytes, err := os.ReadFile(p)
	if err != nil {
		return Config{}, err
	}
	str, err := substitutePlaceholders(string(bytes))
	if err != nil {
		return Config{}, err
	}
	cfg := defaultConfig()
	err = yaml.Unmarshal([]byte(str), &cfg)
	if err != nil {
		return Config{}, err
	}
	errors := Validate(&cfg)
	if errors != nil && len(errors) > 0 {
		return Config{}, fmt.Errorf("Errors: %v", errors)
	}
	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		Listeners: ListenersConfig{
			Public: struct {
				Addr string `yaml:"addr"`
			}{
				Addr: ":8080",
			},
			Admin: struct {
				Addr string `yaml:"addr"`
			}{
				Addr: ":9090",
			},
		},
		Observability: ObservabilityConfig{
			Logs: struct {
				Level string `yaml:"level"`
			}{
				Level: "info",
			},
			Metrics: struct {
				Enabled bool `yaml:"enabled"`
			}{
				Enabled: false,
			},
		},
		Defaults: DefaultsConfig{
			Timeouts: struct {
				Request time.Duration `yaml:"request"`
				UpstreamResponseHeader time.Duration `yaml:"upstream_response_header"`
			}{
				Request: 30 * time.Second,
				UpstreamResponseHeader: 30 * time.Second,
			},
			BodyLimit: 1 * 1024 * 1024,
		},
		Shutdown: ShutdownConfig{
			Timeout: 5 * time.Second,
		},
	}
}

func substitutePlaceholders(data string) (string, error) {
	bytes, err := os.ReadFile(".env")
	if err != nil {
		bytes = []byte("")
	}
	envVars, err := getEnvVariables(string(bytes))
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)
	err = nil
	data = re.ReplaceAllStringFunc(data, func(s string) string {
		s = strings.TrimSpace(s)
		s = s[2 : len(s)-1]
		parts := strings.SplitN(s, ":-", 2)
		hasDefault := len(parts) == 2
		val, ok := envVars[parts[0]]
		if ok {
			return val
		}
		if hasDefault {
			return parts[1]
		}
		err = fmt.Errorf("undefined environment variable %q", parts[0])
		return ""
	})
	if err != nil {
		return "", err
	}
	return data, nil
}

func getEnvVariables(data string) (map[string]string, error) {
	vars := map[string]string{}
	for str := range strings.Lines(data) {
		if strings.Contains(str, "=") {
			parts := strings.SplitN(str, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key == "" || value == "" {
				continue
			}
			vars[key] = value
		}
	}
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			continue
		}
		vars[key] = value
	}
	return vars, nil
	
}
