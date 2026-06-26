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
		auth.PUT("/profile", middleware.AuthMiddleware(), handler.UpdateProfile)
		auth.POST("/avatar", middleware.AuthMiddleware(), handler.UploadAvatar)
		auth.POST("/change-password", middleware.AuthMiddleware(), handler.ChangePassword)
	}
}

