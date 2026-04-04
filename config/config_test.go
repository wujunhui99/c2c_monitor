package config

import (
	"reflect"
	"testing"

	"c2c_monitor/internal/domain"
)

func TestNormalizeMonitorConfig(t *testing.T) {
	cfg := MonitorConfig{
		C2CIntervalMinutes:    3,
		ForexIntervalHours:    1,
		AlertThresholdPercent: 0.1,
		TargetAmounts:         []float64{500, 0, 30, 500},
		Exchanges:             []string{"okx", "binance", "OKX"},
	}

	got, err := NormalizeMonitorConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !reflect.DeepEqual(got.TargetAmounts, []float64{0, 30, 500}) {
		t.Fatalf("expected sorted unique target amounts, got %v", got.TargetAmounts)
	}

	if !reflect.DeepEqual(got.Exchanges, []string{domain.ExchangeOKX, domain.ExchangeBinance}) {
		t.Fatalf("expected normalized exchanges, got %v", got.Exchanges)
	}
}

func TestNormalizeMonitorConfigRejectsUnsupportedExchange(t *testing.T) {
	cfg := MonitorConfig{
		C2CIntervalMinutes:    3,
		ForexIntervalHours:    1,
		AlertThresholdPercent: 0.1,
		TargetAmounts:         []float64{0, 30},
		Exchanges:             []string{"binance", "kraken"},
	}

	if _, err := NormalizeMonitorConfig(cfg); err == nil {
		t.Fatal("expected validation error")
	}
}
