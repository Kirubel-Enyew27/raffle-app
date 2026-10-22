package verifier

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

type ImageExtractor struct {
	geminiAPIKey string
	httpClient   *http.Client
}

func NewImageExtractor(geminiAPIKey string) *ImageExtractor {
	return &ImageExtractor{
		geminiAPIKey: geminiAPIKey,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// ReceiptExtraction is the outcome of reading a payment receipt image.
// Source records which strategy produced the values:
//   - "regex": the receipt QR code plus the receipt page, no OCR model used
//   - "gemini-ocr": Gemini transcribed the screenshot, fields parsed with regex
//   - "gemini": Gemini returned a single transaction number (legacy fallback)
//   - "qr": a QR code token (CBE)
type ReceiptExtraction struct {
	TransactionID   string  `json:"transaction_id"`
	Amount          float64 `json:"amount,omitempty"`
	AmountRaw       string  `json:"amount_raw,omitempty"`
	ReceiverName    string  `json:"receiver_name,omitempty"`
	TransactionTime string  `json:"transaction_time,omitempty"`
	Source          string  `json:"source,omitempty"`
}

// Extract extracts a transaction reference or QR receipt token from image bytes
func (e *ImageExtractor) Extract(ctx context.Context, method string, imageBytes []byte, contentType string) (string, error) {
	if len(imageBytes) == 0 {
		return "", fmt.Errorf("empty image payload")
	}

	normMethod := strings.ToLower(strings.TrimSpace(method))
	if normMethod == "telebirr" {
		result, err := e.extractTelebirrDetails(ctx, imageBytes, contentType)
		if err != nil {
			return "", err
		}
		return result.TransactionID, nil
	}
	return e.extractQRorGemini(ctx, normMethod, imageBytes, contentType)
}

// ExtractDetails reads the transaction number and, when available, the amount
// from a receipt image. Telebirr receipts are parsed with regex extraction first
// and only fall back to Gemini OCR when that fails.
func (e *ImageExtractor) ExtractDetails(ctx context.Context, method string, imageBytes []byte, contentType string) (*ReceiptExtraction, error) {
	if len(imageBytes) == 0 {
		return nil, fmt.Errorf("empty image payload")
	}

	normMethod := strings.ToLower(strings.TrimSpace(method))
	if normMethod == "telebirr" {
		return e.extractTelebirrDetails(ctx, imageBytes, contentType)
	}

	ref, err := e.extractQRorGemini(ctx, normMethod, imageBytes, contentType)
	if err != nil {
		return nil, err
	}
	return &ReceiptExtraction{TransactionID: ref, Source: "qr"}, nil
}

// extractTelebirrDetails extracts the transaction number and amount from a
// Telebirr receipt image.
//
// The receipt QR code is deliberately not decoded: a screenshot is transcribed
// by Gemini OCR and the transaction number, amount, receiver and date are then
// regex-extracted from the transcription. The amount is informational only —
// crediting always re-reads the Ethio Telecom receipt page from the transaction
// number during verification.
func (e *ImageExtractor) extractTelebirrDetails(ctx context.Context, imageBytes []byte, contentType string) (*ReceiptExtraction, error) {
	if e.geminiAPIKey != "" {
		if text, err := e.ocrViaGemini(ctx, imageBytes, contentType); err == nil && strings.TrimSpace(text) != "" {
			fields := parseTelebirrReceiptText(text)
			if fields.TransactionRef != "" {
				amount, amountRaw := telebirrAmount(fields)
				return &ReceiptExtraction{
					TransactionID:   fields.TransactionRef,
					Amount:          amount,
					AmountRaw:       amountRaw,
					ReceiverName:    fields.ReceiverName,
					TransactionTime: fields.PaymentDate,
					Source:          "gemini-ocr",
				}, nil
			}
		}

		// Last resort: the previous single field Gemini extraction.
		if ref, err := e.extractViaGemini(ctx, "telebirr", imageBytes, contentType); err == nil && ref != "" {
			return &ReceiptExtraction{TransactionID: ref, Source: "gemini"}, nil
		}
	}

	return nil, fmt.Errorf("could not extract transaction number from Telebirr receipt image")
}

// extractQRorGemini handles the non Telebirr flow: QR code decoding first, then
// Gemini Vision OCR as a fallback. It returns the transaction reference or token.
func (e *ImageExtractor) extractQRorGemini(ctx context.Context, normMethod string, imageBytes []byte, contentType string) (string, error) {
	// 1. First attempt: In-memory QR code decoding via gozxing
	qrText, err := decodeQRCode(imageBytes)
	if err == nil && qrText != "" {
		if strings.Contains(qrText, "mbreciept.cbe.com.et") || strings.HasPrefix(qrText, "v2-") {
			token := ExtractCBEToken(qrText)
			if token != "" {
				return token, nil
			}
		}
		if normMethod == "cbe" {
			token := ExtractCBEToken(qrText)
			if strings.HasPrefix(token, "v2-") {
				return token, nil
			}
		} else {
			return strings.TrimSpace(qrText), nil
		}
	}

	// 2. Second attempt: Gemini Vision OCR fallback
	if e.geminiAPIKey != "" {
		text, err := e.extractViaGemini(ctx, normMethod, imageBytes, contentType)
		if err == nil && text != "" {
			if normMethod == "cbe" {
				token := ExtractCBEToken(text)
				if strings.HasPrefix(token, "v2-") {
					return token, nil
				}
				return "", fmt.Errorf("could not extract CBE receipt QR token from image; CBE requires the receipt QR code or link (https://mbreciept.cbe.com.et/...), not the FT transaction reference alone")
			}
			return text, nil
		}
	}

	if normMethod == "cbe" {
		return "", fmt.Errorf("could not detect CBE receipt QR code; please ensure the QR code is clearly visible or enter the receipt link (https://mbreciept.cbe.com.et/...)")
	}
	return "", fmt.Errorf("could not extract transaction number or QR code from image")
}

func decodeQRCode(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	// Try full image bitmap
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err == nil {
		qrReader := qrcode.NewQRCodeReader()
		result, err := qrReader.Decode(bmp, nil)
		if err == nil && result != nil && result.GetText() != "" {
			return result.GetText(), nil
		}
	}

	// If full image fails, crop regions where mobile banking receipts place QR codes (center/lower half)
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if h > 400 && w > 200 {
		// Sub-crops: center-bottom (y: 40% to 85%, x: 15% to 85%)
		type subImager interface {
			SubImage(r image.Rectangle) image.Image
		}
		if si, ok := img.(subImager); ok {
			cropRect := image.Rect(
				bounds.Min.X+int(float64(w)*0.15),
				bounds.Min.Y+int(float64(h)*0.45),
				bounds.Min.X+int(float64(w)*0.85),
				bounds.Min.Y+int(float64(h)*0.85),
			)
			cropped := si.SubImage(cropRect)
			if croppedBmp, err := gozxing.NewBinaryBitmapFromImage(cropped); err == nil {
				qrReader := qrcode.NewQRCodeReader()
				result, err := qrReader.Decode(croppedBmp, nil)
				if err == nil && result != nil && result.GetText() != "" {
					return result.GetText(), nil
				}
			}
		}
	}

	return "", fmt.Errorf("no QR code detected")
}

func (e *ImageExtractor) extractViaGemini(ctx context.Context, method string, imageBytes []byte, contentType string) (string, error) {
	prompt := "Analyze this Ethiopian payment receipt. If it is a Telebirr receipt, extract ONLY the Transaction Number (e.g. DGP88F2T36). If it is a CBE receipt, extract the receipt URL (https://mbreciept.cbe.com.et/v2-...) or token starting with 'v2-'. Do NOT extract FT numbers. If not found, output exactly 'Not found'. Do not include labels."
	if strings.ToLower(method) == "telebirr" {
		prompt = "Analyze this Ethiopian Telebirr payment receipt. Extract only the Transaction Number (e.g. DGP88F2T36). If unreadable, output 'Not found'. Do not include labels."
	} else if strings.ToLower(method) == "cbe" {
		prompt = "Analyze this Commercial Bank of Ethiopia (CBE) receipt. Extract ONLY the receipt verification link (e.g. https://mbreciept.cbe.com.et/v2-...) or the receipt token starting with 'v2-'. DO NOT extract the FT reference number (like FT25... or FT26...) because it cannot be used for verification. If no v2- token or receipt URL is found, output 'Not found'. Do not include labels."
	}

	text, err := e.callGemini(ctx, imageBytes, contentType, prompt)
	if err != nil {
		return "", err
	}

	if strings.EqualFold(text, "not found") || text == "" {
		return "", fmt.Errorf("could not detect transaction reference from receipt")
	}

	if strings.ToLower(method) == "cbe" {
		re := regexp.MustCompile(`(https?://mbreciept\.cbe\.com\.et/[A-Za-z0-9_-]+)|(v2-[A-Za-z0-9_-]+)`)
		match := re.FindString(text)
		if match != "" {
			return ExtractCBEToken(match), nil
		}
		return "", fmt.Errorf("could not detect CBE V2 receipt token from receipt image")
	}

	re := regexp.MustCompile(`[A-Za-z0-9_-]+`)
	matches := re.FindAllString(text, -1)
	if len(matches) > 0 {
		return matches[len(matches)-1], nil
	}
	return text, nil
}

// ocrViaGemini transcribes a receipt screenshot and returns the raw text so that
// the caller can parse the transaction number and amount with regexes.
func (e *ImageExtractor) ocrViaGemini(ctx context.Context, imageBytes []byte, contentType string) (string, error) {
	if e.geminiAPIKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is not configured")
	}

	prompt := `You are a precise OCR engine. Transcribe every visible line of this Ethiopian Telebirr payment receipt screenshot exactly as printed, keeping each label and its value together on one line, for example "Transaction Number: DGP88F2T36" and "Total Paid Amount: 387.00 Birr". Output only the transcribed lines. If the image is not a Telebirr receipt, output exactly "Bank mismatch".`

	text, err := e.callGemini(ctx, imageBytes, contentType, prompt)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(strings.ToLower(text), "bank mismatch") {
		return "", fmt.Errorf("the uploaded image does not look like a Telebirr receipt")
	}
	return text, nil
}

// callGemini sends an image plus prompt to Gemini and returns the cleaned text reply.
func (e *ImageExtractor) callGemini(ctx context.Context, imageBytes []byte, contentType, prompt string) (string, error) {
	if e.geminiAPIKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is not configured")
	}
	if contentType == "" {
		contentType = http.DetectContentType(imageBytes)
	}

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

	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=" + e.geminiAPIKey
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Gemini OCR returned status %d", resp.StatusCode)
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
		return "", err
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no text returned by OCR")
	}

	return strings.Trim(strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text), "`\"' \n\r\t"), nil
}
