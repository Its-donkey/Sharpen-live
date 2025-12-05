package metadata

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/logging"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// YouTubeMetaCollect collects metadata from YouTube channels.
type YouTubeMetaCollect struct {
	client *http.Client
	logger *logging.Logger
	apiKey string
}

// Matches returns true if the URL is a YouTube URL.
func (s *YouTubeMetaCollect) Matches(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "youtube.com") || strings.Contains(lower, "youtu.be")
}

// Collect fetches metadata from a YouTube channel page.
func (s *YouTubeMetaCollect) Collect(ctx context.Context, url string) (*Metadata, error) {
	// Create request with context and timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	logInfo := func(category, message string, fields map[string]any) {
		if s.logger != nil {
			s.logger.Info(category, message, fields)
		}
	}
	logError := func(category, message string, err error, fields map[string]any) {
		if s.logger != nil {
			s.logger.Error(category, message, err, fields)
		}
	}

	// Create YouTube service using API key.
	apiKey := strings.TrimSpace(s.apiKey)
	if apiKey == "" {
		apiKey = os.Getenv("YOUTUBE_API_KEY")
	}
	service, err := youtube.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		logError("Metadata - YouTube", "error creating YouTube service", err, map[string]any{
			"url": url,
		})
		return nil, fmt.Errorf("create youtube service: %w", err)
	}

	call := service.Channels.List([]string{"id", "snippet"})

	kind, handle := extractYouTubeHandle(url)
	switch kind {
	case 1:
		call = call.ForHandle(handle)
	case 2:
		call = call.Id(handle)
	case 3:
		call = call.ForUsername(handle)
	case 4:
		call = call.Id(handle)
	default:
		logInfo("Metadata - YouTube", "unable to extract handle from URL", map[string]any{
			"url": url,
		})
		return nil, fmt.Errorf("unable to extract handle from url %q", url)
	}

	users, err := call.Do()
	if err != nil {
		logError("Metadata - YouTube", "API call error for URL", err, map[string]any{
			"url": url,
		})
		return nil, err
	}

	if len(users.Items) == 0 {
		logInfo("Metadata - YouTube", "no channels found for URL", map[string]any{"url": url})
		return nil, fmt.Errorf("no channels found for url %q", url)
	}

	user := users.Items[0]

	metadata := &Metadata{
		Platform:    "youtube",
		Title:       user.Snippet.Title,
		Description: user.Snippet.Description,
		Handle:      user.Snippet.CustomUrl,
		ChannelID:   user.Id,
		Languages:   []string{"English"},
	}
	return metadata, nil
}

func extractYouTubeHandle(url string) (int, string) {
	// kind: which pattern matched
	// prefix: whether to add "@" in front
	type pattern struct {
		kind   int
		prefix string
		expr   string
	}

	patterns := []pattern{
		// Match @handle pattern
		{1, "@", `@([a-zA-Z0-9_-]+)`},

		// Match /c/channelname pattern
		{2, "", `/c/([a-zA-Z0-9_-]+)`},

		// Match /user/username pattern
		{3, "", `/user/([a-zA-Z0-9_-]+)`},

		// Match /channel/UC... pattern
		{4, "", `/channel/(UC[a-zA-Z0-9_-]+)`},
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p.expr)
		if matches := re.FindStringSubmatch(url); len(matches) > 1 {
			return p.kind, p.prefix + matches[1]
		}
	}

	// No match
	return 0, ""
}
