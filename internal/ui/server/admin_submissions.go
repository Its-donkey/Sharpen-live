package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	adminservice "github.com/Its-donkey/Sharpen-live/internal/alert/admin/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/submissions"
)

func (s *server) handleAdminSubmission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.redirectAdmin(w, r, "", "Invalid submission request.")
		return
	}
	token := s.adminTokenFromRequest(r)
	if token == "" {
		s.redirectAdmin(w, r, "", "Log in to moderate submissions.")
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	action := strings.ToLower(strings.TrimSpace(r.FormValue("action")))
	if id == "" || (action != "approve" && action != "reject") {
		s.redirectAdmin(w, r, "", "Choose approve or reject for a submission.")
		return
	}
	if s.adminSubmissions == nil {
		s.redirectAdmin(w, r, "", "Submissions service unavailable.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	_, err := s.adminSubmissions.Process(ctx, adminservice.ActionRequest{
		Action: adminservice.Action(action),
		ID:     id,
	})
	if err != nil {
		s.redirectAdmin(w, r, "", adminSubmissionsErrorMessage(err))
		return
	}
	s.redirectAdmin(w, r, fmt.Sprintf("Submission %s.", pastTense(action)), "")
}

func mapAdminSubmissions(subs []submissions.Submission) []adminSubmission {
	out := make([]adminSubmission, 0, len(subs))
	for _, sub := range subs {
		out = append(out, adminSubmission{
			ID:          sub.ID,
			Alias:       sub.Alias,
			Description: sub.Description,
			Languages:   append([]string(nil), sub.Languages...),
			PlatformURL: sub.PlatformURL,
			SubmittedAt: sub.SubmittedAt.Format("2006-01-02 15:04 MST"),
		})
	}
	return out
}
