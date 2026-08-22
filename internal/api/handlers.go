package api

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"c2c_monitor/internal/appmeta"
	"c2c_monitor/internal/domain"
	"c2c_monitor/internal/service"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *service.MonitorService
}

func NewHandler(svc *service.MonitorService) *Handler {
	return &Handler{svc: svc}
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
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount"})
		return
	}

	// Calculate start time
	now := time.Now()
	var startTime time.Time
	granularity := domain.HistoryGranularityRaw
	switch rangeStr {
	case "1d":
		startTime = now.Add(-24 * time.Hour)
	case "7d":
		startTime = now.Add(-7 * 24 * time.Hour)
		granularity = domain.HistoryGranularityHour
	case "30d":
		startTime = now.Add(-30 * 24 * time.Hour)
		granularity = domain.HistoryGranularityHour
	case "all":
		// Zero time means no lower bound in repository filters.
		startTime = time.Time{}
		granularity = domain.HistoryGranularityDay
	default:
		startTime = now.Add(-24 * time.Hour) // Default 1d
	}

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

	resp := gin.H{"forex": []gin.H{}}
	for _, exchangeName := range domain.SupportedExchangeNames() {
		resp[domain.ExchangeResponseKey(exchangeName)] = []gin.H{}
	}

	appendExchangeHistory := func(exchangeName, responseKey string) {
		filter.Exchange = exchangeName
		prices, err := h.svc.GetPriceHistoryByGranularity(c.Request.Context(), filter, granularity)
		if granularity != domain.HistoryGranularityRaw && (err != nil || len(prices) == 0) {
			prices, err = h.svc.GetPriceHistory(c.Request.Context(), filter)
		}
		if err != nil {
			return
		}

		var list []gin.H
		for _, p := range prices {
			list = append(list, gin.H{
				"t":                p.CreatedAt.Unix(),
				"v":                p.Price,
				"merchant":         p.Merchant,
				"pay_methods":      domain.NormalizePayMethodsString(p.PayMethods),
				"min_amount":       p.MinAmount,
				"max_amount":       p.MaxAmount,
				"available_amount": p.AvailableAmount,
			})
		}
		resp[responseKey] = list
	}

	// 1. Forex
	forexHistory, err := h.svc.GetForexHistoryByGranularity(c.Request.Context(), "USDCNY", startTime, now, granularity)
	if granularity != domain.HistoryGranularityRaw && (err != nil || len(forexHistory) == 0) {
		forexHistory, err = h.svc.GetForexHistory(c.Request.Context(), "USDCNY", startTime, now)
	}
	if err == nil {
		var list []gin.H
		for _, f := range forexHistory {
			list = append(list, gin.H{"t": f.CreatedAt.Unix(), "v": f.Rate})
		}
		resp["forex"] = list
	}

	// 2. Exchanges
	for _, exchangeName := range domain.SupportedExchangeNames() {
		appendExchangeHistory(exchangeName, domain.ExchangeResponseKey(exchangeName))
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": resp})
}

func (h *Handler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.GetConfig())
}

func (h *Handler) GetMeta(c *gin.Context) {
	release, err := appmeta.CurrentRelease()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	historyKeys := make(map[string]string, len(domain.SupportedExchangeNames()))
	for _, exchangeName := range domain.SupportedExchangeNames() {
		historyKeys[exchangeName] = domain.ExchangeResponseKey(exchangeName)
	}

	c.JSON(http.StatusOK, gin.H{
		"version":             appmeta.Version,
		"released_at":         release.ReleasedAt,
		"summary":             release.Summary,
		"changelog_url":       "/api/changelog",
		"supported_exchanges": domain.SupportedExchangeNames(),
		"history_keys":        historyKeys,
	})
}

func (h *Handler) GetChangelog(c *gin.Context) {
	releases, err := appmeta.Catalog()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"releases": releases})
}

func (h *Handler) UpdateConfig(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	newCfg := h.svc.GetConfig()
	if err := c.ShouldBindJSON(&newCfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.UpdateConfig(newCfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (h *Handler) GetAlertStatus(c *gin.Context) {
	states := h.svc.GetAlertStates()
	c.JSON(http.StatusOK, gin.H{"data": states})
}

func (h *Handler) GetAlertBenchmark(c *gin.Context) {
	targetAmount, err := parseOptionalTargetAmount(c.Query("amount"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, err := h.svc.GetAlertBenchmark(c.Request.Context(), targetAmount)
	if err != nil {
		if errors.Is(err, service.ErrInvalidAlertBenchmark) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": status})
}

type UpdateAlertBenchmarkRequest struct {
	BenchmarkPrice *float64 `json:"benchmark_price"`
	TargetAmount   *float64 `json:"target_amount"`
}

func (h *Handler) UpdateAlertBenchmark(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var req UpdateAlertBenchmarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.BenchmarkPrice == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "benchmark_price is required"})
		return
	}

	if math.IsNaN(*req.BenchmarkPrice) || math.IsInf(*req.BenchmarkPrice, 0) || *req.BenchmarkPrice <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "benchmark_price must be greater than 0"})
		return
	}

	status, err := h.svc.UpdateAlertBenchmark(c.Request.Context(), *req.BenchmarkPrice, req.TargetAmount)
	if err != nil {
		if errors.Is(err, service.ErrInvalidAlertBenchmark) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update alert benchmark"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": status})
}

func parseOptionalTargetAmount(raw string) (*float64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	amount, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
		return nil, fmt.Errorf("amount must be >= 0")
	}
	return &amount, nil
}

type ResetAlertRequest struct {
	Exchange string   `json:"exchange" binding:"required"`
	Side     string   `json:"side" binding:"required"`
	Amount   *float64 `json:"amount" binding:"required"`
}

func (h *Handler) ResetAlert(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var req ResetAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	exchange, err := domain.NormalizeExchangeName(req.Exchange)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	side := strings.ToUpper(strings.TrimSpace(req.Side))
	if side != "BUY" && side != "SELL" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "side must be BUY or SELL"})
		return
	}
	if req.Amount == nil || math.IsNaN(*req.Amount) || math.IsInf(*req.Amount, 0) || *req.Amount < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be >= 0"})
		return
	}

	if err := h.svc.ResetAlertState(c.Request.Context(), exchange, side, *req.Amount); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset alert state"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "reset"})
}

func (h *Handler) GetServiceStatus(c *gin.Context) {
	status := h.svc.GetServiceStatuses()
	c.JSON(http.StatusOK, gin.H{"data": status})
}

func (h *Handler) GetHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) GetReady(c *gin.Context) {
	if err := h.svc.ReadinessError(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
