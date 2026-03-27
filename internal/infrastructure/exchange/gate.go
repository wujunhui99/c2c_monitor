package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"c2c_monitor/internal/domain"
)

const gateC2CEndpoint = "https://www.gate.com/api/web/v1/c2c/advertisements"

type GateAdapter struct {
	client   *http.Client
	endpoint string
}

func NewGateAdapter() *GateAdapter {
	return &GateAdapter{
		client:   &http.Client{Timeout: 10 * time.Second},
		endpoint: gateC2CEndpoint,
	}
}

type GateResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Lists []GateAd `json:"lists"`
	} `json:"data"`
}

type GateAd struct {
	Type        string `json:"type"`
	Rate        string `json:"rate"`
	Amount      string `json:"amount"`
	Total       string `json:"total"`
	LimitFiat   string `json:"limit_fiat"`
	MinAmount   string `json:"min_amount"`
	MaxAmount   string `json:"max_amount"`
	PayTypeNum  string `json:"pay_type_num"`
	OID         string `json:"oid"`
	UID         string `json:"uid"`
	Username    string `json:"username"`
	Nick        string `json:"nick"`
	HidePayment string `json:"hide_payment"`
}

func (a *GateAdapter) GetTopPrices(ctx context.Context, symbol, fiat, side string, amount float64) ([]domain.PricePoint, error) {
	pushType, err := gatePushType(side)
	if err != nil {
		return nil, err
	}

	form := buildGateForm(symbol, fiat, amount, pushType)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://www.gate.com")
	req.Header.Set("Referer", "https://www.gate.com/zh/p2p")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("X-Page-Host", "www.gate.com")
	req.Header.Set("csrftoken", "1")
	req.Header.Set("sub_website_id", "")
	req.Header.Set("Cookie", fmt.Sprintf("lang=cn; seo_lang=%%2Fzh; lasturl=%%2Fp2p; defaultP2PFiat=%s", fiat))

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gate api returned status: %d: %s", resp.StatusCode, gateResponseSnippet(payload))
	}

	var data GateResponse
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, err
	}

	if data.Code != 0 {
		return nil, fmt.Errorf("gate api error: %d - %s", data.Code, data.Message)
	}

	now := time.Now()
	points := make([]domain.PricePoint, 0, len(data.Data.Lists))
	for _, ad := range data.Data.Lists {
		point, ok := gateAdToPoint(ad, symbol, fiat, side, amount, now)
		if ok {
			points = append(points, point)
		}
	}

	if side == "BUY" {
		sort.Slice(points, func(i, j int) bool {
			return points[i].Price < points[j].Price
		})
	} else {
		sort.Slice(points, func(i, j int) bool {
			return points[i].Price > points[j].Price
		})
	}

	for i := range points {
		points[i].Rank = i + 1
	}

	if len(points) > 1 {
		points = points[:1]
	}

	return points, nil
}

func gatePushType(side string) (string, error) {
	switch side {
	case "BUY":
		return "sell", nil
	case "SELL":
		return "buy", nil
	default:
		return "", fmt.Errorf("invalid side: %s", side)
	}
}

func buildGateForm(symbol, fiat string, amount float64, pushType string) url.Values {
	form := url.Values{}
	form.Set("type", "push_order_list")
	form.Set("asset_pair", fmt.Sprintf("%s_%s", symbol, fiat))
	form.Set("big_trade", "0")
	form.Set("amount", "")
	form.Set("pay_type", "")
	form.Set("is_blue", "0")
	form.Set("is_crown", "0")
	form.Set("is_shield", "0")
	form.Set("is_follow", "0")
	form.Set("have_traded", "0")
	form.Set("no_query_hide", "0")
	form.Set("per_page", "20")
	form.Set("push_type", pushType)
	form.Set("sort_type", "1")
	form.Set("page", "1")
	form.Set("use_coupon", "0")

	if amount > 0 {
		form.Set("fiat_amount", strconv.FormatFloat(amount, 'f', -1, 64))
		form.Set("remove_limit", "1")
	} else {
		form.Set("fiat_amount", "")
		form.Set("remove_limit", "0")
	}

	return form
}

func gateAdToPoint(ad GateAd, symbol, fiat, side string, targetAmount float64, now time.Time) (domain.PricePoint, bool) {
	price, err := parseGateFloat(ad.Rate)
	if err != nil || price <= 0 {
		return domain.PricePoint{}, false
	}

	minFiat, maxFiat, err := gateFiatRange(ad, price)
	if err != nil {
		return domain.PricePoint{}, false
	}

	if targetAmount > 0 && (targetAmount < minFiat || targetAmount > maxFiat) {
		return domain.PricePoint{}, false
	}

	availableAmount, _ := parseGateFloat(ad.Amount)

	merchant := strings.TrimSpace(ad.Nick)
	if merchant == "" {
		merchant = strings.TrimSpace(ad.Username)
	}
	if merchant == "" {
		merchant = ad.UID
	}

	merchantID := strings.TrimSpace(ad.UID)
	if merchantID == "" {
		merchantID = strings.TrimSpace(ad.OID)
	}

	return domain.PricePoint{
		Exchange:        "Gate",
		Symbol:          symbol,
		Fiat:            fiat,
		Side:            side,
		TargetAmount:    targetAmount,
		Price:           price,
		Merchant:        merchant,
		MerchantID:      merchantID,
		CreatedAt:       now,
		MinAmount:       minFiat,
		MaxAmount:       maxFiat,
		AvailableAmount: availableAmount,
		PayMethods:      normalizeGatePayMethods(ad.PayTypeNum, ad.HidePayment),
	}, true
}

func gateFiatRange(ad GateAd, price float64) (float64, float64, error) {
	if strings.TrimSpace(ad.LimitFiat) != "" {
		return parseGateLimitFiat(ad.LimitFiat)
	}

	minCrypto, err := parseGateFloat(ad.MinAmount)
	if err != nil {
		return 0, 0, err
	}

	maxCrypto, err := parseGateFloat(ad.MaxAmount)
	if err != nil {
		return 0, 0, err
	}

	return minCrypto * price, maxCrypto * price, nil
}

func parseGateLimitFiat(raw string) (float64, float64, error) {
	parts := strings.Split(raw, "~")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid limit_fiat: %s", raw)
	}

	minAmount, err := parseGateFloat(parts[0])
	if err != nil {
		return 0, 0, err
	}

	maxAmount, err := parseGateFloat(parts[1])
	if err != nil {
		return 0, 0, err
	}

	return minAmount, maxAmount, nil
}

func parseGateFloat(raw string) (float64, error) {
	cleaned := strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if cleaned == "" {
		return 0, fmt.Errorf("empty numeric value")
	}
	return strconv.ParseFloat(cleaned, 64)
}

func normalizeGatePayMethods(payTypeNum, hidePayment string) string {
	if strings.TrimSpace(hidePayment) == "1" {
		return "Hidden"
	}

	switch strings.TrimSpace(payTypeNum) {
	case "", "0":
		return ""
	case "255":
		return "Multiple"
	default:
		return strings.ReplaceAll(payTypeNum, ",", ", ")
	}
}

func gateResponseSnippet(payload []byte) string {
	snippet := strings.TrimSpace(string(payload))
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	snippet = strings.ReplaceAll(snippet, "\t", " ")
	if len(snippet) > 180 {
		return snippet[:180] + "..."
	}
	if snippet == "" {
		return "empty response"
	}
	return snippet
}
