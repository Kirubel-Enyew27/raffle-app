package verifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/raffle-app/backend/internal/wallet/domain"
)

type TelebirrDirectVerifier struct {
	client        *http.Client
	baseURL       string
	fallbackURL   string
	fallbackKey   string
	linksETKey    string
	linksETBase   string
}

func NewTelebirrDirectVerifier(baseURL, fallbackURL, fallbackKey, linksETKey, linksETBase string, timeout time.Duration) *TelebirrDirectVerifier {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if baseURL == "" {
		baseURL = defaultTelebirrReceiptBaseURL
	}
	return &TelebirrDirectVerifier{
		client:      &http.Client{Timeout: timeout},
		baseURL:     strings.TrimRight(baseURL, "/"),
		fallbackURL: strings.TrimRight(fallbackURL, "/"),
		fallbackKey: fallbackKey,
		linksETKey:  linksETKey,
		linksETBase: strings.TrimRight(linksETBase, "/"),
	}
}

func (v *TelebirrDirectVerifier) Verify(ctx context.Context, req domain.VerificationRequest) (*domain.VerificationResult, error) {
	ref := strings.TrimSpace(req.TransactionRef)
	if ref == "" && req.RawURL != "" {
		ref = extractTelebirrRefFromURL(req.RawURL)
	}
	if ref == "" {
		return nil, fmt.Errorf("Telebirr verification requires a transaction number or receipt URL")
	}

	// 1. Primary: Direct Ethio Telecom HTML receipt scraping
	result, directErr := v.verifyDirectHTML(ctx, ref)
	if directErr == nil {
		return result, nil
	}

	// 2. Optional Fallback: links.et if configured
	if v.linksETKey != "" {
		result, linkErr := v.verifyViaLinksET(ctx, ref)
		if linkErr == nil {
			return result, nil
		}
	}

	// 3. Optional Fallback: Secondary proxy if configured
	if v.fallbackURL != "" && v.fallbackKey != "" {
		result, fallbackErr := v.verifyViaProxy(ctx, ref)
		if fallbackErr == nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("Telebirr verification failed: %w", directErr)
}

func (v *TelebirrDirectVerifier) verifyDirectHTML(ctx context.Context, ref string) (*domain.VerificationResult, error) {
	receiptURL := fmt.Sprintf("%s/%s", v.baseURL, ref)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, receiptURL, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14; SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Mobile Safari/537.36")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	httpReq.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := v.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethio Telecom receipt server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("receipt server returned status %d for reference %s", resp.StatusCode, ref)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read receipt content: %w", err)
	}

	htmlContent := string(bodyBytes)
	if strings.Contains(htmlContent, "This request is not correct") || strings.Contains(htmlContent, "Record not found") {
		return nil, fmt.Errorf("receipt not found for transaction reference %s", ref)
	}

	fields := parseTelebirrReceiptText(htmlContent)
	payerName := fields.PayerName
	payerPhone := fields.PayerAccount
	receiverName := fields.ReceiverName
	receiverPhone := fields.ReceiverAccount
	status := fields.Status
	paymentDate := fields.PaymentDate
	settledAmountStr := fields.SettledAmount
	totalAmountStr := fields.TotalAmount

	if status != "" && !strings.EqualFold(status, "Completed") {
		return nil, fmt.Errorf("Telebirr transaction is not completed (status: %s)", status)
	}

	// Authoritative amount credited to merchant/receiver is the Settled Amount
	amountStr := settledAmountStr
	if amountStr == "" {
		amountStr = totalAmountStr
	}
	if amountStr == "" {
		return nil, fmt.Errorf("could not extract payment amount from receipt")
	}

	settledAmount, err := parseMonetaryAmount(amountStr)
	if err != nil || settledAmount <= 0 {
		return nil, fmt.Errorf("invalid settled amount %q in Telebirr receipt: %w", amountStr, err)
	}

	totalAmount, _ := parseMonetaryAmount(totalAmountStr)
	fee := 0.0
	if totalAmount > settledAmount {
		fee = totalAmount - settledAmount
	}

	var parsedDate time.Time
	if paymentDate != "" {
		for _, layout := range []string{"02-01-2006 15:04:05", "02/01/2006 15:04:05", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, paymentDate); err == nil {
				parsedDate = t
				break
			}
		}
	}

	return &domain.VerificationResult{
		Provider:        "telebirr",
		TransactionID:   ref,
		Status:          strings.ToUpper(firstNonEmpty(status, "COMPLETED")),
		AmountCredited:  settledAmount,
		AmountDebited:   firstNonZero(totalAmount, settledAmount),
		FeeAmount:       fee,
		Currency:        "ETB",
		ReceiverName:    receiverName,
		ReceiverAccount: receiverPhone,
		PayerName:       payerName,
		PayerAccount:    payerPhone,
		TransactionTime: parsedDate,
		ReceiptURL:      receiptURL,
		RawPayload: map[string]interface{}{
			"settled_amount_raw": settledAmountStr,
			"total_amount_raw":   totalAmountStr,
			"payment_date_raw":   paymentDate,
		},
	}, nil
}

func (v *TelebirrDirectVerifier) verifyViaLinksET(ctx context.Context, ref string) (*domain.VerificationResult, error) {
	apiURL := v.linksETBase + "/api/verify"
	reqBody, _ := json.Marshal(map[string]string{
		"reference": ref,
	})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", v.linksETKey)

	resp, err := v.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("links.et returned status %d", resp.StatusCode)
	}

	var result struct {
		OK      bool `json:"ok"`
		Receipt struct {
			Source                 string `json:"source"`
			PayerName              string `json:"payerName"`
			PayerTelebirrNo        string `json:"payerTelebirrNo"`
			CreditedPartyName      string `json:"creditedPartyName"`
			CreditedPartyAccountNo string `json:"creditedPartyAccountNo"`
			TransactionStatus      string `json:"transactionStatus"`
			ReceiptNo              string `json:"receiptNo"`
			PaymentDate            string `json:"paymentDate"`
			SettledAmount          string `json:"settledAmount"`
			TotalPaidAmount        string `json:"totalPaidAmount"`
		} `json:"receipt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK || !strings.EqualFold(result.Receipt.TransactionStatus, "Completed") {
		return nil, fmt.Errorf("links.et verification failed or uncompleted")
	}

	settled, err := parseMonetaryAmount(result.Receipt.SettledAmount)
	if err != nil || settled <= 0 {
		settled, err = parseMonetaryAmount(result.Receipt.TotalPaidAmount)
		if err != nil || settled <= 0 {
			return nil, fmt.Errorf("invalid amount from links.et")
		}
	}
	total, _ := parseMonetaryAmount(result.Receipt.TotalPaidAmount)

	return &domain.VerificationResult{
		Provider:        "telebirr",
		TransactionID:   firstNonEmpty(result.Receipt.ReceiptNo, ref),
		Status:          "COMPLETED",
		AmountCredited:  settled,
		AmountDebited:   firstNonZero(total, settled),
		FeeAmount:       total - settled,
		Currency:        "ETB",
		ReceiverName:    result.Receipt.CreditedPartyName,
		ReceiverAccount: result.Receipt.CreditedPartyAccountNo,
		PayerName:       result.Receipt.PayerName,
		PayerAccount:    result.Receipt.PayerTelebirrNo,
		ReceiptURL:      "https://transactioninfo.ethiotelecom.et/receipt/" + ref,
	}, nil
}

func (v *TelebirrDirectVerifier) verifyViaProxy(ctx context.Context, ref string) (*domain.VerificationResult, error) {
	reqBody, _ := json.Marshal(map[string]string{"reference": ref})
	proxyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(proxyCtx, http.MethodPost, v.fallbackURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", v.fallbackKey)

	resp, err := v.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxy verification returned status %d", resp.StatusCode)
	}

	var res struct {
		Success bool `json:"success"`
		Data    struct {
			PayerName              string `json:"payerName"`
			PayerTelebirrNo        string `json:"payerTelebirrNo"`
			CreditedPartyName      string `json:"creditedPartyName"`
			CreditedPartyAccountNo string `json:"creditedPartyAccountNo"`
			TransactionStatus      string `json:"transactionStatus"`
			ReceiptNo              string `json:"receiptNo"`
			PaymentDate            string `json:"paymentDate"`
			SettledAmount          string `json:"settledAmount"`
			TotalPaidAmount        string `json:"totalPaidAmount"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	if !res.Success || !strings.EqualFold(res.Data.TransactionStatus, "Completed") {
		return nil, fmt.Errorf("proxy returned uncompleted transaction")
	}

	settled, err := parseMonetaryAmount(res.Data.SettledAmount)
	if err != nil || settled <= 0 {
		settled, err = parseMonetaryAmount(res.Data.TotalPaidAmount)
		if err != nil || settled <= 0 {
			return nil, fmt.Errorf("invalid settled amount from proxy")
		}
	}
	total, _ := parseMonetaryAmount(res.Data.TotalPaidAmount)

	return &domain.VerificationResult{
		Provider:        "telebirr",
		TransactionID:   firstNonEmpty(res.Data.ReceiptNo, ref),
		Status:          "COMPLETED",
		AmountCredited:  settled,
		AmountDebited:   firstNonZero(total, settled),
		FeeAmount:       total - settled,
		Currency:        "ETB",
		ReceiverName:    res.Data.CreditedPartyName,
		ReceiverAccount: res.Data.CreditedPartyAccountNo,
		PayerName:       res.Data.PayerName,
		PayerAccount:    res.Data.PayerTelebirrNo,
		ReceiptURL:      "https://transactioninfo.ethiotelecom.et/receipt/" + ref,
	}, nil
}

func isLabelLine(s string) bool {
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

func extractTelebirrRefFromURL(u string) string {
	re := regexp.MustCompile(`receipt/([A-Za-z0-9]+)`)
	matches := re.FindStringSubmatch(u)
	if len(matches) > 1 {
		return matches[1]
	}
	parts := strings.Split(strings.TrimRight(u, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstNonZero(vals ...float64) float64 {
	for _, v := range vals {
		if v > 0 {
			return v
		}
	}
	return 0
}
