package verifier

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/raffle-app/backend/internal/wallet/domain"
)

type VerificationManager struct {
	cbeVerifier         domain.PaymentVerifier
	telebirrVerifier    domain.PaymentVerifier
	imageExtractor      *ImageExtractor
	cbeAccountName      string
	cbeAccountNumber    string
	telebirrAccountName string
	telebirrPhone       string
	autoCreditEnabled   bool
}

func NewVerificationManager(
	cbeVerifier domain.PaymentVerifier,
	telebirrVerifier domain.PaymentVerifier,
	imageExtractor *ImageExtractor,
	cbeAccountName, cbeAccountNumber, telebirrAccountName, telebirrPhone string,
	autoCreditEnabled bool,
) *VerificationManager {
	return &VerificationManager{
		cbeVerifier:         cbeVerifier,
		telebirrVerifier:    telebirrVerifier,
		imageExtractor:      imageExtractor,
		cbeAccountName:      cbeAccountName,
		cbeAccountNumber:    cbeAccountNumber,
		telebirrAccountName: telebirrAccountName,
		telebirrPhone:       telebirrPhone,
		autoCreditEnabled:   autoCreditEnabled,
	}
}

func (m *VerificationManager) ImageExtractor() *ImageExtractor {
	return m.imageExtractor
}

func (m *VerificationManager) AutoCreditEnabled() bool {
	return m.autoCreditEnabled
}

// IsPlaceholder checks if configured receiving account details have not been finalized
func (m *VerificationManager) IsPlaceholder(method string) bool {
	switch strings.ToLower(method) {
	case "cbe":
		return strings.HasPrefix(m.cbeAccountName, "PLACEHOLDER_") || m.cbeAccountName == ""
	case "telebirr":
		return strings.HasPrefix(m.telebirrAccountName, "PLACEHOLDER_") || m.telebirrAccountName == ""
	default:
		return true
	}
}

func (m *VerificationManager) Verify(ctx context.Context, req domain.VerificationRequest) (*domain.VerificationResult, error) {
	method := strings.ToLower(strings.TrimSpace(req.Method))
	if req.RawURL != "" {
		if err := validateSSRFSafeURL(req.RawURL); err != nil {
			return nil, fmt.Errorf("invalid or untrusted receipt URL: %w", err)
		}
	}

	var verifier domain.PaymentVerifier
	switch method {
	case "cbe":
		verifier = m.cbeVerifier
	case "telebirr":
		verifier = m.telebirrVerifier
	default:
		return nil, fmt.Errorf("unsupported payment method: %s", method)
	}

	result, err := verifier.Verify(ctx, req)
	if err != nil {
		return nil, err
	}

	// 1. Transaction Status Validation
	if !strings.EqualFold(result.Status, "COMPLETED") {
		return nil, fmt.Errorf("payment status is %q, expected COMPLETED", result.Status)
	}

	// 2. Expected Amount Validation
	if req.ExpectedAmount > 0 {
		diff := result.AmountCredited - req.ExpectedAmount
		if diff < -0.01 || diff > 0.01 {
			return nil, fmt.Errorf("verified amount (%.2f ETB) does not match expected deposit amount (%.2f ETB)", result.AmountCredited, req.ExpectedAmount)
		}
	}

	// 3. Currency Validation
	if result.Currency != "" && !strings.EqualFold(result.Currency, "ETB") {
		return nil, fmt.Errorf("unsupported currency %q, expected ETB", result.Currency)
	}

	// 4. Recipient Validation & Auto-Credit Guard
	// If server configuration still contains placeholders, refuse automatic crediting
	if m.IsPlaceholder(method) {
		return nil, fmt.Errorf("automatic crediting is disabled: receiving account details for %s have not yet been configured on the server", strings.ToUpper(method))
	}

	if !m.autoCreditEnabled {
		return nil, fmt.Errorf("automatic wallet crediting is currently disabled by server configuration")
	}

	// Validate recipient against configured account holder name
	if err := m.validateRecipientIdentity(method, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (m *VerificationManager) validateRecipientIdentity(method string, result *domain.VerificationResult) error {
	switch method {
	case "cbe":
		if !namesMatch(result.ReceiverName, m.cbeAccountName) {
			return fmt.Errorf("recipient account holder %q does not match application designated account %q", result.ReceiverName, m.cbeAccountName)
		}
		// If masked account check is possible (e.g. 1000249277383 against 1********7383 or 1********8242)
		if m.cbeAccountNumber != "" && result.ReceiverAccount != "" {
			if !maskedAccountMatches(m.cbeAccountNumber, result.ReceiverAccount) {
				return fmt.Errorf("receiving account number does not match designated account")
			}
		}
	case "telebirr":
		if !namesMatch(result.ReceiverName, m.telebirrAccountName) {
			return fmt.Errorf("recipient party %q does not match application designated account %q", result.ReceiverName, m.telebirrAccountName)
		}
		if m.telebirrPhone != "" && result.ReceiverAccount != "" {
			if !maskedPhoneMatches(m.telebirrPhone, result.ReceiverAccount) {
				return fmt.Errorf("receiving phone number does not match designated account")
			}
		}
	}
	return nil
}

func namesMatch(actual, expected string) bool {
	act := strings.ToLower(strings.TrimSpace(actual))
	exp := strings.ToLower(strings.TrimSpace(expected))
	if act == exp {
		return true
	}
	if act == "" || exp == "" {
		return false
	}
	// Support matching if either name is a substring of the other (e.g. "Samuel")
	if strings.Contains(act, exp) || strings.Contains(exp, act) {
		return true
	}
	// Token overlap: check if any significant first/last name matches
	actWords := strings.Fields(act)
	expWords := strings.Fields(exp)
	for _, aw := range actWords {
		if len(aw) < 3 {
			continue
		}
		for _, ew := range expWords {
			if len(ew) >= 3 && (aw == ew || strings.HasPrefix(aw, ew) || strings.HasPrefix(ew, aw)) {
				return true
			}
		}
	}
	return false
}

func maskedAccountMatches(fullAccount, maskedAccount string) bool {
	full := strings.TrimSpace(fullAccount)
	masked := strings.TrimSpace(maskedAccount)
	if full == "" || masked == "" {
		return true
	}
	// Extract trailing digits if any (e.g. "8242" from "ETB-8242" or "1********8242")
	reDigits := regexp.MustCompile(`[0-9]+`)
	maskedDigits := reDigits.FindAllString(masked, -1)
	if len(maskedDigits) > 0 {
		lastMasked := maskedDigits[len(maskedDigits)-1]
		if len(lastMasked) >= 4 && strings.HasSuffix(full, lastMasked) {
			return true
		}
	}
	if strings.Contains(masked, "*") {
		parts := strings.Split(masked, "*")
		prefix := parts[0]
		suffix := parts[len(parts)-1]
		return strings.HasPrefix(full, prefix) && strings.HasSuffix(full, suffix)
	}
	return strings.HasSuffix(full, masked) || strings.Contains(full, masked)
}

func maskedPhoneMatches(fullPhone, maskedPhone string) bool {
	full := normalizePhone(fullPhone)
	masked := normalizePhone(maskedPhone)
	if full == "" || masked == "" {
		return true
	}
	reDigits := regexp.MustCompile(`[0-9]+`)
	maskedDigits := reDigits.FindAllString(masked, -1)
	if len(maskedDigits) > 0 {
		lastMasked := maskedDigits[len(maskedDigits)-1]
		if len(lastMasked) >= 4 && strings.HasSuffix(full, lastMasked) {
			return true
		}
	}
	if strings.Contains(masked, "*") {
		parts := strings.Split(masked, "*")
		prefix := parts[0]
		suffix := parts[len(parts)-1]
		return strings.HasPrefix(full, prefix) && strings.HasSuffix(full, suffix)
	}
	return strings.HasSuffix(full, masked)
}

func normalizePhone(p string) string {
	clean := strings.ReplaceAll(strings.ReplaceAll(p, "+", ""), " ", "")
	clean = strings.ReplaceAll(clean, "-", "")
	// If starts with 09 or 07, convert to 2519 or 2517
	if strings.HasPrefix(clean, "09") && len(clean) == 10 {
		clean = "2519" + clean[2:]
	} else if strings.HasPrefix(clean, "07") && len(clean) == 10 {
		clean = "2517" + clean[2:]
	}
	return clean
}

func validateSSRFSafeURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("malformed URL")
	}
	if u.Scheme != "https" {
		return fmt.Errorf("only HTTPS URLs are allowed")
	}
	allowedDomains := []string{
		"mb.cbe.com.et",
		"mbreciept.cbe.com.et",
		"transactioninfo.ethiotelecom.et",
		"links.et",
		"verify.et",
	}
	host := strings.ToLower(u.Hostname())
	allowed := false
	for _, d := range allowedDomains {
		if host == d || strings.HasSuffix(host, "."+d) {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("untrusted domain: %s", host)
	}
	return nil
}
