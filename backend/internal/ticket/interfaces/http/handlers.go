package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	stderrors "github.com/raffle-app/backend/pkg/errors"
	ticketapplication "github.com/raffle-app/backend/internal/ticket/application"
	ticketdomain "github.com/raffle-app/backend/internal/ticket/domain"
)

type TicketHandler struct {
	ticketService *ticketapplication.TicketService
}

func NewTicketHandler(svc *ticketapplication.TicketService) *TicketHandler {
	return &TicketHandler{ticketService: svc}
}

func respondError(c *gin.Context, err error) {
	var appErr *stderrors.AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.HTTPStatus, gin.H{"code": appErr.Code, "message": appErr.Message})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
}

func (h *TicketHandler) PurchaseTickets(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "missing user context"})
		return
	}

	var req ticketdomain.PurchaseTicketsInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": err.Error()})
		return
	}
	req.UserID = userID
	req.IdempotencyKey = c.GetHeader("Idempotency-Key")

	result, err := h.ticketService.PurchaseTickets(c.Request.Context(), &req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": "success", "data": result})
}
