package telegram

import (
	"context"

	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"

	"github.com/go-telegram/bot/models"
)

// Repos groups persistence dependencies for the Telegram adapter.
type Repos struct {
	Users     domain.UserRepository
	Settings  domain.SettingsRepository
	Requests  domain.RequestRepository
	Tracks    domain.TrackRepository
	Favorites domain.FavoriteRepository
}

// Infra groups admission/worker/delivery collaborators.
type Infra struct {
	Admission *Admission
	Workers   *WorkerPool
	Delivery  *Delivery
}

// Presentation groups command/callback UX collaborators.
type Presentation struct {
	Commands  *Commands
	Callbacks *Callbacks
	Router    *CallbackRouter
}

// Deps is the grouped dependency set owned by BotHandler.
type Deps struct {
	MediaUC        usecase.ProcessMediaUseCase
	Repos          Repos
	Infra          Infra
	Presentation   Presentation
	ChannelID      string
	MaxUploadBytes int64
}

func (h *BotHandler) rebuildDelivery() {
	h.deps.Infra.Delivery = NewDelivery(h.deps.ChannelID, h.deps.Repos.Tracks, h.deps.Repos.Requests, h.deps.Repos.Settings, h.deps.MaxUploadBytes)
}

func (h *BotHandler) rebuildPresentation() {
	ensure := func(ctx context.Context, user *models.User) *domain.User {
		return ensureUser(ctx, h.deps.Repos.Users, user)
	}
	h.deps.Presentation.Commands = NewCommands(h.deps.Repos.Users, h.deps.Repos.Settings, h.deps.Repos.Requests, h.deps.Repos.Favorites, ensure)
	h.deps.Presentation.Callbacks = NewCallbacks(h.deps.Repos.Users, h.deps.Repos.Settings, h.deps.Repos.Favorites, h.deps.ChannelID, ensure)
	h.deps.Presentation.Router = NewCallbackRouter()
	h.deps.Presentation.Callbacks.Register(h.deps.Presentation.Router)
}
