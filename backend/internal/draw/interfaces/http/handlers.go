package http

import (
	"errors"

	"github.com/gin-gonic/gin"
	apperrors "github.com/raffle-app/backend/pkg/errors"
	"github.com/raffle-app/backend/internal/draw/application"
	"github.com/raffle-app/backend/pkg/crypto"
)

type DrawHandler struct {
	drawService *application.DrawService
}

func NewDrawHandler(drawService *application.DrawService) *DrawHandler {
	return &DrawHandler{drawService: drawService}
}

func respondError(c *gin.Context, err error) {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.HTTPStatus, gin.H{"code": appErr.Code, "message": appErr.Message})
		return
	}
	c.JSON(500, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
}

func (h *DrawHandler) CommitDrawSeed(c *gin.Context) {
	raffleID := c.Param("raffle_id")

	commitment, err := h.drawService.CommitDrawSeed(c.Request.Context(), raffleID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"code": "COMMIT_SUCCESS",
		"data": gin.H{
			"raffle_id":  commitment.RaffleID,
			"commit_hash": commitment.CommitHash,
			"message":    "Seed committed. Publish this hash before the draw.",
		},
	})
}

func (h *DrawHandler) ExecuteDraw(c *gin.Context) {
	raffleID := c.Param("raffle_id")

	result, err := h.drawService.ExecuteDraw(c.Request.Context(), raffleID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"code": "DRAW_COMPLETED",
		"data": result,
	})
}

func (h *DrawHandler) VerifyDraw(c *gin.Context) {
	var req struct {
		RaffleID      string `json:"raffle_id" binding:"required"`
		CommitHash    string `json:"commit_hash"`
		RevealedSeed  string `json:"revealed_seed"`
		ClientSeed    string `json:"client_seed"`
		Nonce         int    `json:"nonce"`
		WinningNumber int    `json:"winning_number"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "VALIDATION_FAILED", "error": gin.H{"message": err.Error()}})
		return
	}

	if req.RaffleID != "" {
		result, err := h.drawService.VerifyDraw(c.Request.Context(), req.RaffleID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(200, gin.H{
			"code": "VERIFICATION_COMPLETE",
			"data": result,
		})
		return
	}

	if req.CommitHash == "" || req.RevealedSeed == "" {
		c.JSON(400, gin.H{"code": "VALIDATION_FAILED", "error": gin.H{"message": "either raffle_id or commit_hash+revealed_seed required"}})
		return
	}

	verified := crypto.VerifyCommit(req.CommitHash, req.RevealedSeed)

	var clientSeed string
	if req.ClientSeed != "" {
		clientSeed = req.ClientSeed
	}

	var combinedHash string
	if clientSeed != "" && req.Nonce > 0 {
		combinedHash = crypto.GenerateCombinedHash(req.RevealedSeed, clientSeed, req.Nonce)
	}

	c.JSON(200, gin.H{
		"code": "VERIFICATION_COMPLETE",
		"data": gin.H{
			"verified":       verified,
			"seed_matches":   verified,
			"commit_hash":    req.CommitHash,
			"revealed_seed":  req.RevealedSeed,
			"combined_hash":  combinedHash,
			"winning_number": req.WinningNumber,
		},
	})
}

func (h *DrawHandler) GetDrawResult(c *gin.Context) {
	raffleID := c.Param("raffle_id")
	result, err := h.drawService.GetDrawResult(c.Request.Context(), raffleID)
	if err != nil {
		c.JSON(404, gin.H{"code": "NOT_FOUND", "error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(200, gin.H{"code": "SUCCESS", "data": result})
}
