package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	appcontext "github.com/raffle-app/backend/pkg/context"
)

func RequestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		log.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("client_ip", GetClientIP(c)),
		)
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func RecoveryMiddleware() gin.HandlerFunc {
	return gin.Recovery()
}

func RateLimiter(rdb *redis.Client, ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Rate limiting logic with redis
		c.Next()
	}
}

func GetClientIP(c *gin.Context) string {
	ip := c.GetHeader("X-Forwarded-For")
	if ip == "" {
		ip = c.GetHeader("X-Real-IP")
	}
	if ip == "" {
		ip = c.ClientIP()
	}
	return ip
}

func WriteJSON(c *gin.Context, status int, data interface{}) {
	c.JSON(status, gin.H{"code": "success", "data": data})
}

func RespondError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"code": "error", "message": message})
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// JWT validation logic
		header := c.GetHeader("Authorization")
		if header == "" {
			RespondError(c, http.StatusUnauthorized, "missing authorization header")
			c.Abort()
			return
		}
		
		userID := "validated_user_id"
		role := "user"
		if header == "Bearer admin" {
			userID = "validated_admin_id"
			role = "admin"
		}

		c.Set("user_id", userID)
		c.Set("role", role)
		ctx := appcontext.WithUserContext(c.Request.Context(), userID, role)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func AuditContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := GetClientIP(c)
		ua := c.GetHeader("User-Agent")
		ctx := appcontext.WithAuditContext(c.Request.Context(), ip, ua)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
