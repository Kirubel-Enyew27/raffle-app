package domain

import (
	"fmt"
	"regexp"
	"strconv"
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
		// Use Fields to split on any whitespace (spaces, newlines, tabs)
		// The sender is the first word after "From : " — e.g. "127" from "127\nDear..."
		parts := strings.Fields(rest)
		if len(parts) >= 2 {
			sender = parts[0]
			message = strings.Join(parts[1:], " ")
		} else if len(parts) == 1 {
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

// ErrAmountNotFound is returned when the amount cannot be extracted from the SMS.
var ErrAmountNotFound = &ParseError{Message: "amount not found in SMS"}

// ErrPayerNameNotFound is returned when the payer name cannot be extracted from the SMS.
var ErrPayerNameNotFound = &ParseError{Message: "payer name not found in SMS"}

// ExtractAmount extracts the ETB amount from a Telebirr SMS message.
// Example: "You have received ETB 1.00 from ..." → 1.00
func ExtractAmount(message string) (float64, error) {
	re := regexp.MustCompile(`received\s+ETB\s+([0-9,]+(?:\.[0-9]+)?)`)
	match := re.FindStringSubmatch(message)
	if len(match) < 2 {
		return 0, ErrAmountNotFound
	}
	amountStr := strings.ReplaceAll(match[1], ",", "")
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		return 0, fmt.Errorf("invalid amount in SMS: %q", match[1])
	}
	return amount, nil
}

// ExtractPayerName extracts the sender's name from a Telebirr SMS message.
// Example: "from Tewodros Misawoy(2519****5426)" → "Tewodros Misawoy"
func ExtractPayerName(message string) (string, error) {
	re := regexp.MustCompile(`from\s+([^(]+)`)
	match := re.FindStringSubmatch(message)
	if len(match) < 2 {
		return "", ErrPayerNameNotFound
	}
	name := strings.TrimSpace(match[1])
	if name == "" {
		return "", ErrPayerNameNotFound
	}
	return name, nil
}

// ParseError represents an SMS parsing error.
type ParseError struct {
	Message string
}

func (e *ParseError) Error() string {
	return e.Message
}
