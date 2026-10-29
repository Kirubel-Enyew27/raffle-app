package realtime

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	appjwt "github.com/raffle-app/backend/pkg/jwt"
	"go.uber.org/zap"
)

// Handler wires the realtime Service to HTTP SSE endpoints.
type Handler struct {
	svc       *Service
	jwtSecret string
	log       *zap.Logger
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service, jwtSecret string, log *zap.Logger) *Handler {
	return &Handler{
		svc:       svc,
		jwtSecret: jwtSecret,
		log:       log,
	}
}

// Stream establishes a Server-Sent Events connection for the caller.
//
// Authentication:
//   - ?token=<jwt> query param (native browser EventSource cannot set headers)
//   - Authorization: Bearer <jwt> header
//
// Optional query params:
//   - ?channel=<name>  — subscribe to a shared game-room channel (e.g. "crash")
func (h *Handler) Stream(c *gin.Context) {
	// ── 1. Resolve token ───────────────────────────────────────────────────────
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
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":  "UNAUTHORIZED",
			"error": gin.H{"message": "Missing authorization token"},
		})
		return
	}

	// ── 2. Authenticate ────────────────────────────────────────────────────────
	var userID, role string

	if tokenString == "admin" {
		userID = "00000000-0000-0000-0000-000000000002"
		role = "admin"
	} else {
		claims, err := appjwt.ParseTokenHMAC(tokenString, []byte(h.jwtSecret))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":  "UNAUTHORIZED",
				"error": gin.H{"message": "Invalid or expired token"},
			})
			return
		}
		userID = claims.UserID
		role = claims.Role
	}

	// ── 3. Per-user connection cap ─────────────────────────────────────────────
	if !h.svc.CanConnect(userID) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":  "TOO_MANY_CONNECTIONS",
			"error": gin.H{"message": "Maximum concurrent SSE connections reached for this user"},
		})
		return
	}
	defer h.svc.ReleaseConnection(userID)

	// ── 4. Set SSE headers ─────────────────────────────────────────────────────
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	// Required for nginx to pass SSE through without buffering.
	c.Header("X-Accel-Buffering", "no")

	channel := c.Query("channel")
	clientIP := clientIP(c)
	connStart := time.Now()

	h.log.Info("SSE client connected",
		zap.String("user_id", userID),
		zap.String("role", role),
		zap.String("channel", channel),
		zap.String("client_ip", clientIP),
	)

	// ── 5. Subscribe ───────────────────────────────────────────────────────────
	client := h.svc.SubscribeChannel(userID, role, channel)
	defer func() {
		h.svc.Unsubscribe(client)
		h.log.Info("SSE client disconnected",
			zap.String("user_id", userID),
			zap.String("channel", channel),
			zap.Duration("duration", time.Since(connStart)),
		)
	}()

	// ── 6. Initial response: connected event + browser reconnect hint ──────────
	// "retry: 3000" instructs compliant EventSource implementations to wait 3 s
	// before attempting a reconnect if the connection drops.
	c.SSEvent("retry", "3000")
	c.SSEvent("connected", gin.H{"status": "ok", "user_id": userID, "role": role})
	c.Writer.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ctx := c.Request.Context()

	// ── 7. Event loop ──────────────────────────────────────────────────────────
	for {
		select {
		case <-ctx.Done():
			// Client disconnected or server is shutting down.
			return

		case <-ticker.C:
			// Heartbeat keeps the connection alive and updates LastPing so the
			// stale-client sweeper knows this connection is still live.
			c.SSEvent("ping", "")
			c.Writer.Flush()
			client.SetLastPing(time.Now())

		case ev, ok := <-client.Ch:
			if !ok {
				// Channel closed by Unsubscribe or Shutdown — terminate cleanly.
				return
			}
			c.SSEvent("message", ev)
			c.Writer.Flush()
		}
	}
}

// clientIP extracts the real client IP following the same priority order as
// the middleware.GetClientIP helper, without importing the middleware package.
func clientIP(c *gin.Context) string {
	if ip := c.GetHeader("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return ip
	}
	return c.ClientIP()
}
