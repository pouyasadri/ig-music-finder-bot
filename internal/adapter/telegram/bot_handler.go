package telegram

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/storage/sqlite"
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
	useCase        usecase.ReelAudioUseCase
	workerQueue    chan struct{}
	admissionQueue chan struct{}
	channelID      string
	rateLimiter    sqlite.RateLimiter
	leases         sqlite.Lease
	rateLimit      int
	rateWindow     time.Duration
	leaseTTL       time.Duration
	requestTTL     time.Duration
	users          domain.UserRepository
	settings       domain.SettingsRepository
	requests       domain.RequestRepository
	tracks         domain.TrackRepository
	favorites      domain.FavoriteRepository
}

func NewBotHandler(token string, uc usecase.ReelAudioUseCase, maxWorkers int, channelID string) *BotHandler {
	return &BotHandler{
		token:       token,
		useCase:     uc,
		workerQueue: make(chan struct{}, maxWorkers),
		channelID:   channelID,
	}
}

func NewProtectedBotHandler(token string, uc usecase.ReelAudioUseCase, maxWorkers, queueLimit int, channelID string, rateLimiter sqlite.RateLimiter, leases sqlite.Lease, rateLimit int, rateWindow, leaseTTL, requestTTL time.Duration) *BotHandler {
	h := NewBotHandler(token, uc, maxWorkers, channelID)
	if queueLimit < 0 {
		queueLimit = 0
	}
	h.admissionQueue = make(chan struct{}, maxWorkers+queueLimit)
	h.rateLimiter = rateLimiter
	h.leases = leases
	h.rateLimit = rateLimit
	h.rateWindow = rateWindow
	h.leaseTTL = leaseTTL
	h.requestTTL = requestTTL
	return h
}

func (h *BotHandler) WithPhase2Repositories(users domain.UserRepository, settings domain.SettingsRepository, requests domain.RequestRepository, tracks domain.TrackRepository, favorites domain.FavoriteRepository) *BotHandler {
	h.users, h.settings, h.requests, h.tracks, h.favorites = users, settings, requests, tracks, favorites
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

	// Handle /start or /help
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
				"فقط کافیه لینک ریلز یا پست اینستاگرام رو برام بفرستی تا آهنگش رو برات پیدا کنم و با کیفیت عالی تحویلت بدم 🎧",
			ReplyMarkup: kb,
		})
		return
	}

	match := igURLRegex.FindString(text)
	mediaID, mediaUniqueID := telegramMediaIDs(update.Message)
	if match == "" && mediaID == "" {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text: "❌ لطفاً یک لینک معتبر ریلز یا پست اینستاگرام بفرست.\n\n" +
				"مثال:\nhttps://www.instagram.com/reel/C-xyz123/",
		})
		return
	}

	reelURL := match
	if reelURL == "" {
		reelURL = "file://" + mediaUniqueID
	}
	normalizedURL := normalizeInstagramURL(reelURL)
	if h.rateLimiter != nil {
		allowed, err := h.rateLimiter.Allow(ctx, userID, h.rateLimit, h.rateWindow)
		if err != nil {
			slog.Error("rate limit check failed", "user_id", userID, "err", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "❌ خطایی در بررسی محدودیت درخواست رخ داد. لطفاً دوباره تلاش کنید."})
			return
		}
		if !allowed {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "⏳ تعداد درخواست‌های شما به حد مجاز رسیده است. لطفاً کمی بعد دوباره تلاش کنید."})
			return
		}
	}
	leaseOwner := fmt.Sprintf("%d-%d", userID, time.Now().UnixNano())
	var request *domain.Request
	if h.users != nil && h.requests != nil {
		storedUser := h.ensureUser(ctx, update.Message.From)
		if storedUser != nil {
			request = &domain.Request{UserID: storedUser.ID, URL: normalizedURL, Status: "processing"}
			if err := h.requests.Create(ctx, request); err != nil {
				slog.Warn("failed to persist request", "err", err)
				request = nil
			}
		}
	}
	if h.leases != nil {
		acquired, err := h.leases.Acquire(ctx, normalizedURL, leaseOwner, h.leaseTTL)
		if err != nil {
			slog.Error("request lease acquisition failed", "user_id", userID, "err", err)
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "❌ خطایی در ثبت درخواست رخ داد. لطفاً دوباره تلاش کنید."})
			return
		}
		if !acquired {
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "🔄 این لینک هم‌اکنون در حال پردازش است. لطفاً کمی صبر کنید."})
			return
		}
	}
	if h.admissionQueue != nil {
		select {
		case h.admissionQueue <- struct{}{}:
		default:
			if h.leases != nil {
				_ = h.leases.Release(context.Background(), normalizedURL, leaseOwner)
			}
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "⏳ ربات در حال حاضر شلوغ است. لطفاً کمی بعد دوباره تلاش کنید."})
			return
		}
	}
	slog.Info("processing new reel request",
		"chat_id", chatID,
		"user_id", userID,
		"url", reelURL,
	)

	// Send initial acknowledgment message in casual Farsi
	statusMsg, _ := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "📥 دریافت شد! در حال استخراج و بررسی صدای ریلز... ⏳",
	})

	go func() {
		if h.admissionQueue != nil {
			defer func() { <-h.admissionQueue }()
		}
		if h.leases != nil {
			defer func() {
				if err := h.leases.Release(context.Background(), normalizedURL, leaseOwner); err != nil {
					slog.Warn("request lease release failed", "err", err)
				}
			}()
		}
		workCtx := ctx
		cancel := func() {}
		if h.requestTTL > 0 {
			workCtx, cancel = context.WithTimeout(ctx, h.requestTTL)
		}
		defer cancel()
		startTime := time.Now()

		if len(h.workerQueue) == cap(h.workerQueue) {
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    chatID,
				MessageID: statusMsg.ID,
				Text:      "شما در صف انتظار هستید. ربات در حال حاضر شلوغ است، لطفاً کمی صبر کنید... ⏳",
			})
		}

		// Acquire worker slot
		h.workerQueue <- struct{}{}
		defer func() { <-h.workerQueue }()

		workDir := filepath.Join(getTempBaseDir(), fmt.Sprintf("bot_req_%d", time.Now().UnixNano()))
		if err := os.MkdirAll(workDir, 0755); err != nil {
			slog.Error("failed to create workdir", "err", err, "dir", workDir)
			h.sendErrorMessage(ctx, b, chatID, statusMsg, err)
			return
		}
		defer os.RemoveAll(workDir) // Strict cleanup
		if mediaID != "" {
			mediaPath, downloadErr := h.downloadTelegramMedia(ctx, b, mediaID, mediaUniqueID, workDir)
			if downloadErr != nil {
				h.finishRequest(ctx, request, "failed", downloadErr.Error(), 0)
				h.sendErrorMessage(ctx, b, chatID, statusMsg, downloadErr)
				return
			}
			reelURL = "file://" + mediaPath
		}

		// Start periodic chat action ticker (pulse upload_document every 4s)
		stopChatAction := make(chan struct{})
		go h.keepChatActionActive(workCtx, b, chatID, stopChatAction)
		defer close(stopChatAction)

		progressCb := func(msg string) {
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    chatID,
				MessageID: statusMsg.ID,
				Text:      msg,
			})
		}

		// Execute business usecase
		payload, err := h.useCase.Execute(workCtx, workDir, reelURL, progressCb)
		if err != nil {
			h.finishRequest(ctx, request, "failed", err.Error(), 0)
			slog.Error("failed to process reel audio", "chat_id", chatID, "err", err, "url", reelURL)
			h.sendErrorMessage(ctx, b, chatID, statusMsg, err)
			return
		}
		if request != nil && h.settings != nil {
			if settings, settingsErr := h.settings.Get(ctx, request.UserID); settingsErr == nil && settings.OutputMode == "original" && payload.OriginalPath != "" {
				payload.FilePath = payload.OriginalPath
				payload.Title = "Original Reel Audio"
				payload.Performer = "Instagram"
				payload.IsFullTrack = false
				payload.ThumbnailPath = ""
				payload.Duration = 0
			}
		}

		// Inspect downloaded file
		fileInfo, err := os.Stat(payload.FilePath)
		if err != nil {
			h.finishRequest(ctx, request, "failed", err.Error(), 0)
			slog.Error("output file stat failed", "err", err, "path", payload.FilePath)
			h.sendErrorMessage(ctx, b, chatID, statusMsg, err)
			return
		}

		// Safety check: Telegram standard bot upload limit is 50MB
		const maxUploadBytes = 49 * 1024 * 1024
		if fileInfo.Size() > maxUploadBytes {
			h.finishRequest(ctx, request, "failed", "file too large", 0)
			slog.Warn("audio file exceeds 50MB limit", "size_bytes", fileInfo.Size())
			_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "⚠️ حجم فایل صوتی بیشتر از حد مجاز تلگرام (۵۰ مگابایت) است.",
			})
			return
		}

		f, err := os.Open(payload.FilePath)
		if err != nil {
			slog.Error("failed to open output file", "err", err)
			h.sendErrorMessage(ctx, b, chatID, statusMsg, err)
			return
		}
		defer f.Close()

		// Generate clean and organized filename for saving (e.g. "Artist - Title.mp3")
		cleanTitle := sanitizeFilename.ReplaceAllString(payload.Title, "")
		cleanArtist := sanitizeFilename.ReplaceAllString(payload.Performer, "")
		if cleanTitle == "" {
			cleanTitle = "Audio"
		}
		cleanFilename := fmt.Sprintf("%s - %s.mp3", cleanArtist, cleanTitle)
		if cleanArtist == "" || cleanArtist == "Instagram" {
			cleanFilename = fmt.Sprintf("%s.mp3", cleanTitle)
		}

		// Build professional Persian caption
		var caption string
		if payload.IsFullTrack {
			durationStr := ""
			if payload.Duration > 0 {
				durationStr = fmt.Sprintf("\n⏱ مدت زمان: %02d:%02d", payload.Duration/60, payload.Duration%60)
			}
			caption = fmt.Sprintf("🎵 نام اثر: %s\n👤 خواننده: %s%s",
				payload.Title, payload.Performer, durationStr)
		} else {
			caption = "🎶 صدای اصلی خود ریلز اینستاگرام\n(نسخه استودیویی در پایگاه داده پیدا نشد)"
		}

		// Prepare send params
		sendParams := &bot.SendAudioParams{
			ChatID:    chatID,
			Audio:     &models.InputFileUpload{Filename: cleanFilename, Data: f},
			Caption:   caption,
			Title:     payload.Title,
			Performer: payload.Performer,
			Duration:  payload.Duration,
		}
		trackID := int64(0)
		if h.tracks != nil && payload.IsFullTrack {
			track := &domain.Track{Title: payload.Title, Artist: payload.Performer, Duration: payload.Duration, SpotifyURL: payload.SpotifyURL, YouTubeURL: payload.YouTubeURL}
			if trackErr := h.tracks.Create(ctx, track); trackErr == nil {
				trackID = track.ID
			}
		}
		payload.TrackID = trackID

		// Attach album cover art thumbnail if available
		if payload.ThumbnailPath != "" {
			if thumbFile, err := os.Open(payload.ThumbnailPath); err == nil {
				defer thumbFile.Close()
				sendParams.Thumbnail = &models.InputFileUpload{
					Filename: filepath.Base(payload.ThumbnailPath),
					Data:     thumbFile,
				}
			}
		}

		// Attach interactive inline buttons (Spotify, YouTube, Instagram Post)
		inlineKeyboard := h.buildInlineKeyboard(payload, reelURL)
		if inlineKeyboard != nil {
			sendParams.ReplyMarkup = inlineKeyboard
		}

		uploadStart := time.Now()
		_, err = b.SendAudio(ctx, sendParams)
		uploadMs := time.Since(uploadStart).Milliseconds()
		if err != nil {
			h.finishRequest(ctx, request, "failed", err.Error(), 0)
			slog.Error("failed to send audio file to user", "chat_id", chatID, "err", err, "upload_ms", uploadMs)
			h.sendErrorMessage(ctx, b, chatID, statusMsg, err)
			return
		}
		h.finishRequest(ctx, request, "completed", "", trackID)

		// Clean up initial status message after sending audio
		if statusMsg != nil {
			_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
				ChatID:    chatID,
				MessageID: statusMsg.ID,
			})
		}

		slog.Info("reel audio processed and delivered successfully",
			"chat_id", chatID,
			"is_full_track", payload.IsFullTrack,
			"title", payload.Title,
			"clean_filename", cleanFilename,
			"has_thumbnail", payload.ThumbnailPath != "",
			"upload_ms", uploadMs,
			"total_ms", time.Since(startTime).Milliseconds(),
		)
	}()
}

func telegramMediaIDs(message *models.Message) (string, string) {
	if message.Audio != nil {
		return message.Audio.FileID, message.Audio.FileUniqueID
	}
	if message.Document != nil {
		return message.Document.FileID, message.Document.FileUniqueID
	}
	if message.Video != nil {
		return message.Video.FileID, message.Video.FileUniqueID
	}
	if message.Voice != nil {
		return message.Voice.FileID, message.Voice.FileUniqueID
	}
	return "", ""
}

func (h *BotHandler) downloadTelegramMedia(ctx context.Context, b *bot.Bot, fileID, uniqueID, workDir string) (string, error) {
	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: fileID})
	if err != nil {
		return "", fmt.Errorf("telegram file lookup failed: %w", err)
	}
	if file.FileSize > 49*1024*1024 {
		return "", fmt.Errorf("telegram media exceeds upload limit")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.FileDownloadLink(file), nil)
	if err != nil {
		return "", fmt.Errorf("telegram media request failed: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("telegram media download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("telegram media download returned status %s", resp.Status)
	}
	path := filepath.Join(workDir, "telegram_"+sanitizeFilename.ReplaceAllString(uniqueID, "")+".media")
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}
	return path, nil
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

func (h *BotHandler) finishRequest(ctx context.Context, request *domain.Request, status, requestError string, trackID int64) {
	if request == nil || h.requests == nil {
		return
	}
	request.Status, request.Error, request.TrackID = status, requestError, trackID
	now := time.Now()
	request.CompletedAt = &now
	if err := h.requests.Update(ctx, request); err != nil {
		slog.Warn("failed to update request history", "err", err)
	}
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
		settings = &domain.UserSettings{UserID: user.ID, OutputMode: "full", KeepHistory: true}
		_ = h.settings.Upsert(ctx, settings)
	}
	mode := settings.OutputMode
	if mode == "" {
		mode = "full"
	}
	markup := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "🎵 آهنگ کامل" + selected(mode == "full"), CallbackData: "settings_mode_full"}},
		{{Text: "🎶 صدای اصلی" + selected(mode == "original"), CallbackData: "settings_mode_original"}},
		{{Text: "📦 هر دو" + selected(mode == "both"), CallbackData: "settings_mode_both"}},
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
	mode := strings.TrimPrefix(update.CallbackQuery.Data, "settings_mode_")
	if mode != "full" && mode != "original" && mode != "both" {
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
	switch request.Status {
	case "completed":
		return "✅ تکمیل شد"
	case "failed":
		return "❌ ناموفق"
	default:
		return "⏳ در حال پردازش"
	}
}

func normalizeInstagramURL(raw string) string {
	normalized := strings.TrimRight(strings.TrimSpace(raw), "/")
	if index := strings.IndexByte(normalized, '?'); index >= 0 {
		normalized = normalized[:index]
	}
	return strings.ToLower(normalized)
}

func (h *BotHandler) buildInlineKeyboard(payload *domain.AudioPayload, reelURL string) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton

	var linkRow []models.InlineKeyboardButton

	if payload.YouTubeURL != "" {
		linkRow = append(linkRow, models.InlineKeyboardButton{
			Text: "📺 یوتیوب (YouTube)",
			URL:  payload.YouTubeURL,
		})
	}

	if reelURL != "" {
		linkRow = append(linkRow, models.InlineKeyboardButton{
			Text: "🔗 مشاهده پست",
			URL:  reelURL,
		})
	}

	if len(linkRow) > 0 {
		rows = append(rows, linkRow)
	}
	if payload.TrackID > 0 {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         "❤️ ذخیره / حذف از ذخیره‌ها",
			CallbackData: "favorite_track_" + strconv.FormatInt(payload.TrackID, 10),
		}})
	}

	if h.channelID != "" {
		rows = append(rows, []models.InlineKeyboardButton{
			{
				Text:         "📣 ارسال به کانال",
				CallbackData: "send_to_channel",
			},
		})
	}

	if len(rows) == 0 {
		return nil
	}

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
}

func (h *BotHandler) keepChatActionActive(ctx context.Context, b *bot.Bot, chatID int64, stop <-chan struct{}) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	// Pulse immediately
	_, _ = b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID: chatID,
		Action: models.ChatActionUploadDocument,
	})

	for {
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = b.SendChatAction(ctx, &bot.SendChatActionParams{
				ChatID: chatID,
				Action: models.ChatActionUploadDocument,
			})
		}
	}
}

func (h *BotHandler) sendErrorMessage(ctx context.Context, b *bot.Bot, chatID int64, statusMsg *models.Message, err error) {
	if statusMsg != nil {
		_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: statusMsg.ID,
		})
	}

	msgText := "متأسفانه نتونستم صدای این پست رو دریافت کنم. ممکنه پیج پرایوت باشه یا اینستاگرام موقتاً محدود کرده باشه 😕"
	if err != nil && strings.Contains(err.Error(), "extraction failed") {
		msgText = "❌ متأسفانه نتوانستم ویدیو را دانلود کنم. اگر این پیج پرایوت (Private) است، ربات قادر به دانلود آن نیست. لطفاً فقط لینک‌های پابلیک بفرستید."
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   msgText,
	})
}

func getTempBaseDir() string {
	if fi, err := os.Stat("/dev/shm"); err == nil && fi.IsDir() {
		return "/dev/shm"
	}
	return os.TempDir()
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

	// Reconstruct the keyboard without the "send_to_channel" button for the channel
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
		replyMarkup = &models.InlineKeyboardMarkup{
			InlineKeyboard: newRows,
		}
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
