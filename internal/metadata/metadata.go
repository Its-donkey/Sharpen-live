package metadata

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/Its-donkey/Sharpen-live/logging"
)

// Metadata represents collected information about a streaming channel.
type Metadata struct {
	Title       string
	Description string
	Handle      string
	ChannelID   string
	Languages   []string
	Platform    string
}

// PlatformCollector defines the interface for platform-specific collector.
type PlatformMetadataCollector interface {
	Matches(url string) bool
	Collect(ctx context.Context, url string) (*Metadata, error)
}

// Service orchestrates metadata collection across different platforms.
type Service struct {
	httpClient *http.Client
	platforms  []PlatformMetadataCollector
	logger     *logging.Logger
}

// NewService creates a new metadata collection request with the given HTTP client.
func NewService(httpClient *http.Client, logger *logging.Logger, youtubeAPIKey string) *Service {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	if logger == nil {
		logger = logging.New("metadata", logging.INFO, io.Discard)
	}
	return &Service{
		httpClient: httpClient,
		platforms: []PlatformMetadataCollector{
			&YouTubeMetaCollect{client: httpClient, logger: logger, apiKey: youtubeAPIKey},
			&TwitchMetadata{client: httpClient, logger: logger},
			&FacebookScraper{client: httpClient},
		},
		logger: logger,
	}
}

// Fetch retrieves metadata for the given URL.
func (s *Service) Fetch(ctx context.Context, rawURL string) (*Metadata, error) {
	// Validate and normalise URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	u.Scheme = "https"

	normalisedURL := u.String()

	var lastErr error

	// Try platform-specific metadata collection
	for _, platform := range s.platforms {
		if platform.Matches(normalisedURL) {
			metadata, err := platform.Collect(ctx, normalisedURL)
			if err == nil && metadata != nil {
				return metadata, nil
			}
			if err != nil {
				lastErr = err
			}
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, fmt.Errorf("no metadata collector matched url")
}
