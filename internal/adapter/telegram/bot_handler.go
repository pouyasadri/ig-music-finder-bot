package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/storage/sqlite"
	"telegram-audio-bot/internal/usecase"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var (
	igURLRegex       = regexp.MustCompile(`https?://(?:www\.)?(?:instagram\.com|instagr\.am)/(?:reel|p|share/reel)/[a-zA-Z0-9_\-\.]+/?(?:\?[^\s]*)?`)
	sanitizeFilename = regexp.MustCompile(`[<>:"/\\|?*]`)
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

func (h *BotHandler) Start(ctx context.Context) error {
	opts := []bot.Option{
		bot.WithDefaultHandler(h.handleMessage),
		bot.WithCallbackQueryDataHandler("send_to_channel", bot.MatchTypeExact, h.handleSendToChannel),
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
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	text := strings.TrimSpace(update.Message.Text)
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

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

	// Validate Instagram URL format
	match := igURLRegex.FindString(text)
	if match == "" {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text: "❌ لطفاً یک لینک معتبر ریلز یا پست اینستاگرام بفرست.\n\n" +
				"مثال:\nhttps://www.instagram.com/reel/C-xyz123/",
		})
		return
	}

	reelURL := match
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
			slog.Error("failed to process reel audio", "chat_id", chatID, "err", err, "url", reelURL)
			h.sendErrorMessage(ctx, b, chatID, statusMsg, err)
			return
		}

		// Inspect downloaded file
		fileInfo, err := os.Stat(payload.FilePath)
		if err != nil {
			slog.Error("output file stat failed", "err", err, "path", payload.FilePath)
			h.sendErrorMessage(ctx, b, chatID, statusMsg, err)
			return
		}

		// Safety check: Telegram standard bot upload limit is 50MB
		const maxUploadBytes = 49 * 1024 * 1024
		if fileInfo.Size() > maxUploadBytes {
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
			slog.Error("failed to send audio file to user", "chat_id", chatID, "err", err, "upload_ms", uploadMs)
			h.sendErrorMessage(ctx, b, chatID, statusMsg, err)
			return
		}

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
