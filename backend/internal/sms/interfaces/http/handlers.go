package http

import (
	"net/http"

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
// POST /api/sms/webhook
func (h *SMSHandler) HandleWebhook(c *gin.Context) {
	var req smsdomain.WebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": "invalid request body: " + err.Error()})
		return
	}

	ip := middleware.GetClientIP(c)
	result := h.svc.ProcessWebhook(c.Request.Context(), req.Sender, req.Message, ip)

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
