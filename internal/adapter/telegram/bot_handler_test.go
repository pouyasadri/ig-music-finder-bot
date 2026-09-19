package telegram

import (
	"testing"
)

func TestURLRegexMatching(t *testing.T) {
	testCases := []struct {
		name         string
		input        string
		wantIG       bool
		wantSC       bool
		expectedLink string
	}{
		{
			name:         "valid instagram reel",
			input:        "https://www.instagram.com/reel/C-xyz123/",
			wantIG:       true,
			wantSC:       false,
			expectedLink: "https://www.instagram.com/reel/C-xyz123/",
		},
		{
			name:         "valid instagram post with query param",
			input:        "Check this out: https://instagram.com/p/ABC12345/?igsh=xyz",
			wantIG:       true,
			wantSC:       false,
			expectedLink: "https://instagram.com/p/ABC12345/?igsh=xyz",
		},
		{
			name:         "valid soundcloud standard track",
			input:        "https://soundcloud.com/artist-name/track-title",
			wantIG:       false,
			wantSC:       true,
			expectedLink: "https://soundcloud.com/artist-name/track-title",
		},
		{
			name:         "valid soundcloud with query params",
			input:        "https://soundcloud.com/artist-name/track-title?si=123456789&utm_source=clipboard",
			wantIG:       false,
			wantSC:       true,
			expectedLink: "https://soundcloud.com/artist-name/track-title?si=123456789&utm_source=clipboard",
		},
		{
			name:         "valid soundcloud mobile link",
			input:        "https://m.soundcloud.com/user_123/my-track",
			wantIG:       false,
			wantSC:       true,
			expectedLink: "https://m.soundcloud.com/user_123/my-track",
		},
		{
			name:         "valid soundcloud short link on.soundcloud.com",
			input:        "Listen here https://on.soundcloud.com/abcdef123 please",
			wantIG:       false,
			wantSC:       true,
			expectedLink: "https://on.soundcloud.com/abcdef123",
		},
		{
			name:         "valid soundcloud app short link",
			input:        "https://soundcloud.app.goo.gl/xyz987",
			wantIG:       false,
			wantSC:       true,
			expectedLink: "https://soundcloud.app.goo.gl/xyz987",
		},
		{
			name:   "random non-supported url",
			input:  "https://twitter.com/user/status/12345",
			wantIG: false,
			wantSC: false,
		},
		{
			name:   "plain text message",
			input:  "hello bot, how are you?",
			wantIG: false,
			wantSC: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			igMatch := igURLRegex.FindString(tc.input)
			scMatch := soundCloudURLRegex.FindString(tc.input)

			if (igMatch != "") != tc.wantIG {
				t.Errorf("instagram match mismatch: got %q, wantIG=%v", igMatch, tc.wantIG)
			}
			if (scMatch != "") != tc.wantSC {
				t.Errorf("soundcloud match mismatch: got %q, wantSC=%v", scMatch, tc.wantSC)
			}

			if tc.wantIG && igMatch != tc.expectedLink {
				t.Errorf("expected ig link %q, got %q", tc.expectedLink, igMatch)
			}
			if tc.wantSC && scMatch != tc.expectedLink {
				t.Errorf("expected sc link %q, got %q", tc.expectedLink, scMatch)
			}
		})
	}
}
