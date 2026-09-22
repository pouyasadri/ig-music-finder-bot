package telegram

import (
	"fmt"
	"strconv"
	"strings"

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
	switch status {
	case "completed":
		return "✅ تکمیل شد"
	case "failed":
		return "❌ ناموفق"
	case "processing":
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

	errStr := err.Error()
	switch {
	case contains(errStr, "extraction failed"):
		return "extraction", "❌ متأسفانه نتوانستم ویدیو را دانلود کنم. اگر این پیج پرایوت (Private) است، ربات قادر به دانلود آن نیست."
	case contains(errStr, "private"):
		return "private", "🔒 این محتوا خصوصی است. لطفاً فقط لینک‌های پابلیک بفرستید."
	case contains(errStr, "rate limit"):
		return "rate_limit", "⏳ تعداد درخواست‌های شما بیشتر از حد مجاز است. لطفاً کمی بعد دوباره تلاش کنید."
	case contains(errStr, "queue full"):
		return "queue_full", "🚦 ربات در حال حاضر شلوغ است. لطفاً کمی بعد دوباره تلاش کنید."
	case contains(errStr, "file too large"):
		return "too_large", "📦 حجم فایل بیش از حد مجاز است (حداکثر ۵۰ مگابایت)."
	case contains(errStr, "invalid"):
		return "invalid", "❌ لینک یا فایل قابل پردازش نیست."
	case contains(errStr, "timeout"):
		return "timeout", "⏱ درخواست برای دریافت پاسخ زمان‌برد. لطفاً دوباره تلاش کنید."
	default:
		return "internal", "⚠️ خطای موقت رخ داد. لطفاً دوباره تلاش کنید."
	}
}

// SourceTypeEmoji returns an emoji for request source type.
func SourceTypeFromURL(raw string) string {
	value := strings.ToLower(raw)
	switch {
	case strings.Contains(value, "instagram.com"), strings.Contains(value, "instagr.am"):
		return "instagram"
	case strings.Contains(value, "soundcloud.com"):
		return "soundcloud"
	default:
		return "upload"
	}
}

func SourceTypeEmoji(sourceType string) string {
	switch sourceType {
	case "instagram":
		return "📷"
	case "soundcloud":
		return "☁️"
	case "upload":
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
