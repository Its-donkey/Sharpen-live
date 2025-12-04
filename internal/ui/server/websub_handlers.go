package server

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/api"
	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/websub"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
)

// handleYouTubeWebSub handles WebSub verification and notifications from YouTube
func (s *server) handleYouTubeWebSub(w http.ResponseWriter, r *http.Request) {
	// Handle verification requests (GET)
	if r.Method == http.MethodGet {
		s.handleWebSubVerification(w, r)
		return
	}

	// Handle notification requests (POST)
	if r.Method == http.MethodPost {
		s.handleWebSubNotification(w, r)
		return
	}

	w.Header().Set("Allow", "GET, POST")
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// handleWebSubVerification handles the WebSub verification challenge
func (s *server) handleWebSubVerification(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("\n=== WEBSUB VERIFICATION REQUEST START ===\n")
	fmt.Printf("Remote Address: %s\n", r.RemoteAddr)
	fmt.Printf("Request URL: %s\n", r.URL.String())
	fmt.Printf("Method: %s\n", r.Method)

	// Extract query parameters
	mode := r.URL.Query().Get("hub.mode")
	topic := r.URL.Query().Get("hub.topic")
	challenge := r.URL.Query().Get("hub.challenge")
	leaseSeconds := r.URL.Query().Get("hub.lease_seconds")
	verifyToken := r.URL.Query().Get("hub.verify_token")

	fmt.Printf("\nReceived Parameters:\n")
	fmt.Printf("  hub.mode: %s\n", mode)
	fmt.Printf("  hub.topic: %s\n", topic)
	fmt.Printf("  hub.challenge: [%d chars]\n", len(challenge))
	fmt.Printf("  hub.lease_seconds: %s\n", leaseSeconds)
	fmt.Printf("  hub.verify_token: %s\n", verifyToken)

	s.logger.Info("websub", "Received WebSub verification request", map[string]any{
		"mode":         mode,
		"topic":        topic,
		"leaseSeconds": leaseSeconds,
		"verifyToken":  verifyToken,
	})

	// Validate required parameters
	if mode == "" || topic == "" || challenge == "" {
		fmt.Printf("\nERROR: Missing required parameters\n")
		fmt.Printf("  mode empty: %v\n", mode == "")
		fmt.Printf("  topic empty: %v\n", topic == "")
		fmt.Printf("  challenge empty: %v\n", challenge == "")
		s.logger.Warn("websub", "Missing required verification parameters", map[string]any{
			"mode":      mode,
			"topic":     topic,
			"challenge": challenge != "",
		})
		fmt.Printf("=== WEBSUB VERIFICATION REQUEST END (failed - missing params) ===\n\n")
		http.Error(w, "missing required parameters", http.StatusBadRequest)
		return
	}

	// Extract channel ID from topic URL
	// Topic format: https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCxxxxxx
	channelID := extractChannelIDFromTopic(topic)
	fmt.Printf("\nExtracted Channel ID: %s\n", channelID)
	if channelID == "" {
		fmt.Printf("ERROR: Could not extract channel ID from topic\n")
		s.logger.Warn("websub", "Could not extract channel ID from topic", map[string]any{
			"topic": topic,
		})
		fmt.Printf("=== WEBSUB VERIFICATION REQUEST END (failed - invalid topic) ===\n\n")
		http.Error(w, "invalid topic URL", http.StatusBadRequest)
		return
	}

	// Verify that we have a streamer with this YouTube channel
	if s.streamersStore == nil {
		fmt.Printf("ERROR: Streamers store is nil\n")
		s.logger.Error("websub", "Streamers store not configured", errors.New("streamers store is nil"), nil)
		fmt.Printf("=== WEBSUB VERIFICATION REQUEST END (failed - store nil) ===\n\n")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	fmt.Printf("\nLooking up streamer with channel ID: %s\n", channelID)
	records, err := s.streamersStore.List()
	if err != nil {
		fmt.Printf("ERROR: Failed to list streamers: %v\n", err)
		s.logger.Error("websub", "Failed to list streamers", err, map[string]any{"channelId": channelID})
		fmt.Printf("=== WEBSUB VERIFICATION REQUEST END (failed - store error) ===\n\n")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	fmt.Printf("Found %d streamer records in store\n", len(records))

	// Find streamer with matching YouTube channel
	found := false
	var foundStreamer string
	for _, record := range records {
		if record.Platforms.YouTube != nil {
			fmt.Printf("  Checking streamer %s (alias: %s) - YouTube channel: %s\n",
				record.Streamer.ID, record.Streamer.Alias, record.Platforms.YouTube.ChannelID)
			if record.Platforms.YouTube.ChannelID == channelID {
				found = true
				foundStreamer = record.Streamer.Alias
				fmt.Printf("  ✓ MATCH FOUND!\n")
				s.logger.Info("websub", "Verified subscription for streamer", map[string]any{
					"streamerId": record.Streamer.ID,
					"alias":      record.Streamer.Alias,
					"channelId":  channelID,
					"mode":       mode,
				})
				break
			}
		} else {
			fmt.Printf("  Streamer %s (alias: %s) - No YouTube platform\n",
				record.Streamer.ID, record.Streamer.Alias)
		}
	}

	if !found {
		fmt.Printf("\nWARNING: No streamer found for channel %s\n", channelID)
		fmt.Printf("This verification request may be for a subscription that was created before the streamer was approved\n")
		s.logger.Warn("websub", "No streamer found for channel", map[string]any{
			"channelId": channelID,
		})
		// Still respond with challenge to avoid breaking existing subscriptions
		// but log the warning for investigation
	} else {
		fmt.Printf("\nSUCCESS: Found streamer '%s' for channel %s\n", foundStreamer, channelID)
	}

	// Respond with the challenge to confirm the subscription
	fmt.Printf("\nResponding with challenge (length: %d)\n", len(challenge))
	fmt.Printf("Response: 200 OK\n")
	fmt.Printf("=== WEBSUB VERIFICATION REQUEST END (success) ===\n\n")
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(challenge))
}

// handleWebSubNotification handles incoming WebSub notifications
func (s *server) handleWebSubNotification(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("\n=== WEBSUB NOTIFICATION START ===\n")
	fmt.Printf("Remote Address: %s\n", r.RemoteAddr)
	fmt.Printf("Content-Type: %s\n", r.Header.Get("Content-Type"))

	// Read the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("ERROR: Failed to read notification body: %v\n", err)
		s.logger.Error("websub", "Failed to read notification body", err, map[string]any{"contentType": r.Header.Get("Content-Type")})
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	fmt.Printf("INFO: Received notification body (%d bytes)\n", len(body))

	// Get signature from header
	signature := r.Header.Get("X-Hub-Signature")
	if signature == "" {
		// Some hubs use different header names
		signature = r.Header.Get("X-Hub-Signature-256")
	}
	fmt.Printf("INFO: Signature present: %v\n", signature != "")

	s.logger.Info("websub", "Received WebSub notification", map[string]any{
		"contentType":   r.Header.Get("Content-Type"),
		"contentLength": len(body),
		"hasSignature":  signature != "",
	})

	// Parse the Atom feed
	feed, err := parseAtomFeed(body)
	if err != nil {
		fmt.Printf("ERROR: Failed to parse Atom feed: %v\n", err)
		s.logger.Warn("websub", "Failed to parse Atom feed", map[string]any{"error": err.Error()})
		// Still accept to avoid retries
		w.WriteHeader(http.StatusOK)
		return
	}

	channelID := feed.ChannelID
	if channelID == "" {
		fmt.Printf("WARNING: No channel ID in Atom feed\n")
		s.logger.Warn("websub", "Could not extract channel ID from notification", nil)
		w.WriteHeader(http.StatusOK)
		return
	}

	fmt.Printf("INFO: Channel ID from feed: %s\n", channelID)
	if feed.VideoID != "" {
		fmt.Printf("INFO: Video ID from feed: %s\n", feed.VideoID)
		fmt.Printf("INFO: Video published at: %s\n", feed.Published.Format("2006-01-02 15:04:05 MST"))
	}

	// Find the streamer across all sites - WebSub notifications are shared across all sites
	// so we need to check all streamer stores, not just the current site's
	stores := s.getAllStreamerStores()
	fmt.Printf("INFO: Searching across %d site(s) for channel ID: %s\n", len(stores), channelID)

	found := false
	for siteKey, store := range stores {
		records, err := store.List()
		if err != nil {
			fmt.Printf("WARNING: Failed to list streamers for site %s: %v\n", siteKey, err)
			s.logger.Warn("websub", "Failed to list streamers for site", map[string]any{
				"site":      siteKey,
				"channelId": channelID,
				"error":     err.Error(),
			})
			continue
		}

		fmt.Printf("INFO: Checking %d streamer(s) in site '%s'\n", len(records), siteKey)
		for _, record := range records {
			if record.Platforms.YouTube != nil && record.Platforms.YouTube.ChannelID == channelID {
				fmt.Printf("INFO: Found matching streamer: %s (ID: %s) in site '%s'\n", record.Streamer.Alias, record.Streamer.ID, siteKey)

				// Verify signature if we have a secret
				if record.Platforms.YouTube.WebSubSecret != "" && signature != "" {
					if !websub.VerifySignature(body, signature, record.Platforms.YouTube.WebSubSecret) {
						fmt.Printf("WARNING: Signature verification failed for site '%s', checking other sites\n", siteKey)
						s.logger.Warn("websub", "Signature verification failed, continuing search", map[string]any{
							"streamerId": record.Streamer.ID,
							"channelId":  channelID,
							"site":       siteKey,
						})
						// Continue to check other sites - the streamer might exist in multiple sites
						// with different secrets, and we need to find the one with the matching secret
						continue
					}
					fmt.Printf("INFO: Signature verified successfully for site '%s'\n", siteKey)
				}

				found = true

				s.logger.Info("websub", "Processing notification for streamer", map[string]any{
					"streamerId": record.Streamer.ID,
					"alias":      record.Streamer.Alias,
					"channelId":  channelID,
					"videoId":    feed.VideoID,
					"site":       siteKey,
				})

				// Check live status using YouTube API
				if feed.VideoID != "" {
					fmt.Printf("\nINFO: Checking live status for video %s\n", feed.VideoID)
					// Use the store from the site where we found the streamer
					if err := s.checkAndUpdateLiveStatusWithStore(r.Context(), store, channelID, feed.VideoID, feed.Published); err != nil {
						fmt.Printf("WARNING: Failed to check/update live status: %v\n", err)
						s.logger.Warn("websub", "Failed to update live status", map[string]any{
							"error":     err.Error(),
							"channelId": channelID,
							"videoId":   feed.VideoID,
							"site":      siteKey,
						})
					}
				}

				break
			}
		}

		if found {
			break
		}
	}

	if !found {
		fmt.Printf("WARNING: No streamer found with valid signature for channel ID: %s across all sites\n", channelID)
		fmt.Printf("INFO: Checking if streamer exists without signature verification\n")

		// If signature verification failed for all sites, try processing without verification
		// This handles cases where the stored secret doesn't match the subscription secret
		for siteKey, store := range stores {
			records, err := store.List()
			if err != nil {
				continue
			}

			for _, record := range records {
				if record.Platforms.YouTube != nil && record.Platforms.YouTube.ChannelID == channelID {
					fmt.Printf("WARNING: Processing notification for %s without signature verification (site: %s)\n", record.Streamer.Alias, siteKey)
					s.logger.Warn("websub", "Processing without signature verification", map[string]any{
						"streamerId": record.Streamer.ID,
						"alias":      record.Streamer.Alias,
						"channelId":  channelID,
						"site":       siteKey,
						"reason":     "Signature verification failed for all sites",
					})

					// Check live status using YouTube API
					if feed.VideoID != "" {
						fmt.Printf("\nINFO: Checking live status for video %s\n", feed.VideoID)
						if err := s.checkAndUpdateLiveStatusWithStore(r.Context(), store, channelID, feed.VideoID, feed.Published); err != nil {
							fmt.Printf("WARNING: Failed to check/update live status: %v\n", err)
							s.logger.Warn("websub", "Failed to update live status", map[string]any{
								"error":     err.Error(),
								"channelId": channelID,
								"videoId":   feed.VideoID,
								"site":      siteKey,
							})
						}
					}

					found = true
					break
				}
			}

			if found {
				break
			}
		}

		if !found {
			fmt.Printf("WARNING: No streamer found for channel ID: %s across all sites\n", channelID)
			s.logger.Warn("websub", "No streamer found for channel", map[string]any{
				"channelId": channelID,
				"videoId":   feed.VideoID,
			})
		}
	}

	fmt.Printf("INFO: Responding with 200 OK\n")
	fmt.Printf("=== WEBSUB NOTIFICATION END ===\n\n")

	// Respond with 200 OK
	w.WriteHeader(http.StatusOK)
}

// atomFeed represents the YouTube Atom feed structure
type atomFeed struct {
	XMLName xml.Name `xml:"feed"`
	Entry   struct {
		ChannelID string    `xml:"http://www.youtube.com/xml/schemas/2015 channelId"`
		VideoID   string    `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
		Published time.Time `xml:"published"`
		Updated   time.Time `xml:"updated"`
		Title     string    `xml:"title"`
	} `xml:"entry"`
}

// atomFeedInfo contains extracted information from the Atom feed
type atomFeedInfo struct {
	ChannelID string
	VideoID   string
	Published time.Time
	Updated   time.Time
	Title     string
}

// parseAtomFeed parses a YouTube WebSub Atom feed notification
func parseAtomFeed(body []byte) (atomFeedInfo, error) {
	var feed atomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return atomFeedInfo{}, fmt.Errorf("parse XML: %w", err)
	}

	return atomFeedInfo{
		ChannelID: strings.TrimSpace(feed.Entry.ChannelID),
		VideoID:   strings.TrimSpace(feed.Entry.VideoID),
		Published: feed.Entry.Published,
		Updated:   feed.Entry.Updated,
		Title:     strings.TrimSpace(feed.Entry.Title),
	}, nil
}

// checkAndUpdateLiveStatus checks if a video is live and updates the streamer status
func (s *server) checkAndUpdateLiveStatus(ctx context.Context, channelID, videoID string, publishedAt time.Time) error {
	if s.streamersStore == nil {
		return fmt.Errorf("streamers store not configured")
	}

	// Type assert to concrete store to access status update methods
	store, ok := s.streamersStore.(*streamers.Store)
	if !ok {
		return fmt.Errorf("streamers store is not a *streamers.Store")
	}

	// Skip YouTube API calls if API key is not configured
	if s.youtubeConfig.APIKey == "" {
		fmt.Printf("WARNING: YouTube API key not configured, skipping live status check for video %s\n", videoID)
		s.logger.Warn("websub", "YouTube API key not configured, cannot verify live status", map[string]any{
			"channelId": channelID,
			"videoId":   videoID,
		})
		return nil
	}

	fmt.Printf("INFO: Checking if video %s is a live stream\n", videoID)

	// Use YouTube API to check if the channel is currently live
	searchClient := api.SearchClient{
		APIKey:     s.youtubeConfig.APIKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	result, err := searchClient.LiveNow(ctx, channelID)
	if err != nil {
		return fmt.Errorf("check live status: %w", err)
	}

	// Update status based on whether the channel is live
	if result.VideoID != "" && result.VideoID == videoID {
		// The video from the notification is currently live
		fmt.Printf("SUCCESS: Video %s is LIVE\n", videoID)
		fmt.Printf("  Started at: %s\n", result.StartedAt.Format("2006-01-02 15:04:05 MST"))

		_, err := store.SetYouTubeLive(channelID, videoID, result.StartedAt)
		if err != nil {
			return fmt.Errorf("set live status: %w", err)
		}
		fmt.Printf("INFO: Updated streamer status to LIVE\n")
	} else if result.VideoID != "" {
		// Channel is live but with a different video
		fmt.Printf("INFO: Channel is live with different video: %s (notification was for %s)\n", result.VideoID, videoID)

		_, err := store.SetYouTubeLive(channelID, result.VideoID, result.StartedAt)
		if err != nil {
			return fmt.Errorf("set live status: %w", err)
		}
		fmt.Printf("INFO: Updated streamer status to LIVE with current video\n")
	} else {
		// Channel is not currently live - video may have ended or not started yet
		fmt.Printf("INFO: Channel is not currently live (video %s may have ended or not started)\n", videoID)

		_, err := store.ClearYouTubeLive(channelID)
		if err != nil {
			return fmt.Errorf("clear live status: %w", err)
		}
		fmt.Printf("INFO: Updated streamer status to OFFLINE\n")
	}

	return nil
}

// extractChannelIDFromTopic extracts the channel ID from a WebSub topic URL
func extractChannelIDFromTopic(topic string) string {
	// Topic format: https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCxxxxxx
	if idx := strings.Index(topic, "channel_id="); idx != -1 {
		channelID := topic[idx+11:]
		// Remove any trailing query parameters
		if ampIdx := strings.Index(channelID, "&"); ampIdx != -1 {
			channelID = channelID[:ampIdx]
		}
		return channelID
	}
	return ""
}

// extractChannelIDFromAtomFeed extracts the channel ID from an Atom feed (fallback method)
func extractChannelIDFromAtomFeed(body []byte) string {
	// Look for <yt:channelId>UCxxxxxx</yt:channelId> in the feed
	content := string(body)
	start := strings.Index(content, "<yt:channelId>")
	if start == -1 {
		return ""
	}
	start += len("<yt:channelId>")
	end := strings.Index(content[start:], "</yt:channelId>")
	if end == -1 {
		return ""
	}
	return content[start : start+end]
}

// checkAllStreamersLiveStatus checks the live status of all streamers with YouTube channels
func (s *server) checkAllStreamersLiveStatus(ctx context.Context) {
	fmt.Printf("\n=== INITIAL LIVE STATUS CHECK START ===\n")

	if s.streamersStore == nil {
		fmt.Printf("WARNING: Streamers store is nil, skipping initial status check\n")
		fmt.Printf("=== INITIAL LIVE STATUS CHECK END ===\n\n")
		return
	}

	// Get concrete store for status update methods
	store, ok := s.streamersStore.(*streamers.Store)
	if !ok {
		fmt.Printf("WARNING: Streamers store is not *streamers.Store, skipping initial status check\n")
		fmt.Printf("=== INITIAL LIVE STATUS CHECK END ===\n\n")
		return
	}

	if s.youtubeConfig.APIKey == "" {
		fmt.Printf("WARNING: YouTube API key not configured, skipping initial status check\n")
		fmt.Printf("=== INITIAL LIVE STATUS CHECK END ===\n\n")
		return
	}

	records, err := s.streamersStore.List()
	if err != nil {
		fmt.Printf("ERROR: Failed to list streamers: %v\n", err)
		s.logger.Error("startup", "Failed to list streamers for initial status check", err, nil)
		fmt.Printf("=== INITIAL LIVE STATUS CHECK END ===\n\n")
		return
	}

	fmt.Printf("INFO: Checking live status for %d streamer(s)\n", len(records))

	searchClient := api.SearchClient{
		APIKey:     s.youtubeConfig.APIKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	checkedCount := 0
	liveCount := 0
	errorCount := 0

	for _, record := range records {
		if record.Platforms.YouTube == nil || record.Platforms.YouTube.ChannelID == "" {
			continue
		}

		channelID := record.Platforms.YouTube.ChannelID
		fmt.Printf("\nINFO: Checking streamer %s (alias: %s) - YouTube channel: %s\n",
			record.Streamer.ID, record.Streamer.Alias, channelID)
		checkedCount++

		// Use individual timeout for each API call to prevent cascading failures
		// Timeout is slightly longer than HTTP client timeout (30s) to allow graceful completion
		callCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
		result, err := searchClient.LiveNow(callCtx, channelID)
		cancel()

		if err != nil {
			fmt.Printf("WARNING: Failed to check live status for %s: %v\n", record.Streamer.Alias, err)
			s.logger.Warn("startup", "Failed to check initial live status", map[string]any{
				"streamerId": record.Streamer.ID,
				"alias":      record.Streamer.Alias,
				"channelId":  channelID,
				"error":      err.Error(),
			})
			errorCount++
			continue
		}

		if result.VideoID != "" {
			// Channel is live
			fmt.Printf("SUCCESS: Streamer %s is LIVE with video %s\n", record.Streamer.Alias, result.VideoID)
			fmt.Printf("  Started at: %s\n", result.StartedAt.Format("2006-01-02 15:04:05 MST"))

			_, err := store.SetYouTubeLive(channelID, result.VideoID, result.StartedAt)
			if err != nil {
				fmt.Printf("ERROR: Failed to set live status for %s: %v\n", record.Streamer.Alias, err)
				s.logger.Error("startup", "Failed to set initial live status", err, map[string]any{
					"streamerId": record.Streamer.ID,
					"alias":      record.Streamer.Alias,
					"channelId":  channelID,
					"videoId":    result.VideoID,
				})
				errorCount++
			} else {
				liveCount++
			}
		} else {
			// Channel is offline
			fmt.Printf("INFO: Streamer %s is OFFLINE\n", record.Streamer.Alias)

			_, err := store.ClearYouTubeLive(channelID)
			if err != nil {
				fmt.Printf("ERROR: Failed to clear live status for %s: %v\n", record.Streamer.Alias, err)
				s.logger.Error("startup", "Failed to clear initial live status", err, map[string]any{
					"streamerId": record.Streamer.ID,
					"alias":      record.Streamer.Alias,
					"channelId":  channelID,
				})
				errorCount++
			}
		}
	}

	fmt.Printf("\n=== INITIAL LIVE STATUS CHECK COMPLETE ===\n")
	fmt.Printf("  Total streamers checked: %d\n", checkedCount)
	fmt.Printf("  Currently live: %d\n", liveCount)
	fmt.Printf("  Errors: %d\n", errorCount)
	fmt.Printf("===========================================\n\n")

	s.logger.Info("startup", "Initial live status check complete", map[string]any{
		"checked": checkedCount,
		"live":    liveCount,
		"errors":  errorCount,
	})
}

// getAllStreamerStores returns all streamer stores across all sites
func (s *server) getAllStreamerStores() map[string]*streamers.Store {
	stores := make(map[string]*streamers.Store)

	// Add the current site's store
	if baseStore, ok := s.streamersStore.(*streamers.Store); ok {
		stores[s.siteKey] = baseStore
	}

	// Scan data directory for other site stores
	dataDir := filepath.Dir(s.streamersStore.Path())
	parentDir := filepath.Dir(dataDir)

	entries, err := os.ReadDir(parentDir)
	if err != nil {
		fmt.Printf("WARNING: Could not scan data directory: %v\n", err)
		return stores
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		siteKey := entry.Name()
		if siteKey == s.siteKey {
			continue // Already added
		}

		streamersPath := filepath.Join(parentDir, siteKey, "streamers.json")
		if _, err := os.Stat(streamersPath); err == nil {
			stores[siteKey] = streamers.NewStore(streamersPath)
		}
	}

	return stores
}

// checkAndUpdateLiveStatusWithStore checks if a video is live and updates the streamer status using a specific store
func (s *server) checkAndUpdateLiveStatusWithStore(ctx context.Context, store *streamers.Store, channelID, videoID string, publishedAt time.Time) error {
	if store == nil {
		return fmt.Errorf("store is nil")
	}

	// Skip YouTube API calls if API key is not configured
	if s.youtubeConfig.APIKey == "" {
		fmt.Printf("WARNING: YouTube API key not configured, skipping live status check for video %s\n", videoID)
		s.logger.Warn("websub", "YouTube API key not configured, cannot verify live status", map[string]any{
			"channelId": channelID,
			"videoId":   videoID,
		})
		return nil
	}

	fmt.Printf("INFO: Checking if video %s is a live stream\n", videoID)

	// Use YouTube API to check if the channel is currently live
	searchClient := api.SearchClient{
		APIKey:     s.youtubeConfig.APIKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	result, err := searchClient.LiveNow(ctx, channelID)
	if err != nil {
		return fmt.Errorf("check live status: %w", err)
	}

	// Update status based on whether the channel is live
	if result.VideoID != "" && result.VideoID == videoID {
		// The video from the notification is currently live
		fmt.Printf("SUCCESS: Video %s is LIVE\n", videoID)
		fmt.Printf("  Started at: %s\n", result.StartedAt.Format("2006-01-02 15:04:05 MST"))

		_, err := store.SetYouTubeLive(channelID, videoID, result.StartedAt)
		if err != nil {
			return fmt.Errorf("set live status: %w", err)
		}
		fmt.Printf("INFO: Updated streamer status to LIVE\n")
	} else if result.VideoID != "" {
		// Channel is live but with a different video
		fmt.Printf("INFO: Channel is live with different video: %s (notification was for %s)\n", result.VideoID, videoID)

		_, err := store.SetYouTubeLive(channelID, result.VideoID, result.StartedAt)
		if err != nil {
			return fmt.Errorf("set live status: %w", err)
		}
		fmt.Printf("INFO: Updated streamer status to LIVE with current video\n")
	} else {
		// Channel is not currently live - video may have ended or not started yet
		fmt.Printf("INFO: Channel is not currently live (video %s may have ended or not started)\n", videoID)

		_, err := store.ClearYouTubeLive(channelID)
		if err != nil {
			return fmt.Errorf("clear live status: %w", err)
		}
		fmt.Printf("INFO: Updated streamer status to OFFLINE\n")
	}

	return nil
}
