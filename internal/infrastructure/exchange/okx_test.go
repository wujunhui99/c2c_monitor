package exchange

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOKXAdapterFindsLaterAmountMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "20" {
			t.Fatalf("expected limit=20, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 0,
			"data": {
				"sell": [
					{
						"price": "7.01",
						"availableAmount": "1000",
						"quoteMinAmountPerOrder": "1000",
						"quoteMaxAmountPerOrder": "5000",
						"nickName": "Too Large",
						"merchantId": "large",
						"paymentMethods": ["bank"]
					},
					{
						"price": "7.05",
						"availableAmount": "1000",
						"quoteMinAmountPerOrder": "100",
						"quoteMaxAmountPerOrder": "5000",
						"nickName": "Matched",
						"merchantId": "matched",
						"paymentMethods": ["alipay"]
					}
				],
				"buy": []
			}
		}`))
	}))
	defer server.Close()

	adapter := &OKXAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}
	points, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 500)
	if err != nil {
		t.Fatalf("GetTopPrices returned error: %v", err)
	}
	if len(points) != 1 || points[0].Merchant != "Matched" || points[0].Price != 7.05 {
		t.Fatalf("unexpected points: %#v", points)
	}
}

func TestOKXAdapterReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	adapter := &OKXAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}
	_, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 500)
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected OKX HTTP error, got %v", err)
	}
}

func TestOKXAdapterRejectsMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	adapter := &OKXAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}
	if _, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 500); err == nil {
		t.Fatal("expected malformed response error")
	}
}

func TestOKXAdapterSkipsInvalidAmountRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 0,
			"data": {
				"sell": [
					{
						"price": "7.01",
						"availableAmount": "1000",
						"quoteMinAmountPerOrder": "NaN",
						"quoteMaxAmountPerOrder": "5000",
						"nickName": "Invalid",
						"merchantId": "invalid",
						"paymentMethods": []
					},
					{
						"price": "7.05",
						"availableAmount": "1000",
						"quoteMinAmountPerOrder": "100",
						"quoteMaxAmountPerOrder": "5000",
						"nickName": "Valid",
						"merchantId": "valid",
						"paymentMethods": []
					}
				],
				"buy": []
			}
		}`))
	}))
	defer server.Close()

	adapter := &OKXAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}
	points, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 500)
	if err != nil {
		t.Fatalf("GetTopPrices returned error: %v", err)
	}
	if len(points) != 1 || points[0].Merchant != "Valid" {
		t.Fatalf("expected valid amount range candidate, got %#v", points)
	}
}
