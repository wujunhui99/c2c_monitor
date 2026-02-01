package api

import (
	"github.com/gin-gonic/gin"
	"c2c_monitor/config"
	"c2c_monitor/internal/service"
	"github.com/gin-contrib/cors"
)

func SetupRouter(svc *service.MonitorService, cfg *config.Config) *gin.Engine {
	r := gin.Default()

	// CORS Middleware - must be before routes
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true // In production, you should restrict this to your frontend's domain
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	r.Use(cors.New(corsConfig))

	h := NewHandler(svc, cfg)

	// API Routes
	v1 := r.Group("/api/v1")
	{
		v1.GET("/history", h.GetHistory)
	}

	// Config Routes
	r.GET("/api/config", h.GetConfig)
	r.POST("/api/config", h.UpdateConfig)

	return r
}
