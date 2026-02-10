package domain

import (
	"context"
	"time"
)

// PricePoint represents a single C2C price record
type PricePoint struct {
	ID              int64     `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	Exchange        string    `json:"exchange"`      // e.g., "Binance", "Gate"
	Symbol          string    `json:"symbol"`        // e.g., "USDT"
	Fiat            string    `json:"fiat"`          // e.g., "CNY"
	Side            string    `json:"side"`          // "BUY" or "SELL"
	TargetAmount    float64   `json:"target_amount"` // The filtered amount tier, e.g., 100
	Rank            int       `json:"rank"`          // 1 = Best price
	Price           float64   `json:"price"`
	Merchant        string    `json:"merchant"`         // Merchant nickname
	MerchantID      string    `json:"merchant_id"`      // External Merchant ID
	PayMethods      string    `json:"pay_methods"`      // Comma separated or JSON
	MinAmount       float64   `json:"min_amount"`       // Min limit per order
	MaxAmount       float64   `json:"max_amount"`       // Max limit per order
	AvailableAmount float64   `json:"available_amount"` // Surplus amount
}

// Merchant represents a crypto merchant/advertiser
type Merchant struct {
	ID         int64     `json:"id"`
	Exchange   string    `json:"exchange"`
	MerchantID string    `json:"merchant_id"` // Unique ID on the exchange
	NickName   string    `json:"nick_name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ForexRate represents an exchange rate record
type ForexRate struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Source    string    `json:"source"` // e.g., "Yahoo"
	Pair      string    `json:"pair"`   // e.g., "USDCNY"
	Rate      float64   `json:"rate"`
}

// PriceQueryFilter defines parameters for querying history
type PriceQueryFilter struct {
	Exchange     string
	Symbol       string
	Fiat         string
	Side         string
	TargetAmount *float64
	Rank         int
	StartTime    time.Time
	EndTime      time.Time
	Limit        int
}

// ServiceStatus represents the health of a scraped service
type ServiceStatus struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"` // "OK" or "Error"
	Message   string    `json:"message"`
	LastCheck time.Time `json:"last_check"`
}

// Interfaces define the behavior of the system's dependencies

type IExchange interface {
	// GetTopPrices returns the top N prices for a specific amount tier
	GetTopPrices(ctx context.Context, symbol, fiat, side string, amount float64) ([]PricePoint, error)
}

type IForex interface {
	GetRate(ctx context.Context, from, to string) (float64, error)
}

type INotifier interface {
	Send(ctx context.Context, subject, body string) error
}

type IRepository interface {
	// Price operations
	SavePricePoints(ctx context.Context, points []*PricePoint) error
	GetPriceHistory(ctx context.Context, filter PriceQueryFilter) ([]*PricePoint, error)
	
	// Merchant operations
	SaveMerchant(ctx context.Context, merchant *Merchant) error
	
	// Forex operations
	SaveForexRate(ctx context.Context, rate *ForexRate) error
	GetLatestForexRate(ctx context.Context, pair string) (*ForexRate, error)
	GetForexHistory(ctx context.Context, pair string, start, end time.Time) ([]*ForexRate, error)
}
