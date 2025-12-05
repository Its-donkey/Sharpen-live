package server

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func (s *server) handleAdminStatusCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	token := s.adminTokenFromRequest(r)
	if token == "" {
		s.redirectAdmin(w, r, "", "Log in to refresh channel status.")
		return
	}
	// Check if YouTube is enabled for this site
	if !s.isYouTubeEnabled() {
		s.logger.Info("admin", "YouTube disabled, skipping status check", map[string]any{
			"siteKey": s.siteKey,
		})
		s.redirectAdmin(w, r, "", "YouTube is disabled for this site.")
		return
	}
	if s.statusChecker == nil {
		s.redirectAdmin(w, r, "", "Status checks unavailable.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	result, err := s.statusChecker.CheckAll(ctx)
	if err != nil {
		s.redirectAdmin(w, r, "", err.Error())
		return
	}

	msg := fmt.Sprintf("Checked %d channel(s): online %d, offline %d, updated %d, failed %d.",
		result.Checked, result.Online, result.Offline, result.Updated, result.Failed)

	// If there are failures, provide detailed error information
	var errMsg string
	if result.Failed > 0 && len(result.FailureList) > 0 {
		errMsg = "Status check failures:\n"
		// Show up to 10 failures in the UI
		displayCount := len(result.FailureList)
		if displayCount > 10 {
			displayCount = 10
		}
		for i := 0; i < displayCount; i++ {
			failure := result.FailureList[i]
			errMsg += fmt.Sprintf("• %s (channel: %s): %s\n",
				failure.StreamerName, failure.ChannelID, failure.Error)
		}
		if len(result.FailureList) > 10 {
			remaining := len(result.FailureList) - 10
			errMsg += fmt.Sprintf("... and %d more failure(s). Check server logs for complete details.", remaining)
		}
	}

	s.redirectAdmin(w, r, msg, errMsg)
}
