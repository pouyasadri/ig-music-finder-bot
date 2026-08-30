package recognizer

import (
	"time"

	"telegram-audio-bot/internal/usecase"
)

// Engine defines a concrete recognizer implementation together with its timeout budget.
type Engine struct {
	Name       string
	Recognizer usecase.MusicRecognizer
	Timeout    time.Duration
}
