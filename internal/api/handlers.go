package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"c2c_monitor/config"
	"c2c_monitor/internal/domain"
	"c2c_monitor/internal/service"
)

type Handler struct {
	svc *service.MonitorService
	cfg *config.Config
}

func NewHandler(svc *service.MonitorService, cfg *config.Config) *Handler {
	return &Handler{svc: svc, cfg: cfg}
}

func (h *Handler) GetHistory(c *gin.Context) {
	// Params: range (1d, 7d), amount (required), exchange (optional)
	rangeStr := c.Query("range")
	amountStr := c.Query("amount")
	
	if amountStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount parameter is required"})
		return
	}
	
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount"})
		return
	}

	// Calculate start time
	now := time.Now()
	var startTime time.Time
	switch rangeStr {
	case "1d":
		startTime = now.Add(-24 * time.Hour)
	case "7d":
		startTime = now.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = now.Add(-30 * 24 * time.Hour)
	default:
		startTime = now.Add(-24 * time.Hour) // Default 1d
	}

	// Fetch C2C History (Binance)
	// We want Rank 1 prices usually for the chart
	filter := domain.PriceQueryFilter{
		Symbol:       "USDT",
		Fiat:         "CNY",
		Side:         "BUY", // Default monitoring side
		TargetAmount: &amount,
		Rank:         1, // Only get best price for chart
		StartTime:    startTime,
		EndTime:      now,
		Limit:        5000, // Safety limit
	}
	
	// We might want multiple exchanges. The PRD says "Line 1 (Binance), Line 2 (Gate)".
	// So we need to query for each exchange.
	
	resp := gin.H{
		"forex":   []gin.H{},
		"binance": []gin.H{},
		"okx":     []gin.H{},
	}

	// 1. Forex
	forexHistory, err := h.svc.GetForexHistory(c.Request.Context(), "USDCNY", startTime, now)
	if err == nil {
		var list []gin.H
		for _, f := range forexHistory {
			list = append(list, gin.H{"t": f.CreatedAt.Unix(), "v": f.Rate})
		}
		resp["forex"] = list
	}

	// 2. Binance
	filter.Exchange = "Binance"
	binancePrices, err := h.svc.GetPriceHistory(c.Request.Context(), filter)
	if err == nil {
		var list []gin.H
		for _, p := range binancePrices {
			list = append(list, gin.H{"t": p.CreatedAt.Unix(), "v": p.Price})
		}
		resp["binance"] = list
	}

	// 3. OKX
	filter.Exchange = "OKX"
	okxPrices, err := h.svc.GetPriceHistory(c.Request.Context(), filter)
	if err == nil {
		var list []gin.H
		for _, p := range okxPrices {
			list = append(list, gin.H{"t": p.CreatedAt.Unix(), "v": p.Price})
		}
		resp["okx"] = list
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": resp})
}

func (h *Handler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.GetConfig())
}

func (h *Handler) UpdateConfig(c *gin.Context) {
	var newCfg config.MonitorConfig
	if err := c.ShouldBindJSON(&newCfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	h.svc.UpdateConfig(newCfg)
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
