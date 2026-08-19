package exchange

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGateAdapterGetTopPricesFiltersByAmount(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", r.Method)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("failed to parse request body: %v", err)
		}

		if got := values.Get("type"); got != "push_order_list" {
			t.Fatalf("expected type=push_order_list, got %q", got)
		}
		if got := values.Get("asset_pair"); got != "USDT_CNY" {
			t.Fatalf("expected asset_pair=USDT_CNY, got %q", got)
		}
		if got := values.Get("push_type"); got != "sell" {
			t.Fatalf("expected push_type=sell, got %q", got)
		}
		if got := values.Get("fiat_amount"); got != "100" {
			t.Fatalf("expected fiat_amount=100, got %q", got)
		}
		if got := values.Get("remove_limit"); got != "1" {
			t.Fatalf("expected remove_limit=1, got %q", got)
		}
		if got := r.Header.Get("X-Page-Host"); got != "www.gate.com" {
			t.Fatalf("expected X-Page-Host header, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"code": 0,
			"message": "success",
			"data": {
				"lists": [
					{
						"rate": "6.99",
						"amount": "500.00",
						"limit_fiat": "699~1398",
						"min_amount": "100",
						"max_amount": "200",
						"pay_type_num": "1",
						"uid": "uid-too-high",
						"nick": "Too High",
						"oid": "ad-1"
					},
						{
							"rate": "6.98",
							"amount": "331.96",
							"limit_fiat": "69.8~767.8",
							"min_amount": "10",
							"max_amount": "110",
							"pay_type_num": "255,1",
							"uid": "uid-match",
							"nick": "Matched Merchant",
							"oid": "ad-2"
						},
					{
						"rate": "6.97",
						"amount": "1000.00",
						"limit_fiat": "10~50",
						"min_amount": "1",
						"max_amount": "10",
						"pay_type_num": "1",
						"uid": "uid-too-low",
						"nick": "Too Low",
						"oid": "ad-3"
					}
				]
			}
		}`)
	}))
	defer server.Close()

	adapter := &GateAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}

	points, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 100)
	if err != nil {
		t.Fatalf("GetTopPrices returned error: %v", err)
	}

	if len(points) != 1 {
		t.Fatalf("expected 1 price point, got %d", len(points))
	}

	point := points[0]
	if point.Exchange != "Gate" {
		t.Fatalf("expected exchange Gate, got %q", point.Exchange)
	}
	if point.Merchant != "Matched Merchant" {
		t.Fatalf("expected merchant Matched Merchant, got %q", point.Merchant)
	}
	if point.MerchantID != "uid-match" {
		t.Fatalf("expected merchant id uid-match, got %q", point.MerchantID)
	}
	if point.PayMethods != "支付宝, 微信" {
		t.Fatalf("expected pay methods 支付宝, 微信, got %q", point.PayMethods)
	}
	if point.Rank != 1 {
		t.Fatalf("expected rank 1, got %d", point.Rank)
	}
	assertCloseFloat(t, point.Price, 6.98)
	assertCloseFloat(t, point.MinAmount, 69.8)
	assertCloseFloat(t, point.MaxAmount, 767.8)
	assertCloseFloat(t, point.AvailableAmount, 331.96)
}

func TestGateAdapterGetTopPricesLowestAmountNoFilter(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("failed to parse request body: %v", err)
		}

		if got := values.Get("fiat_amount"); got != "" {
			t.Fatalf("expected empty fiat_amount, got %q", got)
		}
		if got := values.Get("remove_limit"); got != "0" {
			t.Fatalf("expected remove_limit=0, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"code": 0,
			"message": "success",
			"data": {
				"lists": [
					{
						"rate": "6.95",
						"amount": "120.00",
						"limit_fiat": "100~834",
						"min_amount": "14.38",
						"max_amount": "120.00",
						"pay_type_num": "1",
						"uid": "uid-1",
						"nick": "Merchant 1",
						"oid": "ad-1"
					},
					{
						"rate": "6.92",
						"amount": "1200.00",
						"limit_fiat": "4844~8304",
						"min_amount": "700",
						"max_amount": "1200",
						"pay_type_num": "1",
						"uid": "uid-2",
						"nick": "Merchant 2",
						"oid": "ad-2"
					}
				]
			}
		}`)
	}))
	defer server.Close()

	adapter := &GateAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}

	points, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 0)
	if err != nil {
		t.Fatalf("GetTopPrices returned error: %v", err)
	}

	if len(points) != 1 {
		t.Fatalf("expected 1 price point, got %d", len(points))
	}

	point := points[0]
	assertCloseFloat(t, point.Price, 6.92)
	if point.Merchant != "Merchant 2" {
		t.Fatalf("expected Merchant 2, got %q", point.Merchant)
	}
}

func TestGateAdapterReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	adapter := &GateAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}

	_, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 100)
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected Gate HTTP error, got %v", err)
	}
}

func TestGateAdapterRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", int(maxExchangeResponseBytes)+1))
	}))
	defer server.Close()

	adapter := &GateAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}

	_, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 100)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func assertCloseFloat(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected %.10f, got %.10f", want, got)
	}
}
