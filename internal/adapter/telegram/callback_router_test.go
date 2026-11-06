package telegram

import (
	"context"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func TestCallbackRouterLongestPrefix(t *testing.T) {
	router := NewCallbackRouter()
	var hit string
	router.Register("settings_", func(context.Context, *bot.Bot, *models.Update) error {
		hit = "short"
		return nil
	})
	router.Register("settings_mode_", func(context.Context, *bot.Bot, *models.Update) error {
		hit = "long"
		return nil
	})
	err := router.Handle(context.Background(), nil, &models.Update{
		CallbackQuery: &models.CallbackQuery{Data: "settings_mode_full"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if hit != "long" {
		t.Fatalf("expected longest prefix handler, got %q", hit)
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
