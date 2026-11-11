package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/raffle-app/backend/internal/wallet/application"
)

func newSMSDepositRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc := application.NewWalletService(&stubWalletRepo{}, nil)
	h := NewWalletHandler(svc)
	r := gin.New()
	r.POST("/wallets/deposit/sms", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		h.VerifyDepositFromSMS(c)
	})
	return r
}

func postSMS(t *testing.T, router *gin.Engine, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/wallets/deposit/sms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec.Code, resp.Message
}

func TestVerifyDepositFromSMS_RejectsEmptyInput(t *testing.T) {
	router := newSMSDepositRouter(t)

	status, message := postSMS(t, router, `{"sms_text":"   "}`)
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
	if !strings.Contains(message, "Paste the payment confirmation SMS") {
		t.Errorf("message = %q, want the paste instruction", message)
	}
	if strings.Contains(strings.ToLower(message), "repository") {
		t.Errorf("message leaks internal detail: %q", message)
	}
}

func TestVerifyDepositFromSMS_RejectsUnsupportedProvider(t *testing.T) {
	router := newSMSDepositRouter(t)

	status, message := postSMS(t, router, `{"sms_text":"Your verification code is 482913."}`)
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
	if !strings.Contains(message, "Only Telebirr and CBE") {
		t.Errorf("message = %q, want the supported provider hint", message)
	}
}

func TestVerifyDepositFromSMS_RejectsMalformedBody(t *testing.T) {
	router := newSMSDepositRouter(t)

	status, message := postSMS(t, router, `not json`)
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
	if !strings.Contains(message, "Paste the payment confirmation SMS") {
		t.Errorf("message = %q, want the paste instruction", message)
	}
}
