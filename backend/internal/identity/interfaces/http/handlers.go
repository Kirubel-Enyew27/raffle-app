package http

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/identity/application"
	apperrors "github.com/raffle-app/backend/pkg/errors"
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

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// writeSession returns the credential pair in the shape the frontend expects.
// expires_in lets the client refresh proactively if it ever wants to.
func writeSession(c *gin.Context, status int, session *application.Session) {
	c.JSON(status, gin.H{"code": "success", "data": gin.H{
		"token":         session.AccessToken,
		"refresh_token": session.RefreshToken,
		"expires_in":    session.ExpiresIn,
		"user":          session.User,
	}})
}

func (h *IdentityHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error()})
		return
	}

	user, err := h.service.Register(c.Request.Context(), req.ID, req.Email, req.Password, req.FullName, req.Phone)
	if err != nil {
		writeRegistrationError(c, err)
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

	session, err := h.service.Login(c.Request.Context(), req.Identifier, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "error", "message": err.Error()})
		return
	}

	writeSession(c, http.StatusOK, session)
}

// Refresh exchanges a refresh token for a new access token. It is deliberately
// unauthenticated: the whole point is that the access token has expired. The
// refresh token itself is validated (signature, type, expiry, live user).
func (h *IdentityHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": "refresh_token is required"})
		return
	}

	session, err := h.service.RefreshSession(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, apperrors.ErrUnauthorized) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "SESSION_EXPIRED",
				"message": "your session has expired, please sign in again",
			})
			return
		}
		// Anything else is transient (database down, misconfiguration). Keep the
		// session: the client retries instead of logging the player out.
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "internal server error"})
		return
	}

	writeSession(c, http.StatusOK, session)
}

// Logout revokes the refresh token that came with the request so it cannot be
// exchanged again. It is deliberately unauthenticated: the access token may
// already be expired by the time the user signs out, and the refresh token is
// itself the credential being revoked. The call is idempotent.
func (h *IdentityHandler) Logout(c *gin.Context) {
	var req RefreshRequest
	// An empty body simply means there is nothing to revoke server-side.
	_ = c.ShouldBindJSON(&req)

	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		// Revocation could not be recorded. Report it instead of pretending the
		// token is dead; the client still drops it locally.
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": "success", "message": "signed out"})
}

// writeRegistrationError maps domain errors to their HTTP status and never
// echoes raw driver messages (which can disclose constraint/schema details).
func writeRegistrationError(c *gin.Context, err error) {
	status := apperrors.HTTPStatus(err)
	if status >= http.StatusInternalServerError {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "internal server error"})
		return
	}
	c.JSON(status, gin.H{"code": "error", "message": err.Error()})
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
