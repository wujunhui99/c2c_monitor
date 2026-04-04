package forex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestYahooForexAdapterGetRate(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v8/finance/chart/USDCNY=X" {
			t.Fatalf("expected path /v8/finance/chart/USDCNY=X, got %s", got)
		}
		if got := r.URL.Query().Get("interval"); got != "1d" {
			t.Fatalf("expected interval=1d, got %q", got)
		}
		if got := r.URL.Query().Get("range"); got != "1d" {
			t.Fatalf("expected range=1d, got %q", got)
		}
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Fatalf("expected User-Agent header")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"chart": {
				"result": [
					{
						"meta": {
							"regularMarketPrice": 6.8942
						},
						"indicators": {
							"quote": [
								{
									"close": [6.8851, 6.8942]
								}
							]
						}
					}
				],
				"error": null
			}
		}`))
	}))
	defer server.Close()

	adapter := &YahooForexAdapter{
		client:        &http.Client{Timeout: 2 * time.Second},
		chartEndpoint: server.URL + "/v8/finance/chart/%s=X?interval=1d&range=1d",
	}

	rate, err := adapter.GetRate(context.Background(), "USD", "CNY")
	if err != nil {
		t.Fatalf("GetRate returned error: %v", err)
	}

	if rate != 6.8942 {
		t.Fatalf("expected rate 6.8942, got %f", rate)
	}
}

func TestYahooForexAdapterGetRateFallsBackToClose(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"chart": {
				"result": [
					{
						"meta": {
							"regularMarketPrice": 0
						},
						"indicators": {
							"quote": [
								{
									"close": [0, 6.9012]
								}
							]
						}
					}
				],
				"error": null
			}
		}`))
	}))
	defer server.Close()

	adapter := &YahooForexAdapter{
		client:        &http.Client{Timeout: 2 * time.Second},
		chartEndpoint: server.URL + "/v8/finance/chart/%s=X?interval=1d&range=1d",
	}

	rate, err := adapter.GetRate(context.Background(), "USD", "CNY")
	if err != nil {
		t.Fatalf("GetRate returned error: %v", err)
	}

	if rate != 6.9012 {
		t.Fatalf("expected fallback close rate 6.9012, got %f", rate)
	}
}
