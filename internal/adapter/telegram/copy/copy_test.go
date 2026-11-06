package copy_test

import (
	"fmt"
	"strings"
	"testing"

	"telegram-audio-bot/internal/adapter/telegram/copy"
	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"
)

func TestRequestLabelAndDuration(t *testing.T) {
	if got := copy.RequestLabel("completed"); got != "✅ تکمیل شد" {
		t.Fatalf("unexpected label: %s", got)
	}
	if got := copy.FormatDuration(125); got != "02:05" {
		t.Fatalf("unexpected duration: %s", got)
	}
}

func TestFormatProgressMatchedDownload(t *testing.T) {
	got := copy.FormatProgress(domain.ProgressDownloading, usecase.ProgressDetail{Title: "Song", Matched: true})
	if !strings.Contains(got, "Song") {
		t.Fatalf("unexpected progress copy: %s", got)
	}
}

func TestErrorCategoryAndMessageDomainErrors(t *testing.T) {
	_, msg := copy.ErrorCategoryAndMessage(domain.ErrRateLimited)
	if msg == "" {
		t.Fatal("expected rate limit message")
	}
	_, msg = copy.ErrorCategoryAndMessage(fmt.Errorf("%w: detail", domain.ErrExtractionFailed))
	if !strings.Contains(msg, "دانلود") {
		t.Fatalf("unexpected extraction message: %s", msg)
	}
}

func TestSettingsKeyboardModes(t *testing.T) {
	kb := copy.SettingsModeKeyboard(domain.OutputModeFull)
	if kb == nil || len(kb.InlineKeyboard) != 3 {
		t.Fatalf("unexpected settings keyboard: %+v", kb)
	}
}
