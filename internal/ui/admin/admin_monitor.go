//go:build js && wasm

package admin

import (
	"context"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

func handleMonitorRefresh() {
	if strings.TrimSpace(adminState.Token) == "" {
		setAdminStatus(model.AdminStatus{Tone: "error", Message: "Log in to view monitor data."})
		return
	}
	adminState.MonitorLoading = true
	scheduleAdminRender()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		events, leases, err := adminFetchMonitor(ctx)
		adminState.MonitorLoading = false
		if err != nil {
			setAdminStatus(model.AdminStatus{Tone: "error", Message: err.Error()})
			return
		}
		adminState.MonitorEvents = events
		adminState.YouTubeLeases = leases
		setTransientStatus(model.AdminStatus{Tone: "success", Message: "Monitor updated."})
	}()
}
