package telegram

import (
	"context"

	"telegram-audio-bot/internal/domain"

	"github.com/go-telegram/bot/models"
)

func ensureUser(ctx context.Context, users domain.UserRepository, user *models.User) *domain.User {
	if user == nil || users == nil {
		return nil
	}
	stored, err := users.GetByTelegramID(ctx, user.ID)
	if err == nil {
		stored.Username, stored.FirstName, stored.LastName = user.Username, user.FirstName, user.LastName
		_ = users.Update(ctx, stored)
		return stored
	}
	stored = &domain.User{TelegramID: user.ID, Username: user.Username, FirstName: user.FirstName, LastName: user.LastName}
	if err := users.Create(ctx, stored); err != nil {
		return nil
	}
	return stored
}
