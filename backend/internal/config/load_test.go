package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)


var validYAML = 
`
listeners:
  public:
    addr: ":8080"
  admin:
    addr: ":9090"
observability:
  logs:
    level: "info"
  metrics:
    enabled: false
defaults:
  timeouts:
    request: 30s
    upstream_response_header: 30s
  body_limit: 1MB
shutdown:
  timeout: 5s

routes:
  - id: root
    host: api.example.com
    path_prefix: /
    upstream_pool: main-pool
`

func TestLoad_Success_WithValidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	err := os.WriteFile(cfgPath, []byte(validYAML), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Load(cfgPath)
	if err != nil {
		t.Fatalf("error loading config file: %v", err)
	}
}

func TestLoad_Fails_OnValidationError(t *testing.T) {
	t.Parallel()

	tests := []struct{
		name string
		yaml string
		wantErr error
		wantErrMessage string
	} {
		{
			name: "invalid yaml",
			yaml: "invalid yaml",
			wantErrMessage: "yaml: unmarshal error",	
		},
		{
			name: "invalid public listener port string",
			yaml: strings.ReplaceAll(validYAML, ":8080", "invalid"),
			wantErr: ErrInvalidListenersConfig,
		},
		{
			name: "invalid public listener port number",
			yaml: strings.ReplaceAll(validYAML, ":8080", ":-1"),
			wantErr: ErrInvalidListenersConfig,
		},
		{
			name: "invalid admin listener port string",
			yaml: strings.ReplaceAll(validYAML, ":9090", "invalid"),
			wantErr: ErrInvalidListenersConfig,
		},
		{
			name: "invalid admin listener port number",
			yaml: strings.ReplaceAll(validYAML, ":9090", ":65536"),
			wantErr: ErrInvalidListenersConfig,
		},
		{
			name: "same admin and public listener ports",
			yaml: strings.ReplaceAll(validYAML, ":9090", ":8080"),
			wantErr: ErrInvalidListenersConfig,
		},
		{
			name: "invalid log level",
			yaml: strings.ReplaceAll(validYAML, "info", "invalid"),
			wantErr: ErrInvalidObservabilityConfig,
		},
		{
			name: "invalid request timeout",
			yaml: strings.ReplaceAll(validYAML, "request: 30s", "request: -1s"),
			wantErr: ErrInvalidDefaultsConfig,
		},
		{
			name: "invalid upstream response header timeout",
			yaml: strings.ReplaceAll(validYAML, "upstream_response_header: 30s", "upstream_response_header: -1s"),
			wantErr: ErrInvalidDefaultsConfig,
		},
		{
			name: "invalid body limit value",
			yaml: strings.ReplaceAll(validYAML, "1MB", "0MB"),
			wantErr: ErrInvalidByteSize,
		},
		{
			name: "invalid body limit unit",
			yaml: strings.ReplaceAll(validYAML, "1MB", "1PT"),
			wantErr: ErrInvalidByteUnit,
		},
		{
			name: "invalid shutdown timeout",
			yaml: strings.ReplaceAll(validYAML, "timeout: 5s", "timeout: -1s"),
			wantErr: ErrInvalidShutdownConfig,
		},
		{
			name: "empty routes",
			yaml: strings.SplitN(validYAML, "routes:", 2)[0],
			wantErr: ErrInvalidRoutesConfig,
		},
		{
			name: "empty route id",
			yaml: strings.ReplaceAll(validYAML, "id: root", ""),
			wantErr: ErrInvalidRoutesConfig,
		},
		{
			name: "empty route host",
			yaml: strings.ReplaceAll(validYAML, "host: api.example.com", ""),
			wantErr: ErrInvalidRoutesConfig,
		},
		{
			name: "empty route path prefix",
			yaml: strings.ReplaceAll(validYAML, "path_prefix: /", ""),
			wantErr: ErrInvalidRoutesConfig,
		},
		{
			name: "empty route upstream pool",
			yaml: strings.ReplaceAll(validYAML, "upstream_pool: main-pool", ""),
			wantErr: ErrInvalidRoutesConfig,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfgPath := writeConfigFile(t, tt.yaml)

			_, err := Load(cfgPath)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if tt.wantErrMessage != "" && !strings.Contains(err.Error(), tt.wantErrMessage) {
				t.Fatalf("expected error to contain %q, got %q", tt.wantErrMessage, err.Error())
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) { 
				t.Fatalf("expected error to be %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestLoad_Missing_Env(t *testing.T) {
	t.Parallel()

	yaml := strings.ReplaceAll(validYAML, ":8080", "${MISSING_PUBLIC_ADDR}")
	cfgPath := writeConfigFile(t, yaml)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "undefined environment variable") {
		t.Fatalf("expected error to contain %q, got %q", "undefined environment variable", err.Error())
	}
}

func TestLoad_EnvDefault(t *testing.T) {
	t.Parallel()

	yaml := strings.ReplaceAll(validYAML, "8080", "${PUBLIC_ADDR:-50001}")
	cfgPath := writeConfigFile(t, yaml)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listeners.Public.Addr != ":50001" {
		t.Fatalf("expected %q, got %q", ":50001", cfg.Listeners.Public.Addr)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	yaml := strings.ReplaceAll(validYAML, ":8080", "${PUBLIC_ADDR:-50001}")
	t.Setenv("PUBLIC_ADDR", ":9091")
	cfgPath := writeConfigFile(t, yaml)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listeners.Public.Addr != ":9091" {
		t.Fatalf("expected %q, got %q", ":9091", cfg.Listeners.Public.Addr)
	}
}

func writeConfigFile(t *testing.T, yaml string) string {
	t.Helper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	err := os.WriteFile(cfgPath, []byte(yaml), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	return cfgPath
}