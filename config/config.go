package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App          AppConfig          `mapstructure:"app"`
	Monitor      MonitorConfig      `mapstructure:"monitor"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Notification NotificationConfig `mapstructure:"notification"`
}

type AppConfig struct {
	Port           int      `mapstructure:"port"`
	AdminToken     string   `mapstructure:"admin_token"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

type MonitorConfig struct {
	C2CIntervalMinutes    int       `mapstructure:"c2c_interval_minutes" json:"c2c_interval_minutes"`
	ForexIntervalHours    int       `mapstructure:"forex_interval_hours" json:"forex_interval_hours"`
	ForexMaxAgeHours      int       `mapstructure:"forex_max_age_hours" json:"forex_max_age_hours"`
	AlertThresholdPercent float64   `mapstructure:"alert_threshold_percent" json:"alert_threshold_percent"`
	TargetAmounts         []float64 `mapstructure:"target_amounts" json:"target_amounts"`
	Exchanges             []string  `mapstructure:"exchanges" json:"exchanges"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}

type NotificationConfig struct {
	Email EmailConfig `mapstructure:"email"`
}

type EmailConfig struct {
	SMTPHost string   `mapstructure:"smtp_host"`
	SMTPPort int      `mapstructure:"smtp_port"`
	Username string   `mapstructure:"username"`
	Password string   `mapstructure:"password"`
	From     string   `mapstructure:"from"`
	To       []string `mapstructure:"to"`
}

func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.SetDefault("app.port", 8080)
	v.SetDefault("app.admin_token", "")
	v.SetDefault("app.allowed_origins", []string{"http://localhost:8080", "http://127.0.0.1:8080"})
	v.SetDefault("monitor.c2c_interval_minutes", 3)
	v.SetDefault("monitor.forex_interval_hours", 1)
	v.SetDefault("monitor.forex_max_age_hours", 6)
	v.SetDefault("monitor.alert_threshold_percent", 0.1)
	v.SetDefault("monitor.target_amounts", []float64{0, 30, 50, 200, 500, 1000})
	v.SetDefault("monitor.exchanges", []string{"Binance", "Gate", "OKX"})

	// Environment variable support
	v.SetEnvPrefix("C2C")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	for _, key := range []string{
		"app.admin_token",
		"database.dsn",
		"notification.email.smtp_host",
		"notification.email.smtp_port",
		"notification.email.username",
		"notification.email.password",
		"notification.email.from",
	} {
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("bind environment variable for %s: %w", key, err)
		}
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := NormalizeAndValidate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
