package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/identity/application"
)

type IdentityHandler struct {
	service *application.IdentityService
}

func NewIdentityHandler(service *application.IdentityService) *IdentityHandler {
	return &IdentityHandler{service: service}
}

type RegisterRequest struct {
	ID       string `json:"id"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

func (h *IdentityHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error()})
		return
	}

	user, err := h.service.Register(c.Request.Context(), req.ID, req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": "success", "data": user})
}

func (h *IdentityHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error()})
		return
	}

	token, user, err := h.service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{
		"token": token,
		"user":  user,
	}})
}

func (h *IdentityHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "error", "message": "unauthorized"})
		return
	}

	err := h.service.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": "success", "message": "password changed successfully"})
}
