package exchange

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBinanceAdapterParsesAndSortsPrices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": "000000",
			"data": [
				{
					"adv": {
						"price": "7.12",
						"surplusAmount": "1000",
						"minSingleTransAmount": "100",
						"maxSingleTransAmount": "5000",
						"tradeMethods": [{"tradeMethodName": "Bank Transfer"}]
					},
					"advertiser": {"nickName": "Merchant A", "userNo": "a"}
				},
				{
					"adv": {
						"price": "7.08",
						"surplusAmount": "2000",
						"minSingleTransAmount": "50",
						"maxSingleTransAmount": "5000",
						"tradeMethods": [{"tradeMethodName": "Alipay"}]
					},
					"advertiser": {"nickName": "Merchant B", "userNo": "b"}
				}
			]
		}`))
	}))
	defer server.Close()

	adapter := &BinanceAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}
	points, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 500)
	if err != nil {
		t.Fatalf("GetTopPrices returned error: %v", err)
	}
	if len(points) != 1 || points[0].Price != 7.08 || points[0].Merchant != "Merchant B" {
		t.Fatalf("unexpected points: %#v", points)
	}
	if points[0].PayMethods != "支付宝" {
		t.Fatalf("expected normalized pay method, got %q", points[0].PayMethods)
	}
}

func TestBinanceAdapterReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"100001","message":"rate limited"}`))
	}))
	defer server.Close()

	adapter := &BinanceAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}
	_, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 500)
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("expected Binance API error, got %v", err)
	}
}

func TestBinanceAdapterRejectsMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	adapter := &BinanceAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}
	if _, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 500); err == nil {
		t.Fatal("expected malformed response error")
	}
}

func TestBinanceAdapterSkipsNonFiniteCandidate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": "000000",
			"data": [
				{
					"adv": {
						"price": "NaN",
						"surplusAmount": "1000",
						"minSingleTransAmount": "100",
						"maxSingleTransAmount": "5000",
						"tradeMethods": []
					},
					"advertiser": {"nickName": "Invalid", "userNo": "invalid"}
				},
				{
					"adv": {
						"price": "7.08",
						"surplusAmount": "1000",
						"minSingleTransAmount": "100",
						"maxSingleTransAmount": "5000",
						"tradeMethods": []
					},
					"advertiser": {"nickName": "Valid", "userNo": "valid"}
				}
			]
		}`))
	}))
	defer server.Close()

	adapter := &BinanceAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}
	points, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 500)
	if err != nil {
		t.Fatalf("GetTopPrices returned error: %v", err)
	}
	if len(points) != 1 || points[0].Merchant != "Valid" {
		t.Fatalf("expected valid finite candidate, got %#v", points)
	}
}

func TestBinanceAdapterFindsLaterAmountMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": "000000",
			"data": [
				{
					"adv": {
						"price": "7.01",
						"surplusAmount": "1000",
						"minSingleTransAmount": "1000",
						"maxSingleTransAmount": "5000",
						"tradeMethods": []
					},
					"advertiser": {"nickName": "Too Large", "userNo": "large"}
				},
				{
					"adv": {
						"price": "7.05",
						"surplusAmount": "1000",
						"minSingleTransAmount": "100",
						"maxSingleTransAmount": "5000",
						"tradeMethods": []
					},
					"advertiser": {"nickName": "Matched", "userNo": "matched"}
				}
			]
		}`))
	}))
	defer server.Close()

	adapter := &BinanceAdapter{
		client:   &http.Client{Timeout: 2 * time.Second},
		endpoint: server.URL,
	}
	points, err := adapter.GetTopPrices(context.Background(), "USDT", "CNY", "BUY", 500)
	if err != nil {
		t.Fatalf("GetTopPrices returned error: %v", err)
	}
	if len(points) != 1 || points[0].Merchant != "Matched" {
		t.Fatalf("expected later amount match, got %#v", points)
	}
}
