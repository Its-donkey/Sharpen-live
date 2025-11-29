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
	s.redirectAdmin(w, r, msg, "")
}
