package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv   string
	AppPort  int
	AppDebug bool
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	SMSAPIKey string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret       string
	AccessExpiry time.Duration
}

func Load() (*Config, error) {
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("APP_PORT", 8080)
	viper.SetDefault("APP_DEBUG", true)
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("DB_USER", "raffle")
	viper.SetDefault("DB_PASSWORD", "raffle_dev")
	viper.SetDefault("DB_NAME", "raffle_db")
	viper.SetDefault("DB_SSL_MODE", "disable")
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("REDIS_PASSWORD", "")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("JWT_SECRET", "change-me-in-production")
	viper.SetDefault("JWT_ACCESS_EXPIRY", 15*time.Minute)
	viper.SetDefault("JWT_REFRESH_EXPIRY", 7*24*time.Hour)
	viper.SetDefault("SMS_API_KEY", "dev-sms-api-key-123")

	viper.AutomaticEnv()

	return &Config{
		AppEnv:   viper.GetString("APP_ENV"),
		AppPort:  viper.GetInt("APP_PORT"),
		AppDebug: viper.GetBool("APP_DEBUG"),
		Database: DatabaseConfig{
			Host:     viper.GetString("DB_HOST"),
			Port:     viper.GetString("DB_PORT"),
			User:     viper.GetString("DB_USER"),
			Password: viper.GetString("DB_PASSWORD"),
			DBName:   viper.GetString("DB_NAME"),
			SSLMode:  viper.GetString("DB_SSL_MODE"),
		},
		Redis: RedisConfig{
			Host:     viper.GetString("REDIS_HOST"),
			Port:     viper.GetString("REDIS_PORT"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
		},
		JWT: JWTConfig{
			Secret:       viper.GetString("JWT_SECRET"),
			AccessExpiry: viper.GetDuration("JWT_ACCESS_EXPIRY"),
		},
		SMSAPIKey: viper.GetString("SMS_API_KEY"),
	}, nil
}
