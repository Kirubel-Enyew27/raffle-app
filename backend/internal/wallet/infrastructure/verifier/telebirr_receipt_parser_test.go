package verifier

import (
	"strings"
	"testing"
)

// The text below mirrors what Gemini OCR returns for a Telebirr app screenshot
// (see telebirr.jpg at the repository root).
const telebirrOCRScreenshotText = `16:37
Successful
-387.00 (ETB)
Transaction Time: 2026/07/25 16:37:46
Transaction Type: Transfer Money
Transaction To: Samuel
Transaction Number: DGP88F2T36
QR Code
Finished`

const telebirrReceiptPageHTML = `
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
`

func TestParseTelebirrReceiptTextFromScreenshot(t *testing.T) {
	fields := parseTelebirrReceiptText(telebirrOCRScreenshotText)

	if fields.TransactionRef != "DGP88F2T36" {
		t.Errorf("expected transaction number DGP88F2T36, got %q", fields.TransactionRef)
	}
	if fields.ReceiverName != "Samuel" {
		t.Errorf("expected receiver Samuel, got %q", fields.ReceiverName)
	}
	if fields.PaymentDate != "2026/07/25 16:37:46" {
		t.Errorf("expected payment date 2026/07/25 16:37:46, got %q", fields.PaymentDate)
	}
	if fields.Status != "Completed" {
		t.Errorf("expected status Completed derived from the Successful banner, got %q", fields.Status)
	}

	amount, raw := telebirrAmount(fields)
	if amount != 387.00 {
		t.Errorf("expected amount 387.00, got %.2f", amount)
	}
	if raw != "387.00" {
		t.Errorf("expected raw amount 387.00, got %q", raw)
	}
}

func TestParseTelebirrReceiptTextTabSeparated(t *testing.T) {
	// Some OCR responses keep the tab separator of the receipt layout.
	text := strings.Join([]string{
		"Transaction Time:\t2026/07/25 16:37:46",
		"Transaction To:\tSamuel",
		"Transaction Number:\tDGP88F2T36",
		"-387.00 (ETB)",
	}, "\n")

	fields := parseTelebirrReceiptText(text)
	if fields.TransactionRef != "DGP88F2T36" {
		t.Errorf("expected transaction number DGP88F2T36, got %q", fields.TransactionRef)
	}
	if fields.ReceiverName != "Samuel" {
		t.Errorf("expected receiver Samuel, got %q", fields.ReceiverName)
	}

	amount, _ := telebirrAmount(fields)
	if amount != 387.00 {
		t.Errorf("expected amount 387.00, got %.2f", amount)
	}
}

func TestParseTelebirrReceiptPageHtml(t *testing.T) {
	fields := parseTelebirrReceiptText(telebirrReceiptPageHTML)

	if fields.SettledAmount != "385" {
		t.Errorf("expected settled amount 385, got %q", fields.SettledAmount)
	}
	if fields.TotalAmount != "387" {
		t.Errorf("expected total amount 387, got %q", fields.TotalAmount)
	}
	if fields.ReceiverName != "Samuel Girmayesus" {
		t.Errorf("expected receiver Samuel Girmayesus, got %q", fields.ReceiverName)
	}
	if fields.ReceiverAccount != "2519****0013" {
		t.Errorf("expected receiver account 2519****0013, got %q", fields.ReceiverAccount)
	}
	if fields.PayerName != "Kirubel Enyew" {
		t.Errorf("expected payer Kirubel Enyew, got %q", fields.PayerName)
	}
	if fields.PayerAccount != "2519****0013" {
		t.Errorf("expected payer account 2519****0013, got %q", fields.PayerAccount)
	}
	if fields.Status != "Completed" {
		t.Errorf("expected status Completed, got %q", fields.Status)
	}
	if fields.PaymentDate != "25-07-2026 16:37:46" {
		t.Errorf("expected payment date 25-07-2026 16:37:46, got %q", fields.PaymentDate)
	}
	// The receipt page does not print the transaction number, and the masked
	// phone numbers must never be mistaken for one.
	if fields.TransactionRef != "" {
		t.Errorf("expected no transaction number on the receipt page, got %q", fields.TransactionRef)
	}

	amount, raw := telebirrAmount(fields)
	if amount != 385.00 {
		t.Errorf("expected credited amount 385.00, got %.2f", amount)
	}
	if raw != "385" {
		t.Errorf("expected raw amount \"385\", got %q", raw)
	}
}

func TestTelebirrAmountFallsBackToTotal(t *testing.T) {
	html := `<table><tr><td>Total Paid Amount</td><td>387 Birr</td></tr></table>`
	fields := parseTelebirrReceiptText(html)

	amount, raw := telebirrAmount(fields)
	if amount != 387.00 {
		t.Errorf("expected amount 387.00, got %.2f", amount)
	}
	if raw != "387" {
		t.Errorf("expected raw amount \"387\", got %q", raw)
	}
}

// The Invoice No. column of the real receipt page carries the transaction number
// on the row that follows the "Settled Amount" header.
func TestParseTelebirrReceiptPageInvoiceDetailsTable(t *testing.T) {
	html := `<table>
		<tr><td>የክፍያ ዝርዝር/ Invoice details</td></tr>
		<tr><td>የክፍያ ቁጥር/Invoice No.</td><td>የክፍያ ቀን/Payment date</td><td>የተከፈለው መጠን/Settled Amount</td></tr>
		<tr><td>DGP88F2T36</td><td>25-07-2026 16:37:45</td><td>385 Birr</td></tr>
		<tr><td>የአገልግሎት ክፍያ/Service fee</td><td>1.74 Birr</td></tr>
		<tr><td>ጠቅላላ የተከፈለ/Total Paid Amount</td><td>387 Birr</td></tr>
	</table>`

	fields := parseTelebirrReceiptText(html)
	if fields.TransactionRef != "DGP88F2T36" {
		t.Errorf("expected transaction number DGP88F2T36, got %q", fields.TransactionRef)
	}
	if fields.SettledAmount != "385" {
		t.Errorf("expected settled amount 385, got %q", fields.SettledAmount)
	}
	if fields.TotalAmount != "387" {
		t.Errorf("expected total amount 387, got %q", fields.TotalAmount)
	}

	amount, _ := telebirrAmount(fields)
	if amount != 385.00 {
		t.Errorf("expected credited amount 385.00, got %.2f", amount)
	}
}
