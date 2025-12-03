package websub

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// YouTubeWebSubHub is the YouTube WebSub hub URL
	YouTubeWebSubHub = "https://pubsubhubbub.appspot.com/subscribe"
	// DefaultLeaseSeconds is the default subscription lease duration (5 days)
	DefaultLeaseSeconds = 432000
)

// SubscriptionRequest contains the parameters needed to subscribe to a YouTube channel
type SubscriptionRequest struct {
	ChannelID    string
	CallbackURL  string
	Secret       string
	LeaseSeconds int
}

// SubscriptionResult contains the details of a successful subscription
type SubscriptionResult struct {
	HubURL      string
	TopicURL    string
	CallbackURL string
	Secret      string
	LeaseExpiry time.Time
}

// Subscribe initiates a WebSub subscription to a YouTube channel
func Subscribe(req SubscriptionRequest) (*SubscriptionResult, error) {
	if req.ChannelID == "" {
		return nil, fmt.Errorf("channel ID is required")
	}
	if req.CallbackURL == "" {
		return nil, fmt.Errorf("callback URL is required")
	}

	// Generate secret if not provided
	secret := req.Secret
	if secret == "" {
		var err error
		secret, err = generateSecret()
		if err != nil {
			return nil, fmt.Errorf("generate secret: %w", err)
		}
	}

	// Set default lease seconds
	leaseSeconds := req.LeaseSeconds
	if leaseSeconds == 0 {
		leaseSeconds = DefaultLeaseSeconds
	}

	// Build topic URL
	topicURL := fmt.Sprintf("https://www.youtube.com/xml/feeds/videos.xml?channel_id=%s", req.ChannelID)

	// Prepare subscription request
	form := url.Values{}
	form.Set("hub.callback", req.CallbackURL)
	form.Set("hub.topic", topicURL)
	form.Set("hub.mode", "subscribe")
	form.Set("hub.verify", "async")
	form.Set("hub.secret", secret)
	form.Set("hub.lease_seconds", fmt.Sprintf("%d", leaseSeconds))

	// Send subscription request
	resp, err := http.PostForm(YouTubeWebSubHub, form)
	if err != nil {
		return nil, fmt.Errorf("post subscription request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("subscription request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Calculate lease expiry
	leaseExpiry := time.Now().UTC().Add(time.Duration(leaseSeconds) * time.Second)

	return &SubscriptionResult{
		HubURL:      YouTubeWebSubHub,
		TopicURL:    topicURL,
		CallbackURL: req.CallbackURL,
		Secret:      secret,
		LeaseExpiry: leaseExpiry,
	}, nil
}

// Unsubscribe cancels a WebSub subscription
func Unsubscribe(channelID, callbackURL string) error {
	if channelID == "" {
		return fmt.Errorf("channel ID is required")
	}
	if callbackURL == "" {
		return fmt.Errorf("callback URL is required")
	}

	topicURL := fmt.Sprintf("https://www.youtube.com/xml/feeds/videos.xml?channel_id=%s", channelID)

	form := url.Values{}
	form.Set("hub.callback", callbackURL)
	form.Set("hub.topic", topicURL)
	form.Set("hub.mode", "unsubscribe")
	form.Set("hub.verify", "async")

	resp, err := http.PostForm(YouTubeWebSubHub, form)
	if err != nil {
		return fmt.Errorf("post unsubscribe request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unsubscribe request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// VerifySignature verifies the HMAC-SHA256 signature of a WebSub notification
func VerifySignature(payload []byte, signature, secret string) bool {
	if signature == "" || secret == "" {
		return false
	}

	// Remove "sha256=" prefix if present
	signature = strings.TrimPrefix(signature, "sha256=")
	signature = strings.TrimPrefix(signature, "sha1=")

	// Calculate expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := mac.Sum(nil)
	expectedSig := hex.EncodeToString(expectedMAC)

	// Compare signatures
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

// ExtractChannelIDFromURL extracts a YouTube channel ID from various URL formats
func ExtractChannelIDFromURL(rawURL string) string {
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

	// Extract from path
	path := strings.Trim(parsed.Path, "/")
	segments := strings.Split(path, "/")

	// Handle /channel/UCxxxxxx format
	if len(segments) >= 2 && segments[0] == "channel" {
		return segments[1]
	}

	// Handle /@username or /c/username formats (these require API lookup)
	// For now, return empty - would need YouTube API to resolve
	if len(segments) >= 1 && (strings.HasPrefix(segments[0], "@") || segments[0] == "c") {
		return ""
	}

	return ""
}

// generateSecret generates a random secret for HMAC signing
func generateSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
