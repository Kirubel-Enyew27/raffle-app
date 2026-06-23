package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/notification/application"
)

type Handler struct{ svc *application.NotificationService }

func NewHandler(svc *application.NotificationService) *Handler { return &Handler{svc: svc} }

// List returns the authenticated user's in-app notifications.
func (h *Handler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	ns, total, err := h.svc.ListForUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": "SUCCESS",
		"data": gin.H{"items": ns, "total": total, "limit": limit, "offset": offset},
	})
}

// Unread returns the unread notification count for the authenticated user.
func (h *Handler) Unread(c *gin.Context) {
	userID := c.GetString("user_id")
	count, err := h.svc.CountUnread(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "data": gin.H{"unread": count}})
}

// MarkRead marks a single in-app notification as read.
func (h *Handler) MarkRead(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	if err := h.svc.MarkRead(c.Request.Context(), id, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "MARK_READ_FAILED", "error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "MARKED_READ"})
}
