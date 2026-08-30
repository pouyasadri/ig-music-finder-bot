package domain

type TrackMetadata struct {
	Title       string
	Artist      string
	IsMatched   bool
	Duration    int    // Duration in seconds
	SpotifyURL  string // Spotify external link if available
	YouTubeURL  string // YouTube external link if available
}

type AudioPayload struct {
	Title         string
	Performer     string
	FilePath      string
	ThumbnailPath string // Path to extracted album art / thumbnail jpg/png
	Duration      int    // Duration in seconds
	IsFullTrack   bool
	SpotifyURL    string
	YouTubeURL    string
}
