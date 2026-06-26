package http

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password" binding:"required,min=6"`
	FullName string `json:"full_name"`
}

type LoginRequest struct {
	Identifier string `json:"identifier" binding:"required"`
	Password   string `json:"password" binding:"required"`
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

	user, err := h.service.Register(c.Request.Context(), req.ID, req.Email, req.Password, req.FullName, req.Phone)
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

	token, user, err := h.service.Login(c.Request.Context(), req.Identifier, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{
		"token": token,
		"user":  user,
	}})
}

func (h *IdentityHandler) UpdateProfile(c *gin.Context) {
	var req struct {
		FullName string `json:"full_name"`
		Phone    string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "error", "message": "unauthorized"})
		return
	}

	user, err := h.service.UpdateProfile(c.Request.Context(), userID, req.FullName, req.Phone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": "success", "data": user})
}

func (h *IdentityHandler) UploadAvatar(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "error", "message": "unauthorized"})
		return
	}

	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": "avatar file is required"})
		return
	}
	defer file.Close()

	// Validate file type
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": "only jpg, png, webp files are allowed"})
		return
	}

	// Limit to 2MB
	if header.Size > 2<<20 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": "file size must be under 2MB"})
		return
	}

	uploadDir := "uploads/avatars"

	// Remove any existing avatar file for this user
	oldFiles, _ := filepath.Glob(filepath.Join(uploadDir, userID+".*"))
	for _, f := range oldFiles {
		os.Remove(f)
	}
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": "failed to create upload directory"})
		return
	}

	filename := fmt.Sprintf("%s%s", userID, ext)
	destPath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(destPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": "failed to save file"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": "failed to save file"})
		return
	}

	avatarURL := fmt.Sprintf("/uploads/avatars/%s", filename)

	user, err := h.service.UpdateProfile(c.Request.Context(), userID, "", "", avatarURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": "success", "data": user})
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
