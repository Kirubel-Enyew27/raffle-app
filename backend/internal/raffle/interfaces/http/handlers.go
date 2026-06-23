package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	"github.com/raffle-app/backend/internal/raffle/application"
	"github.com/raffle-app/backend/internal/raffle/domain"
)

type RaffleHandler struct {
	raffleService *application.RaffleService
}

func NewRaffleHandler(svc *application.RaffleService) *RaffleHandler {
	return &RaffleHandler{raffleService: svc}
}

type CreateRaffleRequest struct {
	Title        string    `json:"title" binding:"required"`
	Description  string    `json:"description"`
	TicketPrice  float64   `json:"ticket_price" binding:"required,gt=0"`
	MaxTickets   int       `json:"max_tickets" binding:"required,gt=0"`
	DrawDate     time.Time `json:"draw_date" binding:"required"`
	Status       string    `json:"status"`
	PrizePool    float64   `json:"prize_pool"`
	Currency     string    `json:"currency"`
}

type UpdateRaffleRequest struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	TicketPrice float64   `json:"ticket_price" binding:"omitempty,gt=0"`
	MaxTickets  int       `json:"max_tickets" binding:"omitempty,gt=0"`
	DrawDate    time.Time `json:"draw_date"`
	Status      string    `json:"status"`
	PrizePool   float64   `json:"prize_pool"`
	Currency    string    `json:"currency"`
}

func respondError(c *gin.Context, err error) {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.HTTPStatus, gin.H{"code": appErr.Code, "message": appErr.Message})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
}

func (h *RaffleHandler) CreateRaffle(c *gin.Context) {
	var req CreateRaffleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": err.Error()})
		return
	}

	raffle := &domain.Raffle{
		Title:        req.Title,
		Description:  req.Description,
		TicketPrice:   req.TicketPrice,
		TotalTickets:  req.MaxTickets,
		DrawDate:      req.DrawDate,
		Status:        req.Status,
		PrizePool:     req.PrizePool,
		Currency:      req.Currency,
		CreatorID:     c.GetString("user_id"),
	}
	if raffle.Status == "" {
		raffle.Status = "draft"
	}
	if raffle.Currency == "" {
		raffle.Currency = "USD"
	}

	if err := h.raffleService.CreateRaffle(c.Request.Context(), raffle); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": "success", "data": raffle})
}

func (h *RaffleHandler) GetRaffle(c *gin.Context) {
	id := c.Param("id")
	raffle, err := h.raffleService.GetRaffle(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": raffle})
}

func (h *RaffleHandler) ListRaffles(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	raffles, count, err := h.raffleService.ListRaffles(c.Request.Context(), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{"raffles": raffles, "total": count}})
}

func (h *RaffleHandler) UpdateRaffle(c *gin.Context) {
	id := c.Param("id")
	var req UpdateRaffleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": err.Error()})
		return
	}

	raffle := &domain.Raffle{
		ID:           id,
		Title:        req.Title,
		Description:  req.Description,
		TicketPrice:   req.TicketPrice,
		TotalTickets:  req.MaxTickets,
		DrawDate:      req.DrawDate,
		Status:        req.Status,
		PrizePool:     req.PrizePool,
		Currency:      req.Currency,
	}

	if err := h.raffleService.UpdateRaffle(c.Request.Context(), raffle); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": raffle})
}

func (h *RaffleHandler) CloseRaffle(c *gin.Context) {
	id := c.Param("id")
	if err := h.raffleService.CloseRaffle(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "message": "raffle closed"})
}

type ScheduleDrawDateRequest struct {
	DrawDate time.Time `json:"draw_date" binding:"required"`
}

func (h *RaffleHandler) ScheduleDrawDate(c *gin.Context) {
	id := c.Param("id")
	var req ScheduleDrawDateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_FAILED", "message": err.Error()})
		return
	}
	if err := h.raffleService.ScheduleDrawDate(c.Request.Context(), id, req.DrawDate); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "message": "draw date scheduled"})
}
