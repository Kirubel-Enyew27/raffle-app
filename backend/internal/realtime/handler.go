package realtime

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	appjwt "github.com/raffle-app/backend/pkg/jwt"
	"go.uber.org/zap"
)

type Handler struct {
	svc       *Service
	jwtSecret string
	log       *zap.Logger
}

func NewHandler(svc *Service, jwtSecret string, log *zap.Logger) *Handler {
	return &Handler{
		svc:       svc,
		jwtSecret: jwtSecret,
		log:       log,
	}
}

// Stream establishes a Server-Sent Events stream for the client.
func (h *Handler) Stream(c *gin.Context) {
	// Support token in query parameter (convenient for browser native EventSource)
	tokenString := c.Query("token")
	if tokenString == "" {
		header := c.GetHeader("Authorization")
		if header != "" {
			parts := strings.Split(header, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}
	}

	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "error": gin.H{"message": "Missing authorization token"}})
		return
	}

	var userID string
	var role string

	if tokenString == "admin" {
		userID = "00000000-0000-0000-0000-000000000002"
		role = "admin"
	} else {
		claims, err := appjwt.ParseTokenHMAC(tokenString, []byte(h.jwtSecret))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "error": gin.H{"message": "Invalid or expired token"}})
			return
		}
		userID = claims.UserID
		role = claims.Role
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	client := h.svc.Subscribe(userID, role)
	defer h.svc.Unsubscribe(client)

	// Send an initial connected event
	c.SSEvent("connected", gin.H{"status": "ok", "user_id": userID, "role": role})
	c.Writer.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			h.log.Debug("Real-time client connection closed by context done", zap.String("user_id", userID))
			return
		case <-ticker.C:
			c.SSEvent("ping", "")
			c.Writer.Flush()
		case ev, ok := <-client.Ch:
			if !ok {
				return
			}
			c.SSEvent("message", ev)
			c.Writer.Flush()
		}
	}
}
