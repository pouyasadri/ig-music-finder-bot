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
	"telegram-audio-bot/internal/usecase"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var (
	igURLRegex         = regexp.MustCompile(`https?://(?:www\.)?(?:instagram\.com|instagr\.am)/(?:reel|p|share/reel)/[a-zA-Z0-9_\-\.]+/?(?:\?[^\s]*)?`)
	soundCloudURLRegex = regexp.MustCompile(`https?://(?:(?:www\.|m\.)?soundcloud\.com/[a-zA-Z0-9_\-]+/[a-zA-Z0-9_\-\.]+/?|on\.soundcloud\.com/[a-zA-Z0-9_\-]+/?|soundcloud\.app\.goo\.gl/[a-zA-Z0-9_\-]+/?)(?:\?[^\s]*)?`)
	sanitizeFilename   = regexp.MustCompile(`[<>:"/\\|?*]`)
)

type BotHandler struct {
	token             string
	reelUseCase       usecase.ReelAudioUseCase
	soundCloudUseCase usecase.SoundCloudAudioUseCase
	workerQueue       chan struct{}
	channelID         string
}

func NewBotHandler(token string, ruc usecase.ReelAudioUseCase, scuc usecase.SoundCloudAudioUseCase, maxWorkers int, channelID string) *BotHandler {
	return &BotHandler{
		token:             token,
		reelUseCase:       ruc,
		soundCloudUseCase: scuc,
		workerQueue:       make(chan struct{}, maxWorkers),
		channelID:         channelID,
	}
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

type audioJob struct {
	platform        string
	sourceURL       string
	initialAckText  string
	fallbackErrText string
	execute         func(ctx context.Context, workDir string, progressCb func(string)) (*domain.AudioPayload, error)
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
					{Text: "اشتراک‌گذاری ربات 🚀", URL: "https://t.me/share/url?url=&text=این+ربات+برای+دانلود+آهنگ+های+اینستاگرام+و+ساندکلاد+عالیه!+🎧"},
				},
			},
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text: "سلام! 👋 خوش اومدی.\n\n" +
				"کافیه لینک ریلز/پست اینستاگرام یا لینک آهنگ ساندکلاد (SoundCloud) رو برام بفرستی تا با کیفیت عالی برات دانلود کنم و تحویلت بدم 🎧",
			ReplyMarkup: kb,
		})
		return
	}

	// Validate Instagram URL format
	if match := igURLRegex.FindString(text); match != "" {
		h.handleInstagram(ctx, b, chatID, userID, match)
		return
	}

	// Validate SoundCloud URL format
	if match := soundCloudURLRegex.FindString(text); match != "" {
		h.handleSoundCloud(ctx, b, chatID, userID, match)
		return
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text: "❌ لطفاً یک لینک معتبر ریلز یا پست اینستاگرام، یا لینک آهنگ ساندکلاد بفرست.\n\n" +
			"مثال اینستاگرام:\nhttps://www.instagram.com/reel/C-xyz123/\n\n" +
			"مثال ساندکلاد:\nhttps://soundcloud.com/artist/track-name",
	})
}

func (h *BotHandler) handleInstagram(ctx context.Context, b *bot.Bot, chatID, userID int64, reelURL string) {
	slog.Info("processing new reel request",
		"chat_id", chatID,
		"user_id", userID,
		"url", reelURL,
	)

	h.processAudioJob(ctx, b, chatID, userID, audioJob{
		platform:        "instagram",
		sourceURL:       reelURL,
		initialAckText:  "📥 دریافت شد! در حال استخراج و بررسی صدای ریلز... ⏳",
		fallbackErrText: "متأسفانه نتونستم صدای این پست رو دریافت کنم. ممکنه پیج پرایوت باشه یا اینستاگرام موقتاً محدود کرده باشه 😕",
		execute: func(ctx context.Context, workDir string, progressCb func(string)) (*domain.AudioPayload, error) {
			return h.reelUseCase.Execute(ctx, workDir, reelURL, progressCb)
		},
	})
}

func (h *BotHandler) handleSoundCloud(ctx context.Context, b *bot.Bot, chatID, userID int64, soundCloudURL string) {
	slog.Info("processing new soundcloud request",
		"chat_id", chatID,
		"user_id", userID,
		"url", soundCloudURL,
	)

	h.processAudioJob(ctx, b, chatID, userID, audioJob{
		platform:        "soundcloud",
		sourceURL:       soundCloudURL,
		initialAckText:  "📥 دریافت شد! در حال دانلود از ساندکلاد... ⏳",
		fallbackErrText: "متأسفانه نتونستم این آهنگ رو از ساندکلاد دانلود کنم. لطفاً از صحت لینک اطمینان حاصل کن 😕",
		execute: func(ctx context.Context, workDir string, progressCb func(string)) (*domain.AudioPayload, error) {
			return h.soundCloudUseCase.Execute(ctx, workDir, soundCloudURL, progressCb)
		},
	})
}

func (h *BotHandler) processAudioJob(ctx context.Context, b *bot.Bot, chatID, userID int64, job audioJob) {
	statusMsg, _ := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   job.initialAckText,
	})

	go func() {
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
			h.sendErrorMessage(ctx, b, chatID, statusMsg, job.fallbackErrText, err)
			return
		}
		defer os.RemoveAll(workDir) // Strict cleanup

		// Start periodic chat action ticker (pulse upload_document every 4s)
		stopChatAction := make(chan struct{})
		go h.keepChatActionActive(ctx, b, chatID, stopChatAction)
		defer close(stopChatAction)

		progressCb := func(msg string) {
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    chatID,
				MessageID: statusMsg.ID,
				Text:      msg,
			})
		}

		// Execute business usecase
		payload, err := job.execute(ctx, workDir, progressCb)
		if err != nil {
			slog.Error("failed to process audio", "chat_id", chatID, "platform", job.platform, "err", err, "url", job.sourceURL)
			h.sendErrorMessage(ctx, b, chatID, statusMsg, job.fallbackErrText, err)
			return
		}

		// Inspect downloaded file
		fileInfo, err := os.Stat(payload.FilePath)
		if err != nil {
			slog.Error("output file stat failed", "err", err, "path", payload.FilePath)
			h.sendErrorMessage(ctx, b, chatID, statusMsg, job.fallbackErrText, err)
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
			h.sendErrorMessage(ctx, b, chatID, statusMsg, job.fallbackErrText, err)
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
		if cleanArtist == "" || cleanArtist == "Instagram" || cleanArtist == "SoundCloud" {
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

		// Attach interactive inline buttons (SoundCloud, YouTube, Instagram, Channel)
		inlineKeyboard := h.buildInlineKeyboard(payload, job.sourceURL, job.platform)
		if inlineKeyboard != nil {
			sendParams.ReplyMarkup = inlineKeyboard
		}

		uploadStart := time.Now()
		_, err = b.SendAudio(ctx, sendParams)
		uploadMs := time.Since(uploadStart).Milliseconds()
		if err != nil {
			slog.Error("failed to send audio file to user", "chat_id", chatID, "err", err, "upload_ms", uploadMs)
			h.sendErrorMessage(ctx, b, chatID, statusMsg, job.fallbackErrText, err)
			return
		}

		// Clean up initial status message after sending audio
		if statusMsg != nil {
			_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
				ChatID:    chatID,
				MessageID: statusMsg.ID,
			})
		}

		slog.Info("audio processed and delivered successfully",
			"chat_id", chatID,
			"platform", job.platform,
			"is_full_track", payload.IsFullTrack,
			"title", payload.Title,
			"clean_filename", cleanFilename,
			"has_thumbnail", payload.ThumbnailPath != "",
			"upload_ms", uploadMs,
			"total_ms", time.Since(startTime).Milliseconds(),
		)
	}()
}

func (h *BotHandler) buildInlineKeyboard(payload *domain.AudioPayload, sourceURL, platform string) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton
	var linkRow []models.InlineKeyboardButton

	if payload.SoundCloudURL != "" {
		linkRow = append(linkRow, models.InlineKeyboardButton{
			Text: "☁️ ساندکلاد (SoundCloud)",
			URL:  payload.SoundCloudURL,
		})
	} else if platform == "soundcloud" && sourceURL != "" {
		linkRow = append(linkRow, models.InlineKeyboardButton{
			Text: "☁️ ساندکلاد (SoundCloud)",
			URL:  sourceURL,
		})
	}

	if payload.YouTubeURL != "" {
		linkRow = append(linkRow, models.InlineKeyboardButton{
			Text: "📺 یوتیوب (YouTube)",
			URL:  payload.YouTubeURL,
		})
	}

	if payload.SpotifyURL != "" {
		linkRow = append(linkRow, models.InlineKeyboardButton{
			Text: "🟢 اسپاتیفای (Spotify)",
			URL:  payload.SpotifyURL,
		})
	}

	if platform == "instagram" && sourceURL != "" {
		linkRow = append(linkRow, models.InlineKeyboardButton{
			Text: "🔗 مشاهده پست",
			URL:  sourceURL,
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

func (h *BotHandler) sendErrorMessage(ctx context.Context, b *bot.Bot, chatID int64, statusMsg *models.Message, defaultMsg string, err error) {
	if statusMsg != nil {
		_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: statusMsg.ID,
		})
	}

	msgText := defaultMsg
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
