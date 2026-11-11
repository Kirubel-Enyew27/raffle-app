package application

import (
	"context"
	"strings"
	"testing"

	"github.com/raffle-app/backend/internal/wallet/domain"
	"github.com/raffle-app/backend/internal/wallet/infrastructure/verifier"
)

const (
	smsCBESample = `A debit transaction of ETB 280.00 has occurred on your account &#x31;**\***\*\*\*7383. Service charge of ETB 10.00 and VAT(15%) of ETB 1.50 and Disaster Recovery(5%) of 0.50 with total of ETB 292.00. Your current balance is ETB 9.77. Thanks for Banking with CBE. [https://mbreciept.cbe.com.et/v2-hfHCxNWmthG9UV2qhjhG](https://mbreciept.cbe.com.et/v2-hfHCxNWmthG9UV2qhjhG) for feedback: [https://forms.gle/kGNGQpG3mQCCk3iD6](https://forms.gle/kGNGQpG3mQCCk3iD6)`

	smsTelebirrSample = `You have transferred ETB 200.00 to ALEM AGIMAS (2519****1040) on 11/11/2026 08:56:28. Your transaction number is DKB71O65JR. The service fee is ETB 1.74 and 15% VAT on the service fee is ETB 0.26. Your current E-Money Account balance is ETB 144.97. To download your payment information please click this link: https://transactioninfo.ethiotelecom.et/receipt/DKB71O65JR.

Thank you for using telebirr`
)

// newSMSWalletService builds a wallet service whose provider verifier returns the
// supplied result, so the SMS flow can be exercised without network access.
func newSMSWalletService(t *testing.T, provider string, result *domain.VerificationResult) (*WalletService, *mockWalletRepo) {
	t.Helper()
	wRepo := newMockWalletRepo()
	svc := NewWalletService(wRepo, nil)
	svc.SetDepositRepository(newMockDepositRepo())

	mock := &appMockVerifier{result: result}
	var vm *verifier.VerificationManager
	if provider == verifier.SMSProviderCBE {
		vm = verifier.NewVerificationManager(mock, nil, nil, "Kirubel Enyew", "1000249277383", "Kirubel Enyew", "+251948260013", true)
	} else {
		vm = verifier.NewVerificationManager(nil, mock, nil, "Kirubel Enyew", "1000249277383", "Kirubel Enyew", "+251948260013", true)
	}
	svc.SetVerificationManager(vm)
	return svc, wRepo
}

func TestWalletService_VerifyDepositFromSMS_CBE(t *testing.T) {
	svc, wRepo := newSMSWalletService(t, verifier.SMSProviderCBE, &domain.VerificationResult{
		Provider:        "cbe",
		TransactionID:   "FT26248LDXLB",
		Status:          "COMPLETED",
		AmountCredited:  280.00,
		AmountDebited:   292.00,
		FeeAmount:       12.00,
		Currency:        "ETB",
		ReceiverName:    "Kirubel Enyew",
		ReceiverAccount: "",
	})

	dep, tx, parsed, err := svc.VerifyDepositFromSMS(context.Background(), "user-1", smsCBESample)
	if err != nil {
		t.Fatalf("expected CBE SMS deposit to succeed, got %v", err)
	}
	if parsed.Provider != verifier.SMSProviderCBE {
		t.Errorf("parsed provider = %q, want cbe", parsed.Provider)
	}
	// The SMS transaction amount, not the total debit including charges.
	if parsed.Amount != 280.00 || parsed.TotalDebit != 292.00 {
		t.Errorf("parsed amount/total = %.2f/%.2f, want 280.00/292.00", parsed.Amount, parsed.TotalDebit)
	}
	if dep.ReceiptToken != "v2-hfHCxNWmthG9UV2qhjhG" {
		t.Errorf("deposit receipt token = %q, want the v2 token from the SMS link", dep.ReceiptToken)
	}
	if tx.Amount != 280.00 {
		t.Errorf("credited amount = %.2f, want 280.00", tx.Amount)
	}
	if dep.Metadata["submission_method"] != "sms" {
		t.Errorf("submission_method = %v, want sms", dep.Metadata["submission_method"])
	}

	w, _ := wRepo.FindByUserID(context.Background(), "user-1")
	if w.Balance != 380.00 {
		t.Fatalf("balance = %.2f, want 380.00 (100 starting + 280 deposit)", w.Balance)
	}

	// Replaying the same SMS must not credit the wallet twice.
	if _, _, _, err := svc.VerifyDepositFromSMS(context.Background(), "user-1", smsCBESample); err == nil {
		t.Fatal("expected a replay of the same transaction to be rejected")
	}
	w, _ = wRepo.FindByUserID(context.Background(), "user-1")
	if w.Balance != 380.00 {
		t.Errorf("balance after replay = %.2f, want 380.00", w.Balance)
	}
}

func TestWalletService_VerifyDepositFromSMS_Telebirr(t *testing.T) {
	svc, wRepo := newSMSWalletService(t, verifier.SMSProviderTelebirr, &domain.VerificationResult{
		Provider:        "telebirr",
		TransactionID:   "DKB71O65JR",
		Status:          "COMPLETED",
		AmountCredited:  200.00,
		AmountDebited:   202.00,
		FeeAmount:       2.00,
		Currency:        "ETB",
		ReceiverName:    "Kirubel Enyew",
		ReceiverAccount: "",
		ReceiptURL:      "https://transactioninfo.ethiotelecom.et/receipt/DKB71O65JR",
	})

	dep, tx, parsed, err := svc.VerifyDepositFromSMS(context.Background(), "user-1", smsTelebirrSample)
	if err != nil {
		t.Fatalf("expected Telebirr SMS deposit to succeed, got %v", err)
	}
	if parsed.Provider != verifier.SMSProviderTelebirr {
		t.Errorf("parsed provider = %q, want telebirr", parsed.Provider)
	}
	// Service fee, VAT and remaining balance must not be credited as the deposit.
	if parsed.Amount != 200.00 {
		t.Errorf("parsed amount = %.2f, want 200.00", parsed.Amount)
	}
	if tx.Amount != 200.00 {
		t.Errorf("credited amount = %.2f, want 200.00", tx.Amount)
	}
	if !strings.Contains(dep.ReceiptURL, "receipt/DKB71O65JR") {
		t.Errorf("deposit receipt url = %q, want the Telebirr receipt link", dep.ReceiptURL)
	}

	w, _ := wRepo.FindByUserID(context.Background(), "user-1")
	if w.Balance != 300.00 {
		t.Errorf("balance = %.2f, want 300.00 (100 starting + 200 deposit)", w.Balance)
	}
}

func TestWalletService_VerifyDepositFromSMS_AmountMismatchCreditsNothing(t *testing.T) {
	// The SMS claims 200.00 but the provider receipt only proves 150.00.
	svc, wRepo := newSMSWalletService(t, verifier.SMSProviderTelebirr, &domain.VerificationResult{
		Provider:       "telebirr",
		TransactionID:  "DKB71O65JR",
		Status:         "COMPLETED",
		AmountCredited: 150.00,
		Currency:       "ETB",
		ReceiverName:   "Kirubel Enyew",
	})

	dep, tx, _, err := svc.VerifyDepositFromSMS(context.Background(), "user-1", smsTelebirrSample)
	if err == nil {
		t.Fatal("expected an amount mismatch to be rejected")
	}
	if !strings.Contains(err.Error(), "does not match expected deposit amount") {
		t.Errorf("error = %v, want an amount mismatch error", err)
	}
	if tx != nil {
		t.Errorf("expected no wallet transaction on mismatch, got %+v", tx)
	}
	if dep == nil || dep.Status != domain.DepositStatusFailed {
		t.Errorf("expected the deposit to be marked failed, got %+v", dep)
	}

	w, _ := wRepo.FindByUserID(context.Background(), "user-1")
	if w.Balance != 100.00 {
		t.Errorf("balance = %.2f, want the untouched 100.00", w.Balance)
	}
}

func TestWalletService_VerifyDepositFromSMS_EmptyInput(t *testing.T) {
	svc, _ := newSMSWalletService(t, verifier.SMSProviderCBE, &domain.VerificationResult{})

	if _, _, _, err := svc.VerifyDepositFromSMS(context.Background(), "user-1", "  \n "); err == nil {
		t.Fatal("expected empty SMS input to be rejected")
	}
}
