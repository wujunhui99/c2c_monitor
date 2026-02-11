package forex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// YahooForexAdapter now uses open.er-api.com as Yahoo Finance is blocking server IPs.
// The name is kept for backward compatibility with main.go injection.
type YahooForexAdapter struct {
	client *http.Client
}

func NewYahooForexAdapter() *YahooForexAdapter {
	return &YahooForexAdapter{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// OpenERResponse is the struct for open.er-api.com
type OpenERResponse struct {
	Result string             `json:"result"`
	Rates  map[string]float64 `json:"rates"`
}

func (a *YahooForexAdapter) GetRate(ctx context.Context, from, to string) (float64, error) {
	// API: https://open.er-api.com/v6/latest/USD
	url := fmt.Sprintf("https://open.er-api.com/v6/latest/%s", from)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	
	resp, err := a.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("forex api returned status: %d", resp.StatusCode)
	}

	var data OpenERResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}

	rate, ok := data.Rates[to]
	if !ok {
		return 0, fmt.Errorf("currency %s not found in rates", to)
	}

	if rate == 0 {
		return 0, fmt.Errorf("zero rate returned for %s", to)
	}

	return rate, nil
}
