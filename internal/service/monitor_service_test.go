package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
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
			Pair:   "USDCNY",
			Rate:   7.2145,
			Source: "cached",
		},
	}

	svc := NewMonitorService(
		config.MonitorConfig{Exchanges: []string{domain.ExchangeGate}},
		repo,
		nil,
		failingForex{err: errors.New("open.er-api timeout; HexaRate timeout")},
		stubNotifier{},
	)

	svc.updateForex(context.Background())

	if got := svc.getLastForex(); got != 7.2145 {
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
		config.MonitorConfig{Exchanges: []string{domain.ExchangeGate}},
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

type stubRepository struct {
	latestForex *domain.ForexRate
	savedForex  *domain.ForexRate
}

func (r *stubRepository) SavePricePoints(ctx context.Context, points []*domain.PricePoint) error {
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
	return nil
}

func (r *stubRepository) DeleteAlertState(ctx context.Context, exchange, side string, amount float64) error {
	return nil
}

func (r *stubRepository) GetAlertStates(ctx context.Context) ([]*domain.AlertState, error) {
	return nil, nil
}
