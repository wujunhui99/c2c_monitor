package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"c2c_monitor/config"
	"c2c_monitor/internal/api"
	"c2c_monitor/internal/domain"
	"c2c_monitor/internal/service"
	"github.com/gin-gonic/gin"
)

func TestMonitorServerStartupServesKeyRoutes(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		App: config.AppConfig{Port: 8001},
		Monitor: config.MonitorConfig{
			C2CIntervalMinutes: 1,
			ForexIntervalHours: 1,
			TargetAmounts:      []float64{30},
			Exchanges:          []string{domain.ExchangeBinance, domain.ExchangeGate, domain.ExchangeOKX},
		},
		Database: config.DatabaseConfig{DSN: "integration-test"},
	}

	repo := newMemoryRepository()
	exchanges := map[string]domain.IExchange{
		domain.ExchangeBinance: staticExchange{name: domain.ExchangeBinance, price: 7.08},
		domain.ExchangeGate:    staticExchange{name: domain.ExchangeGate, price: 7.09},
		domain.ExchangeOKX:     staticExchange{name: domain.ExchangeOKX, price: 7.07},
	}

	svc := service.NewMonitorService(
		cfg.Monitor,
		repo,
		exchanges,
		staticForex{rate: 7.2},
		noopNotifier{},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go svc.Start(ctx)

	server := httptest.NewServer(api.SetupRouter(svc, cfg))
	defer server.Close()

	client := server.Client()

	waitFor(t, 3*time.Second, func() error {
		var resp struct {
			Data map[string]domain.ServiceStatus `json:"data"`
		}
		if err := getJSON(client, server.URL+"/api/status", &resp); err != nil {
			return err
		}

		expectedServices := append(domain.SupportedExchangeNames(), "Forex (Reference Sources)")
		for _, name := range expectedServices {
			status, ok := resp.Data[name]
			if !ok {
				return fmt.Errorf("missing service status for %s", name)
			}
			if status.Status != "OK" {
				return fmt.Errorf("service %s not ready yet: %s", name, status.Status)
			}
		}

		return nil
	})

	var metaResp struct {
		Version            string            `json:"version"`
		SupportedExchanges []string          `json:"supported_exchanges"`
		HistoryKeys        map[string]string `json:"history_keys"`
	}
	if err := getJSON(client, server.URL+"/api/meta", &metaResp); err != nil {
		t.Fatalf("failed to read meta route: %v", err)
	}
	if metaResp.Version == "" {
		t.Fatal("expected non-empty version from meta route")
	}
	if len(metaResp.SupportedExchanges) != 3 {
		t.Fatalf("expected 3 supported exchanges from meta route, got %v", metaResp.SupportedExchanges)
	}
	if len(metaResp.HistoryKeys) != 3 {
		t.Fatalf("expected 3 history keys from meta route, got %v", metaResp.HistoryKeys)
	}

	var changelogResp struct {
		Releases []struct {
			Version string `json:"version"`
		} `json:"releases"`
	}
	if err := getJSON(client, server.URL+"/api/changelog", &changelogResp); err != nil {
		t.Fatalf("failed to read changelog route: %v", err)
	}
	if len(changelogResp.Releases) == 0 {
		t.Fatal("expected changelog route to return releases")
	}
	if changelogResp.Releases[0].Version != metaResp.Version {
		t.Fatalf("expected latest changelog version %s, got %s", metaResp.Version, changelogResp.Releases[0].Version)
	}

	var configResp config.MonitorConfig
	if err := getJSON(client, server.URL+"/api/config", &configResp); err != nil {
		t.Fatalf("failed to read config route: %v", err)
	}
	if len(configResp.Exchanges) != 3 {
		t.Fatalf("expected 3 exchanges, got %v", configResp.Exchanges)
	}

	waitFor(t, 3*time.Second, func() error {
		var resp struct {
			Code int                         `json:"code"`
			Data map[string][]map[string]any `json:"data"`
		}
		if err := getJSON(client, server.URL+"/api/v1/history?amount=30&range=1d", &resp); err != nil {
			return err
		}
		if resp.Code != 200 {
			return fmt.Errorf("unexpected history code %d", resp.Code)
		}

		if len(resp.Data["forex"]) == 0 {
			return fmt.Errorf("forex history is empty")
		}
		for _, exchangeName := range domain.SupportedExchangeNames() {
			key := domain.ExchangeResponseKey(exchangeName)
			if len(resp.Data[key]) == 0 {
				return fmt.Errorf("%s history is empty", key)
			}
		}
		return nil
	})
}

func TestHistoryContractMatchesMetaExchangeMetadata(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		App: config.AppConfig{Port: 8001},
		Monitor: config.MonitorConfig{
			C2CIntervalMinutes: 1,
			ForexIntervalHours: 1,
			TargetAmounts:      []float64{30},
			Exchanges:          []string{domain.ExchangeBinance, domain.ExchangeGate, domain.ExchangeOKX},
		},
		Database: config.DatabaseConfig{DSN: "integration-test"},
	}

	repo := newMemoryRepository()
	exchanges := map[string]domain.IExchange{
		domain.ExchangeBinance: staticExchange{name: domain.ExchangeBinance, price: 7.08},
		domain.ExchangeGate:    staticExchange{name: domain.ExchangeGate, price: 7.09},
		domain.ExchangeOKX:     staticExchange{name: domain.ExchangeOKX, price: 7.07},
	}

	svc := service.NewMonitorService(
		cfg.Monitor,
		repo,
		exchanges,
		staticForex{rate: 7.2},
		noopNotifier{},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go svc.Start(ctx)

	server := httptest.NewServer(api.SetupRouter(svc, cfg))
	defer server.Close()

	client := server.Client()

	var metaResp struct {
		SupportedExchanges []string          `json:"supported_exchanges"`
		HistoryKeys        map[string]string `json:"history_keys"`
	}
	if err := getJSON(client, server.URL+"/api/meta", &metaResp); err != nil {
		t.Fatalf("failed to read meta route: %v", err)
	}

	expectedKeys := map[string]struct{}{"forex": {}}
	for _, exchangeName := range metaResp.SupportedExchanges {
		historyKey, ok := metaResp.HistoryKeys[exchangeName]
		if !ok {
			t.Fatalf("expected history key for exchange %s", exchangeName)
		}
		expectedKeys[historyKey] = struct{}{}
	}

	waitFor(t, 3*time.Second, func() error {
		var historyResp struct {
			Code int                         `json:"code"`
			Data map[string][]map[string]any `json:"data"`
		}
		if err := getJSON(client, server.URL+"/api/v1/history?amount=30&range=1d", &historyResp); err != nil {
			return err
		}
		if historyResp.Code != 200 {
			return fmt.Errorf("unexpected history code %d", historyResp.Code)
		}

		if len(historyResp.Data) != len(expectedKeys) {
			return fmt.Errorf("expected %d history keys, got %d", len(expectedKeys), len(historyResp.Data))
		}
		for key := range historyResp.Data {
			if _, ok := expectedKeys[key]; !ok {
				return fmt.Errorf("unexpected history key %s", key)
			}
		}
		for key := range expectedKeys {
			if _, ok := historyResp.Data[key]; !ok {
				return fmt.Errorf("missing history key %s", key)
			}
		}
		return nil
	})
}

type staticExchange struct {
	name  string
	price float64
}

func (e staticExchange) GetTopPrices(ctx context.Context, symbol, fiat, side string, amount float64) ([]domain.PricePoint, error) {
	return []domain.PricePoint{
		{
			Exchange:        e.name,
			Symbol:          symbol,
			Fiat:            fiat,
			Side:            side,
			TargetAmount:    amount,
			Rank:            1,
			Price:           e.price,
			Merchant:        e.name + " Merchant",
			MerchantID:      e.name + "-merchant",
			PayMethods:      domain.JoinNormalizedPayMethods([]string{"银行卡"}),
			MinAmount:       0,
			MaxAmount:       100000,
			AvailableAmount: 9999,
			CreatedAt:       time.Now(),
		},
	}, nil
}

type staticForex struct {
	rate float64
}

func (f staticForex) GetRate(ctx context.Context, from, to string) (float64, error) {
	return f.rate, nil
}

type noopNotifier struct{}

func (noopNotifier) Send(ctx context.Context, subject, body string) error {
	return nil
}

type memoryRepository struct {
	mu                 sync.RWMutex
	prices             []*domain.PricePoint
	forexRates         []*domain.ForexRate
	alertStates        map[string]*domain.AlertState
	alertBenchmark     *domain.AlertBenchmark
	benchmarkOverrides map[float64]*domain.AlertBenchmarkOverride
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		alertStates:        make(map[string]*domain.AlertState),
		benchmarkOverrides: make(map[float64]*domain.AlertBenchmarkOverride),
	}
}

func (r *memoryRepository) SavePricePoints(ctx context.Context, points []*domain.PricePoint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, point := range points {
		copyPoint := *point
		copyPoint.ID = int64(len(r.prices) + 1)
		r.prices = append(r.prices, &copyPoint)
	}
	return nil
}

func (r *memoryRepository) GetPriceHistory(ctx context.Context, filter domain.PriceQueryFilter) ([]*domain.PricePoint, error) {
	return r.filteredPrices(filter), nil
}

func (r *memoryRepository) GetPriceHistoryByGranularity(ctx context.Context, filter domain.PriceQueryFilter, granularity domain.HistoryGranularity) ([]*domain.PricePoint, error) {
	return r.filteredPrices(filter), nil
}

func (r *memoryRepository) SaveMerchant(ctx context.Context, merchant *domain.Merchant) error {
	return nil
}

func (r *memoryRepository) SaveForexRate(ctx context.Context, rate *domain.ForexRate) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	copyRate := *rate
	copyRate.ID = int64(len(r.forexRates) + 1)
	r.forexRates = append(r.forexRates, &copyRate)
	return nil
}

func (r *memoryRepository) GetLatestForexRate(ctx context.Context, pair string) (*domain.ForexRate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := len(r.forexRates) - 1; i >= 0; i-- {
		if r.forexRates[i].Pair == pair {
			copyRate := *r.forexRates[i]
			return &copyRate, nil
		}
	}
	return nil, nil
}

func (r *memoryRepository) GetForexHistory(ctx context.Context, pair string, start, end time.Time) ([]*domain.ForexRate, error) {
	return r.filteredForexRates(pair, start, end), nil
}

func (r *memoryRepository) GetForexHistoryByGranularity(ctx context.Context, pair string, start, end time.Time, granularity domain.HistoryGranularity) ([]*domain.ForexRate, error) {
	return r.filteredForexRates(pair, start, end), nil
}

func (r *memoryRepository) UpsertAlertState(ctx context.Context, state *domain.AlertState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	copyState := *state
	r.alertStates[alertStateKey(state.Exchange, state.Side, state.TargetAmount)] = &copyState
	return nil
}

func (r *memoryRepository) DeleteAlertState(ctx context.Context, exchange, side string, amount float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.alertStates, alertStateKey(exchange, side, amount))
	return nil
}

func (r *memoryRepository) GetAlertStates(ctx context.Context) ([]*domain.AlertState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	states := make([]*domain.AlertState, 0, len(r.alertStates))
	for _, state := range r.alertStates {
		copyState := *state
		states = append(states, &copyState)
	}

	sort.Slice(states, func(i, j int) bool {
		if states[i].Exchange == states[j].Exchange {
			return states[i].TargetAmount < states[j].TargetAmount
		}
		return states[i].Exchange < states[j].Exchange
	})

	return states, nil
}

func (r *memoryRepository) UpsertAlertBenchmark(ctx context.Context, benchmark *domain.AlertBenchmark) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	copyBenchmark := *benchmark
	r.alertBenchmark = &copyBenchmark
	return nil
}

func (r *memoryRepository) GetAlertBenchmark(ctx context.Context, pair string) (*domain.AlertBenchmark, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.alertBenchmark == nil || r.alertBenchmark.Pair != pair {
		return nil, nil
	}
	copyBenchmark := *r.alertBenchmark
	return &copyBenchmark, nil
}

func (r *memoryRepository) UpsertAlertBenchmarkOverride(ctx context.Context, override *domain.AlertBenchmarkOverride) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	copyOverride := *override
	r.benchmarkOverrides[override.TargetAmount] = &copyOverride
	return nil
}

func (r *memoryRepository) GetAlertBenchmarkOverrides(ctx context.Context, pair string) ([]*domain.AlertBenchmarkOverride, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]*domain.AlertBenchmarkOverride, 0, len(r.benchmarkOverrides))
	for _, override := range r.benchmarkOverrides {
		if override.Pair != pair {
			continue
		}
		copyOverride := *override
		results = append(results, &copyOverride)
	}
	return results, nil
}

func (r *memoryRepository) filteredPrices(filter domain.PriceQueryFilter) []*domain.PricePoint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]*domain.PricePoint, 0, len(r.prices))
	for _, point := range r.prices {
		if !matchesPriceFilter(point, filter) {
			continue
		}
		copyPoint := *point
		results = append(results, &copyPoint)
	}

	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}
	return results
}

func (r *memoryRepository) filteredForexRates(pair string, start, end time.Time) []*domain.ForexRate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]*domain.ForexRate, 0, len(r.forexRates))
	for _, rate := range r.forexRates {
		if pair != "" && rate.Pair != pair {
			continue
		}
		if !start.IsZero() && rate.CreatedAt.Before(start) {
			continue
		}
		if !end.IsZero() && rate.CreatedAt.After(end) {
			continue
		}

		copyRate := *rate
		results = append(results, &copyRate)
	}
	return results
}

func matchesPriceFilter(point *domain.PricePoint, filter domain.PriceQueryFilter) bool {
	if filter.Exchange != "" && point.Exchange != filter.Exchange {
		return false
	}
	if filter.Symbol != "" && point.Symbol != filter.Symbol {
		return false
	}
	if filter.Fiat != "" && point.Fiat != filter.Fiat {
		return false
	}
	if filter.Side != "" && point.Side != filter.Side {
		return false
	}
	if filter.TargetAmount != nil && point.TargetAmount != *filter.TargetAmount {
		return false
	}
	if filter.Rank > 0 && point.Rank != filter.Rank {
		return false
	}
	if !filter.StartTime.IsZero() && point.CreatedAt.Before(filter.StartTime) {
		return false
	}
	if !filter.EndTime.IsZero() && point.CreatedAt.After(filter.EndTime) {
		return false
	}
	return true
}

func alertStateKey(exchange, side string, amount float64) string {
	return fmt.Sprintf("%s|%s|%.2f", exchange, side, amount)
}

func getJSON(client *http.Client, url string, target any) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(payload))
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func waitFor(t *testing.T, timeout time.Duration, check func() error) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := check(); err == nil {
			return
		} else {
			lastErr = err
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("condition not met within %v: %v", timeout, lastErr)
}
