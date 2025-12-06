package twitch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const eventsubEndpoint = "https://api.twitch.tv/helix/eventsub/subscriptions"

// EventSubTransport describes the webhook transport configuration.
type EventSubTransport struct {
	Method   string `json:"method"`
	Callback string `json:"callback"`
	Secret   string `json:"secret,omitempty"`
}

// EventSubCondition represents the condition for the subscription.
type EventSubCondition struct {
	BroadcasterUserID string `json:"broadcaster_user_id"`
}

// EventSubSubscription represents an EventSub subscription.
type EventSubSubscription struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	Type      string            `json:"type"`
	Version   string            `json:"version"`
	Condition EventSubCondition `json:"condition"`
	Transport EventSubTransport `json:"transport"`
	CreatedAt time.Time         `json:"created_at"`
	Cost      int               `json:"cost"`
}

// EventSubCreateRequest is the request payload for creating a subscription.
type EventSubCreateRequest struct {
	Type      string            `json:"type"`
	Version   string            `json:"version"`
	Condition EventSubCondition `json:"condition"`
	Transport EventSubTransport `json:"transport"`
}

// EventSubCreateResponse is the response from creating a subscription.
type EventSubCreateResponse struct {
	Data         []EventSubSubscription `json:"data"`
	Total        int                    `json:"total"`
	TotalCost    int                    `json:"total_cost"`
	MaxTotalCost int                    `json:"max_total_cost"`
}

// EventSubListResponse is the response from listing subscriptions.
type EventSubListResponse struct {
	Data       []EventSubSubscription `json:"data"`
	Total      int                    `json:"total"`
	TotalCost  int                    `json:"total_cost"`
	Pagination struct {
		Cursor string `json:"cursor,omitempty"`
	} `json:"pagination"`
}

// EventSubClient handles EventSub API operations.
type EventSubClient struct {
	HTTPClient *http.Client
	Auth       *Authenticator
}

// SubscriptionResult contains the result of creating subscriptions for a broadcaster.
type SubscriptionResult struct {
	BroadcasterID     string
	OnlineSubscription  *EventSubSubscription
	OfflineSubscription *EventSubSubscription
	Errors              []error
}

// Subscribe creates stream.online and stream.offline subscriptions for a broadcaster.
func (c *EventSubClient) Subscribe(ctx context.Context, broadcasterID, callbackURL, secret string) (*SubscriptionResult, error) {
	if c.Auth == nil {
		return nil, fmt.Errorf("authenticator is required")
	}
	if c.HTTPClient == nil {
		c.HTTPClient = http.DefaultClient
	}

	broadcasterID = strings.TrimSpace(broadcasterID)
	if broadcasterID == "" {
		return nil, fmt.Errorf("broadcaster_id is required")
	}
	callbackURL = strings.TrimSpace(callbackURL)
	if callbackURL == "" {
		return nil, fmt.Errorf("callback_url is required")
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("secret is required")
	}
	if len(secret) < 10 || len(secret) > 100 {
		return nil, fmt.Errorf("secret must be between 10 and 100 characters")
	}

	result := &SubscriptionResult{BroadcasterID: broadcasterID}

	// Create stream.online subscription
	onlineSub, err := c.createSubscription(ctx, "stream.online", broadcasterID, callbackURL, secret)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("stream.online: %w", err))
	} else {
		result.OnlineSubscription = onlineSub
	}

	// Create stream.offline subscription
	offlineSub, err := c.createSubscription(ctx, "stream.offline", broadcasterID, callbackURL, secret)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("stream.offline: %w", err))
	} else {
		result.OfflineSubscription = offlineSub
	}

	if result.OnlineSubscription == nil && result.OfflineSubscription == nil {
		return result, fmt.Errorf("failed to create any subscriptions: %v", result.Errors)
	}

	return result, nil
}

// createSubscription creates a single EventSub subscription.
func (c *EventSubClient) createSubscription(ctx context.Context, eventType, broadcasterID, callbackURL, secret string) (*EventSubSubscription, error) {
	reqBody := EventSubCreateRequest{
		Type:    eventType,
		Version: "1",
		Condition: EventSubCondition{
			BroadcasterUserID: broadcasterID,
		},
		Transport: EventSubTransport{
			Method:   "webhook",
			Callback: callbackURL,
			Secret:   secret,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, eventsubEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if err := c.Auth.Apply(ctx, req); err != nil {
		return nil, fmt.Errorf("apply auth: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// 202 Accepted means the subscription was created and webhook verification is pending
	// 409 Conflict means the subscription already exists
	if resp.StatusCode == http.StatusConflict {
		return nil, fmt.Errorf("subscription already exists")
	}
	if resp.StatusCode != http.StatusAccepted {
		var errResp struct {
			Error   string `json:"error"`
			Status  int    `json:"status"`
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("create subscription failed: status %d, error: %s, message: %s", resp.StatusCode, errResp.Error, errResp.Message)
	}

	var createResp EventSubCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(createResp.Data) == 0 {
		return nil, fmt.Errorf("no subscription returned in response")
	}

	return &createResp.Data[0], nil
}

// Unsubscribe removes EventSub subscriptions by subscription ID.
func (c *EventSubClient) Unsubscribe(ctx context.Context, subscriptionID string) error {
	if c.Auth == nil {
		return fmt.Errorf("authenticator is required")
	}
	if c.HTTPClient == nil {
		c.HTTPClient = http.DefaultClient
	}

	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return fmt.Errorf("subscription_id is required")
	}

	endpoint := fmt.Sprintf("%s?id=%s", eventsubEndpoint, subscriptionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if err := c.Auth.Apply(ctx, req); err != nil {
		return fmt.Errorf("apply auth: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete subscription failed: status %d", resp.StatusCode)
	}

	return nil
}

// UnsubscribeByBroadcaster removes all EventSub subscriptions for a broadcaster.
func (c *EventSubClient) UnsubscribeByBroadcaster(ctx context.Context, broadcasterID string) error {
	subs, err := c.ListByBroadcaster(ctx, broadcasterID)
	if err != nil {
		return fmt.Errorf("list subscriptions: %w", err)
	}

	var errors []error
	for _, sub := range subs {
		if err := c.Unsubscribe(ctx, sub.ID); err != nil {
			errors = append(errors, fmt.Errorf("unsubscribe %s: %w", sub.ID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to unsubscribe some subscriptions: %v", errors)
	}

	return nil
}

// List retrieves all EventSub subscriptions.
func (c *EventSubClient) List(ctx context.Context) ([]EventSubSubscription, error) {
	if c.Auth == nil {
		return nil, fmt.Errorf("authenticator is required")
	}
	if c.HTTPClient == nil {
		c.HTTPClient = http.DefaultClient
	}

	var allSubs []EventSubSubscription
	cursor := ""

	for {
		endpoint := eventsubEndpoint
		if cursor != "" {
			endpoint = fmt.Sprintf("%s?after=%s", eventsubEndpoint, cursor)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		if err := c.Auth.Apply(ctx, req); err != nil {
			return nil, fmt.Errorf("apply auth: %w", err)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("execute request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("list subscriptions failed: status %d", resp.StatusCode)
		}

		var listResp EventSubListResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode response: %w", err)
		}
		resp.Body.Close()

		allSubs = append(allSubs, listResp.Data...)

		if listResp.Pagination.Cursor == "" {
			break
		}
		cursor = listResp.Pagination.Cursor
	}

	return allSubs, nil
}

// ListByBroadcaster retrieves EventSub subscriptions for a specific broadcaster.
func (c *EventSubClient) ListByBroadcaster(ctx context.Context, broadcasterID string) ([]EventSubSubscription, error) {
	broadcasterID = strings.TrimSpace(broadcasterID)
	if broadcasterID == "" {
		return nil, fmt.Errorf("broadcaster_id is required")
	}

	allSubs, err := c.List(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []EventSubSubscription
	for _, sub := range allSubs {
		if sub.Condition.BroadcasterUserID == broadcasterID {
			filtered = append(filtered, sub)
		}
	}

	return filtered, nil
}
