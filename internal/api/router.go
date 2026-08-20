package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"c2c_monitor/config"
	"c2c_monitor/internal/service"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter(svc *service.MonitorService, cfg *config.Config) *gin.Engine {
	r := gin.Default()
	if err := r.SetTrustedProxies(nil); err != nil {
		panic("disable trusted proxies: " + err.Error())
	}

	if cfg != nil && len(cfg.App.AllowedOrigins) > 0 {
		corsConfig := cors.DefaultConfig()
		corsConfig.AllowOrigins = append([]string(nil), cfg.App.AllowedOrigins...)
		corsConfig.AllowMethods = []string{http.MethodGet, http.MethodPost, http.MethodOptions}
		corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
		r.Use(cors.New(corsConfig))
	}

	h := NewHandler(svc)
	r.GET("/healthz", h.GetHealth)
	r.GET("/readyz", h.GetReady)

	// API Routes
	v1 := r.Group("/api/v1")
	{
		v1.GET("/history", h.GetHistory)
	}

	// Config Routes
	r.GET("/api/meta", h.GetMeta)
	r.GET("/api/changelog", h.GetChangelog)
	r.GET("/api/config", h.GetConfig)

	// Alert Routes
	r.GET("/api/alerts/status", h.GetAlertStatus)
	r.GET("/api/alerts/benchmark", h.GetAlertBenchmark)

	// Service Status
	r.GET("/api/status", h.GetServiceStatus)

	adminToken := ""
	if cfg != nil {
		adminToken = cfg.App.AdminToken
	}
	admin := r.Group("/api")
	admin.Use(requireAdminToken(adminToken))
	admin.POST("/config", h.UpdateConfig)
	admin.POST("/alerts/benchmark", h.UpdateAlertBenchmark)
	admin.POST("/alerts/reset", h.ResetAlert)

	return r
}

func requireAdminToken(expectedToken string) gin.HandlerFunc {
	expected := []byte(strings.TrimSpace(expectedToken))

	return func(c *gin.Context) {
		authorization := strings.Fields(c.GetHeader("Authorization"))
		if len(authorization) != 2 || !strings.EqualFold(authorization[0], "Bearer") {
			c.Header("WWW-Authenticate", "Bearer")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "admin authorization required"})
			return
		}

		token := []byte(authorization[1])
		if len(expected) == 0 || len(token) != len(expected) || subtle.ConstantTimeCompare(token, expected) != 1 {
			c.Header("WWW-Authenticate", "Bearer")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid admin authorization"})
			return
		}

		c.Next()
	}
}
