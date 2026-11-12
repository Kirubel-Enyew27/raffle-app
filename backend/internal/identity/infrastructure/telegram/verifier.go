// Package telegram adapts the platform initData verifier to the identity
// application's Telegram verifier port.
package telegram

import (
	"time"

	"github.com/raffle-app/backend/internal/identity/domain"
	tgplatform "github.com/raffle-app/backend/internal/telegram"
)

// Verifier verifies a raw Telegram.WebApp.initData payload against the bot
// token. Every failure is collapsed into a single error by the application
// layer, so a forged and an expired payload look identical to a client.
type Verifier struct {
	BotToken string
	MaxAge   time.Duration
}

func (v *Verifier) Verify(initData string) (*domain.TelegramProfile, error) {
	verifier := &tgplatform.Verifier{
		BotToken: v.BotToken,
		MaxAge:   v.MaxAge,
	}
	data, err := verifier.Verify(initData)
	if err != nil {
		return nil, err
	}
	return &domain.TelegramProfile{
		ID:           data.User.ID,
		FirstName:    data.User.FirstName,
		LastName:     data.User.LastName,
		Username:     data.User.Username,
		LanguageCode: data.User.LanguageCode,
		PhotoURL:     data.User.PhotoURL,
	}, nil
}
