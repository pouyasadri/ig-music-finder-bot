package usecase

import "telegram-audio-bot/internal/domain"

// ProgressDetail carries optional metadata for a progress stage.
// Adapters own localization; usecases must not put user-facing copy here.
type ProgressDetail struct {
	Title    string
	Artist   string
	Matched  bool
	Fallback bool
}

// ProgressReporter receives UI-agnostic processing stage updates.
type ProgressReporter interface {
	Report(stage domain.ProgressStage, detail ProgressDetail)
}

type nopProgressReporter struct{}

func (nopProgressReporter) Report(domain.ProgressStage, ProgressDetail) {}

// NopProgress returns a reporter that discards all updates.
func NopProgress() ProgressReporter {
	return nopProgressReporter{}
}

func reportProgress(reporter ProgressReporter, stage domain.ProgressStage, detail ProgressDetail) {
	if reporter == nil {
		return
	}
	reporter.Report(stage, detail)
}
