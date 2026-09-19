package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"telegram-audio-bot/internal/domain"
)

func TestPersistenceOwnershipAndPagination(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	users := NewUserRepository(db)
	one, two := &domain.User{TelegramID: 101}, &domain.User{TelegramID: 202}
	if err := users.Create(ctx, one); err != nil {
		t.Fatal(err)
	}
	if err := users.Create(ctx, two); err != nil {
		t.Fatal(err)
	}
	reqs := NewRequestRepository(db)
	for i := 0; i < 3; i++ {
		if err := reqs.Create(ctx, &domain.Request{UserID: one.ID, URL: "https://example/" + string(rune('a'+i)), Status: "done"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := reqs.GetByIDForUser(ctx, 1, two.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user request lookup error = %v", err)
	}
	got, err := reqs.ListByUser(ctx, one.ID, domain.Page{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d requests, want 2", len(got))
	}
}

func TestPendingCallbackExpiryAndFavoriteDelete(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	users := NewUserRepository(db)
	u := &domain.User{TelegramID: 303}
	if err := users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	tracks := NewTrackRepository(db)
	tr := &domain.Track{Title: "song"}
	if err := tracks.Create(ctx, tr); err != nil {
		t.Fatal(err)
	}
	favorites := NewFavoriteRepository(db)
	if err := favorites.Add(ctx, domain.Favorite{UserID: u.ID, TrackID: tr.ID}); err != nil {
		t.Fatal(err)
	}
	ok, err := favorites.Exists(ctx, u.ID, tr.ID)
	if err != nil || !ok {
		t.Fatalf("favorite Exists() = %v, %v", ok, err)
	}
	if err := favorites.Delete(ctx, u.ID, tr.ID); err != nil {
		t.Fatal(err)
	}
	pending := NewPendingCallbackRepository(db)
	action := &domain.PendingCallbackAction{UserID: u.ID, Action: "send", ExpiresAt: time.Now().Add(-time.Minute)}
	if err := pending.Create(ctx, action); err != nil {
		t.Fatal(err)
	}
	if err := pending.DeleteExpired(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pending.GetByIDForUser(ctx, action.ID, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired callback lookup error = %v", err)
	}
}
