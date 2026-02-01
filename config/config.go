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
	Port int `mapstructure:"port"`
}

type MonitorConfig struct {
	C2CIntervalMinutes    int       `mapstructure:"c2c_interval_minutes"`
	ForexIntervalHours    int       `mapstructure:"forex_interval_hours"`
	AlertThresholdPercent float64   `mapstructure:"alert_threshold_percent"`
	TargetAmounts         []float64 `mapstructure:"target_amounts"`
	Exchanges             []string  `mapstructure:"exchanges"`
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
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	viper.SetDefault("app.port", 8080)
	viper.SetDefault("monitor.c2c_interval_minutes", 3)
	viper.SetDefault("monitor.forex_interval_hours", 1)
	viper.SetDefault("monitor.target_amounts", []float64{0, 30, 50, 200, 500, 1000})
	
	// Environment variable support
	viper.SetEnvPrefix("C2C")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}
