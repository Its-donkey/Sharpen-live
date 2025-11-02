package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound indicates that the requested entity does not exist.
var ErrNotFound = errors.New("storage: not found")

// Platform captures the streaming destinations for a streamer.
type Platform struct {
	Name       string `json:"name"`
	ChannelURL string `json:"channelUrl"`
	LiveURL    string `json:"liveUrl"`
}

// Streamer models a featured streamer entry rendered on the roster.
type Streamer struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	StatusLabel string     `json:"statusLabel"`
	Languages   []string   `json:"languages"`
	Platforms   []Platform `json:"platforms"`
}

// SubmissionPayload mirrors the incoming submission request body.
type SubmissionPayload struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	StatusLabel string     `json:"statusLabel"`
	Languages   []string   `json:"languages"`
	Platforms   []Platform `json:"platforms"`
}

// Submission stores pending streamer submissions awaiting moderation.
type Submission struct {
	ID          string            `json:"id"`
	SubmittedAt time.Time         `json:"submittedAt"`
	Payload     SubmissionPayload `json:"payload"`
}

// JSONStore persists streamer and submission data to JSON files on disk.
type JSONStore struct {
	mu              sync.Mutex
	streamersPath   string
	submissionsPath string
}

// NewJSONStore returns a configured store that reads and writes JSON payloads.
func NewJSONStore(streamersPath, submissionsPath string) (*JSONStore, error) {
	if streamersPath == "" {
		return nil, errors.New("storage: streamers path is required")
	}
	if submissionsPath == "" {
		return nil, errors.New("storage: submissions path is required")
	}

	store := &JSONStore{
		streamersPath:   streamersPath,
		submissionsPath: submissionsPath,
	}

	if err := store.ensureFile(streamersPath); err != nil {
		return nil, err
	}
	if err := store.ensureFile(submissionsPath); err != nil {
		return nil, err
	}

	return store, nil
}

// ListStreamers returns all registered streamers.
func (s *JSONStore) ListStreamers() ([]Streamer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var streamers []Streamer
	if err := s.readJSON(s.streamersPath, &streamers); err != nil {
		return nil, err
	}
	return streamers, nil
}

// CreateStreamer adds a new streamer entry and returns the stored record.
func (s *JSONStore) CreateStreamer(entry Streamer) (Streamer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var streamers []Streamer
	if err := s.readJSON(s.streamersPath, &streamers); err != nil {
		return Streamer{}, err
	}

	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}

	streamers = append(streamers, entry)

	if err := s.writeJSON(s.streamersPath, streamers); err != nil {
		return Streamer{}, err
	}
	return entry, nil
}

// UpdateStreamer replaces an existing streamer with the provided payload.
func (s *JSONStore) UpdateStreamer(entry Streamer) (Streamer, error) {
	if entry.ID == "" {
		return Streamer{}, errors.New("storage: update requires streamer id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var streamers []Streamer
	if err := s.readJSON(s.streamersPath, &streamers); err != nil {
		return Streamer{}, err
	}

	index := -1
	for i, streamer := range streamers {
		if streamer.ID == entry.ID {
			index = i
			break
		}
	}
	if index == -1 {
		return Streamer{}, ErrNotFound
	}

	streamers[index] = entry

	if err := s.writeJSON(s.streamersPath, streamers); err != nil {
		return Streamer{}, err
	}
	return entry, nil
}

// DeleteStreamer removes a streamer from the roster.
func (s *JSONStore) DeleteStreamer(id string) error {
	if id == "" {
		return errors.New("storage: delete requires streamer id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var streamers []Streamer
	if err := s.readJSON(s.streamersPath, &streamers); err != nil {
		return err
	}

	index := -1
	for i, streamer := range streamers {
		if streamer.ID == id {
			index = i
			break
		}
	}

	if index == -1 {
		return ErrNotFound
	}

	streamers = append(streamers[:index], streamers[index+1:]...)
	return s.writeJSON(s.streamersPath, streamers)
}

// ListSubmissions returns all pending submissions.
func (s *JSONStore) ListSubmissions() ([]Submission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var submissions []Submission
	if err := s.readJSON(s.submissionsPath, &submissions); err != nil {
		return nil, err
	}
	return submissions, nil
}

// AddSubmission enqueues a new submission for moderation.
func (s *JSONStore) AddSubmission(payload SubmissionPayload) (Submission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var submissions []Submission
	if err := s.readJSON(s.submissionsPath, &submissions); err != nil {
		return Submission{}, err
	}

	entry := Submission{
		ID:          uuid.NewString(),
		SubmittedAt: time.Now().UTC(),
		Payload:     payload,
	}
	submissions = append(submissions, entry)

	if err := s.writeJSON(s.submissionsPath, submissions); err != nil {
		return Submission{}, err
	}
	return entry, nil
}

// ApproveSubmission promotes a submission to the streamer roster.
func (s *JSONStore) ApproveSubmission(id string) (Streamer, error) {
	if id == "" {
		return Streamer{}, errors.New("storage: approve requires submission id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var submissions []Submission
	if err := s.readJSON(s.submissionsPath, &submissions); err != nil {
		return Streamer{}, err
	}

	index := -1
	for i, submission := range submissions {
		if submission.ID == id {
			index = i
			break
		}
	}
	if index == -1 {
		return Streamer{}, ErrNotFound
	}

	submission := submissions[index]
	streamer := Streamer{
		ID:          uuid.NewString(),
		Name:        submission.Payload.Name,
		Description: submission.Payload.Description,
		Status:      submission.Payload.Status,
		StatusLabel: submission.Payload.StatusLabel,
		Languages:   append([]string(nil), submission.Payload.Languages...),
		Platforms:   append([]Platform(nil), submission.Payload.Platforms...),
	}

	var streamers []Streamer
	if err := s.readJSON(s.streamersPath, &streamers); err != nil {
		return Streamer{}, err
	}
	streamers = append(streamers, streamer)

	submissions = append(submissions[:index], submissions[index+1:]...)

	if err := s.writeJSON(s.streamersPath, streamers); err != nil {
		return Streamer{}, err
	}
	if err := s.writeJSON(s.submissionsPath, submissions); err != nil {
		return Streamer{}, err
	}

	return streamer, nil
}

// RejectSubmission removes a submission without promoting it.
func (s *JSONStore) RejectSubmission(id string) error {
	if id == "" {
		return errors.New("storage: reject requires submission id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var submissions []Submission
	if err := s.readJSON(s.submissionsPath, &submissions); err != nil {
		return err
	}

	index := -1
	for i, submission := range submissions {
		if submission.ID == id {
			index = i
			break
		}
	}
	if index == -1 {
		return ErrNotFound
	}

	submissions = append(submissions[:index], submissions[index+1:]...)
	return s.writeJSON(s.submissionsPath, submissions)
}

func (s *JSONStore) ensureFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString("[]\n")
	return err
}

func (s *JSONStore) readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		data = []byte("[]")
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("storage: decode %s: %w", filepath.Base(path), err)
	}
	return nil
}

func (s *JSONStore) writeJSON(path string, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
