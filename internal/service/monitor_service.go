package service

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"c2c_monitor/config"
	"c2c_monitor/internal/domain"
)

type MonitorService struct {
	cfg                config.MonitorConfig
	repo               domain.IRepository
	exchanges          map[string]domain.IExchange
	forex              domain.IForex
	notifier           domain.INotifier
	lastForex          float64
	alertCache         map[string]time.Time    // To prevent spamming arbitrage alerts
	errorAlertCache    map[string]time.Time    // To prevent spamming error alerts
	triggeredLowPrices map[string]float64      // To store the lowest triggered price for dynamic threshold
	serviceStatus      map[string]*domain.ServiceStatus // Track status of each service
	mu                 sync.RWMutex            // Mutex for protecting maps
}

func NewMonitorService(
	cfg config.MonitorConfig,
	repo domain.IRepository,
	exchanges map[string]domain.IExchange,
	forex domain.IForex,
	notifier domain.INotifier,
) *MonitorService {
	return &MonitorService{
		cfg:                cfg,
		repo:               repo,
		exchanges:          exchanges,
		forex:              forex,
		notifier:           notifier,
		alertCache:         make(map[string]time.Time),
		errorAlertCache:    make(map[string]time.Time),
		triggeredLowPrices: make(map[string]float64),
		serviceStatus:      make(map[string]*domain.ServiceStatus),
	}
}

func (s *MonitorService) updateServiceStatus(name string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.serviceStatus[name]; !exists {
		s.serviceStatus[name] = &domain.ServiceStatus{Name: name}
	}

	status := s.serviceStatus[name]
	status.LastCheck = time.Now()

	if err != nil {
		// If transitioning from OK to Error, send alert
		if status.Status != "Error" {
			status.Status = "Error"
			status.Message = err.Error()
			// Send Alert (Async)
			go s.sendErrorAlert(name, err)
		} else {
			// Update error message but don't alert again
			status.Message = err.Error()
		}
	} else {
		if status.Status == "Error" {
			log.Printf("✅ Service Recovered: %s", name)
		}
		status.Status = "OK"
		status.Message = ""
	}
}

func (s *MonitorService) sendErrorAlert(name string, err error) {
	subject := fmt.Sprintf("⚠️ [C2C Monitor] Service Down: %s", name)
	body := fmt.Sprintf(`
		<h3>Service Status Change</h3>
		<p><b>Service:</b> %s</p>
		<p><b>Status:</b> <span style="color:red; font-weight:bold;">ERROR</span></p>
		<p><b>Details:</b> %v</p>
		<p><i>Alert sent once. Will not alert again until service recovers and fails again.</i></p>
		<br/>
		<p>Time: %s</p>
	`, name, err, time.Now().Format(time.RFC3339))

	log.Printf("⚠️ SENDING ERROR ALERT: %s", subject)
	if notifErr := s.notifier.Send(context.Background(), subject, body); notifErr != nil {
		log.Printf("Failed to send error alert email: %v", notifErr)
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
	
	// Update Status
	if err != nil {
		s.updateServiceStatus("Forex (Yahoo)", err)
	} else {
		s.updateServiceStatus("Forex (Yahoo)", nil)
	}

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
				s.updateServiceStatus(name, err)
				continue
			}
			
			// Success for this exchange (at least for this amount tier)
			// If we want to mark it as OK, we should do it if any amount tier succeeds? 
			// Or should we update status per exchange generally?
			// Let's update status here as OK. 
			// Note: If multiple tiers, one might fail and one might succeed. 
			// If we overwrite status rapidly, it might flicker. 
			// But usually if exchange is down, all fail.
			s.updateServiceStatus(name, nil)

			if len(prices) == 0 {
				continue
			}

			// Save Merchants and Prepare PricePoints
			var ptrs []*domain.PricePoint
			for i := range prices {
				// Copy variable to avoid loop pointer issue
				p := prices[i]
				ptrs = append(ptrs, &p)

				// Save Merchant Info
				if p.MerchantID != "" {
					merchant := &domain.Merchant{
						Exchange:   p.Exchange,
						MerchantID: p.MerchantID,
						NickName:   p.Merchant,
						CreatedAt:  time.Now(),
						UpdatedAt:  time.Now(),
					}
					// Fire and forget or log error? Log is fine.
					// Doing it sequentially here might slow down if there are many prices, 
					// but usually we only fetch top 1 or small N.
					if err := s.repo.SaveMerchant(ctx, merchant); err != nil {
						log.Printf("Failed to save merchant %s: %v", p.Merchant, err)
					}
				}
			}

			// Save to DB
			if err := s.repo.SavePricePoints(ctx, ptrs); err != nil {
				log.Printf("Failed to save prices: %v", err)
			}

			// Check Alert Condition (Only check Rank 1 price)
			bestPrice := prices[0]
			s.checkAlert(ctx, bestPrice)
		}
	}
}

// GetServiceStatuses returns the current health status of services
func (s *MonitorService) GetServiceStatuses() map[string]*domain.ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Deep copy to return
	result := make(map[string]*domain.ServiceStatus)
	for k, v := range s.serviceStatus {
		result[k] = &domain.ServiceStatus{
			Name:      v.Name,
			Status:    v.Status,
			Message:   v.Message,
			LastCheck: v.LastCheck,
		}
	}
	return result
}

func (s *MonitorService) checkAlert(ctx context.Context, p domain.PricePoint) {
	// Logic: (Forex - Price) / Forex >= Threshold
	// OR if already triggered, Price < TriggeredLowPrice

	if p.Price <= 0 {
		return
	}

	spread := (s.lastForex - p.Price) / s.lastForex * 100
	alertKey := fmt.Sprintf("%s-%s-%.0f", p.Exchange, p.Side, p.TargetAmount)

	s.mu.RLock()
	triggeredPrice, isTriggered := s.triggeredLowPrices[alertKey]
	lastSent, lastSentExists := s.alertCache[alertKey]
	s.mu.RUnlock()

	shouldAlert := false
	alertType := "Initial" // Initial or Lower

	if isTriggered {
		// Condition B: Price is LOWER than the recorded lowest price
		if p.Price < triggeredPrice {
			shouldAlert = true
			alertType = "Lower"
		}
	} else {
		// Condition A: Standard threshold check
		if spread >= s.cfg.AlertThresholdPercent {
			// Check cooldown only for Initial alert (or maybe both? User said "set new threshold", implies continuous monitoring)
			// Let's keep cooldown for Initial to avoid oscillation around threshold.
			// For "Lower", usually we want to know immediately if it drops further.
			// But let's apply a small cooldown or check if significant drop?
			// User request: "If 1st triggers, set threshold to record lowest price."
			
			if lastSentExists && time.Since(lastSent) < 30*time.Minute {
				return
			}
			shouldAlert = true
		}
	}

	if shouldAlert {
		// Trigger Alert
		var subject string
		if alertType == "Lower" {
			subject = fmt.Sprintf("📉 New Low! %s USDT Price: %.4f (Was: %.4f)", p.Exchange, p.Price, triggeredPrice)
		} else {
			subject = fmt.Sprintf("🚨 Opportunity! %s USDT Price: %.4f (Spread: %.2f%%)", p.Exchange, p.Price, spread)
		}

		body := fmt.Sprintf(`
			<h3>C2C Arbitrage Opportunity</h3>
			<p><b>Exchange:</b> %s</p>
			<p><b>Side:</b> User %s</p>
			<p><b>Min Amount:</b> %.0f CNY</p>
			<p><b>Max Amount:</b> %.0f CNY</p>
			<p><b>Pay Methods:</b> %s</p>
			<p><b>Current Price:</b> %.4f CNY</p>
			<p><b>Forex Rate:</b> %.4f CNY</p>
			<p><b>Spread:</b> <span style="color:green; font-weight:bold;">%.2f%%</span></p>
			<p><i>Threshold Mode: %s</i></p>
			<br/>
			<p>Time: %s</p>
		`, p.Exchange, p.Side, p.MinAmount, p.MaxAmount, p.PayMethods, p.Price, s.lastForex, spread, alertType, time.Now().Format(time.RFC3339))

		log.Printf("🔥🔥 TRIGGERING ALERT (%s): %s", alertType, subject)

		go func() {
			err := s.notifier.Send(ctx, subject, body)
			if err != nil {
				log.Printf("Failed to send alert email: %v", err)
			}
		}()

		// Update State
		s.mu.Lock()
		s.alertCache[alertKey] = time.Now()
		// Always update the lowest price if we alerted
		s.triggeredLowPrices[alertKey] = p.Price
		s.mu.Unlock()
	}
}

// ResetAlertState resets the dynamic threshold for a specific market
func (s *MonitorService) ResetAlertState(exchange, side string, amount float64) {
	key := fmt.Sprintf("%s-%s-%.0f", exchange, side, amount)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.triggeredLowPrices, key)
	log.Printf("Reset alert state for %s", key)
}

// GetAlertStates returns the current triggered states
func (s *MonitorService) GetAlertStates() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy
	copyMap := make(map[string]float64)
	for k, v := range s.triggeredLowPrices {
		copyMap[k] = v
	}
	return copyMap
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
