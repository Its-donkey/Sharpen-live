package youtube

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	liveinfo "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/liveinfo"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

// MaybeEnrichMetadata pulls lightweight metadata from YouTube to prefill submit forms.
func MaybeEnrichMetadata(ctx context.Context, form *model.SubmitFormState, client *http.Client) {
	if form == nil {
		return
	}
	target := ""
	for _, p := range form.Platforms {
		if url := strings.TrimSpace(p.ChannelURL); url != "" {
			target = url
			break
		}
	}
	if target == "" {
		return
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return
	}
	results, err := (&liveinfo.Client{HTTPClient: client}).Fetch(ctx, []string{parsed.String()})
	if err != nil {
		return
	}
	if video, ok := results[parsed.String()]; ok {
		if form.Description == "" && video.Title != "" {
			form.Description = video.Title
		}
	}
}
