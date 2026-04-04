package forex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const yahooChartEndpoint = "https://query1.finance.yahoo.com/v8/finance/chart/%s=X?interval=1d&range=1d"

// YahooForexAdapter fetches FX rates from Yahoo Finance chart API.
type YahooForexAdapter struct {
	client        *http.Client
	chartEndpoint string
}

func NewYahooForexAdapter() *YahooForexAdapter {
	return &YahooForexAdapter{
		client:        &http.Client{Timeout: 10 * time.Second},
		chartEndpoint: yahooChartEndpoint,
	}
}

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
			} `json:"meta"`
			Indicators struct {
				Quote []struct {
					Close []float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

func (a *YahooForexAdapter) GetRate(ctx context.Context, from, to string) (float64, error) {
	pair := strings.ToUpper(from) + strings.ToUpper(to)
	url := fmt.Sprintf(a.chartEndpoint, pair)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	// Yahoo quote endpoints are stricter now; chart works reliably with browser-like headers.
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")

	resp, err := a.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("yahoo finance api returned status: %d: %s", resp.StatusCode, responseSnippet(body))
	}

	var data yahooChartResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, err
	}

	if data.Chart.Error != nil {
		return 0, fmt.Errorf("yahoo finance api error: %s - %s", data.Chart.Error.Code, data.Chart.Error.Description)
	}

	if len(data.Chart.Result) == 0 {
		return 0, fmt.Errorf("yahoo finance api returned no result")
	}

	result := data.Chart.Result[0]
	if result.Meta.RegularMarketPrice > 0 {
		return result.Meta.RegularMarketPrice, nil
	}

	for _, quote := range result.Indicators.Quote {
		for i := len(quote.Close) - 1; i >= 0; i-- {
			if quote.Close[i] > 0 {
				return quote.Close[i], nil
			}
		}
	}

	return 0, fmt.Errorf("yahoo finance api returned no usable close price")
}

func responseSnippet(body []byte) string {
	snippet := strings.TrimSpace(string(body))
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
