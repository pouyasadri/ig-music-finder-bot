package telegram

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"telegram-audio-bot/internal/domain"

	"github.com/go-telegram/bot/models"
)

// ProcessingState represents the current processing stage for user feedback.
type ProcessingState int

const (
	StateReceived ProcessingState = iota
	StateQueued
	StateExtracting
	StateRecognizing
	StateDownloading
	StateSending
)

// StateMessage returns a user-friendly Persian message for each processing state.
func StateMessage(state ProcessingState) string {
	switch state {
	case StateReceived:
		return "📥 درخواست دریافت شد"
	case StateQueued:
		return "⏳ در صف پردازش"
	case StateExtracting:
		return "🔍 در حال استخراج صدا"
	case StateRecognizing:
		return "🎵 در حال شناسایی آهنگ"
	case StateDownloading:
		return "⬇️ در حال آماده‌سازی نسخه کامل"
	case StateSending:
		return "📤 در حال ارسال فایل"
	default:
		return "⏳ در حال پردازش"
	}
}

// RequestLabel returns a Persian status label for a request.
func RequestLabel(status string) string {
	switch domain.RequestStatus(status) {
	case domain.RequestStatusCompleted:
		return "✅ تکمیل شد"
	case domain.RequestStatusFailed:
		return "❌ ناموفق"
	case domain.RequestStatusProcessing:
		return "⏳ در حال پردازش"
	default:
		return "⏳ نامعلوم"
	}
}

// ErrorCategoryAndMessage maps technical errors to user-facing categories and Persian copy.
func ErrorCategoryAndMessage(err error) (category, message string) {
	if err == nil {
		return "none", ""
	}

	switch {
	case errors.Is(err, domain.ErrExtractionFailed), errors.Is(err, domain.ErrPrivateContent):
		if errors.Is(err, domain.ErrPrivateContent) {
			return "private", "🔒 این محتوا خصوصی است. لطفاً فقط لینک‌های پابلیک بفرستید."
		}
		return "extraction", "❌ متأسفانه نتوانستم ویدیو را دانلود کنم. اگر این پیج پرایوت (Private) است، ربات قادر به دانلود آن نیست."
	case errors.Is(err, domain.ErrSoundCloudDownloadFailed):
		return "soundcloud", "❌ دانلود از ساندکلاد ناموفق بود. لینک ترک را بررسی کنید و دوباره تلاش کنید."
	case errors.Is(err, domain.ErrRateLimited):
		return "rate_limit", "⏳ تعداد درخواست‌های شما بیشتر از حد مجاز است. لطفاً کمی بعد دوباره تلاش کنید."
	case errors.Is(err, domain.ErrQueueFull):
		return "queue_full", "🚦 ربات در حال حاضر شلوغ است. لطفاً کمی بعد دوباره تلاش کنید."
	case errors.Is(err, domain.ErrLeaseHeld):
		return "lease_held", "🔄 این لینک هم‌اکنون در حال پردازش است. لطفاً کمی صبر کنید."
	case errors.Is(err, domain.ErrFileTooLarge):
		return "too_large", "📦 حجم فایل بیش از حد مجاز است (حداکثر ۵۰ مگابایت)."
	case errors.Is(err, domain.ErrInvalidInput):
		return "invalid", "❌ لینک یا فایل قابل پردازش نیست."
	case errors.Is(err, domain.ErrRequestTimeout), errors.Is(err, context.DeadlineExceeded):
		return "timeout", "⏱ درخواست برای دریافت پاسخ زمان‌برد. لطفاً دوباره تلاش کنید."
	case errors.Is(err, domain.ErrDownloadFailed), errors.Is(err, domain.ErrDurationMismatch):
		return "download", "⚠️ دریافت نسخه کامل ناموفق بود. در صورت امکان صدای اصلی ارسال می‌شود."
	default:
		errStr := err.Error()
		if contains(errStr, "private") {
			return "private", "🔒 این محتوا خصوصی است. لطفاً فقط لینک‌های پابلیک بفرستید."
		}
		return "internal", "⚠️ خطای موقت رخ داد. لطفاً دوباره تلاش کنید."
	}
}

// SourceTypeFromURL returns a stable source kind for logging and labels.
func SourceTypeFromURL(raw string) string {
	value := strings.ToLower(raw)
	switch {
	case strings.Contains(value, "instagram.com"), strings.Contains(value, "instagr.am"):
		return string(domain.SourceKindInstagram)
	case strings.Contains(value, "soundcloud.com"), strings.Contains(value, "on.soundcloud.com"):
		return string(domain.SourceKindSoundCloud)
	default:
		return string(domain.SourceKindUpload)
	}
}

func SourceTypeEmoji(sourceType string) string {
	switch domain.SourceKind(sourceType) {
	case domain.SourceKindInstagram:
		return "📷"
	case domain.SourceKindSoundCloud:
		return "☁️"
	case domain.SourceKindUpload:
		return "📤"
	default:
		return "📎"
	}
}

// FormatDuration returns a Persian formatted duration string.
func FormatDuration(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	mins := seconds / 60
	secs := seconds % 60
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

// FormatTrackInfo formats track metadata for display.
func FormatTrackInfo(title, artist string, duration int) string {
	info := fmt.Sprintf("🎵 %s", title)
	if artist != "" && artist != "Unknown Artist" {
		info += fmt.Sprintf("\n👤 %s", artist)
	}
	if duration > 0 {
		info += fmt.Sprintf("\n⏱ %s", FormatDuration(duration))
	}
	return info
}

// HomeKeyboard returns the primary home menu keyboard.
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

// PaginationKeyboard returns navigation buttons for paginated lists.
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

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{buttons},
	}
}

// ParseCallbackData safely parses callback data with a prefix and returns the suffix.
func ParseCallbackData(data, prefix string) (string, bool) {
	if len(data) <= len(prefix) {
		return "", false
	}
	if data[:len(prefix)] != prefix {
		return "", false
	}
	return data[len(prefix):], true
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && strconv.IntSize >= 0 && (len(s) >= len(substr) &&
		(s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && findInString(s, substr))))
}

func findInString(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
