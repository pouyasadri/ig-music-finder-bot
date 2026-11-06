package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
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

const janitorInterval = 5 * time.Minute

// App is the composition root for the Telegram music bot.
type App struct {
	cfg    config.Config
	db     *sql.DB
	bot    *telegram.BotHandler
	cache  *sqlite.SQLiteCacheRepository
	leases *sqlite.SQLiteLeaseRepository
}

// New opens storage, wires adapters/use cases, and returns a runnable App.
func New(cfg config.Config) (*App, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sqlite.Open(context.Background(), cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	cache := sqlite.NewCacheRepository(db)
	leases := sqlite.NewLeaseRepository(db)
	botHandler, err := buildBot(cfg, db, cache, leases)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	slog.Info("configuration loaded",
		"worker_count", cfg.WorkerCount,
		"database_path", cfg.DatabasePath,
		"rate_limit", cfg.RateLimit,
		"recognizers", configuredRecognizers(cfg),
	)

	return &App{cfg: cfg, db: db, bot: botHandler, cache: cache, leases: leases}, nil
}

// Start begins background cleanup and the Telegram long-poll loop until ctx is cancelled.
func (a *App) Start(ctx context.Context) error {
	if a == nil || a.bot == nil {
		return fmt.Errorf("app is not initialized")
	}
	go a.runJanitor(ctx)
	return a.bot.Start(ctx)
}

// Close releases application resources.
func (a *App) Close() error {
	if a == nil || a.db == nil {
		return nil
	}
	return a.db.Close()
}

func (a *App) runJanitor(ctx context.Context) {
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()
	a.purgeExpired(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.purgeExpired(ctx)
		}
	}
}

func (a *App) purgeExpired(ctx context.Context) {
	if a.leases != nil {
		if n, err := a.leases.DeleteExpired(ctx); err != nil {
			slog.Warn("lease cleanup failed", "err", err)
		} else if n > 0 {
			slog.Info("expired leases deleted", "count", n)
		}
	}
	if a.cache != nil {
		if n, err := a.cache.DeleteExpired(ctx); err != nil {
			slog.Warn("cache cleanup failed", "err", err)
		} else if n > 0 {
			slog.Info("expired cache entries deleted", "count", n)
		}
	}
}

func buildBot(cfg config.Config, db *sql.DB, cache *sqlite.SQLiteCacheRepository, leases *sqlite.SQLiteLeaseRepository) (*telegram.BotHandler, error) {
	mediaExtractor := extractor.NewYtDlpExtractor(cfg.CookiesPath, cfg.ExtractTimeout)

	engines := make([]fallback.Engine, 0, 3)
	if cfg.HasShazam() {
		engines = append(engines, fallback.Engine{
			Name:       "ShazamIO",
			Recognizer: recognizer.NewShazamIORecognizer(cfg.ShazamScriptPath),
			Timeout:    cfg.RecognizeShazamTimeout,
		})
	}
	if cfg.HasAudD() {
		engines = append(engines, fallback.Engine{
			Name: "AudD",
			Recognizer: recognizer.NewAudDRecognizer(cfg.AuddToken, &http.Client{
				Timeout: cfg.AudDHTTPTimeout,
			}),
			Timeout: cfg.RecognizeAudDTimeout,
		})
	}
	if cfg.HasACR() {
		engines = append(engines, fallback.Engine{
			Name:       "ACRCloud",
			Recognizer: recognizer.NewACRCloudRecognizer(cfg.ACRHost, cfg.ACRKey, cfg.ACRSecret),
			Timeout:    cfg.RecognizeACRTimeout,
		})
	}
	if len(engines) == 0 {
		return nil, fmt.Errorf("no recognition engines configured")
	}

	compositeRecognizer := fallback.New(engines...)
	platformRecognizer := recognizer.NewPlatformScraperRecognizer(cfg.CookiesPath)
	soundCloudDownloader := downloader.NewSoundCloudDownloader(cfg.SoundCloudDownloadTimeout)
	youTubeDownloader := downloader.NewYouTubeDownloader(cfg.YouTubeDownloadTimeout)
	musicDownloader := downloader.NewFallbackDownloader(
		downloader.NamedDownloader{Name: "SoundCloud", Downloader: soundCloudDownloader},
		downloader.NamedDownloader{Name: "YouTube", Downloader: youTubeDownloader},
	)

	metadataResolver := usecase.NewCachedTrackMetadataResolver(
		usecase.NewTrackMetadataResolver(compositeRecognizer, platformRecognizer, cfg.RecognizeScrapeTimeout),
		sqlite.NewMetadataCacheAdapter(cache),
		cfg.CacheTTL,
	)
	processMediaUC := usecase.NewProcessMediaUseCase(mediaExtractor, metadataResolver, musicDownloader, soundCloudDownloader)

	botHandler := telegram.NewProtectedBotHandler(
		cfg.BotToken,
		processMediaUC,
		cfg.WorkerCount,
		cfg.QueueLimit,
		cfg.ChannelID,
		sqlite.NewRateLimitRepository(db),
		leases,
		cfg.RateLimit,
		cfg.RateWindow,
		cfg.LeaseTTL,
		cfg.RequestTimeout,
	).
		WithMaxUploadBytes(cfg.MaxUploadBytes).
		WithRepositories(
			sqlite.NewUserRepository(db),
			sqlite.NewSettingsRepository(db),
			sqlite.NewRequestRepository(db),
			sqlite.NewTrackRepository(db),
			sqlite.NewFavoriteRepository(db),
		)

	return botHandler, nil
}

func configuredRecognizers(cfg config.Config) []string {
	names := make([]string, 0, 3)
	if cfg.HasShazam() {
		names = append(names, "ShazamIO")
	}
	if cfg.HasAudD() {
		names = append(names, "AudD")
	}
	if cfg.HasACR() {
		names = append(names, "ACRCloud")
	}
	return names
}
