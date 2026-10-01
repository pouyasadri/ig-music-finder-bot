package domain

import (
	"context"
	"time"
)

// --- Persistence ports ---

type UserRepository interface {
	Create(context.Context, *User) error
	GetByID(context.Context, int64) (*User, error)
	GetByTelegramID(context.Context, int64) (*User, error)
	Update(context.Context, *User) error
	Delete(context.Context, int64) error
}

type SettingsRepository interface {
	Get(context.Context, int64) (*UserSettings, error)
	Upsert(context.Context, *UserSettings) error
	Delete(context.Context, int64) error
}

type RequestRepository interface {
	Create(context.Context, *Request) error
	Update(context.Context, *Request) error
	GetByID(context.Context, int64) (*Request, error)
	GetByIDForUser(context.Context, int64, int64) (*Request, error)
	ListByUser(context.Context, int64, Page) ([]Request, error)
	Delete(context.Context, int64) error
	DeleteForUser(context.Context, int64, int64) error
}

type TrackRepository interface {
	Create(context.Context, *Track) error
	GetByID(context.Context, int64) (*Track, error)
	Delete(context.Context, int64) error
}

type FavoriteRepository interface {
	Add(context.Context, Favorite) error
	ListByUser(context.Context, int64, Page) ([]Favorite, error)
	ListEnrichedByUser(context.Context, int64, Page) ([]FavoriteTrack, error)
	Exists(context.Context, int64, int64) (bool, error)
	Delete(context.Context, int64, int64) error
	DeleteForUser(context.Context, int64) error
	DeleteAll(context.Context, int64) error
}

type FavoriteTrack struct {
	Favorite Favorite
	Track    Track
}

// --- Operational ports ---

type RateLimiter interface {
	Allow(ctx context.Context, userID int64, limit int, window time.Duration) (bool, error)
	Reset(ctx context.Context, userID int64) error
}

type Lease interface {
	Acquire(ctx context.Context, key, owner string, ttl time.Duration) (bool, error)
	Renew(ctx context.Context, key, owner string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key, owner string) error
}

// --- Application / media pipeline ports ---

type MediaExtractor interface {
	ExtractReel(ctx context.Context, targetDir, url string) (rawPath, snippetPath string, err error)
}

type MusicRecognizer interface {
	Identify(ctx context.Context, snippetPath string) (*TrackMetadata, error)
}

type URLMusicRecognizer interface {
	IdentifyByURL(ctx context.Context, url string) (*TrackMetadata, error)
}

type MusicDownloader interface {
	Download(ctx context.Context, targetDir, query string, expectedDuration int) (filePath, thumbnailPath string, duration int, err error)
}

type MetadataCache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, expiresAt time.Time) error
}

type SoundCloudTrackDownloader interface {
	DownloadTrack(ctx context.Context, targetDir, url string) (*AudioPayload, error)
}
