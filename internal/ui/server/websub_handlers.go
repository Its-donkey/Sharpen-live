package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/websub"
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
	// Read the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("websub", "Failed to read notification body", err, map[string]any{"contentType": r.Header.Get("Content-Type")})
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Get signature from header
	signature := r.Header.Get("X-Hub-Signature")
	if signature == "" {
		// Some hubs use different header names
		signature = r.Header.Get("X-Hub-Signature-256")
	}

	s.logger.Info("websub", "Received WebSub notification", map[string]any{
		"contentType":   r.Header.Get("Content-Type"),
		"contentLength": len(body),
		"hasSignature":  signature != "",
	})

	// Extract channel ID from the notification body (it's in Atom feed format)
	channelID := extractChannelIDFromAtomFeed(body)
	if channelID == "" {
		s.logger.Warn("websub", "Could not extract channel ID from notification", nil)
		// Still accept the notification with 200 OK to avoid retries
		w.WriteHeader(http.StatusOK)
		return
	}

	// Find the streamer and verify signature
	if s.streamersStore != nil {
		records, err := s.streamersStore.List()
		if err != nil {
			s.logger.Error("websub", "Failed to list streamers", err, map[string]any{"channelId": channelID})
		} else {
			for _, record := range records {
				if record.Platforms.YouTube != nil && record.Platforms.YouTube.ChannelID == channelID {
					// Verify signature if we have a secret
					if record.Platforms.YouTube.WebSubSecret != "" && signature != "" {
						if !websub.VerifySignature(body, signature, record.Platforms.YouTube.WebSubSecret) {
							s.logger.Warn("websub", "Signature verification failed", map[string]any{
								"streamerId": record.Streamer.ID,
								"channelId":  channelID,
							})
							http.Error(w, "invalid signature", http.StatusUnauthorized)
							return
						}
					}

					s.logger.Info("websub", "Processing notification for streamer", map[string]any{
						"streamerId": record.Streamer.ID,
						"alias":      record.Streamer.Alias,
						"channelId":  channelID,
					})

					// TODO: Parse the Atom feed and update streamer status
					// For now, just log that we received it
					break
				}
			}
		}
	}

	// Respond with 200 OK
	w.WriteHeader(http.StatusOK)
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

// extractChannelIDFromAtomFeed extracts the channel ID from an Atom feed
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
