package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/Its-donkey/Sharpen-live/backend/internal/settings"
	"github.com/Its-donkey/Sharpen-live/backend/internal/storage"
)

// Server exposes HTTP handlers backed by the storage layer.
type Server struct {
	store         *storage.JSONStore
	settingsStore settings.Store
	adminToken    string
	adminEmail    string
	adminPassword string
	youtubeAPIKey string
	httpClient    *http.Client
	youtubeHubURL string
	youtubeAlerts struct {
		callbackURL string
		secret      string
		verifyPref  string
		verifySuff  string
		enabled     bool
	}
	youtubeEvents   []youtubeEvent
	listenAddr      string
	dataDir         string
	staticDir       string
	streamersFile   string
	submissionsFile string
	mu              sync.RWMutex
}

// Option mutates server configuration during construction.
type Option func(*Server)

// YouTubeAlertsConfig controls PubSub subscriptions for YouTube channel alerts.
type YouTubeAlertsConfig struct {
	HubURL            string
	CallbackURL       string
	Secret            string
	VerifyTokenPrefix string
	VerifyTokenSuffix string
}

type youtubeEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Mode        string    `json:"mode"`
	ChannelID   string    `json:"channelId"`
	Topic       string    `json:"topic"`
	Callback    string    `json:"callback"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	VerifyToken string    `json:"verifyToken,omitempty"`
	HasSecret   bool      `json:"hasSecret"`
}

const defaultYouTubeHubURL = "https://pubsubhubbub.appspot.com/subscribe"
const youtubeEventLogLimit = 100

type validationError string

func (e validationError) Error() string {
	return string(e)
}

// New constructs a Server with the provided dependencies.
func New(store *storage.JSONStore, settingsStore settings.Store, initial settings.Settings, opts ...Option) *Server {
	normalized := normalizeSettings(initial)

	s := &Server{
		store:           store,
		settingsStore:   settingsStore,
		adminToken:      normalized.AdminToken,
		adminEmail:      strings.ToLower(normalized.AdminEmail),
		adminPassword:   normalized.AdminPassword,
		youtubeAPIKey:   normalized.YouTubeAPIKey,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		youtubeHubURL:   normalized.YouTubeAlertsHubURL,
		listenAddr:      normalized.ListenAddr,
		dataDir:         normalized.DataDir,
		staticDir:       normalized.StaticDir,
		streamersFile:   normalized.StreamersFile,
		submissionsFile: normalized.SubmissionsFile,
	}

	s.youtubeAlerts.callbackURL = normalized.YouTubeAlertsCallback
	s.youtubeAlerts.secret = normalized.YouTubeAlertsSecret
	s.youtubeAlerts.verifyPref = normalized.YouTubeAlertsVerifyPrefix
	s.youtubeAlerts.verifySuff = normalized.YouTubeAlertsVerifySuffix
	s.youtubeAlerts.enabled = s.youtubeAlerts.callbackURL != ""

	if s.youtubeHubURL == "" {
		s.youtubeHubURL = defaultYouTubeHubURL
	}

	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}

	return s
}

// Handler returns the HTTP handler that serves the Sharpen Live API and static assets.
func (s *Server) Handler(static http.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/api/streamers", http.HandlerFunc(s.handleStreamers))
	mux.Handle("/api/admin/login", http.HandlerFunc(s.handleAdminLogin))
	mux.Handle("/api/admin/streamers", http.HandlerFunc(s.handleAdminStreamers))
	mux.Handle("/api/admin/streamers/", http.HandlerFunc(s.handleAdminStreamerByID))
	mux.Handle("/api/submit-streamer", http.HandlerFunc(s.handleSubmitStreamer))
	mux.Handle("/api/admin/submissions", http.HandlerFunc(s.handleAdminSubmissions))
	mux.Handle("/api/admin/settings", http.HandlerFunc(s.handleAdminSettings))
	mux.Handle("/api/admin/monitor/youtube", http.HandlerFunc(s.handleAdminYouTubeMonitor))

	// Mount the static handler as a catch-all for everything else.
	mux.Handle("/", static)

	return addCORS(mux)
}

func (s *Server) handleStreamers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		streamers, err := s.store.ListStreamers()
		if err != nil {
			respondError(w, http.StatusInternalServerError, err)
			return
		}
		respondJSON(w, http.StatusOK, streamers)
	default:
		methodNotAllowed(w, http.MethodGet)
	}
}

// WithYouTubeAlerts configures automatic PubSub subscriptions for YouTube channels.
func WithYouTubeAlerts(cfg YouTubeAlertsConfig) Option {
	return func(s *Server) {
		if cfg.HubURL != "" {
			s.youtubeHubURL = strings.TrimSpace(cfg.HubURL)
		}

		callback := strings.TrimSpace(cfg.CallbackURL)
		if callback == "" {
			s.youtubeAlerts.enabled = false
			s.youtubeAlerts.callbackURL = ""
			s.youtubeAlerts.secret = ""
			s.youtubeAlerts.verifyPref = ""
			s.youtubeAlerts.verifySuff = ""
			return
		}

		s.youtubeAlerts.enabled = true
		s.youtubeAlerts.callbackURL = callback
		s.youtubeAlerts.secret = strings.TrimSpace(cfg.Secret)
		s.youtubeAlerts.verifyPref = strings.TrimSpace(cfg.VerifyTokenPrefix)
		s.youtubeAlerts.verifySuff = strings.TrimSpace(cfg.VerifyTokenSuffix)
	}
}

// WithHTTPClient overrides the HTTP client used for outbound HTTP requests.
func WithHTTPClient(client *http.Client) Option {
	return func(s *Server) {
		if client != nil {
			s.httpClient = client
		}
	}
}

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var payload loginRequest
	if err := decodeJSON(r, &payload); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}

	email := strings.ToLower(strings.TrimSpace(payload.Email))
	password := strings.TrimSpace(payload.Password)

	if email == "" || password == "" {
		respondJSON(w, http.StatusBadRequest, errorPayload{Message: "Email and password are required."})
		return
	}

	s.mu.RLock()
	adminEmail := s.adminEmail
	adminPassword := s.adminPassword
	adminToken := s.adminToken
	s.mu.RUnlock()

	if !constantTimeEquals(email, adminEmail) || !constantTimeEquals(password, adminPassword) {
		respondJSON(w, http.StatusUnauthorized, errorPayload{Message: "Invalid credentials."})
		return
	}

	respondJSON(w, http.StatusOK, loginResponse{Token: adminToken})
}

func (s *Server) handleAdminStreamers(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		streamers, err := s.store.ListStreamers()
		if err != nil {
			respondError(w, http.StatusInternalServerError, err)
			return
		}
		respondJSON(w, http.StatusOK, streamers)
	case http.MethodPost:
		var payload streamerRequest
		if err := decodeJSON(r, &payload); err != nil {
			respondError(w, http.StatusBadRequest, err)
			return
		}
		normalizeStreamer(&payload)
		if errs := validateStreamer(payload); len(errs) > 0 {
			respondJSON(w, http.StatusBadRequest, errorPayload{Message: strings.Join(errs, " ")})
			return
		}
		entry := storage.Streamer{
			Name:        payload.Name,
			Description: payload.Description,
			Status:      payload.Status,
			StatusLabel: payload.StatusLabel,
			Languages:   payload.Languages,
			Platforms:   s.enrichPlatforms(r.Context(), payload.Platforms),
		}
		result, err := s.store.CreateStreamer(entry)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err)
			return
		}
		result.Platforms = entry.Platforms
		respondJSON(w, http.StatusCreated, result)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleAdminStreamerByID(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAdmin(w, r) {
		return
	}

	id := path.Base(r.URL.Path)
	if id == "" || id == "streamers" {
		respondJSON(w, http.StatusBadRequest, errorPayload{Message: "Streamer id is required."})
		return
	}

	switch r.Method {
	case http.MethodPut:
		var payload streamerRequest
		if err := decodeJSON(r, &payload); err != nil {
			respondError(w, http.StatusBadRequest, err)
			return
		}
		payload.ID = id
		normalizeStreamer(&payload)
		if errs := validateStreamer(payload); len(errs) > 0 {
			respondJSON(w, http.StatusBadRequest, errorPayload{Message: strings.Join(errs, " ")})
			return
		}
		entry := storage.Streamer{
			ID:          payload.ID,
			Name:        payload.Name,
			Description: payload.Description,
			Status:      payload.Status,
			StatusLabel: payload.StatusLabel,
			Languages:   payload.Languages,
			Platforms:   s.enrichPlatforms(r.Context(), payload.Platforms),
		}
		result, err := s.store.UpdateStreamer(entry)
		if errors.Is(err, storage.ErrNotFound) {
			respondJSON(w, http.StatusNotFound, errorPayload{Message: "Streamer not found."})
			return
		}
		if err != nil {
			respondError(w, http.StatusInternalServerError, err)
			return
		}
		result.Platforms = entry.Platforms
		respondJSON(w, http.StatusOK, result)
	case http.MethodDelete:
		streamer, err := s.streamerByID(id)
		if errors.Is(err, storage.ErrNotFound) {
			respondJSON(w, http.StatusNotFound, errorPayload{Message: "Streamer not found."})
			return
		}
		if err != nil {
			respondError(w, http.StatusInternalServerError, err)
			return
		}

		err = s.store.DeleteStreamer(id)
		if errors.Is(err, storage.ErrNotFound) {
			respondJSON(w, http.StatusNotFound, errorPayload{Message: "Streamer not found."})
			return
		}
		if err != nil {
			respondError(w, http.StatusInternalServerError, err)
			return
		}
		s.unsubscribeYouTubePlatforms(r.Context(), streamer.Platforms)
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, http.MethodPut, http.MethodDelete)
	}
}

func (s *Server) handleSubmitStreamer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var payload submissionRequest
	if err := decodeJSON(r, &payload); err != nil {
		respondError(w, http.StatusBadRequest, err)
		return
	}
	normalizeSubmission(&payload)
	if errs := validateSubmission(payload); len(errs) > 0 {
		respondJSON(w, http.StatusBadRequest, errorPayload{Message: strings.Join(errs, " ")})
		return
	}

	entry := storage.SubmissionPayload{
		Name:        payload.Name,
		Description: payload.Description,
		Status:      payload.Status,
		StatusLabel: payload.StatusLabel,
		Languages:   payload.Languages,
		Platforms:   payload.Platforms,
	}

	result, err := s.store.AddSubmission(entry)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusAccepted, successPayload{
		Message: "Submission received and queued for review.",
		ID:      result.ID,
	})
}

func (s *Server) handleAdminSubmissions(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAdmin(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		submissions, err := s.store.ListSubmissions()
		if err != nil {
			respondError(w, http.StatusInternalServerError, err)
			return
		}
		respondJSON(w, http.StatusOK, submissions)
	case http.MethodPost:
		var payload adminSubmissionAction
		if err := decodeJSON(r, &payload); err != nil {
			respondError(w, http.StatusBadRequest, err)
			return
		}

		action := strings.ToLower(strings.TrimSpace(payload.Action))
		switch action {
		case "approve":
			streamer, err := s.store.ApproveSubmission(payload.ID)
			if errors.Is(err, storage.ErrNotFound) {
				respondJSON(w, http.StatusNotFound, errorPayload{Message: "Submission not found."})
				return
			}
			if err != nil {
				respondError(w, http.StatusInternalServerError, err)
				return
			}
			streamer.Platforms = s.enrichPlatforms(r.Context(), streamer.Platforms)
			if streamer, err = s.store.UpdateStreamer(streamer); err != nil {
				respondError(w, http.StatusInternalServerError, err)
				return
			}
			respondJSON(w, http.StatusOK, successPayload{
				Message: "Submission approved and added to roster.",
				ID:      streamer.ID,
			})
		case "reject":
			err := s.store.RejectSubmission(payload.ID)
			if errors.Is(err, storage.ErrNotFound) {
				respondJSON(w, http.StatusNotFound, errorPayload{Message: "Submission not found."})
				return
			}
			if err != nil {
				respondError(w, http.StatusInternalServerError, err)
				return
			}
			respondJSON(w, http.StatusOK, successPayload{
				Message: "Submission rejected and removed.",
				ID:      payload.ID,
			})
		default:
			respondJSON(w, http.StatusBadRequest, errorPayload{Message: "Action must be either approve or reject."})
		}
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAdmin(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, s.currentSettingsPayload())
	case http.MethodPut:
		var payload settingsUpdateRequest
		if err := decodeJSON(r, &payload); err != nil {
			respondError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.applySettings(payload); err != nil {
			var vErr validationError
			if errors.As(err, &vErr) {
				respondError(w, http.StatusBadRequest, err)
				return
			}
			respondError(w, http.StatusInternalServerError, err)
			return
		}
		respondJSON(w, http.StatusOK, successPayload{Message: "Settings updated."})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut)
	}
}

func (s *Server) handleAdminYouTubeMonitor(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	events := s.youtubeEventsSnapshot()
	respondJSON(w, http.StatusOK, struct {
		Events []youtubeEvent `json:"events"`
	}{
		Events: events,
	})
}

func (s *Server) currentSettingsPayload() settingsResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hub := strings.TrimSpace(s.youtubeHubURL)
	if hub == "" {
		hub = defaultYouTubeHubURL
	}

	return settingsResponse{
		ListenAddr:                strings.TrimSpace(s.listenAddr),
		AdminToken:                s.adminToken,
		AdminEmail:                s.adminEmail,
		AdminPassword:             s.adminPassword,
		YouTubeAPIKey:             s.youtubeAPIKey,
		DataDir:                   strings.TrimSpace(s.dataDir),
		StaticDir:                 strings.TrimSpace(s.staticDir),
		StreamersFile:             strings.TrimSpace(s.streamersFile),
		SubmissionsFile:           strings.TrimSpace(s.submissionsFile),
		YouTubeAlertsCallback:     s.youtubeAlerts.callbackURL,
		YouTubeAlertsSecret:       s.youtubeAlerts.secret,
		YouTubeAlertsVerifyPrefix: s.youtubeAlerts.verifyPref,
		YouTubeAlertsVerifySuffix: s.youtubeAlerts.verifySuff,
		YouTubeAlertsHubURL:       hub,
	}
}

func (s *Server) applySettings(payload settingsUpdateRequest) error {
	if payload.AdminToken != nil {
		if strings.TrimSpace(*payload.AdminToken) == "" {
			return validationError("admin token cannot be empty")
		}
	}
	if payload.AdminEmail != nil {
		if strings.TrimSpace(*payload.AdminEmail) == "" {
			return validationError("admin email cannot be empty")
		}
	}
	if payload.AdminPassword != nil {
		if strings.TrimSpace(*payload.AdminPassword) == "" {
			return validationError("admin password cannot be empty")
		}
	}
	if payload.YouTubeAPIKey != nil {
		trimmed := strings.TrimSpace(*payload.YouTubeAPIKey)
		*payload.YouTubeAPIKey = trimmed
	}
	if payload.YouTubeAlertsCallback != nil {
		trimmed := strings.TrimSpace(*payload.YouTubeAlertsCallback)
		*payload.YouTubeAlertsCallback = trimmed
	}
	if payload.YouTubeAlertsSecret != nil {
		trimmed := strings.TrimSpace(*payload.YouTubeAlertsSecret)
		*payload.YouTubeAlertsSecret = trimmed
	}
	if payload.YouTubeAlertsVerifyPrefix != nil {
		trimmed := strings.TrimSpace(*payload.YouTubeAlertsVerifyPrefix)
		*payload.YouTubeAlertsVerifyPrefix = trimmed
	}
	if payload.YouTubeAlertsVerifySuffix != nil {
		trimmed := strings.TrimSpace(*payload.YouTubeAlertsVerifySuffix)
		*payload.YouTubeAlertsVerifySuffix = trimmed
	}
	if payload.YouTubeAlertsHubURL != nil {
		trimmed := strings.TrimSpace(*payload.YouTubeAlertsHubURL)
		*payload.YouTubeAlertsHubURL = trimmed
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if payload.AdminToken != nil {
		s.adminToken = strings.TrimSpace(*payload.AdminToken)
		_ = os.Setenv("ADMIN_TOKEN", s.adminToken)
	}
	if payload.AdminEmail != nil {
		s.adminEmail = strings.TrimSpace(*payload.AdminEmail)
		_ = os.Setenv("ADMIN_EMAIL", s.adminEmail)
	}
	if payload.AdminPassword != nil {
		s.adminPassword = strings.TrimSpace(*payload.AdminPassword)
		_ = os.Setenv("ADMIN_PASSWORD", s.adminPassword)
	}
	if payload.YouTubeAPIKey != nil {
		s.youtubeAPIKey = strings.TrimSpace(*payload.YouTubeAPIKey)
		_ = os.Setenv("YOUTUBE_API_KEY", s.youtubeAPIKey)
	}
	if payload.ListenAddr != nil {
		s.listenAddr = strings.TrimSpace(*payload.ListenAddr)
		_ = os.Setenv("LISTEN_ADDR", s.listenAddr)
	}
	if payload.DataDir != nil {
		s.dataDir = strings.TrimSpace(*payload.DataDir)
		_ = os.Setenv("SHARPEN_DATA_DIR", s.dataDir)
	}
	if payload.StaticDir != nil {
		s.staticDir = strings.TrimSpace(*payload.StaticDir)
		_ = os.Setenv("SHARPEN_STATIC_DIR", s.staticDir)
	}
	if payload.StreamersFile != nil {
		s.streamersFile = strings.TrimSpace(*payload.StreamersFile)
		_ = os.Setenv("SHARPEN_STREAMERS_FILE", s.streamersFile)
	}
	if payload.SubmissionsFile != nil {
		s.submissionsFile = strings.TrimSpace(*payload.SubmissionsFile)
		_ = os.Setenv("SHARPEN_SUBMISSIONS_FILE", s.submissionsFile)
	}
	callbackUpdated := payload.YouTubeAlertsCallback != nil
	secretUpdated := payload.YouTubeAlertsSecret != nil
	prefixUpdated := payload.YouTubeAlertsVerifyPrefix != nil
	suffixUpdated := payload.YouTubeAlertsVerifySuffix != nil
	hubUpdated := payload.YouTubeAlertsHubURL != nil

	callback := s.youtubeAlerts.callbackURL
	if payload.YouTubeAlertsCallback != nil {
		callback = *payload.YouTubeAlertsCallback
	}

	secret := s.youtubeAlerts.secret
	if payload.YouTubeAlertsSecret != nil {
		secret = *payload.YouTubeAlertsSecret
	}

	verifyPrefix := s.youtubeAlerts.verifyPref
	if payload.YouTubeAlertsVerifyPrefix != nil {
		verifyPrefix = *payload.YouTubeAlertsVerifyPrefix
	}

	verifySuffix := s.youtubeAlerts.verifySuff
	if payload.YouTubeAlertsVerifySuffix != nil {
		verifySuffix = *payload.YouTubeAlertsVerifySuffix
	}

	hub := s.youtubeHubURL
	hubEnvValue := ""
	if payload.YouTubeAlertsHubURL != nil {
		hubEnvValue = *payload.YouTubeAlertsHubURL
		if hubEnvValue == "" {
			hub = defaultYouTubeHubURL
		} else {
			hub = hubEnvValue
		}
	}

	if callback == "" {
		if secret != "" {
			secret = ""
			secretUpdated = true
		}
		if verifyPrefix != "" {
			verifyPrefix = ""
			prefixUpdated = true
		}
		if verifySuffix != "" {
			verifySuffix = ""
			suffixUpdated = true
		}
		if !hubUpdated && hub != defaultYouTubeHubURL {
			hub = defaultYouTubeHubURL
			hubEnvValue = ""
			hubUpdated = true
		}
	}

	s.youtubeAlerts.callbackURL = callback
	s.youtubeAlerts.secret = secret
	s.youtubeAlerts.verifyPref = verifyPrefix
	s.youtubeAlerts.verifySuff = verifySuffix
	s.youtubeAlerts.enabled = callback != ""
	s.youtubeHubURL = hub

	if callbackUpdated {
		_ = os.Setenv("YOUTUBE_ALERTS_CALLBACK", callback)
	}
	if secretUpdated {
		_ = os.Setenv("YOUTUBE_ALERTS_SECRET", secret)
	}
	if prefixUpdated {
		_ = os.Setenv("YOUTUBE_ALERTS_VERIFY_PREFIX", verifyPrefix)
	}
	if suffixUpdated {
		_ = os.Setenv("YOUTUBE_ALERTS_VERIFY_SUFFIX", verifySuffix)
	}
	if hubUpdated {
		_ = os.Setenv("YOUTUBE_ALERTS_HUB_URL", hubEnvValue)
	}

	if err := s.persistSettings(); err != nil {
		return fmt.Errorf("persist settings: %w", err)
	}

	return nil
}

func (s *Server) authorizeAdmin(w http.ResponseWriter, r *http.Request) bool {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		respondJSON(w, http.StatusUnauthorized, errorPayload{Message: "Missing admin authorization."})
		return false
	}

	const bearer = "Bearer "
	if !strings.HasPrefix(header, bearer) {
		respondJSON(w, http.StatusUnauthorized, errorPayload{Message: "Invalid authorization scheme."})
		return false
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, bearer))
	s.mu.RLock()
	expected := s.adminToken
	s.mu.RUnlock()

	if token == "" || !constantTimeEquals(token, expected) {
		respondJSON(w, http.StatusUnauthorized, errorPayload{Message: "Invalid admin token."})
		return false
	}

	return true
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	return nil
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func respondError(w http.ResponseWriter, status int, err error) {
	respondJSON(w, status, errorPayload{Message: err.Error()})
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	respondJSON(w, http.StatusMethodNotAllowed, errorPayload{Message: "Method Not Allowed"})
}

func constantTimeEquals(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

func (s *Server) streamerByID(id string) (storage.Streamer, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return storage.Streamer{}, storage.ErrNotFound
	}
	streamers, err := s.store.ListStreamers()
	if err != nil {
		return storage.Streamer{}, err
	}
	for _, streamer := range streamers {
		if streamer.ID == trimmed {
			return streamer, nil
		}
	}
	return storage.Streamer{}, storage.ErrNotFound
}

func (s *Server) youtubeEventsSnapshot() []youtubeEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := make([]youtubeEvent, len(s.youtubeEvents))
	copy(events, s.youtubeEvents)
	return events
}

func (s *Server) appendYouTubeEvent(event youtubeEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.youtubeEvents) >= youtubeEventLogLimit {
		copy(s.youtubeEvents, s.youtubeEvents[1:])
		s.youtubeEvents[len(s.youtubeEvents)-1] = event
		return
	}
	s.youtubeEvents = append(s.youtubeEvents, event)
}

func normalizeSettings(value settings.Settings) settings.Settings {
	value.AdminToken = strings.TrimSpace(value.AdminToken)
	value.AdminEmail = strings.TrimSpace(value.AdminEmail)
	value.AdminPassword = strings.TrimSpace(value.AdminPassword)
	value.YouTubeAPIKey = strings.TrimSpace(value.YouTubeAPIKey)
	value.YouTubeAlertsCallback = strings.TrimSpace(value.YouTubeAlertsCallback)
	value.YouTubeAlertsSecret = strings.TrimSpace(value.YouTubeAlertsSecret)
	value.YouTubeAlertsVerifyPrefix = strings.TrimSpace(value.YouTubeAlertsVerifyPrefix)
	value.YouTubeAlertsVerifySuffix = strings.TrimSpace(value.YouTubeAlertsVerifySuffix)
	value.YouTubeAlertsHubURL = strings.TrimSpace(value.YouTubeAlertsHubURL)
	value.ListenAddr = strings.TrimSpace(value.ListenAddr)
	value.DataDir = strings.TrimSpace(value.DataDir)
	value.StaticDir = strings.TrimSpace(value.StaticDir)
	value.StreamersFile = strings.TrimSpace(value.StreamersFile)
	value.SubmissionsFile = strings.TrimSpace(value.SubmissionsFile)
	return value
}

func (s *Server) persistSettings() error {
	if s.settingsStore == nil {
		return nil
	}
	settingsPayload := settings.Settings{
		AdminToken:                s.adminToken,
		AdminEmail:                s.adminEmail,
		AdminPassword:             s.adminPassword,
		YouTubeAPIKey:             s.youtubeAPIKey,
		YouTubeAlertsCallback:     s.youtubeAlerts.callbackURL,
		YouTubeAlertsSecret:       s.youtubeAlerts.secret,
		YouTubeAlertsVerifyPrefix: s.youtubeAlerts.verifyPref,
		YouTubeAlertsVerifySuffix: s.youtubeAlerts.verifySuff,
		YouTubeAlertsHubURL:       s.youtubeHubURL,
		ListenAddr:                s.listenAddr,
		DataDir:                   s.dataDir,
		StaticDir:                 s.staticDir,
		StreamersFile:             s.streamersFile,
		SubmissionsFile:           s.submissionsFile,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.settingsStore.Save(ctx, settingsPayload)
}
