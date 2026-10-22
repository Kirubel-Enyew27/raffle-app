package verifier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/raffle-app/backend/internal/wallet/domain"
)

type CBEDirectVerifier struct {
	client     *http.Client
	baseURL    string
	appID      string
	appVersion string
}

func NewCBEDirectVerifier(baseURL, appID, appVersion string, timeout time.Duration) *CBEDirectVerifier {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if baseURL == "" {
		baseURL = "https://mb.cbe.com.et/api/v1/transactions/public/transaction-detail"
	}
	if appID == "" {
		appID = "d1292e42-7400-49de-a2d3-9731caa4c819"
	}
	if appVersion == "" {
		appVersion = "0a01980b-9859-1369-8198-59f403820000"
	}
	return &CBEDirectVerifier{
		client:     &http.Client{Timeout: timeout},
		baseURL:    strings.TrimRight(baseURL, "/"),
		appID:      appID,
		appVersion: appVersion,
	}
}

// ExtractCBEToken cleans raw tokens, paths, or URLs (e.g. "https://mbreciept.cbe.com.et/v2-abc" -> "v2-abc")
func ExtractCBEToken(input string) string {
	input = strings.TrimSpace(input)
	re := regexp.MustCompile(`v2-[A-Za-z0-9_-]+`)
	if match := re.FindString(input); match != "" {
		return match
	}
	if strings.Contains(input, "(") {
		input = strings.Split(input, "(")[0]
		input = strings.TrimSpace(input)
	}
	if strings.Contains(input, "://") {
		u, err := url.Parse(input)
		if err == nil {
			input = path.Base(u.Path)
		}
	} else if strings.Contains(input, "/") {
		input = path.Base(input)
	}
	return strings.TrimSpace(input)
}

func (v *CBEDirectVerifier) Verify(ctx context.Context, req domain.VerificationRequest) (*domain.VerificationResult, error) {
	token := req.ReceiptToken
	if token == "" {
		token = ExtractCBEToken(req.TransactionRef)
	}
	if token == "" && req.RawURL != "" {
		token = ExtractCBEToken(req.RawURL)
	}
	if token == "" {
		return nil, fmt.Errorf("CBE verification requires a receipt token or receipt URL containing a V2 token")
	}

	// Defensive check: if user passes an FT transaction ID (e.g. FT26248LDXLB) directly as token,
	// warn that CBE requires the receipt token from the QR code or link.
	if strings.HasPrefix(token, "FT") && len(token) >= 10 && !strings.Contains(token, "-") {
		return nil, fmt.Errorf("CBE direct verification requires the receipt token from the receipt QR code or link, not the raw FT reference alone")
	}

	reqURL := fmt.Sprintf("%s/%s", v.baseURL, url.PathEscape(token))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create CBE verification request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	httpReq.Header.Set("Origin", "https://mbreciept.cbe.com.et")
	httpReq.Header.Set("Referer", "https://mbreciept.cbe.com.et/")
	httpReq.Header.Set("X-App-ID", v.appID)
	httpReq.Header.Set("X-App-Version", v.appVersion)

	resp, err := v.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("CBE verification request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read CBE response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Detail  string `json:"detail"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(bodyBytes, &errResp)
		msg := errResp.Detail
		if msg == "" {
			msg = errResp.Error
		}
		if msg == "" {
			msg = errResp.Message
		}
		if msg == "" {
			msg = string(bodyBytes)
		}
		return nil, fmt.Errorf("CBE verification returned status %d: %s", resp.StatusCode, msg)
	}

	var data struct {
		ID                  string   `json:"id"`
		TransactionType     string   `json:"transactionType"`
		DebitAccountNo      string   `json:"debitAccountNo"`
		DebitCurrency       string   `json:"debitCurrency"`
		DebitAmount         string   `json:"debitAmount"`
		CreditAccountNo     string   `json:"creditAccountNo"`
		CreditCurrency      string   `json:"creditCurrency"`
		CreditAccountHolder string   `json:"creditAccountHolder"`
		DebitAccountHolder  string   `json:"debitAccountHolder"`
		AmountDebited       string   `json:"amountDebited"`
		AmountCredited      string   `json:"amountCredited"`
		TotalChargeAmount   string   `json:"totalChargeAmount"`
		DateTimes           []string `json:"dateTimes"`
		EncodedReceipt      string   `json:"encodedReceipt"`
		Status              string   `json:"status"`
	}

	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("failed to parse CBE response: %w", err)
	}

	if data.ID == "" {
		return nil, fmt.Errorf("CBE response did not contain a valid transaction ID")
	}

	if !strings.EqualFold(data.Status, "COMPLETED") {
		return nil, fmt.Errorf("CBE transaction is not completed (status: %s)", data.Status)
	}

	// Amount credited is the authoritative transferred amount received by the account holder
	amountCredited, err := parseMonetaryAmount(data.AmountCredited)
	if err != nil || amountCredited <= 0 {
		return nil, fmt.Errorf("invalid credited amount in CBE response: %q", data.AmountCredited)
	}

	amountDebited, _ := parseMonetaryAmount(data.AmountDebited)
	feeAmount, _ := parseMonetaryAmount(data.TotalChargeAmount)

	var txTime time.Time
	if len(data.DateTimes) > 0 {
		parsedTime, err := time.Parse(time.RFC3339, data.DateTimes[0])
		if err == nil {
			txTime = parsedTime
		}
	}

	currency := data.CreditCurrency
	if currency == "" {
		currency = "ETB"
	}

	var rawMap map[string]interface{}
	_ = json.Unmarshal(bodyBytes, &rawMap)

	receiptURL := data.EncodedReceipt
	if receiptURL == "" {
		receiptURL = "https://mbreciept.cbe.com.et/" + token
	}

	return &domain.VerificationResult{
		Provider:        "cbe",
		TransactionID:   data.ID,
		Status:          strings.ToUpper(data.Status),
		AmountCredited:  amountCredited,
		AmountDebited:   amountDebited,
		FeeAmount:       feeAmount,
		Currency:        currency,
		ReceiverName:    data.CreditAccountHolder,
		ReceiverAccount: data.CreditAccountNo,
		PayerName:       data.DebitAccountHolder,
		PayerAccount:    data.DebitAccountNo,
		TransactionTime: txTime,
		ReceiptURL:      receiptURL,
		RawPayload:      rawMap,
	}, nil
}

func parseMonetaryAmount(val string) (float64, error) {
	clean := strings.TrimSpace(val)
	clean = strings.ReplaceAll(clean, ",", "")
	clean = strings.ReplaceAll(clean, "ETB", "")
	clean = strings.ReplaceAll(clean, "Birr", "")
	clean = strings.ReplaceAll(clean, "birr", "")
	clean = strings.TrimSpace(clean)
	re := regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?`)
	match := re.FindString(clean)
	if match == "" {
		return 0, fmt.Errorf("no numeric amount found in %q", val)
	}
	return strconv.ParseFloat(match, 64)
}
