package telegram

import (
	"context"
	"testing"

	"telegram-audio-bot/internal/domain"

	"github.com/go-telegram/bot/models"
)

type stubUsers struct {
	byTG map[int64]*domain.User
}

func (s *stubUsers) Create(ctx context.Context, u *domain.User) error {
	if s.byTG == nil {
		s.byTG = map[int64]*domain.User{}
	}
	u.ID = int64(len(s.byTG) + 1)
	cp := *u
	s.byTG[u.TelegramID] = &cp
	return nil
}
func (s *stubUsers) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	return nil, errNotFound{}
}
func (s *stubUsers) GetByTelegramID(ctx context.Context, tid int64) (*domain.User, error) {
	if u, ok := s.byTG[tid]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, errNotFound{}
}
func (s *stubUsers) Update(ctx context.Context, u *domain.User) error { return nil }
func (s *stubUsers) Delete(ctx context.Context, id int64) error {
	for k, u := range s.byTG {
		if u.ID == id {
			delete(s.byTG, k)
			return nil
		}
	}
	return errNotFound{}
}

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }

type stubSettings struct {
	mode domain.OutputMode
}

func (s *stubSettings) Get(ctx context.Context, id int64) (*domain.UserSettings, error) {
	return &domain.UserSettings{UserID: id, OutputMode: s.mode, KeepHistory: true}, nil
}
func (s *stubSettings) Upsert(ctx context.Context, settings *domain.UserSettings) error {
	s.mode = settings.OutputMode
	return nil
}
func (s *stubSettings) Delete(ctx context.Context, id int64) error { return nil }

type stubFavorites struct {
	exists  bool
	added   bool
	deleted bool
}

func (s *stubFavorites) Add(ctx context.Context, v domain.Favorite) error {
	s.added = true
	s.exists = true
	return nil
}
func (s *stubFavorites) ListByUser(ctx context.Context, uid int64, p domain.Page) ([]domain.Favorite, error) {
	return nil, nil
}
func (s *stubFavorites) Exists(ctx context.Context, uid, tid int64) (bool, error) {
	return s.exists, nil
}
func (s *stubFavorites) Delete(ctx context.Context, uid, tid int64) error {
	s.deleted = true
	s.exists = false
	return nil
}
func (s *stubFavorites) ListEnrichedByUser(ctx context.Context, uid int64, p domain.Page) ([]domain.FavoriteTrack, error) {
	return nil, nil
}
func (s *stubFavorites) DeleteForUser(ctx context.Context, uid int64) error { return nil }
func (s *stubFavorites) DeleteAll(ctx context.Context, uid int64) error     { return nil }

func TestCommandsHandleRoutes(t *testing.T) {
	c := NewCommands(nil, nil, nil, nil, nil)
	for _, cmd := range []string{"/start", "/help", "/history", "/favorites", "/settings", "/forget"} {
		if !c.Handle(context.Background(), nil, 1, 2, cmd) {
			t.Fatalf("%s should be handled", cmd)
		}
	}
	if c.Handle(context.Background(), nil, 1, 2, "https://example.com") {
		t.Fatal("non-command should not be handled")
	}
}

func TestEnsureUserCreatesAndUpdates(t *testing.T) {
	users := &stubUsers{}
	created := ensureUser(context.Background(), users, &models.User{ID: 42, Username: "a"})
	if created == nil || created.TelegramID != 42 {
		t.Fatalf("create failed: %+v", created)
	}
	updated := ensureUser(context.Background(), users, &models.User{ID: 42, Username: "b", FirstName: "B"})
	if updated == nil || updated.Username != "b" {
		t.Fatalf("update failed: %+v", updated)
	}
}

func TestCallbacksSettingsAndFavorite(t *testing.T) {
	users := &stubUsers{byTG: map[int64]*domain.User{7: {ID: 1, TelegramID: 7}}}
	settings := &stubSettings{mode: domain.OutputModeFull}
	favorites := &stubFavorites{}
	cb := NewCallbacks(users, settings, favorites, "", func(ctx context.Context, user *models.User) *domain.User {
		return ensureUser(ctx, users, user)
	})
	router := NewCallbackRouter()
	cb.Register(router)
	ctx := context.Background()

	if err := router.Handle(ctx, nil, &models.Update{CallbackQuery: &models.CallbackQuery{
		ID: "1", From: models.User{ID: 7}, Data: "settings_mode_original",
	}}); err != nil {
		t.Fatal(err)
	}
	if settings.mode != domain.OutputModeOriginal {
		t.Fatalf("mode = %s", settings.mode)
	}

	if err := router.Handle(ctx, nil, &models.Update{CallbackQuery: &models.CallbackQuery{
		ID: "2", From: models.User{ID: 7}, Data: "favorite_track_9",
	}}); err != nil {
		t.Fatal(err)
	}
	if !favorites.added {
		t.Fatal("expected favorite add")
	}

	if err := router.Handle(ctx, nil, &models.Update{CallbackQuery: &models.CallbackQuery{
		ID: "3", From: models.User{ID: 7}, Data: "favorite_track_9",
	}}); err != nil {
		t.Fatal(err)
	}
	if !favorites.deleted {
		t.Fatal("expected favorite delete")
	}
}

func TestCallbacksChannelNotConfigured(t *testing.T) {
	cb := NewCallbacks(nil, nil, nil, "", nil)
	router := NewCallbackRouter()
	cb.Register(router)
	if err := router.Handle(context.Background(), nil, &models.Update{CallbackQuery: &models.CallbackQuery{
		ID: "c", Data: "send_to_channel",
	}}); err != nil {
		t.Fatal(err)
	}
}
