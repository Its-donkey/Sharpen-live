package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/Its-donkey/Sharpen-live/api/internal/storage"
)

// Server exposes HTTP handlers backed by the storage layer.
type Server struct {
	store         *storage.JSONStore
	adminToken    string
	adminEmail    string
	adminPassword string
	youtubeAPIKey string
	httpClient    *http.Client
	mu            sync.RWMutex
}

// New constructs a Server with the provided dependencies.
func New(store *storage.JSONStore, adminToken, adminEmail, adminPassword, youtubeAPIKey string) *Server {
	return &Server{
		store:         store,
		adminToken:    strings.TrimSpace(adminToken),
		adminEmail:    strings.ToLower(strings.TrimSpace(adminEmail)),
		adminPassword: strings.TrimSpace(adminPassword),
		youtubeAPIKey: strings.TrimSpace(youtubeAPIKey),
		httpClient:    &http.Client{Timeout: 10 * time.Second},
	}
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
	expectedEmail := s.adminEmail
	expectedPassword := s.adminPassword
	s.mu.RUnlock()

	if !constantTimeEquals(email, expectedEmail) || !constantTimeEquals(password, expectedPassword) {
		respondJSON(w, http.StatusUnauthorized, errorPayload{Message: "Invalid credentials."})
		return
	}

	s.mu.RLock()
	token := s.adminToken
	s.mu.RUnlock()

	respondJSON(w, http.StatusOK, loginResponse{Token: token})
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
		err := s.store.DeleteStreamer(id)
		if errors.Is(err, storage.ErrNotFound) {
			respondJSON(w, http.StatusNotFound, errorPayload{Message: "Streamer not found."})
			return
		}
		if err != nil {
			respondError(w, http.StatusInternalServerError, err)
			return
		}
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
			respondError(w, http.StatusBadRequest, err)
			return
		}
		respondJSON(w, http.StatusOK, successPayload{Message: "Settings updated."})
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut)
	}
}

func (s *Server) currentSettingsPayload() settingsResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return settingsResponse{
		ListenAddr:      strings.TrimSpace(os.Getenv("LISTEN_ADDR")),
		AdminToken:      s.adminToken,
		AdminEmail:      s.adminEmail,
		AdminPassword:   s.adminPassword,
		YouTubeAPIKey:   s.youtubeAPIKey,
		DataDir:         strings.TrimSpace(os.Getenv("SHARPEN_DATA_DIR")),
		StaticDir:       strings.TrimSpace(os.Getenv("SHARPEN_STATIC_DIR")),
		StreamersFile:   strings.TrimSpace(os.Getenv("SHARPEN_STREAMERS_FILE")),
		SubmissionsFile: strings.TrimSpace(os.Getenv("SHARPEN_SUBMISSIONS_FILE")),
	}
}

func (s *Server) applySettings(payload settingsUpdateRequest) error {
	if payload.AdminToken != nil {
		if strings.TrimSpace(*payload.AdminToken) == "" {
			return errors.New("admin token cannot be empty")
		}
	}
	if payload.AdminEmail != nil {
		if strings.TrimSpace(*payload.AdminEmail) == "" {
			return errors.New("admin email cannot be empty")
		}
	}
	if payload.AdminPassword != nil {
		if strings.TrimSpace(*payload.AdminPassword) == "" {
			return errors.New("admin password cannot be empty")
		}
	}
	if payload.YouTubeAPIKey != nil {
		trimmed := strings.TrimSpace(*payload.YouTubeAPIKey)
		*payload.YouTubeAPIKey = trimmed
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
		_ = os.Setenv("LISTEN_ADDR", strings.TrimSpace(*payload.ListenAddr))
	}
	if payload.DataDir != nil {
		_ = os.Setenv("SHARPEN_DATA_DIR", strings.TrimSpace(*payload.DataDir))
	}
	if payload.StaticDir != nil {
		_ = os.Setenv("SHARPEN_STATIC_DIR", strings.TrimSpace(*payload.StaticDir))
	}
	if payload.StreamersFile != nil {
		_ = os.Setenv("SHARPEN_STREAMERS_FILE", strings.TrimSpace(*payload.StreamersFile))
	}
	if payload.SubmissionsFile != nil {
		_ = os.Setenv("SHARPEN_SUBMISSIONS_FILE", strings.TrimSpace(*payload.SubmissionsFile))
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
