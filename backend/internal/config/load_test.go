package config

import (
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
		want string
	} {
		{
			name: "invalid yaml",
			yaml: "invalid yaml",
			want: "yaml:",	
		},
		{
			name: "invalid log level",
			yaml: strings.ReplaceAll(validYAML, "info", "invalid"),
			want: "Invalid log level value",
		},
		{
			name: "invalid public listener port string",
			yaml: strings.ReplaceAll(validYAML, ":8080", "invalid"),
			want: "Invalid public listener address",
		},
		{
			name: "invalid public listener port number",
			yaml: strings.ReplaceAll(validYAML, ":8080", ":-1"),
			want: "Public listener should be between 1 and 65535",
		},
		{
			name: "invalid admin listener port string",
			yaml: strings.ReplaceAll(validYAML, ":9090", "invalid"),
			want: "Invalid admin listener address",
		},
		{
			name: "invalid admin listener port number",
			yaml: strings.ReplaceAll(validYAML, ":9090", ":65536"),
			want: "Admin listener should be between 1 and 65535",
		},
		{
			name: "same admin and public listener ports",
			yaml: strings.ReplaceAll(validYAML, ":9090", ":8080"),
			want: "Public and admin listener should be different",
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

			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error to contain %q, got %q", tt.want, err.Error())
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