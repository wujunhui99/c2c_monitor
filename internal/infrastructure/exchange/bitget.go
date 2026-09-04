package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"c2c_monitor/internal/domain"
)

const bitgetC2CEndpoint = "https://www.bitget.com/v1/p2p/pub/adv/queryAdvList"

type BitgetAdapter struct {
	client   *http.Client
	endpoint string
}

func NewBitgetAdapter() *BitgetAdapter {
	return &BitgetAdapter{
		client:   &http.Client{Timeout: 10 * time.Second},
		endpoint: bitgetC2CEndpoint,
	}
}

type BitgetRequest struct {
	Side                  int    `json:"side"`
	PageNo                int    `json:"pageNo"`
	PageSize              int    `json:"pageSize"`
	CoinCode              string `json:"coinCode"`
	FiatCode              string `json:"fiatCode"`
	Price                 string `json:"price,omitempty"`
	OrderBy               int    `json:"orderBy"`
	AdAreaID              int    `json:"adAreaId"`
	AttentionMerchantFlag bool   `json:"attentionMerchantFlag"`
	RookieFriendlyFlag    bool   `json:"rookieFriendlyFlag"`
	CertifiedState        bool   `json:"certifiedState"`
	AllowPlaceOrderFlag   string `json:"allowPlaceOrderFlag"`
	LanguageType          int    `json:"languageType"`
}

type BitgetResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		DataList []BitgetAd `json:"dataList"`
	} `json:"data"`
}

type BitgetAd struct {
	AdNo          string  `json:"adNo"`
	Amount        string  `json:"amount"`
	CoinCode      string  `json:"coinCode"`
	EncryptUserID string  `json:"encryptUserId"`
	FiatCode      string  `json:"fiatCode"`
	MaxAmount     string  `json:"maxAmount"`
	MinAmount     string  `json:"minAmount"`
	NickName      string  `json:"nickName"`
	Price         string  `json:"price"`
	PriceValue    float64 `json:"priceValue"`
	PaymethodInfo []struct {
		PaymethodName string `json:"paymethodName"`
	} `json:"paymethodInfo"`
}

func (a *BitgetAdapter) GetTopPrices(ctx context.Context, symbol, fiat, side string, amount float64) ([]domain.PricePoint, error) {
	bitgetSide, err := bitgetRequestSide(side)
	if err != nil {
		return nil, err
	}

	payload := BitgetRequest{
		Side:                  bitgetSide,
		PageNo:                1,
		PageSize:              20,
		CoinCode:              symbol,
		FiatCode:              fiat,
		OrderBy:               1,
		AdAreaID:              0,
		AttentionMerchantFlag: false,
		RookieFriendlyFlag:    false,
		CertifiedState:        false,
		AllowPlaceOrderFlag:   "1",
		LanguageType:          1,
	}
	if amount > 0 {
		payload.Price = strconv.FormatFloat(amount, 'f', -1, 64)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal Bitget request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://www.bitget.com")
	req.Header.Set("Referer", fmt.Sprintf("https://www.bitget.com/zh-CN/p2p-trade?paymethodIds=-1&fiatName=%s", fiat))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bitget api returned status: %d", resp.StatusCode)
	}

	var data BitgetResponse
	if err := decodeExchangeResponse(resp.Body, &data); err != nil {
		return nil, err
	}
	if data.Code != "00000" {
		return nil, fmt.Errorf("bitget api error: %s - %s", data.Code, data.Msg)
	}

	now := time.Now()
	points := make([]domain.PricePoint, 0, len(data.Data.DataList))
	for _, ad := range data.Data.DataList {
		point, ok := bitgetAdToPoint(ad, symbol, fiat, side, amount, now)
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

func bitgetRequestSide(side string) (int, error) {
	switch side {
	case "BUY":
		return 1, nil
	case "SELL":
		return 2, nil
	default:
		return 0, fmt.Errorf("invalid side: %s", side)
	}
}

func bitgetAdToPoint(ad BitgetAd, symbol, fiat, side string, targetAmount float64, now time.Time) (domain.PricePoint, bool) {
	if ad.CoinCode != "" && !strings.EqualFold(ad.CoinCode, symbol) {
		return domain.PricePoint{}, false
	}
	if ad.FiatCode != "" && !strings.EqualFold(ad.FiatCode, fiat) {
		return domain.PricePoint{}, false
	}

	priceRaw := ad.Price
	if strings.TrimSpace(priceRaw) == "" && ad.PriceValue > 0 {
		priceRaw = strconv.FormatFloat(ad.PriceValue, 'f', -1, 64)
	}
	price, err := parseFiniteFloat(priceRaw)
	if err != nil || price <= 0 {
		return domain.PricePoint{}, false
	}
	minAmount, err := parseFiniteFloat(ad.MinAmount)
	if err != nil || minAmount < 0 {
		return domain.PricePoint{}, false
	}
	maxAmount, err := parseFiniteFloat(ad.MaxAmount)
	if err != nil || maxAmount < minAmount {
		return domain.PricePoint{}, false
	}
	availableAmount, err := parseFiniteFloat(ad.Amount)
	if err != nil || availableAmount < 0 {
		return domain.PricePoint{}, false
	}
	if targetAmount > 0 && (targetAmount < minAmount || targetAmount > maxAmount) {
		return domain.PricePoint{}, false
	}

	merchant := strings.TrimSpace(ad.NickName)
	if merchant == "" {
		merchant = strings.TrimSpace(ad.EncryptUserID)
	}
	if merchant == "" {
		merchant = strings.TrimSpace(ad.AdNo)
	}
	merchantID := strings.TrimSpace(ad.EncryptUserID)
	if merchantID == "" {
		merchantID = strings.TrimSpace(ad.AdNo)
	}

	payMethods := make([]string, 0, len(ad.PaymethodInfo))
	for _, method := range ad.PaymethodInfo {
		payMethods = append(payMethods, method.PaymethodName)
	}

	return domain.PricePoint{
		Exchange:        domain.ExchangeBitget,
		Symbol:          symbol,
		Fiat:            fiat,
		Side:            side,
		TargetAmount:    targetAmount,
		Price:           price,
		Merchant:        merchant,
		MerchantID:      merchantID,
		CreatedAt:       now,
		MinAmount:       minAmount,
		MaxAmount:       maxAmount,
		AvailableAmount: availableAmount,
		PayMethods:      domain.JoinNormalizedPayMethods(payMethods),
	}, true
}
