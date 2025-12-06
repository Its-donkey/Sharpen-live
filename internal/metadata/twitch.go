package metadata

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	twitch "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/twitch/api"
	"github.com/Its-donkey/Sharpen-live/logging"
)

// TwitchMetadata collects metadata from Twitch via the Helix API.
type TwitchMetadata struct {
	client       *http.Client
	logger       *logging.Logger
	clientID     string
	clientSecret string
}

// Matches returns true if the URL is a Twitch URL.
func (s *TwitchMetadata) Matches(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "twitch.tv")
}

// Collect gathers metadata from Twitch using the Helix API.
func (s *TwitchMetadata) Collect(ctx context.Context, url string) (*Metadata, error) {
	twitchLogin := extractTwitchHandle(url)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	httpClient := s.client
	auth := twitch.NewAuthenticator(httpClient, s.clientID, s.clientSecret)

	users, err := twitch.GetUsers(ctx, httpClient, auth, nil, []string{twitchLogin})
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Metadata - Twitch", "fetch user", err, map[string]any{
				"username": twitchLogin,
			})
		}
		return nil, fmt.Errorf("fetch user %q: %w", twitchLogin, err)
	}
	if len(users) == 0 {
		if s.logger != nil {
			s.logger.Info("Metadata - Twitch", "no data returned for user", map[string]any{
				"username": twitchLogin,
			})
		}
		return nil, fmt.Errorf("no data returned for user %q", twitchLogin)
	}

	user := users[0]

	metadata := &Metadata{
		Platform:    "twitch",
		Title:       user.DisplayName,
		Description: user.Description,
		Handle:      user.Login,
		ChannelID:   user.ID,
		Languages:   []string{"English"},
	}

	return metadata, nil
}

// extractTwitchHandle extracts the username from a Twitch URL.
func extractTwitchHandle(url string) string {
	// Match twitch.tv/username pattern
	re := regexp.MustCompile(`twitch\.tv/([a-zA-Z0-9_]+)`)
	if matches := re.FindStringSubmatch(url); len(matches) > 1 {
		return matches[1]
	}

	return ""
}
