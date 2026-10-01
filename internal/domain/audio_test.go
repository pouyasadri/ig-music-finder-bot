package domain_test

import (
	"errors"
	"testing"

	"telegram-audio-bot/internal/domain"
)

func TestDurationPolicyMatches(t *testing.T) {
	policy := domain.DefaultDurationPolicy()
	tests := []struct {
		name     string
		actual   int
		expected int
		wantOK   bool
		wantErr  error
	}{
		{name: "matching duration", actual: 240, expected: 240, wantOK: true},
		{name: "small provider padding", actual: 250, expected: 240, wantOK: true},
		{name: "preview is rejected", actual: 30, expected: 240, wantErr: domain.ErrDurationMismatch},
		{name: "unexpectedly long result is rejected", actual: 1000, expected: 240, wantErr: domain.ErrDurationMismatch},
		{name: "unknown short result is rejected", actual: 30, expected: 0, wantErr: domain.ErrDurationMismatch},
		{name: "unknown normal result is accepted", actual: 180, expected: 0, wantOK: true},
		{name: "zero duration is rejected", actual: 0, expected: 180, wantErr: domain.ErrDurationMismatch},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := policy.Validate(tc.actual, tc.expected)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("Validate() unexpected error: %v", err)
				}
				if !policy.Matches(tc.actual, tc.expected) {
					t.Fatal("Matches() = false, want true")
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tc.wantErr)
			}
			if policy.Matches(tc.actual, tc.expected) {
				t.Fatal("Matches() = true, want false")
			}
		})
	}
}

func TestParseOutputMode(t *testing.T) {
	mode, ok := domain.ParseOutputMode("FULL")
	if !ok || mode != domain.OutputModeFull {
		t.Fatalf("ParseOutputMode(FULL) = %q, %v", mode, ok)
	}
	if _, ok := domain.ParseOutputMode("nope"); ok {
		t.Fatal("expected invalid mode")
	}
	if domain.OutputMode("").OrDefault() != domain.OutputModeFull {
		t.Fatal("empty mode should default to full")
	}
}

func TestNormalizeMediaURL(t *testing.T) {
	got := domain.NormalizeMediaURL(" https://Example.com/path/?utm=1 ")
	want := "https://example.com/path"
	if got != want {
		t.Fatalf("NormalizeMediaURL() = %q, want %q", got, want)
	}
}
