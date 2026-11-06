package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"telegram-audio-bot/internal/adapter/telegram/copy"
	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// BotHandler is the thin Telegram facade: start loop, route commands/callbacks, admit media jobs.
type BotHandler struct {
	token string
	deps  Deps
}

func NewBotHandler(token string, uc usecase.ProcessMediaUseCase, maxWorkers int, channelID string) *BotHandler {
	h := &BotHandler{
		token: token,
		deps: Deps{
			MediaUC:        uc,
			ChannelID:      channelID,
			MaxUploadBytes: defaultMaxUploadBytes,
			Infra:          Infra{Workers: NewWorkerPool(maxWorkers, 0)},
			Presentation:   Presentation{Router: NewCallbackRouter()},
		},
	}
	h.deps.Infra.Delivery = NewDelivery(channelID, nil, nil, nil, h.deps.MaxUploadBytes)
	h.rebuildPresentation()
	return h
}

func NewProtectedBotHandler(token string, uc usecase.ProcessMediaUseCase, maxWorkers, queueLimit int, channelID string, rateLimiter domain.RateLimiter, leases domain.Lease, rateLimit int, rateWindow, leaseTTL, requestTTL time.Duration) *BotHandler {
	h := NewBotHandler(token, uc, maxWorkers, channelID)
	h.deps.Infra.Workers = NewWorkerPool(maxWorkers, requestTTL)
	h.deps.Infra.Admission = NewAdmission(rateLimiter, leases, maxWorkers, queueLimit, rateLimit, rateWindow, leaseTTL)
	return h
}

func (h *BotHandler) WithMaxUploadBytes(n int64) *BotHandler {
	if n > 0 {
		h.deps.MaxUploadBytes = n
	}
	h.rebuildDelivery()
	return h
}

func (h *BotHandler) WithRepositories(users domain.UserRepository, settings domain.SettingsRepository, requests domain.RequestRepository, tracks domain.TrackRepository, favorites domain.FavoriteRepository) *BotHandler {
	h.deps.Repos = Repos{Users: users, Settings: settings, Requests: requests, Tracks: tracks, Favorites: favorites}
	h.rebuildDelivery()
	h.rebuildPresentation()
	return h
}

func (h *BotHandler) Start(ctx context.Context) error {
	opts := []bot.Option{
		bot.WithDefaultHandler(h.handleMessage),
		bot.WithCallbackQueryDataHandler("", bot.MatchTypePrefix, h.handleCallback),
	}
	b, err := bot.New(h.token, opts...)
	if err != nil {
		return fmt.Errorf("failed to create bot: %w", err)
	}
	slog.Info("Telegram Bot started listening for messages")
	b.Start(ctx)
	return nil
}

func (h *BotHandler) handleCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if err := h.deps.Presentation.Router.Handle(ctx, b, update); err != nil {
		slog.Warn("callback handler failed", "err", err)
	}
}

func (h *BotHandler) handleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID, userID := update.Message.Chat.ID, update.Message.From.ID
	text := strings.TrimSpace(update.Message.Text)
	if h.deps.Presentation.Commands != nil && h.deps.Presentation.Commands.Handle(ctx, b, chatID, userID, text) {
		return
	}
	classified, ok := classifyIncoming(text, update.Message)
	if !ok {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: copy.InvalidInput})
		return
	}
	normalizedURL := normalizeSourceURL(classified.SourceURL)
	var permit *AdmissionPermit
	if h.deps.Infra.Admission != nil {
		var err error
		if permit, err = h.deps.Infra.Admission.TryAdmit(ctx, userID, normalizedURL); err != nil {
			h.replyAdmissionError(ctx, b, chatID, userID, err)
			return
		}
	}
	request := h.createRequest(ctx, update.Message.From, normalizedURL)
	slog.Info("processing new media request", "chat_id", chatID, "user_id", userID, "source_kind", classified.Kind, "url", classified.SourceURL)
	statusMsg, _ := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: copy.StatusReceived(classified.Kind)})
	h.submitMediaJob(ctx, b, mediaJob{
		ChatID: chatID, UserID: userID, SourceKind: classified.Kind, SourceURL: classified.SourceURL,
		MediaID: classified.MediaID, MediaUniqueID: classified.MediaUniqueID, StatusMsg: statusMsg, Request: request, Permit: permit,
	})
}
