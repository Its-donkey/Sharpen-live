package handlers

import (
	"context"
	"errors"
	youtubeservice "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	"io"
	"net/http"
	"time"
)

type alertProcessor interface {
	Process(ctx context.Context, req youtubeservice.AlertProcessRequest) (youtubeservice.AlertProcessResult, error)
}

// AlertNotificationOptions configure POST /alerts handling.
type AlertNotificationOptions struct {
	StreamersStore *streamers.Store
	VideoLookup    youtubeservice.LiveVideoLookup
	Processor      alertProcessor
}

// HandleAlertNotification processes YouTube hub POST notifications.
func HandleAlertNotification(w http.ResponseWriter, r *http.Request, opts AlertNotificationOptions) bool {
	if r.Method != http.MethodPost {
		return false
	}
	if !IsAlertPath(r.URL.Path) {
		return false
	}

	proc := opts.Processor
	if proc == nil {
		if opts.VideoLookup == nil || opts.StreamersStore == nil {
			return false
		}
		proc = &youtubeservice.AlertProcessor{
			Streamers:   opts.StreamersStore,
			VideoLookup: opts.VideoLookup,
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result, err := proc.Process(ctx, youtubeservice.AlertProcessRequest{
		Feed:       io.LimitReader(r.Body, 1<<20),
		RemoteAddr: r.RemoteAddr,
	})
	if err != nil {
		handleAlertError(r.Context(), w, err, result)
		return true
	}

	w.WriteHeader(http.StatusNoContent)
	return true
}

func handleAlertError(ctx context.Context, w http.ResponseWriter, err error, result youtubeservice.AlertProcessResult) {
	switch {
	case errors.Is(err, youtubeservice.ErrInvalidFeed):
		http.Error(w, "invalid atom feed", http.StatusBadRequest)
	case errors.Is(err, youtubeservice.ErrLookupFailed):

		w.WriteHeader(http.StatusAccepted)
	default:
		http.Error(w, "failed to process notification", http.StatusInternalServerError)
	}
}
