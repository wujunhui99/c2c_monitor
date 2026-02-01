package main

import (
	"context"
	"fmt"

	"c2c_monitor/internal/infrastructure/exchange"
)

func main() {
	ctx := context.Background()
	
	// 1. Test Binance
	fmt.Println("=== Testing Binance C2C Crawler (Amount: 100,000) ===")
	bnAdapter := exchange.NewBinanceAdapter()
	bnPrices, err := bnAdapter.GetTopPrices(ctx, "USDT", "CNY", "BUY", 100000)
	if err != nil {
		fmt.Printf("❌ Binance Error: %v\n", err)
	} else {
		fmt.Printf("✅ Binance Success! Found %d prices.\n", len(bnPrices))
		for _, p := range bnPrices {
			fmt.Printf("   Rank %d: %.2f CNY\n", p.Rank, p.Price)
		}
	}

	fmt.Println()

	fmt.Println("=== Testing Gate.io C2C Crawler ===")
	gateAdapter := exchange.NewOKXAdapter()
	gatePrices, err := gateAdapter.GetTopPrices(ctx, "USDT", "CNY", "BUY", 100)
	if err != nil {
		fmt.Printf("❌ OKX Error: %v\n", err)
	} else {
		fmt.Printf("✅ OKX Success! Found %d prices.\n", len(gatePrices))
		for _, p := range gatePrices {
			fmt.Printf("   Rank %d: %.2f CNY\n", p.Rank, p.Price)
		}
	}
}
