package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"telegram-audio-bot/internal/domain"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// CallbackHandler defines a handler for Telegram callback queries.
type CallbackHandler func(ctx context.Context, b *bot.Bot, update *models.Update) error

// CallbackRouter routes callback queries to appropriate handlers based on prefix matching.
type CallbackRouter struct {
	handlers map[string]CallbackHandler
}

// NewCallbackRouter creates a new callback router.
func NewCallbackRouter() *CallbackRouter {
	return &CallbackRouter{
		handlers: make(map[string]CallbackHandler),
	}
}

// Register adds a handler for a callback prefix.
func (r *CallbackRouter) Register(prefix string, handler CallbackHandler) {
	r.handlers[prefix] = handler
}

// Handle routes a callback query to the appropriate handler.
func (r *CallbackRouter) Handle(ctx context.Context, b *bot.Bot, update *models.Update) error {
	if update.CallbackQuery == nil {
		return nil
	}

	data := update.CallbackQuery.Data
	for prefix, handler := range r.handlers {
		if strings.HasPrefix(data, prefix) {
			return handler(ctx, b, update)
		}
	}

	// No handler found, answer with a generic message
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		Text:            "درخواست نامعتبر است.",
		ShowAlert:       false,
	})
	return nil
}

// OwnershipValidator validates that a callback action belongs to the requesting user.
type OwnershipValidator struct {
	users domain.UserRepository
}

// NewOwnershipValidator creates a new ownership validator.
func NewOwnershipValidator(users domain.UserRepository) *OwnershipValidator {
	return &OwnershipValidator{users: users}
}

// ValidateOwnership checks that a user owns a request or favorite.
func (v *OwnershipValidator) ValidateOwnership(ctx context.Context, telegramUserID, ownedUserID int64) bool {
	if ownedUserID <= 0 || telegramUserID <= 0 {
		return false
	}
	storedUser, err := v.users.GetByTelegramID(ctx, telegramUserID)
	if err != nil || storedUser == nil {
		return false
	}
	return storedUser.ID == ownedUserID
}

// SafeAnswerCallback answers a callback query and handles common errors gracefully.
func SafeAnswerCallback(ctx context.Context, b *bot.Bot, callbackID, text string, isAlert bool) {
	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
		ShowAlert:       isAlert,
	})
	if err != nil {
		slog.Warn("failed to answer callback query", "callback_id", callbackID, "err", err)
	}
}

// SafeEditMessage edits a message or falls back to sending a new one if the message cannot be edited.
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

// BuildDeliveryKeyboard constructs the interactive keyboard for a delivered track.
func BuildDeliveryKeyboard(payload *domain.AudioPayload, trackID int64, isFavorite bool, reelURL string) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton

	// Favorite toggle
	favoriteText := "❤️ ذخیره"
	if isFavorite {
		favoriteText = "💔 حذف از ذخیره"
	}
	if trackID > 0 {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: favoriteText, CallbackData: fmt.Sprintf("favorite:toggle:%d", trackID)},
			{Text: "🔁 دانلود دوباره", CallbackData: fmt.Sprintf("result:retry:%d", trackID)},
		})
	}

	// Original audio button if full track
	if payload.IsFullTrack && payload.OriginalPath != "" {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: "🎶 صدای اصلی", CallbackData: fmt.Sprintf("result:original:%d", trackID)},
		})
	}

	// External links
	var linkRow []models.InlineKeyboardButton
	if payload.YouTubeURL != "" {
		linkRow = append(linkRow, models.InlineKeyboardButton{
			Text: "📺 یوتیوب",
			URL:  payload.YouTubeURL,
		})
	}
	if payload.SpotifyURL != "" {
		linkRow = append(linkRow, models.InlineKeyboardButton{
			Text: "🟢 اسپاتیفای",
			URL:  payload.SpotifyURL,
		})
	}
	if reelURL != "" && !strings.HasPrefix(reelURL, "file://") {
		linkRow = append(linkRow, models.InlineKeyboardButton{
			Text: "🔗 مشاهده پست",
			URL:  reelURL,
		})
	}
	if len(linkRow) > 0 {
		rows = append(rows, linkRow)
	}

	if len(rows) == 0 {
		return nil
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// BuildHistoryItemKeyboard constructs the keyboard for a single history item.
func BuildHistoryItemKeyboard(requestID int64) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🎵 دریافت دوباره", CallbackData: fmt.Sprintf("history:retry:%d", requestID)},
				{Text: "❤️ ذخیره", CallbackData: fmt.Sprintf("history:favorite:%d", requestID)},
			},
			{
				{Text: "🗑 حذف", CallbackData: fmt.Sprintf("history:delete:%d", requestID)},
			},
		},
	}
}

// BuildFavoritesItemKeyboard constructs the keyboard for a single favorite item.
func BuildFavoritesItemKeyboard(trackID int64) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "⬇️ دانلود", CallbackData: fmt.Sprintf("favorite:download:%d", trackID)},
				{Text: "💔 حذف", CallbackData: fmt.Sprintf("favorite:delete:%d", trackID)},
			},
		},
	}
}

// BuildSettingsKeyboard constructs the settings menu keyboard.
func BuildSettingsKeyboard(currentMode string) *models.InlineKeyboardMarkup {
	selected := func(mode, current string) string {
		if mode == current {
			return " ✅"
		}
		return ""
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: "🎵 آهنگ کامل" + selected("full", currentMode), CallbackData: "settings:output:full"}},
			{{Text: "🎶 صدای اصلی" + selected("original", currentMode), CallbackData: "settings:output:original"}},
			{{Text: "📦 هر دو" + selected("both", currentMode), CallbackData: "settings:output:both"}},
			{{Text: "🏠 برگشت", CallbackData: "menu:home"}},
		},
	}
}

// BuildPrivacyConfirmationKeyboard constructs the privacy deletion confirmation keyboard.
func BuildPrivacyConfirmationKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ بله، حذف کن", CallbackData: "privacy:forget:confirm"},
				{Text: "❌ انصراف", CallbackData: "privacy:forget:cancel"},
			},
		},
	}
}

// ExtractPageFromCallback parses a page number from pagination callback data.
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

// ExtractIDFromCallback parses an ID from a callback data string.
func ExtractIDFromCallback(data, prefix string) (int64, error) {
	suffix, ok := ParseCallbackData(data, prefix)
	if !ok {
		return 0, fmt.Errorf("invalid callback format")
	}
	id, err := strconv.ParseInt(suffix, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id: %w", err)
	}
	return id, nil
}
