package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"telegram-audio-bot/internal/adapter/telegram/copy"
	"telegram-audio-bot/internal/domain"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const defaultMaxUploadBytes = 49 * 1024 * 1024

// Delivery sends processed audio payloads back to Telegram users.
type Delivery struct {
	channelID      string
	maxUploadBytes int64
	tracks         domain.TrackRepository
	requests       domain.RequestRepository
	settings       domain.SettingsRepository
}

func NewDelivery(channelID string, tracks domain.TrackRepository, requests domain.RequestRepository, settings domain.SettingsRepository, maxUploadBytes int64) *Delivery {
	if maxUploadBytes <= 0 {
		maxUploadBytes = defaultMaxUploadBytes
	}
	return &Delivery{
		channelID:      channelID,
		maxUploadBytes: maxUploadBytes,
		tracks:         tracks,
		requests:       requests,
		settings:       settings,
	}
}

func (d *Delivery) ApplyOutputMode(ctx context.Context, request *domain.Request, payload *domain.AudioPayload) {
	if d == nil || request == nil || d.settings == nil || payload == nil {
		return
	}
	settings, err := d.settings.Get(ctx, request.UserID)
	if err != nil || settings.OutputMode.OrDefault() != domain.OutputModeOriginal || payload.OriginalPath == "" {
		return
	}
	payload.FilePath = payload.OriginalPath
	payload.Title = "Original Audio"
	payload.Performer = "Source"
	payload.IsFullTrack = false
	payload.ThumbnailPath = ""
	payload.Duration = 0
}

type deliverResult struct {
	TrackID      int64
	Filename     string
	UploadMs     int64
	HasThumbnail bool
}

func (d *Delivery) SendAudio(ctx context.Context, b *bot.Bot, chatID int64, statusMsg *models.Message, payload *domain.AudioPayload, sourceURL string, request *domain.Request) (*deliverResult, error) {
	fileInfo, err := os.Stat(payload.FilePath)
	if err != nil {
		return nil, err
	}
	if fileInfo.Size() > d.maxUploadBytes {
		return nil, domain.ErrFileTooLarge
	}

	f, err := os.Open(payload.FilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cleanFilename := audioFilename(payload)
	caption := copy.AudioCaption(payload)

	sendParams := &bot.SendAudioParams{
		ChatID:    chatID,
		Audio:     &models.InputFileUpload{Filename: cleanFilename, Data: f},
		Caption:   caption,
		Title:     payload.Title,
		Performer: payload.Performer,
		Duration:  payload.Duration,
	}

	var (
		tracks    domain.TrackRepository
		requests  domain.RequestRepository
		channelID string
	)
	if d != nil {
		tracks, requests, channelID = d.tracks, d.requests, d.channelID
	}

	trackID := int64(0)
	if tracks != nil && payload.IsFullTrack {
		track := &domain.Track{
			Title:      payload.Title,
			Artist:     payload.Performer,
			Duration:   payload.Duration,
			SpotifyURL: payload.SpotifyURL,
			YouTubeURL: payload.YouTubeURL,
		}
		if trackErr := tracks.Create(ctx, track); trackErr == nil {
			trackID = track.ID
		}
	}
	payload.TrackID = trackID

	if payload.ThumbnailPath != "" {
		if thumbFile, err := os.Open(payload.ThumbnailPath); err == nil {
			defer thumbFile.Close()
			sendParams.Thumbnail = &models.InputFileUpload{
				Filename: filepath.Base(payload.ThumbnailPath),
				Data:     thumbFile,
			}
		}
	}

	if kb := copy.DeliveryKeyboard(payload, sourceURL, channelID); kb != nil {
		sendParams.ReplyMarkup = kb
	}

	uploadStart := time.Now()
	_, err = b.SendAudio(ctx, sendParams)
	uploadMs := time.Since(uploadStart).Milliseconds()
	if err != nil {
		return nil, err
	}

	finishRequest(ctx, requests, request, domain.RequestStatusCompleted, "", trackID)
	if statusMsg != nil {
		_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: statusMsg.ID,
		})
	}

	return &deliverResult{
		TrackID:      trackID,
		Filename:     cleanFilename,
		UploadMs:     uploadMs,
		HasThumbnail: payload.ThumbnailPath != "",
	}, nil
}

func (d *Delivery) Fail(ctx context.Context, b *bot.Bot, chatID int64, statusMsg *models.Message, request *domain.Request, err error) {
	var requests domain.RequestRepository
	if d != nil {
		requests = d.requests
	}
	finishRequest(ctx, requests, request, domain.RequestStatusFailed, errorString(err), 0)
	sendErrorMessage(ctx, b, chatID, statusMsg, err)
}

func finishRequest(ctx context.Context, requests domain.RequestRepository, request *domain.Request, status domain.RequestStatus, requestError string, trackID int64) {
	if request == nil || requests == nil {
		return
	}
	request.Status, request.Error, request.TrackID = status, requestError, trackID
	now := time.Now()
	request.CompletedAt = &now
	if err := requests.Update(ctx, request); err != nil {
		slog.Warn("failed to update request history", "err", err)
	}
}

func sendErrorMessage(ctx context.Context, b *bot.Bot, chatID int64, statusMsg *models.Message, err error) {
	if statusMsg != nil {
		_, _ = b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: statusMsg.ID,
		})
	}
	_, msgText := copy.ErrorCategoryAndMessage(err)
	if msgText == "" {
		msgText = copy.GenericError
	}
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   msgText,
	})
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func audioFilename(payload *domain.AudioPayload) string {
	cleanTitle := sanitizeFilename.ReplaceAllString(payload.Title, "")
	cleanArtist := sanitizeFilename.ReplaceAllString(payload.Performer, "")
	if cleanTitle == "" {
		cleanTitle = "Audio"
	}
	if cleanArtist == "" || cleanArtist == "Instagram" {
		return fmt.Sprintf("%s.mp3", cleanTitle)
	}
	return fmt.Sprintf("%s - %s.mp3", cleanArtist, cleanTitle)
}
