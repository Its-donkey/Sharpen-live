package server

import "github.com/Its-donkey/Sharpen-live/backend/internal/storage"

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
