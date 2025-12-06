package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	adminauth "github.com/Its-donkey/Sharpen-live/internal/alert/admin/auth"
	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

const adminCookieName = "sharpen_admin_token"

type SiteInfo struct {
	Key      string
	Name     string
	URL      string
	AdminURL string
}

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
	OtherSites       []SiteInfo
	YouTubeSites     []YouTubeSiteConfig
	IsAlertserver    bool
}

type adminSubmission struct {
	ID          string
	Alias       string
	Description string
	Languages   []string
	PlatformURL string
	SubmittedAt string
}

func (s *server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	msg := strings.TrimSpace(r.URL.Query().Get("msg"))
	errMsg := strings.TrimSpace(r.URL.Query().Get("err"))
	siteName := s.siteDisplayName()
	desc := fmt.Sprintf("%s admin dashboard for roster moderation and submissions.", siteName)
	base := s.buildBasePageData(r, fmt.Sprintf("Admin · %s", siteName), desc, "/admin")
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
	data.IsAlertserver = s.isAlertserver()
	if data.IsAlertserver {
		data.OtherSites = s.resolveOtherSites()
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
			// Sort streamers with online ones at the top
			sort.Slice(data.Streamers, func(i, j int) bool {
				statusOrder := map[string]int{"online": 0, "busy": 1, "offline": 2}
				orderI := statusOrder[data.Streamers[i].Status]
				orderJ := statusOrder[data.Streamers[j].Status]
				if orderI != orderJ {
					return orderI < orderJ
				}
				// If same status, sort alphabetically by name
				return strings.ToLower(data.Streamers[i].Name) < strings.ToLower(data.Streamers[j].Name)
			})
		}
	}
	// Load YouTube site configurations
	youtubeConfigs, err := s.getYouTubeSiteConfigs()
	if err != nil {
		s.logger.Warn("admin", "failed to load YouTube configs", map[string]any{
			"error": err.Error(),
		})
	} else {
		s.logger.Info("admin", "loaded YouTube configs", map[string]any{
			"count":   len(youtubeConfigs),
			"siteKey": s.siteKey,
		})
		data.YouTubeSites = youtubeConfigs
	}
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
		s.logger.Warn("admin", "login failed", map[string]any{
			"email": email,
			"error": err.Error(),
		})
		s.redirectAdmin(w, r, "", "Invalid credentials.")
		return
	}
	s.logger.Info("admin", "login successful", map[string]any{
		"email": email,
	})
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

func configuredSiteKeys(cfg config.Config) []string {
	var keys []string
	for rawKey, site := range cfg.Sites {
		key := strings.TrimSpace(site.Key)
		if key == "" {
			key = strings.TrimSpace(rawKey)
		}
		if strings.EqualFold(key, config.AlertserverKey) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *server) resolveOtherSites() []SiteInfo {
	seen := make(map[string]struct{})
	add := func(val string) {
		v := strings.TrimSpace(val)
		if v == "" || strings.EqualFold(v, config.AlertserverKey) || strings.EqualFold(v, "default-site") || strings.EqualFold(v, s.siteKey) {
			return
		}
		seen[v] = struct{}{}
	}
	for _, key := range s.availableSites {
		add(key)
	}
	for _, key := range listSiblingSites(s.assetsDir) {
		add(key)
	}
	var keys []string
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var result []SiteInfo
	for _, key := range keys {
		siteInfo := s.buildSiteInfo(key)
		result = append(result, siteInfo)
	}
	return result
}

func (s *server) buildSiteInfo(key string) SiteInfo {
	info := SiteInfo{
		Key:      key,
		Name:     key,
		URL:      "",
		AdminURL: "",
	}

	cfg, err := config.Load(s.configPath)
	if err != nil {
		return info
	}

	site, ok := cfg.Sites[key]
	if !ok {
		return info
	}

	if site.Name != "" {
		info.Name = site.Name
	}

	addr := site.Server.Addr
	port := site.Server.Port
	if addr != "" && port != "" {
		info.URL = fmt.Sprintf("http://%s%s", addr, port)
		info.AdminURL = fmt.Sprintf("http://%s%s/admin", addr, port)
	}

	return info
}

func (s *server) isAlertserver() bool {
	if strings.EqualFold(s.siteKey, config.AlertserverKey) {
		return true
	}
	clean := filepath.Clean(s.assetsDir)
	if strings.Contains(clean, string(filepath.Separator)+config.AlertserverKey) {
		return true
	}
	// Also check for "default-site" which is the actual directory name for alertserver
	return strings.Contains(clean, string(filepath.Separator)+"default-site")
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

// listSiblingSites returns directories under ui/sites (based on assetsDir) excluding the default-site.
func listSiblingSites(assetsDir string) []string {
	root := filepath.Dir(filepath.Clean(assetsDir))
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var sites []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.EqualFold(name, config.AlertserverKey) || strings.EqualFold(name, "default-site") {
			continue
		}
		sites = append(sites, name)
	}
	return sites
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
