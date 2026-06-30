package http

import (
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
	ip := middleware.GetClientIP(c)
	result := h.svc.ProcessWebhook(c.Request.Context(), sender, message, ip)

	if result.Error != nil {
		if result.Verified {
			// Transaction was verified via receipt but couldn't be credited
			// (e.g. user not found, duplicate, DB error)
			c.JSON(http.StatusOK, smsdomain.WebhookResponse{
				Status:        "verified",
				TransactionID: result.TransactionID,
				Amount:        result.Amount,
				Verified:      true,
			})
			return
		}
		if result.TransactionID != "" {
			// Duplicate / already processed
			c.JSON(http.StatusOK, smsdomain.WebhookResponse{
				Status:        "duplicate",
				TransactionID: result.TransactionID,
				Amount:        result.Amount,
				Verified:      false,
			})
			return
		}
		// Not a valid Telebirr SMS or parsing failed
		c.JSON(http.StatusOK, smsdomain.WebhookResponse{
			Status:   "ignored",
			Verified: false,
		})
		return
	}

	c.JSON(http.StatusOK, smsdomain.WebhookResponse{
		Status:        "success",
		TransactionID: result.TransactionID,
		Amount:        result.Amount,
		Verified:      true,
	})
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
