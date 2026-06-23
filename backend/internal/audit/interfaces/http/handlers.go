package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/audit/application"
	"github.com/raffle-app/backend/internal/audit/domain"
)

type AuditHandler struct {
	auditService *application.AuditService
}

func NewAuditHandler(auditService *application.AuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

func (h *AuditHandler) GetAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := domain.AuditLogFilter{
		Limit:  limit,
		Offset: offset,
	}

	if action := c.Query("action"); action != "" {
		filter.Action = &action
	}
	if resourceType := c.Query("resource_type"); resourceType != "" {
		filter.ResourceType = &resourceType
	}
	if userID := c.Query("user_id"); userID != "" {
		filter.ActorID = &userID
	}

	logs, total, err := h.auditService.GetAuditLogs(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "error": gin.H{"message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": "SUCCESS",
		"data": gin.H{
			"logs":    logs,
			"total":   total,
			"limit":   limit,
			"offset":  offset,
		},
	})
}

func (h *AuditHandler) GetAuditLog(c *gin.Context) {
	id := c.Param("id")
	log, err := h.auditService.GetAuditLogByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "data": log})
}
