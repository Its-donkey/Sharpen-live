package youtube

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	youtubehandlers "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/handlers"
	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/liveinfo"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
)

type AlertsHandlerOptions struct {
	StreamersStore *streamers.Store
}

func NewAlertsHandler(opts AlertsHandlerOptions) http.Handler {
	notificationOpts := youtubehandlers.AlertNotificationOptions{
		StreamersStore: opts.StreamersStore,
		VideoLookup:    &liveinfo.Client{HTTPClient: &http.Client{Timeout: 10 * time.Second}},
	}
	allowedMethods := strings.Join([]string{http.MethodGet, http.MethodPost}, ", ")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !youtubehandlers.IsAlertPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}

		platform := PlatformFromRequest(r)

		switch r.Method {
		case http.MethodGet:
			if platform == "youtube" {
				if youtubehandlers.HandleSubscriptionConfirmation(w, r, youtubehandlers.SubscriptionConfirmationOptions{
					StreamersStore: notificationOpts.StreamersStore,
				}) {
					return
				}
				http.Error(w, "invalid subscription confirmation", http.StatusBadRequest)
				return
			}
			w.Header().Set("Allow", allowedMethods)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		case http.MethodPost:
			if platform != "youtube" {
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

func CallbackPaths(callbackURL string) []string {
	return dedupePaths(map[string]string{
		"default": "/alerts",
		"config":  resolveCallbackPath(callbackURL),
	})
}

func PlatformFromRequest(r *http.Request) string {
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	from := strings.ToLower(r.Header.Get("From"))
	switch {
	case strings.Contains(ua, "youtube"), strings.Contains(from, "youtube"):
		return "youtube"
	default:
		return "unknown"
	}
}

func dedupePaths(paths map[string]string) []string {
	used := make(map[string]struct{})
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if _, ok := used[p]; ok {
			continue
		}
		used[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func resolveCallbackPath(path string) string {
	if path == "" {
		return "/alerts"
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		u, err := url.Parse(path)
		if err != nil || u.Path == "" {
			return "/alerts"
		}
		return u.Path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}
