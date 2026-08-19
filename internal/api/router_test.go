package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"c2c_monitor/config"
	"c2c_monitor/internal/domain"
	"c2c_monitor/internal/service"
	"github.com/gin-gonic/gin"
)

const testAdminToken = "0123456789abcdef0123456789abcdef"

func TestAdminRoutesRequireBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newTestService()
	router := SetupRouter(svc, testAPIConfig())

	tests := []struct {
		name          string
		authorization string
		wantStatus    int
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "wrong", authorization: "Bearer wrong-token", wantStatus: http.StatusUnauthorized},
		{name: "valid", authorization: "Bearer " + testAdminToken, wantStatus: http.StatusOK},
		{name: "case insensitive scheme", authorization: "bearer " + testAdminToken, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"forex_interval_hours":2}`)
			req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestHealthAndReadinessRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newTestService()
	router := SetupRouter(svc, testAPIConfig())

	healthRecorder := httptest.NewRecorder()
	router.ServeHTTP(healthRecorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if healthRecorder.Code != http.StatusOK {
		t.Fatalf("expected health status 200, got %d", healthRecorder.Code)
	}

	readyRecorder := httptest.NewRecorder()
	router.ServeHTTP(readyRecorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if readyRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected readiness status 503 before Forex initialization, got %d", readyRecorder.Code)
	}
}

func TestResetAlertAcceptsZeroAmount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, repo := newTestService()
	router := SetupRouter(svc, testAPIConfig())

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/alerts/reset",
		bytes.NewBufferString(`{"exchange":"gate","side":"buy","amount":0}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testAdminToken)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if repo.deletedExchange != domain.ExchangeGate || repo.deletedSide != "BUY" || repo.deletedAmount != 0 {
		t.Fatalf("unexpected reset target: exchange=%q side=%q amount=%v", repo.deletedExchange, repo.deletedSide, repo.deletedAmount)
	}
}

func TestCORSAllowsOnlyConfiguredOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newTestService()
	router := SetupRouter(svc, testAPIConfig())

	tests := []struct {
		origin          string
		wantAllowOrigin string
	}{
		{origin: "https://monitor.example.com", wantAllowOrigin: "https://monitor.example.com"},
		{origin: "https://attacker.example.com", wantAllowOrigin: ""},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, "/api/meta", nil)
		req.Header.Set("Origin", tt.origin)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, req)

		if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllowOrigin {
			t.Fatalf("origin %s: expected Access-Control-Allow-Origin %q, got %q", tt.origin, tt.wantAllowOrigin, got)
		}
	}
}

func testAPIConfig() *config.Config {
	return &config.Config{
		App: config.AppConfig{
			Port:           8001,
			AdminToken:     testAdminToken,
			AllowedOrigins: []string{"https://monitor.example.com"},
		},
	}
}

func newTestService() (*service.MonitorService, *apiTestRepository) {
	repo := &apiTestRepository{}
	svc := service.NewMonitorService(
		config.MonitorConfig{
			C2CIntervalMinutes:    3,
			ForexIntervalHours:    1,
			ForexMaxAgeHours:      6,
			AlertThresholdPercent: 0.1,
			TargetAmounts:         []float64{0, 30},
			Exchanges:             []string{domain.ExchangeGate},
		},
		repo,
		map[string]domain.IExchange{},
		apiTestForex{},
		apiTestNotifier{},
	)
	return svc, repo
}

type apiTestForex struct{}

func (apiTestForex) GetRate(ctx context.Context, from, to string) (float64, error) {
	return 7.2, nil
}

type apiTestNotifier struct{}

func (apiTestNotifier) Send(ctx context.Context, subject, body string) error {
	return nil
}

type apiTestRepository struct {
	deletedExchange string
	deletedSide     string
	deletedAmount   float64
}

func (r *apiTestRepository) SavePricePoints(ctx context.Context, points []*domain.PricePoint) error {
	return nil
}

func (r *apiTestRepository) GetPriceHistory(ctx context.Context, filter domain.PriceQueryFilter) ([]*domain.PricePoint, error) {
	return nil, nil
}

func (r *apiTestRepository) GetPriceHistoryByGranularity(ctx context.Context, filter domain.PriceQueryFilter, granularity domain.HistoryGranularity) ([]*domain.PricePoint, error) {
	return nil, nil
}

func (r *apiTestRepository) SaveMerchant(ctx context.Context, merchant *domain.Merchant) error {
	return nil
}

func (r *apiTestRepository) SaveForexRate(ctx context.Context, rate *domain.ForexRate) error {
	return nil
}

func (r *apiTestRepository) GetLatestForexRate(ctx context.Context, pair string) (*domain.ForexRate, error) {
	return nil, nil
}

func (r *apiTestRepository) GetForexHistory(ctx context.Context, pair string, start, end time.Time) ([]*domain.ForexRate, error) {
	return nil, nil
}

func (r *apiTestRepository) GetForexHistoryByGranularity(ctx context.Context, pair string, start, end time.Time, granularity domain.HistoryGranularity) ([]*domain.ForexRate, error) {
	return nil, nil
}

func (r *apiTestRepository) UpsertAlertState(ctx context.Context, state *domain.AlertState) error {
	return nil
}

func (r *apiTestRepository) DeleteAlertState(ctx context.Context, exchange, side string, amount float64) error {
	r.deletedExchange = exchange
	r.deletedSide = side
	r.deletedAmount = amount
	return nil
}

func (r *apiTestRepository) GetAlertStates(ctx context.Context) ([]*domain.AlertState, error) {
	return nil, nil
}
