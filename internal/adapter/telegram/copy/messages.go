package copy

import (
	"fmt"

	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"
)

const (
	Welcome = "سلام! 👋 خوش اومدی.\n\n" +
		"لینک ریلز/پست اینستاگرام یا ترک ساندکلاد رو بفرست تا آهنگش رو برات پیدا کنم 🎧\n" +
		"همچنین می‌تونی فایل صوتی یا ویدیو هم آپلود کنی.\n\n" +
		"دستورات: /history /favorites /settings /forget"

	InvalidInput = "❌ لطفاً یک لینک معتبر اینستاگرام یا ساندکلاد بفرست، یا فایل صوتی/ویدیو آپلود کن.\n\n" +
		"مثال اینستاگرام:\nhttps://www.instagram.com/reel/C-xyz123/\n\n" +
		"مثال ساندکلاد:\nhttps://soundcloud.com/artist/track"

	AdmissionFailed       = "❌ خطایی در ثبت درخواست رخ داد. لطفاً دوباره تلاش کنید."
	HistoryDisabled       = "📚 تاریخچه هنوز فعال نشده است."
	HistoryLoadFailed     = "❌ خطا در بارگذاری تاریخچه."
	HistoryEmpty          = "📚 تاریخچه‌ای برای نمایش وجود ندارد."
	HistoryHeader         = "📚 تاریخچه درخواست‌ها:\n\n"
	FavoritesDisabled     = "❤️ ذخیره‌ها هنوز فعال نشده است."
	FavoritesLoadFailed   = "❌ خطا در بارگذاری ذخیره‌ها."
	FavoritesEmpty        = "❤️ آهنگ ذخیره‌شده‌ای وجود ندارد."
	FavoritesHeader       = "❤️ آهنگ‌های ذخیره‌شده:\n\n"
	SettingsDisabled      = "⚙️ تنظیمات هنوز فعال نشده است."
	SettingsLoadFailed    = "❌ خطا در بارگذاری تنظیمات."
	SettingsPrompt        = "⚙️ حالت خروجی پیش‌فرض را انتخاب کنید:"
	ForgetNothing         = "ℹ️ داده‌ای برای حذف وجود ندارد."
	ForgetFailed          = "❌ حذف اطلاعات انجام نشد."
	ForgetSuccess         = "✅ اطلاعات شخصی و تاریخچه شما حذف شد."
	SettingsInvalid       = "تنظیم نامعتبر است."
	SettingsSaveFailed    = "ذخیره تنظیمات انجام نشد."
	SettingsSaved         = "تنظیمات ذخیره شد ✅"
	FavoriteInvalid       = "آهنگ نامعتبر است."
	FavoriteSaveFailed    = "ذخیره انجام نشد."
	FavoriteAdded         = "به ذخیره‌ها اضافه شد ❤️"
	FavoriteRemoved       = "از ذخیره‌ها حذف شد."
	ChannelNotConfigured  = "کانال تنظیم نشده است."
	ChannelMessageMissing = "خطا در دریافت پیام اصلی ❌"
	ChannelSendFailed     = "خطا در ارسال به کانال ❌"
	ChannelSendSuccess    = "با موفقیت به کانال ارسال شد ✅"
	CallbackInvalid       = "درخواست نامعتبر است."
	WorkerQueued          = "شما در صف انتظار هستید. ربات در حال حاضر شلوغ است، لطفاً کمی صبر کنید... ⏳"
	GenericError          = "⚠️ خطای موقت رخ داد. لطفاً دوباره تلاش کنید."
	OriginalAudioCaption  = "🎶 صدای اصلی خود ریلز اینستاگرام\n(نسخه استودیویی در پایگاه داده پیدا نشد)"
	ShareBotURL           = "https://t.me/share/url?url=&text=این+ربات+برای+دانلود+آهنگ+های+اینستاگرام+عالیه!+🎧"
)

func StatusReceived(kind domain.SourceKind) string {
	switch kind {
	case domain.SourceKindSoundCloud:
		return "📥 دریافت شد! در حال دانلود از ساندکلاد... ⏳"
	case domain.SourceKindUpload:
		return "📥 فایل دریافت شد! در حال پردازش صدا... ⏳"
	default:
		return "📥 دریافت شد! در حال استخراج و بررسی صدای ریلز... ⏳"
	}
}

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

func FormatDuration(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

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

func AudioCaption(payload *domain.AudioPayload) string {
	if payload.IsFullTrack {
		durationStr := ""
		if payload.Duration > 0 {
			durationStr = fmt.Sprintf("\n⏱ مدت زمان: %s", FormatDuration(payload.Duration))
		}
		return fmt.Sprintf("🎵 نام اثر: %s\n👤 خواننده: %s%s", payload.Title, payload.Performer, durationStr)
	}
	return OriginalAudioCaption
}

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

func HistoryLine(index int, label, url string) string {
	return fmt.Sprintf("%d. %s — %s\n", index, label, url)
}

func FavoriteLine(index int, trackID int64) string {
	return fmt.Sprintf("%d. track #%d\n", index, trackID)
}

func SelectedMark(selected bool) string {
	if selected {
		return " ✅"
	}
	return ""
}
