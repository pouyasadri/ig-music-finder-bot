package telegram

import (
	"context"
	"errors"
	"log/slog"

	"telegram-audio-bot/internal/adapter/telegram/copy"
	"telegram-audio-bot/internal/domain"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (h *BotHandler) createRequest(ctx context.Context, from *models.User, normalizedURL string) *domain.Request {
	if h.deps.Repos.Users == nil || h.deps.Repos.Requests == nil {
		return nil
	}
	storedUser := ensureUser(ctx, h.deps.Repos.Users, from)
	if storedUser == nil {
		return nil
	}
	request := &domain.Request{UserID: storedUser.ID, URL: normalizedURL, Status: domain.RequestStatusProcessing}
	if err := h.deps.Repos.Requests.Create(ctx, request); err != nil {
		slog.Warn("failed to persist request", "err", err)
		return nil
	}
	return request
}

func (h *BotHandler) replyAdmissionError(ctx context.Context, b *bot.Bot, chatID, userID int64, err error) {
	switch {
	case errors.Is(err, domain.ErrRateLimited), errors.Is(err, domain.ErrLeaseHeld), errors.Is(err, domain.ErrQueueFull):
		_, msg := copy.ErrorCategoryAndMessage(err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: msg})
	default:
		slog.Error("admission failed", "user_id", userID, "err", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: copy.AdmissionFailed})
	}
}
