package forex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenERAdapterGetRate(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v6/latest/USD" {
			t.Fatalf("expected path /v6/latest/USD, got %s", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"result": "success",
			"base_code": "USD",
			"rates": {
				"CNY": 6.891216
			}
		}`))
	}))
	defer server.Close()

	adapter := &OpenERAdapter{
		client:         &http.Client{Timeout: 2 * time.Second},
		latestEndpoint: server.URL + "/v6/latest/%s",
	}

	rate, err := adapter.GetRate(context.Background(), "USD", "CNY")
	if err != nil {
		t.Fatalf("GetRate returned error: %v", err)
	}

	if rate != 6.891216 {
		t.Fatalf("expected rate 6.891216, got %f", rate)
	}
}

func TestOpenERAdapterRejectsMissingTargetRate(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"success","rates":{"USD":1}}`))
	}))
	defer server.Close()

	adapter := &OpenERAdapter{
		client:         &http.Client{Timeout: 2 * time.Second},
		latestEndpoint: server.URL + "/v6/latest/%s",
	}

	if _, err := adapter.GetRate(context.Background(), "USD", "CNY"); err == nil {
		t.Fatal("expected error when target rate is missing")
	}
}
