package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"telegram-audio-bot/internal/adapter/downloader"
	"telegram-audio-bot/internal/adapter/extractor"
	"telegram-audio-bot/internal/adapter/recognizer"
	"telegram-audio-bot/internal/adapter/telegram"
	"telegram-audio-bot/internal/usecase"
)

func main() {
	// Initialize structured slog logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	acrHost := os.Getenv("ACR_HOST")
	acrKey := os.Getenv("ACR_KEY")
	acrSecret := os.Getenv("ACR_SECRET")
	cookiesPath := os.Getenv("COOKIES_PATH")
	if cookiesPath == "" {
		cookiesPath = "/app/cookies.txt"
	}

	if botToken == "" || acrHost == "" || acrKey == "" || acrSecret == "" {
		slog.Error("Missing required environment variables (TELEGRAM_BOT_TOKEN, ACR_HOST, ACR_KEY, ACR_SECRET)")
		os.Exit(1)
	}

	slog.Info("Initializing bot components and adapters")

	// 1. Adapters
	mediaExtractor := extractor.NewYtDlpExtractor(cookiesPath)

	acrRecognizer := recognizer.NewACRCloudRecognizer(acrHost, acrKey, acrSecret)

	shazamScriptPath := os.Getenv("SHAZAM_SCRIPT_PATH")
	if shazamScriptPath == "" {
		shazamScriptPath = "/app/scripts/shazam_recognize.py"
	}
	shazamRecognizer := recognizer.NewShazamIORecognizer(shazamScriptPath)

	auddToken := os.Getenv("AUDD_API_TOKEN")
	auddRecognizer := recognizer.NewAudDRecognizer(auddToken, &http.Client{Timeout: 5 * time.Second})

	compositeRecognizer := recognizer.NewFallbackRecognizer(
		recognizer.Engine{Name: "ShazamIO", Recognizer: shazamRecognizer, Timeout: 4 * time.Second},
		recognizer.Engine{Name: "AudD", Recognizer: auddRecognizer, Timeout: 3 * time.Second},
		recognizer.Engine{Name: "ACRCloud", Recognizer: acrRecognizer, Timeout: 3 * time.Second},
	)

	platformRecognizer := recognizer.NewPlatformScraperRecognizer(cookiesPath)
	soundCloudDownloader := downloader.NewSoundCloudDownloader()
	youTubeDownloader := downloader.NewYouTubeDownloader()
	musicDownloader := downloader.NewFallbackDownloader(
		downloader.NamedDownloader{Name: "SoundCloud", Downloader: soundCloudDownloader},
		downloader.NamedDownloader{Name: "YouTube", Downloader: youTubeDownloader},
	)

	// 2. Use Case
	processReelUC := usecase.NewProcessReelUseCase(mediaExtractor, compositeRecognizer, platformRecognizer, musicDownloader)

	workerCount := 5
	if wcStr := os.Getenv("WORKER_COUNT"); wcStr != "" {
		if wc, err := strconv.Atoi(wcStr); err == nil && wc > 0 {
			workerCount = wc
		}
	}

	// 3. Telegram Controller
	channelID := os.Getenv("CHANNEL_ID")
	botHandler := telegram.NewBotHandler(botToken, processReelUC, workerCount, channelID)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := botHandler.Start(ctx); err != nil {
		slog.Error("Fatal bot error", "err", err)
		os.Exit(1)
	}
}
