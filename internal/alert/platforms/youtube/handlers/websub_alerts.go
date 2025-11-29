// Package handlers exposes HTTP handlers for YouTube WebSub workflows.
package handlers

import (
	youtubesub "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/subscriptions"
	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/websub"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	// SubscriptionConfirmationOptions configures how hub verification requests are handled.
)

type SubscriptionConfirmationOptions struct {
	StreamersStore *streamers.Store
}

type hubRequest struct {
	Challenge     string
	VerifyToken   string
	Topic         string
	Mode          string
	LeaseProvided bool
	LeaseValue    int
}

func (req hubRequest) IsUnsubscribe() bool {
	return strings.EqualFold(req.Mode, "unsubscribe")
}

// HandleSubscriptionConfirmation processes YouTube PubSubHubbub GET verification requests.
// It returns true when the request has been handled (regardless of success).
func HandleSubscriptionConfirmation(w http.ResponseWriter, r *http.Request, opts SubscriptionConfirmationOptions) bool {
	if !isAlertsVerificationRequest(r) {
		return false
	}

	query := r.URL.Query()
	req, baseValidation := parseHubRequest(query)
	if !baseValidation.IsValid {
		http.Error(w, baseValidation.Error, http.StatusBadRequest)
		return true
	}

	exp, ok := websub.LookupExpectation(req.VerifyToken)
	if !ok {
		http.Error(w, "unknown verification token", http.StatusBadRequest)
		return true
	}

	expectationValidation := validateAgainstExpectation(req, exp)
	if !expectationValidation.IsValid {
		http.Error(w, expectationValidation.Error, http.StatusBadRequest)
		return true
	}

	finalExp := finalizeExpectation(req.VerifyToken, exp)
	if opts.StreamersStore != nil {
		updateLeaseIfNeeded(req, finalExp, opts.StreamersStore, time.Now().UTC())
	}

	prepareHubResponse(w, req.Challenge)

	writeChallengeResponse(w, req.Challenge)

	return true
}

func isAlertsVerificationRequest(r *http.Request) bool {
	return r.Method == http.MethodGet && IsAlertPath(r.URL.Path)
}

func parseHubRequest(query url.Values) (hubRequest, ValidationResult) {
	req := hubRequest{
		Challenge:   query.Get("hub.challenge"),
		VerifyToken: strings.TrimSpace(query.Get("hub.verify_token")),
		Topic:       strings.TrimSpace(query.Get("hub.topic")),
		Mode:        strings.TrimSpace(query.Get("hub.mode")),
	}

	if req.Challenge == "" {
		return req, ValidationResult{IsValid: false, Error: "missing hub.challenge"}
	}
	if !validChallenge(req.Challenge) {
		return req, ValidationResult{IsValid: false, Error: "invalid hub.challenge"}
	}
	if req.VerifyToken == "" {
		return req, ValidationResult{IsValid: false, Error: "missing hub.verify_token"}
	}

	leaseParam := strings.TrimSpace(query.Get("hub.lease_seconds"))
	if leaseParam != "" {
		parsedLease, err := strconv.Atoi(leaseParam)
		if err != nil {
			return req, ValidationResult{IsValid: false, Error: "invalid hub.lease_seconds"}
		}
		req.LeaseProvided = true
		req.LeaseValue = parsedLease
	}

	return req, ValidationResult{IsValid: true}
}

func validateAgainstExpectation(req hubRequest, exp websub.Expectation) ValidationResult {
	topic := req.Topic
	if exp.Topic != "" && topic != exp.Topic {
		return ValidationResult{IsValid: false, Error: "hub.topic mismatch"}
	}

	expIsUnsubscribe := strings.EqualFold(exp.Mode, "unsubscribe")
	leaseProvided := req.LeaseProvided
	if leaseProvided && exp.LeaseSeconds > 0 && req.LeaseValue != exp.LeaseSeconds && !expIsUnsubscribe {
		return ValidationResult{IsValid: false, Error: "hub.lease_seconds mismatch"}
	}

	if exp.Mode != "" && !strings.EqualFold(req.Mode, exp.Mode) {
		return ValidationResult{IsValid: false, Error: "hub.mode mismatch"}
	}

	return ValidationResult{IsValid: true}
}

func prepareHubResponse(w http.ResponseWriter, challenge string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(challenge)))
}

func updateLeaseIfNeeded(req hubRequest, exp websub.Expectation, store *streamers.Store, verifiedAt time.Time) string {
	channelID := exp.ChannelID
	if channelID == "" {
		channelID = websub.ExtractChannelID(req.Topic)
	}

	if channelID != "" && !req.IsUnsubscribe() && req.LeaseProvided {
		if err := youtubesub.RecordLease(store, channelID, verifiedAt); err != nil {
		}
	}

	return channelID
}

func finalizeExpectation(verifyToken string, exp websub.Expectation) websub.Expectation {
	finalExp := exp
	if consumed, ok := websub.ConsumeExpectation(verifyToken); ok {
		finalExp = consumed
	}
	return finalExp
}

func writeChallengeResponse(w http.ResponseWriter, challenge string) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, challenge)
}

var challengePattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{1,200}$`)

func validChallenge(challenge string) bool {
	return challengePattern.MatchString(challenge)
}
