package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	adminauth "github.com/Its-donkey/Sharpen-live/internal/alert/admin/auth"
	adminservice "github.com/Its-donkey/Sharpen-live/internal/alert/admin/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/onboarding"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	streamersvc "github.com/Its-donkey/Sharpen-live/internal/alert/streamers/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/submissions"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

const adminCookieName = "sharpen_admin_token"
const adminLogLimit = 120

type adminPageData struct {
	basePageData
	LoggedIn         bool
	Flash            string
	Error            string
	Submissions      []adminSubmission
	SubmissionsError string
	Streamers        []model.Streamer
	RosterError      string
	AdminEmail       string
	Logs             []logCategoryView
	LogLimit         int
	LogsError        string
}

type adminSubmission struct {
	ID          string
	Alias       string
	Description string
	Languages   []string
	PlatformURL string
	SubmittedAt string
}

type logCategoryView struct {
	Title   string
	Entries []logEntryView
	Error   string
}

type logEntryView struct {
	Timestamp string
	Message   string
	Meta      string
	Category  string
	RequestID string
}

func (s *server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	msg := strings.TrimSpace(r.URL.Query().Get("msg"))
	errMsg := strings.TrimSpace(r.URL.Query().Get("err"))

	base := s.buildBasePageData(r, fmt.Sprintf("Admin · %s", s.siteName), "Sharpen.Live admin dashboard for roster moderation and submissions.", "/admin")
	base.SecondaryAction = &navAction{
		Label: "Back to site",
		Href:  "/",
	}
	base.Robots = "noindex, nofollow"

	data := adminPageData{
		basePageData: base,
		Flash:        msg,
		Error:        errMsg,
		AdminEmail:   s.adminEmail,
	}

	token := s.adminTokenFromRequest(r)
	if token == "" {
		s.renderAdminPage(w, data)
		return
	}
	data.LoggedIn = true

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	if s.adminSubmissions != nil {
		subs, subErr := s.adminSubmissions.List(ctx)
		if subErr != nil {
			data.SubmissionsError = subErr.Error()
		} else {
			data.Submissions = mapAdminSubmissions(subs)
		}
	}

	if s.streamersStore != nil {
		records, err := s.streamersStore.List()
		if err != nil {
			data.RosterError = err.Error()
		} else {
			data.Streamers = mapStreamerRecords(records)
		}
	}

	logs, logErr := s.loadAdminLogs(adminLogLimit)
	if logErr != nil {
		data.LogsError = logErr.Error()
	}
	data.Logs = logs
	data.LogLimit = adminLogLimit

	s.renderAdminPage(w, data)
}

func (s *server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.redirectAdmin(w, r, "", "Invalid login form.")
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	password := strings.TrimSpace(r.FormValue("password"))
	if email == "" || password == "" {
		s.redirectAdmin(w, r, "", "Email and password are required.")
		return
	}

	if s.adminManager == nil {
		s.redirectAdmin(w, r, "", "Admin login is not configured.")
		return
	}

	token, err := s.adminManager.Login(email, password)
	if err != nil {
		s.redirectAdmin(w, r, "", "Invalid credentials.")
		return
	}
	s.setAdminSession(w, r, token)
	s.redirectAdmin(w, r, "Logged in successfully.", "")
}

func (s *server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	s.clearAdminSession(w)
	s.redirectAdmin(w, r, "Logged out.", "")
}

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

func (s *server) handleAdminStreamerUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.redirectAdmin(w, r, "", "Invalid update request.")
		return
	}
	token := s.adminTokenFromRequest(r)
	if token == "" {
		s.redirectAdmin(w, r, "", "Log in to edit streamers.")
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	alias := strings.TrimSpace(r.FormValue("alias"))
	description := strings.TrimSpace(r.FormValue("description"))
	languages := parseLanguagesInput(r.FormValue("languages"))
	platformURL := strings.TrimSpace(r.FormValue("platform_url"))

	if id == "" || alias == "" || description == "" {
		s.redirectAdmin(w, r, "", "Name and description are required.")
		return
	}

	if s.streamerService == nil {
		s.redirectAdmin(w, r, "", "Streamer service unavailable.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	_, err := s.streamerService.Update(ctx, streamersvc.UpdateRequest{
		ID:          id,
		Alias:       &alias,
		Description: &description,
		Languages:   &languages,
	})
	if err != nil {
		s.redirectAdmin(w, r, "", adminStreamersErrorMessage(err))
		return
	}

	if platformURL != "" {
		baseStore, ok := s.streamersStore.(*streamers.Store)
		if !ok {
			s.redirectAdmin(w, r, "", "Platform updates are unavailable.")
			return
		}
		record, err := baseStore.Get(id)
		if err != nil {
			s.redirectAdmin(w, r, "", adminStreamersErrorMessage(err))
			return
		}
		currentPlatformURL := ""
		if record.Platforms.YouTube != nil {
			currentPlatformURL = youtubeChannelURL(record.Platforms.YouTube)
		}
		if strings.EqualFold(strings.TrimSpace(currentPlatformURL), platformURL) {
			s.redirectAdmin(w, r, "Streamer updated.", "")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		onboardErr := onboarding.FromURL(ctx, record, platformURL, onboarding.Options{
			Client:       &http.Client{Timeout: 10 * time.Second},
			HubURL:       s.youtubeConfig.HubURL,
			CallbackURL:  s.youtubeConfig.CallbackURL,
			VerifyMode:   s.youtubeConfig.Verify,
			LeaseSeconds: s.youtubeConfig.LeaseSeconds,
			Logger:       s.logger,
			Store:        baseStore,
		})
		if onboardErr != nil {
			s.redirectAdmin(w, r, "", fmt.Sprintf("Failed to update platform: %v", onboardErr))
			return
		}
	}

	s.redirectAdmin(w, r, "Streamer updated.", "")
}

func (s *server) handleAdminStreamerDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.redirectAdmin(w, r, "", "Invalid delete request.")
		return
	}
	token := s.adminTokenFromRequest(r)
	if token == "" {
		s.redirectAdmin(w, r, "", "Log in to delete streamers.")
		return
	}
	id := strings.TrimSpace(r.FormValue("id"))
	if id == "" {
		s.redirectAdmin(w, r, "", "Missing streamer id.")
		return
	}

	if s.streamerService == nil {
		s.redirectAdmin(w, r, "", "Streamer service unavailable.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	err := s.streamerService.Delete(ctx, streamersvc.DeleteRequest{ID: id})
	if err != nil {
		s.redirectAdmin(w, r, "", adminStreamersErrorMessage(err))
		return
	}
	s.redirectAdmin(w, r, "Streamer removed.", "")
}

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

func (s *server) renderAdminPage(w http.ResponseWriter, data adminPageData) {
	tmpl, ok := s.templates["admin"]
	if !ok {
		http.Error(w, "admin template missing", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "admin", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *server) redirectAdmin(w http.ResponseWriter, r *http.Request, msg, errMsg string) {
	values := make(urlValues)
	values.setIf("msg", msg)
	values.setIf("err", errMsg)
	target := "/admin"
	if encoded := values.encode(); encoded != "" {
		target += "?" + encoded
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

type urlValues map[string]string

func (v urlValues) setIf(key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	v[key] = value
}

func (v urlValues) encode() string {
	if len(v) == 0 {
		return ""
	}
	var parts []string
	for k, val := range v {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(val))
	}
	return strings.Join(parts, "&")
}

func (s *server) adminTokenFromRequest(r *http.Request) string {
	if r == nil || s.adminManager == nil {
		return ""
	}
	cookie, err := r.Cookie(adminCookieName)
	if err != nil {
		return ""
	}
	token := strings.TrimSpace(cookie.Value)
	if token == "" {
		return ""
	}
	if !s.adminManager.Validate(token) {
		return ""
	}
	return token
}

func (s *server) setAdminSession(w http.ResponseWriter, r *http.Request, token adminauth.Token) {
	secure := r != nil && (r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"))
	cookie := &http.Cookie{
		Name:     adminCookieName,
		Value:    token.Value,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Path:     "/",
	}
	if !token.ExpiresAt.IsZero() {
		cookie.Expires = token.ExpiresAt
	}
	http.SetCookie(w, cookie)
}

func (s *server) clearAdminSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

func parseLanguagesInput(raw string) []string {
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		values = append(values, trimmed)
		if len(values) >= model.MaxLanguages {
			break
		}
	}
	return values
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

func adminSubmissionsErrorMessage(err error) string {
	switch {
	case errors.Is(err, adminservice.ErrInvalidAction):
		return "Choose approve or reject for a submission."
	case errors.Is(err, adminservice.ErrMissingIdentifier):
		return "Submission id is required."
	case errors.Is(err, submissions.ErrNotFound):
		return "Submission not found or already processed."
	default:
		return "Failed to process submission."
	}
}

func adminStreamersErrorMessage(err error) string {
	switch {
	case errors.Is(err, streamers.ErrStreamerNotFound):
		return "Streamer not found."
	case errors.Is(err, streamers.ErrDuplicateAlias):
		return "A streamer with that alias already exists."
	case errors.Is(err, streamersvc.ErrValidation):
		return "Invalid streamer details."
	case errors.Is(err, streamersvc.ErrSubscription):
		return "Failed to update channel subscription."
	default:
		return "Failed to update streamer."
	}
}

func pastTense(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "approve":
		return "approved"
	case "reject":
		return "rejected"
	default:
		return action + "ed"
	}
}

func (s *server) loadAdminLogs(limit int) ([]logCategoryView, error) {
	if limit <= 0 {
		limit = adminLogLimit
	}
	if strings.TrimSpace(s.logDir) == "" {
		return nil, errors.New("log directory not configured")
	}
	categories := []struct {
		File  string
		Title string
	}{
		{File: "general.json", Title: "General"},
		{File: "http.json", Title: "HTTP"},
		{File: "websub.json", Title: "WebSub"},
	}

	views := make([]logCategoryView, 0, len(categories))
	for _, cat := range categories {
		view := logCategoryView{Title: cat.Title}
		entries, err := s.readLogFile(filepath.Join(s.logDir, cat.File), limit)
		if err != nil {
			view.Error = err.Error()
		} else {
			view.Entries = entries
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *server) readLogFile(path string, limit int) ([]logEntryView, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	var payload struct {
		LogEvents []json.RawMessage `json:"logevents"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	if len(payload.LogEvents) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = adminLogLimit
	}
	entries := make([]logEntryView, 0, min(limit, len(payload.LogEvents)))
	for i := len(payload.LogEvents) - 1; i >= 0 && len(entries) < limit; i-- {
		entries = append(entries, mapLogEntry(payload.LogEvents[i]))
	}
	return entries, nil
}

func mapLogEntry(raw json.RawMessage) logEntryView {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return logEntryView{
			Timestamp: "(unknown time)",
			Message:   "Unparseable log entry",
			Meta:      err.Error(),
		}
	}
	entry := logEntryView{
		Category:  stringVal(payload, "category"),
		RequestID: stringVal(payload, "id"),
		Message:   stringVal(payload, "message"),
		Timestamp: formatLogTime(stringVal(payload, "time")),
	}
	method := stringVal(payload, "method")
	path := stringVal(payload, "path")
	direction := stringVal(payload, "direction")
	status := intVal(payload, "status")
	remote := stringVal(payload, "remote")
	duration := intVal(payload, "durationMs")

	if method != "" || path != "" {
		var parts []string
		if direction != "" {
			parts = append(parts, titleCase(direction))
		}
		methodPath := strings.TrimSpace(strings.TrimSpace(method + " " + path))
		if methodPath != "" {
			parts = append(parts, methodPath)
		}
		if status > 0 {
			parts = append(parts, fmt.Sprintf("(%d)", status))
		}
		if len(parts) > 0 {
			entry.Message = strings.Join(parts, " ")
		}
	}

	var meta []string
	if duration > 0 {
		meta = append(meta, fmt.Sprintf("%dms", duration))
	}
	if remote != "" {
		meta = append(meta, "from "+remote)
	}
	if entry.RequestID != "" {
		meta = append(meta, "id "+entry.RequestID)
	}
	entry.Meta = strings.Join(meta, " • ")

	if entry.Message == "" {
		entry.Message = "(no message)"
	}
	if entry.Timestamp == "" {
		entry.Timestamp = "(unknown time)"
	}
	return entry
}

func stringVal(values map[string]any, key string) string {
	if raw, ok := values[key]; ok {
		switch v := raw.(type) {
		case string:
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func intVal(values map[string]any, key string) int {
	if raw, ok := values[key]; ok {
		switch v := raw.(type) {
		case float64:
			return int(v)
		case int64:
			return int(v)
		case int:
			return v
		}
	}
	return 0
}

func formatLogTime(raw string) string {
	if raw == "" {
		return ""
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts.Local().Format("2006-01-02 15:04:05")
	}
	return strings.TrimSpace(raw)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func titleCase(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	return strings.ToUpper(lower[:1]) + lower[1:]
}
