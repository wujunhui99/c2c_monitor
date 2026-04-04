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

const hexaRateLatestEndpoint = "https://hexarate.paikama.co/api/rates/latest/%s?target=%s"

// HexaRateAdapter fetches FX reference rates from HexaRate.
type HexaRateAdapter struct {
	client         *http.Client
	latestEndpoint string
}

func NewHexaRateAdapter() *HexaRateAdapter {
	return &HexaRateAdapter{
		client:         &http.Client{Timeout: 10 * time.Second},
		latestEndpoint: hexaRateLatestEndpoint,
	}
}

func (a *HexaRateAdapter) SourceName() string {
	return "HexaRate"
}

type hexaRateResponse struct {
	StatusCode int `json:"status_code"`
	Data       struct {
		Mid float64 `json:"mid"`
	} `json:"data"`
}

func (a *HexaRateAdapter) GetRate(ctx context.Context, from, to string) (float64, error) {
	url := fmt.Sprintf(a.latestEndpoint, strings.ToUpper(from), strings.ToUpper(to))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

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
		return 0, fmt.Errorf("HexaRate returned status %d: %s", resp.StatusCode, responseSnippet(body))
	}

	var data hexaRateResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, err
	}

	if data.StatusCode != 0 && data.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HexaRate returned status_code %d", data.StatusCode)
	}

	if data.Data.Mid <= 0 {
		return 0, fmt.Errorf("HexaRate returned no usable mid rate")
	}

	return data.Data.Mid, nil
}
