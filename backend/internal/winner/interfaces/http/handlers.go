package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/winner/application"
)

type WinnerHandler struct {
	winnerService *application.WinnerService
}

func NewWinnerHandler(winnerService *application.WinnerService) *WinnerHandler {
	return &WinnerHandler{winnerService: winnerService}
}

func (h *WinnerHandler) GetWinnersByRaffle(c *gin.Context) {
	raffleID := c.Param("raffle_id")
	winners, err := h.winnerService.GetWinnersByRaffle(c.Request.Context(), raffleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "data": winners})
}

func (h *WinnerHandler) GetWinnerDetail(c *gin.Context) {
	id := c.Param("id")
	winner, err := h.winnerService.GetWinnerByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "data": winner})
}

func (h *WinnerHandler) MarkPrizePaid(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		PaymentReference string `json:"payment_reference" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "error": gin.H{"message": err.Error()}})
		return
	}
	winner, err := h.winnerService.MarkPrizePaid(c.Request.Context(), id, req.PaymentReference)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "PAYMENT_FAILED", "error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "PRIZE_PAID", "data": winner})
}
