package domain

import (
	"reflect"
	"testing"
)

func TestNormalizeExchangeNames(t *testing.T) {
	got, err := NormalizeExchangeNames([]string{"binance", "BitGet", "OKX", "gate", "Binance"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	want := []string{ExchangeBinance, ExchangeBitget, ExchangeOKX, ExchangeGate}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestNormalizeExchangeNameRejectsUnsupportedValue(t *testing.T) {
	if _, err := NormalizeExchangeName("kraken"); err == nil {
		t.Fatal("expected unsupported exchange error")
	}
}
