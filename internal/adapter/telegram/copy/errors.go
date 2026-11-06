package copy

import (
	"context"
	"errors"
	"strings"

	"telegram-audio-bot/internal/domain"
)

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
		if strings.Contains(err.Error(), "private") {
			return "private", "🔒 این محتوا خصوصی است. لطفاً فقط لینک‌های پابلیک بفرستید."
		}
		return "internal", GenericError
	}
}
