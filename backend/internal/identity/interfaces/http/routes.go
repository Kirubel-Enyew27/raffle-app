package http

import (
	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/pkg/middleware"
)

func RegisterIdentityRoutes(r *gin.RouterGroup, handler *IdentityHandler) {
	auth := r.Group("/auth")
	{
		auth.POST("/register", handler.Register)
		auth.POST("/login", handler.Login)
		auth.POST("/change-password", middleware.AuthMiddleware(), handler.ChangePassword)
	}
}
