package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"telegram-audio-bot/internal/domain"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

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

func downloadTelegramMedia(ctx context.Context, b *bot.Bot, fileID, uniqueID, workDir string) (string, error) {
	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: fileID})
	if err != nil {
		return "", fmt.Errorf("telegram file lookup failed: %w", err)
	}
	if file.FileSize > 49*1024*1024 {
		return "", domain.ErrFileTooLarge
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

func getTempBaseDir() string {
	if fi, err := os.Stat("/dev/shm"); err == nil && fi.IsDir() {
		return "/dev/shm"
	}
	return os.TempDir()
}

func keepChatActionActive(ctx context.Context, b *bot.Bot, chatID int64, stop <-chan struct{}) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

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
