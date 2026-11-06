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

	ExtractTimeout            time.Duration
	YouTubeDownloadTimeout    time.Duration
	SoundCloudDownloadTimeout time.Duration
	RecognizeShazamTimeout    time.Duration
	RecognizeAudDTimeout      time.Duration
	RecognizeACRTimeout       time.Duration
	RecognizeScrapeTimeout    time.Duration
	AudDHTTPTimeout           time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		BotToken:         os.Getenv("TELEGRAM_BOT_TOKEN"),
		ACRHost:          os.Getenv("ACR_HOST"),
		ACRKey:           os.Getenv("ACR_KEY"),
		ACRSecret:        os.Getenv("ACR_SECRET"),
		CookiesPath:      envOr("COOKIES_PATH", "/app/cookies.txt"),
		ShazamScriptPath: shazamScriptPath(),
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

		ExtractTimeout:            60 * time.Second,
		YouTubeDownloadTimeout:    90 * time.Second,
		SoundCloudDownloadTimeout: 45 * time.Second,
		RecognizeShazamTimeout:    4 * time.Second,
		RecognizeAudDTimeout:      3 * time.Second,
		RecognizeACRTimeout:       3 * time.Second,
		RecognizeScrapeTimeout:    3 * time.Second,
		AudDHTTPTimeout:           5 * time.Second,
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
	if cfg.MaxUploadBytes, err = positiveInt64("MAX_UPLOAD_BYTES", cfg.MaxUploadBytes); err != nil {
		return Config{}, err
	}
	if cfg.ExtractTimeout, err = duration("EXTRACT_TIMEOUT", cfg.ExtractTimeout); err != nil {
		return Config{}, err
	}
	if cfg.YouTubeDownloadTimeout, err = duration("YOUTUBE_DOWNLOAD_TIMEOUT", cfg.YouTubeDownloadTimeout); err != nil {
		return Config{}, err
	}
	if cfg.SoundCloudDownloadTimeout, err = duration("SOUNDCLOUD_DOWNLOAD_TIMEOUT", cfg.SoundCloudDownloadTimeout); err != nil {
		return Config{}, err
	}
	if cfg.RecognizeShazamTimeout, err = duration("RECOGNIZE_SHAZAM_TIMEOUT", cfg.RecognizeShazamTimeout); err != nil {
		return Config{}, err
	}
	if cfg.RecognizeAudDTimeout, err = duration("RECOGNIZE_AUDD_TIMEOUT", cfg.RecognizeAudDTimeout); err != nil {
		return Config{}, err
	}
	if cfg.RecognizeACRTimeout, err = duration("RECOGNIZE_ACR_TIMEOUT", cfg.RecognizeACRTimeout); err != nil {
		return Config{}, err
	}
	if cfg.RecognizeScrapeTimeout, err = duration("RECOGNIZE_SCRAPE_TIMEOUT", cfg.RecognizeScrapeTimeout); err != nil {
		return Config{}, err
	}
	if cfg.AudDHTTPTimeout, err = duration("AUDD_HTTP_TIMEOUT", cfg.AudDHTTPTimeout); err != nil {
		return Config{}, err
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) HasACR() bool {
	return c.ACRHost != "" && c.ACRKey != "" && c.ACRSecret != ""
}

func (c Config) HasAudD() bool {
	return c.AuddToken != ""
}

func (c Config) HasShazam() bool {
	return c.ShazamScriptPath != ""
}

func (c Config) validate() error {
	if c.BotToken == "" {
		return fmt.Errorf("missing required environment variable: TELEGRAM_BOT_TOKEN")
	}
	acrPartial := c.ACRHost != "" || c.ACRKey != "" || c.ACRSecret != ""
	if acrPartial && !c.HasACR() {
		return fmt.Errorf("ACR_HOST, ACR_KEY, and ACR_SECRET must all be set together")
	}
	if !c.HasACR() && !c.HasAudD() && !c.HasShazam() {
		return fmt.Errorf("at least one recognizer is required: configure ACR_HOST/ACR_KEY/ACR_SECRET, AUDD_API_TOKEN, or SHAZAM_SCRIPT_PATH")
	}
	return nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// shazamScriptPath defaults when unset; an explicitly empty SHAZAM_SCRIPT_PATH disables Shazam.
func shazamScriptPath() string {
	if value, ok := os.LookupEnv("SHAZAM_SCRIPT_PATH"); ok {
		return value
	}
	return "/app/scripts/shazam_recognize.py"
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

func positiveInt64(key string, fallback int64) (int64, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
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
