package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
)

// EventSub message types
const (
	eventsubMessageTypeVerification = "webhook_callback_verification"
	eventsubMessageTypeNotification = "notification"
	eventsubMessageTypeRevocation   = "revocation"
)

// EventSub event types
const (
	eventTypeStreamOnline  = "stream.online"
	eventTypeStreamOffline = "stream.offline"
)

// EventSubHeaders contains the standard Twitch EventSub headers
type EventSubHeaders struct {
	MessageID        string
	MessageRetry     string
	MessageType      string
	MessageSignature string
	MessageTimestamp string
	SubscriptionType string
}

// EventSubVerification represents the webhook verification payload
type EventSubVerification struct {
	Challenge    string                 `json:"challenge"`
	Subscription EventSubSubscriptionInfo `json:"subscription"`
}

// EventSubSubscriptionInfo contains subscription details from notifications
type EventSubSubscriptionInfo struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	Type      string            `json:"type"`
	Version   string            `json:"version"`
	Condition map[string]string `json:"condition"`
	Transport struct {
		Method   string `json:"method"`
		Callback string `json:"callback"`
	} `json:"transport"`
	CreatedAt time.Time `json:"created_at"`
	Cost      int       `json:"cost"`
}

// EventSubNotification represents a notification payload
type EventSubNotification struct {
	Subscription EventSubSubscriptionInfo `json:"subscription"`
	Event        json.RawMessage          `json:"event"`
}

// StreamOnlineEvent represents the stream.online event payload
type StreamOnlineEvent struct {
	ID                   string    `json:"id"`
	BroadcasterUserID    string    `json:"broadcaster_user_id"`
	BroadcasterUserLogin string    `json:"broadcaster_user_login"`
	BroadcasterUserName  string    `json:"broadcaster_user_name"`
	Type                 string    `json:"type"`
	StartedAt            time.Time `json:"started_at"`
}

// StreamOfflineEvent represents the stream.offline event payload
type StreamOfflineEvent struct {
	BroadcasterUserID    string `json:"broadcaster_user_id"`
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
}

// handleTwitchEventSub handles Twitch EventSub webhook callbacks
func (s *server) handleTwitchEventSub(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("\n=== TWITCH EVENTSUB REQUEST START ===\n")
	fmt.Printf("Remote Address: %s\n", r.RemoteAddr)
	fmt.Printf("Method: %s\n", r.Method)

	// Read headers
	headers := EventSubHeaders{
		MessageID:        r.Header.Get("Twitch-Eventsub-Message-Id"),
		MessageRetry:     r.Header.Get("Twitch-Eventsub-Message-Retry"),
		MessageType:      r.Header.Get("Twitch-Eventsub-Message-Type"),
		MessageSignature: r.Header.Get("Twitch-Eventsub-Message-Signature"),
		MessageTimestamp: r.Header.Get("Twitch-Eventsub-Message-Timestamp"),
		SubscriptionType: r.Header.Get("Twitch-Eventsub-Subscription-Type"),
	}

	fmt.Printf("Message ID: %s\n", headers.MessageID)
	fmt.Printf("Message Type: %s\n", headers.MessageType)
	fmt.Printf("Subscription Type: %s\n", headers.SubscriptionType)

	s.logger.Info("twitch-eventsub", "Received EventSub request", map[string]any{
		"messageId":        headers.MessageID,
		"messageType":      headers.MessageType,
		"subscriptionType": headers.SubscriptionType,
	})

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("ERROR: Failed to read body: %v\n", err)
		s.logger.Error("twitch-eventsub", "Failed to read request body", err, nil)
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	fmt.Printf("Body length: %d bytes\n", len(body))

	// Verify signature
	if !s.verifyEventSubSignature(headers, body) {
		fmt.Printf("ERROR: Signature verification failed\n")
		s.logger.Warn("twitch-eventsub", "Signature verification failed", map[string]any{
			"messageId": headers.MessageID,
		})
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	fmt.Printf("INFO: Signature verified successfully\n")

	// Handle based on message type
	switch headers.MessageType {
	case eventsubMessageTypeVerification:
		s.handleEventSubVerification(w, body)
	case eventsubMessageTypeNotification:
		s.handleEventSubNotification(w, headers, body)
	case eventsubMessageTypeRevocation:
		s.handleEventSubRevocation(w, headers, body)
	default:
		fmt.Printf("WARNING: Unknown message type: %s\n", headers.MessageType)
		s.logger.Warn("twitch-eventsub", "Unknown message type", map[string]any{
			"messageType": headers.MessageType,
		})
		w.WriteHeader(http.StatusOK)
	}

	fmt.Printf("=== TWITCH EVENTSUB REQUEST END ===\n\n")
}

// verifyEventSubSignature verifies the HMAC-SHA256 signature from Twitch
func (s *server) verifyEventSubSignature(headers EventSubHeaders, body []byte) bool {
	secret := s.getTwitchEventSubSecret()
	if secret == "" {
		fmt.Printf("WARNING: No EventSub secret configured, skipping signature verification\n")
		s.logger.Warn("twitch-eventsub", "No EventSub secret configured", nil)
		return false
	}

	// Build the message to sign: message_id + message_timestamp + body
	message := headers.MessageID + headers.MessageTimestamp + string(body)

	// Compute HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Compare signatures
	return hmac.Equal([]byte(expectedSig), []byte(headers.MessageSignature))
}

// getTwitchEventSubSecret returns the EventSub secret from configuration
func (s *server) getTwitchEventSubSecret() string {
	// Try to load from config file
	if s.configPath != "" {
		data, err := readTwitchConfigFromFile(s.configPath)
		if err == nil && data.EventSubSecret != "" {
			return data.EventSubSecret
		}
	}
	return ""
}

// twitchConfigData holds Twitch configuration
type twitchConfigData struct {
	EventSubSecret string `json:"eventsub_secret"`
}

// readTwitchConfigFromFile reads Twitch config from the config file
func readTwitchConfigFromFile(configPath string) (twitchConfigData, error) {
	type platformsBlock struct {
		Twitch *twitchConfigData `json:"twitch"`
	}
	type configFile struct {
		Platforms *platformsBlock `json:"platforms"`
	}

	data, err := io.ReadAll(mustOpen(configPath))
	if err != nil {
		return twitchConfigData{}, err
	}

	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return twitchConfigData{}, err
	}

	if cfg.Platforms != nil && cfg.Platforms.Twitch != nil {
		return *cfg.Platforms.Twitch, nil
	}

	return twitchConfigData{}, fmt.Errorf("twitch config not found")
}

func mustOpen(path string) io.Reader {
	f, err := http.Dir(".").Open(path)
	if err != nil {
		return strings.NewReader("")
	}
	return f
}

// handleEventSubVerification handles the webhook verification challenge
func (s *server) handleEventSubVerification(w http.ResponseWriter, body []byte) {
	fmt.Printf("INFO: Handling webhook verification challenge\n")

	var verification EventSubVerification
	if err := json.Unmarshal(body, &verification); err != nil {
		fmt.Printf("ERROR: Failed to parse verification payload: %v\n", err)
		s.logger.Error("twitch-eventsub", "Failed to parse verification payload", err, nil)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	fmt.Printf("INFO: Subscription ID: %s\n", verification.Subscription.ID)
	fmt.Printf("INFO: Subscription Type: %s\n", verification.Subscription.Type)
	fmt.Printf("INFO: Challenge: %s\n", verification.Challenge)

	s.logger.Info("twitch-eventsub", "Responding to verification challenge", map[string]any{
		"subscriptionId":   verification.Subscription.ID,
		"subscriptionType": verification.Subscription.Type,
	})

	// Respond with the challenge
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(verification.Challenge))
}

// handleEventSubNotification handles stream event notifications
func (s *server) handleEventSubNotification(w http.ResponseWriter, headers EventSubHeaders, body []byte) {
	fmt.Printf("INFO: Handling notification for %s\n", headers.SubscriptionType)

	var notification EventSubNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		fmt.Printf("ERROR: Failed to parse notification payload: %v\n", err)
		s.logger.Error("twitch-eventsub", "Failed to parse notification payload", err, nil)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	fmt.Printf("INFO: Subscription ID: %s\n", notification.Subscription.ID)
	fmt.Printf("INFO: Event Type: %s\n", notification.Subscription.Type)

	switch notification.Subscription.Type {
	case eventTypeStreamOnline:
		s.handleStreamOnline(notification)
	case eventTypeStreamOffline:
		s.handleStreamOffline(notification)
	default:
		fmt.Printf("INFO: Unhandled event type: %s\n", notification.Subscription.Type)
		s.logger.Info("twitch-eventsub", "Unhandled event type", map[string]any{
			"eventType": notification.Subscription.Type,
		})
	}

	// Always respond with 200 OK to acknowledge receipt
	w.WriteHeader(http.StatusOK)
}

// handleStreamOnline processes stream.online events
func (s *server) handleStreamOnline(notification EventSubNotification) {
	var event StreamOnlineEvent
	if err := json.Unmarshal(notification.Event, &event); err != nil {
		fmt.Printf("ERROR: Failed to parse stream.online event: %v\n", err)
		s.logger.Error("twitch-eventsub", "Failed to parse stream.online event", err, nil)
		return
	}

	fmt.Printf("INFO: Stream ONLINE for broadcaster %s (%s)\n", event.BroadcasterUserName, event.BroadcasterUserID)
	fmt.Printf("INFO: Stream ID: %s\n", event.ID)
	fmt.Printf("INFO: Started at: %s\n", event.StartedAt.Format("2006-01-02 15:04:05 MST"))

	s.logger.Info("twitch-eventsub", "Stream went online", map[string]any{
		"broadcasterID":    event.BroadcasterUserID,
		"broadcasterLogin": event.BroadcasterUserLogin,
		"broadcasterName":  event.BroadcasterUserName,
		"streamID":         event.ID,
		"startedAt":        event.StartedAt,
	})

	// Find and update streamer across all sites
	stores := s.getAllStreamerStores()
	found := false
	for siteKey, store := range stores {
		_, err := store.SetTwitchLive(event.BroadcasterUserID, event.ID, event.StartedAt)
		if err == nil {
			found = true
			fmt.Printf("SUCCESS: Updated streamer status to LIVE in site '%s'\n", siteKey)
			s.logger.Info("twitch-eventsub", "Updated streamer to live", map[string]any{
				"broadcasterID": event.BroadcasterUserID,
				"site":          siteKey,
			})
			break
		}
		if !strings.Contains(err.Error(), "not found") {
			fmt.Printf("ERROR: Failed to update status in site '%s': %v\n", siteKey, err)
		}
	}

	if !found {
		fmt.Printf("WARNING: No streamer found for Twitch broadcaster %s\n", event.BroadcasterUserID)
		s.logger.Warn("twitch-eventsub", "No streamer found for broadcaster", map[string]any{
			"broadcasterID": event.BroadcasterUserID,
		})
	}
}

// handleStreamOffline processes stream.offline events
func (s *server) handleStreamOffline(notification EventSubNotification) {
	var event StreamOfflineEvent
	if err := json.Unmarshal(notification.Event, &event); err != nil {
		fmt.Printf("ERROR: Failed to parse stream.offline event: %v\n", err)
		s.logger.Error("twitch-eventsub", "Failed to parse stream.offline event", err, nil)
		return
	}

	fmt.Printf("INFO: Stream OFFLINE for broadcaster %s (%s)\n", event.BroadcasterUserName, event.BroadcasterUserID)

	s.logger.Info("twitch-eventsub", "Stream went offline", map[string]any{
		"broadcasterID":    event.BroadcasterUserID,
		"broadcasterLogin": event.BroadcasterUserLogin,
		"broadcasterName":  event.BroadcasterUserName,
	})

	// Find and update streamer across all sites
	stores := s.getAllStreamerStores()
	found := false
	for siteKey, store := range stores {
		_, err := store.ClearTwitchLive(event.BroadcasterUserID)
		if err == nil {
			found = true
			fmt.Printf("SUCCESS: Updated streamer status to OFFLINE in site '%s'\n", siteKey)
			s.logger.Info("twitch-eventsub", "Updated streamer to offline", map[string]any{
				"broadcasterID": event.BroadcasterUserID,
				"site":          siteKey,
			})
			break
		}
		if !strings.Contains(err.Error(), "not found") {
			fmt.Printf("ERROR: Failed to update status in site '%s': %v\n", siteKey, err)
		}
	}

	if !found {
		fmt.Printf("WARNING: No streamer found for Twitch broadcaster %s\n", event.BroadcasterUserID)
		s.logger.Warn("twitch-eventsub", "No streamer found for broadcaster", map[string]any{
			"broadcasterID": event.BroadcasterUserID,
		})
	}
}

// handleEventSubRevocation handles subscription revocation notifications
func (s *server) handleEventSubRevocation(w http.ResponseWriter, headers EventSubHeaders, body []byte) {
	fmt.Printf("INFO: Handling subscription revocation\n")

	var notification EventSubNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		fmt.Printf("ERROR: Failed to parse revocation payload: %v\n", err)
		s.logger.Error("twitch-eventsub", "Failed to parse revocation payload", err, nil)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	fmt.Printf("INFO: Subscription %s has been revoked\n", notification.Subscription.ID)
	fmt.Printf("INFO: Revocation reason: %s\n", notification.Subscription.Status)

	s.logger.Warn("twitch-eventsub", "Subscription revoked", map[string]any{
		"subscriptionId": notification.Subscription.ID,
		"reason":         notification.Subscription.Status,
		"type":           notification.Subscription.Type,
	})

	// Update the streamer's EventSub status if we can identify them
	broadcasterID := notification.Subscription.Condition["broadcaster_user_id"]
	if broadcasterID != "" {
		stores := s.getAllStreamerStores()
		for siteKey, store := range stores {
			record, err := store.GetByTwitchBroadcasterID(broadcasterID)
			if err == nil && record.Platforms.Twitch != nil {
				// Clear the subscription IDs to indicate it needs resubscription
				platform := record.Platforms.Twitch
				if notification.Subscription.Type == eventTypeStreamOnline {
					platform.EventSubOnlineID = ""
				} else if notification.Subscription.Type == eventTypeStreamOffline {
					platform.EventSubOfflineID = ""
				}
				if platform.EventSubOnlineID == "" && platform.EventSubOfflineID == "" {
					platform.EventSubSubscribed = false
				}
				store.UpdateTwitchPlatform(record.Streamer.ID, platform)
				fmt.Printf("INFO: Updated Twitch platform for streamer in site '%s'\n", siteKey)
				break
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// isTwitchEventSubEnabled checks if Twitch EventSub is enabled for the current site
func (s *server) isTwitchEventSubEnabled() bool {
	return isTwitchEnabledForSiteKey(s.configPath, s.siteKey)
}

// isTwitchEnabledForSiteKey checks if Twitch is enabled for a specific site
func isTwitchEnabledForSiteKey(configPath, siteKey string) bool {
	type siteTwitchConfig struct {
		Enabled *bool `json:"enabled"`
	}
	type siteConfig struct {
		Twitch *siteTwitchConfig `json:"twitch"`
	}
	type configFile struct {
		Sites map[string]siteConfig `json:"sites"`
	}

	data, err := io.ReadAll(mustOpen(configPath))
	if err != nil {
		return false
	}

	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false
	}

	if site, ok := cfg.Sites[siteKey]; ok {
		if site.Twitch != nil && site.Twitch.Enabled != nil {
			return *site.Twitch.Enabled
		}
	}

	return false
}

// GetTwitchCallbackURL returns the EventSub callback URL for the current site
func (s *server) getTwitchCallbackURL() string {
	type siteTwitchConfig struct {
		CallbackURL string `json:"callback_url"`
	}
	type siteConfig struct {
		Twitch *siteTwitchConfig `json:"twitch"`
	}
	type configFile struct {
		Sites map[string]siteConfig `json:"sites"`
	}

	data, err := io.ReadAll(mustOpen(s.configPath))
	if err != nil {
		return ""
	}

	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}

	if site, ok := cfg.Sites[s.siteKey]; ok {
		if site.Twitch != nil {
			return site.Twitch.CallbackURL
		}
	}

	return ""
}

// Helper type alias for consistency
type twitchStreamersStore interface {
	SetTwitchLive(broadcasterID, streamID string, startedAt time.Time) (streamers.Record, error)
	ClearTwitchLive(broadcasterID string) (streamers.Record, error)
	GetByTwitchBroadcasterID(broadcasterID string) (streamers.Record, error)
	UpdateTwitchPlatform(streamerID string, platform *streamers.TwitchPlatform) (streamers.Record, error)
}
