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
	fmt.Printf("=== WebSub Subscribe START ===\n")
	fmt.Printf("  Channel ID: %s\n", req.ChannelID)
	fmt.Printf("  Callback URL: %s\n", req.CallbackURL)
	fmt.Printf("  Secret provided: %v\n", req.Secret != "")
	fmt.Printf("  Lease seconds: %d\n", req.LeaseSeconds)

	if req.ChannelID == "" {
		fmt.Printf("ERROR: Channel ID is required\n")
		return nil, fmt.Errorf("channel ID is required")
	}
	if req.CallbackURL == "" {
		fmt.Printf("ERROR: Callback URL is required\n")
		return nil, fmt.Errorf("callback URL is required")
	}

	// Generate secret if not provided
	secret := req.Secret
	if secret == "" {
		fmt.Printf("INFO: Generating new secret...\n")
		var err error
		secret, err = generateSecret()
		if err != nil {
			fmt.Printf("ERROR: Failed to generate secret: %v\n", err)
			return nil, fmt.Errorf("generate secret: %w", err)
		}
		fmt.Printf("INFO: Secret generated successfully (length: %d)\n", len(secret))
	}

	// Set default lease seconds
	leaseSeconds := req.LeaseSeconds
	if leaseSeconds == 0 {
		leaseSeconds = DefaultLeaseSeconds
		fmt.Printf("INFO: Using default lease seconds: %d\n", leaseSeconds)
	}

	// Build topic URL
	topicURL := fmt.Sprintf("https://www.youtube.com/xml/feeds/videos.xml?channel_id=%s", req.ChannelID)
	fmt.Printf("INFO: Topic URL: %s\n", topicURL)

	// Prepare subscription request
	form := url.Values{}
	form.Set("hub.callback", req.CallbackURL)
	form.Set("hub.topic", topicURL)
	form.Set("hub.mode", "subscribe")
	form.Set("hub.verify", "async")
	form.Set("hub.secret", secret)
	form.Set("hub.lease_seconds", fmt.Sprintf("%d", leaseSeconds))

	fmt.Printf("INFO: Sending subscription request to hub: %s\n", YouTubeWebSubHub)
	fmt.Printf("INFO: Request parameters:\n")
	fmt.Printf("  hub.callback: %s\n", form.Get("hub.callback"))
	fmt.Printf("  hub.topic: %s\n", form.Get("hub.topic"))
	fmt.Printf("  hub.mode: %s\n", form.Get("hub.mode"))
	fmt.Printf("  hub.verify: %s\n", form.Get("hub.verify"))
	fmt.Printf("  hub.secret: [%d chars]\n", len(form.Get("hub.secret")))
	fmt.Printf("  hub.lease_seconds: %s\n", form.Get("hub.lease_seconds"))

	// Send subscription request
	resp, err := http.PostForm(YouTubeWebSubHub, form)
	if err != nil {
		fmt.Printf("ERROR: Failed to send subscription request: %v\n", err)
		return nil, fmt.Errorf("post subscription request: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("INFO: Hub response status: %d %s\n", resp.StatusCode, resp.Status)
	fmt.Printf("INFO: Response headers:\n")
	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}

	// Read response body
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		fmt.Printf("WARNING: Failed to read response body: %v\n", readErr)
	} else if len(body) > 0 {
		fmt.Printf("INFO: Response body: %s\n", string(body))
	} else {
		fmt.Printf("INFO: Response body is empty\n")
	}

	// Check response status
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		fmt.Printf("ERROR: Subscription request failed with status %d: %s\n", resp.StatusCode, string(body))
		return nil, fmt.Errorf("subscription request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Calculate lease expiry
	leaseExpiry := time.Now().UTC().Add(time.Duration(leaseSeconds) * time.Second)
	fmt.Printf("INFO: Calculated lease expiry: %s\n", leaseExpiry.Format("2006-01-02 15:04:05 MST"))

	fmt.Printf("SUCCESS: WebSub subscription request accepted by hub\n")
	fmt.Printf("=== WebSub Subscribe END ===\n")

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
	fmt.Printf("=== WebSub Unsubscribe START ===\n")
	fmt.Printf("  Channel ID: %s\n", channelID)
	fmt.Printf("  Callback URL: %s\n", callbackURL)

	if channelID == "" {
		fmt.Printf("ERROR: Channel ID is required\n")
		return fmt.Errorf("channel ID is required")
	}
	if callbackURL == "" {
		fmt.Printf("ERROR: Callback URL is required\n")
		return fmt.Errorf("callback URL is required")
	}

	topicURL := fmt.Sprintf("https://www.youtube.com/xml/feeds/videos.xml?channel_id=%s", channelID)
	fmt.Printf("INFO: Topic URL: %s\n", topicURL)

	form := url.Values{}
	form.Set("hub.callback", callbackURL)
	form.Set("hub.topic", topicURL)
	form.Set("hub.mode", "unsubscribe")
	form.Set("hub.verify", "async")

	fmt.Printf("INFO: Sending unsubscribe request to hub: %s\n", YouTubeWebSubHub)
	fmt.Printf("INFO: Request parameters:\n")
	fmt.Printf("  hub.callback: %s\n", form.Get("hub.callback"))
	fmt.Printf("  hub.topic: %s\n", form.Get("hub.topic"))
	fmt.Printf("  hub.mode: %s\n", form.Get("hub.mode"))
	fmt.Printf("  hub.verify: %s\n", form.Get("hub.verify"))

	resp, err := http.PostForm(YouTubeWebSubHub, form)
	if err != nil {
		fmt.Printf("ERROR: Failed to send unsubscribe request: %v\n", err)
		return fmt.Errorf("post unsubscribe request: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("INFO: Hub response status: %d %s\n", resp.StatusCode, resp.Status)
	fmt.Printf("INFO: Response headers:\n")
	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}

	// Read response body
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		fmt.Printf("WARNING: Failed to read response body: %v\n", readErr)
	} else if len(body) > 0 {
		fmt.Printf("INFO: Response body: %s\n", string(body))
	} else {
		fmt.Printf("INFO: Response body is empty\n")
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		fmt.Printf("ERROR: Unsubscribe request failed with status %d: %s\n", resp.StatusCode, string(body))
		return fmt.Errorf("unsubscribe request failed with status %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("SUCCESS: WebSub unsubscribe request accepted by hub\n")
	fmt.Printf("=== WebSub Unsubscribe END ===\n")

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
