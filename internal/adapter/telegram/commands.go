package telegram

import (
	"context"
	"strings"

	"telegram-audio-bot/internal/adapter/telegram/copy"
	"telegram-audio-bot/internal/domain"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Commands handles slash-command UX (history, favorites, settings, forget, help).
type Commands struct {
	users      domain.UserRepository
	settings   domain.SettingsRepository
	requests   domain.RequestRepository
	favorites  domain.FavoriteRepository
	ensureUser func(context.Context, *models.User) *domain.User
}

func NewCommands(
	users domain.UserRepository,
	settings domain.SettingsRepository,
	requests domain.RequestRepository,
	favorites domain.FavoriteRepository,
	ensureUser func(context.Context, *models.User) *domain.User,
) *Commands {
	return &Commands{
		users:      users,
		settings:   settings,
		requests:   requests,
		favorites:  favorites,
		ensureUser: ensureUser,
	}
}

func (c *Commands) Handle(ctx context.Context, b *bot.Bot, chatID, telegramID int64, text string) bool {
	switch text {
	case "/history":
		c.SendHistory(ctx, b, chatID, telegramID, 0)
		return true
	case "/favorites":
		c.SendFavorites(ctx, b, chatID, telegramID, 0)
		return true
	case "/settings":
		c.SendSettings(ctx, b, chatID, telegramID)
		return true
	case "/forget":
		c.ForgetUser(ctx, b, chatID, telegramID)
		return true
	case "/start", "/help":
		c.SendWelcome(ctx, b, chatID)
		return true
	default:
		return false
	}
}

func (c *Commands) SendWelcome(ctx context.Context, b *bot.Bot, chatID int64) {
	if b == nil {
		return
	}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        copy.Welcome,
		ReplyMarkup: copy.WelcomeKeyboard(),
	})
}

func (c *Commands) SendHistory(ctx context.Context, b *bot.Bot, chatID, telegramID int64, offset int) {
	if c == nil || c.users == nil || c.requests == nil {
		sendText(ctx, b, chatID, copy.HistoryDisabled)
		return
	}
	user := c.ensureUser(ctx, &models.User{ID: telegramID})
	if user == nil {
		sendText(ctx, b, chatID, copy.HistoryLoadFailed)
		return
	}
	items, err := c.requests.ListByUser(ctx, user.ID, domain.Page{Limit: 10, Offset: offset})
	if err != nil || len(items) == 0 {
		sendText(ctx, b, chatID, copy.HistoryEmpty)
		return
	}
	var lines strings.Builder
	lines.WriteString(copy.HistoryHeader)
	for i, item := range items {
		lines.WriteString(copy.HistoryLine(offset+i+1, copy.RequestLabel(string(item.Status)), item.URL))
	}
	sendText(ctx, b, chatID, lines.String())
}

func (c *Commands) SendFavorites(ctx context.Context, b *bot.Bot, chatID, telegramID int64, offset int) {
	if c == nil || c.users == nil || c.favorites == nil {
		sendText(ctx, b, chatID, copy.FavoritesDisabled)
		return
	}
	user := c.ensureUser(ctx, &models.User{ID: telegramID})
	if user == nil {
		sendText(ctx, b, chatID, copy.FavoritesLoadFailed)
		return
	}
	items, err := c.favorites.ListByUser(ctx, user.ID, domain.Page{Limit: 10, Offset: offset})
	if err != nil || len(items) == 0 {
		sendText(ctx, b, chatID, copy.FavoritesEmpty)
		return
	}
	var lines strings.Builder
	lines.WriteString(copy.FavoritesHeader)
	for i, item := range items {
		lines.WriteString(copy.FavoriteLine(offset+i+1, item.TrackID))
	}
	sendText(ctx, b, chatID, lines.String())
}

func (c *Commands) SendSettings(ctx context.Context, b *bot.Bot, chatID, telegramID int64) {
	if c == nil || c.users == nil || c.settings == nil {
		sendText(ctx, b, chatID, copy.SettingsDisabled)
		return
	}
	user := c.ensureUser(ctx, &models.User{ID: telegramID})
	if user == nil {
		sendText(ctx, b, chatID, copy.SettingsLoadFailed)
		return
	}
	settings, err := c.settings.Get(ctx, user.ID)
	if err != nil {
		settings = &domain.UserSettings{UserID: user.ID, OutputMode: domain.OutputModeFull, KeepHistory: true}
		_ = c.settings.Upsert(ctx, settings)
	}
	if b == nil {
		return
	}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        copy.SettingsPrompt,
		ReplyMarkup: copy.SettingsModeKeyboard(settings.OutputMode.OrDefault()),
	})
}

func (c *Commands) ForgetUser(ctx context.Context, b *bot.Bot, chatID, telegramID int64) {
	if c == nil || c.users == nil {
		sendText(ctx, b, chatID, copy.ForgetNothing)
		return
	}
	user, err := c.users.GetByTelegramID(ctx, telegramID)
	if err == nil {
		if err := c.users.Delete(ctx, user.ID); err != nil {
			sendText(ctx, b, chatID, copy.ForgetFailed)
			return
		}
	}
	sendText(ctx, b, chatID, copy.ForgetSuccess)
}

func sendText(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	if b == nil {
		return
	}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text})
}
