package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// requestIDMiddleware adds a unique request ID to each request.
func (s *Server) requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}

// loggingMiddleware logs all requests.
func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		if raw != "" {
			path = path + "?" + raw
		}

		logger := s.logger.With().
			Str("request_id", c.GetString("request_id")).
			Str("client_ip", clientIP).
			Str("method", method).
			Str("path", path).
			Int("status", statusCode).
			Dur("latency", latency).
			Logger()

		switch {
		case statusCode >= 500:
			logger.Error().Str("error", errorMessage).Msg("Server error")
		case statusCode >= 400:
			logger.Warn().Str("error", errorMessage).Msg("Client error")
		case statusCode >= 300:
			logger.Info().Msg("Redirect")
		default:
			logger.Info().Msg("Request completed")
		}
	}
}

// recoveryMiddleware recovers from panics.
func (s *Server) recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				s.logger.Error().
					Interface("error", err).
					Str("request_id", c.GetString("request_id")).
					Msg("Panic recovered")

				c.JSON(500, ErrorResponse{
					Error:     "internal_error",
					Message:   "An internal error occurred",
					RequestID: c.GetString("request_id"),
					Timestamp: time.Now().Unix(),
				})

				c.Abort()
			}
		}()

		c.Next()
	}
}

// corsMiddleware handles CORS headers.
func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Request-ID")
		c.Header("Access-Control-Allow-Methods", "POST, HEAD, PATCH, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// TODO: Implement these middleware functions when needed
// // rateLimitMiddleware implements rate limiting.
// func (s *Server) rateLimitMiddleware(limit int, window time.Duration) gin.HandlerFunc {
// 	// TODO: Implement rate limiting
// 	return func(c *gin.Context) {
// 		c.Next()
// 	}
// }

// // authMiddleware implements authentication.
// func (s *Server) authMiddleware() gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		// TODO: Implement authentication for write operations
// 		token := c.GetHeader("Authorization")
// 		if token == "" {
// 			c.JSON(401, ErrorResponse{
// 				Error:     "unauthorized",
// 				Message:   "Authentication required",
// 				RequestID: c.GetString("request_id"),
// 				Timestamp: time.Now().Unix(),
// 			})
// 			c.Abort()
// 			return
// 		}

// 		// Validate token
// 		if !s.validateToken(token) {
// 			c.JSON(401, ErrorResponse{
// 				Error:     "invalid_token",
// 				Message:   "Invalid authentication token",
// 				RequestID: c.GetString("request_id"),
// 				Timestamp: time.Now().Unix(),
// 			})
// 			c.Abort()
// 			return
// 		}

// 		c.Next()
// 	}
// }

// // validateToken validates an authentication token.
// func (s *Server) validateToken(token string) bool {
// 	// TODO: Implement proper token validation
// 	expectedToken := fmt.Sprintf("Bearer %s", s.config.ClaudeAPIKey)
// 	return token == expectedToken
// }
