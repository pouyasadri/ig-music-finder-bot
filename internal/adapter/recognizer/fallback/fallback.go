package fallback

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"telegram-audio-bot/internal/domain"
)

const (
	defaultMinAcceptScore    = 0.7
	defaultShortCircuitScore = 0.9
)

// Engine defines a concrete recognizer implementation together with its timeout budget.
type Engine struct {
	Name       string
	Recognizer domain.MusicRecognizer
	Timeout    time.Duration
}

// Recognizer tries engines in order, optionally short-circuiting on a strong match.
type Recognizer struct {
	engines           []Engine
	minAcceptScore    float64
	shortCircuitScore float64
}

var candidateKeyPattern = regexp.MustCompile(`[^a-z0-9]+`)

func New(engines ...Engine) *Recognizer {
	return &Recognizer{
		engines:           engines,
		minAcceptScore:    defaultMinAcceptScore,
		shortCircuitScore: defaultShortCircuitScore,
	}
}

// WithScores configures acceptance and early-exit thresholds.
// A shortCircuit <= 0 disables short-circuiting.
func (f *Recognizer) WithScores(minAccept, shortCircuit float64) *Recognizer {
	if minAccept > 0 {
		f.minAcceptScore = minAccept
	}
	f.shortCircuitScore = shortCircuit
	return f
}

func (f *Recognizer) Identify(ctx context.Context, snippetPath string) (*domain.TrackMetadata, error) {
	candidates := make([]domain.RecognitionCandidate, 0, len(f.engines))
	for _, engine := range f.engines {
		slog.Info("attempting music recognition", "engine", engine.Name)

		engineCtx, cancel := context.WithTimeout(ctx, engine.Timeout)
		t0 := time.Now()
		meta, err := engine.Recognizer.Identify(engineCtx, snippetPath)
		cancel()

		elapsed := time.Since(t0).Milliseconds()

		if err == nil && meta != nil && meta.IsMatched {
			score := scoreRecognitionCandidate(meta)
			candidate := domain.RecognitionCandidate{Provider: engine.Name, Metadata: meta, Score: score}
			candidates = append(candidates, candidate)
			slog.Info("recognition successful", "engine", engine.Name, "title", meta.Title, "artist", meta.Artist, "score", score, "elapsed_ms", elapsed)

			if f.shortCircuitScore > 0 && score+1e-9 >= f.shortCircuitScore {
				slog.Info("recognition short-circuit", "engine", engine.Name, "score", score, "threshold", f.shortCircuitScore)
				return meta, nil
			}
			continue
		}

		slog.Warn("recognition missed or failed, moving to next tier", "engine", engine.Name, "elapsed_ms", elapsed, "err", err)
	}

	winner := selectBestRecognitionCandidate(candidates)
	if winner == nil || winner.Score < f.minAcceptScore {
		return &domain.TrackMetadata{IsMatched: false}, nil
	}

	return winner.Metadata, nil
}

func scoreRecognitionCandidate(meta *domain.TrackMetadata) float64 {
	if meta == nil {
		return 0
	}

	score := 0.0
	if strings.TrimSpace(meta.Title) != "" {
		score += 0.35
	}
	if strings.TrimSpace(meta.Artist) != "" {
		score += 0.3
	}
	if meta.Duration > 0 {
		score += 0.15
	}
	if strings.TrimSpace(meta.SpotifyURL) != "" || strings.TrimSpace(meta.YouTubeURL) != "" {
		score += 0.1
	}
	title := strings.ToLower(strings.TrimSpace(meta.Title))
	if strings.Contains(title, "unknown") || strings.Contains(title, "unmatched") {
		score -= 0.25
	}
	if strings.Contains(title, "original") {
		score -= 0.15
	}
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score
}

func selectBestRecognitionCandidate(candidates []domain.RecognitionCandidate) *domain.RecognitionCandidate {
	if len(candidates) == 0 {
		return nil
	}

	grouped := map[string][]domain.RecognitionCandidate{}
	for _, candidate := range candidates {
		if candidate.Metadata == nil {
			continue
		}
		key := normalizeTrackKey(candidate.Metadata.Title, candidate.Metadata.Artist)
		grouped[key] = append(grouped[key], candidate)
	}

	best := &domain.RecognitionCandidate{Score: -1}
	for _, group := range grouped {
		maxScore := 0.0
		groupTotal := 0.0
		winningCandidate := group[0]
		for _, candidate := range group {
			groupTotal += candidate.Score
			if candidate.Score > maxScore {
				maxScore = candidate.Score
				winningCandidate = candidate
			}
		}

		agreementBoost := 0.0
		if len(group) > 1 {
			agreementBoost = 0.15 * float64(len(group)-1)
		}
		finalScore := maxScore + (groupTotal/float64(len(group)))*0.3 + agreementBoost
		if finalScore > 1 {
			finalScore = 1
		}

		winningCandidate.Score = finalScore
		if finalScore > best.Score {
			best = &winningCandidate
		}
	}

	if best.Score < 0 {
		return nil
	}
	return best
}

func normalizeTrackKey(title, artist string) string {
	serial := strings.ToLower(strings.TrimSpace(title) + "|" + strings.TrimSpace(artist))
	serial = candidateKeyPattern.ReplaceAllString(serial, " ")
	serial = strings.Join(strings.Fields(serial), " ")
	return serial
}
