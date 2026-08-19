package config

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"

	"c2c_monitor/internal/domain"
)

func NormalizeAndValidate(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	if cfg.App.Port <= 0 || cfg.App.Port > 65535 {
		return fmt.Errorf("app.port must be between 1 and 65535")
	}

	cfg.App.AdminToken = strings.TrimSpace(cfg.App.AdminToken)
	if len(cfg.App.AdminToken) < 16 {
		return fmt.Errorf("app.admin_token must be at least 16 characters")
	}

	allowedOrigins, err := normalizeAllowedOrigins(cfg.App.AllowedOrigins)
	if err != nil {
		return err
	}
	cfg.App.AllowedOrigins = allowedOrigins

	monitorCfg, err := NormalizeMonitorConfig(cfg.Monitor)
	if err != nil {
		return err
	}
	cfg.Monitor = monitorCfg

	if strings.TrimSpace(cfg.Database.DSN) == "" {
		return fmt.Errorf("database.dsn must not be empty")
	}

	email := &cfg.Notification.Email
	email.SMTPHost = strings.TrimSpace(email.SMTPHost)
	email.Username = strings.TrimSpace(email.Username)
	email.Password = strings.TrimSpace(email.Password)
	email.From = strings.TrimSpace(email.From)
	if email.SMTPHost == "" {
		return fmt.Errorf("notification.email.smtp_host must not be empty")
	}
	if email.SMTPPort <= 0 || email.SMTPPort > 65535 {
		return fmt.Errorf("notification.email.smtp_port must be between 1 and 65535")
	}
	if email.Username == "" || email.Password == "" {
		return fmt.Errorf("notification.email username and password must not be empty")
	}
	if email.From == "" {
		return fmt.Errorf("notification.email.from must not be empty")
	}
	cfg.Notification.Email.To = trimNonEmptyStrings(cfg.Notification.Email.To)
	if len(cfg.Notification.Email.To) == 0 {
		return fmt.Errorf("notification.email.to must not be empty")
	}
	return nil
}

func NormalizeMonitorConfig(cfg MonitorConfig) (MonitorConfig, error) {
	if cfg.C2CIntervalMinutes <= 0 {
		return cfg, fmt.Errorf("monitor.c2c_interval_minutes must be > 0")
	}
	if cfg.ForexIntervalHours <= 0 {
		return cfg, fmt.Errorf("monitor.forex_interval_hours must be > 0")
	}
	if cfg.ForexMaxAgeHours <= 0 {
		return cfg, fmt.Errorf("monitor.forex_max_age_hours must be > 0")
	}
	if math.IsNaN(cfg.AlertThresholdPercent) || math.IsInf(cfg.AlertThresholdPercent, 0) || cfg.AlertThresholdPercent < 0 {
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
		if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
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

func normalizeAllowedOrigins(values []string) ([]string, error) {
	values = trimNonEmptyStrings(values)
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		parsed, err := url.Parse(value)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, fmt.Errorf("app.allowed_origins contains invalid origin %q", value)
		}
		if parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("app.allowed_origins must contain origins only, got %q", value)
		}

		origin := parsed.Scheme + "://" + parsed.Host
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		result = append(result, origin)
	}

	return result, nil
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
