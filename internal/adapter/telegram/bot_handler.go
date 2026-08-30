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
	igURLRegex      = regexp.MustCompile(`https?://(?:www\.)?(?:instagram\.com|instagr\.am)/(?:reel|p|share/reel)/[a-zA-Z0-9_\-\.]+/?(?:\?[^\s]*)?`)
	sanitizeFilename = regexp.MustCompile(`[<>:"/\\|?*]`)
)

type BotHandler struct {
	token       string
	useCase     usecase.ReelAudioUseCase
	workerQueue chan struct{}
}

func NewBotHandler(token string, uc usecase.ReelAudioUseCase, maxWorkers int) *BotHandler {
	return &BotHandler{
		token:       token,
		useCase:     uc,
		workerQueue: make(chan struct{}, maxWorkers),
	}
}

func (h *BotHandler) Start(ctx context.Context) error {
	opts := []bot.Option{
		bot.WithDefaultHandler(h.handleMessage),
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
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text: "سلام! 👋 خوش اومدی.\n\n" +
				"فقط کافیه لینک ریلز یا پست اینستاگرام رو برام بفرستی تا آهنگش رو برات پیدا کنم و با کیفیت عالی تحویلت بدم 🎧",
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
		startTime := time.Now()

		// Acquire worker slot
		h.workerQueue <- struct{}{}
		defer func() { <-h.workerQueue }()

		workDir := fmt.Sprintf("/tmp/bot_req_%d", time.Now().UnixNano())
		if err := os.MkdirAll(workDir, 0755); err != nil {
			slog.Error("failed to create workdir", "err", err, "dir", workDir)
			h.sendErrorMessage(ctx, b, chatID, statusMsg)
			return
		}
		defer os.RemoveAll(workDir) // Strict cleanup

		// Start periodic chat action ticker (pulse upload_document every 4s)
		stopChatAction := make(chan struct{})
		go h.keepChatActionActive(ctx, b, chatID, stopChatAction)
		defer close(stopChatAction)

		// Execute business usecase
		payload, err := h.useCase.Execute(ctx, workDir, reelURL)
		if err != nil {
			slog.Error("failed to process reel audio", "chat_id", chatID, "err", err, "url", reelURL)
			h.sendErrorMessage(ctx, b, chatID, statusMsg)
			return
		}

		// Inspect downloaded file
		fileInfo, err := os.Stat(payload.FilePath)
		if err != nil {
			slog.Error("output file stat failed", "err", err, "path", payload.FilePath)
			h.sendErrorMessage(ctx, b, chatID, statusMsg)
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
			h.sendErrorMessage(ctx, b, chatID, statusMsg)
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
			caption = fmt.Sprintf("✨ نسخه کامل و باکیفیت استودیویی (320kbps)\n\n🎵 نام اثر: %s\n👤 خواننده: %s%s",
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

		_, err = b.SendAudio(ctx, sendParams)
		if err != nil {
			slog.Error("failed to send audio file to user", "chat_id", chatID, "err", err)
			h.sendErrorMessage(ctx, b, chatID, statusMsg)
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
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
	}()
}

func (h *BotHandler) buildInlineKeyboard(payload *domain.AudioPayload, reelURL string) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton

	var streamButtons []models.InlineKeyboardButton
	if payload.SpotifyURL != "" {
		streamButtons = append(streamButtons, models.InlineKeyboardButton{
			Text: "🎵 اسپاتیفای (Spotify)",
			URL:  payload.SpotifyURL,
		})
	}
	if payload.YouTubeURL != "" {
		streamButtons = append(streamButtons, models.InlineKeyboardButton{
			Text: "📺 یوتیوب (YouTube)",
			URL:  payload.YouTubeURL,
		})
	}

	if len(streamButtons) > 0 {
		rows = append(rows, streamButtons)
	}

	// Always add link back to the original Instagram post
	if reelURL != "" {
		rows = append(rows, []models.InlineKeyboardButton{
			{
				Text: "🔗 مشاهده پست اینستاگرام",
				URL:  reelURL,
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

func (h *BotHandler) sendErrorMessage(ctx context.Context, b *bot.Bot, chatID int64, statusMsg *models.Message) {
	if statusMsg != nil {
		_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: statusMsg.ID,
		})
	}

	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "متأسفانه نتونستم صدای این پست رو دریافت کنم. ممکنه پیج پرایوت باشه یا اینستاگرام موقتاً محدود کرده باشه 😕",
	})
}
