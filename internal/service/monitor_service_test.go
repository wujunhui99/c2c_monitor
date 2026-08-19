package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"c2c_monitor/config"
	"c2c_monitor/internal/domain"
	"c2c_monitor/internal/logging"
)

func TestLogServiceDown(t *testing.T) {
	var buf bytes.Buffer

	svc := &MonitorService{
		downEventLogger: logging.NewJSONLogger(&buf, slog.LevelInfo),
	}

	svc.logServiceDown("Gate", errors.New("timeout"))

	got := buf.String()
	if !strings.Contains(got, `"event":"service_down"`) {
		t.Fatalf("expected service name in log, got %q", got)
	}
	if !strings.Contains(got, `"service":"Gate"`) {
		t.Fatalf("expected service field in log, got %q", got)
	}
	if !strings.Contains(got, `"details":"timeout"`) {
		t.Fatalf("expected error details in log, got %q", got)
	}
}

func TestUpdateForexFallsBackToCachedDatabaseRate(t *testing.T) {
	repo := &stubRepository{
		latestForex: &domain.ForexRate{
			CreatedAt: time.Now().Add(-time.Hour),
			Pair:      "USDCNY",
			Rate:      7.2145,
			Source:    "cached",
		},
	}

	svc := NewMonitorService(
		testMonitorConfig(),
		repo,
		nil,
		failingForex{err: errors.New("open.er-api timeout; HexaRate timeout")},
		stubNotifier{},
	)

	svc.updateForex(context.Background())

	if got, _ := svc.getLastForex(); got != 7.2145 {
		t.Fatalf("expected cached forex rate 7.2145, got %f", got)
	}

	statuses := svc.GetServiceStatuses()
	status, ok := statuses[forexServiceName]
	if !ok {
		t.Fatalf("expected %s status to exist", forexServiceName)
	}
	if status.Status != "Error" {
		t.Fatalf("expected forex status Error, got %s", status.Status)
	}
	if !strings.Contains(status.Message, "timeout") {
		t.Fatalf("expected forex error message to mention timeout, got %q", status.Message)
	}
	if repo.savedForex != nil {
		t.Fatalf("did not expect failed forex update to save a new rate")
	}
}

func TestUpdateForexPersistsSuccessfulSourceName(t *testing.T) {
	repo := &stubRepository{}

	svc := NewMonitorService(
		testMonitorConfig(),
		repo,
		nil,
		sourceAwareForex{rate: 6.8912, source: "open.er-api"},
		stubNotifier{},
	)

	svc.updateForex(context.Background())

	if repo.savedForex == nil {
		t.Fatal("expected successful forex update to be saved")
	}
	if repo.savedForex.Source != "open.er-api" {
		t.Fatalf("expected saved source open.er-api, got %q", repo.savedForex.Source)
	}
	if repo.savedForex.Rate != 6.8912 {
		t.Fatalf("expected saved rate 6.8912, got %f", repo.savedForex.Rate)
	}
}

func TestUpdateForexRejectsStaleCachedRate(t *testing.T) {
	repo := &stubRepository{
		latestForex: &domain.ForexRate{
			CreatedAt: time.Now().Add(-7 * time.Hour),
			Pair:      "USDCNY",
			Rate:      7.2145,
			Source:    "cached",
		},
	}

	svc := NewMonitorService(
		testMonitorConfig(),
		repo,
		nil,
		failingForex{err: errors.New("all sources unavailable")},
		stubNotifier{},
	)

	svc.updateForex(context.Background())

	if got, observedAt := svc.getLastForex(); got != 0 || !observedAt.IsZero() {
		t.Fatalf("expected stale cached forex to be rejected, got rate=%f observed_at=%v", got, observedAt)
	}
	if err := svc.ReadinessError(); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("expected readiness error for missing forex, got %v", err)
	}
}

func TestUpdateForexRejectsInvalidSuccessfulRate(t *testing.T) {
	svc := NewMonitorService(
		testMonitorConfig(),
		&stubRepository{},
		nil,
		sourceAwareForex{rate: 0, source: "test"},
		stubNotifier{},
	)

	svc.updateForex(context.Background())

	if err := svc.ReadinessError(); err == nil {
		t.Fatal("expected invalid Forex rate to leave service unready")
	}
	status := svc.GetServiceStatuses()[forexServiceName]
	if status == nil || status.Status != "Error" || !strings.Contains(status.Message, "invalid rate") {
		t.Fatalf("expected invalid Forex status, got %#v", status)
	}
}

func TestCheckAlertDoesNotAdvanceStateWhenNotificationFails(t *testing.T) {
	repo := &stubRepository{}
	svc := NewMonitorService(
		testMonitorConfig(),
		repo,
		nil,
		sourceAwareForex{rate: 7.2, source: "test"},
		failingNotifier{err: errors.New("SMTP timeout")},
	)
	svc.setLastForex(7.2, time.Now())

	svc.checkAlert(context.Background(), testPricePoint(7.0, 0))

	if len(svc.GetAlertStates()) != 0 {
		t.Fatalf("expected failed notification not to advance alert state, got %v", svc.GetAlertStates())
	}
	if repo.savedAlert != nil {
		t.Fatal("expected failed notification not to persist alert state")
	}
}

func TestCheckAlertPersistsStateAfterNotificationSucceeds(t *testing.T) {
	repo := &stubRepository{}
	notifier := &recordingNotifier{}
	svc := NewMonitorService(
		testMonitorConfig(),
		repo,
		nil,
		sourceAwareForex{rate: 7.2, source: "test"},
		notifier,
	)
	svc.setLastForex(7.2, time.Now())

	svc.checkAlert(context.Background(), testPricePoint(7.0, 0))

	key := domain.AlertStateKey(domain.ExchangeGate, "BUY", 0)
	if got := svc.GetAlertStates()[key]; got != 7.0 {
		t.Fatalf("expected alert state 7.0, got %v", svc.GetAlertStates())
	}
	if repo.savedAlert == nil || repo.savedAlert.TriggerPrice != 7.0 {
		t.Fatalf("expected persisted alert state, got %#v", repo.savedAlert)
	}
	if notifier.calls != 1 {
		t.Fatalf("expected one notification, got %d", notifier.calls)
	}
}

func TestCheckAlertRejectsStaleForex(t *testing.T) {
	repo := &stubRepository{}
	notifier := &recordingNotifier{}
	svc := NewMonitorService(
		testMonitorConfig(),
		repo,
		nil,
		sourceAwareForex{rate: 7.2, source: "test"},
		notifier,
	)
	svc.setLastForex(7.2, time.Now().Add(-7*time.Hour))

	svc.checkAlert(context.Background(), testPricePoint(7.0, 30))

	if notifier.calls != 0 {
		t.Fatalf("expected stale forex not to send alert, got %d calls", notifier.calls)
	}
}

func TestCheckC2CStillCollectsWhenForexIsStale(t *testing.T) {
	repo := &stubRepository{}
	svc := NewMonitorService(
		testMonitorConfig(),
		repo,
		map[string]domain.IExchange{
			domain.ExchangeGate: staticTestExchange{},
		},
		sourceAwareForex{rate: 7.2, source: "test"},
		&recordingNotifier{},
	)
	svc.setLastForex(7.2, time.Now().Add(-7*time.Hour))

	svc.checkC2C(context.Background())

	if got := atomic.LoadInt64(&repo.savedPriceBatches); got != 2 {
		t.Fatalf("expected both configured amount tiers to be collected, got %d batches", got)
	}
	if status := svc.GetServiceStatuses()[domain.ExchangeGate]; status == nil || status.Status != "OK" {
		t.Fatalf("expected exchange collection to remain healthy, got %#v", status)
	}
}

func TestCheckC2CMarksPartialAmountCoverageDegraded(t *testing.T) {
	svc := NewMonitorService(
		testMonitorConfig(),
		&stubRepository{},
		map[string]domain.IExchange{
			domain.ExchangeGate: partialTestExchange{},
		},
		sourceAwareForex{rate: 7.2, source: "test"},
		stubNotifier{},
	)
	svc.setLastForex(7.2, time.Now())

	svc.checkC2C(context.Background())

	status := svc.GetServiceStatuses()[domain.ExchangeGate]
	if status == nil || status.Status != "Degraded" {
		t.Fatalf("expected Degraded status, got %#v", status)
	}
	if !strings.Contains(status.Message, "1/2 amount tiers returned data") {
		t.Fatalf("expected amount coverage in status message, got %q", status.Message)
	}
}

func TestCheckC2CDoesNotRestoreRemovedExchangeStatus(t *testing.T) {
	cfg := testMonitorConfig()
	cfg.TargetAmounts = []float64{0}
	exchange := &blockingTestExchange{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	svc := NewMonitorService(
		cfg,
		&stubRepository{},
		map[string]domain.IExchange{
			domain.ExchangeGate: exchange,
		},
		sourceAwareForex{rate: 7.2, source: "test"},
		stubNotifier{},
	)
	svc.setLastForex(7.2, time.Now())

	done := make(chan struct{})
	go func() {
		svc.checkC2C(context.Background())
		close(done)
	}()

	select {
	case <-exchange.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for exchange fetch")
	}

	updated := cfg
	updated.Exchanges = []string{domain.ExchangeBinance}
	if err := svc.UpdateConfig(updated); err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}
	close(exchange.release)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for c2c check")
	}

	statuses := svc.GetServiceStatuses()
	if _, exists := statuses[domain.ExchangeGate]; exists {
		t.Fatalf("removed exchange status was restored: %#v", statuses)
	}
	if _, exists := statuses[domain.ExchangeBinance]; !exists {
		t.Fatalf("expected newly configured exchange status: %#v", statuses)
	}
}

func TestResetAlertStateLeavesMemoryIntactWhenDeleteFails(t *testing.T) {
	repo := &stubRepository{deleteAlertErr: errors.New("database unavailable")}
	svc := NewMonitorService(
		testMonitorConfig(),
		repo,
		nil,
		sourceAwareForex{rate: 7.2, source: "test"},
		stubNotifier{},
	)
	key := domain.AlertStateKey(domain.ExchangeGate, "BUY", 0)
	svc.triggeredLowPrices[key] = 7.0

	err := svc.ResetAlertState(context.Background(), domain.ExchangeGate, "BUY", 0)
	if err == nil {
		t.Fatal("expected reset error")
	}
	if got := svc.GetAlertStates()[key]; got != 7.0 {
		t.Fatalf("expected in-memory state to remain after database failure, got %v", svc.GetAlertStates())
	}
}

func TestUpdateConfigNotifiesSchedulers(t *testing.T) {
	svc := NewMonitorService(
		testMonitorConfig(),
		&stubRepository{},
		nil,
		sourceAwareForex{rate: 7.2, source: "test"},
		stubNotifier{},
	)
	signal := svc.configChangeSignal()

	updated := testMonitorConfig()
	updated.ForexIntervalHours = 2
	if err := svc.UpdateConfig(updated); err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}

	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal("expected config update to wake schedulers")
	}
}

func testMonitorConfig() config.MonitorConfig {
	return config.MonitorConfig{
		C2CIntervalMinutes:    3,
		ForexIntervalHours:    1,
		ForexMaxAgeHours:      6,
		AlertThresholdPercent: 0.1,
		TargetAmounts:         []float64{0, 30},
		Exchanges:             []string{domain.ExchangeGate},
	}
}

func testPricePoint(price, amount float64) domain.PricePoint {
	return domain.PricePoint{
		Exchange:     domain.ExchangeGate,
		Symbol:       "USDT",
		Fiat:         "CNY",
		Side:         "BUY",
		TargetAmount: amount,
		Price:        price,
		Merchant:     "<merchant>",
		PayMethods:   "银行卡",
	}
}

type failingForex struct {
	err error
}

func (f failingForex) GetRate(ctx context.Context, from, to string) (float64, error) {
	return 0, f.err
}

type sourceAwareForex struct {
	rate   float64
	source string
}

func (f sourceAwareForex) GetRate(ctx context.Context, from, to string) (float64, error) {
	return f.rate, nil
}

func (f sourceAwareForex) SourceName() string {
	return f.source
}

type stubNotifier struct{}

func (stubNotifier) Send(ctx context.Context, subject, body string) error {
	return nil
}

type failingNotifier struct {
	err error
}

func (n failingNotifier) Send(ctx context.Context, subject, body string) error {
	return n.err
}

type recordingNotifier struct {
	calls int
}

func (n *recordingNotifier) Send(ctx context.Context, subject, body string) error {
	n.calls++
	return nil
}

type staticTestExchange struct{}

func (staticTestExchange) GetTopPrices(ctx context.Context, symbol, fiat, side string, amount float64) ([]domain.PricePoint, error) {
	return []domain.PricePoint{testPricePoint(7.0, amount)}, nil
}

type partialTestExchange struct{}

func (partialTestExchange) GetTopPrices(ctx context.Context, symbol, fiat, side string, amount float64) ([]domain.PricePoint, error) {
	if amount == 0 {
		return []domain.PricePoint{testPricePoint(7.0, amount)}, nil
	}
	return nil, nil
}

type blockingTestExchange struct {
	started chan struct{}
	release chan struct{}
}

func (e *blockingTestExchange) GetTopPrices(ctx context.Context, symbol, fiat, side string, amount float64) ([]domain.PricePoint, error) {
	select {
	case e.started <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-e.release:
		return []domain.PricePoint{testPricePoint(7.3, amount)}, nil
	}
}

type stubRepository struct {
	latestForex       *domain.ForexRate
	savedForex        *domain.ForexRate
	savedAlert        *domain.AlertState
	deleteAlertErr    error
	savedPriceBatches int64
}

func (r *stubRepository) SavePricePoints(ctx context.Context, points []*domain.PricePoint) error {
	atomic.AddInt64(&r.savedPriceBatches, 1)
	return nil
}

func (r *stubRepository) GetPriceHistory(ctx context.Context, filter domain.PriceQueryFilter) ([]*domain.PricePoint, error) {
	return nil, nil
}

func (r *stubRepository) GetPriceHistoryByGranularity(ctx context.Context, filter domain.PriceQueryFilter, granularity domain.HistoryGranularity) ([]*domain.PricePoint, error) {
	return nil, nil
}

func (r *stubRepository) SaveMerchant(ctx context.Context, merchant *domain.Merchant) error {
	return nil
}

func (r *stubRepository) SaveForexRate(ctx context.Context, rate *domain.ForexRate) error {
	copyRate := *rate
	r.savedForex = &copyRate
	return nil
}

func (r *stubRepository) GetLatestForexRate(ctx context.Context, pair string) (*domain.ForexRate, error) {
	if r.latestForex == nil {
		return nil, nil
	}
	copyRate := *r.latestForex
	return &copyRate, nil
}

func (r *stubRepository) GetForexHistory(ctx context.Context, pair string, start, end time.Time) ([]*domain.ForexRate, error) {
	return nil, nil
}

func (r *stubRepository) GetForexHistoryByGranularity(ctx context.Context, pair string, start, end time.Time, granularity domain.HistoryGranularity) ([]*domain.ForexRate, error) {
	return nil, nil
}

func (r *stubRepository) UpsertAlertState(ctx context.Context, state *domain.AlertState) error {
	copyState := *state
	r.savedAlert = &copyState
	return nil
}

func (r *stubRepository) DeleteAlertState(ctx context.Context, exchange, side string, amount float64) error {
	return r.deleteAlertErr
}

func (r *stubRepository) GetAlertStates(ctx context.Context) ([]*domain.AlertState, error) {
	return nil, nil
}
