package verifier

import (
	"bytes"
	"context"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// buildQRImage renders a QR code encoding contents as a PNG image.
func buildQRImage(t *testing.T, contents string) []byte {
	t.Helper()

	bitmap, err := qrcode.NewQRCodeWriter().Encode(contents, gozxing.BarcodeFormat_QR_CODE, 512, 512, nil)
	if err != nil {
		t.Fatalf("failed to encode QR image: %v", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, bitmap); err != nil {
		t.Fatalf("failed to encode QR image as PNG: %v", err)
	}
	return buf.Bytes()
}

// The Telebirr receipt QR code is intentionally not decoded: a Telebirr
// screenshot must always go through Gemini OCR, even when it carries a QR that
// points at the receipt page.
func TestTelebirrExtractionIgnoresReceiptQR(t *testing.T) {
	imageBytes := buildQRImage(t, "https://transactioninfo.ethiotelecom.et/receipt/DGP88F2T36")

	// Sanity check: the QR code really is decodable, so the previous QR based
	// flow would have returned DGP88F2T36 without calling OCR.
	decoded, err := decodeQRCode(imageBytes)
	if err != nil || !strings.Contains(decoded, "DGP88F2T36") {
		t.Fatalf("test setup: expected a decodable QR payload, got %q (err=%v)", decoded, err)
	}

	// With no API key configured the OCR path is unavailable, so extraction has
	// to fail instead of quietly falling back to the QR code.
	extractor := NewImageExtractor("")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ref, err := extractor.Extract(ctx, "telebirr", imageBytes, "image/png")
	if err == nil {
		t.Fatalf("expected Telebirr extraction to require OCR, but the QR code produced %q", ref)
	}

	if details, err := extractor.ExtractDetails(ctx, "telebirr", imageBytes, "image/png"); err == nil {
		t.Fatalf("expected Telebirr extraction details to require OCR, got %+v", details)
	}
}

// CBE receipts still rely on their QR code token, which is unaffected by the
// Telebirr change.
func TestCBEExtractionStillUsesQR(t *testing.T) {
	imageBytes := buildQRImage(t, "https://mbreciept.cbe.com.et/v2-hfHCxGL8LagYvERAaCnr")

	extractor := NewImageExtractor("")
	ref, err := extractor.Extract(context.Background(), "cbe", imageBytes, "image/png")
	if err != nil {
		t.Fatalf("expected CBE extraction from the QR code, got error: %v", err)
	}
	if ref != "v2-hfHCxGL8LagYvERAaCnr" {
		t.Errorf("expected CBE token v2-hfHCxGL8LagYvERAaCnr, got %q", ref)
	}
}
