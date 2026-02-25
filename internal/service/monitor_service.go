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
	cfgMu              sync.RWMutex
	forexMu            sync.RWMutex
	alertCache         map[string]time.Time             // To prevent spamming arbitrage alerts
	errorAlertCache    map[string]time.Time             // To prevent spamming error alerts
	triggeredLowPrices map[string]float64               // To store the lowest triggered price for dynamic threshold
	serviceStatus      map[string]*domain.ServiceStatus // Track status of each service
	mu                 sync.RWMutex                     // Mutex for protecting maps
}

func NewMonitorService(
	cfg config.MonitorConfig,
	repo domain.IRepository,
	exchanges map[string]domain.IExchange,
	forex domain.IForex,
	notifier domain.INotifier,
) *MonitorService {
	cfgCopy := cloneMonitorConfig(cfg)
	ms := &MonitorService{
		cfg:                cfgCopy,
		repo:               repo,
		exchanges:          exchanges,
		forex:              forex,
		notifier:           notifier,
		alertCache:         make(map[string]time.Time),
		errorAlertCache:    make(map[string]time.Time),
		triggeredLowPrices: make(map[string]float64),
		serviceStatus:      make(map[string]*domain.ServiceStatus),
	}

	// Initialize status for exchanges to ensure they appear in frontend
	for _, name := range cfgCopy.Exchanges {
		ms.serviceStatus[name] = &domain.ServiceStatus{
			Name:      name,
			Status:    "Pending",
			Message:   "Initializing...",
			LastCheck: time.Now(),
		}
	}
	ms.serviceStatus["Forex (OpenER)"] = &domain.ServiceStatus{
		Name:      "Forex (OpenER)",
		Status:    "Pending",
		Message:   "Initializing...",
		LastCheck: time.Now(),
	}

	return ms
}

func cloneMonitorConfig(cfg config.MonitorConfig) config.MonitorConfig {
	copyCfg := cfg
	if cfg.TargetAmounts != nil {
		copyCfg.TargetAmounts = append([]float64(nil), cfg.TargetAmounts...)
	}
	if cfg.Exchanges != nil {
		copyCfg.Exchanges = append([]string(nil), cfg.Exchanges...)
	}
	return copyCfg
}

func (s *MonitorService) getConfigSnapshot() config.MonitorConfig {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return cloneMonitorConfig(s.cfg)
}

func (s *MonitorService) setLastForex(rate float64) {
	s.forexMu.Lock()
	s.lastForex = rate
	s.forexMu.Unlock()
}

func (s *MonitorService) getLastForex() float64 {
	s.forexMu.RLock()
	defer s.forexMu.RUnlock()
	return s.lastForex
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
	cfg := s.getConfigSnapshot()
	baseMinutes := cfg.C2CIntervalMinutes
	if baseMinutes <= 0 {
		baseMinutes = 3
	}
	base := time.Duration(baseMinutes) * time.Minute
	// Add random jitter: 0 to 60 seconds
	jitter := time.Duration(rand.Intn(61)) * time.Second
	return base + jitter
}

func (s *MonitorService) loadPersistedAlertStates(ctx context.Context) {
	states, err := s.repo.GetAlertStates(ctx)
	if err != nil {
		log.Printf("Failed to load persisted alert states: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, state := range states {
		key := fmt.Sprintf("%s-%s-%.0f", state.Exchange, state.Side, state.TargetAmount)
		s.triggeredLowPrices[key] = state.TriggerPrice
		if !state.LastAlertAt.IsZero() {
			s.alertCache[key] = state.LastAlertAt
		}
	}

	log.Printf("Loaded %d persisted alert states", len(states))
}

// Start begins the monitoring loops
func (s *MonitorService) Start(ctx context.Context) {
	log.Println("Monitor Service started...")

	// Recover persisted dynamic thresholds and cooldown timestamps.
	s.loadPersistedAlertStates(ctx)

	// Initial Forex fetch
	s.updateForex(ctx)

	// Tickers
	// Forex can remain on a fixed ticker as it's infrequent (1h)
	cfg := s.getConfigSnapshot()
	forexIntervalHours := cfg.ForexIntervalHours
	if forexIntervalHours <= 0 {
		forexIntervalHours = 1
	}
	forexTicker := time.NewTicker(time.Duration(forexIntervalHours) * time.Hour)

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
		s.updateServiceStatus("Forex (OpenER)", err)
	} else {
		s.updateServiceStatus("Forex (OpenER)", nil)
	}

	if err != nil {
		log.Printf("❌ Failed to fetch Forex rate: %v", err)
		// Try to load latest from DB if fetch fails
		latest, dbErr := s.repo.GetLatestForexRate(ctx, "USDCNY")
		if dbErr == nil && latest != nil {
			s.setLastForex(latest.Rate)
			log.Printf("Using cached Forex rate from DB: %.4f", latest.Rate)
		}
		return
	}

	s.setLastForex(rate)
	log.Printf("Updated Forex Rate: %.4f", rate)

	// Save to DB
	err = s.repo.SaveForexRate(ctx, &domain.ForexRate{
		CreatedAt: time.Now(),
		Source:    "OpenER",
		Pair:      "USDCNY",
		Rate:      rate,
	})
	if err != nil {
		log.Printf("Failed to save Forex rate: %v", err)
	}
}

func (s *MonitorService) checkC2C(ctx context.Context) {
	if s.getLastForex() == 0 {
		log.Println("Skipping C2C check: Forex rate not yet available")
		return
	}

	cfg := s.getConfigSnapshot()
	configuredExchanges := make(map[string]struct{}, len(cfg.Exchanges))
	for _, name := range cfg.Exchanges {
		configuredExchanges[strings.ToLower(name)] = struct{}{}
	}

	type c2cJob struct {
		name     string
		exchange domain.IExchange
		amount   float64
	}

	var jobs []c2cJob
	targetedExchanges := make(map[string]struct{})

	for name, exchange := range s.exchanges {
		if _, ok := configuredExchanges[strings.ToLower(name)]; !ok {
			continue
		}
		targetedExchanges[name] = struct{}{}
		for _, amount := range cfg.TargetAmounts {
			jobs = append(jobs, c2cJob{
				name:     name,
				exchange: exchange,
				amount:   amount,
			})
		}
	}

	if len(jobs) == 0 {
		return
	}

	const maxConcurrentFetches = 6
	sem := make(chan struct{}, maxConcurrentFetches)
	var wg sync.WaitGroup

	var resultMu sync.Mutex
	successByExchange := make(map[string]bool, len(targetedExchanges))
	errByExchange := make(map[string]error, len(targetedExchanges))

	for _, j := range jobs {
		job := j
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			prices, err := s.fetchTopPricesWithRetry(ctx, job.name, job.exchange, job.amount)
			if err != nil {
				resultMu.Lock()
				if _, exists := errByExchange[job.name]; !exists {
					errByExchange[job.name] = err
				}
				resultMu.Unlock()
				return
			}

			resultMu.Lock()
			successByExchange[job.name] = true
			resultMu.Unlock()

			if len(prices) == 0 {
				return
			}

			s.persistPricesAndMerchants(ctx, prices)
			s.checkAlert(ctx, prices[0])
		}()
	}

	wg.Wait()

	for exchangeName := range targetedExchanges {
		if successByExchange[exchangeName] {
			s.updateServiceStatus(exchangeName, nil)
			continue
		}
		if err, ok := errByExchange[exchangeName]; ok {
			s.updateServiceStatus(exchangeName, err)
		}
	}
}

func (s *MonitorService) fetchTopPricesWithRetry(ctx context.Context, exchangeName string, exchange domain.IExchange, amount float64) ([]domain.PricePoint, error) {
	maxRetries := 3
	retryInterval := 90 * time.Second
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		prices, err := exchange.GetTopPrices(attemptCtx, "USDT", "CNY", "BUY", amount)
		cancel()
		if err == nil {
			return prices, nil
		}
		lastErr = err

		if attempt < maxRetries {
			log.Printf("⚠️ [%s] Failed to fetch prices for amount %.0f (Attempt %d/%d): %v. Retrying in %v...", exchangeName, amount, attempt+1, maxRetries, err, retryInterval)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryInterval):
				continue
			}
		}
	}

	finalErr := fmt.Errorf("failed to fetch prices for amount %.0f after %d retries: %w", amount, maxRetries, lastErr)
	log.Printf("❌ [%s] %v", exchangeName, finalErr)
	return nil, finalErr
}

func (s *MonitorService) persistPricesAndMerchants(ctx context.Context, prices []domain.PricePoint) {
	var ptrs []*domain.PricePoint
	for i := range prices {
		p := prices[i]
		ptrs = append(ptrs, &p)

		if p.MerchantID != "" {
			merchant := &domain.Merchant{
				Exchange:   p.Exchange,
				MerchantID: p.MerchantID,
				NickName:   p.Merchant,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
			if err := s.repo.SaveMerchant(ctx, merchant); err != nil {
				log.Printf("Failed to save merchant %s: %v", p.Merchant, err)
			}
		}
	}

	if err := s.repo.SavePricePoints(ctx, ptrs); err != nil {
		log.Printf("Failed to save prices: %v", err)
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

	forexRate := s.getLastForex()
	if forexRate <= 0 {
		return
	}
	cfg := s.getConfigSnapshot()

	spread := (forexRate - p.Price) / forexRate * 100
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
		if spread >= cfg.AlertThresholdPercent {
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
		now := time.Now()

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
		`, p.Exchange, p.Side, p.MinAmount, p.MaxAmount, p.PayMethods, p.Price, forexRate, spread, alertType, now.Format(time.RFC3339))

		log.Printf("🔥🔥 TRIGGERING ALERT (%s): %s", alertType, subject)

		go func() {
			err := s.notifier.Send(ctx, subject, body)
			if err != nil {
				log.Printf("Failed to send alert email: %v", err)
			}
		}()

		// Update State
		s.mu.Lock()
		s.alertCache[alertKey] = now
		// Always update the lowest price if we alerted
		s.triggeredLowPrices[alertKey] = p.Price
		s.mu.Unlock()

		if err := s.repo.UpsertAlertState(ctx, &domain.AlertState{
			Exchange:     p.Exchange,
			Side:         p.Side,
			TargetAmount: p.TargetAmount,
			TriggerPrice: p.Price,
			LastAlertAt:  now,
		}); err != nil {
			log.Printf("Failed to persist alert state for %s: %v", alertKey, err)
		}
	}
}

// ResetAlertState resets the dynamic threshold for a specific market
func (s *MonitorService) ResetAlertState(ctx context.Context, exchange, side string, amount float64) error {
	key := fmt.Sprintf("%s-%s-%.0f", exchange, side, amount)
	s.mu.Lock()
	delete(s.triggeredLowPrices, key)
	delete(s.alertCache, key)
	s.mu.Unlock()

	if err := s.repo.DeleteAlertState(ctx, exchange, side, amount); err != nil {
		return err
	}

	log.Printf("Reset alert state for %s", key)
	return nil
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

func (s *MonitorService) GetPriceHistoryByGranularity(ctx context.Context, filter domain.PriceQueryFilter, granularity domain.HistoryGranularity) ([]*domain.PricePoint, error) {
	return s.repo.GetPriceHistoryByGranularity(ctx, filter, granularity)
}

func (s *MonitorService) GetForexHistory(ctx context.Context, pair string, start, end time.Time) ([]*domain.ForexRate, error) {
	return s.repo.GetForexHistory(ctx, pair, start, end)
}

func (s *MonitorService) GetForexHistoryByGranularity(ctx context.Context, pair string, start, end time.Time, granularity domain.HistoryGranularity) ([]*domain.ForexRate, error) {
	return s.repo.GetForexHistoryByGranularity(ctx, pair, start, end, granularity)
}

func (s *MonitorService) GetConfig() config.MonitorConfig {
	return s.getConfigSnapshot()
}

func (s *MonitorService) UpdateConfig(newCfg config.MonitorConfig) {
	s.cfgMu.Lock()
	s.cfg = cloneMonitorConfig(newCfg)
	s.cfgMu.Unlock()
}
