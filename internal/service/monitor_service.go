package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"c2c_monitor/config"
	"c2c_monitor/internal/domain"
	"c2c_monitor/internal/logging"
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
	downEventLogger    *slog.Logger
	mu                 sync.RWMutex // Mutex for protecting maps
}

const forexServiceName = "Forex (Reference Sources)"
const serviceDownLogPath = "logs/service_down.log"

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
		downEventLogger:    newServiceDownLogger(serviceDownLogPath),
	}

	ms.syncConfiguredServiceStatuses(cfgCopy.Exchanges)

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

func (s *MonitorService) syncConfiguredServiceStatuses(exchangeNames []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	keep := map[string]struct{}{
		forexServiceName: {},
	}

	for _, name := range exchangeNames {
		keep[name] = struct{}{}
		if _, exists := s.serviceStatus[name]; exists {
			continue
		}
		s.serviceStatus[name] = &domain.ServiceStatus{
			Name:      name,
			Status:    "Pending",
			Message:   "Initializing...",
			LastCheck: now,
		}
	}

	if _, exists := s.serviceStatus[forexServiceName]; !exists {
		s.serviceStatus[forexServiceName] = &domain.ServiceStatus{
			Name:      forexServiceName,
			Status:    "Pending",
			Message:   "Initializing...",
			LastCheck: now,
		}
	}

	for name := range s.serviceStatus {
		if _, ok := keep[name]; ok {
			continue
		}
		delete(s.serviceStatus, name)
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
			s.logServiceDown(name, err)
			// Send Alert (Async)
			go s.sendErrorAlert(name, err)
		} else {
			// Update error message but don't alert again
			status.Message = err.Error()
		}
	} else {
		if status.Status == "Error" {
			slog.Info("service recovered", "event", "service_recovered", "service", name)
		}
		status.Status = "OK"
		status.Message = ""
	}
}

func newServiceDownLogger(path string) *slog.Logger {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		slog.Error("failed to create service down log directory", "event", "service_down_log_dir_failed", "path", path, "error", err)
		return logging.NewJSONLogger(io.Discard, slog.LevelInfo)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		slog.Error("failed to open service down log file", "event", "service_down_log_open_failed", "path", path, "error", err)
		return logging.NewJSONLogger(io.Discard, slog.LevelInfo)
	}

	return logging.NewJSONLogger(file, slog.LevelInfo)
}

func (s *MonitorService) logServiceDown(name string, err error) {
	if s.downEventLogger == nil {
		return
	}
	s.downEventLogger.Error("service down", "event", "service_down", "service", name, "details", err.Error())
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

	slog.Warn("sending error alert", "event", "error_alert_sending", "service", name, "subject", subject)
	if notifErr := s.notifier.Send(context.Background(), subject, body); notifErr != nil {
		slog.Error("failed to send error alert email", "event", "error_alert_send_failed", "service", name, "error", notifErr)
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
		slog.Error("failed to load persisted alert states", "event", "alert_states_load_failed", "error", err)
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

	slog.Info("loaded persisted alert states", "event", "alert_states_loaded", "count", len(states))
}

// Start begins the monitoring loops
func (s *MonitorService) Start(ctx context.Context) {
	slog.Info("monitor service started", "event", "monitor_service_started")

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
			slog.Info("scheduled next c2c check", "event", "c2c_check_scheduled", "delay", nextInterval.String())

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
			slog.Info("monitor service stopping", "event", "monitor_service_stopping")
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
		s.updateServiceStatus(forexServiceName, err)
	} else {
		s.updateServiceStatus(forexServiceName, nil)
	}

	if err != nil {
		slog.Error("failed to fetch forex rate", "event", "forex_fetch_failed", "pair", "USDCNY", "error", err)
		// Try to load latest from DB if fetch fails
		latest, dbErr := s.repo.GetLatestForexRate(ctx, "USDCNY")
		if dbErr == nil && latest != nil {
			s.setLastForex(latest.Rate)
			slog.Warn("using cached forex rate from database", "event", "forex_cache_used", "pair", "USDCNY", "rate", latest.Rate, "source", latest.Source)
		}
		return
	}

	sourceName := s.forexSourceName()
	s.setLastForex(rate)
	slog.Info("updated forex rate", "event", "forex_updated", "pair", "USDCNY", "rate", rate, "source", sourceName)

	// Save to DB
	err = s.repo.SaveForexRate(ctx, &domain.ForexRate{
		CreatedAt: time.Now(),
		Source:    sourceName,
		Pair:      "USDCNY",
		Rate:      rate,
	})
	if err != nil {
		slog.Error("failed to save forex rate", "event", "forex_save_failed", "pair", "USDCNY", "source", sourceName, "error", err)
	}
}

func (s *MonitorService) forexSourceName() string {
	type lastSourceNamer interface {
		LastSourceName() string
	}
	type sourceNamer interface {
		SourceName() string
	}

	if named, ok := s.forex.(lastSourceNamer); ok {
		if source := strings.TrimSpace(named.LastSourceName()); source != "" {
			return source
		}
	}

	if named, ok := s.forex.(sourceNamer); ok {
		if source := strings.TrimSpace(named.SourceName()); source != "" {
			return source
		}
	}

	return "External FX API"
}

func (s *MonitorService) checkC2C(ctx context.Context) {
	if s.getLastForex() == 0 {
		slog.Warn("skipping c2c check because forex rate is unavailable", "event", "c2c_check_skipped_missing_forex")
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
			slog.Warn("failed to fetch prices; retrying",
				"event", "exchange_fetch_retry",
				"exchange", exchangeName,
				"amount", amount,
				"attempt", attempt+1,
				"max_attempts", maxRetries,
				"retry_in", retryInterval.String(),
				"error", err,
			)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryInterval):
				continue
			}
		}
	}

	finalErr := fmt.Errorf("failed to fetch prices for amount %.0f after %d retries: %w", amount, maxRetries, lastErr)
	slog.Error("exchange fetch failed after retries", "event", "exchange_fetch_failed", "exchange", exchangeName, "amount", amount, "error", finalErr)
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
				slog.Error("failed to save merchant", "event", "merchant_save_failed", "exchange", p.Exchange, "merchant", p.Merchant, "merchant_id", p.MerchantID, "error", err)
			}
		}
	}

	if err := s.repo.SavePricePoints(ctx, ptrs); err != nil {
		slog.Error("failed to save prices", "event", "prices_save_failed", "count", len(ptrs), "error", err)
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
			subject = fmt.Sprintf("📉 New Low! %s %s USDT Price: %.4f (Was: %.4f)", p.Exchange, p.Merchant, p.Price, triggeredPrice)
		} else {
			subject = fmt.Sprintf("🚨 Opportunity! %s %s USDT Price: %.4f (Spread: %.2f%%)", p.Exchange, p.Merchant, p.Price, spread)
		}

		body := fmt.Sprintf(`
			<h3>C2C Arbitrage Opportunity</h3>
			<p><b>Exchange:</b> %s</p>
			<p><b>Merchant:</b> %s</p>
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
		`, p.Exchange, p.Merchant, p.Side, p.MinAmount, p.MaxAmount, p.PayMethods, p.Price, forexRate, spread, alertType, now.Format(time.RFC3339))

		slog.Warn("triggering price alert", "event", "price_alert_triggered", "alert_type", alertType, "exchange", p.Exchange, "merchant", p.Merchant, "price", p.Price, "spread", spread)

		go func() {
			err := s.notifier.Send(ctx, subject, body)
			if err != nil {
				slog.Error("failed to send alert email", "event", "price_alert_send_failed", "exchange", p.Exchange, "merchant", p.Merchant, "error", err)
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
			slog.Error("failed to persist alert state", "event", "alert_state_persist_failed", "key", alertKey, "error", err)
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

	slog.Info("reset alert state", "event", "alert_state_reset", "key", key)
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

func (s *MonitorService) UpdateConfig(newCfg config.MonitorConfig) error {
	normalizedCfg, err := config.NormalizeMonitorConfig(newCfg)
	if err != nil {
		return err
	}

	s.cfgMu.Lock()
	s.cfg = cloneMonitorConfig(normalizedCfg)
	s.cfgMu.Unlock()

	s.syncConfiguredServiceStatuses(normalizedCfg.Exchanges)
	return nil
}
