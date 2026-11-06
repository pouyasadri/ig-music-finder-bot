package telegram

import (
	"regexp"
	"strings"

	"telegram-audio-bot/internal/domain"

	"github.com/go-telegram/bot/models"
)

var (
	igURLRegex         = regexp.MustCompile(`https?://(?:www\.)?(?:instagram\.com|instagr\.am)/(?:reel|p|share/reel)/[a-zA-Z0-9_\-\.]+/?(?:\?[^\s]*)?`)
	soundCloudURLRegex = regexp.MustCompile(`https?://(?:www\.|m\.)?(?:soundcloud\.com|on\.soundcloud\.com|soundcloud\.app\.goo\.gl)/[^\s]+`)
	sanitizeFilename   = regexp.MustCompile(`[<>:"/\\|?*]`)
)

type classifiedMedia struct {
	Kind          domain.SourceKind
	SourceURL     string
	MediaID       string
	MediaUniqueID string
}

func classifyIncoming(text string, message *models.Message) (classifiedMedia, bool) {
	igMatch := igURLRegex.FindString(text)
	scMatch := soundCloudURLRegex.FindString(text)
	mediaID, mediaUniqueID := telegramMediaIDs(message)
	if igMatch == "" && scMatch == "" && mediaID == "" {
		return classifiedMedia{}, false
	}

	out := classifiedMedia{
		Kind:          domain.SourceKindInstagram,
		SourceURL:     igMatch,
		MediaID:       mediaID,
		MediaUniqueID: mediaUniqueID,
	}
	if out.SourceURL == "" && scMatch != "" {
		out.Kind = domain.SourceKindSoundCloud
		out.SourceURL = scMatch
	}
	if out.SourceURL == "" {
		out.Kind = domain.SourceKindUpload
		out.SourceURL = "file://" + mediaUniqueID
	}
	return out, true
}

func normalizeSourceURL(raw string) string {
	return domain.NormalizeMediaURL(strings.TrimSpace(raw))
}
