package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultDataDir        = "api/data"
	streamersFileName     = "streamers.json"
	submissionsFileName   = "submissions.json"
	defaultStatusLabel    = "Offline"
	maxStreamerNameLength = 80
	maxDescriptionLength  = 480
	maxPlatformsPerEntry  = 8
	maxLanguagesPerEntry  = 8
	adminTokenEnvKey      = "ADMIN_TOKEN" // documented for parity with admin endpoints
	dataDirEnvKey         = "SHARPEN_DATA_DIR"
	streamersFileEnvKey   = "SHARPEN_STREAMERS_FILE"
	submissionsFileEnvKey = "SHARPEN_SUBMISSIONS_FILE"
)

type submissionRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	StatusLabel string          `json:"statusLabel"`
	Languages   []string        `json:"languages"`
	Platforms   []platformEntry `json:"platforms"`
}

type platformEntry struct {
	Name       string `json:"name"`
	ChannelURL string `json:"channelUrl"`
	LiveURL    string `json:"liveUrl"`
}

type storedSubmission struct {
	ID          string            `json:"id"`
	SubmittedAt time.Time         `json:"submittedAt"`
	Payload     submissionRequest `json:"payload"`
}

type submissionResponse struct {
	Message string `json:"message"`
	ID      string `json:"id"`
}

var statusDefaults = map[string]string{
	"online":  "Online",
	"busy":    "Workshop",
	"offline": "Offline",
}

// Handler receives streamer submissions and stores them for review.
func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"message": "Method Not Allowed",
		})
		return
	}

	req, err := decodeSubmission(r.Body)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"message": err.Error(),
		})
		return
	}

	id, err := appendSubmission(req)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"message": err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusAccepted, submissionResponse{
		Message: "Submission received and queued for review.",
		ID:      id,
	})
}

func decodeSubmission(body io.ReadCloser) (*submissionRequest, error) {
	defer body.Close()

	var payload submissionRequest
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid submission payload: %w", err)
	}

	normalizeSubmission(&payload)
	if errs := validateSubmission(payload); len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, " "))
	}

	return &payload, nil
}

func normalizeSubmission(req *submissionRequest) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.StatusLabel = strings.TrimSpace(req.StatusLabel)

	req.Languages = filterStrings(req.Languages, maxLanguagesPerEntry)
	req.Platforms = filterPlatforms(req.Platforms, maxPlatformsPerEntry)

	if req.StatusLabel == "" && statusDefaults[req.Status] != "" {
		req.StatusLabel = statusDefaults[req.Status]
	} else if req.StatusLabel == "" {
		req.StatusLabel = defaultStatusLabel
	}
}

func validateSubmission(req submissionRequest) []string {
	var errs []string

	if req.Name == "" {
		errs = append(errs, "Streamer name is required.")
	} else if len(req.Name) > maxStreamerNameLength {
		errs = append(errs, fmt.Sprintf("Streamer name must be under %d characters.", maxStreamerNameLength))
	}

	if req.Description == "" {
		errs = append(errs, "Description is required.")
	} else if len(req.Description) > maxDescriptionLength {
		errs = append(errs, fmt.Sprintf("Description must be under %d characters.", maxDescriptionLength))
	}

	if req.Status == "" || statusDefaults[req.Status] == "" {
		errs = append(errs, "Status is required and must be one of: online, busy, or offline.")
	}

	if len(req.Languages) == 0 {
		errs = append(errs, "At least one language is required.")
	}

	if len(req.Platforms) == 0 {
		errs = append(errs, "At least one platform with channel and live URLs is required.")
	}

	return errs
}

func filterStrings(values []string, max int) []string {
	result := make([]string, 0, len(values))
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			result = append(result, trimmed)
		}
		if len(result) >= max && max > 0 {
			break
		}
	}
	return result
}

func filterPlatforms(values []platformEntry, max int) []platformEntry {
	result := make([]platformEntry, 0, len(values))
	for _, v := range values {
		entry := platformEntry{
			Name:       strings.TrimSpace(v.Name),
			ChannelURL: strings.TrimSpace(v.ChannelURL),
			LiveURL:    strings.TrimSpace(v.LiveURL),
		}
		if entry.Name == "" || entry.ChannelURL == "" || entry.LiveURL == "" {
			continue
		}
		result = append(result, entry)
		if len(result) >= max && max > 0 {
			break
		}
	}
	return result
}

func appendSubmission(req *submissionRequest) (string, error) {
	submissionsPath := resolveDataPath(submissionsFileEnvKey, submissionsFileName)
	submissions, err := readSubmissions(submissionsPath)
	if err != nil {
		return "", err
	}

	entry := storedSubmission{
		ID:          uuid.NewString(),
		SubmittedAt: time.Now().UTC(),
		Payload:     *req,
	}
	submissions = append(submissions, entry)

	if err := writeJSON(submissionsPath, submissions); err != nil {
		return "", err
	}

	return entry.ID, nil
}

func readSubmissions(path string) ([]storedSubmission, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := bootstrapDataFile(path, []storedSubmission{}); err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var submissions []storedSubmission
	if len(data) == 0 {
		return submissions, nil
	}

	if err := json.Unmarshal(data, &submissions); err != nil {
		return nil, fmt.Errorf("unable to parse submissions: %w", err)
	}

	return submissions, nil
}

func resolveDataPath(envKey, fileName string) string {
	if path := strings.TrimSpace(os.Getenv(envKey)); path != "" {
		return path
	}
	dir := strings.TrimSpace(os.Getenv(dataDirEnvKey))
	if dir == "" {
		dir = defaultDataDir
	}
	return filepath.Join(dir, fileName)
}

func writeJSON(path string, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func bootstrapDataFile(path string, payload any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return writeJSON(path, payload)
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}
