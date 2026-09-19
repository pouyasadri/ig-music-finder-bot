package domain

import "errors"

var (
	ErrExtractionFailed         = errors.New("failed to extract media from instagram")
	ErrDownloadFailed           = errors.New("failed to download full track from youtube")
	ErrSoundCloudDownloadFailed = errors.New("failed to download track from soundcloud")
)
