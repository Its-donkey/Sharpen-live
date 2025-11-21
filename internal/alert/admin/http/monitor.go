package adminhttp

import (
	"context"
	"errors"
	"net/http"

	adminauth "github.com/Its-donkey/Sharpen-live/internal/alert/admin/auth"
	adminservice "github.com/Its-donkey/Sharpen-live/internal/alert/admin/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
	"github.com/Its-donkey/Sharpen-live/internal/alert/logging"
	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/monitoring"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
)

// MonitorHandlerOptions configures the YouTube monitor handler.
type MonitorHandlerOptions struct {
	Authorizer     authorizer
	Service        monitorService
	Manager        *adminauth.Manager
	Logger         logging.Logger
	StreamersStore *streamers.Store
	YouTube        config.YouTubeConfig
}

type monitorService interface {
	Overview(ctx context.Context) (monitoring.Overview, error)
}

type monitorHandler struct {
	authorizer authorizer
	service    monitorService
	logger     logging.Logger
}

// NewMonitorHandler constructs the admin monitor HTTP handler.
func NewMonitorHandler(opts MonitorHandlerOptions) http.Handler {
	auth := opts.Authorizer
	if auth == nil && opts.Manager != nil {
		auth = adminservice.AuthService{Manager: opts.Manager}
	}
	svc := opts.Service
	if svc == nil {
		svc = monitoring.NewService(monitoring.ServiceOptions{
			StreamersStore:      opts.StreamersStore,
			DefaultLeaseSeconds: opts.YouTube.LeaseSeconds,
		})
	}
	return monitorHandler{
		authorizer: auth,
		service:    svc,
		logger:     opts.Logger,
	}
}

func (h monitorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.authorizer == nil || h.service == nil {
		http.Error(w, "admin monitor disabled", http.StatusServiceUnavailable)
		return
	}
	if err := h.authorizer.AuthorizeRequest(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	overview, err := h.service.Overview(r.Context())
	if err != nil {
		if h.logger != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			h.logger.Printf("monitor overview: %v", err)
		}
		http.Error(w, "failed to load monitor data", http.StatusInternalServerError)
		return
	}
	respondJSON(w, overview)
}
