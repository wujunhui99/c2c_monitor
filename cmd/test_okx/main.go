package main

import (
	"context"
	"fmt"
	"log"

	"c2c_monitor/internal/infrastructure/exchange"
)

func main() {
	adapter := exchange.NewOKXAdapter()
	ctx := context.Background()

	// Test 1: Lowest Price (Amount = 0)
	fmt.Println("--- Testing OKX Amount=0 (Lowest Price) ---")
	lowestPrices, err := adapter.GetTopPrices(ctx, "USDT", "CNY", "BUY", 0)
	if err != nil {
		log.Fatalf("Error fetching lowest prices: %v", err)
	}
	if len(lowestPrices) == 0 {
		log.Println("No prices found for amount=0")
	} else {
		bestLowestPrice := lowestPrices[0].Price
		fmt.Printf("Best Price (Amount=0): %.2f\n", bestLowestPrice)
		for _, p := range lowestPrices {
			fmt.Printf("Rank %d: %.2f (Merchant: %s)\n", p.Rank, p.Price, p.Merchant)
		}
	}

	// Test 2: Amount = 30
	fmt.Println("\n--- Testing OKX Amount=30 ---")
	tier30Prices, err := adapter.GetTopPrices(ctx, "USDT", "CNY", "BUY", 30)
	if err != nil {
		log.Fatalf("Error fetching tier 30 prices: %v", err)
	}
	if len(tier30Prices) == 0 {
		log.Println("No prices found for amount=30")
	} else {
		bestTier30Price := tier30Prices[0].Price
		fmt.Printf("Best Price (Amount=30): %.2f\n", bestTier30Price)
		for _, p := range tier30Prices {
			fmt.Printf("Rank %d: %.2f (Merchant: %s)\n", p.Rank, p.Price, p.Merchant)
		}
	}
	
	// Test 3: Amount = 5000
	fmt.Println("\n--- Testing OKX Amount=5000 ---")
	tier5000Prices, err := adapter.GetTopPrices(ctx, "USDT", "CNY", "BUY", 5000)
    if err != nil {
		log.Fatalf("Error fetching tier 5000 prices: %v", err)
	}
    if len(tier5000Prices) > 0 {
        fmt.Printf("Best Price (Amount=5000): %.2f\n", tier5000Prices[0].Price)
		for _, p := range tier5000Prices {
			fmt.Printf("Rank %d: %.2f (Merchant: %s)\n", p.Rank, p.Price, p.Merchant)
		}
    } else {
		log.Println("No prices found for amount=5000")
	}
}
