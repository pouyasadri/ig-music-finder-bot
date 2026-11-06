package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"telegram-audio-bot/internal/adapter/telegram/copy"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// CallbackHandler defines a handler for Telegram callback queries.
type CallbackHandler func(ctx context.Context, b *bot.Bot, update *models.Update) error

// CallbackRouter routes callback queries to appropriate handlers based on prefix matching.
type CallbackRouter struct {
	handlers map[string]CallbackHandler
}

func NewCallbackRouter() *CallbackRouter {
	return &CallbackRouter{handlers: make(map[string]CallbackHandler)}
}

func (r *CallbackRouter) Register(prefix string, handler CallbackHandler) {
	r.handlers[prefix] = handler
}

func (r *CallbackRouter) Handle(ctx context.Context, b *bot.Bot, update *models.Update) error {
	if r == nil || update.CallbackQuery == nil {
		return nil
	}
	data := update.CallbackQuery.Data
	var (
		bestPrefix  string
		bestHandler CallbackHandler
	)
	for prefix, handler := range r.handlers {
		if strings.HasPrefix(data, prefix) && len(prefix) >= len(bestPrefix) {
			bestPrefix = prefix
			bestHandler = handler
		}
	}
	if bestHandler != nil {
		return bestHandler(ctx, b, update)
	}
	SafeAnswerCallback(ctx, b, update.CallbackQuery.ID, copy.CallbackInvalid, false)
	return nil
}

func SafeAnswerCallback(ctx context.Context, b *bot.Bot, callbackID, text string, isAlert bool) {
	if b == nil {
		return
	}
	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
		ShowAlert:       isAlert,
	})
	if err != nil {
		slog.Warn("failed to answer callback query", "callback_id", callbackID, "err", err)
	}
}

func SafeEditMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int, text string, markup *models.InlineKeyboardMarkup) error {
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ReplyMarkup: markup,
	})
	if err != nil {
		slog.Debug("failed to edit message, sending new message", "chat_id", chatID, "err", err)
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: markup,
		})
	}
	return err
}

func ParseCallbackData(data, prefix string) (string, bool) {
	if !strings.HasPrefix(data, prefix) || len(data) <= len(prefix) {
		return "", false
	}
	return data[len(prefix):], true
}

func ExtractPageFromCallback(data, prefix string) (int, error) {
	suffix, ok := ParseCallbackData(data, prefix)
	if !ok {
		return 0, fmt.Errorf("invalid callback format")
	}
	page, err := strconv.Atoi(suffix)
	if err != nil {
		return 0, fmt.Errorf("invalid page number: %w", err)
	}
	if page < 0 {
		page = 0
	}
	return page, nil
}
