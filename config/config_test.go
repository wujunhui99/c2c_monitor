package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"c2c_monitor/internal/domain"
)

func TestNormalizeMonitorConfig(t *testing.T) {
	cfg := MonitorConfig{
		C2CIntervalMinutes: 3,
		ForexIntervalHours: 1,
		ForexMaxAgeHours:   6,
		TargetAmounts:      []float64{500, 0, 30, 500},
		Exchanges:          []string{"okx", "binance", "OKX"},
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
		C2CIntervalMinutes: 3,
		ForexIntervalHours: 1,
		ForexMaxAgeHours:   6,
		TargetAmounts:      []float64{0, 30},
		Exchanges:          []string{"binance", "kraken"},
	}

	if _, err := NormalizeMonitorConfig(cfg); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestNormalizeAndValidateAppSecurity(t *testing.T) {
	cfg := &Config{
		App: AppConfig{
			Port:           8001,
			AdminToken:     "0123456789abcdef",
			AllowedOrigins: []string{" https://example.com ", "https://example.com"},
		},
		Monitor: MonitorConfig{
			C2CIntervalMinutes: 3,
			ForexIntervalHours: 1,
			ForexMaxAgeHours:   6,
			TargetAmounts:      []float64{0, 30},
			Exchanges:          []string{"Binance"},
		},
		Database: DatabaseConfig{DSN: "test"},
		Notification: NotificationConfig{Email: EmailConfig{
			Enabled:  true,
			SMTPHost: "smtp.example.com",
			SMTPPort: 587,
			Username: "sender@example.com",
			Password: "secret",
			From:     "sender@example.com",
			To:       []string{"receiver@example.com"},
		}},
	}

	if err := NormalizeAndValidate(cfg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !reflect.DeepEqual(cfg.App.AllowedOrigins, []string{"https://example.com"}) {
		t.Fatalf("unexpected normalized origins: %v", cfg.App.AllowedOrigins)
	}
}

func TestNormalizeAndValidateRejectsWeakAdminToken(t *testing.T) {
	cfg := &Config{
		App: AppConfig{Port: 8001, AdminToken: "short"},
		Monitor: MonitorConfig{
			C2CIntervalMinutes: 3,
			ForexIntervalHours: 1,
			ForexMaxAgeHours:   6,
			TargetAmounts:      []float64{0},
			Exchanges:          []string{"Binance"},
		},
		Database: DatabaseConfig{DSN: "test"},
		Notification: NotificationConfig{Email: EmailConfig{
			Enabled:  true,
			SMTPHost: "smtp.example.com",
			SMTPPort: 587,
			Username: "sender@example.com",
			Password: "secret",
			From:     "sender@example.com",
			To:       []string{"receiver@example.com"},
		}},
	}

	if err := NormalizeAndValidate(cfg); err == nil {
		t.Fatal("expected weak admin token to be rejected")
	}
}

func TestNormalizeAndValidateAllowsDisabledEmail(t *testing.T) {
	cfg := &Config{
		App: AppConfig{
			Port:       8001,
			AdminToken: "0123456789abcdef",
		},
		Monitor: MonitorConfig{
			C2CIntervalMinutes: 3,
			ForexIntervalHours: 1,
			ForexMaxAgeHours:   6,
			TargetAmounts:      []float64{0},
			Exchanges:          []string{"Gate"},
		},
		Database: DatabaseConfig{DSN: "test"},
		Notification: NotificationConfig{Email: EmailConfig{
			Enabled: false,
		}},
	}

	if err := NormalizeAndValidate(cfg); err != nil {
		t.Fatalf("expected disabled email to allow empty SMTP settings, got %v", err)
	}
}

func TestLoadConfigUsesSecretEnvironmentOverrides(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	body := []byte(`
app:
  port: 8001
  admin_token: ""
  allowed_origins: []
monitor:
  c2c_interval_minutes: 3
  forex_interval_hours: 1
  forex_max_age_hours: 6
  target_amounts: [0]
  exchanges: ["Gate"]
database:
  dsn: ""
notification:
  email:
    smtp_host: ""
    smtp_port: 0
    username: ""
    password: ""
    from: ""
    to: ["receiver@example.com"]
`)
	if err := os.WriteFile(configPath, body, 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	t.Setenv("C2C_APP_ADMIN_TOKEN", "0123456789abcdef")
	t.Setenv("C2C_DATABASE_DSN", "user:password@tcp(mysql:3306)/c2c_monitor")
	t.Setenv("C2C_NOTIFICATION_EMAIL_SMTP_HOST", "smtp.example.com")
	t.Setenv("C2C_NOTIFICATION_EMAIL_SMTP_PORT", "587")
	t.Setenv("C2C_NOTIFICATION_EMAIL_USERNAME", "sender@example.com")
	t.Setenv("C2C_NOTIFICATION_EMAIL_PASSWORD", "smtp-password")
	t.Setenv("C2C_NOTIFICATION_EMAIL_FROM", "sender@example.com")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.App.AdminToken != "0123456789abcdef" {
		t.Fatalf("expected admin token override, got %q", cfg.App.AdminToken)
	}
	if cfg.Database.DSN != "user:password@tcp(mysql:3306)/c2c_monitor" {
		t.Fatalf("expected database DSN override, got %q", cfg.Database.DSN)
	}
	if cfg.Notification.Email.SMTPHost != "smtp.example.com" || cfg.Notification.Email.SMTPPort != 587 {
		t.Fatalf("unexpected SMTP address: %s:%d", cfg.Notification.Email.SMTPHost, cfg.Notification.Email.SMTPPort)
	}
	if cfg.Notification.Email.Username != "sender@example.com" ||
		cfg.Notification.Email.Password != "smtp-password" ||
		cfg.Notification.Email.From != "sender@example.com" {
		t.Fatalf("SMTP secret overrides were not applied: %#v", cfg.Notification.Email)
	}
}
