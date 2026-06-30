package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ReceiptData holds the verified data extracted from the Telebirr receipt page.
type ReceiptData struct {
	TransactionID   string
	Status          string // "Completed" or similar
	TotalPaidAmount float64
	PayerName       string
	PayerPhone      string // still masked (e.g. "2519****1116"), informational only
	PaymentDate     time.Time
	RawHTML         string // stored for audit
}

// TelebirrVerifyRequest is sent to the Telebirr verification API.
type TelebirrVerifyRequest struct {
	Reference string `json:"reference"`
}

// TelebirrData is the response payload from the Telebirr verification API.
type TelebirrData struct {
	PayerName              string `json:"payerName"`
	PayerTelebirrNo        string `json:"payerTelebirrNo"`
	CreditedPartyName      string `json:"creditedPartyName"`
	CreditedPartyAccountNo string `json:"creditedPartyAccountNo"`
	TransactionStatus      string `json:"transactionStatus"`
	ReceiptNo              string `json:"receiptNo"`
	PaymentDate            string `json:"paymentDate"`
	SettledAmount          string `json:"settledAmount"`
	TotalPaidAmount        string `json:"totalPaidAmount"`
}

// TelebirrVerifyResponse is the top-level API response wrapper.
type TelebirrVerifyResponse struct {
	Success bool          `json:"success"`
	Data    TelebirrData `json:"data"`
}

// ReceiptFetcher fetches and verifies Telebirr transaction receipts.
// It supports two methods:
//  1. API-based verification (primary) — POST to TeleVerifyUrl with API key
//  2. HTML page scraping (fallback) — fetch from transactioninfo.ethiotelecom.et
type ReceiptFetcher struct {
	client        *http.Client
	teleVerifyURL string
	teleVerifyKey string
}

func NewReceiptFetcher(timeout time.Duration, teleVerifyURL, teleVerifyKey string) *ReceiptFetcher {
	return &ReceiptFetcher{
		client:        &http.Client{Timeout: timeout},
		teleVerifyURL: teleVerifyURL,
		teleVerifyKey: teleVerifyKey,
	}
}

// Fetch retrieves and parses the Telebirr receipt page for the given transaction ID.
// The URL format is: https://transactioninfo.ethiotelecom.et/receipt/{transaction_id}
func (f *ReceiptFetcher) Fetch(transactionID string) (*ReceiptData, error) {
	url := fmt.Sprintf("https://transactioninfo.ethiotelecom.et/receipt/%s", transactionID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Use a browser-like User-Agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14; SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Mobile Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch receipt page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("receipt page returned status %d for transaction %s", resp.StatusCode, transactionID)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read receipt page body: %w", err)
	}

	rawText := string(body)
	return parseReceipt(rawText, transactionID)
}

// parseReceipt extracts structured data from the Telebirr receipt page text.
// The page uses a line-based format: label/EnglishLabel on one line, value on the next.
func parseReceipt(rawHTML, transactionID string) (*ReceiptData, error) {
	result := &ReceiptData{
		TransactionID: transactionID,
		RawHTML:       rawHTML,
	}

	// Normalize: collapse whitespace, split into lines
	text := rawHTML
	lines := strings.Split(text, "\n")

	// Extract transaction status
	// Label: "የክፍያው ሁኔታ/transaction status"
	result.Status = extractValueAfterLabel(lines, "transaction status")
	if result.Status == "" {
		// Also try the Amharic label
		result.Status = extractValueAfterLabel(lines, "የክፍያው ሁኔታ")
	}
	result.Status = strings.TrimSpace(result.Status)

	// Must have a completed status to credit
	if !strings.EqualFold(result.Status, "completed") && result.Status != "" {
		// If we found a status but it's not completed, the transaction is not valid
		return result, fmt.Errorf("transaction status is %q, not completed", result.Status)
	}

	// Extract payer name
	// Label: "የከፋይ ስም/Payer Name"
	result.PayerName = extractValueAfterLabel(lines, "Payer Name")
	if result.PayerName == "" {
		result.PayerName = extractValueAfterLabel(lines, "የከፋይ ስም")
	}
	result.PayerName = strings.TrimSpace(result.PayerName)

	// Extract payer phone (informational only, still masked)
	result.PayerPhone = extractValueAfterLabel(lines, "Payer telebirr no")
	if result.PayerPhone == "" {
		result.PayerPhone = extractValueAfterLabel(lines, "የከፋይ ቴሌብር ቁ")
	}
	result.PayerPhone = strings.TrimSpace(result.PayerPhone)

	// Extract total paid amount
	// Label: "ጠቅላላ የተከፈለ/Total Paid Amount"
	amountStr := extractValueAfterLabel(lines, "Total Paid Amount")
	if amountStr == "" {
		amountStr = extractValueAfterLabel(lines, "ጠቅላላ የተከፈለ")
	}
	if amountStr == "" {
		// Fallback: try "Settled Amount" (might not include fees)
		amountStr = extractValueAfterLabel(lines, "Settled Amount")
		if amountStr == "" {
			amountStr = extractValueAfterLabel(lines, "የተከፈለው መጠን")
		}
	}

	if amountStr != "" {
		// Amount format: "8,858.92 Birr" or "843.71 Birr"
		amountStr = strings.TrimSpace(amountStr)
		// Remove "Birr" suffix
		amountStr = strings.TrimSuffix(amountStr, "Birr")
		amountStr = strings.TrimSuffix(amountStr, "birr")
		amountStr = strings.TrimSpace(amountStr)
		// Remove commas
		amountStr = strings.ReplaceAll(amountStr, ",", "")
		parsed, err := strconv.ParseFloat(amountStr, 64)
		if err == nil && parsed > 0 {
			result.TotalPaidAmount = parsed
		}
	}

	// Extract payment date
	// Label: "የክፍያ ቀን/Payment date"
	dateStr := extractValueAfterLabel(lines, "Payment date")
	if dateStr == "" {
		dateStr = extractValueAfterLabel(lines, "የክፍያ ቀን")
	}
	if dateStr != "" {
		dateStr = strings.TrimSpace(dateStr)
		// Format: "12-08-2025 12:39:06" or "22/06/2026 13:40:40"
		parsed, err := time.Parse("02-01-2006 15:04:05", dateStr)
		if err != nil {
			parsed, _ = time.Parse("02/01/2006 15:04:05", dateStr)
		}
		if !parsed.IsZero() {
			result.PaymentDate = parsed
		}
	}

	return result, nil
}

// extractValueAfterLabel finds a line containing the label (case-insensitive) and returns
// the next non-empty line as the value.
func extractValueAfterLabel(lines []string, label string) string {
	labelLower := strings.ToLower(label)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(trimmed), labelLower) {
			// Look ahead for the next non-empty line
			for j := i + 1; j < len(lines); j++ {
				next := strings.TrimSpace(lines[j])
				// Skip standalone number-only lines (like "1", "2", etc. - HTML artifacts)
				if next == "" {
					continue
				}
				if isLikelyHTMLLine(next) {
					continue
				}
				// Don't grab the next label line
				if containsLabel(next) {
					continue
				}
				return next
			}
		}
	}
	return ""
}

// isLikelyHTMLLine checks if a line looks like HTML/XML markup
func isLikelyHTMLLine(s string) bool {
	return strings.HasPrefix(s, "<") || strings.HasPrefix(s, "{") || strings.HasPrefix(s, "&")
}

// containsLabel checks if a line looks like a label (contains a slash with English text)
func containsLabel(s string) bool {
	labels := []string{"Payer", "Credited", "transaction", "Payment", "Invoice", "Settled",
		"Discount", "VAT", "Stamp", "Service", "Total", "Customer"}
	lower := strings.ToLower(s)
	for _, l := range labels {
		if strings.Contains(lower, strings.ToLower(l)) {
			return true
		}
	}
	return false
}

// ValidateReceipt checks whether a receipt indicates a valid completed payment.
func ValidateReceipt(data *ReceiptData) error {
	if data.Status == "" {
		return fmt.Errorf("could not determine transaction status from receipt")
	}
	if !strings.EqualFold(data.Status, "completed") {
		return fmt.Errorf("transaction status is %q, expected completed", data.Status)
	}
	if data.TotalPaidAmount <= 0 {
		return fmt.Errorf("invalid total paid amount: %.2f", data.TotalPaidAmount)
	}
	if data.PayerName == "" {
		return fmt.Errorf("could not determine payer name from receipt")
	}
	return nil
}

// MaskSensitivePhone masks the payer phone for logging: "251912345049" → "2519****5049"
func MaskSensitivePhone(phone string) string {
	if len(phone) < 8 {
		return phone
	}
	return phone[:4] + "****" + phone[len(phone)-4:]
}

// VerifyTelebirrTransaction calls the Telebirr verification API to validate
// a transaction and returns structured receipt data.
// This is the preferred method over HTML page scraping.
func (f *ReceiptFetcher) VerifyTelebirrTransaction(ctx context.Context, reference string) (*ReceiptData, error) {
	reqBody := TelebirrVerifyRequest{Reference: reference}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.teleVerifyURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", f.teleVerifyKey)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("verification API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result TelebirrVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("verification API returned failure for reference %s", reference)
	}

	// Parse the amount — try SettledAmount first, fall back to TotalPaidAmount
	amountStr := strings.TrimSpace(result.Data.SettledAmount)
	if amountStr == "" {
		amountStr = strings.TrimSpace(result.Data.TotalPaidAmount)
	}
	amountStr = strings.ReplaceAll(amountStr, ",", "")
	var parsedAmount float64
	if amountStr != "" {
		parsed, err := strconv.ParseFloat(amountStr, 64)
		if err == nil {
			parsedAmount = parsed
		}
	}

	// Parse the payment date
	var parsedDate time.Time
	dateStr := strings.TrimSpace(result.Data.PaymentDate)
	if dateStr != "" {
		// Try common formats: "30/06/2026 09:37:49" or "12-08-2025 12:39:06"
		parsed, err := time.Parse("02/01/2006 15:04:05", dateStr)
		if err != nil {
			parsed, _ = time.Parse("02-01-2006 15:04:05", dateStr)
		}
		parsedDate = parsed
	}

	status := strings.TrimSpace(result.Data.TransactionStatus)

	return &ReceiptData{
		TransactionID:   reference,
		Status:          status,
		TotalPaidAmount: parsedAmount,
		PayerName:       strings.TrimSpace(result.Data.PayerName),
		PaymentDate:     parsedDate,
	}, nil
}

// ReceiptRegex matches the receipt number from the URL or page content
var ReceiptRegex = regexp.MustCompile(`[A-Z0-9]{9,15}`)
