package exchange

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	"c2c_monitor/internal/domain"
)

type OKXAdapter struct {
	client   *http.Client
	endpoint string
}

const okxC2CEndpoint = "https://www.okx.com/v3/c2c/tradingOrders/books"

func NewOKXAdapter() *OKXAdapter {
	return &OKXAdapter{
		client:   &http.Client{Timeout: 10 * time.Second},
		endpoint: okxC2CEndpoint,
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
	Price                  string   `json:"price"`
	AvailableAmount        string   `json:"availableAmount"`
	QuoteMinAmountPerOrder string   `json:"quoteMinAmountPerOrder"`
	QuoteMaxAmountPerOrder string   `json:"quoteMaxAmountPerOrder"`
	NickName               string   `json:"nickName"`
	MerchantId             string   `json:"merchantId"`
	PaymentMethods         []string `json:"paymentMethods"`
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

	requestURL, err := url.Parse(a.endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse OKX endpoint: %w", err)
	}
	query := requestURL.Query()
	query.Set("quoteCurrency", fiat)
	query.Set("baseCurrency", symbol)
	query.Set("side", okxSide)
	query.Set("paymentMethod", "all")
	query.Set("userType", "all")
	query.Set("showTrade", "false")
	query.Set("showFollow", "false")
	query.Set("showAlreadyTraded", "false")
	query.Set("isHideHk", "false")
	query.Set("limit", "20")
	requestURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
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
	if err := decodeExchangeResponse(resp.Body, &data); err != nil {
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

	for _, ad := range ads {
		price, err := parseFiniteFloat(ad.Price)
		if err != nil || price <= 0 {
			continue
		}
		minAmount, err := parseFiniteFloat(ad.QuoteMinAmountPerOrder)
		if err != nil || minAmount < 0 {
			continue
		}
		maxAmount, err := parseFiniteFloat(ad.QuoteMaxAmountPerOrder)
		if err != nil || maxAmount < minAmount {
			continue
		}
		availableAmount, err := parseFiniteFloat(ad.AvailableAmount)
		if err != nil || availableAmount < 0 {
			continue
		}

		payMethodsStr := domain.JoinNormalizedPayMethods(ad.PaymentMethods)

		point := domain.PricePoint{
			Exchange:        domain.ExchangeOKX,
			Symbol:          symbol,
			Fiat:            fiat,
			Side:            side,
			TargetAmount:    amount,
			Rank:            0, // Will be assigned later
			Price:           price,
			Merchant:        ad.NickName,
			MerchantID:      ad.MerchantId,
			CreatedAt:       time.Now(),
			MinAmount:       minAmount,
			MaxAmount:       maxAmount,
			AvailableAmount: availableAmount,
			PayMethods:      payMethodsStr,
		}

		// Filter by amount (CNY)
		if amount > 0 {
			if amount < minAmount || amount > maxAmount {
				continue
			}
		}
		points = append(points, point)
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

	if len(points) > 1 {
		points = points[:1]
	}

	return points, nil
}
