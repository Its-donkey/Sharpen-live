//go:build js && wasm

package admin

import (
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

var adminStatusClear chan struct{}

const statusAutoClearDuration = 4 * time.Second

func setAdminStatus(status model.AdminStatus) {
	setAdminStatusDuration(status, 0)
}

func setTransientStatus(status model.AdminStatus) {
	setAdminStatusDuration(status, statusAutoClearDuration)
}

func setAdminStatusDuration(status model.AdminStatus, duration time.Duration) {
	cancelAdminStatusTimer()
	adminState.Status = status
	scheduleAdminRender()
	if duration <= 0 || strings.TrimSpace(status.Message) == "" {
		return
	}
	cancel := make(chan struct{})
	adminStatusClear = cancel
	go func(expected model.AdminStatus, done chan struct{}) {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		select {
		case <-timer.C:
			if adminState.Status == expected {
				adminState.Status = model.AdminStatus{}
				scheduleAdminRender()
			}
		case <-done:
		}
	}(status, cancel)
}

func cancelAdminStatusTimer() {
	if adminStatusClear != nil {
		close(adminStatusClear)
		adminStatusClear = nil
	}
}
