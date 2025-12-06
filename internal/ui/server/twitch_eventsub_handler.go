package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	// Read headers
	headers := EventSubHeaders{
		MessageID:        r.Header.Get("Twitch-Eventsub-Message-Id"),
		MessageRetry:     r.Header.Get("Twitch-Eventsub-Message-Retry"),
		MessageType:      r.Header.Get("Twitch-Eventsub-Message-Type"),
		MessageSignature: r.Header.Get("Twitch-Eventsub-Message-Signature"),
		MessageTimestamp: r.Header.Get("Twitch-Eventsub-Message-Timestamp"),
		SubscriptionType: r.Header.Get("Twitch-Eventsub-Subscription-Type"),
	}

	s.logger.Info("twitch-eventsub", "Received EventSub request", map[string]any{
		"messageId":        headers.MessageID,
		"messageType":      headers.MessageType,
		"subscriptionType": headers.SubscriptionType,
		"remoteAddr":       r.RemoteAddr,
	})

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("twitch-eventsub", "Failed to read request body", err, nil)
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify signature
	if !s.verifyEventSubSignature(headers, body) {
		s.logger.Warn("twitch-eventsub", "Signature verification failed", map[string]any{
			"messageId": headers.MessageID,
		})
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// Handle based on message type
	switch headers.MessageType {
	case eventsubMessageTypeVerification:
		s.handleEventSubVerification(w, body)
	case eventsubMessageTypeNotification:
		s.handleEventSubNotification(w, headers, body)
	case eventsubMessageTypeRevocation:
		s.handleEventSubRevocation(w, headers, body)
	default:
		s.logger.Warn("twitch-eventsub", "Unknown message type", map[string]any{
			"messageType": headers.MessageType,
		})
		w.WriteHeader(http.StatusOK)
	}
}

// verifyEventSubSignature verifies the HMAC-SHA256 signature from Twitch
func (s *server) verifyEventSubSignature(headers EventSubHeaders, body []byte) bool {
	secret := s.getTwitchEventSubSecret()
	if secret == "" {
		s.logger.Warn("twitch-eventsub", "No EventSub secret configured", map[string]any{
			"site": s.siteKey,
		})
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
	// Use the already-loaded config
	if s.twitchConfig.EventSubSecret != "" {
		return s.twitchConfig.EventSubSecret
	}
	return ""
}


// handleEventSubVerification handles the webhook verification challenge
func (s *server) handleEventSubVerification(w http.ResponseWriter, body []byte) {
	var verification EventSubVerification
	if err := json.Unmarshal(body, &verification); err != nil {
		s.logger.Error("twitch-eventsub", "Failed to parse verification payload", err, nil)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	s.logger.Info("twitch-eventsub", "Responding to verification challenge", map[string]any{
		"subscriptionId":   verification.Subscription.ID,
		"subscriptionType": verification.Subscription.Type,
		"site":             s.siteKey,
	})

	// Respond with the challenge
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(verification.Challenge))
}

// handleEventSubNotification handles stream event notifications
func (s *server) handleEventSubNotification(w http.ResponseWriter, headers EventSubHeaders, body []byte) {
	var notification EventSubNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		s.logger.Error("twitch-eventsub", "Failed to parse notification payload", err, nil)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	switch notification.Subscription.Type {
	case eventTypeStreamOnline:
		s.handleStreamOnline(notification)
	case eventTypeStreamOffline:
		s.handleStreamOffline(notification)
	default:
		s.logger.Info("twitch-eventsub", "Unhandled event type", map[string]any{
			"eventType":      notification.Subscription.Type,
			"subscriptionId": notification.Subscription.ID,
		})
	}

	// Always respond with 200 OK to acknowledge receipt
	w.WriteHeader(http.StatusOK)
}

// handleStreamOnline processes stream.online events
func (s *server) handleStreamOnline(notification EventSubNotification) {
	var event StreamOnlineEvent
	if err := json.Unmarshal(notification.Event, &event); err != nil {
		s.logger.Error("twitch-eventsub", "Failed to parse stream.online event", err, nil)
		return
	}

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
			s.logger.Info("twitch-eventsub", "Updated streamer to live", map[string]any{
				"broadcasterID":   event.BroadcasterUserID,
				"broadcasterName": event.BroadcasterUserName,
				"streamID":        event.ID,
				"site":            siteKey,
			})
			break
		}
		if !strings.Contains(err.Error(), "not found") {
			s.logger.Error("twitch-eventsub", "Failed to update streamer status", err, map[string]any{
				"broadcasterID": event.BroadcasterUserID,
				"site":          siteKey,
			})
		}
	}

	if !found {
		s.logger.Warn("twitch-eventsub", "No streamer found for broadcaster", map[string]any{
			"broadcasterID":   event.BroadcasterUserID,
			"broadcasterName": event.BroadcasterUserName,
		})
	}
}

// handleStreamOffline processes stream.offline events
func (s *server) handleStreamOffline(notification EventSubNotification) {
	var event StreamOfflineEvent
	if err := json.Unmarshal(notification.Event, &event); err != nil {
		s.logger.Error("twitch-eventsub", "Failed to parse stream.offline event", err, nil)
		return
	}

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
			s.logger.Info("twitch-eventsub", "Updated streamer to offline", map[string]any{
				"broadcasterID":   event.BroadcasterUserID,
				"broadcasterName": event.BroadcasterUserName,
				"site":            siteKey,
			})
			break
		}
		if !strings.Contains(err.Error(), "not found") {
			s.logger.Error("twitch-eventsub", "Failed to update streamer status", err, map[string]any{
				"broadcasterID": event.BroadcasterUserID,
				"site":          siteKey,
			})
		}
	}

	if !found {
		s.logger.Warn("twitch-eventsub", "No streamer found for broadcaster", map[string]any{
			"broadcasterID":   event.BroadcasterUserID,
			"broadcasterName": event.BroadcasterUserName,
		})
	}
}

// handleEventSubRevocation handles subscription revocation notifications
func (s *server) handleEventSubRevocation(w http.ResponseWriter, headers EventSubHeaders, body []byte) {
	var notification EventSubNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		s.logger.Error("twitch-eventsub", "Failed to parse revocation payload", err, nil)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	broadcasterID := notification.Subscription.Condition["broadcaster_user_id"]

	s.logger.Warn("twitch-eventsub", "Subscription revoked", map[string]any{
		"subscriptionId": notification.Subscription.ID,
		"reason":         notification.Subscription.Status,
		"type":           notification.Subscription.Type,
		"broadcasterID":  broadcasterID,
	})

	// Update the streamer's EventSub status if we can identify them
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
				s.logger.Info("twitch-eventsub", "Cleared EventSub subscription for streamer", map[string]any{
					"broadcasterID":    broadcasterID,
					"subscriptionType": notification.Subscription.Type,
					"site":             siteKey,
				})
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

func mustOpen(path string) io.Reader {
	f, err := http.Dir(".").Open(path)
	if err != nil {
		return strings.NewReader("")
	}
	return f
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
