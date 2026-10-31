package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv              string
	AppPort             int
	AppDebug            bool
	Database            DatabaseConfig
	Redis               RedisConfig
	JWT                 JWTConfig
	Crash               CrashConfig
	SMSAPIKey           string
	TeleVerifyURL       string
	TeleVerifyKey       string
	CBEReceiptAPI       string
	CBEAppID            string
	CBEAppVersion       string
	CBEAccountName      string
	CBEAccountNumber    string
	TelebirrAccountName string
	TelebirrPhone       string
	AutoCreditEnabled   bool
	LinksETAPIKey       string
	LinksETBaseURL      string
	GeminiAPIKey        string
}

type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	Host       string
	Port       string
	Password   string
	DB         int
	TLSEnabled bool
}

type JWTConfig struct {
	Secret string
	// AccessExpiry is deliberately short; clients renew it with the refresh
	// token instead of forcing a new login mid-session.
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

// IsDevelopmentEnvironment reports whether development-only shortcuts may be
// enabled. Unknown and production-like values fail closed.
func IsDevelopmentEnvironment() bool {
	appEnv, explicitlyConfigured := os.LookupEnv("APP_ENV")
	if !explicitlyConfigured && viper.InConfig("APP_ENV") {
		appEnv = viper.GetString("APP_ENV")
		explicitlyConfigured = true
	}
	if !explicitlyConfigured {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "development", "dev", "test":
		return true
	default:
		return false
	}
}

// CrashConfig holds the multiplayer crash game tunables. Every timing and limit
// is configurable so the engine never relies on hardcoded durations.
type CrashConfig struct {
	Enabled            bool
	Room               string
	WaitingSeconds     float64
	BettingSeconds     float64
	CrashedSeconds     float64
	NextRoundSeconds   float64
	GrowthRate         float64
	HouseEdgeBP        int
	MaxMultiplierBP    int
	MinBet             float64
	MaxBet             float64
	TickInterval       time.Duration
	BroadcastInterval  time.Duration
	RateLimitPerSecond int
	// P2P configuration
	ServiceFeeBP    int
	MinParticipants int
	MaxParticipants int
	PlatformUserID  string
}

// loadDotEnv fills values in from a local .env file so a plain `go run
// ./cmd/server` (or the compiled binary started from the repo root) sees the
// same secrets the compose stack injects. Real environment variables still win,
// because viper resolves env before config-file values. A missing file is
// normal — deployments get their configuration from the environment.
func loadDotEnv() error {
	for _, path := range []string{".env", filepath.Join("backend", ".env")} {
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("failed to check %s: %w", path, err)
		}

		viper.SetConfigFile(path)
		viper.SetConfigType("dotenv")
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}
		return nil
	}
	return nil
}

// positiveDuration reads a duration setting and falls back to def when the
// value is missing or non-positive. An explicit 0 would otherwise mint tokens
// that are already expired and log every user out immediately.
func positiveDuration(key string, def time.Duration) time.Duration {
	d := viper.GetDuration(key)
	if d <= 0 {
		return def
	}
	return d
}

func Load() (*Config, error) {
	if err := loadDotEnv(); err != nil {
		return nil, err
	}

	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("APP_PORT", 8080)
	viper.SetDefault("APP_DEBUG", true)
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("DB_USER", "raffle")
	viper.SetDefault("DB_PASSWORD", "raffle_dev")
	viper.SetDefault("DB_NAME", "raffle_db")
	viper.SetDefault("DB_SSL_MODE", "disable")
	viper.SetDefault("DB_MAX_OPEN_CONNS", 25)
	viper.SetDefault("DB_MAX_IDLE_CONNS", 5)
	viper.SetDefault("DB_CONN_MAX_LIFETIME", 5*time.Minute)
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("REDIS_PASSWORD", "")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("REDIS_TLS_ENABLED", false)
	viper.SetDefault("JWT_SECRET", "change-me-in-production")
	viper.SetDefault("JWT_ACCESS_EXPIRY", 15*time.Minute)
	viper.SetDefault("JWT_REFRESH_EXPIRY", 7*24*time.Hour)
	viper.SetDefault("SMS_API_KEY", "dev-sms-api-key-123")
	viper.SetDefault("TELE_VERIFY_URL", "https://verifyapi.leulzenebe.pro/verify-telebirr")
	// Credentials must come from the environment. Never commit live keys here.
	viper.SetDefault("TELE_VERIFY_KEY", "")
	viper.SetDefault("CBE_RECEIPT_API", "https://mb.cbe.com.et/api/v1/transactions/public/transaction-detail")
	viper.SetDefault("CBE_APP_ID", "d1292e42-7400-49de-a2d3-9731caa4c819")
	viper.SetDefault("CBE_APP_VERSION", "0a01980b-9859-1369-8198-59f403820000")
	viper.SetDefault("CBE_ACCOUNT_NAME", "Kirubel Enyew")
	viper.SetDefault("CBE_ACCOUNT_NUMBER", "1000249277383")
	viper.SetDefault("TELEBIRR_ACCOUNT_NAME", "Kirubel Enyew")
	viper.SetDefault("TELEBIRR_PHONE", "+251948260013")
	viper.SetDefault("AUTO_CREDIT_ENABLED", true)
	viper.SetDefault("LINKS_ET_API_KEY", "")
	viper.SetDefault("LINKS_ET_BASE_URL", "https://links.et")
	// Credentials must come from the environment. Never commit live keys here.
	viper.SetDefault("GEMINI_API_KEY", "")

	// Multiplayer crash game
	viper.SetDefault("CRASH_ENABLED", true)
	viper.SetDefault("CRASH_ROOM", "default")
	viper.SetDefault("CRASH_WAITING_SECONDS", 5)
	viper.SetDefault("CRASH_BETTING_SECONDS", 30)
	viper.SetDefault("CRASH_CRASHED_SECONDS", 5)
	viper.SetDefault("CRASH_NEXT_ROUND_SECONDS", 3)
	viper.SetDefault("CRASH_GROWTH_RATE", 0.07)
	viper.SetDefault("CRASH_HOUSE_EDGE_BP", 300)
	viper.SetDefault("CRASH_MAX_MULTIPLIER_BP", 100000)
	viper.SetDefault("CRASH_MIN_BET", 10.0)
	viper.SetDefault("CRASH_MAX_BET", 10000.0)
	viper.SetDefault("CRASH_TICK_INTERVAL", 50*time.Millisecond)
	viper.SetDefault("CRASH_BROADCAST_INTERVAL", 250*time.Millisecond)
	viper.SetDefault("CRASH_RATE_LIMIT_PER_SECOND", 20)
	// P2P settings
	viper.SetDefault("CRASH_SERVICE_FEE_BP", 500)
	viper.SetDefault("CRASH_MIN_PARTICIPANTS", 2)
	viper.SetDefault("CRASH_MAX_PARTICIPANTS", 100)
	viper.SetDefault("CRASH_PLATFORM_USER_ID", "")

	viper.AutomaticEnv()

	// Fail closed in production when the signing secret is still the shipped
	// development placeholder. Tokens signed with it are forgeable by anyone
	// who has read the repository.
	if viper.GetString("APP_ENV") == "production" {
		secret := viper.GetString("JWT_SECRET")
		if secret == "" || secret == "change-me-in-production" {
			return nil, fmt.Errorf("JWT_SECRET must be set to a strong, unique value when APP_ENV=production")
		}
	}

	return &Config{
		AppEnv:   viper.GetString("APP_ENV"),
		AppPort:  viper.GetInt("APP_PORT"),
		AppDebug: viper.GetBool("APP_DEBUG"),
		Database: DatabaseConfig{
			Host:            viper.GetString("DB_HOST"),
			Port:            viper.GetString("DB_PORT"),
			User:            viper.GetString("DB_USER"),
			Password:        viper.GetString("DB_PASSWORD"),
			DBName:          viper.GetString("DB_NAME"),
			SSLMode:         viper.GetString("DB_SSL_MODE"),
			MaxOpenConns:    positiveInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    positiveInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: positiveDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Host:       viper.GetString("REDIS_HOST"),
			Port:       viper.GetString("REDIS_PORT"),
			Password:   viper.GetString("REDIS_PASSWORD"),
			DB:         viper.GetInt("REDIS_DB"),
			TLSEnabled: viper.GetBool("REDIS_TLS_ENABLED"),
		},
		JWT: JWTConfig{
			Secret:        viper.GetString("JWT_SECRET"),
			AccessExpiry:  positiveDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
			RefreshExpiry: positiveDuration("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		},
		SMSAPIKey:           viper.GetString("SMS_API_KEY"),
		TeleVerifyURL:       viper.GetString("TELE_VERIFY_URL"),
		TeleVerifyKey:       viper.GetString("TELE_VERIFY_KEY"),
		CBEReceiptAPI:       viper.GetString("CBE_RECEIPT_API"),
		CBEAppID:            viper.GetString("CBE_APP_ID"),
		CBEAppVersion:       viper.GetString("CBE_APP_VERSION"),
		CBEAccountName:      viper.GetString("CBE_ACCOUNT_NAME"),
		CBEAccountNumber:    viper.GetString("CBE_ACCOUNT_NUMBER"),
		TelebirrAccountName: viper.GetString("TELEBIRR_ACCOUNT_NAME"),
		TelebirrPhone:       viper.GetString("TELEBIRR_PHONE"),
		AutoCreditEnabled:   viper.GetBool("AUTO_CREDIT_ENABLED"),
		LinksETAPIKey:       viper.GetString("LINKS_ET_API_KEY"),
		LinksETBaseURL:      viper.GetString("LINKS_ET_BASE_URL"),
		GeminiAPIKey:        viper.GetString("GEMINI_API_KEY"),
		Crash: CrashConfig{
			Enabled:            viper.GetBool("CRASH_ENABLED"),
			Room:               viper.GetString("CRASH_ROOM"),
			WaitingSeconds:     viper.GetFloat64("CRASH_WAITING_SECONDS"),
			BettingSeconds:     viper.GetFloat64("CRASH_BETTING_SECONDS"),
			CrashedSeconds:     viper.GetFloat64("CRASH_CRASHED_SECONDS"),
			NextRoundSeconds:   viper.GetFloat64("CRASH_NEXT_ROUND_SECONDS"),
			GrowthRate:         viper.GetFloat64("CRASH_GROWTH_RATE"),
			HouseEdgeBP:        viper.GetInt("CRASH_HOUSE_EDGE_BP"),
			MaxMultiplierBP:    viper.GetInt("CRASH_MAX_MULTIPLIER_BP"),
			MinBet:             viper.GetFloat64("CRASH_MIN_BET"),
			MaxBet:             viper.GetFloat64("CRASH_MAX_BET"),
			TickInterval:       viper.GetDuration("CRASH_TICK_INTERVAL"),
			BroadcastInterval:  viper.GetDuration("CRASH_BROADCAST_INTERVAL"),
			RateLimitPerSecond: viper.GetInt("CRASH_RATE_LIMIT_PER_SECOND"),
			ServiceFeeBP:       viper.GetInt("CRASH_SERVICE_FEE_BP"),
			MinParticipants:    viper.GetInt("CRASH_MIN_PARTICIPANTS"),
			MaxParticipants:    viper.GetInt("CRASH_MAX_PARTICIPANTS"),
			PlatformUserID:     viper.GetString("CRASH_PLATFORM_USER_ID"),
		},
	}, nil
}

func positiveInt(key string, def int) int {
	n := viper.GetInt(key)
	if n <= 0 {
		return def
	}
	return n
}
