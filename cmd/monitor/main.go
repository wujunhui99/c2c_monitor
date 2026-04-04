package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
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
	"c2c_monitor/internal/service"
	gmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	configPath := flag.String("config", defaultConfigPath(), "path to config yaml")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := gorm.Open(gmysql.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	repo := mysqlrepo.NewMySQLRepository(db)
	if err := repo.AutoMigrate(); err != nil {
		log.Fatalf("failed to auto migrate database: %v", err)
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
		forex.NewYahooForexAdapter(),
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
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}()

	log.Printf("C2C monitor %s starting on %s with exchanges=%s", appmeta.Version, server.Addr, strings.Join(cfg.Monitor.Exchanges, ", "))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("http server exited unexpectedly: %v", err)
	}
}

func defaultConfigPath() string {
	if path := strings.TrimSpace(os.Getenv("C2C_CONFIG")); path != "" {
		return path
	}
	return "config/config.yaml"
}
