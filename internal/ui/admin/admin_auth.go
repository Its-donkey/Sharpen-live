//go:build js && wasm

package admin

import (
	"context"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"github.com/Its-donkey/Sharpen-live/internal/ui/state"
)

func performAdminLogin() {
	email := strings.TrimSpace(adminState.LoginEmail)
	password := adminState.LoginPassword
	if email == "" || password == "" {
		setAdminStatus(model.AdminStatus{Tone: "error", Message: "Email and password are required."})
		return
	}
	adminState.Loading = true
	setAdminStatus(model.AdminStatus{Tone: "info", Message: "Logging in…"})

	go func(email, password string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		token, err := adminLogin(ctx, email, password)
		if err != nil {
			adminState.MonitorLoading = false
			adminState.SettingsLoading = false
			adminState.Loading = false
			setAdminStatus(model.AdminStatus{Tone: "error", Message: err.Error()})
			return
		}
		adminState.Token = token
		adminState.Loading = false
		setTransientStatus(model.AdminStatus{Tone: "success", Message: "Login successful."})
		persistAdminToken(token)
		scheduleAdminRender()
		refreshAdminData()
	}(email, password)
}

func refreshAdminData() {
	if strings.TrimSpace(adminState.Token) == "" {
		setAdminStatus(model.AdminStatus{Tone: "error", Message: "Log in to load admin data."})
		return
	}
	adminState.Loading = true
	adminState.MonitorLoading = true
	adminState.SettingsLoading = true
	setAdminStatus(model.AdminStatus{Tone: "info", Message: "Updating admin data…"})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		statusTone := "success"
		var statusMessages []string

		submissions, err := adminFetchSubmissions(ctx)
		if err != nil {
			statusTone = escalateAdminStatusTone(statusTone, "warning")
			statusMessages = append(statusMessages, "Submissions unavailable: "+err.Error())
		} else {
			adminState.Submissions = submissions
		}

		streamers, usedFallbackRoster, err := adminFetchStreamers(ctx)
		cachedRoster := state.LoadRosterSnapshot()
		fallbackMessage := ""
		if (err != nil || len(streamers) == 0) && len(cachedRoster) > 0 {
			streamers = cachedRoster
			usedFallbackRoster = true
			if err != nil {
				fallbackMessage = "Streamers endpoint unavailable—showing cached public roster."
			} else {
				fallbackMessage = "Streamers endpoint returned no entries—showing cached public roster."
			}
			err = nil
		}
		if err != nil {
			statusTone = escalateAdminStatusTone(statusTone, "error")
			statusMessages = append(statusMessages, "Streamers unavailable: "+err.Error())
			adminState.Streamers = nil
			adminState.StreamerForms = make(map[string]*model.AdminStreamerForm)
		} else {
			adminState.Streamers = streamers
			adminState.StreamerForms = make(map[string]*model.AdminStreamerForm, len(streamers))
			for _, s := range streamers {
				adminState.StreamerForms[s.ID] = newStreamerFormFromStreamer(s)
			}
			if fallbackMessage != "" {
				statusTone = escalateAdminStatusTone(statusTone, "warning")
				statusMessages = append(statusMessages, fallbackMessage)
			} else if usedFallbackRoster {
				statusTone = escalateAdminStatusTone(statusTone, "warning")
				statusMessages = append(statusMessages, "Streamers endpoint unavailable—showing fallback roster data.")
			}
		}

		settings, err := adminFetchSettings(ctx)
		if err != nil {
			statusTone = escalateAdminStatusTone(statusTone, "warning")
			statusMessages = append(statusMessages, "Settings unavailable: "+err.Error())
			adminState.Settings = nil
			adminState.SettingsDraft = nil
		} else if settings != nil {
			adminState.Settings = settings
			copyDraft := *settings
			adminState.SettingsDraft = &copyDraft
		} else {
			adminState.Settings = nil
			adminState.SettingsDraft = nil
		}
		adminState.SettingsLoading = false

		monitor, leases, err := adminFetchMonitor(ctx)
		if err != nil {
			statusTone = escalateAdminStatusTone(statusTone, "warning")
			statusMessages = append(statusMessages, "Monitor events unavailable: "+err.Error())
		} else {
			adminState.MonitorEvents = monitor
			adminState.YouTubeLeases = leases
		}
		adminState.MonitorLoading = false

		adminState.Loading = false
		if len(statusMessages) == 0 {
			setTransientStatus(model.AdminStatus{Tone: "success", Message: "Admin data updated."})
		} else {
			setAdminStatus(model.AdminStatus{Tone: statusTone, Message: strings.Join(statusMessages, " ")})
		}
		scheduleAdminRender()
		go handleRosterStatusCheck()
	}()
}

func escalateAdminStatusTone(current, candidate string) string {
	if candidate == "error" || current == "error" {
		return "error"
	}
	if candidate == "warning" && current != "error" {
		return "warning"
	}
	return current
}

// HandleAdminLogout clears the admin session and removes any cached data.
func HandleAdminLogout() {
	performAdminLogout(model.AdminStatus{Tone: "info", Message: "Logged out of admin console."}, true)
}

func handleAdminUnauthorizedResponse() {
	status := model.AdminStatus{Tone: "error", Message: "Session expired. Log in again."}
	performAdminLogout(status, false)
}

func performAdminLogout(status model.AdminStatus, transient bool) {
	adminState.Token = ""
	adminState.Loading = false
	adminState.StatusCheckRunning = false
	adminState.Submissions = nil
	adminState.Streamers = nil
	adminState.StreamerForms = make(map[string]*model.AdminStreamerForm)
	adminState.Settings = nil
	adminState.SettingsDraft = nil
	adminState.SettingsLoading = false
	adminState.SettingsSaving = false
	adminState.MonitorEvents = nil
	adminState.MonitorLoading = false
	adminState.YouTubeLeases = make(map[string]model.YouTubeLeaseStatus)
	adminState.ActivityLogs = nil
	adminState.ActivityLogsError = ""
	closeWebsiteLogStream()
	if transient {
		setTransientStatus(status)
	} else {
		setAdminStatus(status)
	}
	persistAdminToken("")
	scheduleAdminRender()
}

func handleAdminTabChange(tab string) {
	tab = strings.TrimSpace(tab)
	if tab == "" {
		tab = "streamers"
	}
	if adminState.ActiveTab == tab {
		return
	}
	adminState.ActiveTab = tab
	scheduleAdminRender()
	if tab == "activity" && adminState.ActivityTab == "website" {
		ensureWebsiteLogStream()
	}
}

func handleActivityTabChange(tab string) {
	tab = strings.TrimSpace(tab)
	if tab == "" {
		tab = "website"
	}
	if adminState.ActivityTab == tab {
		return
	}
	adminState.ActivityTab = tab
	scheduleAdminRender()
	if tab == "website" && adminState.ActiveTab == "activity" {
		ensureWebsiteLogStream()
	}
}
