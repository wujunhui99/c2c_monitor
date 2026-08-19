package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"c2c_monitor/config"
	"c2c_monitor/internal/api"
	"c2c_monitor/internal/appmeta"
	"c2c_monitor/internal/domain"
	"c2c_monitor/internal/infrastructure/exchange"
	"c2c_monitor/internal/infrastructure/forex"
	"c2c_monitor/internal/infrastructure/notifier"
	mysqlrepo "c2c_monitor/internal/infrastructure/persistence/mysql"
	"c2c_monitor/internal/logging"
	"c2c_monitor/internal/service"
	gmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	logging.Configure()

	configPath := flag.String("config", defaultConfigPath(), "path to config yaml")
	flag.Parse()

	if err := appmeta.ValidateCatalog(); err != nil {
		slog.Error("failed to validate release catalog", "event", "release_catalog_invalid", "error", err)
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		slog.Error("failed to load config", "event", "config_load_failed", "path", *configPath, "error", err)
		os.Exit(1)
	}

	db, err := gorm.Open(gmysql.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		slog.Error("failed to connect database", "event", "database_connect_failed", "error", err)
		os.Exit(1)
	}

	repo := mysqlrepo.NewMySQLRepository(db)
	if err := repo.RunMigrations(context.Background()); err != nil {
		slog.Error("failed to run database migrations", "event", "database_migration_failed", "error", err)
		os.Exit(1)
	}

	exchanges := map[string]domain.IExchange{
		domain.ExchangeBinance: exchange.NewBinanceAdapter(),
		domain.ExchangeGate:    exchange.NewGateAdapter(),
		domain.ExchangeOKX:     exchange.NewOKXAdapter(),
	}

	svc := service.NewMonitorService(
		cfg.Monitor,
		repo,
		exchanges,
		forex.NewFallbackAdapter(
			forex.NewOpenERAdapter(),
			forex.NewHexaRateAdapter(),
		),
		notifier.NewSMTPNotifier(
			cfg.Notification.Email.SMTPHost,
			cfg.Notification.Email.SMTPPort,
			cfg.Notification.Email.Username,
			cfg.Notification.Email.Password,
			cfg.Notification.Email.From,
			cfg.Notification.Email.To,
		),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go svc.Start(ctx)

	router := api.SetupRouter(svc, cfg)
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.App.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "event", "http_shutdown_failed", "error", err)
		}
	}()

	slog.Info("starting c2c monitor",
		"event", "app_starting",
		"version", appmeta.Version,
		"addr", server.Addr,
		"exchanges", cfg.Monitor.Exchanges,
	)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("http server exited unexpectedly", "event", "http_server_failed", "error", err)
		os.Exit(1)
	}
}

func defaultConfigPath() string {
	if path := strings.TrimSpace(os.Getenv("C2C_CONFIG")); path != "" {
		return path
	}
	return "config/config.yaml"
}
