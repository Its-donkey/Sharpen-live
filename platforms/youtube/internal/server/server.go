package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Its-donkey/Sharpen-live/platforms/youtube/internal/alerts"
)

// AlertProcessor processes incoming stream alerts.
type AlertProcessor interface {
	Handle(ctx context.Context, alert alerts.StreamAlert) error
}

// Logger mirrors the log.Printf signature.
type Logger interface {
	Printf(format string, v ...any)
}

// Config configures a Server instance.
type Config struct {
	Processor AlertProcessor
	Logger    Logger
}

// Server exposes HTTP endpoints for alert delivery.
type Server struct {
	processor AlertProcessor
	logger    Logger
}

// New constructs a Server.
func New(cfg Config) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = httpLogger{}
	}

	return &Server{processor: cfg.Processor, logger: logger}
}

// Routes returns the HTTP handler for the server.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/alerts", s.handleAlerts())
	return mux
}

func (s *Server) handleAlerts() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			s.handleVerification(w, r)
			return
		}

		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost+", "+http.MethodGet)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var alert alerts.StreamAlert
		if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
			s.logger.Printf("invalid payload: %v", err)
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if s.processor == nil {
			http.Error(w, "alert processor unavailable", http.StatusServiceUnavailable)
			return
		}

		if err := s.processor.Handle(r.Context(), alert); err != nil {
			if errors.Is(err, alerts.ErrMissingChannelID) {
				http.Error(w, "channelId is required", http.StatusBadRequest)
				return
			}
			s.logger.Printf("alert processing failed: %v", err)
			http.Error(w, "processing error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	})
}

func (s *Server) handleVerification(w http.ResponseWriter, r *http.Request) {
	challenge := r.URL.Query().Get("hub.challenge")
	mode := r.URL.Query().Get("hub.mode")
	topic := r.URL.Query().Get("hub.topic")
	verifyToken := r.URL.Query().Get("hub.verify_token")

	if challenge == "" {
		http.Error(w, "missing hub.challenge", http.StatusBadRequest)
		return
	}

	s.logger.Printf("verification request: mode=%s topic=%s verify_token=%s", mode, topic, verifyToken)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(challenge))
	s.logger.Printf("verification response sent: status=200 challenge=%s", challenge)
}

type httpLogger struct{}

func (httpLogger) Printf(string, ...any) {}
