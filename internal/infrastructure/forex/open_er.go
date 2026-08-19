package forex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const openERLatestEndpoint = "https://open.er-api.com/v6/latest/%s"

// OpenERAdapter fetches FX reference rates from open.er-api.com.
type OpenERAdapter struct {
	client         *http.Client
	latestEndpoint string
}

func NewOpenERAdapter() *OpenERAdapter {
	return &OpenERAdapter{
		client:         &http.Client{Timeout: 10 * time.Second},
		latestEndpoint: openERLatestEndpoint,
	}
}

func (a *OpenERAdapter) SourceName() string {
	return "open.er-api"
}

type openERResponse struct {
	Result    string             `json:"result"`
	ErrorType string             `json:"error-type"`
	Rates     map[string]float64 `json:"rates"`
}

func (a *OpenERAdapter) GetRate(ctx context.Context, from, to string) (float64, error) {
	url := fmt.Sprintf(a.latestEndpoint, strings.ToUpper(from))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("open.er-api returned status %d: %s", resp.StatusCode, responseSnippet(body))
	}

	var data openERResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, err
	}

	if data.Result != "" && data.Result != "success" {
		if data.ErrorType != "" {
			return 0, fmt.Errorf("open.er-api error: %s", data.ErrorType)
		}
		return 0, fmt.Errorf("open.er-api returned result %q", data.Result)
	}

	rate := data.Rates[strings.ToUpper(to)]
	if rate <= 0 {
		return 0, fmt.Errorf("open.er-api returned no usable %s rate", strings.ToUpper(to))
	}

	return rate, nil
}
