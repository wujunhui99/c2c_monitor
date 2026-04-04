package config

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"c2c_monitor/internal/domain"
)

func NormalizeAndValidate(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	if cfg.App.Port <= 0 {
		return fmt.Errorf("app.port must be > 0")
	}

	monitorCfg, err := NormalizeMonitorConfig(cfg.Monitor)
	if err != nil {
		return err
	}
	cfg.Monitor = monitorCfg

	if strings.TrimSpace(cfg.Database.DSN) == "" {
		return fmt.Errorf("database.dsn must not be empty")
	}

	cfg.Notification.Email.To = trimNonEmptyStrings(cfg.Notification.Email.To)
	return nil
}

func NormalizeMonitorConfig(cfg MonitorConfig) (MonitorConfig, error) {
	if cfg.C2CIntervalMinutes <= 0 {
		return cfg, fmt.Errorf("monitor.c2c_interval_minutes must be > 0")
	}
	if cfg.ForexIntervalHours <= 0 {
		return cfg, fmt.Errorf("monitor.forex_interval_hours must be > 0")
	}
	if cfg.AlertThresholdPercent < 0 {
		return cfg, fmt.Errorf("monitor.alert_threshold_percent must be >= 0")
	}
	if len(cfg.TargetAmounts) == 0 {
		return cfg, fmt.Errorf("monitor.target_amounts must not be empty")
	}
	if len(cfg.Exchanges) == 0 {
		return cfg, fmt.Errorf("monitor.exchanges must not be empty")
	}

	normalizedAmounts := make([]float64, 0, len(cfg.TargetAmounts))
	seenAmounts := make(map[float64]struct{}, len(cfg.TargetAmounts))
	for _, amount := range cfg.TargetAmounts {
		if amount < 0 {
			return cfg, fmt.Errorf("monitor.target_amounts must be >= 0, got %.4f", amount)
		}
		if _, exists := seenAmounts[amount]; exists {
			continue
		}
		seenAmounts[amount] = struct{}{}
		normalizedAmounts = append(normalizedAmounts, amount)
	}
	sort.Float64s(normalizedAmounts)
	cfg.TargetAmounts = normalizedAmounts

	normalizedExchanges, err := domain.NormalizeExchangeNames(cfg.Exchanges)
	if err != nil {
		return cfg, err
	}
	cfg.Exchanges = normalizedExchanges

	return cfg, nil
}

func trimNonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}
