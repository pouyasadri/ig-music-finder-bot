package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"telegram-audio-bot/internal/app"
	"telegram-audio-bot/internal/config"
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

	application, err := app.New(cfg)
	if err != nil {
		slog.Error("failed to initialize application", "err", err)
		os.Exit(1)
	}
	defer application.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := application.Start(ctx); err != nil {
		slog.Error("Fatal bot error", "err", err)
		os.Exit(1)
	}
}
