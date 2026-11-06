package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"telegram-audio-bot/internal/adapter/downloader"
	"telegram-audio-bot/internal/adapter/extractor"
	"telegram-audio-bot/internal/adapter/recognizer"
	"telegram-audio-bot/internal/adapter/recognizer/fallback"
	"telegram-audio-bot/internal/adapter/telegram"
	"telegram-audio-bot/internal/config"
	"telegram-audio-bot/internal/storage/sqlite"
	"telegram-audio-bot/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0755); err != nil {
		slog.Error("failed to create database directory", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	db, err := sqlite.Open(ctx, cfg.DatabasePath)
	if err != nil {
		slog.Error("failed to open sqlite database", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("configuration loaded", "worker_count", cfg.WorkerCount, "database_path", cfg.DatabasePath, "rate_limit", cfg.RateLimit)

	slog.Info("Initializing bot components and adapters")

	mediaExtractor := extractor.NewYtDlpExtractor(cfg.CookiesPath)
	acrRecognizer := recognizer.NewACRCloudRecognizer(cfg.ACRHost, cfg.ACRKey, cfg.ACRSecret)
	shazamRecognizer := recognizer.NewShazamIORecognizer(cfg.ShazamScriptPath)
	auddRecognizer := recognizer.NewAudDRecognizer(cfg.AuddToken, &http.Client{Timeout: 5 * time.Second})
	compositeRecognizer := fallback.New(
		fallback.Engine{Name: "ShazamIO", Recognizer: shazamRecognizer, Timeout: 4 * time.Second},
		fallback.Engine{Name: "AudD", Recognizer: auddRecognizer, Timeout: 3 * time.Second},
		fallback.Engine{Name: "ACRCloud", Recognizer: acrRecognizer, Timeout: 3 * time.Second},
	)
	platformRecognizer := recognizer.NewPlatformScraperRecognizer(cfg.CookiesPath)
	soundCloudDownloader := downloader.NewSoundCloudDownloader()
	youTubeDownloader := downloader.NewYouTubeDownloader()
	musicDownloader := downloader.NewFallbackDownloader(
		downloader.NamedDownloader{Name: "SoundCloud", Downloader: soundCloudDownloader},
		downloader.NamedDownloader{Name: "YouTube", Downloader: youTubeDownloader},
	)

	metadataCache := sqlite.NewMetadataCacheAdapter(sqlite.NewCacheRepository(db))
	metadataResolver := usecase.NewCachedTrackMetadataResolver(
		usecase.NewTrackMetadataResolver(compositeRecognizer, platformRecognizer),
		metadataCache,
		cfg.CacheTTL,
	)
	processMediaUC := usecase.NewProcessMediaUseCase(mediaExtractor, metadataResolver, musicDownloader, soundCloudDownloader)

	rateLimiter := sqlite.NewRateLimitRepository(db)
	leases := sqlite.NewLeaseRepository(db)
	botHandler := telegram.NewProtectedBotHandler(cfg.BotToken, processMediaUC, cfg.WorkerCount, cfg.QueueLimit, cfg.ChannelID, rateLimiter, leases, cfg.RateLimit, cfg.RateWindow, cfg.LeaseTTL, cfg.RequestTimeout).
		WithRepositories(
			sqlite.NewUserRepository(db),
			sqlite.NewSettingsRepository(db),
			sqlite.NewRequestRepository(db),
			sqlite.NewTrackRepository(db),
			sqlite.NewFavoriteRepository(db),
		)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := botHandler.Start(ctx); err != nil {
		slog.Error("Fatal bot error", "err", err)
		os.Exit(1)
	}
}
