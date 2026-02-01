package main

import (
	"context"
	"fmt"
	"log"

	"c2c_monitor/config"
	"c2c_monitor/internal/infrastructure/notifier"
)

func main() {
	fmt.Println("Testing Email Configuration...")

	// Load Config
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	emailCfg := cfg.Notification.Email
	fmt.Printf("SMTP Host: %s:%d\n", emailCfg.SMTPHost, emailCfg.SMTPPort)
	fmt.Printf("Sender: %s\n", emailCfg.From)
	fmt.Printf("Recipient: %v\n", emailCfg.To)

	// Init Sender
	sender := notifier.NewSMTPNotifier(
		emailCfg.SMTPHost,
		emailCfg.SMTPPort,
		emailCfg.Username,
		emailCfg.Password,
		emailCfg.From,
		emailCfg.To,
	)

	// Send Test Email
	subject := "C2C Monitor Test Email"
	body := "<h1>Hello!</h1><p>This is a test email from C2C Monitor (你的小助手).</p>"

	err = sender.Send(context.Background(), subject, body)
	if err != nil {
		log.Fatalf("❌ Failed to send email: %v", err)
	}

	fmt.Println("✅ Email sent successfully!")
}
