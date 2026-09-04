package exchange

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBitgetAdapterFiltersByAmountAndSorts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Origin") != "https://www.bitget.com" {
			t.Fatalf("unexpected Origin header: %q", r.Header.Get("Origin"))
		}

		var request BitgetRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Side != 1 || request.CoinCode != "USDT" || request.FiatCode != "CNY" ||
			request.Price != "1000" || request.PageSize != 20 || request.AllowPlaceOrderFlag != "1" {
			t.Fatalf("unexpected request: %#v", request)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": "00000",
			"msg": "success",
			"data": {
				"dataList": [
					{
						"adNo": "too-large",
						"amount": "1000",
						"coinCode": "USDT",
						"fiatCode": "CNY",
						"encryptUserId": "merchant-a",
						"minAmount": "2000",
						"maxAmount": "5000",
						"nickName": "Too Large",
						"price": "6.69",
						"paymethodInfo": []
					},
					{
						"adNo": "valid-expensive",
						"amount": "900",
						"coinCode": "USDT",
						"fiatCode": "CNY",
						"encryptUserId": "merchant-b",
						"minAmount": "100",
						"maxAmount": "5000",
						"nickName": "Merchant B",
						"price": "6.72",
						"paymethodInfo": [{"paymethodName": "银行卡"}]
					},
					{
						"adNo": "valid-lowest",
						"amount": "800",
						"coinCode": "USDT",
						"fiatCode": "CNY",
						"encryptUserId": "merchant-c",
						"minAmount": "300",
						"maxAmount": "5000",
						"nickName": "Merchant C",
						"price": "6.70",
						"paymethodInfo": [
							{"paymethodName": "微信支付"},
							{"paymethodName": "支付宝"}
						]
					}
				]
			}
		}`))
	}))
	defer server.Close()

	adapter := &BitgetAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}
	points, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 1000)
	if err != nil {
		t.Fatalf("GetTopPrices returned error: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected one point, got %#v", points)
	}
	point := points[0]
	if point.Exchange != "Bitget" || point.Price != 6.70 || point.Merchant != "Merchant C" ||
		point.MerchantID != "merchant-c" || point.MinAmount != 300 || point.MaxAmount != 5000 ||
		point.AvailableAmount != 800 || point.Rank != 1 {
		t.Fatalf("unexpected point: %#v", point)
	}
	if point.PayMethods != "微信, 支付宝" {
		t.Fatalf("unexpected payment methods: %q", point.PayMethods)
	}
}

func TestBitgetAdapterMapsSellSideAndOmitsZeroAmount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request BitgetRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Side != 2 || request.Price != "" {
			t.Fatalf("unexpected request: %#v", request)
		}
		_, _ = w.Write([]byte(`{
			"code": "00000",
			"data": {
				"dataList": [
					{"amount":"100","minAmount":"10","maxAmount":"10000","price":"6.65","nickName":"A"},
					{"amount":"100","minAmount":"10","maxAmount":"10000","priceValue":6.66,"nickName":"B"}
				]
			}
		}`))
	}))
	defer server.Close()

	adapter := &BitgetAdapter{client: &http.Client{Timeout: 2 * time.Second}, endpoint: server.URL}
	points, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "SELL", 0)
	if err != nil {
		t.Fatalf("GetTopPrices returned error: %v", err)
	}
	if len(points) != 1 || points[0].Price != 6.66 || points[0].Merchant != "B" {
		t.Fatalf("expected highest sell-side price, got %#v", points)
	}
}

func TestBitgetAdapterReturnsUpstreamErrors(t *testing.T) {
	t.Run("api error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"code":"40017","msg":"invalid fiat"}`))
		}))
		defer server.Close()

		adapter := &BitgetAdapter{client: &http.Client{Timeout: 2 * time.Second}, endpoint: server.URL}
		_, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 1000)
		if err == nil || !strings.Contains(err.Error(), "invalid fiat") {
			t.Fatalf("expected Bitget API error, got %v", err)
		}
	})

	t.Run("http error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		}))
		defer server.Close()

		adapter := &BitgetAdapter{client: &http.Client{Timeout: 2 * time.Second}, endpoint: server.URL}
		_, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 1000)
		if err == nil || !strings.Contains(err.Error(), "503") {
			t.Fatalf("expected Bitget HTTP error, got %v", err)
		}
	})

	t.Run("malformed response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not-json`))
		}))
		defer server.Close()

		adapter := &BitgetAdapter{client: &http.Client{Timeout: 2 * time.Second}, endpoint: server.URL}
		if _, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 1000); err == nil {
			t.Fatal("expected malformed response error")
		}
	})

	t.Run("oversized response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, strings.Repeat("x", int(maxExchangeResponseBytes)+1))
		}))
		defer server.Close()

		adapter := &BitgetAdapter{client: &http.Client{Timeout: 2 * time.Second}, endpoint: server.URL}
		_, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 1000)
		if err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("expected oversized response error, got %v", err)
		}
	})
}

func TestBitgetAdapterRejectsInvalidSide(t *testing.T) {
	adapter := NewBitgetAdapter()
	if _, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "INVALID", 1000); err == nil {
		t.Fatal("expected invalid side error")
	}
}
