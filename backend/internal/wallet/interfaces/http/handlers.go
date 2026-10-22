package http

import (
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

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
	txType := c.Query("type")
	txs, count, err := h.walletService.GetTransactionHistory(c.Request.Context(), userID, limit, offset, txType)
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

func (h *WalletHandler) CreateDepositIntent(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Amount        float64 `json:"amount"`
		PaymentMethod string  `json:"payment_method"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error()})
		return
	}
	deposit, err := h.walletService.CreateDepositIntent(c.Request.Context(), userID, req.Amount, req.PaymentMethod)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": "success", "data": deposit})
}

func (h *WalletHandler) GetDeposit(c *gin.Context) {
	userID := c.GetString("user_id")
	depositID := c.Param("id")
	deposit, err := h.walletService.GetDeposit(c.Request.Context(), depositID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": deposit})
}

func (h *WalletHandler) ListUserDeposits(c *gin.Context) {
	userID := c.GetString("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	deposits, total, err := h.walletService.ListUserDeposits(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{"deposits": deposits, "total": total}})
}

func (h *WalletHandler) VerifyDeposit(c *gin.Context) {
	userID := c.GetString("user_id")
	depositID := c.Param("id")

	var tokenOrRef string
	var rawURL string
	var file multipart.File
	var contentType string

	reqContentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(reqContentType, "multipart/form-data") {
		tokenOrRef = c.PostForm("transaction_id")
		if tokenOrRef == "" {
			tokenOrRef = c.PostForm("reference")
		}
		rawURL = c.PostForm("receipt_url")

		fileHeader, err := c.FormFile("file")
		if err != nil {
			fileHeader, err = c.FormFile("receipt")
		}
		if err == nil {
			openedFile, err := fileHeader.Open()
			if err == nil {
				defer openedFile.Close()
				file = openedFile
				contentType = fileHeader.Header.Get("Content-Type")
			}
		}
	} else {
		var req struct {
			TransactionID string `json:"transaction_id"`
			Reference     string `json:"reference"`
			ReceiptURL    string `json:"receipt_url"`
		}
		if err := c.ShouldBindJSON(&req); err == nil {
			tokenOrRef = req.TransactionID
			if tokenOrRef == "" {
				tokenOrRef = req.Reference
			}
			rawURL = req.ReceiptURL
		}
	}

	deposit, tx, err := h.walletService.VerifyDeposit(c.Request.Context(), depositID, userID, tokenOrRef, rawURL, file, contentType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error(), "data": deposit})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": "success",
		"data": gin.H{
			"deposit":     deposit,
			"transaction": tx,
		},
	})
}


func (h *WalletHandler) ExtractReceipt(c *gin.Context) {
	method := c.PostForm("method")
	if method == "" {
		method = c.DefaultQuery("method", "telebirr")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		fileHeader, err = c.FormFile("receipt")
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": "file is required"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": "failed to open uploaded file"})
		return
	}
	defer file.Close()

	contentType := fileHeader.Header.Get("Content-Type")
	extraction, err := h.walletService.ExtractReceiptDetails(c.Request.Context(), method, file, contentType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error()})
		return
	}

	transactionID := extraction.TransactionID
	if strings.ToLower(method) == "cbe" && strings.HasPrefix(strings.TrimSpace(transactionID), "FT") && !strings.Contains(transactionID, "v2-") {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "error",
			"message": "CBE verification requires the receipt token (v2-...) from the QR code or the receipt link (https://mbreciept.cbe.com.et/...), not the FT reference.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": "success", "data": gin.H{
		"transaction_id":   transactionID,
		"amount":           extraction.Amount,
		"amount_raw":       extraction.AmountRaw,
		"receiver_name":    extraction.ReceiverName,
		"transaction_time": extraction.TransactionTime,
		"source":           extraction.Source,
	}})
}

func (h *WalletHandler) VerifyAndDepositReceipt(c *gin.Context) {
	userID := c.GetString("user_id")
	var method string
	var transactionID string

	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		method = c.PostForm("method")
		transactionID = c.PostForm("transaction_id")
		if transactionID == "" || (strings.ToLower(method) == "cbe" && strings.HasPrefix(strings.TrimSpace(transactionID), "FT") && !strings.Contains(transactionID, "v2-")) {
			fileHeader, err := c.FormFile("file")
			if err != nil {
				fileHeader, err = c.FormFile("receipt")
			}
			if err == nil {
				file, err := fileHeader.Open()
				if err == nil {
					defer file.Close()
					extractedID, err := h.walletService.ExtractReceiptTransactionID(c.Request.Context(), method, file, fileHeader.Header.Get("Content-Type"))
					if err != nil {
						c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error()})
						return
					}
					transactionID = extractedID
				}
			}
		}
	} else {
		var req struct {
			Method        string `json:"method"`
			TransactionID string `json:"transaction_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error()})
			return
		}
		method = req.Method
		transactionID = req.TransactionID
	}

	if method == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": "payment method is required"})
		return
	}
	if transactionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": "transaction number is required or could not be detected from image"})
		return
	}
	if strings.ToLower(method) == "cbe" && strings.HasPrefix(strings.TrimSpace(transactionID), "FT") && !strings.Contains(transactionID, "v2-") {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "error",
			"message": "CBE verification requires the receipt token (v2-...) from the QR code or the receipt link (https://mbreciept.cbe.com.et/...), not the FT transaction reference alone.",
		})
		return
	}

	tx, receiptInfo, err := h.walletService.VerifyAndDepositReceipt(c.Request.Context(), userID, method, transactionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": "success",
		"data": gin.H{
			"transaction": tx,
			"receipt": gin.H{
				"payment_method":  receiptInfo.PaymentMethod,
				"amount":          receiptInfo.Amount,
				"transaction_ref": receiptInfo.TransactionRef,
				"transfer_date":   receiptInfo.TransferDate,
				"receiver_name":   receiptInfo.ReceiverName,
				"status":          receiptInfo.Status,
				"receipt_url":     receiptInfo.ReceiptURL,
			},
		},
	})
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
