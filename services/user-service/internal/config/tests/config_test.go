package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Damirka228/PerkBack/services/user-service/internal/config"
)

const testDSN = "postgres://test:test@127.0.0.1:5433/user_service?sslmode=disable"

func TestLoadLocalConfig(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate local config")
	}
	cfg, err := config.Load(filepath.Join(filepath.Dir(source), "../../../configs/local.yaml"))
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	want := config.Config{
		App:             config.AppConfig{Name: "user-service", Env: "local"},
		GRPC:            config.GRPCConfig{Address: ":9094"},
		HTTP:            config.HTTPConfig{Address: ":8083"},
		DB:              config.DB{ConnectTimeout: 5 * time.Second},
		Log:             config.LogConfig{Level: "debug"},
		Timeout:         10 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("loaded config = %+v, want %+v", cfg, want)
	}

	// Loading YAML and applying environment variables are separate steps,
	// matching the other services. No database connection is made here.
	t.Setenv("USER_DATABASE_URL", testDSN)
	config.ApplyEnv(&cfg)
	if cfg.DB.DSN != testDSN {
		t.Fatal("ApplyEnv did not set the database DSN")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("local config with database env must be valid: %v", err)
	}
}

func TestLoadRejectsInvalidYAML(t *testing.T) {
	for _, tt := range []struct {
		name string
		yaml string
	}{
		{"malformed", "app: ["},
		{"unknown_top_level_field", "unknown: true\n"},
		{"unknown_nested_field", "database:\n  connect_timout: 5s\n"},
		{"invalid_duration", "database:\n  connect_timeout: tomorrow\n"},
		{"dsn_not_allowed_in_yaml", "database:\n  dsn: postgres://test\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := config.Load(path); err == nil {
				t.Fatal("Load() must reject invalid YAML")
			}
		})
	}
}

func TestLoadPreservesFileError(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load() = %v, want wrapped os.ErrNotExist", err)
	}
}

func TestApplyEnvOverridesOnlyDSN(t *testing.T) {
	for _, tt := range []struct {
		name string
		dsn  string
	}{
		{"non_empty", testDSN},
		{"empty", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			want := cfg
			want.DB.DSN = tt.dsn
			t.Setenv("USER_DATABASE_URL", tt.dsn)
			config.ApplyEnv(&cfg)
			if !reflect.DeepEqual(cfg, want) {
				t.Fatal("ApplyEnv must override only DB.DSN, including an explicitly empty value")
			}
			if tt.dsn == "" && cfg.Validate() == nil {
				t.Fatal("an explicitly empty database URL must fail validation")
			}
		})
	}
}

func TestApplyEnvMissingPreservesConfig(t *testing.T) {
	t.Setenv("USER_DATABASE_URL", "temporary")
	if err := os.Unsetenv("USER_DATABASE_URL"); err != nil {
		t.Fatal(err)
	}
	cfg := validConfig()
	want := cfg
	config.ApplyEnv(&cfg)
	if !reflect.DeepEqual(cfg, want) {
		t.Fatal("ApplyEnv modified config when USER_DATABASE_URL was absent")
	}
}

func TestConfigValidate(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	for _, tt := range []struct {
		name    string
		change  func(*config.Config)
		message string
	}{
		{"empty_app", func(c *config.Config) { c.App.Name = "" }, "app name"},
		{"blank_app", func(c *config.Config) { c.App.Name = " \t" }, "app name"},
		{"empty_grpc", func(c *config.Config) { c.GRPC.Address = "" }, "grpc address"},
		{"blank_grpc", func(c *config.Config) { c.GRPC.Address = " \t" }, "grpc address"},
		{"empty_http", func(c *config.Config) { c.HTTP.Address = "" }, "http address"},
		{"blank_http", func(c *config.Config) { c.HTTP.Address = " \t" }, "http address"},
		{"empty_dsn", func(c *config.Config) { c.DB.DSN = "" }, "USER_DATABASE_URL"},
		{"blank_dsn", func(c *config.Config) { c.DB.DSN = " \t\n" }, "USER_DATABASE_URL"},
		{"zero_connect_timeout", func(c *config.Config) { c.DB.ConnectTimeout = 0 }, "database connect timeout"},
		{"negative_connect_timeout", func(c *config.Config) { c.DB.ConnectTimeout = -time.Second }, "database connect timeout"},
		{"zero_timeout", func(c *config.Config) { c.Timeout = 0 }, "timeout"},
		{"negative_timeout", func(c *config.Config) { c.Timeout = -time.Second }, "timeout"},
		{"zero_shutdown_timeout", func(c *config.Config) { c.ShutdownTimeout = 0 }, "shutdown timeout"},
		{"negative_shutdown_timeout", func(c *config.Config) { c.ShutdownTimeout = -time.Second }, "shutdown timeout"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.change(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("Validate() = %v, want an error about %q", err, tt.message)
			}
		})
	}
}

func validConfig() config.Config {
	return config.Config{
		App:             config.AppConfig{Name: "user-service", Env: "local"},
		GRPC:            config.GRPCConfig{Address: ":9094"},
		HTTP:            config.HTTPConfig{Address: ":8083"},
		DB:              config.DB{DSN: testDSN, ConnectTimeout: 5 * time.Second},
		Log:             config.LogConfig{Level: "debug"},
		Timeout:         10 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}
}
