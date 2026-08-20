package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"math"
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
	cfg                 config.MonitorConfig
	repo                domain.IRepository
	exchanges           map[string]domain.IExchange
	forex               domain.IForex
	notifier            domain.INotifier
	lastForex           float64
	lastForexAt         time.Time
	cfgMu               sync.RWMutex
	forexMu             sync.RWMutex
	benchmarkMu         sync.RWMutex
	alertBenchmark      float64
	alertBenchmarkDirty bool
	scheduleMu          sync.Mutex
	configChanged       chan struct{}
	errorAlertCache     map[string]time.Time             // To prevent spamming error alerts
	triggeredLowPrices  map[string]float64               // To store the lowest triggered price for dynamic threshold
	serviceStatus       map[string]*domain.ServiceStatus // Track status of each service
	downEventLogger     *slog.Logger
	mu                  sync.RWMutex // Mutex for protecting maps
}

const forexServiceName = "Forex (Reference Sources)"
const alertBenchmarkPair = "USDCNY"
const serviceDownLogPath = "logs/service_down.log"

var ErrInvalidAlertBenchmark = errors.New("invalid alert benchmark")

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
		configChanged:      make(chan struct{}),
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

func (s *MonitorService) setLastForex(rate float64, observedAt time.Time) {
	s.forexMu.Lock()
	s.lastForex = rate
	s.lastForexAt = observedAt
	s.forexMu.Unlock()
}

func (s *MonitorService) getLastForex() (float64, time.Time) {
	s.forexMu.RLock()
	defer s.forexMu.RUnlock()
	return s.lastForex, s.lastForexAt
}

func (s *MonitorService) usableForex(now time.Time) (float64, error) {
	rate, observedAt := s.getLastForex()
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 || observedAt.IsZero() {
		return 0, fmt.Errorf("forex rate is unavailable")
	}

	cfg := s.getConfigSnapshot()
	maxAge := time.Duration(cfg.ForexMaxAgeHours) * time.Hour
	if maxAge <= 0 {
		maxAge = 6 * time.Hour
	}
	if age := now.Sub(observedAt); age > maxAge {
		return 0, fmt.Errorf("forex rate is stale: age %s exceeds %s", age.Round(time.Second), maxAge)
	}

	return rate, nil
}

func (s *MonitorService) configChangeSignal() <-chan struct{} {
	s.scheduleMu.Lock()
	defer s.scheduleMu.Unlock()
	return s.configChanged
}

func (s *MonitorService) notifyConfigChanged() {
	s.scheduleMu.Lock()
	close(s.configChanged)
	s.configChanged = make(chan struct{})
	s.scheduleMu.Unlock()
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
	if err != nil {
		s.updateServiceHealth(name, "Error", err.Error())
		return
	}
	s.updateServiceHealth(name, "OK", "")
}

func (s *MonitorService) updateServiceHealth(name, statusValue, message string) {
	s.mu.Lock()

	if _, exists := s.serviceStatus[name]; !exists {
		s.serviceStatus[name] = &domain.ServiceStatus{Name: name}
	}

	status := s.serviceStatus[name]
	previousStatus := status.Status
	status.LastCheck = time.Now()
	status.Status = statusValue
	status.Message = message

	shouldSendErrorAlert := false
	if statusValue != "Error" {
		delete(s.errorAlertCache, name)
	} else if statusValue == "Error" && s.notifierEnabled() {
		if _, exists := s.errorAlertCache[name]; !exists {
			s.errorAlertCache[name] = time.Now()
			shouldSendErrorAlert = true
		}
	}
	s.mu.Unlock()

	if statusValue == "Error" && previousStatus != "Error" {
		err := fmt.Errorf("%s", message)
		s.logServiceDown(name, err)
	}
	if shouldSendErrorAlert {
		go s.sendErrorAlert(name, fmt.Errorf("%s", message))
	}
	if statusValue == "OK" && previousStatus != "" && previousStatus != "Pending" && previousStatus != "OK" {
		slog.Info("service recovered", "event", "service_recovered", "service", name)
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
	if !s.notifierEnabled() {
		return
	}
	subject := fmt.Sprintf("⚠️ [C2C Monitor] Service Down: %s", name)
	body := fmt.Sprintf(`
		<h3>Service Status Change</h3>
		<p><b>Service:</b> %s</p>
		<p><b>Status:</b> <span style="color:red; font-weight:bold;">ERROR</span></p>
		<p><b>Details:</b> %v</p>
		<p><i>Alert sent once. Will not alert again until service recovers and fails again.</i></p>
		<br/>
		<p>Time: %s</p>
	`, html.EscapeString(name), html.EscapeString(err.Error()), time.Now().Format(time.RFC3339))

	slog.Warn("sending error alert", "event", "error_alert_sending", "service", name, "subject", subject)
	if notifErr := s.notifier.Send(context.Background(), subject, body); notifErr != nil {
		slog.Error("failed to send error alert email", "event", "error_alert_send_failed", "service", name, "error", notifErr)
		s.mu.Lock()
		delete(s.errorAlertCache, name)
		s.mu.Unlock()
		return
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
		key := domain.AlertStateKey(state.Exchange, state.Side, state.TargetAmount)
		s.triggeredLowPrices[key] = state.TriggerPrice
	}

	slog.Info("loaded persisted alert states", "event", "alert_states_loaded", "count", len(states))
}

func (s *MonitorService) loadPersistedAlertBenchmark(ctx context.Context) {
	benchmark, err := s.repo.GetAlertBenchmark(ctx, alertBenchmarkPair)
	if err != nil {
		slog.Error("failed to load persisted alert benchmark", "event", "alert_benchmark_load_failed", "error", err)
		return
	}
	if benchmark == nil {
		return
	}
	if math.IsNaN(benchmark.Price) || math.IsInf(benchmark.Price, 0) || benchmark.Price <= 0 {
		slog.Error("persisted alert benchmark is invalid", "event", "alert_benchmark_invalid", "price", benchmark.Price)
		return
	}

	s.benchmarkMu.Lock()
	s.alertBenchmark = benchmark.Price
	s.alertBenchmarkDirty = false
	s.benchmarkMu.Unlock()
	slog.Info("loaded persisted alert benchmark", "event", "alert_benchmark_loaded", "price", benchmark.Price)
}

// Start begins the monitoring loops
func (s *MonitorService) Start(ctx context.Context) {
	slog.Info("monitor service started", "event", "monitor_service_started")

	// Recover persisted dynamic thresholds and cooldown timestamps.
	s.loadPersistedAlertStates(ctx)
	s.loadPersistedAlertBenchmark(ctx)

	// Initial Forex fetch
	s.updateForex(ctx)

	go s.runC2CLoop(ctx)
	s.runForexLoop(ctx)
	slog.Info("monitor service stopping", "event", "monitor_service_stopping")
}

func (s *MonitorService) runC2CLoop(ctx context.Context) {
	s.checkC2C(ctx)

	for ctx.Err() == nil {
		configChanged := s.configChangeSignal()
		nextInterval := s.getNextC2CDuration()
		timer := time.NewTimer(nextInterval)
		slog.Info("scheduled next c2c check", "event", "c2c_check_scheduled", "delay", nextInterval.String())

		select {
		case <-ctx.Done():
			stopTimer(timer)
			return
		case <-configChanged:
			stopTimer(timer)
			continue
		case <-timer.C:
			s.checkC2C(ctx)
		}
	}
}

func (s *MonitorService) runForexLoop(ctx context.Context) {
	for ctx.Err() == nil {
		configChanged := s.configChangeSignal()
		cfg := s.getConfigSnapshot()
		interval := time.Duration(cfg.ForexIntervalHours) * time.Hour
		if interval <= 0 {
			interval = time.Hour
		}
		timer := time.NewTimer(interval)

		select {
		case <-ctx.Done():
			stopTimer(timer)
			return
		case <-configChanged:
			stopTimer(timer)
			continue
		case <-timer.C:
			s.updateForex(ctx)
		}
	}
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func (s *MonitorService) updateForex(ctx context.Context) {
	rate, err := s.forex.GetRate(ctx, "USD", "CNY")
	if err == nil && (math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0) {
		err = fmt.Errorf("forex source returned invalid rate")
	}

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
		if dbErr != nil {
			slog.Error("failed to load cached forex rate", "event", "forex_cache_load_failed", "pair", "USDCNY", "error", dbErr)
			return
		}
		if latest != nil {
			if math.IsNaN(latest.Rate) || math.IsInf(latest.Rate, 0) || latest.Rate <= 0 {
				slog.Error("cached forex rate is invalid", "event", "forex_cache_invalid", "pair", "USDCNY", "rate", latest.Rate)
				return
			}
			maxAge := time.Duration(s.getConfigSnapshot().ForexMaxAgeHours) * time.Hour
			if maxAge <= 0 {
				maxAge = 6 * time.Hour
			}
			if latest.CreatedAt.IsZero() || time.Since(latest.CreatedAt) > maxAge {
				slog.Error("cached forex rate is stale", "event", "forex_cache_stale", "pair", "USDCNY", "observed_at", latest.CreatedAt, "max_age", maxAge.String())
				return
			}
			s.setLastForex(latest.Rate, latest.CreatedAt)
			s.reconcileAlertBenchmark(ctx, latest.Rate)
			slog.Warn("using cached forex rate from database", "event", "forex_cache_used", "pair", "USDCNY", "rate", latest.Rate, "source", latest.Source, "observed_at", latest.CreatedAt)
		}
		return
	}

	sourceName := s.forexSourceName()
	now := time.Now()
	s.setLastForex(rate, now)
	s.reconcileAlertBenchmark(ctx, rate)
	slog.Info("updated forex rate", "event", "forex_updated", "pair", "USDCNY", "rate", rate, "source", sourceName)

	// Save to DB
	err = s.repo.SaveForexRate(ctx, &domain.ForexRate{
		CreatedAt: now,
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
	if _, err := s.usableForex(time.Now()); err != nil {
		s.updateServiceStatus(forexServiceName, err)
		slog.Warn("collecting c2c prices without opportunity alerts because forex rate is unusable", "event", "c2c_alerts_paused_forex", "error", err)
	}

	cfg := s.getConfigSnapshot()

	type c2cJob struct {
		name     string
		exchange domain.IExchange
		amount   float64
	}

	var jobs []c2cJob
	type exchangeResult struct {
		attempted int
		succeeded int
		empty     int
		failed    int
		errors    []string
	}
	results := make(map[string]*exchangeResult, len(cfg.Exchanges))

	for _, name := range cfg.Exchanges {
		result := &exchangeResult{attempted: len(cfg.TargetAmounts)}
		results[name] = result
		exchange, ok := s.exchanges[name]
		if !ok {
			result.failed = result.attempted
			result.errors = append(result.errors, "adapter is not configured")
			continue
		}
		for _, amount := range cfg.TargetAmounts {
			jobs = append(jobs, c2cJob{
				name:     name,
				exchange: exchange,
				amount:   amount,
			})
		}
	}

	const maxConcurrentFetches = 6
	sem := make(chan struct{}, maxConcurrentFetches)
	var wg sync.WaitGroup

	var resultMu sync.Mutex
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
				results[job.name].failed++
				results[job.name].errors = append(results[job.name].errors, fmt.Sprintf("%.4g: %v", job.amount, err))
				resultMu.Unlock()
				return
			}

			resultMu.Lock()
			if len(prices) == 0 {
				results[job.name].empty++
			} else {
				results[job.name].succeeded++
			}
			resultMu.Unlock()

			if len(prices) == 0 {
				return
			}
			if !s.collectionTargetConfigured(job.name, job.amount) {
				return
			}

			s.persistPricesAndMerchants(ctx, prices)
			s.checkAlert(ctx, prices[0])
		}()
	}

	wg.Wait()

	if ctx.Err() != nil {
		return
	}
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	if !sameCollectionScope(cfg, s.cfg) {
		slog.Info("discarding c2c service status from superseded config", "event", "c2c_status_discarded_config_changed")
		return
	}

	for exchangeName, result := range results {
		switch {
		case result.succeeded == result.attempted:
			s.updateServiceHealth(exchangeName, "OK", "")
		case result.succeeded > 0:
			message := fmt.Sprintf("%d/%d amount tiers returned data; %d failed; %d empty", result.succeeded, result.attempted, result.failed, result.empty)
			if len(result.errors) > 0 {
				message += ": " + strings.Join(result.errors, "; ")
			}
			s.updateServiceHealth(exchangeName, "Degraded", message)
		default:
			message := fmt.Sprintf("no amount tiers returned data; %d failed; %d empty", result.failed, result.empty)
			if len(result.errors) > 0 {
				message += ": " + strings.Join(result.errors, "; ")
			}
			s.updateServiceHealth(exchangeName, "Error", message)
			continue
		}
	}
}

func (s *MonitorService) collectionTargetConfigured(exchangeName string, amount float64) bool {
	cfg := s.getConfigSnapshot()
	exchangeConfigured := false
	for _, configuredExchange := range cfg.Exchanges {
		if configuredExchange == exchangeName {
			exchangeConfigured = true
			break
		}
	}
	if !exchangeConfigured {
		return false
	}
	for _, configuredAmount := range cfg.TargetAmounts {
		if configuredAmount == amount {
			return true
		}
	}
	return false
}

func sameCollectionScope(left, right config.MonitorConfig) bool {
	if len(left.Exchanges) != len(right.Exchanges) || len(left.TargetAmounts) != len(right.TargetAmounts) {
		return false
	}
	for index := range left.Exchanges {
		if left.Exchanges[index] != right.Exchanges[index] {
			return false
		}
	}
	for index := range left.TargetAmounts {
		if left.TargetAmounts[index] != right.TargetAmounts[index] {
			return false
		}
	}
	return true
}

func (s *MonitorService) fetchTopPricesWithRetry(ctx context.Context, exchangeName string, exchange domain.IExchange, amount float64) ([]domain.PricePoint, error) {
	const maxAttempts = 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		prices, err := exchange.GetTopPrices(attemptCtx, "USDT", "CNY", "BUY", amount)
		cancel()
		if err == nil {
			return prices, nil
		}
		lastErr = err

		if attempt < maxAttempts {
			retryInterval := time.Duration(1<<(attempt-1)) * 2 * time.Second
			slog.Warn("failed to fetch prices; retrying",
				"event", "exchange_fetch_retry",
				"exchange", exchangeName,
				"amount", amount,
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"retry_in", retryInterval.String(),
				"error", err,
			)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryInterval):
			}
		}
	}

	finalErr := fmt.Errorf("failed to fetch prices for amount %.4g after %d attempts: %w", amount, maxAttempts, lastErr)
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
	if p.Price <= 0 {
		return
	}

	forexRate, err := s.usableForex(time.Now())
	if err != nil {
		return
	}
	benchmarkPrice := s.reconcileAlertBenchmark(ctx, forexRate)

	spread := (forexRate - p.Price) / forexRate * 100
	alertKey := domain.AlertStateKey(p.Exchange, p.Side, p.TargetAmount)

	s.mu.RLock()
	triggeredPrice, isTriggered := s.triggeredLowPrices[alertKey]
	s.mu.RUnlock()

	effectiveBenchmark := benchmarkPrice
	alertType := "Initial" // Initial or Lower
	if isTriggered {
		alertType = "Lower"
		if triggeredPrice < effectiveBenchmark {
			effectiveBenchmark = triggeredPrice
		}
	}

	if p.Price >= effectiveBenchmark || !s.notifierEnabled() {
		return
	}

	now := time.Now()

	var subject string
	if alertType == "Lower" {
		subject = fmt.Sprintf("📉 New Low! %s %s USDT Price: %.4f (Benchmark: %.4f)", p.Exchange, p.Merchant, p.Price, effectiveBenchmark)
	} else {
		subject = fmt.Sprintf("🚨 Opportunity! %s %s USDT Price: %.4f (Benchmark: %.4f)", p.Exchange, p.Merchant, p.Price, effectiveBenchmark)
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
			<p><b>Alert Benchmark:</b> %.4f CNY</p>
			<p><b>Forex Rate:</b> %.4f CNY</p>
			<p><b>Spread:</b> <span style="color:green; font-weight:bold;">%.2f%%</span></p>
			<p><i>Threshold Mode: %s</i></p>
			<br/>
			<p>Time: %s</p>
		`, html.EscapeString(p.Exchange), html.EscapeString(p.Merchant), html.EscapeString(p.Side), p.MinAmount, p.MaxAmount, html.EscapeString(p.PayMethods), p.Price, effectiveBenchmark, forexRate, spread, alertType, now.Format(time.RFC3339))

	slog.Warn("triggering price alert", "event", "price_alert_triggered", "alert_type", alertType, "exchange", p.Exchange, "merchant", p.Merchant, "price", p.Price, "benchmark", effectiveBenchmark, "forex_rate", forexRate, "spread", spread)

	if err := s.notifier.Send(ctx, subject, body); err != nil {
		slog.Error("failed to send alert email", "event", "price_alert_send_failed", "exchange", p.Exchange, "merchant", p.Merchant, "error", err)
		return
	}

	if err := s.repo.UpsertAlertState(ctx, &domain.AlertState{
		Exchange:     p.Exchange,
		Side:         p.Side,
		TargetAmount: p.TargetAmount,
		TriggerPrice: p.Price,
		LastAlertAt:  now,
	}); err != nil {
		slog.Error("failed to persist alert state", "event", "alert_state_persist_failed", "key", alertKey, "error", err)
	}

	s.mu.Lock()
	s.triggeredLowPrices[alertKey] = p.Price
	s.mu.Unlock()
}

func (s *MonitorService) notifierEnabled() bool {
	type enabledNotifier interface {
		Enabled() bool
	}
	if notifier, ok := s.notifier.(enabledNotifier); ok {
		return notifier.Enabled()
	}
	return s.notifier != nil
}

// ResetAlertState resets the dynamic threshold for a specific market
func (s *MonitorService) ResetAlertState(ctx context.Context, exchange, side string, amount float64) error {
	key := domain.AlertStateKey(exchange, side, amount)
	if err := s.repo.DeleteAlertState(ctx, exchange, side, amount); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.triggeredLowPrices, key)
	s.mu.Unlock()

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

func (s *MonitorService) GetAlertBenchmark(ctx context.Context) (float64, float64, error) {
	forexRate, err := s.usableForex(time.Now())
	if err != nil {
		return 0, 0, err
	}

	return s.reconcileAlertBenchmark(ctx, forexRate), forexRate, nil
}

func (s *MonitorService) UpdateAlertBenchmark(ctx context.Context, requestedPrice float64) (float64, float64, error) {
	forexRate, err := s.usableForex(time.Now())
	if err != nil {
		return 0, 0, err
	}

	if math.IsNaN(requestedPrice) || math.IsInf(requestedPrice, 0) || requestedPrice <= 0 {
		return 0, forexRate, fmt.Errorf("%w: benchmark_price must be greater than 0", ErrInvalidAlertBenchmark)
	}
	if requestedPrice >= forexRate {
		return 0, forexRate, fmt.Errorf("%w: benchmark_price must be lower than the current Forex rate %.4f", ErrInvalidAlertBenchmark, forexRate)
	}

	s.benchmarkMu.Lock()
	defer s.benchmarkMu.Unlock()

	currentPrice := forexRate
	if s.alertBenchmark > 0 && s.alertBenchmark < currentPrice {
		currentPrice = s.alertBenchmark
	}
	if requestedPrice >= currentPrice {
		return currentPrice, forexRate, fmt.Errorf("%w: benchmark_price must be lower than the current benchmark %.4f", ErrInvalidAlertBenchmark, currentPrice)
	}

	if err := s.repo.UpsertAlertBenchmark(ctx, &domain.AlertBenchmark{
		Pair:  alertBenchmarkPair,
		Price: requestedPrice,
	}); err != nil {
		return 0, forexRate, fmt.Errorf("persist alert benchmark: %w", err)
	}
	s.alertBenchmark = requestedPrice
	s.alertBenchmarkDirty = false

	slog.Info("updated alert benchmark", "event", "alert_benchmark_updated", "price", requestedPrice, "forex_rate", forexRate)
	return requestedPrice, forexRate, nil
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
	s.notifyConfigChanged()
	return nil
}

func (s *MonitorService) ReadinessError() error {
	_, err := s.usableForex(time.Now())
	return err
}

func (s *MonitorService) reconcileAlertBenchmark(ctx context.Context, forexRate float64) float64 {
	s.benchmarkMu.Lock()
	defer s.benchmarkMu.Unlock()

	nextPrice := forexRate
	if s.alertBenchmark > 0 && s.alertBenchmark < nextPrice {
		nextPrice = s.alertBenchmark
	}
	if s.alertBenchmark == nextPrice && !s.alertBenchmarkDirty {
		return nextPrice
	}

	s.alertBenchmark = nextPrice
	if err := s.repo.UpsertAlertBenchmark(ctx, &domain.AlertBenchmark{
		Pair:  alertBenchmarkPair,
		Price: nextPrice,
	}); err != nil {
		s.alertBenchmarkDirty = true
		slog.Error("failed to persist reconciled alert benchmark", "event", "alert_benchmark_persist_failed", "price", nextPrice, "forex_rate", forexRate, "error", err)
		return nextPrice
	}
	s.alertBenchmarkDirty = false

	return nextPrice
}
