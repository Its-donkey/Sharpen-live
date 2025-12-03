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
	fmt.Printf("\n###################################\n")
	fmt.Printf("### ADMIN SUBMISSION HANDLER ###\n")
	fmt.Printf("###################################\n")
	fmt.Printf("Method: %s\n", r.Method)
	fmt.Printf("Path: %s\n", r.URL.Path)
	fmt.Printf("Remote Addr: %s\n", r.RemoteAddr)

	if r.Method != http.MethodPost {
		fmt.Printf("WARNING: Non-POST request, redirecting to /admin\n")
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		fmt.Printf("ERROR: Failed to parse form: %v\n", err)
		s.redirectAdmin(w, r, "", "Invalid submission request.")
		return
	}
	token := s.adminTokenFromRequest(r)
	if token == "" {
		fmt.Printf("WARNING: No admin token found, authentication required\n")
		s.redirectAdmin(w, r, "", "Log in to moderate submissions.")
		return
	}
	fmt.Printf("INFO: Admin token validated\n")

	id := strings.TrimSpace(r.FormValue("id"))
	action := strings.ToLower(strings.TrimSpace(r.FormValue("action")))
	fmt.Printf("INFO: Parsed form values:\n")
	fmt.Printf("  Submission ID: %s\n", id)
	fmt.Printf("  Action: %s\n", action)

	if id == "" || (action != "approve" && action != "reject") {
		fmt.Printf("ERROR: Invalid form values - id or action missing/invalid\n")
		s.redirectAdmin(w, r, "", "Choose approve or reject for a submission.")
		return
	}
	if s.adminSubmissions == nil {
		fmt.Printf("ERROR: Admin submissions service is nil\n")
		s.redirectAdmin(w, r, "", "Submissions service unavailable.")
		return
	}
	fmt.Printf("\nINFO: Calling adminSubmissions.Process...\n")
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	_, err := s.adminSubmissions.Process(ctx, adminservice.ActionRequest{
		Action: adminservice.Action(action),
		ID:     id,
	})
	if err != nil {
		fmt.Printf("\nERROR: adminSubmissions.Process failed: %v\n", err)
		s.logger.Warn("admin", "submission moderation failed", map[string]any{
			"submission_id": id,
			"action":        action,
			"error":         err.Error(),
		})
		fmt.Printf("### ADMIN SUBMISSION HANDLER END (failed) ###\n\n")
		s.redirectAdmin(w, r, "", adminSubmissionsErrorMessage(err))
		return
	}
	fmt.Printf("\nSUCCESS: adminSubmissions.Process completed successfully\n")
	s.logger.Info("admin", "submission moderated", map[string]any{
		"submission_id": id,
		"action":        action,
	})
	fmt.Printf("###################################\n")
	fmt.Printf("### ADMIN SUBMISSION HANDLER END (success) ###\n")
	fmt.Printf("###################################\n\n")
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
