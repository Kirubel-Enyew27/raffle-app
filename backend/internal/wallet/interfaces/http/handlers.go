package http

import (
	"errors"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/wallet/application"
	"github.com/raffle-app/backend/internal/wallet/infrastructure/verifier"
	apperrors "github.com/raffle-app/backend/pkg/errors"
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

// VerifyDepositFromSMS parses a pasted Telebirr or CBE confirmation SMS and runs
// it through the same provider verification pipeline as the receipt and
// transaction-number deposit methods. The SMS only supplies the reference and an
// expected amount; the provider receipt decides whether the payment happened.
func (h *WalletHandler) VerifyDepositFromSMS(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		SMSText string `json:"sms_text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": "Paste the payment confirmation SMS you received from Telebirr or CBE."})
		return
	}

	deposit, tx, parsed, err := h.walletService.VerifyDepositFromSMS(c.Request.Context(), userID, req.SMSText)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": smsDepositErrorMessage(err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": "success",
		"data": gin.H{
			"transaction": tx,
			"receipt": gin.H{
				"payment_method":  deposit.PaymentMethod,
				"amount":          tx.Amount,
				"transaction_ref": tx.Reference,
				"transfer_date":   parsed.TransactionDate,
				"receiver_name":   deposit.ReceiverName,
				"status":          "Completed",
				"receipt_url":     deposit.ReceiptURL,
			},
			"parsed": parsed,
		},
	})
}

// smsDepositErrorMessage keeps provider and database internals out of the
// response while still telling the user what they can act on.
func smsDepositErrorMessage(err error) string {
	var parseErr *verifier.SMSParseError
	if errors.As(err, &parseErr) {
		return parseErr.Message
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "already been credited"), strings.Contains(msg, "already been processed"):
		return "This transaction has already been credited to a wallet."
	case strings.Contains(msg, "does not match expected deposit amount"):
		return "The verified payment amount does not match the amount in the SMS."
	case strings.Contains(msg, "receipt not found"):
		return "We couldn't find a receipt for this payment yet. If you just paid, wait a moment and try again."
	case strings.Contains(msg, "not completed"):
		return "This payment has not completed yet. Please try again once the transfer settles."
	case strings.Contains(msg, "automatic crediting"),
		strings.Contains(msg, "not yet been configured"),
		strings.Contains(msg, "disabled by server configuration"):
		return "Automatic deposit verification is temporarily unavailable. Please try again later or contact support."
	default:
		return "We couldn't verify this payment. Please check the SMS and try again."
	}
}

func (h *WalletHandler) Withdraw(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Amount float64 `json:"amount"`
		Phone  string  `json:"phone"`
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

func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

func (h *WalletHandler) ApproveWithdrawal(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"code": "error", "message": "admin only"})
		return
	}
	adminUserID := c.GetString("user_id")
	txID := c.Param("id")
	if !isUUID(txID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": "invalid transaction id"})
		return
	}
	tx, err := h.walletService.ApproveWithdrawal(c.Request.Context(), txID, adminUserID)
	if err != nil {
		c.JSON(apperrors.HTTPStatus(err), gin.H{"code": "error", "message": err.Error()})
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
	if !isUUID(txID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "error", "message": "invalid transaction id"})
		return
	}
	tx, err := h.walletService.RejectWithdrawal(c.Request.Context(), txID, adminUserID)
	if err != nil {
		c.JSON(apperrors.HTTPStatus(err), gin.H{"code": "error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "success", "data": tx})
}
