package telegram

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"telegram-audio-bot/internal/adapter/telegram/copy"
	"telegram-audio-bot/internal/domain"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Callbacks owns inline-callback handlers and registers them on a CallbackRouter.
type Callbacks struct {
	users      domain.UserRepository
	settings   domain.SettingsRepository
	favorites  domain.FavoriteRepository
	channelID  string
	ensureUser func(context.Context, *models.User) *domain.User
}

func NewCallbacks(
	users domain.UserRepository,
	settings domain.SettingsRepository,
	favorites domain.FavoriteRepository,
	channelID string,
	ensureUser func(context.Context, *models.User) *domain.User,
) *Callbacks {
	return &Callbacks{
		users:      users,
		settings:   settings,
		favorites:  favorites,
		channelID:  channelID,
		ensureUser: ensureUser,
	}
}

func (c *Callbacks) Register(router *CallbackRouter) {
	if c == nil || router == nil {
		return
	}
	router.Register("settings_mode_", c.handleSettingsMode)
	router.Register("favorite_track_", c.handleFavoriteTrack)
	router.Register("send_to_channel", c.handleSendToChannel)
}

func (c *Callbacks) handleSettingsMode(ctx context.Context, b *bot.Bot, update *models.Update) error {
	if update.CallbackQuery == nil || c.users == nil || c.settings == nil {
		return nil
	}
	user := c.ensureUser(ctx, &update.CallbackQuery.From)
	if user == nil {
		return nil
	}
	mode, ok := domain.ParseOutputMode(strings.TrimPrefix(update.CallbackQuery.Data, "settings_mode_"))
	if !ok {
		SafeAnswerCallback(ctx, b, update.CallbackQuery.ID, copy.SettingsInvalid, true)
		return nil
	}
	settings, err := c.settings.Get(ctx, user.ID)
	if err != nil {
		settings = &domain.UserSettings{UserID: user.ID, KeepHistory: true}
	}
	settings.OutputMode = mode
	if err := c.settings.Upsert(ctx, settings); err != nil {
		SafeAnswerCallback(ctx, b, update.CallbackQuery.ID, copy.SettingsSaveFailed, true)
		return nil
	}
	SafeAnswerCallback(ctx, b, update.CallbackQuery.ID, copy.SettingsSaved, false)
	return nil
}

func (c *Callbacks) handleFavoriteTrack(ctx context.Context, b *bot.Bot, update *models.Update) error {
	if update.CallbackQuery == nil || c.users == nil || c.favorites == nil {
		return nil
	}
	trackID, err := strconv.ParseInt(strings.TrimPrefix(update.CallbackQuery.Data, "favorite_track_"), 10, 64)
	if err != nil {
		SafeAnswerCallback(ctx, b, update.CallbackQuery.ID, copy.FavoriteInvalid, true)
		return nil
	}
	user := c.ensureUser(ctx, &update.CallbackQuery.From)
	if user == nil {
		return nil
	}
	exists, err := c.favorites.Exists(ctx, user.ID, trackID)
	if err != nil {
		return nil
	}
	if exists {
		err = c.favorites.Delete(ctx, user.ID, trackID)
	} else {
		err = c.favorites.Add(ctx, domain.Favorite{UserID: user.ID, TrackID: trackID})
	}
	if err != nil {
		SafeAnswerCallback(ctx, b, update.CallbackQuery.ID, copy.FavoriteSaveFailed, true)
		return nil
	}
	text := copy.FavoriteAdded
	if exists {
		text = copy.FavoriteRemoved
	}
	SafeAnswerCallback(ctx, b, update.CallbackQuery.ID, text, false)
	return nil
}

func (c *Callbacks) handleSendToChannel(ctx context.Context, b *bot.Bot, update *models.Update) error {
	if c.channelID == "" {
		SafeAnswerCallback(ctx, b, update.CallbackQuery.ID, copy.ChannelNotConfigured, true)
		return nil
	}
	msg := update.CallbackQuery.Message.Message
	if msg == nil {
		SafeAnswerCallback(ctx, b, update.CallbackQuery.ID, copy.ChannelMessageMissing, true)
		return nil
	}

	var newRows [][]models.InlineKeyboardButton
	if msg.ReplyMarkup != nil && len(msg.ReplyMarkup.InlineKeyboard) > 0 {
		for _, row := range msg.ReplyMarkup.InlineKeyboard {
			var newRow []models.InlineKeyboardButton
			for _, btn := range row {
				if btn.CallbackData != "send_to_channel" {
					newRow = append(newRow, btn)
				}
			}
			if len(newRow) > 0 {
				newRows = append(newRows, newRow)
			}
		}
	}

	var replyMarkup models.ReplyMarkup
	if len(newRows) > 0 {
		replyMarkup = &models.InlineKeyboardMarkup{InlineKeyboard: newRows}
	}

	_, err := b.CopyMessage(ctx, &bot.CopyMessageParams{
		ChatID:      c.channelID,
		FromChatID:  msg.Chat.ID,
		MessageID:   msg.ID,
		ReplyMarkup: replyMarkup,
	})
	if err != nil {
		slog.Error("failed to copy message to channel", "err", err)
		SafeAnswerCallback(ctx, b, update.CallbackQuery.ID, copy.ChannelSendFailed, true)
		return nil
	}
	SafeAnswerCallback(ctx, b, update.CallbackQuery.ID, copy.ChannelSendSuccess, false)
	return nil
}
