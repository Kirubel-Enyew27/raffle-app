package verifier

import (
	"errors"
	"strings"
	"testing"
)

// The two SMS bodies supplied with the feature request, verbatim.
const (
	telebirrSMSExample = `You have transferred ETB 200.00 to ALEM AGIMAS (2519****1040) on 11/11/2026 08:56:28. Your transaction number is DKB71O65JR. The service fee is ETB 1.74 and 15% VAT on the service fee is ETB 0.26. Your current E-Money Account balance is ETB 144.97. To download your payment information please click this link: https://transactioninfo.ethiotelecom.et/receipt/DKB71O65JR.

Thank you for using telebirr
Ethio telecom`

	cbeSMSExample = `A debit transaction of ETB 280.00 has occurred on your account &#x31;**\***\*\*\*7383. Service charge of ETB 10.00 and VAT(15%) of ETB 1.50 and Disaster Recovery(5%) of 0.50 with total of ETB 292.00. Your current balance is ETB 9.77. Thanks for Banking with CBE. [https://mbreciept.cbe.com.et/v2-hfHCxNWmthG9UV2qhjhG](https://mbreciept.cbe.com.et/v2-hfHCxNWmthG9UV2qhjhG) for feedback: [https://forms.gle/kGNGQpG3mQCCk3iD6](https://forms.gle/kGNGQpG3mQCCk3iD6)`
)

func TestParsePaymentSMS_TelebirrExample(t *testing.T) {
	details, err := ParsePaymentSMS(telebirrSMSExample)
	if err != nil {
		t.Fatalf("expected Telebirr SMS to parse, got error: %v", err)
	}

	if details.Provider != SMSProviderTelebirr {
		t.Errorf("provider = %q, want telebirr", details.Provider)
	}
	if details.TransactionRef != "DKB71O65JR" {
		t.Errorf("transaction ref = %q, want DKB71O65JR", details.TransactionRef)
	}
	if want := "https://transactioninfo.ethiotelecom.et/receipt/DKB71O65JR"; details.ReceiptURL != want {
		t.Errorf("receipt url = %q, want %q", details.ReceiptURL, want)
	}
	// The transfer amount, not the service fee, VAT or remaining balance.
	if details.Amount != 200.00 {
		t.Errorf("amount = %.2f, want 200.00", details.Amount)
	}
	if details.ReceiverName != "ALEM AGIMAS" {
		t.Errorf("receiver = %q, want ALEM AGIMAS", details.ReceiverName)
	}
	if details.TransactionDate != "11/11/2026 08:56:28" {
		t.Errorf("transaction date = %q, want 11/11/2026 08:56:28", details.TransactionDate)
	}
}

func TestParsePaymentSMS_TelebirrWithoutReceiptLink(t *testing.T) {
	const sms = `Dear customer, you have transferred ETB 1,250.50 to SAMUEL BEKELE (2519****1040) on 02/01/2027 10:11:12. Your transaction number is ABC123XY9Z. Thank you for using telebirr`

	details, err := ParsePaymentSMS(sms)
	if err != nil {
		t.Fatalf("expected SMS without a link to parse, got error: %v", err)
	}
	if details.TransactionRef != "ABC123XY9Z" {
		t.Errorf("transaction ref = %q, want ABC123XY9Z", details.TransactionRef)
	}
	if want := "https://transactioninfo.ethiotelecom.et/receipt/ABC123XY9Z"; details.ReceiptURL != want {
		t.Errorf("receipt url = %q, want constructed %q", details.ReceiptURL, want)
	}
	if details.Amount != 1250.50 {
		t.Errorf("amount = %.2f, want 1250.50", details.Amount)
	}
}

func TestParsePaymentSMS_CBEExample(t *testing.T) {
	details, err := ParsePaymentSMS(cbeSMSExample)
	if err != nil {
		t.Fatalf("expected CBE SMS to parse, got error: %v", err)
	}

	if details.Provider != SMSProviderCBE {
		t.Errorf("provider = %q, want cbe", details.Provider)
	}
	if details.ReceiptToken != "v2-hfHCxNWmthG9UV2qhjhG" {
		t.Errorf("receipt token = %q, want v2-hfHCxNWmthG9UV2qhjhG", details.ReceiptToken)
	}
	if want := "https://mbreciept.cbe.com.et/v2-hfHCxNWmthG9UV2qhjhG"; details.ReceiptURL != want {
		t.Errorf("receipt url = %q, want %q", details.ReceiptURL, want)
	}
	// 280.00 is the transfer; 292.00 is the total debit including charges.
	if details.Amount != 280.00 {
		t.Errorf("amount = %.2f, want 280.00", details.Amount)
	}
	if details.TotalDebit != 292.00 {
		t.Errorf("total debit = %.2f, want 292.00", details.TotalDebit)
	}
	// The feedback form link must never be mistaken for the receipt.
	if strings.Contains(details.ReceiptURL, "forms.gle") {
		t.Errorf("receipt url picked up the feedback link: %q", details.ReceiptURL)
	}
}

func TestParsePaymentSMS_HTMLEncodedAndWhitespaceVariants(t *testing.T) {
	const sms = "  A debit transaction of ETB 280.00 has occurred on your account &amp; is complete.\r\n" +
		"\r\n Service   charge of ETB 10.00 and VAT(15%) of ETB 1.50 with total of ETB 292.00.\r\n" +
		" Thanks for Banking with CBE.   https://mbreciept.cbe.com.et/v2-hfHCxNWmthG9UV2qhjhG  "

	details, err := ParsePaymentSMS(sms)
	if err != nil {
		t.Fatalf("expected whitespace/entity variant to parse, got error: %v", err)
	}
	if details.Amount != 280.00 {
		t.Errorf("amount = %.2f, want 280.00", details.Amount)
	}
	if details.ReceiptToken != "v2-hfHCxNWmthG9UV2qhjhG" {
		t.Errorf("receipt token = %q, want v2-hfHCxNWmthG9UV2qhjhG", details.ReceiptToken)
	}
}

func TestParsePaymentSMS_RejectsUnusableInput(t *testing.T) {
	cases := []struct {
		name string
		sms  string
		code string
	}{
		{
			name: "empty",
			sms:  "   \n  ",
			code: SMSErrorEmpty,
		},
		{
			name: "unsupported provider",
			sms:  "Your KWIK verification code is 482913. Do not share it with anyone.",
			code: SMSErrorUnsupported,
		},
		{
			name: "ambiguous providers",
			sms:  "You have transferred ETB 200.00 via telebirr. Thanks for Banking with CBE.",
			code: SMSErrorAmbiguous,
		},
		{
			name: "telebirr without a transaction number",
			sms:  "You have transferred money successfully. Thank you for using telebirr",
			code: SMSErrorMissingRef,
		},
		{
			name: "cbe with only the FT reference",
			sms:  "A debit transaction of ETB 280.00 has occurred on your account. Ref FT26248LDXLB. Thanks for Banking with CBE.",
			code: SMSErrorMissingRef,
		},
		{
			name: "cbe token on a foreign domain",
			sms:  "A debit transaction of ETB 280.00 has occurred. Thanks for Banking with CBE. Receipt: https://evil.example/v2-abc123def456",
			code: SMSErrorInvalidURL,
		},
		{
			name: "telebirr link on a foreign domain only",
			sms:  "Payment sent. Thank you for using telebirr. Receipt: https://evil.example/receipt/DKB71O65JR",
			code: SMSErrorMissingRef,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParsePaymentSMS(tc.sms)
			if err == nil {
				t.Fatal("expected the SMS to be rejected, got nil error")
			}
			var parseErr *SMSParseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("expected *SMSParseError, got %T", err)
			}
			if parseErr.Code != tc.code {
				t.Errorf("code = %q, want %q (message: %s)", parseErr.Code, tc.code, parseErr.Message)
			}
			if strings.TrimSpace(parseErr.Message) == "" {
				t.Error("parse error must carry a user facing message")
			}
		})
	}
}
