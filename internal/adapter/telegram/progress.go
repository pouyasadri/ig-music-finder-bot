package telegram

import (
	"telegram-audio-bot/internal/adapter/telegram/copy"
	"telegram-audio-bot/internal/domain"
	"telegram-audio-bot/internal/usecase"
)

type messageProgressReporter struct {
	update func(string)
}

func newMessageProgressReporter(update func(string)) usecase.ProgressReporter {
	if update == nil {
		return usecase.NopProgress()
	}
	return messageProgressReporter{update: update}
}

func (r messageProgressReporter) Report(stage domain.ProgressStage, detail usecase.ProgressDetail) {
	r.update(copy.FormatProgress(stage, detail))
}
