package server

import "github.com/Its-donkey/Sharpen-live/api/internal/storage"

type streamerRequest struct {
	ID          string             `json:"id,omitempty"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Status      string             `json:"status"`
	StatusLabel string             `json:"statusLabel"`
	Languages   []string           `json:"languages"`
	Platforms   []storage.Platform `json:"platforms"`
}

type submissionRequest struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Status      string             `json:"status"`
	StatusLabel string             `json:"statusLabel"`
	Languages   []string           `json:"languages"`
	Platforms   []storage.Platform `json:"platforms"`
}

type adminSubmissionAction struct {
	Action string `json:"action"`
	ID     string `json:"id"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type errorPayload struct {
	Message string `json:"message"`
}

type successPayload struct {
	Message string `json:"message"`
	ID      string `json:"id,omitempty"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type settingsResponse struct {
	ListenAddr      string `json:"listenAddr"`
	AdminToken      string `json:"adminToken"`
	AdminEmail      string `json:"adminEmail"`
	AdminPassword   string `json:"adminPassword"`
	YouTubeAPIKey   string `json:"youtubeApiKey"`
	DataDir         string `json:"dataDir"`
	StaticDir       string `json:"staticDir"`
	StreamersFile   string `json:"streamersFile"`
	SubmissionsFile string `json:"submissionsFile"`
}

type settingsUpdateRequest struct {
	ListenAddr      *string `json:"listenAddr"`
	AdminToken      *string `json:"adminToken"`
	AdminEmail      *string `json:"adminEmail"`
	AdminPassword   *string `json:"adminPassword"`
	YouTubeAPIKey   *string `json:"youtubeApiKey"`
	DataDir         *string `json:"dataDir"`
	StaticDir       *string `json:"staticDir"`
	StreamersFile   *string `json:"streamersFile"`
	SubmissionsFile *string `json:"submissionsFile"`
}
