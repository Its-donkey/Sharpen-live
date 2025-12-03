package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/websub"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	"github.com/Its-donkey/Sharpen-live/internal/alert/submissions"
	"github.com/Its-donkey/Sharpen-live/internal/metadata"
	// Action represents the allowed admin submission actions.
)

type Action string

const (
	// ActionApprove represents approving a pending submission.
	ActionApprove Action = "approve"
	// ActionReject represents rejecting a pending submission.
	ActionReject Action = "reject"
)

// ActionRequest captures the payload required to mutate a submission.
type ActionRequest struct {
	Action Action `json:"action"`
	ID     string `json:"id"`
}

// ActionResult contains the final status for the processed submission.
type ActionResult struct {
	Status     Action                 `json:"status"`
	Submission submissions.Submission `json:"submission"`
}

// SubmissionsOptions configures the SubmissionsService.
type SubmissionsOptions struct {
	SubmissionsStore      *submissions.Store
	StreamersStore        *streamers.Store
	WebSubCallbackBaseURL string
	MetadataService       *metadata.Service
}

// SubmissionsService encapsulates streamer submission review logic.
type SubmissionsService struct {
	submissionsStore      *submissions.Store
	streamersStore        *streamers.Store
	websubCallbackBaseURL string
	metadataService       *metadata.Service
}

// NewSubmissionsService constructs a SubmissionsService with the provided options.
func NewSubmissionsService(opts SubmissionsOptions) *SubmissionsService {
	submissionsStore := opts.SubmissionsStore
	if submissionsStore == nil {
		submissionsStore = submissions.NewStore(submissions.DefaultFilePath)
	}
	streamersStore := opts.StreamersStore
	if streamersStore == nil {
		streamersStore = streamers.NewStore(streamers.DefaultFilePath)
	}
	svc := &SubmissionsService{
		submissionsStore:      submissionsStore,
		streamersStore:        streamersStore,
		websubCallbackBaseURL: opts.WebSubCallbackBaseURL,
		metadataService:       opts.MetadataService,
	}
	return svc
}

// List returns every pending submission.
func (s *SubmissionsService) List(ctx context.Context) ([]submissions.Submission, error) {
	if err := s.ensureStores(); err != nil {
		return nil, err
	}
	return s.submissionsStore.List()
}

// Process mutates a submission according to the provided action.
func (s *SubmissionsService) Process(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := s.ensureStores(); err != nil {
		return ActionResult{}, err
	}
	action := normaliseAction(req.Action)
	if action == "" {
		return ActionResult{}, ErrInvalidAction
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		return ActionResult{}, ErrMissingIdentifier
	}
	removed, err := s.submissionsStore.Remove(id)
	if err != nil {
		return ActionResult{}, err
	}
	if action == ActionApprove {
		if err := s.approve(ctx, removed); err != nil {
			// requeue the submission when approval fails
			_, _ = s.submissionsStore.Append(removed)
			return ActionResult{}, err
		}
		return ActionResult{Status: ActionApprove, Submission: removed}, nil
	}
	return ActionResult{Status: ActionReject, Submission: removed}, nil
}

func (s *SubmissionsService) ensureStores() error {
	if s == nil {
		return errors.New("submissions service is nil")
	}
	if s.submissionsStore == nil {
		return errors.New("submissions store is not configured")
	}
	if s.streamersStore == nil {
		return errors.New("streamers store is not configured")
	}
	return nil
}

func (s *SubmissionsService) approve(ctx context.Context, submission submissions.Submission) error {
	record := streamers.Record{
		Streamer: streamers.Streamer{
			Alias:       strings.TrimSpace(submission.Alias),
			Description: strings.TrimSpace(submission.Description),
			Languages:   append([]string(nil), submission.Languages...),
		},
	}

	// Map platform details
	if submission.Platforms != nil {
		for platformKey, platformInfo := range submission.Platforms {
			p := strings.ToLower(strings.TrimSpace(platformInfo.Platform))
			if p == "" {
				p = strings.ToLower(strings.TrimSpace(platformKey))
			}

			switch {
			case p == "youtube" || strings.Contains(p, "youtube"):
				channelURL := strings.TrimSpace(platformInfo.URL)
				channelID := strings.TrimSpace(platformInfo.ChannelID)

				fmt.Printf("INFO: Processing YouTube platform for submission\n")
				fmt.Printf("  URL: %s\n", channelURL)
				fmt.Printf("  Channel ID from submission: %s\n", channelID)

				if channelID == "" {
					channelID = extractYouTubeChannelID(channelURL)
					fmt.Printf("  Channel ID from URL extraction: %s\n", channelID)
				}
				if channelID == "" && s.metadataService != nil {
					fmt.Printf("  Attempting to fetch channel ID via metadata service...\n")
					if meta, err := s.metadataService.Fetch(ctx, channelURL); err == nil && meta != nil {
						if cid := strings.TrimSpace(meta.ChannelID); cid != "" {
							channelID = cid
							fmt.Printf("  Channel ID from metadata: %s\n", channelID)
						}
					} else if err != nil {
						fmt.Printf("  Metadata fetch error: %v\n", err)
					}
				}

				ytPlatform := &streamers.YouTubePlatform{
					ChannelID:  channelID,
					ChannelURL: channelURL,
				}

				if channelID != "" {
					fmt.Printf("INFO: Channel ID available, setting up WebSub...\n")
					if subscribed, err := s.setupYouTubeWebSub(ctx, channelID, channelURL); err != nil {
						fmt.Printf("WARNING: Failed to set up YouTube WebSub for channel %s: %v\n", channelID, err)
					} else if subscribed != nil {
						ytPlatform = subscribed
						fmt.Printf("INFO: WebSub setup successful, platform data updated\n")
					}
				} else {
					fmt.Printf("WARNING: No channel ID available for YouTube platform, skipping WebSub\n")
				}
				record.Platforms.YouTube = ytPlatform
				fmt.Printf("INFO: YouTube platform configured in record\n")

				// case p == "twitch" || strings.Contains(p, "twitch"):
				// 	username := inferTwitchUsername(platformInfo.URL, platformInfo.Handle, platformInfo.Label)
				// 	if username != "" && record.Platforms.Twitch == nil {
				// 		record.Platforms.Twitch = &streamers.TwitchPlatform{Username: username}
				// 	}

				// case p == "facebook" || strings.Contains(p, "facebook"):
				// 	pageID := inferFacebookPageID(platformInfo.URL, platformInfo.Handle, platformInfo.Label)
				// 	if pageID != "" && record.Platforms.Facebook == nil {
				// 		record.Platforms.Facebook = &streamers.FacebookPlatform{PageID: pageID}
				// 	}
			}
		}
	}

	saved, err := s.streamersStore.Append(record)
	if err != nil {
		fmt.Printf("ERROR: Failed to save streamer record: %v\n", err)
		return err
	}

	// Verify the record was saved correctly
	fmt.Printf("SUCCESS: Streamer record saved\n")
	fmt.Printf("  ID: %s\n", saved.Streamer.ID)
	fmt.Printf("  Alias: %s\n", saved.Streamer.Alias)
	if saved.Platforms.YouTube != nil {
		fmt.Printf("  YouTube Channel ID: %s\n", saved.Platforms.YouTube.ChannelID)
		fmt.Printf("  YouTube WebSub Subscribed: %v\n", saved.Platforms.YouTube.WebSubSubscribed)
		if saved.Platforms.YouTube.WebSubSubscribed {
			fmt.Printf("  YouTube WebSub Hub: %s\n", saved.Platforms.YouTube.WebSubHubURL)
			fmt.Printf("  YouTube WebSub Topic: %s\n", saved.Platforms.YouTube.WebSubTopicURL)
			fmt.Printf("  YouTube WebSub Callback: %s\n", saved.Platforms.YouTube.WebSubCallbackURL)
			if saved.Platforms.YouTube.WebSubLeaseExpiry != nil {
				fmt.Printf("  YouTube WebSub Expiry: %s\n", saved.Platforms.YouTube.WebSubLeaseExpiry.Format("2006-01-02 15:04:05 MST"))
			}
		}
	}

	return nil
}

// setupYouTubeWebSub sets up a WebSub subscription for a YouTube channel
func (s *SubmissionsService) setupYouTubeWebSub(ctx context.Context, channelID, channelURL string) (*streamers.YouTubePlatform, error) {
	if s.websubCallbackBaseURL == "" {
		fmt.Printf("INFO: WebSub callback base URL not configured, skipping subscription for channel %s\n", channelID)
		return &streamers.YouTubePlatform{
			ChannelID:  channelID,
			ChannelURL: channelURL,
		}, nil
	}

	// Use callback URL exactly as configured in config.json
	callbackURL := s.websubCallbackBaseURL

	fmt.Printf("INFO: Setting up YouTube WebSub subscription\n")
	fmt.Printf("  Channel ID: %s\n", channelID)
	fmt.Printf("  Channel URL: %s\n", channelURL)
	fmt.Printf("  Callback URL: %s\n", callbackURL)

	// Subscribe to WebSub
	result, err := websub.Subscribe(websub.SubscriptionRequest{
		ChannelID:    channelID,
		CallbackURL:  callbackURL,
		LeaseSeconds: websub.DefaultLeaseSeconds,
	})
	if err != nil {
		fmt.Printf("ERROR: Failed to subscribe to WebSub for channel %s: %v\n", channelID, err)
		return nil, fmt.Errorf("subscribe to websub: %w", err)
	}

	fmt.Printf("SUCCESS: WebSub subscription created for channel %s\n", channelID)
	fmt.Printf("  Hub URL: %s\n", result.HubURL)
	fmt.Printf("  Topic URL: %s\n", result.TopicURL)
	fmt.Printf("  Lease Expiry: %s\n", result.LeaseExpiry.Format("2006-01-02 15:04:05 MST"))

	return &streamers.YouTubePlatform{
		ChannelID:         channelID,
		ChannelURL:        channelURL,
		WebSubHubURL:      result.HubURL,
		WebSubTopicURL:    result.TopicURL,
		WebSubCallbackURL: result.CallbackURL,
		WebSubSecret:      result.Secret,
		WebSubLeaseExpiry: &result.LeaseExpiry,
		WebSubSubscribed:  true,
	}, nil
}

// extractYouTubeChannelID extracts a YouTube channel ID from various URL formats
func extractYouTubeChannelID(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	// If it's already a channel ID (starts with UC and is 24 chars)
	if strings.HasPrefix(rawURL, "UC") && len(rawURL) == 24 {
		return rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// Check query parameters for channel_id
	if channelID := parsed.Query().Get("channel_id"); channelID != "" {
		return channelID
	}

	// Extract from path
	path := strings.Trim(parsed.Path, "/")
	segments := strings.Split(path, "/")

	// Handle /channel/UCxxxxxx format
	if len(segments) >= 2 && segments[0] == "channel" {
		return segments[1]
	}

	return ""
}

func normaliseAction(value Action) Action {
	normalized := Action(strings.ToLower(strings.TrimSpace(string(value))))
	switch normalized {
	case ActionApprove, ActionReject:
		return normalized
	default:
		return ""
	}
}

var (
	// ErrInvalidAction indicates the request payload contained an unsupported action.
	ErrInvalidAction = errors.New("action must be approve or reject")
	// ErrMissingIdentifier signals that the submission ID was omitted.
	ErrMissingIdentifier = errors.New("submission id is required")
)
