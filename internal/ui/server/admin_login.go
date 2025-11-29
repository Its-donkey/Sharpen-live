package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	adminauth "github.com/Its-donkey/Sharpen-live/internal/alert/admin/auth"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

const adminCookieName = "sharpen_admin_token"
const adminLogLimit = 120
const adminBodyPreviewLimit = 600
const adminHeadersPreviewLimit = 600

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
	Details   string
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
