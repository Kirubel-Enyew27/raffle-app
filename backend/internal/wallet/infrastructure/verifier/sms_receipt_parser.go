package verifier

import (
	"html"
	"regexp"
	"strings"
)

// Payment providers a pasted SMS can be attributed to.
const (
	SMSProviderTelebirr = "telebirr"
	SMSProviderCBE      = "cbe"
)

// SMS parse error codes. Handlers map these to user facing messages.
const (
	SMSErrorEmpty       = "sms_empty"
	SMSErrorUnsupported = "sms_unsupported_provider"
	SMSErrorAmbiguous   = "sms_ambiguous_provider"
	SMSErrorMissingRef  = "sms_missing_reference"
	SMSErrorInvalidURL  = "sms_invalid_receipt_url"
)

// SMSParseError explains why a pasted SMS could not be used. The message is
// written for the end user; it never contains provider or database internals.
type SMSParseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *SMSParseError) Error() string { return e.Message }

// SMSPaymentDetails is the payment information extracted from a confirmation
// SMS. It is only an input to provider verification: the provider receipt stays
// the source of truth for whether the payment happened and for how much was
// actually credited.
type SMSPaymentDetails struct {
	Provider        string  `json:"provider"`
	TransactionRef  string  `json:"transaction_ref,omitempty"`
	ReceiptURL      string  `json:"receipt_url,omitempty"`
	ReceiptToken    string  `json:"receipt_token,omitempty"`
	Amount          float64 `json:"amount,omitempty"`
	AmountRaw       string  `json:"amount_raw,omitempty"`
	TotalDebit      float64 `json:"total_debit,omitempty"`
	TotalDebitRaw   string  `json:"total_debit_raw,omitempty"`
	ReceiverName    string  `json:"receiver_name,omitempty"`
	PayerAccount    string  `json:"payer_account,omitempty"`
	TransactionDate string  `json:"transaction_date,omitempty"`
}

var (
	// Links inside SMS bodies are usually wrapped in brackets or followed by
	// punctuation, so the character class stops before those.
	smsURLRe = regexp.MustCompile(`(?i)https?://[^\s<>"'()\[\]]+`)

	smsAmountRe = `([0-9][0-9,]*(?:\.[0-9]+)?)`
	smsDateRe   = `(\d{2}[/-]\d{2}[/-]\d{4}(?:\s+\d{1,2}:\d{2}(?::\d{2})?)?)`

	// Telebirr
	telebirrTxnSMSRe = []*regexp.Regexp{
		regexp.MustCompile(`(?i)transaction\s+(?:number|no\.?)\s*(?:is|:|-)?\s*([A-Za-z0-9]{6,})`),
		regexp.MustCompile(`(?i)receipt\s+(?:number|no\.?)\s*(?:is|:|-)?\s*([A-Za-z0-9]{6,})`),
	}
	telebirrAmountSMSRe = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:transferred|transfer|paid|sent)\s*(?:ETB|Birr)\s*` + smsAmountRe),
		regexp.MustCompile(`(?i)(?:ETB|Birr)\s*` + smsAmountRe + `\s+(?:has\s+been\s+)?(?:transferred|sent|paid)`),
		regexp.MustCompile(`(?i)amount\s+(?:of\s+)?(?:ETB|Birr)\s*` + smsAmountRe),
	}
	telebirrReceiverSMSRe = []*regexp.Regexp{
		// The amount between the verb and the recipient contains a decimal point,
		// so only newlines and parentheses may stop the scan.
		regexp.MustCompile(`(?i)(?:transferred|sent|paid)[^\n()]*?\bto\s+([A-Za-z][A-Za-z .'\-]{1,60}?)\s*\(`),
		regexp.MustCompile(`(?i)credited\s+party(?:\s+name)?\s*[:\-]\s*([^\n()]+)`),
	}

	// CBE
	cbeAmountSMSRe = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:debit|credit)\s+transaction\s+of\s*(?:ETB|Birr)\s*` + smsAmountRe),
		regexp.MustCompile(`(?i)(?:ETB|Birr)\s*` + smsAmountRe + `\s+has\s+occurred`),
		regexp.MustCompile(`(?i)transaction\s+amount\s+(?:of\s+)?(?:ETB|Birr)\s*` + smsAmountRe),
	}
	cbeTotalDebitSMSRe = regexp.MustCompile(`(?i)(?:with\s+)?total\s+of\s*(?:ETB|Birr)\s*` + smsAmountRe)
	cbeBareTokenRe     = regexp.MustCompile(`(v2-[A-Za-z0-9_-]{4,})`)
	cbeFTRefRe         = regexp.MustCompile(`\b(FT[A-Z0-9]{6,})\b`)
)

// ParsePaymentSMS turns the text of a Telebirr or CBE confirmation SMS into the
// details needed to run the existing receipt verification pipeline. Anything it
// cannot attribute to exactly one provider is rejected rather than guessed.
func ParsePaymentSMS(raw string) (*SMSPaymentDetails, error) {
	text := normalizeSMSText(raw)
	if text == "" {
		return nil, &SMSParseError{
			Code:    SMSErrorEmpty,
			Message: "Paste the payment confirmation SMS you received from Telebirr or CBE.",
		}
	}

	telebirr := looksLikeTelebirrSMS(text)
	cbe := looksLikeCBESMS(text)

	switch {
	case telebirr && cbe:
		return nil, &SMSParseError{
			Code:    SMSErrorAmbiguous,
			Message: "That SMS mixes Telebirr and CBE details. Paste the original confirmation SMS from one provider.",
		}
	case telebirr:
		return parseTelebirrSMS(text)
	case cbe:
		return parseCBESMS(text)
	default:
		return nil, &SMSParseError{
			Code:    SMSErrorUnsupported,
			Message: "Only Telebirr and CBE payment confirmation SMS messages are supported.",
		}
	}
}

// normalizeSMSText makes the parser tolerant of HTML-encoded characters,
// non-breaking spaces, Windows line endings and padding whitespace.
func normalizeSMSText(raw string) string {
	text := html.UnescapeString(raw)
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = strings.ReplaceAll(text, "\u200b", "")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimSpace(text)
}

func looksLikeTelebirrSMS(text string) bool {
	lower := strings.ToLower(text)
	markers := []string{
		"telebirr",
		"ethiotelecom.et",
		"e-money account",
		"transaction number is",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func looksLikeCBESMS(text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "cbe.com.et") ||
		strings.Contains(lower, "commercial bank of ethiopia") ||
		strings.Contains(lower, "banking with cbe") {
		return true
	}
	return strings.Contains(lower, "debit transaction") && strings.Contains(lower, "has occurred")
}

func parseTelebirrSMS(text string) (*SMSPaymentDetails, error) {
	details := &SMSPaymentDetails{Provider: SMSProviderTelebirr}

	receiptURL, urlRef, err := telebirrReceiptLink(text)
	if err != nil {
		return nil, err
	}

	ref := matchSMSGroup(text, telebirrTxnSMSRe...)
	if ref == "" {
		ref = urlRef
	}
	if ref == "" {
		return nil, &SMSParseError{
			Code:    SMSErrorMissingRef,
			Message: "That SMS doesn't contain a Telebirr transaction number. Paste the original payment confirmation SMS.",
		}
	}
	details.TransactionRef = ref
	if receiptURL == "" {
		// The receipt endpoint is deterministic for a transaction number, so a
		// missing link in the SMS does not block verification.
		receiptURL = defaultTelebirrReceiptBaseURL + "/" + ref
	}
	details.ReceiptURL = receiptURL

	details.AmountRaw = matchSMSGroup(text, telebirrAmountSMSRe...)
	details.Amount = parseSMSAmount(details.AmountRaw)
	details.ReceiverName = matchSMSGroup(text, telebirrReceiverSMSRe...)
	details.TransactionDate = matchSMSGroup(text, regexp.MustCompile(smsDateRe))
	return details, nil
}

// telebirrReceiptLink returns the canonical receipt URL and the transaction
// number embedded in it. Non-HTTPS or foreign links are ignored: the URL is
// rebuilt from the transaction number instead of trusting the message.
func telebirrReceiptLink(text string) (receiptURL, ref string, err error) {
	for _, raw := range smsURLRe.FindAllString(text, -1) {
		link := strings.TrimRight(raw, ".,;:!")
		lower := strings.ToLower(link)
		if !strings.Contains(lower, "ethiotelecom.et") {
			continue
		}
		if !strings.HasPrefix(lower, "https://") {
			continue
		}
		if err := validateSSRFSafeURL(link); err != nil {
			return "", "", &SMSParseError{
				Code:    SMSErrorInvalidURL,
				Message: "The receipt link in that SMS is not a Telebirr receipt link. Paste the original payment confirmation SMS.",
			}
		}
		return link, extractTelebirrRefFromURL(link), nil
	}
	return "", "", nil
}

func parseCBESMS(text string) (*SMSPaymentDetails, error) {
	details := &SMSPaymentDetails{Provider: SMSProviderCBE}

	receiptURL, token, err := cbeReceiptToken(text)
	if err != nil {
		return nil, err
	}
	if receiptURL == "" && token != "" {
		receiptURL = "https://mbreciept.cbe.com.et/" + token
	}
	if token == "" {
		if ftRef := cbeFTRefRe.FindString(text); ftRef != "" {
			return nil, &SMSParseError{
				Code:    SMSErrorMissingRef,
				Message: "CBE needs the receipt link from the SMS, not the FT reference alone. Paste the original payment confirmation SMS.",
			}
		}
		return nil, &SMSParseError{
			Code:    SMSErrorMissingRef,
			Message: "That SMS doesn't contain a CBE receipt link or receipt token. Paste the original payment confirmation SMS.",
		}
	}

	details.ReceiptToken = token
	details.TransactionRef = token
	details.ReceiptURL = receiptURL
	details.AmountRaw = matchSMSGroup(text, cbeAmountSMSRe...)
	details.Amount = parseSMSAmount(details.AmountRaw)
	details.TotalDebitRaw = matchSMSGroup(text, cbeTotalDebitSMSRe)
	details.TotalDebit = parseSMSAmount(details.TotalDebitRaw)
	details.TransactionDate = matchSMSGroup(text, regexp.MustCompile(smsDateRe))
	return details, nil
}

// cbeReceiptToken finds the receipt token in a CBE receipt link, or a bare v2-
// token pasted without its link. A v2- token on a non-CBE host is rejected so a
// pasted message cannot redirect verification to an attacker-controlled domain.
func cbeReceiptToken(text string) (receiptURL, token string, err error) {
	for _, raw := range smsURLRe.FindAllString(text, -1) {
		link := strings.TrimRight(raw, ".,;:!")
		lower := strings.ToLower(link)
		if strings.Contains(lower, "cbe.com.et") {
			if err := validateSSRFSafeURL(link); err != nil {
				return "", "", &SMSParseError{
					Code:    SMSErrorInvalidURL,
					Message: "The receipt link in that SMS is not a CBE receipt link. Paste the original payment confirmation SMS.",
				}
			}
			if tok := ExtractCBEToken(link); tok != "" {
				return link, tok, nil
			}
			continue
		}
		if strings.Contains(lower, "v2-") {
			return "", "", &SMSParseError{
				Code:    SMSErrorInvalidURL,
				Message: "The receipt link in that SMS is not a CBE receipt link. Paste the original payment confirmation SMS.",
			}
		}
	}
	if tok := cbeBareTokenRe.FindStringSubmatch(text); len(tok) > 1 {
		return "", tok[1], nil
	}
	return "", "", nil
}

func matchSMSGroup(text string, patterns ...*regexp.Regexp) string {
	for _, re := range patterns {
		match := re.FindStringSubmatch(text)
		if len(match) < 2 {
			continue
		}
		value := strings.TrimSpace(match[1])
		value = strings.TrimSuffix(value, ".")
		if value == "" || isLabelLine(value) {
			continue
		}
		return value
	}
	return ""
}

func parseSMSAmount(raw string) float64 {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	amount, err := parseMonetaryAmount(raw)
	if err != nil {
		return 0
	}
	if amount < 0 {
		amount = -amount
	}
	return amount
}
