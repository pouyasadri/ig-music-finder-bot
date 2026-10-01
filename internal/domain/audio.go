package domain

import (
	"fmt"
	"strings"
)

// OutputMode controls which audio variant is delivered to the user.
type OutputMode string

const (
	OutputModeFull     OutputMode = "full"
	OutputModeOriginal OutputMode = "original"
	OutputModeBoth     OutputMode = "both"
)

func ParseOutputMode(raw string) (OutputMode, bool) {
	mode := OutputMode(strings.TrimSpace(strings.ToLower(raw)))
	switch mode {
	case OutputModeFull, OutputModeOriginal, OutputModeBoth:
		return mode, true
	case "":
		return OutputModeFull, true
	default:
		return "", false
	}
}

func (m OutputMode) OrDefault() OutputMode {
	if mode, ok := ParseOutputMode(string(m)); ok {
		return mode
	}
	return OutputModeFull
}

// RequestStatus is the lifecycle state of a persisted processing request.
type RequestStatus string

const (
	RequestStatusProcessing RequestStatus = "processing"
	RequestStatusCompleted  RequestStatus = "completed"
	RequestStatusFailed     RequestStatus = "failed"
)

// SourceKind identifies the inbound media source.
type SourceKind string

const (
	SourceKindInstagram  SourceKind = "instagram"
	SourceKindSoundCloud SourceKind = "soundcloud"
	SourceKindUpload     SourceKind = "upload"
)

// ProgressStage is a UI-agnostic processing stage for progress reporters.
type ProgressStage int

const (
	ProgressReceived ProgressStage = iota
	ProgressQueued
	ProgressExtracting
	ProgressRecognizing
	ProgressDownloading
	ProgressSending
)

// DurationPolicy decides whether a downloaded track is compatible with a recognized recording.
type DurationPolicy struct {
	MinFullTrackSeconds int
	RelativeTolerance   int // percent of expected duration
	MinToleranceSeconds int
	MaxOvershootFactor  int
}

// DefaultDurationPolicy is the shared policy used by recognition downloads and providers.
func DefaultDurationPolicy() DurationPolicy {
	return DurationPolicy{
		MinFullTrackSeconds: 60,
		RelativeTolerance:   10,
		MinToleranceSeconds: 10,
		MaxOvershootFactor:  3,
	}
}

func (p DurationPolicy) toleranceSeconds(expected int) int {
	tolerance := expected * p.RelativeTolerance / 100
	if tolerance < p.MinToleranceSeconds {
		return p.MinToleranceSeconds
	}
	return tolerance
}

// Matches reports whether actual duration is acceptable for expected duration.
func (p DurationPolicy) Matches(actual, expected int) bool {
	return p.Validate(actual, expected) == nil
}

// Validate returns a domain error when actual duration is incompatible with expected.
func (p DurationPolicy) Validate(actual, expected int) error {
	if actual <= 0 {
		return fmt.Errorf("%w: audio duration is unavailable", ErrDurationMismatch)
	}
	if expected <= 0 {
		if actual < p.MinFullTrackSeconds {
			return fmt.Errorf("%w: audio appears to be a preview: %d seconds", ErrDurationMismatch, actual)
		}
		return nil
	}

	// Providers may add intros/outros, but a result substantially shorter than
	// the recognized recording is almost always a preview or an incomplete file.
	tolerance := p.toleranceSeconds(expected)
	if actual+tolerance < expected {
		return fmt.Errorf("%w: audio is shorter than expected: actual=%ds expected=%ds", ErrDurationMismatch, actual, expected)
	}
	if actual > expected+tolerance*p.MaxOvershootFactor {
		return fmt.Errorf("%w: audio is longer than expected: actual=%ds expected=%ds", ErrDurationMismatch, actual, expected)
	}
	return nil
}

// NormalizeMediaURL strips trailing slashes and query strings for cache/lease keys.
func NormalizeMediaURL(raw string) string {
	normalized := strings.TrimSpace(raw)
	if index := strings.IndexByte(normalized, '?'); index >= 0 {
		normalized = normalized[:index]
	}
	normalized = strings.TrimRight(normalized, "/")
	return strings.ToLower(normalized)
}
