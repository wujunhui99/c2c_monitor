package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"c2c_monitor/internal/domain"
)

type BinanceAdapter struct {
	client *http.Client
}

func NewBinanceAdapter() *BinanceAdapter {
	return &BinanceAdapter{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// BinanceRequest payload for search
type BinanceRequest struct {
	Asset         string  `json:"asset"`
	Fiat          string  `json:"fiat"`
	TradeType     string  `json:"tradeType"` // "BUY" or "SELL" (from user perspective)
	TransAmount   float64 `json:"transAmount,omitempty"`
	Order         string  `json:"order"` // "price"
	Page          int     `json:"page"`
	Rows          int     `json:"rows"`
	PayTypes      []string `json:"payTypes"` // []
}

// BinanceResponse minimal struct
type BinanceResponse struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	Data          []struct {
		Adv struct {
			AdvNo                 string `json:"advNo"`
			Price                 string `json:"price"`
			TradableQuantity      string `json:"tradableQuantity"`
		} `json:"adv"`
		Advertiser struct {
			NickName string `json:"nickName"`
		} `json:"advertiser"`
	} `json:"data"`
}

func (a *BinanceAdapter) GetTopPrices(ctx context.Context, symbol, fiat, side string, amount float64) ([]domain.PricePoint, error) {
	// Map "BUY" (User buys) -> "BUY" tradeType in Binance API (Advertiser Sells? No, Binance API "BUY" means user buys)
	// Actually check: If I want to Buy USDT, I search for Ads where tradeType="BUY".
	// Wait, usually if I want to BUY, the advertiser is SELLING.
	// In Binance P2P Web API:
	// If I click "Buy", the payload sends "tradeType": "BUY".
	
	url := "https://p2p.binance.com/bapi/c2c/v2/friendly/c2c/adv/search"

	payload := BinanceRequest{
		Asset:       symbol,
		Fiat:        fiat,
		TradeType:   side,
		TransAmount: amount,
		Order:       "",
		Page:        1,
		Rows:        5, // Fetch top 5 ads
		PayTypes:    []string{},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Clienttype", "web")
	req.Header.Set("Lang", "zh-CN") // Add language to get CN ads

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance api returned status: %d", resp.StatusCode)
	}

	var data BinanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Code != "000000" {
		return nil, fmt.Errorf("binance api error: %s - %s", data.Code, data.Message)
	}

	var points []domain.PricePoint
	
	for i, item := range data.Data {
		var price float64
		fmt.Sscanf(item.Adv.Price, "%f", &price)
		
		if price > 0 {
			// fmt.Printf("DEBUG: Binance Ad - Price: %.2f, Merchant: %s\n", price, item.Advertiser.NickName)
			points = append(points, domain.PricePoint{
				Exchange:     "Binance",
				Symbol:       symbol,
				Fiat:         fiat,
				Side:         side,
				TargetAmount: amount,
				Rank:         i + 1,
				Price:        price,
				Merchant:     item.Advertiser.NickName,
				CreatedAt:    time.Now(),
			})
		}
	}
	
	// Sort by price
	if side == "BUY" {
		sort.Slice(points, func(i, j int) bool {
			return points[i].Price < points[j].Price
		})
	} else {
		sort.Slice(points, func(i, j int) bool {
			return points[i].Price > points[j].Price
		})
	}

	// Re-assign ranks after sorting (just in case)
	for i := range points {
		points[i].Rank = i + 1
	}

	// Return top 5 as per default
	if len(points) > 5 {
		points = points[:5]
	}

	return points, nil
}
