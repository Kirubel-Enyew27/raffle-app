package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/wallet/application"
)

type WalletHandler struct {
	walletService *application.WalletService
}

func NewWalletHandler(svc *application.WalletService) *WalletHandler {
	return &WalletHandler{walletService: svc}
}

func (h *WalletHandler) GetBalance(c *gin.Context) {
	userID := c.GetString("user_id")
	wallet, err := h.walletService.GetWallet(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": wallet})
}

func (h *WalletHandler) GetTransactions(c *gin.Context) {
	userID := c.GetString("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	txs, count, err := h.walletService.GetTransactionHistory(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{"transactions": txs, "total": count}})
}

func (h *WalletHandler) Deposit(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Amount      float64 `json:"amount"`
		Reference   string  `json:"reference"`
		Description string  `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error()})
		return
	}
	tx, err := h.walletService.Deposit(c.Request.Context(), userID, req.Amount, req.Reference, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": tx})
}

func (h *WalletHandler) Withdraw(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Amount      float64 `json:"amount"`
		Phone       string  `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error()})
		return
	}
	tx, err := h.walletService.Withdraw(c.Request.Context(), userID, req.Amount, "Withdraw to "+req.Phone, "Withdrawal requested")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": tx})
}

// ─── Admin: Pending Withdrawals ────────────────────────────────────────────────

func (h *WalletHandler) ListPendingWithdrawals(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"code": "error", "message": "admin only"})
		return
	}
	txs, err := h.walletService.ListPendingWithdrawals(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": txs})
}

func (h *WalletHandler) ApproveWithdrawal(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"code": "error", "message": "admin only"})
		return
	}
	adminUserID := c.GetString("user_id")
	txID := c.Param("id")
	tx, err := h.walletService.ApproveWithdrawal(c.Request.Context(), txID, adminUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": tx})
}

func (h *WalletHandler) RejectWithdrawal(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"code": "error", "message": "admin only"})
		return
	}
	adminUserID := c.GetString("user_id")
	txID := c.Param("id")
	tx, err := h.walletService.RejectWithdrawal(c.Request.Context(), txID, adminUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": tx})
}
