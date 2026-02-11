package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// Telegram
	TelegramToken   string
	BotUsername     string
	AdminTelegramID int64

	// Database
	DatabaseURL string

	// Tinkoff
	TinkoffTerminalKey string
	TinkoffPassword    string
	TinkoffTestMode    bool

	// App
	SubscriptionPrice int64
	Environment       string
	BaseURL           string
	Port              string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		TelegramToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		BotUsername:        os.Getenv("BOT_USERNAME"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		TinkoffTerminalKey: os.Getenv("TINKOFF_TERMINAL_KEY"),
		TinkoffPassword:    os.Getenv("TINKOFF_PASSWORD"),
		TinkoffTestMode:    os.Getenv("TINKOFF_TEST_MODE") == "true",
		Environment:        getEnv("ENVIRONMENT", "development"),
		BaseURL:            os.Getenv("BASE_URL"),
		Port:               getEnv("PORT", "8080"),
	}

	if adminID := os.Getenv("ADMIN_TELEGRAM_ID"); adminID != "" {
		cfg.AdminTelegramID, _ = strconv.ParseInt(adminID, 10, 64)
	}

	price, err := strconv.ParseInt(getEnv("SUBSCRIPTION_PRICE", "19900"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid subscription price: %w", err)
	}
	cfg.SubscriptionPrice = price

	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
