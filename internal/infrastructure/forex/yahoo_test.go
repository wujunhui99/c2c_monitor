package forex

import (
	"context"
	"testing"
	"time"
)

func TestYahooForexAdapter_GetRate(t *testing.T) {
	adapter := NewYahooForexAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test getting USD to CNY rate
	rate, err := adapter.GetRate(ctx, "USD", "CNY")
	if err != nil {
		t.Fatalf("Failed to get forex rate: %v", err)
	}

	if rate <= 0 {
		t.Errorf("Expected positive rate, got %f", rate)
	}

	t.Logf("Successfully fetched USD/CNY rate: %f", rate)
}
