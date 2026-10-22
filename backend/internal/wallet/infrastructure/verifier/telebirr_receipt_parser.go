package verifier

import (
	"regexp"
	"strings"
)

// telebirrReceiptFields holds the details regex-extracted from a Telebirr receipt.
//
// The same label/pattern extraction runs against both the Ethio Telecom HTML
// receipt page and the raw OCR text of a receipt screenshot, following the
// approach used by the cashflow-receipt backend (internal/utils/receipt.go:
// ExtractTelebirrInfo, which reads the transaction/receipt number, the settled
// amount and the credited party name out of the receipt body).
type telebirrReceiptFields struct {
	TransactionRef  string
	Status          string
	PayerName       string
	PayerAccount    string
	ReceiverName    string
	ReceiverAccount string
	SettledAmount   string
	TotalAmount     string
	PaymentDate     string
}

// defaultTelebirrReceiptBaseURL is the public Ethio Telecom receipt endpoint that
// serves the HTML receipt for a Telebirr transaction number.
const defaultTelebirrReceiptBaseURL = "https://transactioninfo.ethiotelecom.et/receipt"

// Telebirr receipt patterns. Values are either on the same line as their label
// (OCR text: "Transaction Number: DGP88F2T36") or on the following line
// (receipt page table: "Settled Amount" then "385 Birr").
var (
	telebirrTagRe = regexp.MustCompile(`<[^>]+>`)

	// Transaction number: a labelled value on the screenshot, or the first cell
	// of the "Invoice No. | Payment date | Settled Amount" row on the receipt page.
	telebirrTxnNumberRe = []string{
		`(?i:Transaction\s+Number)\s*[:\x{1361}]?\s*([A-Za-z0-9]{6,})`,
		`(?i:Invoice\s+Number)\s*[:\x{1361}]?\s*([A-Za-z0-9]{6,})`,
		`(?i:Receipt\s+No\.?)\s*[:\x{1361}]?\s*([A-Za-z0-9]{6,})`,
		`(?i:Invoice\s+No\.?)[\s\S]*?\b([A-Z0-9]{6,})\b`,
	}
	// Amounts are anchored on their currency marker so that neighbouring values
	// (transaction number, payment date) are never mistaken for an amount.
	telebirrSettledAmountRe = []string{
		`(?i:Settled\s+Amount)[\s\S]*?([0-9][0-9,]*(?:\.[0-9]+)?)\s*(?:Birr|ETB)`,
		`(?i:የተከፈለው\s+መጠን)[\s\S]*?([0-9][0-9,]*(?:\.[0-9]+)?)\s*(?:Birr|ETB)`,
	}
	telebirrTotalAmountRe = []string{
		`(?i:Total\s+Paid\s+Amount)[\s\S]*?([0-9][0-9,]*(?:\.[0-9]+)?)\s*(?:Birr|ETB)`,
		`(?i:ጠቅላላ\s+የተከፈለ)[\s\S]*?([0-9][0-9,]*(?:\.[0-9]+)?)\s*(?:Birr|ETB)`,
	}
	// Mobile app screenshots only print a single signed amount, e.g. "-387.00 (ETB)".
	telebirrPlainAmountRe = []string{
		`(?im)^\s*-?\s*([0-9][0-9,]*\.[0-9]{2})\s*\(?\s*ETB\)?`,
		`(?i)([0-9][0-9,]*\.[0-9]{2})\s*Birr\b`,
	}
	telebirrStatusRe = []string{
		`(?i)transaction\s+status\s*[:\x{1361}]?\s*([^\n]+)`,
		`(?i)የክፍያው\s+ሁኔታ\s*[:\x{1361}]?\s*([^\n]+)`,
	}
	telebirrPayerNameRe = []string{
		`(?i)Payer\s+Name\s*[:\x{1361}]?\s*([^\n]+)`,
		`(?i)የከፋይ\s+ስም\s*[:\x{1361}]?\s*([^\n]+)`,
	}
	telebirrPayerAccountRe = []string{
		`(?i)Payer\s+telebirr\s+no\.?\s*[:\x{1361}]?\s*([^\n]+)`,
		`(?i)የከፋይ\s+ቴሌብር\s+ቁ\.?\s*[:\x{1361}]?\s*([^\n]+)`,
	}
	telebirrReceiverNameRe = []string{
		`(?i)Credited\s+Party\s+name\s*[:\x{1361}]?\s*([^\n]+)`,
		`(?i)የገንዘብ\s+ተቀባይ\s+ስም\s*[:\x{1361}]?\s*([^\n]+)`,
		`(?i)Transaction\s+To\s*[:\x{1361}]?\s*([^\n]+)`,
	}
	telebirrReceiverAccountRe = []string{
		`(?i)Credited\s+party\s+account\s+no\s*[:\x{1361}]?\s*([^\n]+)`,
		`(?i)የገንዘብ\s+ተቀባይ\s+ቴሌብር\s+ቁ\.?\s*[:\x{1361}]?\s*([^\n]+)`,
	}
	// The payment date sits either beside its label or in the second cell of the
	// "Invoice No. | Payment date | Settled Amount" row, so it is matched by shape.
	telebirrPaymentDateRe = []string{
		`(?i:Payment\s+date)[\s\S]*?(\d{2}[-/]\d{2}[-/]\d{4}(?:\s+\d{1,2}:\d{2}(?::\d{2})?)?)`,
		`(?i:Transaction\s+Time)\s*[:\x{1361}]?\s*(\d{4}[-/]\d{2}[-/]\d{2}\s+\d{1,2}:\d{2}(?::\d{2})?)`,
		`(?i:የክፍያ\s+ቀን)[\s\S]*?(\d{2}[-/]\d{2}[-/]\d{4}(?:\s+\d{1,2}:\d{2}(?::\d{2})?)?)`,
	}
)

// parseTelebirrReceiptText extracts the transaction number and monetary amounts
// from Telebirr receipt text (HTML receipt page or OCR text) using regexes.
func parseTelebirrReceiptText(raw string) telebirrReceiptFields {
	text := normalizeReceiptText(raw)

	var fields telebirrReceiptFields
	fields.TransactionRef = matchFirstGroup(text, telebirrTxnNumberRe...)
	fields.SettledAmount = matchFirstGroup(text, telebirrSettledAmountRe...)
	fields.TotalAmount = matchFirstGroup(text, telebirrTotalAmountRe...)
	fields.Status = matchFirstGroup(text, telebirrStatusRe...)
	fields.PayerName = matchFirstGroup(text, telebirrPayerNameRe...)
	fields.PayerAccount = matchFirstGroup(text, telebirrPayerAccountRe...)
	fields.ReceiverName = matchFirstGroup(text, telebirrReceiverNameRe...)
	fields.ReceiverAccount = matchFirstGroup(text, telebirrReceiverAccountRe...)
	fields.PaymentDate = matchFirstGroup(text, telebirrPaymentDateRe...)

	// App screenshots do not label the amount; only a single signed value is shown.
	if fields.SettledAmount == "" && fields.TotalAmount == "" {
		fields.TotalAmount = matchFirstGroup(text, telebirrPlainAmountRe...)
	}
	// A successful app screenshot is a completed transfer.
	if fields.Status == "" && strings.Contains(strings.ToLower(text), "successful") {
		fields.Status = "Completed"
	}

	return fields
}

// matchFirstGroup returns the first non empty capture group across the patterns.
func matchFirstGroup(text string, patterns ...string) string {
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		match := re.FindStringSubmatch(text)
		if len(match) < 2 {
			continue
		}
		value := strings.TrimSpace(match[1])
		if value == "" || isLabelLine(value) {
			continue
		}
		return value
	}
	return ""
}

// normalizeReceiptText strips HTML markup and blank lines so that both receipt
// pages and OCR output can be matched with the same label/value patterns.
func normalizeReceiptText(raw string) string {
	text := telebirrTagRe.ReplaceAllString(raw, "\n")
	text = strings.ReplaceAll(text, "\r", "")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", `"`)

	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "\n")
}

// telebirrAmount returns the amount credited to the receiving account together
// with the raw text it was parsed from. The settled amount is preferred because
// it excludes the transfer fee.
func telebirrAmount(fields telebirrReceiptFields) (float64, string) {
	for _, raw := range []string{fields.SettledAmount, fields.TotalAmount} {
		if raw == "" {
			continue
		}
		amount, err := parseMonetaryAmount(raw)
		if err != nil || amount == 0 {
			continue
		}
		if amount < 0 {
			amount = -amount
		}
		return amount, raw
	}
	return 0, firstNonEmpty(fields.SettledAmount, fields.TotalAmount)
}
