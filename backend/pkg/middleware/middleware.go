package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/config"
	appcontext "github.com/raffle-app/backend/pkg/context"
	appjwt "github.com/raffle-app/backend/pkg/jwt"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
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
	return CORSMiddlewareWithLogger(nil)
}

// CORSMiddlewareWithLogger builds a CORS middleware that reads allowed origins
// from the CORS_ALLOWED_ORIGINS environment variable (comma-separated list).
// Defaults to "*" when the variable is unset — safe for local development but
// not for production. In production (APP_ENV=production) a warning is logged
// when the wildcard is in use.
func CORSMiddlewareWithLogger(log *zap.Logger) gin.HandlerFunc {
	rawOrigins := viper.GetString("CORS_ALLOWED_ORIGINS")
	if rawOrigins == "" {
		rawOrigins = "*"
	}

	// Warn in production when the wildcard is used.
	if viper.GetString("APP_ENV") == "production" && rawOrigins == "*" {
		if log != nil {
			log.Warn("CORS_ALLOWED_ORIGINS is '*' in production — consider restricting to specific origins")
		}
	}

	// Build the origin set for O(1) lookup.
	wildcard := rawOrigins == "*"
	allowedSet := make(map[string]bool)
	if !wildcard {
		for _, o := range strings.Split(rawOrigins, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				allowedSet[o] = true
			}
		}
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		var allowOrigin string
		switch {
		case wildcard:
			allowOrigin = "*"
		case origin != "" && allowedSet[origin]:
			allowOrigin = origin
		default:
			// Not a recognised origin; skip setting the header so the browser
			// blocks the preflight/request.
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
			c.Next()
			return
		}

		c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
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

// RateLimiter is a sliding-window rate limiter backed by Redis.
//
// Algorithm:
//   - Key:   "rl:{ip}:{bucket}" where bucket = unix_seconds / window_seconds
//   - On each request: INCR the key, set EXPIRE to 2× window on first write.
//   - If the resulting count exceeds limit, return 429 with a Retry-After header.
//
// The window_bucket approach means a new bucket (and counter) is created every
// window duration, giving an approximate sliding window with at most 2× over-
// counting at bucket boundaries.
func RateLimiter(rdb *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	windowSecs := int64(window.Seconds())
	if windowSecs <= 0 {
		windowSecs = 1
	}

	return func(c *gin.Context) {
		ip := GetClientIP(c)
		bucket := time.Now().Unix() / windowSecs
		key := fmt.Sprintf("rl:%s:%d", ip, bucket)

		ctx := c.Request.Context()

		// INCR is atomic; if the key did not exist it is created with value 1.
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// Redis unavailable — fail open to avoid blocking all traffic.
			c.Next()
			return
		}

		// Set expiry on first increment so the key does not accumulate forever.
		if count == 1 {
			rdb.Expire(ctx, key, window*2)
		}

		if count > int64(limit) {
			retryAfter := windowSecs - (time.Now().Unix() % windowSecs)
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    "RATE_LIMIT_EXCEEDED",
				"message": "Too many requests — please slow down",
			})
			return
		}

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
		header := c.GetHeader("Authorization")
		if header == "" {
			RespondError(c, http.StatusUnauthorized, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.Split(header, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			RespondError(c, http.StatusUnauthorized, "invalid authorization header format")
			c.Abort()
			return
		}
		tokenString := parts[1]

		// The E2E shortcut must never grant production administrator access.
		if tokenString == "admin" && config.IsDevelopmentEnvironment() {
			userID := "00000000-0000-0000-0000-000000000002"
			role := "admin"
			c.Set("user_id", userID)
			c.Set("role", role)
			ctx := appcontext.WithUserContext(c.Request.Context(), userID, role)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		// Validate real JWT token with the HMAC secret
		jwtSecret := viper.GetString("JWT_SECRET")
		if jwtSecret == "" {
			RespondError(c, http.StatusInternalServerError, "server misconfiguration: JWT_SECRET not set")
			c.Abort()
			return
		}

		claims, err := appjwt.ParseTokenHMAC(tokenString, []byte(jwtSecret))
		if err != nil {
			RespondError(c, http.StatusUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		ctx := appcontext.WithUserContext(c.Request.Context(), claims.UserID, claims.Role)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// OptionalAuthMiddleware parses bearer JWT if present without rejecting unauthenticated requests.
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.Next()
			return
		}

		parts := strings.Split(header, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}
		tokenString := parts[1]

		if tokenString == "admin" && config.IsDevelopmentEnvironment() {
			userID := "00000000-0000-0000-0000-000000000002"
			role := "admin"
			c.Set("user_id", userID)
			c.Set("role", role)
			ctx := appcontext.WithUserContext(c.Request.Context(), userID, role)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		jwtSecret := viper.GetString("JWT_SECRET")
		if jwtSecret == "" {
			c.Next()
			return
		}

		claims, err := appjwt.ParseTokenHMAC(tokenString, []byte(jwtSecret))
		if err == nil && claims != nil {
			c.Set("user_id", claims.UserID)
			c.Set("role", claims.Role)
			ctx := appcontext.WithUserContext(c.Request.Context(), claims.UserID, claims.Role)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}

// AdminMiddleware rejects requests from non-admin users. It must be placed
// after AuthMiddleware so that c.GetString("role") is already populated.
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "admin access required"})
			c.Abort()
			return
		}
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
