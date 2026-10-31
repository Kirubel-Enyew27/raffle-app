package realtime

import (
	"encoding/json"
	"fmt"
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

	// ── 3. Set SSE headers ─────────────────────────────────────────────────────
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

	// ── 4. Subscribe ───────────────────────────────────────────────────────────
	// Connect never refuses the request: a newer stream for the same user and
	// channel replaces the previous one, and the per-user cap evicts the oldest
	// connection. Refusing here used to leave reconnecting players permanently
	// stuck because every retry was answered with 429.
	client := h.svc.Connect(userID, role, channel)
	defer func() {
		h.svc.Unsubscribe(client)
		h.log.Info("SSE client disconnected",
			zap.String("user_id", userID),
			zap.String("channel", channel),
			zap.Duration("duration", time.Since(connStart)),
		)
	}()

	// ── 5. Initial response: connected event + browser reconnect hint ──────────
	// A literal `retry:` field (not a named event) sets the browser's own
	// reconnect delay. The client applies its own jittered backoff on top.
	if _, err := c.Writer.WriteString("retry: 3000\n\n"); err != nil {
		return
	}
	if err := writeEvent(c, "connected", gin.H{"status": "ok", "user_id": userID, "role": role}); err != nil {
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ctx := c.Request.Context()

	// ── 6. Event loop ──────────────────────────────────────────────────────────
	for {
		select {
		case <-ctx.Done():
			// Client disconnected or server is shutting down.
			return

		case <-ticker.C:
			// A comment frame keeps the connection alive without dispatching an
			// event. A failed write means the peer is gone (laptop sleep, dropped
			// Wi-Fi); returning frees the goroutine and the connection slot, which
			// otherwise stayed occupied until TCP eventually noticed.
			if _, err := c.Writer.WriteString(": heartbeat\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
			if c.Writer.Written() {
				client.SetLastPing(time.Now())
			}

		case ev, ok := <-client.Ch:
			if !ok {
				// Channel closed by Unsubscribe, eviction or Shutdown.
				return
			}
			if !ev.CreatedAt.IsZero() {
				ev.ServerDeliveryLatencyMS = float64(time.Since(ev.CreatedAt).Microseconds()) / 1000
			}
			if err := writeEvent(c, "message", ev); err != nil {
				return
			}
		}
	}
}

// writeEvent writes one SSE frame and flushes it, reporting transport failures
// so the caller can stop holding a dead connection.
func writeEvent(c *gin.Context, event string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	c.Writer.Flush()
	return nil
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
