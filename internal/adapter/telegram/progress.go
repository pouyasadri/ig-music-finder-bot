package telegram

import (
	"fmt"

	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"
)

// FormatProgress maps a UI-agnostic progress stage to Persian Telegram copy.
func FormatProgress(stage domain.ProgressStage, detail usecase.ProgressDetail) string {
	switch stage {
	case domain.ProgressReceived:
		return "📥 درخواست دریافت شد"
	case domain.ProgressQueued:
		return "⏳ در صف پردازش"
	case domain.ProgressExtracting:
		return "🔍 در حال استخراج ویدیو از اینستاگرام..."
	case domain.ProgressRecognizing:
		return "🎵 در حال تشخیص آهنگ..."
	case domain.ProgressDownloading:
		if detail.Matched && detail.Title != "" {
			return fmt.Sprintf("✅ آهنگ پیدا شد: %s\n⬇️ در حال دانلود کیفیت بالا...", detail.Title)
		}
		return "⬇️ در حال آماده‌سازی نسخه کامل..."
	case domain.ProgressSending:
		if detail.Fallback {
			return "📤 آهنگ پیدا نشد. در حال ارسال صدای اصلی ویدیو..."
		}
		return "📤 در حال ارسال فایل صوتی..."
	default:
		return "⏳ در حال پردازش"
	}
}

type messageProgressReporter struct {
	update func(string)
}

func newMessageProgressReporter(update func(string)) usecase.ProgressReporter {
	if update == nil {
		return usecase.NopProgress()
	}
	return messageProgressReporter{update: update}
}

func (r messageProgressReporter) Report(stage domain.ProgressStage, detail usecase.ProgressDetail) {
	r.update(FormatProgress(stage, detail))
}
