package forex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFallbackAdapterUsesSecondarySourceAfter429(t *testing.T) {
	t.Helper()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
	}))
	defer primary.Close()

	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status_code":200,"data":{"mid":6.8822}}`))
	}))
	defer secondary.Close()

	adapter := NewFallbackAdapter(
		&OpenERAdapter{
			client:         &http.Client{Timeout: 200 * time.Millisecond},
			latestEndpoint: primary.URL + "/v6/latest/%s",
		},
		&HexaRateAdapter{
			client:         &http.Client{Timeout: 200 * time.Millisecond},
			latestEndpoint: secondary.URL + "/api/rates/latest/%s?target=%s",
		},
	)

	rate, err := adapter.GetRate(context.Background(), "USD", "CNY")
	if err != nil {
		t.Fatalf("GetRate returned error: %v", err)
	}
	if rate != 6.8822 {
		t.Fatalf("expected fallback rate 6.8822, got %f", rate)
	}
	if adapter.LastSourceName() != "HexaRate" {
		t.Fatalf("expected last source HexaRate, got %q", adapter.LastSourceName())
	}
}

func TestFallbackAdapterUsesSecondarySourceAfterTimeout(t *testing.T) {
	t.Helper()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"success","rates":{"CNY":6.891216}}`))
	}))
	defer primary.Close()

	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status_code":200,"data":{"mid":6.8822}}`))
	}))
	defer secondary.Close()

	adapter := NewFallbackAdapter(
		&OpenERAdapter{
			client:         &http.Client{Timeout: 50 * time.Millisecond},
			latestEndpoint: primary.URL + "/v6/latest/%s",
		},
		&HexaRateAdapter{
			client:         &http.Client{Timeout: 200 * time.Millisecond},
			latestEndpoint: secondary.URL + "/api/rates/latest/%s?target=%s",
		},
	)

	rate, err := adapter.GetRate(context.Background(), "USD", "CNY")
	if err != nil {
		t.Fatalf("GetRate returned error: %v", err)
	}
	if rate != 6.8822 {
		t.Fatalf("expected fallback rate 6.8822, got %f", rate)
	}
}

func TestFallbackAdapterReturnsJoinedErrorWhenAllSourcesFail(t *testing.T) {
	t.Helper()

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
	}))
	defer primary.Close()

	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	defer secondary.Close()

	adapter := NewFallbackAdapter(
		&OpenERAdapter{
			client:         &http.Client{Timeout: 200 * time.Millisecond},
			latestEndpoint: primary.URL + "/v6/latest/%s",
		},
		&HexaRateAdapter{
			client:         &http.Client{Timeout: 200 * time.Millisecond},
			latestEndpoint: secondary.URL + "/api/rates/latest/%s?target=%s",
		},
	)

	if _, err := adapter.GetRate(context.Background(), "USD", "CNY"); err == nil {
		t.Fatal("expected error when all sources fail")
	} else {
		if !strings.Contains(err.Error(), "open.er-api") {
			t.Fatalf("expected error to mention open.er-api, got %v", err)
		}
		if !strings.Contains(err.Error(), "HexaRate") {
			t.Fatalf("expected error to mention HexaRate, got %v", err)
		}
	}
}
