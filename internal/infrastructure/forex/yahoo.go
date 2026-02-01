package forex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type YahooForexAdapter struct {
	client *http.Client
}

func NewYahooForexAdapter() *YahooForexAdapter {
	return &YahooForexAdapter{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// YahooChartResponse is a minimal struct to parse the Yahoo Finance API response
type YahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
			} `json:"meta"`
		} `json:"result"`
	} `json:"chart"`
}

func (a *YahooForexAdapter) GetRate(ctx context.Context, from, to string) (float64, error) {
	// Yahoo symbol format: "USDCNY=X"
	symbol := fmt.Sprintf("%s%s=X", from, to)
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=1d", symbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	
	// Yahoo requires User-Agent sometimes
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko)")

	resp, err := a.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("yahoo api returned status: %d", resp.StatusCode)
	}

	var data YahooChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}

	if len(data.Chart.Result) == 0 {
		return 0, fmt.Errorf("no result in yahoo api response")
	}

	price := data.Chart.Result[0].Meta.RegularMarketPrice
	if price == 0 {
		return 0, fmt.Errorf("zero price returned")
	}

	return price, nil
}
