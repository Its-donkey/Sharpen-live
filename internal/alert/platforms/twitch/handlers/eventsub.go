package handlers

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
	"github.com/Its-donkey/Sharpen-live/logging"
)

// Twitch EventSub header names
const (
	HeaderMessageID        = "Twitch-Eventsub-Message-Id"
	HeaderMessageTimestamp = "Twitch-Eventsub-Message-Timestamp"
	HeaderMessageType      = "Twitch-Eventsub-Message-Type"
	HeaderMessageSignature = "Twitch-Eventsub-Message-Signature"
	HeaderSubscriptionType = "Twitch-Eventsub-Subscription-Type"
)

// Message types
const (
	MessageTypeVerification = "webhook_callback_verification"
	MessageTypeNotification = "notification"
	MessageTypeRevocation   = "revocation"
)

// EventSub subscription types we care about
const (
	SubscriptionStreamOnline  = "stream.online"
	SubscriptionStreamOffline = "stream.offline"
)

// EventSubHandler handles Twitch EventSub webhook callbacks.
type EventSubHandler struct {
	// Secret is the shared secret used for HMAC signature verification.
	// This should match the secret used when creating the EventSub subscription.
	Secret string

	// StreamersStore is used to update streamer live status.
	StreamersStore *streamers.Store

	// Logger for structured logging.
	Logger *logging.Logger

	// GetAllStores returns all streamer stores across sites for multi-site support.
	// If nil, only StreamersStore is used.
	GetAllStores func() map[string]*streamers.Store
}

// EventSubPayload represents the common structure of EventSub webhook payloads.
type EventSubPayload struct {
	Challenge    string              `json:"challenge,omitempty"`
	Subscription EventSubSubscription `json:"subscription"`
	Event        json.RawMessage     `json:"event,omitempty"`
}

// EventSubSubscription contains subscription metadata.
type EventSubSubscription struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	Type      string            `json:"type"`
	Version   string            `json:"version"`
	Condition map[string]string `json:"condition"`
	CreatedAt time.Time         `json:"created_at"`
}

// StreamOnlineEvent represents the stream.online event payload.
type StreamOnlineEvent struct {
	ID               string    `json:"id"`
	BroadcasterUserID   string `json:"broadcaster_user_id"`
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
	Type             string    `json:"type"`
	StartedAt        time.Time `json:"started_at"`
}

// StreamOfflineEvent represents the stream.offline event payload.
type StreamOfflineEvent struct {
	BroadcasterUserID   string `json:"broadcaster_user_id"`
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
}

// ServeHTTP implements http.Handler for EventSub webhooks.
func (h *EventSubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.handleEventSub(w, r)
}

func (h *EventSubHandler) handleEventSub(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("\n=== TWITCH EVENTSUB CALLBACK START ===\n")
	fmt.Printf("Remote Address: %s\n", r.RemoteAddr)
	fmt.Printf("Content-Type: %s\n", r.Header.Get("Content-Type"))

	// Extract EventSub headers
	messageID := r.Header.Get(HeaderMessageID)
	messageTimestamp := r.Header.Get(HeaderMessageTimestamp)
	messageType := r.Header.Get(HeaderMessageType)
	signature := r.Header.Get(HeaderMessageSignature)
	subscriptionType := r.Header.Get(HeaderSubscriptionType)

	fmt.Printf("Message-ID: %s\n", messageID)
	fmt.Printf("Message-Type: %s\n", messageType)
	fmt.Printf("Subscription-Type: %s\n", subscriptionType)

	if h.Logger != nil {
		h.Logger.Info("twitch-eventsub", "Received EventSub callback", map[string]any{
			"messageId":        messageID,
			"messageType":      messageType,
			"subscriptionType": subscriptionType,
		})
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("ERROR: Failed to read body: %v\n", err)
		if h.Logger != nil {
			h.Logger.Error("twitch-eventsub", "Failed to read request body", err, nil)
		}
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	fmt.Printf("Body length: %d bytes\n", len(body))

	// Verify signature if secret is configured
	if h.Secret != "" {
		if !h.verifySignature(messageID, messageTimestamp, body, signature) {
			fmt.Printf("ERROR: Signature verification failed\n")
			if h.Logger != nil {
				h.Logger.Warn("twitch-eventsub", "Signature verification failed", map[string]any{
					"messageId": messageID,
				})
			}
			http.Error(w, "invalid signature", http.StatusForbidden)
			return
		}
		fmt.Printf("INFO: Signature verified\n")
	} else {
		fmt.Printf("WARNING: No secret configured, skipping signature verification\n")
	}

	// Parse payload
	var payload EventSubPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		fmt.Printf("ERROR: Failed to parse payload: %v\n", err)
		if h.Logger != nil {
			h.Logger.Error("twitch-eventsub", "Failed to parse EventSub payload", err, nil)
		}
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Handle based on message type
	switch messageType {
	case MessageTypeVerification:
		h.handleVerification(w, payload)
	case MessageTypeNotification:
		h.handleNotification(w, payload)
	case MessageTypeRevocation:
		h.handleRevocation(w, payload)
	default:
		fmt.Printf("WARNING: Unknown message type: %s\n", messageType)
		w.WriteHeader(http.StatusOK)
	}

	fmt.Printf("=== TWITCH EVENTSUB CALLBACK END ===\n\n")
}

// verifySignature verifies the HMAC-SHA256 signature from Twitch.
// The signature is computed over: message_id + message_timestamp + body
func (h *EventSubHandler) verifySignature(messageID, timestamp string, body []byte, signature string) bool {
	if h.Secret == "" {
		return true // No secret configured, skip verification
	}

	// Signature format: "sha256=<hex>"
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	expectedSig := strings.TrimPrefix(signature, "sha256=")

	// Compute HMAC
	message := messageID + timestamp + string(body)
	mac := hmac.New(sha256.New, []byte(h.Secret))
	mac.Write([]byte(message))
	computedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSig), []byte(computedSig))
}

// handleVerification responds to webhook verification challenges.
func (h *EventSubHandler) handleVerification(w http.ResponseWriter, payload EventSubPayload) {
	fmt.Printf("INFO: Handling verification challenge\n")
	fmt.Printf("  Subscription ID: %s\n", payload.Subscription.ID)
	fmt.Printf("  Subscription Type: %s\n", payload.Subscription.Type)
	fmt.Printf("  Challenge length: %d\n", len(payload.Challenge))

	if h.Logger != nil {
		h.Logger.Info("twitch-eventsub", "Responding to verification challenge", map[string]any{
			"subscriptionId":   payload.Subscription.ID,
			"subscriptionType": payload.Subscription.Type,
		})
	}

	// Respond with the challenge to confirm subscription
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(payload.Challenge))
}

// handleNotification processes incoming event notifications.
func (h *EventSubHandler) handleNotification(w http.ResponseWriter, payload EventSubPayload) {
	fmt.Printf("INFO: Handling notification\n")
	fmt.Printf("  Subscription Type: %s\n", payload.Subscription.Type)

	switch payload.Subscription.Type {
	case SubscriptionStreamOnline:
		h.handleStreamOnline(payload)
	case SubscriptionStreamOffline:
		h.handleStreamOffline(payload)
	default:
		fmt.Printf("INFO: Unhandled subscription type: %s\n", payload.Subscription.Type)
		if h.Logger != nil {
			h.Logger.Info("twitch-eventsub", "Received unhandled subscription type", map[string]any{
				"subscriptionType": payload.Subscription.Type,
			})
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleRevocation handles subscription revocation notices.
func (h *EventSubHandler) handleRevocation(w http.ResponseWriter, payload EventSubPayload) {
	fmt.Printf("WARNING: Subscription revoked\n")
	fmt.Printf("  Subscription ID: %s\n", payload.Subscription.ID)
	fmt.Printf("  Subscription Type: %s\n", payload.Subscription.Type)
	fmt.Printf("  Status: %s\n", payload.Subscription.Status)

	if h.Logger != nil {
		h.Logger.Warn("twitch-eventsub", "Subscription revoked", map[string]any{
			"subscriptionId":   payload.Subscription.ID,
			"subscriptionType": payload.Subscription.Type,
			"status":           payload.Subscription.Status,
		})
	}

	w.WriteHeader(http.StatusOK)
}

// handleStreamOnline processes stream.online events.
func (h *EventSubHandler) handleStreamOnline(payload EventSubPayload) {
	var event StreamOnlineEvent
	if err := json.Unmarshal(payload.Event, &event); err != nil {
		fmt.Printf("ERROR: Failed to parse stream.online event: %v\n", err)
		if h.Logger != nil {
			h.Logger.Error("twitch-eventsub", "Failed to parse stream.online event", err, nil)
		}
		return
	}

	fmt.Printf("INFO: Stream online event\n")
	fmt.Printf("  Broadcaster ID: %s\n", event.BroadcasterUserID)
	fmt.Printf("  Broadcaster Login: %s\n", event.BroadcasterUserLogin)
	fmt.Printf("  Stream ID: %s\n", event.ID)
	fmt.Printf("  Started At: %s\n", event.StartedAt.Format(time.RFC3339))

	if h.Logger != nil {
		h.Logger.Info("twitch-eventsub", "Stream went online", map[string]any{
			"broadcasterId":    event.BroadcasterUserID,
			"broadcasterLogin": event.BroadcasterUserLogin,
			"streamId":         event.ID,
			"startedAt":        event.StartedAt,
		})
	}

	// Update streamer status
	h.updateStreamerLive(event.BroadcasterUserID, event.ID, event.StartedAt)
}

// handleStreamOffline processes stream.offline events.
func (h *EventSubHandler) handleStreamOffline(payload EventSubPayload) {
	var event StreamOfflineEvent
	if err := json.Unmarshal(payload.Event, &event); err != nil {
		fmt.Printf("ERROR: Failed to parse stream.offline event: %v\n", err)
		if h.Logger != nil {
			h.Logger.Error("twitch-eventsub", "Failed to parse stream.offline event", err, nil)
		}
		return
	}

	fmt.Printf("INFO: Stream offline event\n")
	fmt.Printf("  Broadcaster ID: %s\n", event.BroadcasterUserID)
	fmt.Printf("  Broadcaster Login: %s\n", event.BroadcasterUserLogin)

	if h.Logger != nil {
		h.Logger.Info("twitch-eventsub", "Stream went offline", map[string]any{
			"broadcasterId":    event.BroadcasterUserID,
			"broadcasterLogin": event.BroadcasterUserLogin,
		})
	}

	// Update streamer status
	h.updateStreamerOffline(event.BroadcasterUserID)
}

// updateStreamerLive marks a Twitch streamer as live.
func (h *EventSubHandler) updateStreamerLive(broadcasterID, streamID string, startedAt time.Time) {
	stores := h.getStores()

	for siteKey, store := range stores {
		records, err := store.List()
		if err != nil {
			fmt.Printf("WARNING: Failed to list streamers for site %s: %v\n", siteKey, err)
			continue
		}

		for _, record := range records {
			if record.Platforms.Twitch != nil && record.Platforms.Twitch.BroadcasterID == broadcasterID {
				fmt.Printf("INFO: Found matching streamer: %s (site: %s)\n", record.Streamer.Alias, siteKey)

				_, err := store.SetTwitchLive(broadcasterID, streamID, startedAt)
				if err != nil {
					fmt.Printf("ERROR: Failed to set Twitch live status: %v\n", err)
					if h.Logger != nil {
						h.Logger.Error("twitch-eventsub", "Failed to set Twitch live status", err, map[string]any{
							"broadcasterId": broadcasterID,
							"streamerId":    record.Streamer.ID,
							"site":          siteKey,
						})
					}
				} else {
					fmt.Printf("SUCCESS: Updated streamer %s to LIVE\n", record.Streamer.Alias)
					if h.Logger != nil {
						h.Logger.Info("twitch-eventsub", "Set streamer live", map[string]any{
							"broadcasterId": broadcasterID,
							"streamerId":    record.Streamer.ID,
							"alias":         record.Streamer.Alias,
							"streamId":      streamID,
							"site":          siteKey,
						})
					}
				}
				return
			}
		}
	}

	fmt.Printf("WARNING: No streamer found for broadcaster ID: %s\n", broadcasterID)
	if h.Logger != nil {
		h.Logger.Warn("twitch-eventsub", "No streamer found for broadcaster", map[string]any{
			"broadcasterId": broadcasterID,
		})
	}
}

// updateStreamerOffline marks a Twitch streamer as offline.
func (h *EventSubHandler) updateStreamerOffline(broadcasterID string) {
	stores := h.getStores()

	for siteKey, store := range stores {
		records, err := store.List()
		if err != nil {
			fmt.Printf("WARNING: Failed to list streamers for site %s: %v\n", siteKey, err)
			continue
		}

		for _, record := range records {
			if record.Platforms.Twitch != nil && record.Platforms.Twitch.BroadcasterID == broadcasterID {
				fmt.Printf("INFO: Found matching streamer: %s (site: %s)\n", record.Streamer.Alias, siteKey)

				_, err := store.ClearTwitchLive(broadcasterID)
				if err != nil {
					fmt.Printf("ERROR: Failed to clear Twitch live status: %v\n", err)
					if h.Logger != nil {
						h.Logger.Error("twitch-eventsub", "Failed to clear Twitch live status", err, map[string]any{
							"broadcasterId": broadcasterID,
							"streamerId":    record.Streamer.ID,
							"site":          siteKey,
						})
					}
				} else {
					fmt.Printf("SUCCESS: Updated streamer %s to OFFLINE\n", record.Streamer.Alias)
					if h.Logger != nil {
						h.Logger.Info("twitch-eventsub", "Set streamer offline", map[string]any{
							"broadcasterId": broadcasterID,
							"streamerId":    record.Streamer.ID,
							"alias":         record.Streamer.Alias,
							"site":          siteKey,
						})
					}
				}
				return
			}
		}
	}

	fmt.Printf("WARNING: No streamer found for broadcaster ID: %s\n", broadcasterID)
	if h.Logger != nil {
		h.Logger.Warn("twitch-eventsub", "No streamer found for broadcaster", map[string]any{
			"broadcasterId": broadcasterID,
		})
	}
}

// getStores returns all streamer stores to check.
func (h *EventSubHandler) getStores() map[string]*streamers.Store {
	if h.GetAllStores != nil {
		return h.GetAllStores()
	}

	if h.StreamersStore != nil {
		return map[string]*streamers.Store{"default": h.StreamersStore}
	}

	return nil
}
