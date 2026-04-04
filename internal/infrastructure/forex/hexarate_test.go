package forex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHexaRateAdapterGetRate(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/rates/latest/USD" {
			t.Fatalf("expected path /api/rates/latest/USD, got %s", got)
		}
		if got := r.URL.Query().Get("target"); got != "CNY" {
			t.Fatalf("expected target=CNY, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status_code": 200,
			"data": {
				"base": "USD",
				"target": "CNY",
				"mid": 6.8822
			}
		}`))
	}))
	defer server.Close()

	adapter := &HexaRateAdapter{
		client:         &http.Client{Timeout: 2 * time.Second},
		latestEndpoint: server.URL + "/api/rates/latest/%s?target=%s",
	}

	rate, err := adapter.GetRate(context.Background(), "USD", "CNY")
	if err != nil {
		t.Fatalf("GetRate returned error: %v", err)
	}

	if rate != 6.8822 {
		t.Fatalf("expected rate 6.8822, got %f", rate)
	}
}

func TestHexaRateAdapterRejectsEmptyMidRate(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status_code":200,"data":{"mid":0}}`))
	}))
	defer server.Close()

	adapter := &HexaRateAdapter{
		client:         &http.Client{Timeout: 2 * time.Second},
		latestEndpoint: server.URL + "/api/rates/latest/%s?target=%s",
	}

	if _, err := adapter.GetRate(context.Background(), "USD", "CNY"); err == nil {
		t.Fatal("expected error when mid rate is empty")
	}
}
