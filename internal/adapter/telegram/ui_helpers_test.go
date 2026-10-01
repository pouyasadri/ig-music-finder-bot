package telegram

import (
	"fmt"
	"strings"
	"testing"

	"telegram-audio-bot/internal/domain"
)

func TestStateMessageAndFormatting(t *testing.T) {
	if got := StateMessage(StateRecognizing); got != "🎵 در حال شناسایی آهنگ" {
		t.Fatalf("unexpected state message: %s", got)
	}
	if got := FormatDuration(125); got != "02:05" {
		t.Fatalf("unexpected duration: %s", got)
	}
	if got := RequestLabel("completed"); got != "✅ تکمیل شد" {
		t.Fatalf("unexpected request label: %s", got)
	}
}

func TestErrorCategoryAndMessageDomainErrors(t *testing.T) {
	_, msg := ErrorCategoryAndMessage(domain.ErrRateLimited)
	if msg == "" {
		t.Fatal("expected rate limit message")
	}
	_, msg = ErrorCategoryAndMessage(fmt.Errorf("%w: detail", domain.ErrExtractionFailed))
	if !strings.Contains(msg, "دانلود") {
		t.Fatalf("unexpected extraction message: %s", msg)
	}
}

func TestCallbackParsing(t *testing.T) {
	value, ok := ParseCallbackData("menu:history:3", "menu:history:")
	if !ok || value != "3" {
		t.Fatalf("unexpected parsed callback: %q, %v", value, ok)
	}
	page, err := ExtractPageFromCallback("menu:history:3", "menu:history:")
	if err != nil || page != 3 {
		t.Fatalf("unexpected page: %d, %v", page, err)
	}
}
