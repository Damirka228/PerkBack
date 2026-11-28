package config_test

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Damirka228/PerkBack/services/benefit-service/internal/config"
)

func loadLocalConfig(t *testing.T) config.Config {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate local config")
	}
	cfg, err := config.Load(filepath.Join(filepath.Dir(source), "../../../configs/local.yaml"))
	if err != nil {
		t.Fatalf("load local config: %v", err)
	}
	return cfg
}

func TestLocalConfigSelectedCategoryRate(t *testing.T) {
	cfg := loadLocalConfig(t)
	if cfg.SelectedCategoryRateBPS != 500 {
		t.Fatalf("selected category rate = %d, want 500 BPS", cfg.SelectedCategoryRateBPS)
	}
}

func TestSelectedCategoryRateValidation(t *testing.T) {
	for _, tt := range []struct {
		rate    int64
		wantErr bool
	}{
		{0, false},
		{500, false},
		{10000, false},
		{-1, true},
		{10001, true},
	} {
		t.Run(fmt.Sprintf("rate_%d", tt.rate), func(t *testing.T) {
			cfg := loadLocalConfig(t)
			// Supply a test-only DSN without reading .env or connecting to PostgreSQL.
			cfg.DB.DSN = "postgres://test:test@127.0.0.1:5433/benefit?sslmode=disable"
			cfg.SelectedCategoryRateBPS = tt.rate
			err := cfg.Validate()
			if !tt.wantErr && err != nil {
				t.Fatalf("valid rate %d rejected: %v", tt.rate, err)
			}
			if tt.wantErr && (err == nil || !strings.Contains(err.Error(), "selected_category_rate_bps")) {
				t.Fatalf("Validate() = %v, want an error about selected_category_rate_bps", err)
			}
		})
	}
}
