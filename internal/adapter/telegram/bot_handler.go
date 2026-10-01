package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	token        string
	useCase      usecase.ReelAudioUseCase
	soundCloudUC usecase.SoundCloudAudioUseCase
	admission    *Admission
	workers      *WorkerPool
	delivery     *Delivery
	channelID    string
	users        domain.UserRepository
	settings     domain.SettingsRepository
	requests     domain.RequestRepository
	tracks       domain.TrackRepository
	favorites    domain.FavoriteRepository
}

func NewBotHandler(token string, uc usecase.ReelAudioUseCase, maxWorkers int, channelID string) *BotHandler {
	return &BotHandler{
		token:     token,
		useCase:   uc,
		workers:   NewWorkerPool(maxWorkers, 0),
		delivery:  NewDelivery(channelID, nil, nil, nil),
		channelID: channelID,
	}
}

func NewProtectedBotHandler(token string, uc usecase.ReelAudioUseCase, maxWorkers, queueLimit int, channelID string, rateLimiter domain.RateLimiter, leases domain.Lease, rateLimit int, rateWindow, leaseTTL, requestTTL time.Duration) *BotHandler {
	h := NewBotHandler(token, uc, maxWorkers, channelID)
	h.workers = NewWorkerPool(maxWorkers, requestTTL)
	h.admission = NewAdmission(rateLimiter, leases, maxWorkers, queueLimit, rateLimit, rateWindow, leaseTTL)
	return h
}

func (h *BotHandler) WithRepositories(users domain.UserRepository, settings domain.SettingsRepository, requests domain.RequestRepository, tracks domain.TrackRepository, favorites domain.FavoriteRepository) *BotHandler {
	h.users, h.settings, h.requests, h.tracks, h.favorites = users, settings, requests, tracks, favorites
	h.delivery = NewDelivery(h.channelID, tracks, requests, settings)
	return h
}

func (h *BotHandler) WithSoundCloudUseCase(uc usecase.SoundCloudAudioUseCase) *BotHandler {
	h.soundCloudUC = uc
	return h
}

func (h *BotHandler) Start(ctx context.Context) error {
	opts := []bot.Option{
		bot.WithDefaultHandler(h.handleMessage),
		bot.WithCallbackQueryDataHandler("send_to_channel", bot.MatchTypeExact, h.handleSendToChannel),
		bot.WithCallbackQueryDataHandler("settings_mode_", bot.MatchTypePrefix, h.handleSettingsMode),
		bot.WithCallbackQueryDataHandler("favorite_track_", bot.MatchTypePrefix, h.handleFavoriteTrack),
	}
	b, err := bot.New(h.token, opts...)
	if err != nil {
		return fmt.Errorf("failed to create bot: %w", err)
	}

	slog.Info("Telegram Bot started listening for messages")
	b.Start(ctx)
	return nil
}

func (h *BotHandler) handleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	text := strings.TrimSpace(update.Message.Text)
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	switch text {
	case "/history":
		h.sendHistory(ctx, b, chatID, userID, 0)
		return
	case "/favorites":
		h.sendFavorites(ctx, b, chatID, userID, 0)
		return
	case "/settings":
		h.sendSettings(ctx, b, chatID, userID)
		return
	case "/forget":
		h.forgetUser(ctx, b, chatID, userID)
		return
	}

	if text == "/start" || text == "/help" {
		kb := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "اشتراک‌گذاری ربات 🚀", URL: "https://t.me/share/url?url=&text=این+ربات+برای+دانلود+آهنگ+های+اینستاگرام+عالیه!+🎧"},
				},
			},
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text: "سلام! 👋 خوش اومدی.\n\n" +
				"لینک ریلز/پست اینستاگرام یا ترک ساندکلاد رو بفرست تا آهنگش رو برات پیدا کنم 🎧\n" +
				"همچنین می‌تونی فایل صوتی یا ویدیو هم آپلود کنی.\n\n" +
				"دستورات: /history /favorites /settings /forget",
			ReplyMarkup: kb,
		})
		return
	}

	igMatch := igURLRegex.FindString(text)
	scMatch := soundCloudURLRegex.FindString(text)
	mediaID, mediaUniqueID := telegramMediaIDs(update.Message)
	if igMatch == "" && scMatch == "" && mediaID == "" {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text: "❌ لطفاً یک لینک معتبر اینستاگرام یا ساندکلاد بفرست، یا فایل صوتی/ویدیو آپلود کن.\n\n" +
				"مثال اینستاگرام:\nhttps://www.instagram.com/reel/C-xyz123/\n\n" +
				"مثال ساندکلاد:\nhttps://soundcloud.com/artist/track",
		})
		return
	}

	sourceKind := domain.SourceKindInstagram
	sourceURL := igMatch
	statusText := "📥 دریافت شد! در حال استخراج و بررسی صدای ریلز... ⏳"
	if sourceURL == "" && scMatch != "" {
		sourceKind = domain.SourceKindSoundCloud
		sourceURL = scMatch
		statusText = "📥 دریافت شد! در حال دانلود از ساندکلاد... ⏳"
	}
	if sourceURL == "" {
		sourceKind = domain.SourceKindUpload
		sourceURL = "file://" + mediaUniqueID
		statusText = "📥 فایل دریافت شد! در حال پردازش صدا... ⏳"
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
		Text:   statusText,
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
		_, msg := ErrorCategoryAndMessage(err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: msg})
	default:
		slog.Error("admission failed", "user_id", userID, "err", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "❌ خطایی در ثبت درخواست رخ داد. لطفاً دوباره تلاش کنید."})
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

func (h *BotHandler) sendHistory(ctx context.Context, b *bot.Bot, chatID, telegramID int64, offset int) {
	if h.users == nil || h.requests == nil {
		h.sendMessage(ctx, b, chatID, "📚 تاریخچه هنوز فعال نشده است.")
		return
	}
	user := h.ensureUser(ctx, &models.User{ID: telegramID})
	if user == nil {
		h.sendMessage(ctx, b, chatID, "❌ خطا در بارگذاری تاریخچه.")
		return
	}
	items, err := h.requests.ListByUser(ctx, user.ID, domain.Page{Limit: 10, Offset: offset})
	if err != nil || len(items) == 0 {
		h.sendMessage(ctx, b, chatID, "📚 تاریخچه‌ای برای نمایش وجود ندارد.")
		return
	}
	var lines strings.Builder
	lines.WriteString("📚 تاریخچه درخواست‌ها:\n\n")
	for i, item := range items {
		lines.WriteString(fmt.Sprintf("%d. %s — %s\n", offset+i+1, requestLabel(item), item.URL))
	}
	h.sendMessage(ctx, b, chatID, lines.String())
}

func (h *BotHandler) sendFavorites(ctx context.Context, b *bot.Bot, chatID, telegramID int64, offset int) {
	if h.users == nil || h.favorites == nil {
		h.sendMessage(ctx, b, chatID, "❤️ ذخیره‌ها هنوز فعال نشده است.")
		return
	}
	user := h.ensureUser(ctx, &models.User{ID: telegramID})
	if user == nil {
		h.sendMessage(ctx, b, chatID, "❌ خطا در بارگذاری ذخیره‌ها.")
		return
	}
	items, err := h.favorites.ListByUser(ctx, user.ID, domain.Page{Limit: 10, Offset: offset})
	if err != nil || len(items) == 0 {
		h.sendMessage(ctx, b, chatID, "❤️ آهنگ ذخیره‌شده‌ای وجود ندارد.")
		return
	}
	var lines strings.Builder
	lines.WriteString("❤️ آهنگ‌های ذخیره‌شده:\n\n")
	for i, item := range items {
		lines.WriteString(fmt.Sprintf("%d. track #%d\n", offset+i+1, item.TrackID))
	}
	h.sendMessage(ctx, b, chatID, lines.String())
}

func (h *BotHandler) sendSettings(ctx context.Context, b *bot.Bot, chatID, telegramID int64) {
	if h.users == nil || h.settings == nil {
		h.sendMessage(ctx, b, chatID, "⚙️ تنظیمات هنوز فعال نشده است.")
		return
	}
	user := h.ensureUser(ctx, &models.User{ID: telegramID})
	if user == nil {
		h.sendMessage(ctx, b, chatID, "❌ خطا در بارگذاری تنظیمات.")
		return
	}
	settings, err := h.settings.Get(ctx, user.ID)
	if err != nil {
		settings = &domain.UserSettings{UserID: user.ID, OutputMode: domain.OutputModeFull, KeepHistory: true}
		_ = h.settings.Upsert(ctx, settings)
	}
	mode := settings.OutputMode.OrDefault()
	markup := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "🎵 آهنگ کامل" + selected(mode == domain.OutputModeFull), CallbackData: "settings_mode_full"}},
		{{Text: "🎶 صدای اصلی" + selected(mode == domain.OutputModeOriginal), CallbackData: "settings_mode_original"}},
		{{Text: "📦 هر دو" + selected(mode == domain.OutputModeBoth), CallbackData: "settings_mode_both"}},
	}}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "⚙️ حالت خروجی پیش‌فرض را انتخاب کنید:", ReplyMarkup: markup})
}

func selected(value bool) string {
	if value {
		return " ✅"
	}
	return ""
}

func (h *BotHandler) handleSettingsMode(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil || h.users == nil || h.settings == nil {
		return
	}

	user := h.ensureUser(ctx, &update.CallbackQuery.From)
	if user == nil {
		return
	}
	mode, ok := domain.ParseOutputMode(strings.TrimPrefix(update.CallbackQuery.Data, "settings_mode_"))
	if !ok {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: update.CallbackQuery.ID, Text: "تنظیم نامعتبر است.", ShowAlert: true})
		return
	}
	settings, err := h.settings.Get(ctx, user.ID)
	if err != nil {
		settings = &domain.UserSettings{UserID: user.ID, KeepHistory: true}
	}
	settings.OutputMode = mode
	if err := h.settings.Upsert(ctx, settings); err != nil {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: update.CallbackQuery.ID, Text: "ذخیره تنظیمات انجام نشد.", ShowAlert: true})
		return
	}
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: update.CallbackQuery.ID, Text: "تنظیمات ذخیره شد ✅"})
}

func (h *BotHandler) handleFavoriteTrack(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil || h.users == nil || h.favorites == nil {
		return
	}
	trackID, err := strconv.ParseInt(strings.TrimPrefix(update.CallbackQuery.Data, "favorite_track_"), 10, 64)
	if err != nil {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: update.CallbackQuery.ID, Text: "آهنگ نامعتبر است.", ShowAlert: true})
		return
	}
	user := h.ensureUser(ctx, &update.CallbackQuery.From)
	if user == nil {
		return
	}
	exists, err := h.favorites.Exists(ctx, user.ID, trackID)
	if err != nil {
		return
	}
	if exists {
		err = h.favorites.Delete(ctx, user.ID, trackID)
	} else {
		err = h.favorites.Add(ctx, domain.Favorite{UserID: user.ID, TrackID: trackID})
	}
	if err != nil {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: update.CallbackQuery.ID, Text: "ذخیره انجام نشد.", ShowAlert: true})
		return
	}
	text := "به ذخیره‌ها اضافه شد ❤️"
	if exists {
		text = "از ذخیره‌ها حذف شد."
	}
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: update.CallbackQuery.ID, Text: text})
}

func (h *BotHandler) forgetUser(ctx context.Context, b *bot.Bot, chatID, telegramID int64) {
	if h.users == nil {
		h.sendMessage(ctx, b, chatID, "ℹ️ داده‌ای برای حذف وجود ندارد.")
		return
	}
	user, err := h.users.GetByTelegramID(ctx, telegramID)
	if err == nil {
		if err := h.users.Delete(ctx, user.ID); err != nil {
			h.sendMessage(ctx, b, chatID, "❌ حذف اطلاعات انجام نشد.")
			return
		}
	}
	h.sendMessage(ctx, b, chatID, "✅ اطلاعات شخصی و تاریخچه شما حذف شد.")
}

func (h *BotHandler) sendMessage(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text})
}

func requestLabel(request domain.Request) string {
	return RequestLabel(string(request.Status))
}

func (h *BotHandler) handleSendToChannel(ctx context.Context, b *bot.Bot, update *models.Update) {
	if h.channelID == "" {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "کانال تنظیم نشده است.",
			ShowAlert:       true,
		})
		return
	}

	msg := update.CallbackQuery.Message.Message
	if msg == nil {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "خطا در دریافت پیام اصلی ❌",
			ShowAlert:       true,
		})
		return
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
		ChatID:      h.channelID,
		FromChatID:  msg.Chat.ID,
		MessageID:   msg.ID,
		ReplyMarkup: replyMarkup,
	})
	if err != nil {
		slog.Error("failed to copy message to channel", "err", err)
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "خطا در ارسال به کانال ❌",
			ShowAlert:       true,
		})
		return
	}

	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		Text:            "با موفقیت به کانال ارسال شد ✅",
	})
}
