package http

import (
	"context"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/crash/application"
	"github.com/raffle-app/backend/internal/crash/domain"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	"github.com/raffle-app/backend/pkg/money"
)

// RateLimiter guards the money-moving endpoints.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

type CrashHandler struct {
	service *application.CrashService
	limiter RateLimiter
}

func NewCrashHandler(service *application.CrashService) *CrashHandler {
	return &CrashHandler{service: service}
}

// SetRateLimiter enables per-account throttling on betting and cash-out.
func (h *CrashHandler) SetRateLimiter(limiter RateLimiter) { h.limiter = limiter }

// GetState returns the full game snapshot used on load and after reconnects.
func (h *CrashHandler) GetState(c *gin.Context) {
	userID := c.GetString("user_id")
	state, err := h.service.GetState(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": state})
}

// GetHistory returns recent settled rounds for the crash-history strip.
func (h *CrashHandler) GetHistory(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	history, err := h.service.History(c.Request.Context(), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": history})
}

type placeBetRequest struct {
	Amount                float64 `json:"amount"`
	AutoCashoutMultiplier float64 `json:"auto_cashout_multiplier"`
}

// PlaceBet debits the stake and records the bet for the open round.
func (h *CrashHandler) PlaceBet(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		respondError(c, apperrors.ErrUnauthorized)
		return
	}

	var req placeBetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.WithField("INVALID_BODY", "invalid request body", 400, err))
		return
	}
	if math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) || req.Amount <= 0 {
		respondError(c, apperrors.WithField("INVALID_AMOUNT", "amount must be a positive number", 400, nil))
		return
	}

	amount := money.FromFloat(req.Amount)
	if !amount.Valid() {
		respondError(c, apperrors.WithField("INVALID_AMOUNT", "amount must be between 0.01 and 10000000000", 400, nil))
		return
	}

	autoBP := 0
	if req.AutoCashoutMultiplier > 0 {
		autoBP = domain.MultiplierToBasisPoints(req.AutoCashoutMultiplier)
		if autoBP == 0 {
			respondError(c, apperrors.WithField("INVALID_AUTO_CASHOUT", "auto cash-out multiplier must be at least 1.00x", 400, nil))
			return
		}
	}

	if !h.allow(c, "crash:bet:"+userID) {
		return
	}

	bet, err := h.service.PlaceBet(c.Request.Context(), userID, amount, autoBP, c.GetHeader("Idempotency-Key"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": "success", "data": bet})
}

// CashOut settles the caller's active bet at the server-side multiplier.
// In P2P mode this records the cash-out multiplier; the actual wallet credit
// happens at round settlement.
func (h *CrashHandler) CashOut(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		respondError(c, apperrors.ErrUnauthorized)
		return
	}
	if !h.allow(c, "crash:cashout:"+userID) {
		return
	}

	bet, err := h.service.CashOut(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": bet})
}

// GetMyBets returns the caller's own crash bet history.
func (h *CrashHandler) GetMyBets(c *gin.Context) {
	userID := c.GetString("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	bets, total, err := h.service.MyBets(c.Request.Context(), userID, limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{"bets": bets, "total": total}})
}

// GetRoundBets returns the public player table for a round.
func (h *CrashHandler) GetRoundBets(c *gin.Context) {
	roundID := c.Param("id")
	bets, err := h.service.RoundBets(c.Request.Context(), roundID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": bets})
}

// VerifyRound publishes the commit-reveal proof for a finished round.
func (h *CrashHandler) VerifyRound(c *gin.Context) {
	roundID := c.Param("id")
	proof, err := h.service.VerifyRound(c.Request.Context(), roundID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": proof})
}

// ---------------------------------------------------------------------------
// Admin settings endpoints
// ---------------------------------------------------------------------------

// GetSettings returns all admin-configurable game settings (admin only).
func (h *CrashHandler) GetSettings(c *gin.Context) {
	settings, err := h.service.GetSettings(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": settings})
}

type updateSettingRequest struct {
	Value string `json:"value" binding:"required"`
}

// UpdateSetting writes a single setting value (admin only).
func (h *CrashHandler) UpdateSetting(c *gin.Context) {
	adminUserID := c.GetString("user_id")
	if adminUserID == "" {
		respondError(c, apperrors.ErrUnauthorized)
		return
	}
	key := c.Param("key")
	var req updateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperrors.WithField("INVALID_BODY", "invalid request body", 400, err))
		return
	}
	if err := h.service.UpdateSetting(c.Request.Context(), key, req.Value, adminUserID); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "message": "setting updated"})
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

// allow applies the rate limiter, failing open when it is unavailable.
func (h *CrashHandler) allow(c *gin.Context, key string) bool {
	if h.limiter == nil {
		return true
	}
	ok, err := h.limiter.Allow(c.Request.Context(), key)
	if err != nil {
		return true
	}
	if !ok {
		respondError(c, apperrors.ErrRateLimited)
		return false
	}
	return true
}

// respondError maps application errors onto HTTP responses.
func respondError(c *gin.Context, err error) {
	status := apperrors.HTTPStatus(err)
	code := "INTERNAL_ERROR"
	message := err.Error()
	if appErr, ok := err.(*apperrors.AppError); ok {
		code = appErr.Code
		message = appErr.Message
	}
	c.JSON(status, gin.H{"code": code, "message": message})
}
