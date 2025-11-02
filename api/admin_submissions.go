package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type adminActionRequest struct {
	Action string `json:"action"`
	ID     string `json:"id"`
}

type adminSubmissionsResponse struct {
	Submissions []storedSubmission `json:"submissions"`
}

// HandlerAdminSubmissions exposes management endpoints for pending submissions.
func HandlerAdminSubmissions(w http.ResponseWriter, r *http.Request) {
	if err := authorizeAdmin(r); err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{
			"message": err.Error(),
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		handleAdminList(w, r)
	case http.MethodPost:
		handleAdminAction(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"message": "Method Not Allowed",
		})
	}
}

func authorizeAdmin(r *http.Request) error {
	expected := strings.TrimSpace(os.Getenv(adminTokenEnvKey))
	if expected == "" {
		return errors.New("admin access is not configured")
	}

	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return errors.New("missing admin authorization")
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(header, bearerPrefix) {
		return errors.New("invalid authorization scheme")
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
	if token == "" || subtleConstantTimeCompare(token, expected) == false {
		return errors.New("invalid admin token")
	}
	return nil
}

func handleAdminList(w http.ResponseWriter, _ *http.Request) {
	submissionsPath := resolveDataPath(submissionsFileEnvKey, submissionsFileName)
	submissions, err := readSubmissions(submissionsPath)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"message": err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, adminSubmissionsResponse{Submissions: submissions})
}

func handleAdminAction(w http.ResponseWriter, r *http.Request) {
	var payload adminActionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"message": fmt.Sprintf("invalid action payload: %v", err),
		})
		return
	}
	payload.Action = strings.ToLower(strings.TrimSpace(payload.Action))
	payload.ID = strings.TrimSpace(payload.ID)

	if payload.ID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"message": "Submission id is required.",
		})
		return
	}

	switch payload.Action {
	case "approve":
		if err := approveSubmission(payload.ID); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{
			"message": "Submission approved and added to roster.",
		})
	case "reject":
		if err := deleteSubmission(payload.ID); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{
			"message": "Submission rejected and removed.",
		})
	default:
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"message": "Action must be either approve or reject.",
		})
	}
}

func approveSubmission(id string) error {
	submissionsPath := resolveDataPath(submissionsFileEnvKey, submissionsFileName)
	streamersPath := resolveDataPath(streamersFileEnvKey, streamersFileName)

	submissions, err := readSubmissions(submissionsPath)
	if err != nil {
		return err
	}

	index := -1
	for i, sub := range submissions {
		if sub.ID == id {
			index = i
			break
		}
	}

	if index == -1 {
		return errors.New("submission not found")
	}

	streamers, err := readStreamers(streamersPath)
	if err != nil {
		return err
	}

	selected := submissions[index]
	streamers = append(streamers, streamerEntry{
		Name:        selected.Payload.Name,
		Description: selected.Payload.Description,
		Status:      selected.Payload.Status,
		StatusLabel: selected.Payload.StatusLabel,
		Languages:   selected.Payload.Languages,
		Platforms:   selected.Payload.Platforms,
	})

	submissions = append(submissions[:index], submissions[index+1:]...)

	if err := writeJSON(streamersPath, streamers); err != nil {
		return err
	}
	return writeJSON(submissionsPath, submissions)
}

func deleteSubmission(id string) error {
	submissionsPath := resolveDataPath(submissionsFileEnvKey, submissionsFileName)
	submissions, err := readSubmissions(submissionsPath)
	if err != nil {
		return err
	}

	index := -1
	for i, sub := range submissions {
		if sub.ID == id {
			index = i
			break
		}
	}

	if index == -1 {
		return errors.New("submission not found")
	}

	submissions = append(submissions[:index], submissions[index+1:]...)
	return writeJSON(submissionsPath, submissions)
}

type streamerEntry struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	StatusLabel string          `json:"statusLabel"`
	Languages   []string        `json:"languages"`
	Platforms   []platformEntry `json:"platforms"`
}

func readStreamers(path string) ([]streamerEntry, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := bootstrapDataFile(path, []streamerEntry{}); err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return []streamerEntry{}, nil
	}

	var entries []streamerEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("unable to parse streamer roster: %w", err)
	}
	return entries, nil
}

func subtleConstantTimeCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
