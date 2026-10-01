package recognizer

import (
	"telegram-audio-bot/internal/domain"
	"time"
)

// Engine defines a concrete recognizer implementation together with its timeout budget.
type Engine struct {
	Name       string
	Recognizer domain.MusicRecognizer
	Timeout    time.Duration
}
