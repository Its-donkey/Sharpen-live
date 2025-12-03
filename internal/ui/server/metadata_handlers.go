package server

import (
	"encoding/json"
	"net/http"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

// handleMetadata handles POST requests to /api/metadata.
func (s *server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.metadataService == nil {
		http.Error(w, "metadata service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Parse request body
	var req model.MetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate URL
	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Fetch metadata
	result, err := s.metadataService.Fetch(r.Context(), req.URL)

	var resp model.MetadataResponse
	if result != nil {
		resp = model.MetadataResponse{
			Title:       result.Title,
			Description: result.Description,
			Handle:      result.Handle,
			ChannelID:   result.ChannelID,
			Languages:   result.Languages,
		}
	}

	// Always return 200 OK with available data (graceful degradation)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Ensure default language
	if len(resp.Languages) == 0 {
		resp.Languages = []string{"English"}
	}

	// Log the fetch (info level for success, warn for errors)
	switch {
	case err != nil:
		s.logger.Debug("metadata fetch", "failed to fetch metadata", map[string]any{
			"url":   req.URL,
			"error": err.Error(),
		})
	case result == nil:
		s.logger.Warn("metadata fetch", "metadata unavailable", map[string]any{
			"url": req.URL,
		})
	default:
		s.logger.Info("metadata fetch", "metadata fetched successfully", map[string]any{
			"url":       req.URL,
			"title":     resp.Title,
			"languages": resp.Languages,
		})
	}

	// Encode and send response
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Error("metadata response", "failed to encode response", err, nil)
	}
}
