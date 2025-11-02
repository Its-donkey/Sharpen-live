package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/Its-donkey/Sharpen-live/api/internal/storage"
)

// Server exposes HTTP handlers backed by the storage layer.
type Server struct {
	store         *storage.JSONStore
	adminToken    string
	adminEmail    string
	adminPassword string
}

// New constructs a Server with the provided dependencies.
func New(store *storage.JSONStore, adminToken, adminEmail, adminPassword string) *Server {
	return &Server{
		store:         store,
		adminToken:    strings.TrimSpace(adminToken),
		adminEmail:    strings.ToLower(strings.TrimSpace(adminEmail)),
		adminPassword: strings.TrimSpace(adminPassword),
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

	if !constantTimeEquals(email, s.adminEmail) || !constantTimeEquals(password, s.adminPassword) {
		respondJSON(w, http.StatusUnauthorized, errorPayload{Message: "Invalid credentials."})
		return
	}

	respondJSON(w, http.StatusOK, loginResponse{Token: s.adminToken})
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
			Platforms:   payload.Platforms,
		}
		result, err := s.store.CreateStreamer(entry)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err)
			return
		}
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
			Platforms:   payload.Platforms,
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
	if token == "" || !constantTimeEquals(token, s.adminToken) {
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
