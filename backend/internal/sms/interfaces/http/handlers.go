package http

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	smsapp "github.com/raffle-app/backend/internal/sms/application"
	smsdomain "github.com/raffle-app/backend/internal/sms/domain"
	"github.com/raffle-app/backend/pkg/middleware"
)

type SMSHandler struct {
	svc *smsapp.SMSService
}

func NewSMSHandler(svc *smsapp.SMSService) *SMSHandler {
	return &SMSHandler{svc: svc}
}

// HandleWebhook processes incoming SMS from the SMS Forwarder app.
// The app can send data in two formats:
//  1. Form-urlencoded: Content-Type: application/x-www-form-urlencoded
//     with the SMS text in "key", "message", "text", or "body" field
//  2. Raw text body: the entire POST body is the SMS text
// POST /api/sms/webhook
//
// IMPORTANT: The SMS Forwarder app expects a quick 200 OK response, or it
// shows "failed due to no response from the server". So we respond immediately
// and process the SMS asynchronously.
func (h *SMSHandler) HandleWebhook(c *gin.Context) {
	rawBody, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": "failed to read request body"})
		return
	}

	raw := string(rawBody)
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": "empty request body"})
		return
	}

	fmt.Printf("Received raw SMS: %s\n", raw)

	// Try to extract the SMS text from the raw body.
	// The SMS Forwarder app sends form-urlencoded data with the SMS text in a field.
	smsText := extractSMSText(raw)
	if smsText == "" {
		// Not form-urlencoded or no known field found — use the raw body directly
		smsText = raw
	}

	sender, message := smsdomain.ParseRawSMS(smsText)
	fmt.Printf("[SMS] Parsed: sender=%q, message_len=%d\n", sender, len(message))

	// Respond immediately with 200 OK — the SMS Forwarder app expects a quick response
	// or it shows "failed due to no response from the server".
	c.JSON(http.StatusOK, gin.H{"status": "received"})

	// Process the SMS asynchronously
	ip := middleware.GetClientIP(c)
	go h.svc.ProcessWebhook(context.Background(), sender, message, ip)
}

// extractSMSText tries to parse the body as form-urlencoded and extract
// the SMS text from known field names (key, message, text, body, sms).
// Returns empty string if the body is not form-urlencoded.
func extractSMSText(raw string) string {
	// Try to parse as form-urlencoded data
	values, err := url.ParseQuery(raw)
	if err != nil {
		return ""
	}

	// Check common field names that the SMS Forwarder app might use
	for _, field := range []string{"key", "message", "text", "body", "sms", "msg"} {
		if v := values.Get(field); v != "" {
			return v
		}
	}

	return ""
}
