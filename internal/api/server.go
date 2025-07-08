package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"github.com/ryanrussell/claude-cache-service/internal/cache"
	"github.com/ryanrussell/claude-cache-service/internal/config"
)

// Server represents the API server.
type Server struct {
	config   *config.Config
	cache    *cache.Manager
	logger   zerolog.Logger
	router   *gin.Engine
	upgrader websocket.Upgrader
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Timestamp int64  `json:"timestamp"`
}

// SuccessResponse represents a success response.
type SuccessResponse struct {
	Data      interface{} `json:"data"`
	Message   string      `json:"message"`
	RequestID string      `json:"request_id"`
	Timestamp int64       `json:"timestamp"`
}

// NewServer creates a new API server.
func NewServer(cfg *config.Config, cacheManager *cache.Manager, logger zerolog.Logger) *Server {
	s := &Server{
		config: cfg,
		cache:  cacheManager,
		logger: logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO: Implement proper CORS check for production
				return true
			},
		},
	}

	s.setupRouter()
	return s
}

// setupRouter configures all routes.
func (s *Server) setupRouter() {
	if s.config.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Middleware
	r.Use(s.requestIDMiddleware())
	r.Use(s.loggingMiddleware())
	r.Use(s.recoveryMiddleware())
	r.Use(s.corsMiddleware())

	// Health check
	r.GET("/health", s.handleHealth)

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Cache operations
		cache := v1.Group("/cache")
		{
			cache.GET("/summary", s.handleCacheSummary)
			cache.GET("/project/:name", s.handleGetProjectCache)
			cache.GET("/sdk/:name", s.handleGetSDKCache)
			cache.POST("/refresh", s.handleRefreshCache)
			cache.DELETE("/key/:key", s.handleDeleteCacheKey)
		}

		// Analytics
		analytics := v1.Group("/analytics")
		{
			analytics.GET("/usage", s.handleUsageAnalytics)
			analytics.GET("/performance", s.handlePerformanceAnalytics)
		}
	}

	// WebSocket endpoints
	r.GET("/ws/updates", s.handleWebSocketUpdates)
	r.GET("/ws/project/:name", s.handleWebSocketProject)

	s.router = r
}

// Run starts the server.
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	// TODO: Implement graceful shutdown
	return nil
}

// Handlers

func (s *Server) handleHealth(c *gin.Context) {
	stats := s.cache.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"version": s.config.Version,
		"cache": gin.H{
			"items":    stats.ItemCount,
			"size":     stats.TotalSize,
			"hit_rate": calculateHitRate(stats.Hits, stats.Misses),
		},
		"timestamp": time.Now().Unix(),
	})
}

func (s *Server) handleCacheSummary(c *gin.Context) {
	stats := s.cache.GetStats()

	response := SuccessResponse{
		Data: gin.H{
			"statistics": gin.H{
				"hits":       stats.Hits,
				"misses":     stats.Misses,
				"sets":       stats.Sets,
				"deletes":    stats.Deletes,
				"total_size": stats.TotalSize,
				"item_count": stats.ItemCount,
				"hit_rate":   calculateHitRate(stats.Hits, stats.Misses),
			},
			"configuration": gin.H{
				"cache_dir": s.config.CacheDir,
				"max_size":  s.config.MaxCacheSize,
				"ttl":       s.config.CacheTTL.String(),
			},
		},
		Message:   "Cache summary retrieved successfully",
		RequestID: c.GetString("request_id"),
		Timestamp: time.Now().Unix(),
	}

	c.JSON(http.StatusOK, response)
}

func (s *Server) handleGetProjectCache(c *gin.Context) {
	projectName := c.Param("name")

	cacheKey := "project:" + projectName
	value, err := s.cache.Get(cacheKey)

	if err != nil {
		s.logger.Error().Err(err).Str("project", projectName).Msg("Failed to get project cache")
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:     "not_found",
			Message:   "Project cache not found",
			RequestID: c.GetString("request_id"),
			Timestamp: time.Now().Unix(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Data:      value,
		Message:   "Project cache retrieved successfully",
		RequestID: c.GetString("request_id"),
		Timestamp: time.Now().Unix(),
	})
}

func (s *Server) handleGetSDKCache(c *gin.Context) {
	sdkName := c.Param("name")

	cacheKey := "sdk:" + sdkName
	value, err := s.cache.Get(cacheKey)

	if err != nil {
		s.logger.Error().Err(err).Str("sdk", sdkName).Msg("Failed to get SDK cache")
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:     "not_found",
			Message:   "SDK cache not found",
			RequestID: c.GetString("request_id"),
			Timestamp: time.Now().Unix(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Data:      value,
		Message:   "SDK cache retrieved successfully",
		RequestID: c.GetString("request_id"),
		Timestamp: time.Now().Unix(),
	})
}

func (s *Server) handleRefreshCache(c *gin.Context) {
	// TODO: Implement cache refresh logic
	c.JSON(http.StatusAccepted, SuccessResponse{
		Data:      gin.H{"status": "refresh_initiated"},
		Message:   "Cache refresh initiated",
		RequestID: c.GetString("request_id"),
		Timestamp: time.Now().Unix(),
	})
}

func (s *Server) handleDeleteCacheKey(c *gin.Context) {
	key := c.Param("key")

	if err := s.cache.Delete(key); err != nil {
		s.logger.Error().Err(err).Str("key", key).Msg("Failed to delete cache key")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:     "internal_error",
			Message:   "Failed to delete cache key",
			RequestID: c.GetString("request_id"),
			Timestamp: time.Now().Unix(),
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Data:      gin.H{"deleted": key},
		Message:   "Cache key deleted successfully",
		RequestID: c.GetString("request_id"),
		Timestamp: time.Now().Unix(),
	})
}

func (s *Server) handleUsageAnalytics(c *gin.Context) {
	// TODO: Implement usage analytics
	c.JSON(http.StatusOK, SuccessResponse{
		Data: gin.H{
			"token_savings": gin.H{
				"total":      1234567,
				"percentage": 89.5,
			},
			"requests": gin.H{
				"total":  10000,
				"cached": 8500,
			},
		},
		Message:   "Usage analytics retrieved successfully",
		RequestID: c.GetString("request_id"),
		Timestamp: time.Now().Unix(),
	})
}

func (s *Server) handlePerformanceAnalytics(c *gin.Context) {
	// TODO: Implement performance analytics
	c.JSON(http.StatusOK, SuccessResponse{
		Data: gin.H{
			"response_times": gin.H{
				"p50": 10,
				"p95": 50,
				"p99": 100,
			},
			"cache_performance": gin.H{
				"hit_rate":       85.5,
				"avg_latency_ms": 2.5,
			},
		},
		Message:   "Performance analytics retrieved successfully",
		RequestID: c.GetString("request_id"),
		Timestamp: time.Now().Unix(),
	})
}

func (s *Server) handleWebSocketUpdates(c *gin.Context) {
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			s.logger.Error().Err(err).Msg("Failed to close WebSocket connection")
		}
	}()

	// TODO: Implement WebSocket updates
	s.logger.Info().Str("remote", conn.RemoteAddr().String()).Msg("WebSocket connection established")
}

func (s *Server) handleWebSocketProject(c *gin.Context) {
	projectName := c.Param("name")

	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			s.logger.Error().Err(err).Msg("Failed to close WebSocket connection")
		}
	}()

	// TODO: Implement project-specific WebSocket updates
	s.logger.Info().
		Str("remote", conn.RemoteAddr().String()).
		Str("project", projectName).
		Msg("Project WebSocket connection established")
}

// Helper functions

func calculateHitRate(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}
