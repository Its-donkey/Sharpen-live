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
	fmt.Printf("\n========================================\n")
	fmt.Printf("=== PROCESS SUBMISSION REQUEST START ===\n")
	fmt.Printf("========================================\n")
	fmt.Printf("Action: %s\n", req.Action)
	fmt.Printf("Submission ID: %s\n", req.ID)

	if err := s.ensureStores(); err != nil {
		fmt.Printf("ERROR: Store validation failed: %v\n", err)
		fmt.Printf("=== PROCESS SUBMISSION REQUEST END (failed) ===\n\n")
		return ActionResult{}, err
	}
	action := normaliseAction(req.Action)
	if action == "" {
		fmt.Printf("ERROR: Invalid action provided: %s\n", req.Action)
		fmt.Printf("=== PROCESS SUBMISSION REQUEST END (failed) ===\n\n")
		return ActionResult{}, ErrInvalidAction
	}
	fmt.Printf("Normalized action: %s\n", action)

	id := strings.TrimSpace(req.ID)
	if id == "" {
		fmt.Printf("ERROR: No submission ID provided\n")
		fmt.Printf("=== PROCESS SUBMISSION REQUEST END (failed) ===\n\n")
		return ActionResult{}, ErrMissingIdentifier
	}

	fmt.Printf("\nINFO: Removing submission %s from submissions store...\n", id)
	removed, err := s.submissionsStore.Remove(id)
	if err != nil {
		fmt.Printf("ERROR: Failed to remove submission: %v\n", err)
		fmt.Printf("=== PROCESS SUBMISSION REQUEST END (failed) ===\n\n")
		return ActionResult{}, err
	}
	fmt.Printf("SUCCESS: Submission removed from store\n")
	fmt.Printf("  Alias: %s\n", removed.Alias)
	fmt.Printf("  Platforms: %d\n", len(removed.Platforms))

	if action == ActionApprove {
		fmt.Printf("\n>>> ACTION IS APPROVE - Starting approval process...\n")
		if err := s.approve(ctx, removed); err != nil {
			fmt.Printf("\nERROR: Approval process failed: %v\n", err)
			fmt.Printf("INFO: Re-queueing submission to submissions store...\n")
			// requeue the submission when approval fails
			_, _ = s.submissionsStore.Append(removed)
			fmt.Printf("=== PROCESS SUBMISSION REQUEST END (failed) ===\n\n")
			return ActionResult{}, err
		}
		fmt.Printf("\nSUCCESS: Approval process completed\n")
		fmt.Printf("========================================\n")
		fmt.Printf("=== PROCESS SUBMISSION REQUEST END (success) ===\n")
		fmt.Printf("========================================\n\n")
		return ActionResult{Status: ActionApprove, Submission: removed}, nil
	}

	fmt.Printf("\n>>> ACTION IS REJECT - Submission rejected\n")
	fmt.Printf("========================================\n")
	fmt.Printf("=== PROCESS SUBMISSION REQUEST END (success) ===\n")
	fmt.Printf("========================================\n\n")
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
	fmt.Printf("\n=== APPROVE SUBMISSION START ===\n")
	fmt.Printf("Submission ID: %s\n", submission.ID)
	fmt.Printf("Submission Alias: %s\n", submission.Alias)
	fmt.Printf("Submission Description: %s\n", submission.Description)
	fmt.Printf("Submission Languages: %v\n", submission.Languages)
	fmt.Printf("Submission Platforms: %d platform(s)\n", len(submission.Platforms))

	record := streamers.Record{
		Streamer: streamers.Streamer{
			Alias:       strings.TrimSpace(submission.Alias),
			Description: strings.TrimSpace(submission.Description),
			Languages:   append([]string(nil), submission.Languages...),
		},
	}

	fmt.Printf("\nINFO: Created initial streamer record\n")
	fmt.Printf("  Alias: %s\n", record.Streamer.Alias)
	fmt.Printf("  Description: %s\n", record.Streamer.Description)
	fmt.Printf("  Languages: %v\n", record.Streamer.Languages)

	// Map platform details
	if submission.Platforms != nil {
		fmt.Printf("\nINFO: Processing %d platform(s)...\n", len(submission.Platforms))
		for platformKey, platformInfo := range submission.Platforms {
			fmt.Printf("\n--- Processing platform: %s ---\n", platformKey)
			fmt.Printf("  Platform info URL: %s\n", platformInfo.URL)
			fmt.Printf("  Platform info Channel ID: %s\n", platformInfo.ChannelID)
			fmt.Printf("  Platform info Platform field: %s\n", platformInfo.Platform)

			p := strings.ToLower(strings.TrimSpace(platformInfo.Platform))
			if p == "" {
				p = strings.ToLower(strings.TrimSpace(platformKey))
			}
			fmt.Printf("  Normalized platform: %s\n", p)

			switch {
			case p == "youtube" || strings.Contains(p, "youtube"):
				fmt.Printf("\n*** YouTube Platform Detected ***\n")
				channelURL := strings.TrimSpace(platformInfo.URL)
				channelID := strings.TrimSpace(platformInfo.ChannelID)

				fmt.Printf("INFO: Processing YouTube platform for submission\n")
				fmt.Printf("  URL: %s\n", channelURL)
				fmt.Printf("  Channel ID from submission: %s\n", channelID)

				if channelID == "" {
					fmt.Printf("INFO: Channel ID not in submission, attempting URL extraction...\n")
					channelID = extractYouTubeChannelID(channelURL)
					fmt.Printf("  Channel ID from URL extraction: %s\n", channelID)
				}
				if channelID == "" && s.metadataService != nil {
					fmt.Printf("INFO: Attempting to fetch channel ID via metadata service...\n")
					if meta, err := s.metadataService.Fetch(ctx, channelURL); err == nil && meta != nil {
						if cid := strings.TrimSpace(meta.ChannelID); cid != "" {
							channelID = cid
							fmt.Printf("SUCCESS: Channel ID from metadata: %s\n", channelID)
						} else {
							fmt.Printf("WARNING: Metadata service returned no channel ID\n")
						}
					} else if err != nil {
						fmt.Printf("ERROR: Metadata fetch error: %v\n", err)
					}
				} else if channelID == "" && s.metadataService == nil {
					fmt.Printf("WARNING: No metadata service available for channel ID lookup\n")
				}

				ytPlatform := &streamers.YouTubePlatform{
					ChannelID:  channelID,
					ChannelURL: channelURL,
				}
				fmt.Printf("INFO: Created initial YouTubePlatform struct\n")

				if channelID != "" {
					fmt.Printf("\nINFO: Channel ID available (%s), proceeding with WebSub setup...\n", channelID)
					if subscribed, err := s.setupYouTubeWebSub(ctx, channelID, channelURL); err != nil {
						fmt.Printf("\nERROR: setupYouTubeWebSub failed for channel %s: %v\n", channelID, err)
						fmt.Printf("WARNING: Continuing with approval but WebSub subscription may not be active\n")
					} else if subscribed != nil {
						ytPlatform = subscribed
						fmt.Printf("\nSUCCESS: WebSub setup completed, platform data updated\n")
						fmt.Printf("  WebSubSubscribed: %v\n", ytPlatform.WebSubSubscribed)
						fmt.Printf("  WebSubHubURL: %s\n", ytPlatform.WebSubHubURL)
						fmt.Printf("  WebSubTopicURL: %s\n", ytPlatform.WebSubTopicURL)
						fmt.Printf("  WebSubCallbackURL: %s\n", ytPlatform.WebSubCallbackURL)
					}
				} else {
					fmt.Printf("\nWARNING: No channel ID available for YouTube platform\n")
					fmt.Printf("WARNING: WebSub subscription will NOT be set up\n")
					fmt.Printf("WARNING: The streamer will be saved but alerts may not work\n")
				}
				record.Platforms.YouTube = ytPlatform
				fmt.Printf("\nINFO: YouTube platform configured in record\n")
				fmt.Printf("  Final ChannelID: %s\n", record.Platforms.YouTube.ChannelID)
				fmt.Printf("  Final WebSubSubscribed: %v\n", record.Platforms.YouTube.WebSubSubscribed)

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
			default:
				fmt.Printf("INFO: Platform %s not recognized, skipping\n", p)
			}
		}
	} else {
		fmt.Printf("\nWARNING: No platforms provided in submission\n")
	}

	fmt.Printf("\n--- Saving streamer record to store ---\n")
	saved, err := s.streamersStore.Append(record)
	if err != nil {
		fmt.Printf("\nERROR: Failed to save streamer record: %v\n", err)
		fmt.Printf("=== APPROVE SUBMISSION END (failed) ===\n\n")
		return err
	}

	// Verify the record was saved correctly
	fmt.Printf("\n*** STREAMER RECORD SAVED SUCCESSFULLY ***\n")
	fmt.Printf("  ID: %s\n", saved.Streamer.ID)
	fmt.Printf("  Alias: %s\n", saved.Streamer.Alias)
	fmt.Printf("  Description: %s\n", saved.Streamer.Description)
	fmt.Printf("  Languages: %v\n", saved.Streamer.Languages)
	if saved.Platforms.YouTube != nil {
		fmt.Printf("\n  YouTube Platform Details:\n")
		fmt.Printf("    Channel ID: %s\n", saved.Platforms.YouTube.ChannelID)
		fmt.Printf("    Channel URL: %s\n", saved.Platforms.YouTube.ChannelURL)
		fmt.Printf("    WebSub Subscribed: %v\n", saved.Platforms.YouTube.WebSubSubscribed)
		if saved.Platforms.YouTube.WebSubSubscribed {
			fmt.Printf("    WebSub Hub: %s\n", saved.Platforms.YouTube.WebSubHubURL)
			fmt.Printf("    WebSub Topic: %s\n", saved.Platforms.YouTube.WebSubTopicURL)
			fmt.Printf("    WebSub Callback: %s\n", saved.Platforms.YouTube.WebSubCallbackURL)
			fmt.Printf("    WebSub Secret: [%d chars]\n", len(saved.Platforms.YouTube.WebSubSecret))
			if saved.Platforms.YouTube.WebSubLeaseExpiry != nil {
				fmt.Printf("    WebSub Expiry: %s\n", saved.Platforms.YouTube.WebSubLeaseExpiry.Format("2006-01-02 15:04:05 MST"))
			}
		} else {
			fmt.Printf("    WARNING: WebSub is NOT subscribed - alerts may not work!\n")
		}
	} else {
		fmt.Printf("\n  No YouTube platform configured\n")
	}

	fmt.Printf("\n=== APPROVE SUBMISSION END (success) ===\n\n")
	return nil
}

// setupYouTubeWebSub sets up a WebSub subscription for a YouTube channel
func (s *SubmissionsService) setupYouTubeWebSub(ctx context.Context, channelID, channelURL string) (*streamers.YouTubePlatform, error) {
	fmt.Printf("=== setupYouTubeWebSub START ===\n")
	fmt.Printf("  Channel ID: %s\n", channelID)
	fmt.Printf("  Channel URL: %s\n", channelURL)
	fmt.Printf("  WebSub callback base URL: %s\n", s.websubCallbackBaseURL)

	if s.websubCallbackBaseURL == "" {
		fmt.Printf("WARNING: WebSub callback base URL not configured, skipping subscription for channel %s\n", channelID)
		fmt.Printf("INFO: Returning YouTubePlatform without WebSub subscription\n")
		fmt.Printf("=== setupYouTubeWebSub END (skipped) ===\n")
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
	fmt.Printf("  Lease Seconds: %d (default)\n", websub.DefaultLeaseSeconds)
	fmt.Printf("INFO: Calling websub.Subscribe...\n")

	// Subscribe to WebSub
	result, err := websub.Subscribe(websub.SubscriptionRequest{
		ChannelID:    channelID,
		CallbackURL:  callbackURL,
		LeaseSeconds: websub.DefaultLeaseSeconds,
	})
	if err != nil {
		fmt.Printf("ERROR: websub.Subscribe returned error: %v\n", err)
		fmt.Printf("=== setupYouTubeWebSub END (failed) ===\n")
		return nil, fmt.Errorf("subscribe to websub: %w", err)
	}

	fmt.Printf("SUCCESS: websub.Subscribe returned successfully\n")
	fmt.Printf("INFO: Subscription result details:\n")
	fmt.Printf("  Hub URL: %s\n", result.HubURL)
	fmt.Printf("  Topic URL: %s\n", result.TopicURL)
	fmt.Printf("  Callback URL: %s\n", result.CallbackURL)
	fmt.Printf("  Secret: [%d chars]\n", len(result.Secret))
	fmt.Printf("  Lease Expiry: %s\n", result.LeaseExpiry.Format("2006-01-02 15:04:05 MST"))

	ytPlatform := &streamers.YouTubePlatform{
		ChannelID:         channelID,
		ChannelURL:        channelURL,
		WebSubHubURL:      result.HubURL,
		WebSubTopicURL:    result.TopicURL,
		WebSubCallbackURL: result.CallbackURL,
		WebSubSecret:      result.Secret,
		WebSubLeaseExpiry: &result.LeaseExpiry,
		WebSubSubscribed:  true,
	}

	fmt.Printf("INFO: Created YouTubePlatform struct with WebSub details\n")
	fmt.Printf("  WebSubSubscribed: %v\n", ytPlatform.WebSubSubscribed)
	fmt.Printf("=== setupYouTubeWebSub END (success) ===\n")

	return ytPlatform, nil
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
