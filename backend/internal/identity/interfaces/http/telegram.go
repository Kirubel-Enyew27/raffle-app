package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/raffle-app/backend/internal/identity/application"
	apperrors "github.com/raffle-app/backend/pkg/errors"
)

// TelegramAuthHandler exposes Mini App authentication. The raw initData string
// is the only credential accepted; user ids or usernames sent separately are
// never trusted.
type TelegramAuthHandler struct {
	service *application.IdentityService
}

func NewTelegramAuthHandler(service *application.IdentityService) *TelegramAuthHandler {
	return &TelegramAuthHandler{service: service}
}

// Login verifies Telegram.WebApp.initData and returns a normal KWIK session.
func (h *TelegramAuthHandler) Login(c *gin.Context) {
	var req struct {
		InitData string `json:"init_data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_FAILED",
			"message": "telegram init data is required",
		})
		return
	}

	session, err := h.service.LoginWithTelegram(c.Request.Context(), req.InitData)
	if err != nil {
		writeTelegramError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	writeSession(c, http.StatusOK, session)
}

// LinkStatus reports whether the signed-in account has Telegram connected.
func (h *TelegramAuthHandler) LinkStatus(c *gin.Context) {
	linked, err := h.service.TelegramLinkStatus(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		writeTelegramError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{"linked": linked}})
}

// Link connects the current Telegram identity to the signed-in account.
func (h *TelegramAuthHandler) Link(c *gin.Context) {
	var req struct {
		InitData string `json:"init_data" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_FAILED",
			"message": "telegram init data and your password are required",
		})
		return
	}

	err := h.service.LinkTelegram(
		c.Request.Context(),
		c.GetString("user_id"),
		req.InitData,
		req.Password,
	)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrInvalidPassword):
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "INVALID_PASSWORD",
				"message": "the password you entered is incorrect",
			})
		case errors.Is(err, application.ErrTelegramAlreadyLinked):
			c.JSON(http.StatusConflict, gin.H{
				"code":    "ALREADY_LINKED",
				"message": "a telegram account is already connected to this account",
			})
		case errors.Is(err, application.ErrTelegramLinkConflict):
			c.JSON(http.StatusConflict, gin.H{
				"code":    "LINK_CONFLICT",
				"message": "that telegram account is already connected to another user",
			})
		default:
			writeTelegramError(c, err)
		}
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"code": "success", "message": "telegram connected"})
}

// writeTelegramError maps service failures onto stable client codes. Internal
// detail is never returned.
func writeTelegramError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, application.ErrTelegramUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    "TELEGRAM_NOT_CONFIGURED",
			"message": "Telegram sign-in is not configured on this server",
		})
	case errors.Is(err, application.ErrTelegramAuthInvalid):
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "TELEGRAM_AUTH_INVALID",
			"message": "this Telegram sign-in could not be verified; reopen KWIK from Telegram",
		})
	case errors.Is(err, application.ErrTelegramLinkPasswordRequired):
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "PASSWORD_REQUIRED",
			"message": "this account has no password set; sign in another way to connect Telegram",
		})
	case errors.Is(err, apperrors.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "this account is not available"})
	case errors.Is(err, apperrors.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
	case errors.Is(err, apperrors.ErrValidationFailed):
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": "your password is required"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "internal server error"})
	}
}
