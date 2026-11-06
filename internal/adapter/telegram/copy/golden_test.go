package copy_test

import (
	"testing"

	"telegram-audio-bot/internal/adapter/telegram/copy"
	"telegram-audio-bot/internal/domain"
)

func TestGoldenPersianConsts(t *testing.T) {
	cases := map[string]string{
		"Welcome":               copy.Welcome,
		"InvalidInput":          copy.InvalidInput,
		"AdmissionFailed":       copy.AdmissionFailed,
		"HistoryDisabled":       copy.HistoryDisabled,
		"HistoryLoadFailed":     copy.HistoryLoadFailed,
		"HistoryEmpty":          copy.HistoryEmpty,
		"HistoryHeader":         copy.HistoryHeader,
		"FavoritesDisabled":     copy.FavoritesDisabled,
		"FavoritesLoadFailed":   copy.FavoritesLoadFailed,
		"FavoritesEmpty":        copy.FavoritesEmpty,
		"FavoritesHeader":       copy.FavoritesHeader,
		"SettingsDisabled":      copy.SettingsDisabled,
		"SettingsLoadFailed":    copy.SettingsLoadFailed,
		"SettingsPrompt":        copy.SettingsPrompt,
		"ForgetNothing":         copy.ForgetNothing,
		"ForgetFailed":          copy.ForgetFailed,
		"ForgetSuccess":         copy.ForgetSuccess,
		"SettingsInvalid":       copy.SettingsInvalid,
		"SettingsSaveFailed":    copy.SettingsSaveFailed,
		"SettingsSaved":         copy.SettingsSaved,
		"FavoriteInvalid":       copy.FavoriteInvalid,
		"FavoriteSaveFailed":    copy.FavoriteSaveFailed,
		"FavoriteAdded":         copy.FavoriteAdded,
		"FavoriteRemoved":       copy.FavoriteRemoved,
		"ChannelNotConfigured":  copy.ChannelNotConfigured,
		"ChannelMessageMissing": copy.ChannelMessageMissing,
		"ChannelSendFailed":     copy.ChannelSendFailed,
		"ChannelSendSuccess":    copy.ChannelSendSuccess,
		"CallbackInvalid":       copy.CallbackInvalid,
		"WorkerQueued":          copy.WorkerQueued,
		"GenericError":          copy.GenericError,
		"OriginalAudioCaption":  copy.OriginalAudioCaption,
	}
	for name, value := range cases {
		if value == "" {
			t.Fatalf("%s is empty", name)
		}
	}

	if got := copy.StatusReceived(domain.SourceKindSoundCloud); got == "" {
		t.Fatal("StatusReceived soundcloud empty")
	}
	if got := copy.StatusReceived(domain.SourceKindUpload); got == "" {
		t.Fatal("StatusReceived upload empty")
	}
	if got := copy.StatusReceived(domain.SourceKindInstagram); got == "" {
		t.Fatal("StatusReceived instagram empty")
	}
	if copy.RequestLabel(string(domain.RequestStatusCompleted)) != "✅ تکمیل شد" {
		t.Fatalf("unexpected completed label")
	}
}
