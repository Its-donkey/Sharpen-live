package twitch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const tokenURL = "https://id.twitch.tv/oauth2/token"

// Authenticator handles Twitch app access token retrieval and caching.
type Authenticator struct {
	client       *http.Client
	clientID     string
	clientSecret string

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// NewAuthenticator builds an Authenticator that reads credentials from the provided values or TWITCH_CLIENT_ID/TWITCH_CLIENT_SECRET when empty.
func NewAuthenticator(client *http.Client, clientID, clientSecret string) *Authenticator {
	if client == nil {
		client = &http.Client{}
	}
	return &Authenticator{
		client:       client,
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
	}
}

// Token returns a cached app access token or fetches a new one when expired.
func (a *Authenticator) Token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.clientID == "" {
		a.clientID = strings.TrimSpace(os.Getenv("TWITCH_CLIENT_ID"))
	}
	if a.clientSecret == "" {
		a.clientSecret = strings.TrimSpace(os.Getenv("TWITCH_CLIENT_SECRET"))
	}

	if a.clientID == "" || a.clientSecret == "" {
		return "", fmt.Errorf("missing Twitch credentials: set TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET")
	}

	// Reuse cached token if still valid (with small buffer).
	if a.accessToken != "" && time.Now().Add(30*time.Second).Before(a.tokenExpiry) {
		return a.accessToken, nil
	}

	token, expiry, err := a.requestAccessToken(ctx)
	if err != nil {
		return "", err
	}

	a.accessToken = token
	a.tokenExpiry = expiry
	return token, nil
}

// Apply adds Client-Id and Authorization headers to an HTTP request, fetching a token if needed.
func (a *Authenticator) Apply(ctx context.Context, req *http.Request) error {
	token, err := a.Token(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Client-Id", a.clientID)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	return nil
}

func (a *Authenticator) requestAccessToken(ctx context.Context) (string, time.Time, error) {
	form := url.Values{}
	form.Set("client_id", a.clientID)
	form.Set("client_secret", a.clientSecret)
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("request token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("token request failed: status %d", resp.StatusCode)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", time.Time{}, fmt.Errorf("decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("token response missing access_token")
	}

	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return tokenResp.AccessToken, expiry, nil
}
