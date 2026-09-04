package domain

import (
	"fmt"
	"strings"
)

const (
	ExchangeBinance = "Binance"
	ExchangeBitget  = "Bitget"
	ExchangeGate    = "Gate"
	ExchangeOKX     = "OKX"
)

var supportedExchanges = []string{
	ExchangeBinance,
	ExchangeBitget,
	ExchangeGate,
	ExchangeOKX,
}

var exchangeAliases = map[string]string{
	"binance": ExchangeBinance,
	"bitget":  ExchangeBitget,
	"gate":    ExchangeGate,
	"okx":     ExchangeOKX,
}

func SupportedExchangeNames() []string {
	return append([]string(nil), supportedExchanges...)
}

func NormalizeExchangeName(name string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if normalized, ok := exchangeAliases[key]; ok {
		return normalized, nil
	}

	return "", fmt.Errorf("unsupported exchange %q (supported: %s)", name, strings.Join(supportedExchanges, ", "))
}

func NormalizeExchangeNames(names []string) ([]string, error) {
	result := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))

	for _, name := range names {
		normalized, err := NormalizeExchangeName(name)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[normalized]; exists {
			continue
		}

		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result, nil
}

func ExchangeResponseKey(name string) string {
	if name == ExchangeOKX {
		return "okx"
	}
	return strings.ToLower(name)
}
