package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"telegram-audio-bot/internal/adapter/telegram/copy"
	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var (
	igURLRegex         = regexp.MustCompile(`https?://(?:www\.)?(?:instagram\.com|instagr\.am)/(?:reel|p|share/reel)/[a-zA-Z0-9_\-\.]+/?(?:\?[^\s]*)?`)
	soundCloudURLRegex = regexp.MustCompile(`https?://(?:www\.|m\.)?(?:soundcloud\.com|on\.soundcloud\.com|soundcloud\.app\.goo\.gl)/[^\s]+`)
	sanitizeFilename   = regexp.MustCompile(`[<>:"/\\|?*]`)
)

type BotHandler struct {
	token          string
	mediaUC        usecase.ProcessMediaUseCase
	admission      *Admission
	workers        *WorkerPool
	delivery       *Delivery
	commands       *Commands
	callbacks      *Callbacks
	router         *CallbackRouter
	channelID      string
	maxUploadBytes int64
	users          domain.UserRepository
	settings       domain.SettingsRepository
	requests       domain.RequestRepository
	tracks         domain.TrackRepository
	favorites      domain.FavoriteRepository
}

func NewBotHandler(token string, uc usecase.ProcessMediaUseCase, maxWorkers int, channelID string) *BotHandler {
	h := &BotHandler{
		token:          token,
		mediaUC:        uc,
		workers:        NewWorkerPool(maxWorkers, 0),
		channelID:      channelID,
		maxUploadBytes: defaultMaxUploadBytes,
		router:         NewCallbackRouter(),
	}
	h.delivery = NewDelivery(channelID, nil, nil, nil, h.maxUploadBytes)
	h.rebuildPresentation()
	return h
}

func NewProtectedBotHandler(token string, uc usecase.ProcessMediaUseCase, maxWorkers, queueLimit int, channelID string, rateLimiter domain.RateLimiter, leases domain.Lease, rateLimit int, rateWindow, leaseTTL, requestTTL time.Duration) *BotHandler {
	h := NewBotHandler(token, uc, maxWorkers, channelID)
	h.workers = NewWorkerPool(maxWorkers, requestTTL)
	h.admission = NewAdmission(rateLimiter, leases, maxWorkers, queueLimit, rateLimit, rateWindow, leaseTTL)
	return h
}

func (h *BotHandler) WithMaxUploadBytes(n int64) *BotHandler {
	if n > 0 {
		h.maxUploadBytes = n
	}
	h.delivery = NewDelivery(h.channelID, h.tracks, h.requests, h.settings, h.maxUploadBytes)
	return h
}

func (h *BotHandler) WithRepositories(users domain.UserRepository, settings domain.SettingsRepository, requests domain.RequestRepository, tracks domain.TrackRepository, favorites domain.FavoriteRepository) *BotHandler {
	h.users, h.settings, h.requests, h.tracks, h.favorites = users, settings, requests, tracks, favorites
	h.delivery = NewDelivery(h.channelID, tracks, requests, settings, h.maxUploadBytes)
	h.rebuildPresentation()
	return h
}

func (h *BotHandler) rebuildPresentation() {
	h.commands = NewCommands(h.users, h.settings, h.requests, h.favorites, h.ensureUser)
	h.callbacks = NewCallbacks(h.users, h.settings, h.favorites, h.channelID, h.ensureUser)
	h.router = NewCallbackRouter()
	h.callbacks.Register(h.router)
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
	if err := h.router.Handle(ctx, b, update); err != nil {
		slog.Warn("callback handler failed", "err", err)
	}
}

func (h *BotHandler) handleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	text := strings.TrimSpace(update.Message.Text)
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	if h.commands != nil && h.commands.Handle(ctx, b, chatID, userID, text) {
		return
	}

	igMatch := igURLRegex.FindString(text)
	scMatch := soundCloudURLRegex.FindString(text)
	mediaID, mediaUniqueID := telegramMediaIDs(update.Message)
	if igMatch == "" && scMatch == "" && mediaID == "" {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: copy.InvalidInput})
		return
	}

	sourceKind := domain.SourceKindInstagram
	sourceURL := igMatch
	if sourceURL == "" && scMatch != "" {
		sourceKind = domain.SourceKindSoundCloud
		sourceURL = scMatch
	}
	if sourceURL == "" {
		sourceKind = domain.SourceKindUpload
		sourceURL = "file://" + mediaUniqueID
	}
	normalizedURL := domain.NormalizeMediaURL(sourceURL)

	permit, err := h.admission.TryAdmit(ctx, userID, normalizedURL)
	if err != nil {
		h.replyAdmissionError(ctx, b, chatID, userID, err)
		return
	}

	var request *domain.Request
	if h.users != nil && h.requests != nil {
		storedUser := h.ensureUser(ctx, update.Message.From)
		if storedUser != nil {
			request = &domain.Request{UserID: storedUser.ID, URL: normalizedURL, Status: domain.RequestStatusProcessing}
			if err := h.requests.Create(ctx, request); err != nil {
				slog.Warn("failed to persist request", "err", err)
				request = nil
			}
		}
	}

	slog.Info("processing new media request",
		"chat_id", chatID,
		"user_id", userID,
		"source_kind", sourceKind,
		"url", sourceURL,
	)

	statusMsg, _ := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   copy.StatusReceived(sourceKind),
	})

	h.submitMediaJob(ctx, b, mediaJob{
		ChatID:        chatID,
		UserID:        userID,
		SourceKind:    sourceKind,
		SourceURL:     sourceURL,
		MediaID:       mediaID,
		MediaUniqueID: mediaUniqueID,
		StatusMsg:     statusMsg,
		Request:       request,
		Permit:        permit,
	})
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

func (h *BotHandler) ensureUser(ctx context.Context, user *models.User) *domain.User {
	if user == nil || h.users == nil {
		return nil
	}
	stored, err := h.users.GetByTelegramID(ctx, user.ID)
	if err == nil {
		stored.Username, stored.FirstName, stored.LastName = user.Username, user.FirstName, user.LastName
		_ = h.users.Update(ctx, stored)
		return stored
	}
	stored = &domain.User{TelegramID: user.ID, Username: user.Username, FirstName: user.FirstName, LastName: user.LastName}
	if err := h.users.Create(ctx, stored); err != nil {
		return nil
	}
	return stored
}
