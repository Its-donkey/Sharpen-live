package v1

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	adminauth "github.com/Its-donkey/Sharpen-live/internal/alert/admin/auth"
	adminhttp "github.com/Its-donkey/Sharpen-live/internal/alert/admin/http"
	adminservice "github.com/Its-donkey/Sharpen-live/internal/alert/admin/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
	"github.com/Its-donkey/Sharpen-live/internal/alert/logging"
	youtubehandlers "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/handlers"
	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/liveinfo"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	streamhandlers "github.com/Its-donkey/Sharpen-live/internal/alert/streamers/handlers"
	streamsvc "github.com/Its-donkey/Sharpen-live/internal/alert/streamers/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/submissions"
)

const rootPlaceholder = "Sharpen Live alerts service (API disabled).\n"

// Options configures the HTTP router.
type Options struct {
	Logger             logging.Logger
	StreamersPath      string
	StreamersStore     *streamers.Store
	YouTube            config.YouTubeConfig
	Admin              config.AdminConfig
	AlertNotifications youtubehandlers.AlertNotificationOptions
}

// NewRouter constructs the HTTP router for the public API.
func NewRouter(opts Options) http.Handler {
	mux := http.NewServeMux()
	logger := opts.Logger
	streamersPath := opts.StreamersPath
	if streamersPath == "" {
		streamersPath = streamers.DefaultFilePath
	}
	streamersStore := opts.StreamersStore
	if streamersStore == nil {
		streamersStore = streamers.NewStore(streamersPath)
	}

	alertsOpts := opts.AlertNotifications
	if alertsOpts.Logger == nil {
		alertsOpts.Logger = logger
	}
	if alertsOpts.StreamersStore == nil {
		alertsOpts.StreamersStore = streamersStore
	}
	if alertsOpts.VideoLookup == nil {
		alertsOpts.VideoLookup = &liveinfo.Client{Logger: logger}
	}

	adminMgr := adminauth.NewManager(adminauth.Config{
		Email:    opts.Admin.Email,
		Password: opts.Admin.Password,
		TokenTTL: time.Duration(opts.Admin.TokenTTLSeconds) * time.Second,
	})
	authSvc := adminservice.AuthService{Manager: adminMgr}

	adminAuthorized := func(r *http.Request) bool {
		if r == nil {
			return false
		}
		if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
			return true
		}
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(header), "bearer ") {
			if strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")) != "" {
				return true
			}
		}
		return false
	}

	streamSvc := streamsvc.New(streamsvc.Options{
		Streamers:     streamersStore,
		Submissions:   submissions.NewStore(submissions.DefaultFilePath),
		YouTubeClient: &http.Client{Timeout: 8 * time.Second},
		YouTubeHubURL: opts.YouTube.HubURL,
	})

	alertsHandler := handleAlerts(alertsOpts)
	mux.Handle("/alerts", alertsHandler)
	mux.Handle("/alert", alertsHandler)
	mux.Handle("/api/youtube/metadata", youtubehandlers.NewMetadataHandler(youtubehandlers.MetadataHandlerOptions{
		Logger: logger,
	}))

	mux.Handle("/api/admin/login", adminhttp.NewLoginHandler(adminhttp.LoginHandlerOptions{
		Service: authSvc,
		Manager: adminMgr,
	}))
	mux.Handle("/api/admin/submissions", adminhttp.NewSubmissionsHandler(adminhttp.SubmissionsHandlerOptions{
		Authorizer:       adminservice.AuthService{Manager: adminMgr},
		Manager:          adminMgr,
		SubmissionsStore: submissions.NewStore(submissions.DefaultFilePath),
		StreamersStore:   streamersStore,
		YouTubeClient:    &http.Client{Timeout: 8 * time.Second},
		Logger:           logger,
		YouTube:          opts.YouTube,
	}))
	mux.Handle("/api/admin/monitor/youtube", adminhttp.NewMonitorHandler(adminhttp.MonitorHandlerOptions{
		Authorizer:     adminservice.AuthService{Manager: adminMgr},
		Manager:        adminMgr,
		Logger:         logger,
		StreamersStore: streamersStore,
		YouTube:        opts.YouTube,
	}))
	mux.HandleFunc("/api/admin/settings", func(w http.ResponseWriter, r *http.Request) {
		if !adminAuthorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodGet:
			settings := adminSettingsResponse{
				ListenAddr:      opts.YouTube.CallbackURL,
				AdminEmail:      opts.Admin.Email,
				StreamersFile:   streamersPath,
				SubmissionsFile: submissions.DefaultFilePath,
			}
			respondJSON(w, settings)
		case http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Allow", "GET, PUT")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.Handle("/api/streamers", streamhandlers.StreamersHandler(streamhandlers.StreamOptions{
		Service: streamSvc,
		Logger:  logger,
	}))
	mux.Handle("/api/streamers/watch", streamersWatchHandler(streamersWatchOptions{
		FilePath: streamersPath,
		Logger:   logger,
	}))
	mux.HandleFunc("/admin/logs", func(w http.ResponseWriter, r *http.Request) {
		if !adminAuthorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		writeHTTPLogs(w, r)
	})

	mux.HandleFunc("/streamers.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.ServeFile(w, r, streamersPath)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, rootPlaceholder)
	})

	return logging.WithHTTPLogging(mux, logger)
}

// handleAlerts returns an HTTP handler that only treats likely Google/YouTube
// requests as WebSub subscription confirmations/notifications.
func handleAlerts(notificationOpts youtubehandlers.AlertNotificationOptions) http.Handler {
	allowedMethods := strings.Join([]string{http.MethodGet, http.MethodPost}, ", ")
	logger := notificationOpts.Logger
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/alerts" && r.URL.Path != "/alert" {
			http.NotFound(w, r)
			return
		}

		userAgent := r.Header.Get("User-Agent")
		from := r.Header.Get("From")
		forwardedFor := r.Header.Get("X-Forwarded-For")
		platform := alertPlatform(userAgent, from)

		switch r.Method {
		case http.MethodGet:
			if platform == "youtube" {
				if youtubehandlers.HandleSubscriptionConfirmation(w, r, youtubehandlers.SubscriptionConfirmationOptions{
					Logger:         logger,
					StreamersStore: notificationOpts.StreamersStore,
				}) {
					return
				}
				http.Error(w, "invalid subscription confirmation", http.StatusBadRequest)
				return
			}
			if logger != nil {
				logger.Printf("suspicious /alerts GET request: platform=%q ua=%q from=%q xff=%q", platform, userAgent, from, forwardedFor)
			}
			w.Header().Set("Allow", allowedMethods)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		case http.MethodPost:
			if platform != "youtube" {
				if logger != nil {
					logger.Printf("suspicious /alerts POST request: platform=%q ua=%q from=%q xff=%q", platform, userAgent, from, forwardedFor)
				}
				w.Header().Set("Allow", allowedMethods)
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if youtubehandlers.HandleAlertNotification(w, r, notificationOpts) {
				return
			}
			http.Error(w, "failed to process notification", http.StatusInternalServerError)
		default:
			w.Header().Set("Allow", allowedMethods)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func respondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func alertPlatform(userAgent, from string) string {
	if strings.HasPrefix(userAgent, "FeedFetcher-Google") && from == "googlebot(at)googlebot.com" {
		return "youtube"
	}
	return ""
}

type adminSettingsResponse struct {
	ListenAddr      string `json:"listenAddr"`
	AdminEmail      string `json:"adminEmail"`
	StreamersFile   string `json:"streamersFile"`
	SubmissionsFile string `json:"submissionsFile"`
}

func writeHTTPLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, snapshot := logging.Subscribe()
	defer logging.Unsubscribe(ch)

	writeEvent := func(entry []byte) {
		// SSE requires each event to be prefixed with "data:" and end with a blank line.
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(entry)
		_, _ = w.Write([]byte("\n\n"))
	}

	for _, entry := range snapshot {
		writeEvent(entry)
	}
	flusher.Flush()

	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-ch:
			if !ok {
				return
			}
			writeEvent(entry)
			flusher.Flush()
		}
	}
}
