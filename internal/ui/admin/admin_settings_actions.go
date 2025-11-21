//go:build js && wasm

package admin

import (
	"context"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

func handleSettingsFieldChange(field, value string) {
	if adminState.SettingsDraft == nil {
		return
	}
	switch field {
	case "adminEmail":
		adminState.SettingsDraft.AdminEmail = value
	case "adminPassword":
		adminState.SettingsDraft.AdminPassword = value
	case "adminToken":
		adminState.SettingsDraft.AdminToken = value
	case "youtubeApiKey":
		adminState.SettingsDraft.YouTubeAPIKey = value
	case "youtubeAlertsCallback":
		adminState.SettingsDraft.YouTubeAlertsCallback = value
	case "youtubeAlertsSecret":
		adminState.SettingsDraft.YouTubeAlertsSecret = value
	case "youtubeAlertsVerifyPrefix":
		adminState.SettingsDraft.YouTubeAlertsVerifyPref = value
	case "youtubeAlertsVerifySuffix":
		adminState.SettingsDraft.YouTubeAlertsVerifySuff = value
	case "youtubeAlertsHubUrl":
		adminState.SettingsDraft.YouTubeAlertsHubURL = value
	case "listenAddr":
		adminState.SettingsDraft.ListenAddr = value
	case "dataDir":
		adminState.SettingsDraft.DataDir = value
	case "staticDir":
		adminState.SettingsDraft.StaticDir = value
	case "streamersFile":
		adminState.SettingsDraft.StreamersFile = value
	case "submissionsFile":
		adminState.SettingsDraft.SubmissionsFile = value
	}
}

func handleSettingsSubmit() {
	if adminState.Settings == nil || adminState.SettingsDraft == nil {
		return
	}
	updates := model.AdminSettingsUpdate{}
	original := adminState.Settings
	draft := adminState.SettingsDraft

	if draft.AdminEmail != original.AdminEmail {
		updates["adminEmail"] = draft.AdminEmail
	}
	if draft.AdminPassword != original.AdminPassword {
		updates["adminPassword"] = draft.AdminPassword
	}
	if draft.AdminToken != original.AdminToken {
		updates["adminToken"] = draft.AdminToken
	}
	if draft.YouTubeAPIKey != original.YouTubeAPIKey {
		updates["youtubeApiKey"] = draft.YouTubeAPIKey
	}
	if draft.YouTubeAlertsCallback != original.YouTubeAlertsCallback {
		updates["youtubeAlertsCallback"] = draft.YouTubeAlertsCallback
	}
	if draft.YouTubeAlertsSecret != original.YouTubeAlertsSecret {
		updates["youtubeAlertsSecret"] = draft.YouTubeAlertsSecret
	}
	if draft.YouTubeAlertsVerifyPref != original.YouTubeAlertsVerifyPref {
		updates["youtubeAlertsVerifyPrefix"] = draft.YouTubeAlertsVerifyPref
	}
	if draft.YouTubeAlertsVerifySuff != original.YouTubeAlertsVerifySuff {
		updates["youtubeAlertsVerifySuffix"] = draft.YouTubeAlertsVerifySuff
	}
	if draft.YouTubeAlertsHubURL != original.YouTubeAlertsHubURL {
		updates["youtubeAlertsHubUrl"] = draft.YouTubeAlertsHubURL
	}
	if draft.ListenAddr != original.ListenAddr {
		updates["listenAddr"] = draft.ListenAddr
	}
	if draft.DataDir != original.DataDir {
		updates["dataDir"] = draft.DataDir
	}
	if draft.StaticDir != original.StaticDir {
		updates["staticDir"] = draft.StaticDir
	}
	if draft.StreamersFile != original.StreamersFile {
		updates["streamersFile"] = draft.StreamersFile
	}
	if draft.SubmissionsFile != original.SubmissionsFile {
		updates["submissionsFile"] = draft.SubmissionsFile
	}

	if len(updates) == 0 {
		setTransientStatus(model.AdminStatus{Tone: "info", Message: "No changes to update."})
		return
	}

	setAdminStatus(model.AdminStatus{Tone: "info", Message: "Saving settings…"})
	adminState.SettingsSaving = true
	scheduleAdminRender()

	go func(changes model.AdminSettingsUpdate) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := adminUpdateSettings(ctx, changes); err != nil {
			adminState.SettingsSaving = false
			setAdminStatus(model.AdminStatus{Tone: "error", Message: err.Error()})
			return
		}
		adminState.SettingsSaving = false
		setTransientStatus(model.AdminStatus{Tone: "success", Message: "Settings updated."})
		refreshAdminData()
	}(updates)
}
