package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testBotToken = "123456:TEST-bot-token"

// signInitData builds a payload exactly the way Telegram's clients do, so the
// verifier is tested against the specification rather than against itself.
func signInitData(t *testing.T, token string, fields map[string]string) string {
	t.Helper()
	values := url.Values{}
	for key, value := range fields {
		values.Set(key, value)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var check strings.Builder
	for i, key := range keys {
		if i > 0 {
			check.WriteByte('\n')
		}
		check.WriteString(key + "=" + values.Get(key))
	}

	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	secretMAC.Write([]byte(token))
	payloadMAC := hmac.New(sha256.New, secretMAC.Sum(nil))
	payloadMAC.Write([]byte(check.String()))
	values.Set("hash", hex.EncodeToString(payloadMAC.Sum(nil)))
	return values.Encode()
}

func validFields(now time.Time) map[string]string {
	return map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
		"query_id":  "AAH-test-query",
		"user":      `{"id":900001,"first_name":"Ada","last_name":"Lovelace","username":"ada","language_code":"en"}`,
	}
}

func TestVerifyInitDataAcceptsValidPayload(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	raw := signInitData(t, testBotToken, validFields(now))
	verifier := &Verifier{BotToken: testBotToken, MaxAge: 24 * time.Hour, Now: func() time.Time { return now }}

	data, err := verifier.Verify(raw)
	if err != nil {
		t.Fatalf("valid initData rejected: %v", err)
	}
	if data.User.ID != 900001 || data.User.Username != "ada" {
		t.Fatalf("unexpected user: %+v", data.User)
	}
	if data.User.DisplayName() != "Ada Lovelace" {
		t.Errorf("unexpected display name %q", data.User.DisplayName())
	}
	if data.QueryID != "AAH-test-query" {
		t.Errorf("unexpected query id %q", data.QueryID)
	}
	if !data.AuthDate.Equal(now) {
		t.Errorf("unexpected auth date %v", data.AuthDate)
	}
}

func TestVerifyInitDataRejectsTamperedFields(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	raw := signInitData(t, testBotToken, validFields(now))

	tampered, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatal(err)
	}
	tampered.Set("user", `{"id":1,"first_name":"Mallory"}`)
	verifier := &Verifier{BotToken: testBotToken, MaxAge: 24 * time.Hour, Now: func() time.Time { return now }}

	if _, err := verifier.Verify(tampered.Encode()); !errors.Is(err, ErrInitDataInvalid) {
		t.Fatalf("tampered payload must be invalid, got %v", err)
	}
}

func TestVerifyInitDataRejectsWrongBotToken(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	raw := signInitData(t, "999:other-bot", validFields(now))
	verifier := &Verifier{BotToken: testBotToken, MaxAge: 24 * time.Hour, Now: func() time.Time { return now }}

	if _, err := verifier.Verify(raw); !errors.Is(err, ErrInitDataInvalid) {
		t.Fatalf("payload for another bot must be invalid, got %v", err)
	}
}

func TestVerifyInitDataRejectsExpiredAndFuturePayloads(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	verifier := &Verifier{BotToken: testBotToken, MaxAge: 24 * time.Hour, Now: func() time.Time { return now }}

	expired := validFields(now.Add(-25 * time.Hour))
	if _, err := verifier.Verify(signInitData(t, testBotToken, expired)); !errors.Is(err, ErrInitDataExpired) {
		t.Fatalf("expired payload must be rejected, got %v", err)
	}

	// Older than the window, but the signature is still valid: this is what a
	// captured launch payload looks like when replayed later.
	justExpired := validFields(now.Add(-24*time.Hour - time.Second))
	if _, err := verifier.Verify(signInitData(t, testBotToken, justExpired)); !errors.Is(err, ErrInitDataExpired) {
		t.Fatalf("payload older than maxAge must be rejected, got %v", err)
	}

	future := validFields(now.Add(10 * time.Minute))
	if _, err := verifier.Verify(signInitData(t, testBotToken, future)); !errors.Is(err, ErrInitDataInvalid) {
		t.Fatalf("payload from the future must be rejected, got %v", err)
	}

	// Small clock skew is tolerated so a phone a few seconds ahead still works.
	skewed := validFields(now.Add(2 * time.Minute))
	if _, err := verifier.Verify(signInitData(t, testBotToken, skewed)); err != nil {
		t.Fatalf("small clock skew should be accepted: %v", err)
	}
}

func TestVerifyInitDataRejectsMalformedPayloads(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	verifier := &Verifier{BotToken: testBotToken, MaxAge: 24 * time.Hour, Now: func() time.Time { return now }}

	cases := []struct {
		name string
		raw  string
		want error
	}{
		{name: "empty", raw: "", want: ErrInitDataMissing},
		{name: "no hash", raw: "auth_date=" + strconv.FormatInt(now.Unix(), 10), want: ErrInitDataInvalid},
		{name: "short hash", raw: "auth_date=1&hash=abc", want: ErrInitDataInvalid},
		{name: "not hex", raw: "auth_date=1&hash=" + strings.Repeat("z", 64), want: ErrInitDataInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := verifier.Verify(tc.raw); !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}

	noUser := validFields(now)
	delete(noUser, "user")
	if _, err := verifier.Verify(signInitData(t, testBotToken, noUser)); !errors.Is(err, ErrInitDataInvalid) {
		t.Fatalf("payload without a user must be rejected, got %v", err)
	}

	zeroID := validFields(now)
	zeroID["user"] = `{"id":0,"first_name":"Nobody"}`
	if _, err := verifier.Verify(signInitData(t, testBotToken, zeroID)); !errors.Is(err, ErrInitDataInvalid) {
		t.Fatalf("payload without a user id must be rejected, got %v", err)
	}
}

func TestVerifyInitDataSanitizesUserText(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	fields := validFields(now)
	fields["user"] = `{"id":7,"first_name":"  Ada\u0000\n","last_name":"","username":""}`
	verifier := &Verifier{BotToken: testBotToken, MaxAge: 24 * time.Hour, Now: func() time.Time { return now }}

	data, err := verifier.Verify(signInitData(t, testBotToken, fields))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if data.User.FirstName != "Ada" {
		t.Errorf("control characters were not stripped: %q", data.User.FirstName)
	}
	if data.User.DisplayName() != "Ada" {
		t.Errorf("unexpected display name %q", data.User.DisplayName())
	}
}

func TestVerifyInitDataBoundsPayloadSize(t *testing.T) {
	verifier := &Verifier{BotToken: testBotToken, MaxAge: time.Hour}
	raw := "user=" + strings.Repeat("a", maxInitDataBytes) + "&hash=" + strings.Repeat("0", 64)
	if _, err := verifier.Verify(raw); !errors.Is(err, ErrInitDataInvalid) {
		t.Fatalf("oversized payload must be rejected, got %v", err)
	}
}

func TestVerifyInitDataRequiresBotToken(t *testing.T) {
	verifier := &Verifier{MaxAge: time.Hour}
	if _, err := verifier.Verify(fmt.Sprintf("user={}&hash=%s", strings.Repeat("0", 64))); !errors.Is(err, ErrInitDataMissing) {
		t.Fatalf("missing bot token must not verify anything, got %v", err)
	}
}
