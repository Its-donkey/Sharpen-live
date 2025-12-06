package adminhttp

import (
	"context"
	adminauth "github.com/Its-donkey/Sharpen-live/internal/alert/admin/auth"
	adminservice "github.com/Its-donkey/Sharpen-live/internal/alert/admin/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	"net/http"
	"time"
	// StatusHandlerOptions configures the roster status refresh handler.
)

type StatusHandlerOptions struct {
	Authorizer     authorizer
	Service        statusService
	Manager        *adminauth.Manager
	StreamersStore *streamers.Store
}

type statusService interface {
	CheckAll(ctx context.Context) (adminservice.StatusCheckResult, error)
}

type statusHandler struct {
	authorizer authorizer
	service    statusService
}

// NewStatusHandler constructs the admin roster status handler.
func NewStatusHandler(opts StatusHandlerOptions) http.Handler {
	auth := opts.Authorizer
	if auth == nil && opts.Manager != nil {
		auth = adminservice.AuthService{Manager: opts.Manager}
	}
	svc := opts.Service
	if svc == nil {
		svc = adminservice.StatusChecker{
			Streamers: opts.StreamersStore,
		}
	}
	return statusHandler{
		authorizer: auth,
		service:    svc,
	}
}

func (h statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.authorizer == nil || h.service == nil {
		http.Error(w, "admin status checks disabled", http.StatusServiceUnavailable)
		return
	}
	if err := h.authorizer.AuthorizeRequest(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := h.service.CheckAll(ctx)
	if err != nil {
		http.Error(w, "failed to refresh channel status", http.StatusInternalServerError)
		return
	}
	respondJSON(w, result)
}
