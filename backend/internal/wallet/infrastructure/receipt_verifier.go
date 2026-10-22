package infrastructure

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ReceiptInfo struct {
	PaymentMethod  string
	Amount         float64
	TransactionRef string
	TransferDate   string
	ReceiverName   string
	Status         string
	ReceiptURL     string
}

type ReceiptVerifier struct {
	client        *http.Client
	teleVerifyURL string
	teleVerifyKey string
	cbeReceiptAPI string
	geminiAPIKey  string
}

func NewReceiptVerifier(teleVerifyURL, teleVerifyKey, cbeReceiptAPI, geminiAPIKey string) *ReceiptVerifier {
	return &ReceiptVerifier{
		client:        &http.Client{Timeout: 30 * time.Second},
		teleVerifyURL: strings.TrimRight(teleVerifyURL, "/"),
		teleVerifyKey: teleVerifyKey,
		cbeReceiptAPI: strings.TrimRight(cbeReceiptAPI, "/"),
		geminiAPIKey:  geminiAPIKey,
	}
}

func (v *ReceiptVerifier) Verify(ctx context.Context, method, transactionID string) (*ReceiptInfo, error) {
	method = normalizeMethod(method)
	transactionID = cleanTransactionID(transactionID)
	if transactionID == "" {
		return nil, fmt.Errorf("transaction number is required")
	}

	switch method {
	case "telebirr":
		return v.verifyTelebirr(ctx, transactionID)
	case "cbe":
		return v.verifyCBE(ctx, transactionID)
	default:
		return nil, fmt.Errorf("unsupported payment method %q", method)
	}
}

func (v *ReceiptVerifier) ExtractTransactionID(ctx context.Context, method string, file multipart.File, contentType string) (string, error) {
	if v.geminiAPIKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is required for screenshot extraction")
	}

	imageBytes, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read receipt image: %w", err)
	}
	if contentType == "" {
		contentType = http.DetectContentType(imageBytes)
	}
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("receipt upload must be an image")
	}

	prompt := transactionPrompt(normalizeMethod(method))
	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"inline_data": map[string]string{
							"mime_type": contentType,
							"data":      base64.StdEncoding.EncodeToString(imageBytes),
						},
					},
					{"text": prompt},
				},
			},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=" + v.geminiAPIKey
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OCR request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("OCR request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to decode OCR response: %w", err)
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no transaction number found in receipt image")
	}

	text := strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text)
	text = strings.Trim(text, "`\"' \n\r\t")
	if strings.EqualFold(text, "bank mismatch") {
		return "", fmt.Errorf("the uploaded photo does not match %s", displayMethod(method))
	}
	if strings.EqualFold(text, "not found") || text == "" {
		return "", fmt.Errorf("could not read the transaction number from the screenshot")
	}
	if normalizeMethod(method) == "cbe" {
		re := regexp.MustCompile(`(https?://mbreciept\.cbe\.com\.et/[A-Za-z0-9_-]+)|(v2-[A-Za-z0-9_-]+)`)
		match := re.FindString(text)
		if match != "" {
			return cleanTransactionID(match), nil
		}
		return "", fmt.Errorf("could not extract CBE receipt QR token; CBE requires the receipt QR code or link (https://mbreciept.cbe.com.et/...), not the FT transaction reference alone")
	}
	return cleanTransactionID(text), nil
}

func (v *ReceiptVerifier) verifyTelebirr(ctx context.Context, reference string) (*ReceiptInfo, error) {
	// Try direct Ethio Telecom HTML receipt scraping first (fastest and primary)
	info, err := v.fetchTelebirrHTML(ctx, reference)
	if err == nil {
		return info, nil
	}

	// Fallback to proxy API if direct scraping fails
	if v.teleVerifyURL != "" && v.teleVerifyKey != "" {
		apiCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		apiInfo, apiErr := v.verifyTelebirrAPI(apiCtx, reference)
		if apiErr == nil {
			return apiInfo, nil
		}
	}

	return nil, fmt.Errorf("Telebirr verification failed: %w", err)
}

func (v *ReceiptVerifier) verifyTelebirrAPI(ctx context.Context, reference string) (*ReceiptInfo, error) {
	reqBody, _ := json.Marshal(map[string]string{"reference": reference})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.teleVerifyURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", v.teleVerifyKey)

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Telebirr verification request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Telebirr verification returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			CreditedPartyName string `json:"creditedPartyName"`
			TransactionStatus string `json:"transactionStatus"`
			ReceiptNo         string `json:"receiptNo"`
			PaymentDate       string `json:"paymentDate"`
			SettledAmount     string `json:"settledAmount"`
			TotalPaidAmount   string `json:"totalPaidAmount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode Telebirr verification response: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("Telebirr verification failed for %s", reference)
	}
	if result.Data.TransactionStatus != "" && !strings.EqualFold(result.Data.TransactionStatus, "completed") {
		return nil, fmt.Errorf("Telebirr transaction status is %q", result.Data.TransactionStatus)
	}

	amount, err := parseAmount(firstNonEmpty(result.Data.SettledAmount, result.Data.TotalPaidAmount))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Telebirr amount: %w", err)
	}
	ref := firstNonEmpty(result.Data.ReceiptNo, reference)
	return &ReceiptInfo{
		PaymentMethod:  "telebirr",
		Amount:         amount,
		TransactionRef: ref,
		TransferDate:   result.Data.PaymentDate,
		ReceiverName:   result.Data.CreditedPartyName,
		Status:         firstNonEmpty(result.Data.TransactionStatus, "Completed"),
		ReceiptURL:     "https://transactioninfo.ethiotelecom.et/receipt/" + ref,
	}, nil
}

func (v *ReceiptVerifier) fetchTelebirrHTML(ctx context.Context, reference string) (*ReceiptInfo, error) {
	url := fmt.Sprintf("https://transactioninfo.ethiotelecom.et/receipt/%s", reference)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14; SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Mobile Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach Telebirr receipt page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("receipt page returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read receipt page: %w", err)
	}

	rawText := string(bodyBytes)
	if strings.Contains(rawText, "This request is not correct") {
		return nil, fmt.Errorf("receipt not found for transaction reference %s", reference)
	}

	// Clean HTML tags and parse line values
	tagRe := regexp.MustCompile(`<[^>]+>`)
	clean := tagRe.ReplaceAllString(rawText, "\n")
	rawLines := strings.Split(clean, "\n")
	var lines []string
	for _, l := range rawLines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}

	status := extractHTMLValue(lines, "transaction status", "የክፍያው ሁኔታ")
	if status != "" && !strings.EqualFold(status, "completed") {
		return nil, fmt.Errorf("transaction status is %q, expected Completed", status)
	}

	receiver := extractHTMLValue(lines, "Credited Party name", "የገንዘብ ተቀባይ ስም")
	dateStr := extractHTMLValue(lines, "Payment date", "የክፍያ ቀን")
	amountStr := extractHTMLValue(lines, "Settled Amount", "የተከፈለው መጠን", "Total Paid Amount", "ጠቅላላ የተከፈለ")
	if amountStr == "" {
		return nil, fmt.Errorf("could not extract payment amount from receipt")
	}

	amount, err := parseAmount(amountStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse receipt amount %q: %w", amountStr, err)
	}

	return &ReceiptInfo{
		PaymentMethod:  "telebirr",
		Amount:         amount,
		TransactionRef: reference,
		TransferDate:   dateStr,
		ReceiverName:   receiver,
		Status:         firstNonEmpty(status, "Completed"),
		ReceiptURL:     url,
	}, nil
}

func extractHTMLValue(lines []string, labels ...string) string {
	for i, line := range lines {
		lowerLine := strings.ToLower(line)
		for _, label := range labels {
			if strings.Contains(lowerLine, strings.ToLower(label)) {
				// The value is in subsequent lines
				for j := i + 1; j < len(lines) && j <= i+5; j++ {
					val := strings.TrimSpace(lines[j])
					if val == "" || isHeaderLabel(val) {
						continue
					}
					return val
				}
			}
		}
	}
	return ""
}

func isHeaderLabel(s string) bool {
	labels := []string{"Payer", "Credited", "transaction", "Payment", "Invoice", "Settled",
		"Discount", "VAT", "Stamp", "Service", "Total", "Customer", "የከፋይ", "የገንዘብ", "የክፍያ"}
	lower := strings.ToLower(s)
	for _, l := range labels {
		if strings.Contains(lower, strings.ToLower(l)) && strings.Contains(s, "/") {
			return true
		}
	}
	return false
}

func (v *ReceiptVerifier) verifyCBE(ctx context.Context, transactionID string) (*ReceiptInfo, error) {
	if v.cbeReceiptAPI == "" {
		return nil, fmt.Errorf("CBE verification is not configured")
	}

	url := v.cbeReceiptAPI + "/" + path.Base(transactionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Origin", "https://mbreciept.cbe.com.et")
	req.Header.Set("Referer", "https://mbreciept.cbe.com.et/")
	req.Header.Set("X-App-ID", "d1292e42-7400-49de-a2d3-9731caa4c819")
	req.Header.Set("X-App-Version", "0a01980b-9859-1369-8198-59f403820000")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CBE verification request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("CBE verification returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID                  string   `json:"id"`
		AmountCredited      string   `json:"amountCredited"`
		CreditAccountHolder string   `json:"creditAccountHolder"`
		DateTimes           []string `json:"dateTimes"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode CBE verification response: %w", err)
	}

	amount, err := parseAmount(result.AmountCredited)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CBE amount: %w", err)
	}
	transferDate := ""
	if len(result.DateTimes) > 0 {
		transferDate = result.DateTimes[0]
	}
	return &ReceiptInfo{
		PaymentMethod:  "cbe",
		Amount:         amount,
		TransactionRef: firstNonEmpty(result.ID, path.Base(transactionID)),
		TransferDate:   transferDate,
		ReceiverName:   result.CreditAccountHolder,
		Status:         "Completed",
		ReceiptURL:     "https://mbreciept.cbe.com.et/receipt/" + firstNonEmpty(result.ID, path.Base(transactionID)),
	}, nil
}

func transactionPrompt(method string) string {
	switch method {
	case "telebirr":
		return `Analyze this Ethiopian Telebirr payment receipt screenshot. If it is not a Telebirr receipt, output exactly "Bank mismatch". Otherwise extract only the Transaction Number exactly as printed. If unreadable, output exactly "Not found". Do not include labels or formatting.`
	case "cbe":
		return `Analyze this Commercial Bank of Ethiopia (CBE) payment receipt screenshot. If it is not a CBE receipt, output exactly "Bank mismatch". CBE payment verification strictly requires the receipt QR code token or receipt URL (which looks like https://mbreciept.cbe.com.et/v2-... or starts with v2-). Extract ONLY the receipt token or URL containing "v2-". DO NOT extract the FT transaction number (such as FT25... or FT26...) because it cannot be verified. If no v2- token or receipt link is found in the QR code or text, output exactly "Not found".`
	default:
		return `Extract only the payment transaction number from this receipt image. If unreadable, output exactly "Not found".`
	}
}

func normalizeMethod(method string) string {
	method = strings.ToLower(strings.TrimSpace(method))
	if method == "commercial_bank_of_ethiopia" || method == "commercial bank of ethiopia" {
		return "cbe"
	}
	return method
}

func displayMethod(method string) string {
	if normalizeMethod(method) == "cbe" {
		return "CBE"
	}
	return "Telebirr"
}

func cleanTransactionID(value string) string {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "/") {
		value = path.Base(value)
	}
	re := regexp.MustCompile(`[A-Za-z0-9_-]+`)
	matches := re.FindAllString(value, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

func parseAmount(value string) (float64, error) {
	clean := strings.TrimSpace(value)
	clean = strings.ReplaceAll(clean, ",", "")
	clean = strings.ReplaceAll(clean, "ETB", "")
	clean = strings.ReplaceAll(clean, "Birr", "")
	clean = strings.ReplaceAll(clean, "birr", "")
	clean = strings.ReplaceAll(clean, "Br", "")
	clean = strings.TrimSpace(clean)
	amount, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, err
	}
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be greater than zero")
	}
	return amount, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
