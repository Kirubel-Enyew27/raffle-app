package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const testToken = "123:secret-bot-token"
const testMiniApp = "https://raffle-app-eosin-phi.vercel.app"

type recordedCall struct {
	Method string
	Body   map[string]any
}

func newFakeBotAPI(t *testing.T) (*httptest.Server, func() []recordedCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []recordedCall

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := strings.TrimPrefix(r.URL.Path, "/bot"+testToken+"/")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		calls = append(calls, recordedCall{Method: method, Body: body})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))

	return server, func() []recordedCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedCall(nil), calls...)
	}
}

func newTestBot(apiBase string) *Bot {
	return New(Config{Token: testToken, MiniAppURL: testMiniApp, APIBaseURL: apiBase})
}

func TestConfigureSetsMenuButtonAndCommands(t *testing.T) {
	server, calls := newFakeBotAPI(t)
	defer server.Close()

	bot := newTestBot(server.URL)
	if err := bot.Configure(context.Background()); err != nil {
		t.Fatalf("configure: %v", err)
	}

	recorded := calls()
	if len(recorded) != 2 {
		t.Fatalf("expected 2 API calls, got %d", len(recorded))
	}
	if recorded[0].Method != "setChatMenuButton" {
		t.Fatalf("expected setChatMenuButton first, got %s", recorded[0].Method)
	}
	menuButton, _ := recorded[0].Body["menu_button"].(map[string]any)
	if menuButton["type"] != "web_app" || menuButton["text"] != "PLAY KWIK" {
		t.Fatalf("unexpected menu button: %+v", menuButton)
	}
	webApp, _ := menuButton["web_app"].(map[string]any)
	if webApp["url"] != testMiniApp {
		t.Fatalf("menu button should open the Mini App, got %+v", webApp)
	}

	if recorded[1].Method != "setMyCommands" {
		t.Fatalf("expected setMyCommands second, got %s", recorded[1].Method)
	}
	commands, _ := recorded[1].Body["commands"].([]any)
	if len(commands) != 5 {
		t.Fatalf("expected 5 commands, got %d", len(commands))
	}
}

func TestConfigureRejectsNonHTTPSMiniAppURL(t *testing.T) {
	bot := New(Config{Token: testToken, MiniAppURL: "http://example.com"})
	if err := bot.Configure(context.Background()); err == nil {
		t.Fatal("expected an insecure Mini App URL to be rejected")
	}
}

func TestStartCommandSendsBrandedWelcomeWithMiniAppButton(t *testing.T) {
	server, calls := newFakeBotAPI(t)
	defer server.Close()

	bot := newTestBot(server.URL)
	bot.HandleUpdate(context.Background(), update{
		UpdateID: 1,
		Message: &message{Text: "/start", Chat: struct {
			ID int64 `json:"id"`
		}{ID: 42}},
	})

	recorded := calls()
	if len(recorded) != 1 || recorded[0].Method != "sendMessage" {
		t.Fatalf("expected one sendMessage, got %+v", recorded)
	}
	text, _ := recorded[0].Body["text"].(string)
	if !strings.Contains(text, "KWIK – PLAY. COMPETE. EARN.") || !strings.Contains(text, "Welcome to KWIK!") {
		t.Fatalf("welcome copy missing: %q", text)
	}
	if !strings.Contains(text, "Ethiopian Dama") || !strings.Contains(text, "Climb the leaderboard") {
		t.Fatalf("welcome copy incomplete: %q", text)
	}
	if recorded[0].Body["chat_id"] != float64(42) {
		t.Fatalf("wrong chat id: %+v", recorded[0].Body["chat_id"])
	}

	markup, _ := recorded[0].Body["reply_markup"].(map[string]any)
	rows, _ := markup["inline_keyboard"].([]any)
	if len(rows) != 1 {
		t.Fatalf("expected an inline keyboard, got %+v", markup)
	}
	row, _ := rows[0].([]any)
	button, _ := row[0].(map[string]any)
	if button["text"] != "PLAY KWIK" {
		t.Fatalf("unexpected button text: %+v", button)
	}
	webApp, _ := button["web_app"].(map[string]any)
	if webApp["url"] != testMiniApp {
		t.Fatalf("start button should open the Mini App, got %+v", webApp)
	}
}

func TestCommandsDeepLinkIntoTheApp(t *testing.T) {
	server, calls := newFakeBotAPI(t)
	defer server.Close()
	bot := newTestBot(server.URL)

	cases := []struct {
		command string
		path    string
	}{
		{"/games", "/games"},
		{"/profile", "/profile"},
		{"/wallet", "/wallet"},
	}
	for _, tc := range cases {
		callsSnapshot := len(calls())
		bot.HandleUpdate(context.Background(), update{
			Message: &message{Text: tc.command, Chat: struct {
				ID int64 `json:"id"`
			}{ID: 7}},
		})
		recorded := calls()
		if len(recorded) != callsSnapshot+1 {
			t.Fatalf("%s: expected one sendMessage", tc.command)
		}
		markup, _ := recorded[len(recorded)-1].Body["reply_markup"].(map[string]any)
		rows, _ := markup["inline_keyboard"].([]any)
		row, _ := rows[0].([]any)
		button, _ := row[0].(map[string]any)
		webApp, _ := button["web_app"].(map[string]any)
		if webApp["url"] != testMiniApp+tc.path {
			t.Fatalf("%s: expected %s, got %+v", tc.command, testMiniApp+tc.path, webApp)
		}
	}
}

func TestCommandWithBotMentionAndUnknownText(t *testing.T) {
	server, calls := newFakeBotAPI(t)
	defer server.Close()
	bot := newTestBot(server.URL)

	bot.HandleUpdate(context.Background(), update{
		Message: &message{Text: "/help@kwik_bot", Chat: struct {
			ID int64 `json:"id"`
		}{ID: 9}},
	})
	bot.HandleUpdate(context.Background(), update{
		Message: &message{Text: "hello", Chat: struct {
			ID int64 `json:"id"`
		}{ID: 9}},
	})
	bot.HandleUpdate(context.Background(), update{
		Message: &message{Text: "   ", Chat: struct {
			ID int64 `json:"id"`
		}{ID: 9}},
	})

	recorded := calls()
	if len(recorded) != 2 {
		t.Fatalf("expected two replies (help + fallback), got %d", len(recorded))
	}
	for _, call := range recorded {
		text, _ := call.Body["text"].(string)
		if !strings.Contains(text, "/wallet") {
			t.Fatalf("expected help text, got %q", text)
		}
	}
}

func TestAPIErrorNeverIncludesTheToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":401,"description":"Unauthorized"}`))
	}))
	defer server.Close()

	bot := newTestBot(server.URL)
	err := bot.Configure(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), testToken) || strings.Contains(err.Error(), "secret-bot-token") {
		t.Fatalf("error leaked the bot token: %v", err)
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("expected Telegram's description, got %v", err)
	}
}
