package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIKeyMiddleware returns a middleware that validates the X-API-Key header.
func APIKeyMiddleware(expectedKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expectedKey == "" {
			RespondError(c, http.StatusInternalServerError, "server misconfiguration: API key not configured")
			c.Abort()
			return
		}

		key := c.GetHeader("X-API-Key")
		if key == "" {
			RespondError(c, http.StatusUnauthorized, "missing X-API-Key header")
			c.Abort()
			return
		}

		if key != expectedKey {
			RespondError(c, http.StatusForbidden, "invalid API key")
			c.Abort()
			return
		}

		c.Next()
	}
}
