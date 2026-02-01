package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"c2c_monitor/config"
	"c2c_monitor/internal/domain"
	"c2c_monitor/internal/infrastructure/exchange"
	"c2c_monitor/internal/infrastructure/forex"
	"c2c_monitor/internal/infrastructure/notifier"
	"c2c_monitor/internal/infrastructure/persistence/mysql"
	"c2c_monitor/internal/service"
	"c2c_monitor/internal/api"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 1. Load Config
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Init Database
	db, err := gorm.Open(gormmysql.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	
	repo := mysql.NewMySQLRepository(db)
	if err := repo.AutoMigrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// 3. Init Adapters
	// Exchanges
	exchanges := make(map[string]domain.IExchange)
	exchanges["binance"] = exchange.NewBinanceAdapter()
	exchanges["okx"] = exchange.NewOKXAdapter()

	// Forex
	forexAdapter := forex.NewYahooForexAdapter()

	// Notifier
	emailNotifier := notifier.NewSMTPNotifier(
		cfg.Notification.Email.SMTPHost,
		cfg.Notification.Email.SMTPPort,
		cfg.Notification.Email.Username,
		cfg.Notification.Email.Password,
		cfg.Notification.Email.From,
		cfg.Notification.Email.To,
	)

	// 4. Init Service
	svc := service.NewMonitorService(
		cfg.Monitor,
		repo,
		exchanges,
		forexAdapter,
		emailNotifier,
	)

	// 5. Start Service (Background)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go svc.Start(ctx)

	// 6. Start Web Server
	router := api.SetupRouter(svc, cfg)
	
	go func() {
		addr := fmt.Sprintf(":%d", cfg.App.Port)
		log.Printf("Web server listening on %s", addr)
		if err := router.Run(addr); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// 7. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
}