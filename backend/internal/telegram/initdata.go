// Package telegram implements the parts of the Telegram Bot/Mini App platform
// the KWIK backend needs: verification of WebApp initData and the Bot API
// client used by the launch experience.
package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// maxInitDataBytes bounds the payload before it is parsed. Telegram's
	// initData is normally well under 2 KB; anything huge is not a real launch.
	maxInitDataBytes = 16 << 10
	// clockSkew allows for a Mini App launched on a device whose clock is a
	// little ahead of ours.
	clockSkew = 5 * time.Minute
)

var (
	// ErrInitDataMissing means there is no token/initData to verify at all.
	ErrInitDataMissing = errors.New("telegram init data is missing")
	// ErrInitDataInvalid covers a malformed payload, a bad signature, or a
	// payload without a usable user.
	ErrInitDataInvalid = errors.New("telegram init data is invalid")
	// ErrInitDataExpired means auth_date is older than the allowed age.
	ErrInitDataExpired = errors.New("telegram init data has expired")
)

// User is the portion of the Telegram user object KWIK stores or displays.
type User struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
	PhotoURL     string `json:"photo_url"`
	IsPremium    bool   `json:"is_premium"`
}

// DisplayName returns the friendliest available name.
func (u User) DisplayName() string {
	name := strings.TrimSpace(strings.TrimSpace(u.FirstName) + " " + strings.TrimSpace(u.LastName))
	if name != "" {
		return name
	}
	if u.Username != "" {
		return u.Username
	}
	return "KWIK Player"
}

// InitData is a verified Mini App launch payload. Every field comes from data
// covered by the signature; nothing is read from initDataUnsafe.
type InitData struct {
	User       User
	AuthDate   time.Time
	QueryID    string
	StartParam string
}

// Verifier validates raw Telegram.WebApp.initData strings.
type Verifier struct {
	BotToken string
	MaxAge   time.Duration
	// Now is the clock used for freshness checks; tests override it.
	Now func() time.Time
}

// Verify checks the HMAC-SHA256 signature exactly as Telegram specifies:
//
//	secret = HMAC_SHA256("WebAppData", bot_token)
//	hash   = HMAC_SHA256(secret, sorted "key=value" lines)
//
// The comparison is constant time. auth_date must be recent and not in the
// future, so a captured payload cannot be replayed indefinitely.
func (v *Verifier) Verify(raw string) (*InitData, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || v.BotToken == "" {
		return nil, ErrInitDataMissing
	}
	if len(raw) > maxInitDataBytes {
		return nil, ErrInitDataInvalid
	}

	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, ErrInitDataInvalid
	}

	hash := values.Get("hash")
	if len(hash) != 64 {
		return nil, ErrInitDataInvalid
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return nil, ErrInitDataInvalid
	}

	// The data-check string excludes `hash` and every other field is taken
	// URL-decoded, sorted by key, one "key=value" per line.
	values.Del("hash")
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
		check.WriteString(key)
		check.WriteByte('=')
		check.WriteString(values.Get(key))
	}

	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	secretMAC.Write([]byte(v.BotToken))
	secret := secretMAC.Sum(nil)

	payloadMAC := hmac.New(sha256.New, secret)
	payloadMAC.Write([]byte(check.String()))
	expected := hex.EncodeToString(payloadMAC.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(hash))) != 1 {
		return nil, ErrInitDataInvalid
	}

	authUnix, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil || authUnix <= 0 {
		return nil, ErrInitDataInvalid
	}
	now := time.Now()
	if v.Now != nil {
		now = v.Now()
	}
	authDate := time.Unix(authUnix, 0)
	if authDate.After(now.Add(clockSkew)) {
		return nil, ErrInitDataInvalid
	}
	if v.MaxAge > 0 && now.Sub(authDate) > v.MaxAge {
		return nil, ErrInitDataExpired
	}

	rawUser := values.Get("user")
	if rawUser == "" {
		return nil, ErrInitDataInvalid
	}
	var user User
	if err := json.Unmarshal([]byte(rawUser), &user); err != nil {
		return nil, ErrInitDataInvalid
	}
	if user.ID <= 0 {
		return nil, ErrInitDataInvalid
	}
	// Trim control characters before the name is ever rendered or stored.
	user.FirstName = cleanText(user.FirstName, 128)
	user.LastName = cleanText(user.LastName, 128)
	user.Username = cleanText(user.Username, 64)
	user.LanguageCode = cleanText(user.LanguageCode, 16)
	user.PhotoURL = cleanText(user.PhotoURL, 512)

	return &InitData{
		User:       user,
		AuthDate:   authDate,
		QueryID:    values.Get("query_id"),
		StartParam: values.Get("start_param"),
	}, nil
}

// cleanText strips control characters and bounds the length of untrusted text.
func cleanText(value string, max int) string {
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	if len(value) > max {
		value = value[:max]
	}
	return value
}
