package domain

type TrackMetadata struct {
	Title     string
	Artist    string
	IsMatched bool
}

type AudioPayload struct {
	Title       string
	Performer   string
	FilePath    string
	IsFullTrack bool
}
