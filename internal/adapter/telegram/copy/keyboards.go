package copy

import (
	"fmt"
	"strconv"
	"strings"

	"telegram-audio-bot/internal/domain"

	"github.com/go-telegram/bot/models"
)

func WelcomeKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{{Text: "اشتراک‌گذاری ربات 🚀", URL: ShareBotURL}},
		},
	}
}

func HomeKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🎧 ارسال لینک", CallbackData: "menu:process_link"},
				{Text: "📤 ارسال فایل", CallbackData: "menu:process_file"},
			},
			{
				{Text: "📚 تاریخچه", CallbackData: "menu:history:0"},
				{Text: "❤️ ذخیره‌ها", CallbackData: "menu:favorites:0"},
			},
			{
				{Text: "⚙️ تنظیمات", CallbackData: "menu:settings"},
				{Text: "❓ راهنما", CallbackData: "menu:help"},
			},
		},
	}
}

func SettingsModeKeyboard(mode domain.OutputMode) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "🎵 آهنگ کامل" + SelectedMark(mode == domain.OutputModeFull), CallbackData: "settings_mode_full"}},
		{{Text: "🎶 صدای اصلی" + SelectedMark(mode == domain.OutputModeOriginal), CallbackData: "settings_mode_original"}},
		{{Text: "📦 هر دو" + SelectedMark(mode == domain.OutputModeBoth), CallbackData: "settings_mode_both"}},
	}}
}

func DeliveryKeyboard(payload *domain.AudioPayload, sourceURL, channelID string) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton
	var linkRow []models.InlineKeyboardButton

	if payload.YouTubeURL != "" {
		linkRow = append(linkRow, models.InlineKeyboardButton{Text: "📺 یوتیوب (YouTube)", URL: payload.YouTubeURL})
	}
	if payload.SoundCloudURL != "" {
		linkRow = append(linkRow, models.InlineKeyboardButton{Text: "☁️ ساندکلاد", URL: payload.SoundCloudURL})
	}
	if sourceURL != "" && !strings.HasPrefix(sourceURL, "file://") && sourceURL != payload.SoundCloudURL {
		linkRow = append(linkRow, models.InlineKeyboardButton{Text: "🔗 مشاهده پست", URL: sourceURL})
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
	if channelID != "" {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         "📣 ارسال به کانال",
			CallbackData: "send_to_channel",
		}})
	}
	if len(rows) == 0 {
		return nil
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func PaginationKeyboard(prefix string, currentPage, totalPages int) *models.InlineKeyboardMarkup {
	var buttons []models.InlineKeyboardButton
	buttons = append(buttons, models.InlineKeyboardButton{Text: "🏠 خانه", CallbackData: "menu:home"})
	if currentPage > 0 {
		buttons = append(buttons, models.InlineKeyboardButton{
			Text:         "⬅️ قبلی",
			CallbackData: fmt.Sprintf("%s:%d", prefix, currentPage-1),
		})
	}
	if totalPages > 0 && currentPage < totalPages-1 {
		buttons = append(buttons, models.InlineKeyboardButton{
			Text:         "➡️ بعدی",
			CallbackData: fmt.Sprintf("%s:%d", prefix, currentPage+1),
		})
	}
	if totalPages > 0 {
		buttons = append(buttons, models.InlineKeyboardButton{
			Text:         fmt.Sprintf("صفحه %d/%d", currentPage+1, totalPages),
			CallbackData: "noop",
		})
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{buttons}}
}

func PrivacyConfirmationKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{{
			{Text: "✅ بله، حذف کن", CallbackData: "privacy:forget:confirm"},
			{Text: "❌ انصراف", CallbackData: "privacy:forget:cancel"},
		}},
	}
}
