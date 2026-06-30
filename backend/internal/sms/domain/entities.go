package domain

import (
	"regexp"
	"strings"
)

// WebhookRequest is the payload received from the SMS Forwarder app.
type WebhookRequest struct {
	Sender     string `json:"sender" binding:"required"`
	Message    string `json:"message" binding:"required"`
	ReceivedAt string `json:"received_at"`
}

// WebhookResponse is returned after processing an SMS webhook.
type WebhookResponse struct {
	Status        string  `json:"status"`
	TransactionID string  `json:"transaction_id"`
	Amount        float64 `json:"amount"`
	Verified      bool    `json:"verified"`
}

// Accepts if sender is "127" OR the message contains the known Telebirr phrase.
func IsTelebirrSMS(sender, message string) bool {
	sender = strings.TrimSpace(sender)
	message = strings.TrimSpace(message)
	if sender == "127" {
		return true
	}
	return strings.Contains(message, "Your transaction number is")
}

// The SMS contains: "Your transaction number is DFM26GE790."
func ExtractTransactionID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	re := regexp.MustCompile(`transaction number is\s+(\S+)`)
	match := re.FindStringSubmatch(raw)
	if len(match) < 2 {
		return "", ErrTransactionIDNotFound
	}
	txID := strings.TrimSpace(match[1])
	// Remove trailing punctuation
	txID = strings.TrimRight(txID, ".")
	if txID == "" {
		return "", ErrTransactionIDNotFound
	}
	return txID, nil
}

// Returns the sender number and the remaining message text.
func ParseRawSMS(raw string) (sender, message string) {
	raw = strings.TrimSpace(raw)
	prefix := "from : "
	if strings.HasPrefix(strings.ToLower(raw), prefix) {
		rest := strings.TrimSpace(raw[len(prefix):])
		// The sender is the first word after "From : "
		parts := strings.SplitN(rest, " ", 2)
		if len(parts) >= 2 {
			sender = parts[0]
			message = parts[1]
		} else {
			sender = parts[0]
			message = ""
		}
	} else {
		// No "From : " prefix — use full body as message
		sender = ""
		message = raw
	}
	return sender, message
}

// ErrTransactionIDNotFound is returned when no transaction ID can be extracted.
var ErrTransactionIDNotFound = &ParseError{Message: "transaction ID not found in SMS"}

// ParseError represents an SMS parsing error.
type ParseError struct {
	Message string
}

func (e *ParseError) Error() string {
	return e.Message
}
