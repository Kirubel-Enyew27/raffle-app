// Package bot implements KWIK's Telegram bot: its launch buttons, commands and
// the long-polling worker that serves them.
//
// The worker deliberately uses getUpdates instead of a webhook. The existing
// backend is a long-running container with no authenticated public webhook
// route, so polling avoids adding a new externally reachable endpoint. The bot
// token only ever lives in the process; nothing here is exposed to the browser.
package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	defaultAPIBase = "https://api.telegram.org"
	pollTimeout    = 25 * time.Second
	// requestTimeout is longer than the long-poll timeout so a healthy poll is
	// not cut off, while a dead connection still fails.
	requestTimeout = pollTimeout + 10*time.Second
	menuButtonText = "PLAY KWIK"
	welcomeText    = "KWIK – PLAY. COMPETE. EARN.\n\n" +
		"Welcome to KWIK! 🎮\n\n" +
		"Challenge friends, compete against players, and prove your skills.\n\n" +
		"Play Ethiopian Dama, Tic-Tac-Toe, Connect Four, Rock-Paper-Scissors, and more.\n\n" +
		"🏆 Compete. Win. Climb the leaderboard.\n\n" +
		"Ready to play? Open KWIK and join the competition!"
	helpText = "KWIK puts fast multiplayer games in your pocket.\n\n" +
		"• /games – open the games lobby\n" +
		"• /profile – view your profile and stats\n" +
		"• /wallet – check your balance and transactions\n\n" +
		"Tap the PLAY KWIK button below to launch the app. Signed-in matches, " +
		"wallets and history are the same as on the web."
	adminOnlyText = "Only group administrators can use this command."
)

// Config configures the bot. APIBaseURL and HTTPClient are injectable so the
// Bot API contract can be tested without network access.
type Config struct {
	Token      string
	MiniAppURL string
	APIBaseURL string
	HTTPClient *http.Client
	// Logger is optional; without it failures go to the standard logger.
	Logger *zap.Logger
}

// Bot is a Telegram Bot API client plus command handler.
type Bot struct {
	token      string
	miniAppURL string
	apiBase    string
	client     *http.Client
	logger     *zap.Logger
}

// New builds a bot. A token is required; callers only start the bot when one is
// configured.
func New(cfg Config) *Bot {
	base := strings.TrimRight(cfg.APIBaseURL, "/")
	if base == "" {
		base = defaultAPIBase
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Bot{
		token:      cfg.Token,
		miniAppURL: strings.TrimRight(cfg.MiniAppURL, "/"),
		apiBase:    base,
		client:     client,
		logger:     logger,
	}
}

// APIError describes a failed Bot API call without ever including the request
// URL, which contains the bot token.
type APIError struct {
	Method      string
	Status      int
	Description string
}

func (e *APIError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("telegram %s failed (%d): %s", e.Method, e.Status, e.Description)
	}
	return fmt.Sprintf("telegram %s failed (%d)", e.Method, e.Status)
}

type apiEnvelope struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
}

// call performs one Bot API method. Transport errors are rewritten without the
// URL so the token cannot leak into logs through a wrapped *url.Error.
func (b *Bot) call(ctx context.Context, method string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode telegram %s request: %w", method, err)
	}
	url := b.apiBase + "/bot" + b.token + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return &APIError{Method: method, Status: 0, Description: "could not build request"}
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := b.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &APIError{Method: method, Status: 0, Description: "network error"}
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return &APIError{Method: method, Status: res.StatusCode, Description: "could not read response"}
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return &APIError{Method: method, Status: res.StatusCode, Description: "invalid response"}
	}
	if res.StatusCode != http.StatusOK || !envelope.OK {
		return &APIError{
			Method:      method,
			Status:      res.StatusCode,
			Description: envelope.Description,
		}
	}
	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return &APIError{Method: method, Status: res.StatusCode, Description: "invalid result"}
		}
	}
	return nil
}

// Configure sets the bot's menu button and command list. It is safe to call on
// every start: both calls are idempotent. A failure is reported to the caller,
// which logs it, but polling still starts so commands keep working.
func (b *Bot) Configure(ctx context.Context) error {
	if b.token == "" {
		return &APIError{Method: "configure", Description: "bot token is not configured"}
	}
	if b.miniAppURL == "" || !strings.HasPrefix(b.miniAppURL, "https://") {
		return &APIError{Method: "configure", Description: "KWIK_MINI_APP_URL must be an https URL"}
	}

	err := b.call(ctx, "setChatMenuButton", map[string]any{
		"menu_button": map[string]any{
			"type":    "web_app",
			"text":    menuButtonText,
			"web_app": map[string]string{"url": b.miniAppURL},
		},
	}, nil)
	if err != nil {
		return err
	}

	return b.call(ctx, "setMyCommands", map[string]any{
		"commands": []map[string]string{
			{"command": "start", "description": "Open KWIK"},
			{"command": "games", "description": "Browse the games lobby"},
			{"command": "profile", "description": "Your KWIK profile"},
			{"command": "wallet", "description": "Balance and transactions"},
			{"command": "help", "description": "How KWIK works"},
		},
	}, nil)
}

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

// Chat carries the fields needed to identify a group and its type.
type Chat struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

type user struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type message struct {
	MessageID int64  `json:"message_id"`
	Text      string `json:"text"`
	Chat      Chat   `json:"chat"`
	From      *user  `json:"from"`
}

type webAppInfo struct {
	URL string `json:"url"`
}

type inlineKeyboardButton struct {
	Text   string      `json:"text"`
	WebApp *webAppInfo `json:"web_app,omitempty"`
}

type inlineKeyboardMarkup struct {
	InlineKeyboard [][]inlineKeyboardButton `json:"inline_keyboard"`
}

// Run mentions the bot's lifetime. It configures the launch experience, then
// long-polls for updates until the context is cancelled. A 409 from getUpdates
// means a webhook is configured for this bot: polling cannot work in that
// state, so it stops instead of hammering the API.
func (b *Bot) Run(ctx context.Context) {
	if err := b.Configure(ctx); err != nil {
		// The error text never contains the token: APIError does not carry the
		// request URL.
		b.logger.Warn("telegram bot configuration failed", zap.String("error", err.Error()))
	}

	var offset int64
	backoff := 3 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}

		var updates []update
		err := b.call(ctx, "getUpdates", map[string]any{
			"offset":          offset,
			"timeout":         int(pollTimeout.Seconds()),
			"allowed_updates": []string{"message"},
		}, &updates)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if apiErr, ok := err.(*APIError); ok && apiErr.Status == http.StatusConflict {
				b.logger.Warn("telegram bot polling stopped: a webhook is configured for this bot; remove it with deleteWebhook to use polling")
				return
			}
			b.logger.Warn("telegram bot polling failed", zap.String("error", err.Error()))
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = 3 * time.Second

		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			b.HandleUpdate(ctx, u)
		}
	}
}

// HandleUpdate answers one update. It is exported so tests can drive commands
// without a polling loop.
func (b *Bot) HandleUpdate(ctx context.Context, u update) {
	if u.Message == nil || u.Message.Chat.ID == 0 {
		return
	}
	fields := strings.Fields(u.Message.Text)
	if len(fields) == 0 {
		return
	}
	command := strings.ToLower(fields[0])
	if at := strings.IndexByte(command, '@'); at > 0 {
		command = command[:at]
	}

	if command == "/groupid" {
		b.handleGroupID(ctx, u.Message)
		return
	}

	var body, path, button string
	switch command {
	case "/start":
		body, path, button = welcomeText, "", "PLAY KWIK"
	case "/help":
		body, path, button = helpText, "", "OPEN KWIK"
	case "/games":
		body, path, button = "Pick a game and challenge an opponent. Ethiopian Dama, Tic-Tac-Toe, Connect Four and Rock-Paper-Scissors are all waiting.", "/games", "PLAY NOW"
	case "/profile":
		body, path, button = "Your profile, match history and stats are in KWIK.", "/profile", "OPEN PROFILE"
	case "/wallet":
		body, path, button = "Check your balance, deposit, withdraw and review transactions in KWIK.", "/wallet", "OPEN WALLET"
	default:
		body, path, button = helpText, "", "OPEN KWIK"
	}

	_ = b.sendMessage(ctx, u.Message.Chat.ID, body, path, button)
}

// handleGroupID reports a group's identity to its administrators. It only
// runs in group chats and confirms the sender's role with getChatMember, which
// is a Bot API method unavailable to ordinary members.
func (b *Bot) handleGroupID(ctx context.Context, msg *message) {
	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		return
	}
	if msg.From == nil {
		_ = b.sendMessage(ctx, msg.Chat.ID, adminOnlyText, "", "")
		return
	}

	var member struct {
		Status string `json:"status"`
	}
	err := b.call(ctx, "getChatMember", map[string]any{
		"chat_id": msg.Chat.ID,
		"user_id": msg.From.ID,
	}, &member)
	if err != nil {
		// APIError never carries the request URL, so the bot token cannot leak.
		b.logger.Warn("telegram getChatMember failed", zap.String("error", err.Error()))
		_ = b.sendMessage(ctx, msg.Chat.ID, "Could not verify administrator status. Please try again.", "", "")
		return
	}
	if member.Status != "administrator" && member.Status != "creator" {
		_ = b.sendMessage(ctx, msg.Chat.ID, adminOnlyText, "", "")
		return
	}

	body := fmt.Sprintf("Group name: %s\nChat ID: %d\nChat type: %s", msg.Chat.Title, msg.Chat.ID, msg.Chat.Type)
	_ = b.sendMessage(ctx, msg.Chat.ID, body, "", "")
}

func (b *Bot) sendMessage(ctx context.Context, chatID int64, text, path, button string) error {
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if button != "" && b.miniAppURL != "" {
		payload["reply_markup"] = inlineKeyboardMarkup{
			InlineKeyboard: [][]inlineKeyboardButton{{
				{Text: button, WebApp: &webAppInfo{URL: b.appURL(path)}},
			}},
		}
	}
	return b.call(ctx, "sendMessage", payload, nil)
}

// appURL joins the Mini App origin and an in-app path.
func (b *Bot) appURL(path string) string {
	if path == "" {
		return b.miniAppURL
	}
	return b.miniAppURL + path
}
