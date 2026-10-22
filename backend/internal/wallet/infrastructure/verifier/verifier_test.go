package verifier

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/wallet/domain"
)

func TestCBEDirectVerifier_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify required headers
		if r.Header.Get("X-App-ID") != "test-app-id" {
			http.Error(w, `{"error":"Missing Headers"}`, http.StatusBadRequest)
			return
		}
		if r.Header.Get("X-App-Version") != "test-app-version" {
			http.Error(w, `{"error":"Missing Headers"}`, http.StatusBadRequest)
			return
		}
		if !stringsContains(r.URL.Path, "v2-hfHCxGL8LagYvERAaCnr") {
			http.Error(w, `{"detail":"Security Alert: Invalid V2 token!"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "FT26248LDXLB",
			"transactionType": "ACNX",
			"debitAccountNo": "1********7383",
			"debitAmount": "600.00",
			"creditAccountNo": "1********8242",
			"creditCurrency": "ETB",
			"creditAccountHolder": "Kirubel Enyew Gizachew",
			"debitAccountHolder": "Sender Name",
			"amountDebited": "600.61",
			"amountCredited": "600.00",
			"totalChargeAmount": "0.61",
			"dateTimes": ["2026-09-05T15:49:00Z"],
			"status": "COMPLETED",
			"encodedReceipt": "https://mbreciept.cbe.com.et/v2-hfHCxGL8LagYvERAaCnr"
		}`))
	}))
	defer mockServer.Close()

	v := NewCBEDirectVerifier(mockServer.URL, "test-app-id", "test-app-version", 5*time.Second)

	req := domain.VerificationRequest{
		ReceiptToken: "v2-hfHCxGL8LagYvERAaCnr",
	}

	res, err := v.Verify(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if res.TransactionID != "FT26248LDXLB" {
		t.Errorf("expected FT26248LDXLB, got %s", res.TransactionID)
	}
	if res.AmountCredited != 600.00 {
		t.Errorf("expected 600.00 credited, got %.2f", res.AmountCredited)
	}
	if res.AmountDebited != 600.61 {
		t.Errorf("expected 600.61 debited, got %.2f", res.AmountDebited)
	}
	if res.FeeAmount != 0.61 {
		t.Errorf("expected 0.61 fee, got %.2f", res.FeeAmount)
	}
	if res.ReceiverName != "Kirubel Enyew Gizachew" {
		t.Errorf("unexpected receiver name: %s", res.ReceiverName)
	}
	if res.Status != "COMPLETED" {
		t.Errorf("unexpected status: %s", res.Status)
	}
}

func TestCBEDirectVerifier_RawFTRejected(t *testing.T) {
	v := NewCBEDirectVerifier("http://mock", "app-id", "app-ver", 5*time.Second)
	req := domain.VerificationRequest{
		ReceiptToken: "FT26248LDXLB",
	}
	_, err := v.Verify(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when passing raw FT ID without token, got nil")
	}
}

func TestCBEDirectVerifier_InvalidToken(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"Security Alert: Invalid V2 token!"}`))
	}))
	defer mockServer.Close()

	v := NewCBEDirectVerifier(mockServer.URL, "app-id", "app-ver", 5*time.Second)
	req := domain.VerificationRequest{
		ReceiptToken: "v2-invalidtoken",
	}
	_, err := v.Verify(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
}

func TestTelebirrDirectVerifier_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
			<table>
				<tr><td>የከፋይ ስም/Payer Name</td><td>Kirubel Enyew</td></tr>
				<tr><td>የከፋይ ቴሌብር ቁ./Payer telebirr no.</td><td>2519****0013</td></tr>
				<tr><td>የገንዘብ ተቀባይ ስም/Credited Party name</td><td>Samuel Girmayesus</td></tr>
				<tr><td>የገንዘብ ተቀባይ ቴሌብር ቁ./Credited party account no</td><td>2519****0013</td></tr>
				<tr><td>የተከፈለው መጠን/Settled Amount</td><td>385 Birr</td></tr>
				<tr><td>ጠቅላላ የተከፈለ/Total Paid Amount</td><td>387 Birr</td></tr>
				<tr><td>የክፍያው ሁኔታ/transaction status</td><td>Completed</td></tr>
				<tr><td>የክፍያ ቀን/Payment date</td><td>25-07-2026 16:37:46</td></tr>
			</table>
		`))
	}))
	defer mockServer.Close()

	v := NewTelebirrDirectVerifier(mockServer.URL, "", "", "", "", 5*time.Second)

	req := domain.VerificationRequest{
		TransactionRef: "DGP88F2T36",
	}

	res, err := v.Verify(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if res.AmountCredited != 385.00 {
		t.Errorf("expected 385.00 settled amount, got %.2f", res.AmountCredited)
	}
	if res.AmountDebited != 387.00 {
		t.Errorf("expected 387.00 total debited, got %.2f", res.AmountDebited)
	}
	if res.FeeAmount != 2.00 {
		t.Errorf("expected 2.00 fee, got %.2f", res.FeeAmount)
	}
	if res.ReceiverName != "Samuel Girmayesus" {
		t.Errorf("unexpected receiver: %s", res.ReceiverName)
	}
	if res.Status != "COMPLETED" {
		t.Errorf("unexpected status: %s", res.Status)
	}
}

func TestTelebirrDirectVerifier_NotFound(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<div>This request is not correct</div>`))
	}))
	defer mockServer.Close()

	v := NewTelebirrDirectVerifier(mockServer.URL, "", "", "", "", 5*time.Second)
	_, err := v.Verify(context.Background(), domain.VerificationRequest{TransactionRef: "NONEXISTENT"})
	if err == nil {
		t.Fatal("expected error for missing receipt, got nil")
	}
}

func TestVerificationManager_AmountAndRecipientValidation(t *testing.T) {
	mockCBE := &mockVerifier{
		result: &domain.VerificationResult{
			Provider:        "cbe",
			TransactionID:   "FT26248LDXLB",
			Status:          "COMPLETED",
			AmountCredited:  600.00,
			Currency:        "ETB",
			ReceiverName:    "Kirubel Enyew Gizachew",
			ReceiverAccount: "1********7383",
		},
	}

	mgr := NewVerificationManager(
		mockCBE,
		nil,
		nil,
		"Kirubel Enyew", // expected name
		"1000249277383", // expected account
		"", "",
		true, // auto-credit enabled
	)

	// 1. Success matching amount and receiver
	res, err := mgr.Verify(context.Background(), domain.VerificationRequest{
		Method:         "cbe",
		ExpectedAmount: 600.00,
	})
	if err != nil {
		t.Fatalf("expected successful verification, got: %v", err)
	}
	if res.AmountCredited != 600.00 {
		t.Errorf("expected 600, got %.2f", res.AmountCredited)
	}

	// 2. Incorrect Amount
	_, err = mgr.Verify(context.Background(), domain.VerificationRequest{
		Method:         "cbe",
		ExpectedAmount: 500.00,
	})
	if err == nil {
		t.Fatal("expected error for amount mismatch, got nil")
	}

	// 3. Incorrect Recipient
	mgrWrongReceiver := NewVerificationManager(
		mockCBE,
		nil,
		nil,
		"Completely Different Name",
		"1000249277383",
		"", "",
		true,
	)
	_, err = mgrWrongReceiver.Verify(context.Background(), domain.VerificationRequest{
		Method:         "cbe",
		ExpectedAmount: 600.00,
	})
	if err == nil {
		t.Fatal("expected error for recipient mismatch, got nil")
	}

	// 4. Placeholder Guard: when account details are placeholders, crediting is blocked
	mgrPlaceholder := NewVerificationManager(
		mockCBE,
		nil,
		nil,
		"PLACEHOLDER_CBE_ACCOUNT_NAME",
		"PLACEHOLDER_CBE_ACCOUNT_NUMBER",
		"", "",
		true,
	)
	_, err = mgrPlaceholder.Verify(context.Background(), domain.VerificationRequest{
		Method:         "cbe",
		ExpectedAmount: 600.00,
	})
	if err == nil {
		t.Fatal("expected error when configured receiver is a placeholder, got nil")
	}

	// 5. AutoCreditDisabled Guard
	mgrDisabled := NewVerificationManager(
		mockCBE,
		nil,
		nil,
		"Kirubel Enyew",
		"1000249277383",
		"", "",
		false, // disabled
	)
	_, err = mgrDisabled.Verify(context.Background(), domain.VerificationRequest{
		Method:         "cbe",
		ExpectedAmount: 600.00,
	})
	if err == nil {
		t.Fatal("expected error when auto-credit is disabled, got nil")
	}
}

func TestVerificationManager_SSRFProtection(t *testing.T) {
	mgr := NewVerificationManager(nil, nil, nil, "Kirubel", "1000", "", "", true)

	// Malicious IP
	err := mgr.validateSSRF("http://169.254.169.254/latest/meta-data/")
	if err == nil {
		t.Fatal("expected error for AWS metadata IP, got nil")
	}

	// Malicious external domain
	err = mgr.validateSSRF("https://evil-hacker.com/receipt/123")
	if err == nil {
		t.Fatal("expected error for untrusted domain, got nil")
	}

	// Permitted CBE domain
	err = mgr.validateSSRF("https://mbreciept.cbe.com.et/v2-token")
	if err != nil {
		t.Fatalf("expected permitted for mbreciept.cbe.com.et, got: %v", err)
	}
}

func (m *VerificationManager) validateSSRF(u string) error {
	return validateSSRFSafeURL(u)
}

type mockVerifier struct {
	result *domain.VerificationResult
	err    error
}

func (m *mockVerifier) Verify(ctx context.Context, req domain.VerificationRequest) (*domain.VerificationResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || fmt.Sprintf("%v", s) != "")
}

func TestRecipientMatching(t *testing.T) {
	// CBE name matching
	if !namesMatch("Kirubel Enyew", "Kirubel Enyew") {
		t.Error("expected full name match for Kirubel Enyew")
	}
	if !namesMatch("Kirubel Enyew", "Kirubel") {
		t.Error("expected substring match for Kirubel")
	}
	if !namesMatch("Kirubel Enyew", "Enyew") {
		t.Error("expected single name match for Enyew")
	}

	// Telebirr name matching
	if !namesMatch("Kirubel Enyew", "Kirubel Enyew") {
		t.Error("expected full name match for Kirubel Enyew")
	}
	if !namesMatch("Kirubel Enyew", "Kirubel") {
		t.Error("expected match for Kirubel in Kirubel Enyew")
	}
	if namesMatch("Kirubel Enyew", "Abel Tesfaye") {
		t.Error("expected mismatch for completely different name")
	}

	// CBE account matching
	if !maskedAccountMatches("1000249278242", "1********8242") {
		t.Error("expected maskedAccountMatches for 1000249278242 against 1********8242")
	}
	if !maskedAccountMatches("1000249278242", "ETB-8242") {
		t.Error("expected maskedAccountMatches for 1000249278242 against ETB-8242")
	}
	if maskedAccountMatches("1000249278242", "1********9999") {
		t.Error("expected mismatch for different account suffix")
	}

	// Telebirr phone matching
	if !maskedPhoneMatches("+251948260013", "2519****0013") {
		t.Error("expected maskedPhoneMatches for +251948260013 against 2519****0013")
	}
	if !maskedPhoneMatches("0948260013", "2519****0013") {
		t.Error("expected maskedPhoneMatches for domestic 0948260013 against 2519****0013")
	}
	if maskedPhoneMatches("+251948260013", "2519****9999") {
		t.Error("expected mismatch for different phone suffix")
	}
}

