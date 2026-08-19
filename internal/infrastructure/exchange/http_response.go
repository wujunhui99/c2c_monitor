package exchange

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

const maxExchangeResponseBytes int64 = 2 << 20

func readExchangeResponse(body io.Reader) ([]byte, error) {
	limited := &io.LimitedReader{R: body, N: maxExchangeResponseBytes + 1}
	payload, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > maxExchangeResponseBytes {
		return nil, fmt.Errorf("exchange response exceeds %d bytes", maxExchangeResponseBytes)
	}
	return payload, nil
}

func decodeExchangeResponse(body io.Reader, target any) error {
	payload, err := readExchangeResponse(body)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, target)
}

func parseFiniteFloat(raw string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("numeric value is not finite")
	}
	return value, nil
}
