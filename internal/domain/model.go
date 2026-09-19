package domain

import "time"

type TrackMetadata struct {
	Title      string
	Artist     string
	IsMatched  bool
	Duration   int    // Duration in seconds
	SpotifyURL string // Spotify external link if available
	YouTubeURL string // YouTube external link if available
}

type AudioPayload struct {
	TrackID       int64
	OriginalPath  string
	Title         string
	Performer     string
	FilePath      string
	ThumbnailPath string // Path to extracted album art / thumbnail jpg/png
	Duration      int    // Duration in seconds
	IsFullTrack   bool
	SpotifyURL    string
	YouTubeURL    string
}

type User struct {
	ID         int64
	TelegramID int64
	Username   string
	FirstName  string
	LastName   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type UserSettings struct {
	UserID               int64
	Language             string
	NotificationsEnabled bool
	OutputMode           string
	KeepHistory          bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Request struct {
	ID          int64
	UserID      int64
	URL         string
	Status      string
	Error       string
	TrackID     int64
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// RequestHistory is kept as a descriptive alias for persistence integrations.
type RequestHistory = Request

type Track struct {
	ID         int64
	Title      string
	Artist     string
	Duration   int
	SpotifyURL string
	YouTubeURL string
	CreatedAt  time.Time
}

type Favorite struct {
	UserID    int64
	TrackID   int64
	CreatedAt time.Time
}

type PendingCallbackAction struct {
	ID        int64
	UserID    int64
	Action    string
	Payload   string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type Page struct {
	Limit  int
	Offset int
}
