package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

func RegisterSMSRoutes(r *gin.RouterGroup, handler *SMSHandler, apiKey string) {
	// SMS webhook endpoint - protected by API key, NOT by JWT auth
	// The SMS Forwarder app sends requests here without a user JWT
	sms := r.Group("/sms")
	sms.Use(middleware.APIKeyMiddleware(apiKey))
	{
		sms.POST("/webhook", handler.HandleWebhook)
	}
}
