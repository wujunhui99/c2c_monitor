package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"c2c_monitor/internal/domain"
)

type OKXAdapter struct {
	client *http.Client
}

func NewOKXAdapter() *OKXAdapter {
	return &OKXAdapter{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// OKXResponse structure
type OKXResponse struct {
	Code int `json:"code"`
	Data struct {
		Sell []OKXAd `json:"sell"`
		Buy  []OKXAd `json:"buy"`
	} `json:"data"`
	Msg string `json:"msg"`
}

type OKXAd struct {
	Price                  string `json:"price"`
	AvailableAmount        string `json:"availableAmount"`
	QuoteMinAmountPerOrder string `json:"quoteMinAmountPerOrder"`
	QuoteMaxAmountPerOrder string `json:"quoteMaxAmountPerOrder"`
	NickName               string `json:"nickName"`
	MerchantId             string `json:"merchantId"`
}

func (a *OKXAdapter) GetTopPrices(ctx context.Context, symbol, fiat, side string, amount float64) ([]domain.PricePoint, error) {
	// Map User Side to OKX Advertiser Side
	// User BUY -> Advertiser SELL
	// User SELL -> Advertiser BUY
	var okxSide string
	if side == "BUY" {
		okxSide = "sell"
	} else if side == "SELL" {
		okxSide = "buy"
	} else {
		return nil, fmt.Errorf("invalid side: %s", side)
	}

	url := fmt.Sprintf("https://www.okx.com/v3/c2c/tradingOrders/books?quoteCurrency=%s&baseCurrency=%s&side=%s&paymentMethod=all&userType=all&showTrade=false&showFollow=false&showAlreadyTraded=false&isHideHk=false&limit=50",
		fiat, symbol, okxSide)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("okx api returned status: %d", resp.StatusCode)
	}

	var data OKXResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Code != 0 {
		return nil, fmt.Errorf("okx api error code: %d, msg: %s", data.Code, data.Msg)
	}

	var ads []OKXAd
	if okxSide == "sell" {
		ads = data.Data.Sell
	} else {
		ads = data.Data.Buy
	}

	var points []domain.PricePoint
	var allPoints []domain.PricePoint // Store all ads without amount filtering

	for _, ad := range ads {
		price, _ := strconv.ParseFloat(ad.Price, 64)
		minAmount, _ := strconv.ParseFloat(ad.QuoteMinAmountPerOrder, 64)
		maxAmount, _ := strconv.ParseFloat(ad.QuoteMaxAmountPerOrder, 64)

		if price > 0 {
			point := domain.PricePoint{
				Exchange:     "OKX",
				Symbol:       symbol,
				Fiat:         fiat,
				Side:         side,
				TargetAmount: amount,
				Rank:         0, // Will be assigned later
				Price:        price,
				Merchant:     ad.NickName,
				CreatedAt:    time.Now(),
			}
			allPoints = append(allPoints, point)

			// Filter by amount (CNY)
			if amount > 0 {
				if amount < minAmount || amount > maxAmount {
					continue
				}
			}
			points = append(points, point)
		}
	}

	// If no ads match the amount filter, use all ads (OKX merchants often have high minimums)
	if len(points) == 0 && len(allPoints) > 0 {
		points = allPoints
	}

	// Sort
	if side == "BUY" {
		// User buying: want lowest price
		sort.Slice(points, func(i, j int) bool {
			return points[i].Price < points[j].Price
		})
	} else {
		// User selling: want highest price
		sort.Slice(points, func(i, j int) bool {
			return points[i].Price > points[j].Price
		})
	}

	// Assign Rank and Top N
	for i := range points {
		points[i].Rank = i + 1
	}

	if len(points) > 5 {
		points = points[:5]
	}

	return points, nil
}
