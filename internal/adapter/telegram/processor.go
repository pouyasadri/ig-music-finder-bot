package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"telegram-audio-bot/internal/adapter/telegram/copy"
	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type mediaJob struct {
	ChatID        int64
	UserID        int64
	SourceKind    domain.SourceKind
	SourceURL     string
	MediaID       string
	MediaUniqueID string
	StatusMsg     *models.Message
	Request       *domain.Request
	Permit        *AdmissionPermit
}

func (h *BotHandler) submitMediaJob(ctx context.Context, b *bot.Bot, job mediaJob) {
	go h.runMediaJob(ctx, b, job)
}

func (h *BotHandler) runMediaJob(ctx context.Context, b *bot.Bot, job mediaJob) {
	if job.Permit != nil {
		defer job.Permit.Release()
	}

	workers := h.deps.Infra.Workers
	delivery := h.deps.Infra.Delivery

	workCtx, cancel := workers.WithTimeout(ctx)
	defer cancel()
	startTime := time.Now()

	if workers != nil && workers.Busy() {
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    job.ChatID,
			MessageID: job.StatusMsg.ID,
			Text:      copy.WorkerQueued,
		})
	}

	releaseWorker := workers.Acquire()
	defer releaseWorker()

	workDir := filepath.Join(getTempBaseDir(), fmt.Sprintf("bot_req_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		slog.Error("failed to create workdir", "err", err, "dir", workDir)
		delivery.Fail(ctx, b, job.ChatID, job.StatusMsg, job.Request, err)
		return
	}
	defer os.RemoveAll(workDir)

	sourceURL := job.SourceURL
	if job.MediaID != "" {
		mediaPath, downloadErr := downloadTelegramMedia(ctx, b, job.MediaID, job.MediaUniqueID, workDir)
		if downloadErr != nil {
			delivery.Fail(ctx, b, job.ChatID, job.StatusMsg, job.Request, downloadErr)
			return
		}
		sourceURL = "file://" + mediaPath
	}

	stopChatAction := make(chan struct{})
	go keepChatActionActive(workCtx, b, job.ChatID, stopChatAction)
	defer close(stopChatAction)

	progress := newMessageProgressReporter(func(msg string) {
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    job.ChatID,
			MessageID: job.StatusMsg.ID,
			Text:      msg,
		})
	})

	payload, err := h.deps.MediaUC.Execute(workCtx, usecase.ProcessMediaInput{
		Kind:      job.SourceKind,
		SourceURL: sourceURL,
		WorkDir:   workDir,
	}, progress)
	if err != nil {
		slog.Error("failed to process media", "chat_id", job.ChatID, "err", err, "url", sourceURL, "source_kind", job.SourceKind)
		delivery.Fail(ctx, b, job.ChatID, job.StatusMsg, job.Request, err)
		return
	}

	delivery.ApplyOutputMode(ctx, job.Request, payload)

	result, err := delivery.SendAudio(ctx, b, job.ChatID, job.StatusMsg, payload, sourceURL, job.Request)
	if err != nil {
		if errors.Is(err, domain.ErrFileTooLarge) {
			slog.Warn("audio file exceeds 50MB limit")
		} else {
			slog.Error("failed to send audio file to user", "chat_id", job.ChatID, "err", err)
		}
		delivery.Fail(ctx, b, job.ChatID, job.StatusMsg, job.Request, err)
		return
	}

	slog.Info("media processed and delivered successfully",
		"chat_id", job.ChatID,
		"source_kind", job.SourceKind,
		"is_full_track", payload.IsFullTrack,
		"title", payload.Title,
		"clean_filename", result.Filename,
		"has_thumbnail", result.HasThumbnail,
		"upload_ms", result.UploadMs,
		"total_ms", time.Since(startTime).Milliseconds(),
	)
}
