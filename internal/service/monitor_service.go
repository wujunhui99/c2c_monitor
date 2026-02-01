package service

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"c2c_monitor/config"
	"c2c_monitor/internal/domain"
)

type MonitorService struct {
	cfg        config.MonitorConfig
	repo       domain.IRepository
	exchanges  map[string]domain.IExchange
	forex      domain.IForex
	notifier   domain.INotifier
	lastForex  float64
	alertCache      map[string]time.Time // To prevent spamming arbitrage alerts
	errorAlertCache map[string]time.Time // To prevent spamming error alerts
}

func NewMonitorService(
	cfg config.MonitorConfig,
	repo domain.IRepository,
	exchanges map[string]domain.IExchange,
	forex domain.IForex,
	notifier domain.INotifier,
) *MonitorService {
	return &MonitorService{
		cfg:             cfg,
		repo:            repo,
		exchanges:       exchanges,
		forex:           forex,
		notifier:        notifier,
		alertCache:      make(map[string]time.Time),
		errorAlertCache: make(map[string]time.Time),
	}
}

func (s *MonitorService) getNextC2CDuration() time.Duration {
	baseMinutes := s.cfg.C2CIntervalMinutes
	if baseMinutes <= 0 {
		baseMinutes = 3
	}
	base := time.Duration(baseMinutes) * time.Minute
	// Add random jitter: 0 to 60 seconds
	jitter := time.Duration(rand.Intn(61)) * time.Second
	return base + jitter
}

// Start begins the monitoring loops
func (s *MonitorService) Start(ctx context.Context) {
	log.Println("Monitor Service started...")

	// Initial Forex fetch
	s.updateForex(ctx)

	// Tickers
	// Forex can remain on a fixed ticker as it's infrequent (1h)
	forexTicker := time.NewTicker(time.Duration(s.cfg.ForexIntervalHours) * time.Hour)

	// Run C2C check immediately on start
	go s.checkC2C(ctx)

	// C2C Loop with Random Jitter
	go func() {
		for {
			// Calculate next interval with jitter
			nextInterval := s.getNextC2CDuration()
			timer := time.NewTimer(nextInterval)
			log.Printf("Next C2C check in %v", nextInterval)

			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				s.checkC2C(ctx)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Println("Monitor Service stopping...")
			return
		case <-forexTicker.C:
			s.updateForex(ctx)
		}
	}
}

func (s *MonitorService) updateForex(ctx context.Context) {
	rate, err := s.forex.GetRate(ctx, "USD", "CNY")
	if err != nil {
		log.Printf("❌ Failed to fetch Forex rate: %v", err)
		// Try to load latest from DB if fetch fails
		latest, dbErr := s.repo.GetLatestForexRate(ctx, "USDCNY")
		if dbErr == nil && latest != nil {
			s.lastForex = latest.Rate
			log.Printf("Using cached Forex rate from DB: %.4f", s.lastForex)
		}
		return
	}

	s.lastForex = rate
	log.Printf("Updated Forex Rate: %.4f", s.lastForex)

	// Save to DB
	err = s.repo.SaveForexRate(ctx, &domain.ForexRate{
		CreatedAt: time.Now(),
		Source:    "Yahoo",
		Pair:      "USDCNY",
		Rate:      rate,
	})
	if err != nil {
		log.Printf("Failed to save Forex rate: %v", err)
	}
}

func (s *MonitorService) checkC2C(ctx context.Context) {
	if s.lastForex == 0 {
		log.Println("Skipping C2C check: Forex rate not yet available")
		return
	}

	for name, exchange := range s.exchanges {
		// Only check configured exchanges
		found := false
		for _, configuredName := range s.cfg.Exchanges {
			if strings.EqualFold(name, configuredName) {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		for _, amount := range s.cfg.TargetAmounts {
			// Fetch prices
			// Assuming we monitor BUY prices (User Buys USDT) for now
			prices, err := exchange.GetTopPrices(ctx, "USDT", "CNY", "BUY", amount)
			if err != nil {
				log.Printf("❌ [%s] Failed to fetch prices for amount %.0f: %v", name, amount, err)
				s.handleScrapeError(ctx, name, err)
				continue
			}

			if len(prices) == 0 {
				continue
			}

			// Save to DB
			// Convert []domain.PricePoint to []*domain.PricePoint
			var ptrs []*domain.PricePoint
			for i := range prices {
				// Copy variable to avoid loop pointer issue
				p := prices[i]
				ptrs = append(ptrs, &p)
			}
			if err := s.repo.SavePricePoints(ctx, ptrs); err != nil {
				log.Printf("Failed to save prices: %v", err)
			}

			// Check Alert Condition (Only check Rank 1 price)
			bestPrice := prices[0]
			s.checkAlert(ctx, bestPrice)
		}
	}
}

func (s *MonitorService) handleScrapeError(ctx context.Context, exchangeName string, err error) {
	// Key is just the exchange name, we don't want to spam for every amount tier
	key := exchangeName
	lastSent, exists := s.errorAlertCache[key]

	// Cooldown: 1 hour for error alerts
	if exists && time.Since(lastSent) < 60*time.Minute {
		return
	}

	subject := fmt.Sprintf("⚠️ [C2C Monitor] Scrape Error: %s", exchangeName)
	body := fmt.Sprintf(`
		<h3>Data Collection Failed</h3>
		<p><b>Exchange:</b> %s</p>
		<p><b>Error Details:</b> %v</p>
		<p><b>Possible Causes:</b></p>
		<ul>
			<li>IP Blocked / Rate Limiting (429/403)</li>
			<li>API Endpoint Changed</li>
			<li>Network Timeout</li>
			<li>Validation/CAPTCHA Triggered</li>
		</ul>
		<p><i>Next alert for this exchange will be suppressed for 1 hour.</i></p>
		<br/>
		<p>Time: %s</p>
	`, exchangeName, err, time.Now().Format(time.RFC3339))

	log.Printf("⚠️ SENDING ERROR ALERT: %s", subject)

	go func() {
		if notifErr := s.notifier.Send(ctx, subject, body); notifErr != nil {
			log.Printf("Failed to send error alert email: %v", notifErr)
		}
	}()

	s.errorAlertCache[key] = time.Now()
}

func (s *MonitorService) checkAlert(ctx context.Context, p domain.PricePoint) {
	// Logic: (Forex - Price) / Forex >= Threshold
	// Example: (7.20 - 7.00) / 7.20 = 0.027 (2.7%)
	
	if p.Price <= 0 {
		return
	}

	spread := (s.lastForex - p.Price) / s.lastForex * 100
	
	// Log checking
	// log.Printf("[%s] Amount: %.0f, Price: %.4f, Forex: %.4f, Spread: %.2f%%", p.Exchange, p.TargetAmount, p.Price, s.lastForex, spread)

	if spread >= s.cfg.AlertThresholdPercent {
		// Check cooldown
		alertKey := fmt.Sprintf("%s-%s-%.0f", p.Exchange, p.Side, p.TargetAmount)
		lastSent, exists := s.alertCache[alertKey]
		
		// Cooldown: 30 minutes
		if exists && time.Since(lastSent) < 30*time.Minute {
			return 
		}

		// Trigger Alert
		subject := fmt.Sprintf("🚨 Opportunity! %s USDT Price: %.4f (Spread: %.2f%%)", p.Exchange, p.Price, spread)
		body := fmt.Sprintf(`
			<h3>C2C Arbitrage Opportunity</h3>
			<p><b>Exchange:</b> %s</p>
			<p><b>Side:</b> User %s</p>
			<p><b>Amount Tier:</b> %.0f CNY</p>
			<p><b>Current Price:</b> %.4f CNY</p>
			<p><b>Forex Rate:</b> %.4f CNY</p>
			<p><b>Spread:</b> <span style="color:green; font-weight:bold;">%.2f%%</span></p>
			<p><i>Threshold: %.2f%%</i></p>
			<br/>
			<p>Time: %s</p>
		`, p.Exchange, p.Side, p.TargetAmount, p.Price, s.lastForex, spread, s.cfg.AlertThresholdPercent, time.Now().Format(time.RFC3339))

		log.Printf("🔥🔥 TRIGGERING ALERT: %s", subject)
		
		go func() {
			err := s.notifier.Send(ctx, subject, body)
			if err != nil {
				log.Printf("Failed to send alert email: %v", err)
			}
		}()

		// Update cache
		s.alertCache[alertKey] = time.Now()
	}
}

// --- API Support Methods ---

func (s *MonitorService) GetPriceHistory(ctx context.Context, filter domain.PriceQueryFilter) ([]*domain.PricePoint, error) {
	return s.repo.GetPriceHistory(ctx, filter)
}

func (s *MonitorService) GetForexHistory(ctx context.Context, pair string, start, end time.Time) ([]*domain.ForexRate, error) {
	return s.repo.GetForexHistory(ctx, pair, start, end)
}

func (s *MonitorService) GetConfig() config.MonitorConfig {
	return s.cfg
}

func (s *MonitorService) UpdateConfig(newCfg config.MonitorConfig) {
	s.cfg = newCfg
	// Note: In a real robust system, we'd need mutexes here.
	// For this MVP, atomic replacement of struct fields isn't guaranteed but likely fine for infrequent updates.
}
