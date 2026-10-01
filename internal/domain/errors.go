package domain

import "errors"

var (
	ErrExtractionFailed         = errors.New("failed to extract media from instagram")
	ErrDownloadFailed           = errors.New("failed to download full track from youtube")
	ErrSoundCloudDownloadFailed = errors.New("failed to download track from soundcloud")
	ErrDurationMismatch         = errors.New("downloaded track duration is incompatible")
	ErrRateLimited              = errors.New("rate limit exceeded")
	ErrQueueFull                = errors.New("request queue is full")
	ErrLeaseHeld                = errors.New("request lease already held")
	ErrFileTooLarge             = errors.New("file exceeds upload limit")
	ErrPrivateContent           = errors.New("content is private")
	ErrInvalidInput             = errors.New("invalid input")
	ErrRequestTimeout           = errors.New("request timed out")
)
