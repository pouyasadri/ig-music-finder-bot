package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	BotToken         string
	ACRHost          string
	ACRKey           string
	ACRSecret        string
	CookiesPath      string
	ShazamScriptPath string
	AuddToken        string
	ChannelID        string
	WorkerCount      int
	QueueLimit       int
	RateLimit        int
	RateWindow       time.Duration
	RequestTimeout   time.Duration
	DatabasePath     string
	LeaseTTL         time.Duration
	CacheTTL         time.Duration
	MaxUploadBytes   int64
}

func Load() (Config, error) {
	cfg := Config{
		BotToken:         os.Getenv("TELEGRAM_BOT_TOKEN"),
		ACRHost:          os.Getenv("ACR_HOST"),
		ACRKey:           os.Getenv("ACR_KEY"),
		ACRSecret:        os.Getenv("ACR_SECRET"),
		CookiesPath:      envOr("COOKIES_PATH", "/app/cookies.txt"),
		ShazamScriptPath: envOr("SHAZAM_SCRIPT_PATH", "/app/scripts/shazam_recognize.py"),
		AuddToken:        os.Getenv("AUDD_API_TOKEN"),
		ChannelID:        os.Getenv("CHANNEL_ID"),
		WorkerCount:      5,
		QueueLimit:       20,
		RateLimit:        10,
		RateWindow:       time.Hour,
		RequestTimeout:   3 * time.Minute,
		DatabasePath:     envOr("DATABASE_PATH", "/app/data/bot.db"),
		LeaseTTL:         5 * time.Minute,
		CacheTTL:         24 * time.Hour,
		MaxUploadBytes:   49 * 1024 * 1024,
	}
	var err error
	if cfg.WorkerCount, err = positiveInt("WORKER_COUNT", cfg.WorkerCount); err != nil {
		return Config{}, err
	}
	if cfg.QueueLimit, err = positiveInt("QUEUE_LIMIT", cfg.QueueLimit); err != nil {
		return Config{}, err
	}
	if cfg.RateLimit, err = positiveInt("RATE_LIMIT", cfg.RateLimit); err != nil {
		return Config{}, err
	}
	if cfg.RateWindow, err = duration("RATE_WINDOW", cfg.RateWindow); err != nil {
		return Config{}, err
	}
	if cfg.RequestTimeout, err = duration("REQUEST_TIMEOUT", cfg.RequestTimeout); err != nil {
		return Config{}, err
	}
	if cfg.LeaseTTL, err = duration("LEASE_TTL", cfg.LeaseTTL); err != nil {
		return Config{}, err
	}
	if cfg.CacheTTL, err = duration("CACHE_TTL", cfg.CacheTTL); err != nil {
		return Config{}, err
	}
	if cfg.BotToken == "" || cfg.ACRHost == "" || cfg.ACRKey == "" || cfg.ACRSecret == "" {
		return Config{}, fmt.Errorf("missing required environment variables: TELEGRAM_BOT_TOKEN, ACR_HOST, ACR_KEY, ACR_SECRET")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func positiveInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return n, nil
}

func duration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return d, nil
}
