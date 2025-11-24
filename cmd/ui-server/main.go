package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/forms"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"github.com/Its-donkey/Sharpen-live/internal/ui/streamers"
)

type server struct {
	apiBase        string
	assetsDir      string
	stylesPath     string
	client         *http.Client
	templates      map[string]*template.Template
	currentYear    int
	submitEndpoint string
}

type navAction struct {
	Label string
	Href  string
}

type basePageData struct {
	PageTitle       string
	StylesheetPath  string
	SubmitLink      string
	SecondaryAction *navAction
	CurrentYear     int
}

type homePageData struct {
	basePageData
	Streamers   []model.Streamer
	RosterError string
	Submit      submitFormView
}

type submitFormView struct {
	State           model.SubmitFormState
	LanguageOptions []model.LanguageOption
	FormAction      string
	MaxPlatforms    int
}

type streamerPageData struct {
	basePageData
	Streamer model.Streamer
}

func main() {
	listen := flag.String("listen", "127.0.0.1:4173", "address to serve the Sharpen.Live UI")
	apiBase := flag.String("api", "http://127.0.0.1:8880", "base URL for the alert server API")
	templatesDir := flag.String("templates", "ui/templates", "path to the html/template files")
	assetsDir := flag.String("assets", "ui", "path where styles.css is located")
	flag.Parse()

	apiURL, err := url.Parse(strings.TrimSpace(*apiBase))
	if err != nil || apiURL.Scheme == "" {
		log.Fatalf("invalid api url %q: %v", *apiBase, err)
	}
	apiURL.Path = strings.TrimSuffix(apiURL.Path, "/")

	templateRoot, err := filepath.Abs(*templatesDir)
	if err != nil {
		log.Fatalf("resolve templates dir: %v", err)
	}

	tmpl, err := loadTemplates(templateRoot)
	if err != nil {
		log.Fatalf("load templates: %v", err)
	}

	assetsPath, err := filepath.Abs(*assetsDir)
	if err != nil {
		log.Fatalf("resolve assets dir: %v", err)
	}

	srv := &server{
		apiBase:        strings.TrimSuffix(strings.TrimSpace(*apiBase), "/"),
		assetsDir:      assetsPath,
		stylesPath:     "/styles.css",
		client:         &http.Client{Timeout: 12 * time.Second},
		templates:      tmpl,
		currentYear:    time.Now().Year(),
		submitEndpoint: "/submit",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleHome)
	mux.HandleFunc("/streamers/", srv.handleStreamer)
	mux.HandleFunc("/submit", srv.handleSubmit)
	mux.Handle("/styles.css", srv.assetHandler("styles.css", "text/css"))
	mux.Handle("/submit.js", srv.assetHandler("submit.js", "application/javascript"))
	mux.Handle("/wasm_exec.js", srv.assetHandler("wasm_exec.js", "application/javascript"))
	mux.Handle("/main.wasm", srv.assetHandler("main.wasm", "application/wasm"))
	mux.HandleFunc("/admin", srv.handleAdmin)
	mux.HandleFunc("/admin/", srv.handleAdmin)
	mux.Handle("/admin/logs", singleProxyHandler(apiURL))
	mux.Handle("/api/", apiProxyHandler(apiURL))
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	log.Printf("Serving Sharpen.Live UI on http://%s (API: %s)", *listen, srv.apiBase)
	if err := http.ListenAndServe(*listen, logRequests(mux)); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func loadTemplates(dir string) (map[string]*template.Template, error) {
	funcs := template.FuncMap{
		"join":            strings.Join,
		"contains":        forms.ContainsString,
		"displayLanguage": forms.DisplayLanguage,
		"statusClass":     statusClass,
		"statusLabel":     statusLabel,
		"lower":           strings.ToLower,
	}
	base := filepath.Join(dir, "base.tmpl")
	home := filepath.Join(dir, "home.tmpl")
	streamer := filepath.Join(dir, "streamer.tmpl")
	submit := filepath.Join(dir, "submit_form.tmpl")

	homeTmpl, err := template.New("home").Funcs(funcs).ParseFiles(base, home, submit)
	if err != nil {
		return nil, fmt.Errorf("parse home templates: %w", err)
	}
	streamerTmpl, err := template.New("streamer").Funcs(funcs).ParseFiles(base, streamer)
	if err != nil {
		return nil, fmt.Errorf("parse streamer templates: %w", err)
	}
	return map[string]*template.Template{
		"home":     homeTmpl,
		"streamer": streamerTmpl,
	}, nil
}

func (s *server) assetHandler(name, contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(s.assetsDir, name)
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		http.ServeFile(w, r, path)
	})
}

func (s *server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	streamersList, rosterErr := s.fetchRoster(r.Context())
	formState := defaultSubmitState()
	if r.Method == http.MethodGet && r.URL.Query().Get("submitted") == "1" {
		formState.ResultState = "success"
		message := strings.TrimSpace(r.URL.Query().Get("message"))
		if message == "" {
			message = "Submission received and queued for review."
		}
		formState.ResultMessage = message
	}

	s.renderHome(w, formState, streamersList, rosterErr, http.StatusOK)
}

func (s *server) handleStreamer(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/streamers/") {
		http.NotFound(w, r)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/streamers/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	streamersList, err := streamers.FetchStreamersFrom(ctx, s.apiBase)
	if err != nil && len(streamersList) == 0 {
		http.Error(w, fmt.Sprintf("failed to load streamer roster: %v", err), http.StatusBadGateway)
		return
	}

	var match *model.Streamer
	for _, s := range streamersList {
		if strings.EqualFold(strings.TrimSpace(s.ID), id) {
			match = &s
			break
		}
	}

	if match == nil {
		http.NotFound(w, r)
		return
	}

	data := streamerPageData{
		basePageData: basePageData{
			PageTitle:       fmt.Sprintf("%s · Sharpen.Live", match.Name),
			StylesheetPath:  s.stylesPath,
			SubmitLink:      "/#submit",
			SecondaryAction: &navAction{Label: "Back to roster", Href: "/"},
			CurrentYear:     s.currentYear,
		},
		Streamer: *match,
	}
	if err := s.templates["streamer"].ExecuteTemplate(w, "streamer", data); err != nil {
		log.Printf("render streamer detail: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	formState := parseSubmitForm(r)
	removeID := strings.TrimSpace(r.FormValue("remove_platform"))
	action := strings.TrimSpace(r.FormValue("action"))

	switch {
	case removeID != "":
		formState.Platforms = removePlatformRow(formState.Platforms, removeID)
		formState.Errors.Platforms = make(map[string]model.PlatformFieldError)
		s.renderHomeWithRoster(w, r, formState, http.StatusOK)
		return
	case action == "add-platform":
		if len(formState.Platforms) < model.MaxPlatforms {
			formState.Platforms = append(formState.Platforms, forms.NewPlatformRow())
		}
		formState.Errors.Platforms = make(map[string]model.PlatformFieldError)
		s.renderHomeWithRoster(w, r, formState, http.StatusOK)
		return
	}

	errors := forms.ValidateSubmitForm(&formState)
	formState.Errors = errors
	if errors.Name || errors.Description || errors.Languages || len(errors.Platforms) > 0 {
		s.renderHomeWithRoster(w, r, formState, http.StatusUnprocessableEntity)
		return
	}

	maybeEnrichMetadata(r.Context(), s.apiBase, s.client, &formState)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	message, err := submitStreamer(ctx, s.client, s.apiBase, formState)
	if err != nil {
		formState.ResultState = "error"
		formState.ResultMessage = err.Error()
		s.renderHomeWithRoster(w, r, formState, http.StatusBadGateway)
		return
	}
	if message == "" {
		message = "Submission received and queued for review."
	}
	redirectURL := "/?submitted=1&message=" + url.QueryEscape(message)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (s *server) renderHome(w http.ResponseWriter, formState model.SubmitFormState, roster []model.Streamer, rosterErr string, status int) {
	ensureSubmitDefaults(&formState)
	data := homePageData{
		basePageData: basePageData{
			PageTitle:       "Sharpen.Live",
			StylesheetPath:  s.stylesPath,
			SubmitLink:      "/#submit",
			SecondaryAction: &navAction{Label: "Roster", Href: "/"},
			CurrentYear:     s.currentYear,
		},
		Streamers:   roster,
		RosterError: rosterErr,
		Submit: submitFormView{
			State:           formState,
			LanguageOptions: model.LanguageOptions,
			FormAction:      s.submitEndpoint,
			MaxPlatforms:    model.MaxPlatforms,
		},
	}
	if status > 0 {
		w.WriteHeader(status)
	}
	if err := s.templates["home"].ExecuteTemplate(w, "home", data); err != nil {
		log.Printf("render home: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *server) renderHomeWithRoster(w http.ResponseWriter, r *http.Request, formState model.SubmitFormState, status int) {
	streamersList, rosterErr := s.fetchRoster(r.Context())
	s.renderHome(w, formState, streamersList, rosterErr, status)
}

func (s *server) fetchRoster(ctx context.Context) ([]model.Streamer, string) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	streamersList, err := streamers.FetchStreamersFrom(ctx, s.apiBase)
	if err != nil {
		return streamersList, err.Error()
	}
	return streamersList, ""
}

func defaultSubmitState() model.SubmitFormState {
	return model.SubmitFormState{
		Languages: []string{"English"},
		Platforms: []model.PlatformFormRow{forms.NewPlatformRow()},
		Errors: model.SubmitFormErrors{
			Platforms: make(map[string]model.PlatformFieldError),
		},
	}
}

func ensureSubmitDefaults(state *model.SubmitFormState) {
	if state == nil {
		return
	}
	if len(state.Platforms) == 0 {
		state.Platforms = []model.PlatformFormRow{forms.NewPlatformRow()}
	}
	if state.Errors.Platforms == nil {
		state.Errors.Platforms = make(map[string]model.PlatformFieldError)
	}
}

func parseSubmitForm(r *http.Request) model.SubmitFormState {
	ids := r.Form["platform_id"]
	urls := r.Form["platform_url"]
	langs := r.Form["languages"]
	if len(langs) == 0 {
		langs = r.Form["languages[]"]
	}
	platforms := make([]model.PlatformFormRow, 0, len(urls))
	for i, raw := range urls {
		normalized := forms.CanonicalizeChannelInput(raw)
		rowID := ""
		if i < len(ids) {
			rowID = strings.TrimSpace(ids[i])
		}
		if rowID == "" {
			rowID = fmt.Sprintf("platform-%d", time.Now().UnixNano()+int64(i))
		}
		platforms = append(platforms, model.PlatformFormRow{
			ID:         rowID,
			ChannelURL: normalized,
			Name:       forms.DerivePlatformLabel(normalized),
		})
	}
	if len(platforms) == 0 {
		platforms = []model.PlatformFormRow{forms.NewPlatformRow()}
	}

	return model.SubmitFormState{
		Name:        strings.TrimSpace(r.FormValue("name")),
		Description: strings.TrimSpace(r.FormValue("description")),
		Languages:   normalizeLanguages(langs),
		Platforms:   platforms,
		Errors: model.SubmitFormErrors{
			Platforms: make(map[string]model.PlatformFieldError),
		},
	}
}

func normalizeLanguages(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
		if len(normalized) >= model.MaxLanguages {
			break
		}
	}
	return normalized
}

func removePlatformRow(rows []model.PlatformFormRow, removeID string) []model.PlatformFormRow {
	if len(rows) <= 1 {
		return []model.PlatformFormRow{forms.NewPlatformRow()}
	}
	next := make([]model.PlatformFormRow, 0, len(rows))
	for _, row := range rows {
		if row.ID != removeID {
			next = append(next, row)
		}
	}
	if len(next) == 0 {
		return []model.PlatformFormRow{forms.NewPlatformRow()}
	}
	return next
}

func maybeEnrichMetadata(ctx context.Context, apiBase string, client *http.Client, form *model.SubmitFormState) {
	if form == nil {
		return
	}
	target := ""
	for _, p := range form.Platforms {
		if url := strings.TrimSpace(p.ChannelURL); url != "" {
			target = url
			break
		}
	}
	if target == "" {
		return
	}
	desc := strings.TrimSpace(form.Description)
	name := strings.TrimSpace(form.Name)
	if desc != "" && name != "" {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	payload, err := json.Marshal(model.MetadataRequest{URL: target})
	if err != nil {
		return
	}
	endpoint := strings.TrimSuffix(strings.TrimSpace(apiBase), "/") + "/api/youtube/metadata"
	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	var meta model.MetadataResponse
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return
	}

	if desc == "" {
		if trimmed := strings.TrimSpace(meta.Description); trimmed != "" {
			form.Description = trimmed
			desc = trimmed
		} else if title := strings.TrimSpace(meta.Title); desc == "" && title != "" {
			form.Description = title
		}
	}
	if name == "" {
		if title := strings.TrimSpace(meta.Title); title != "" {
			form.Name = title
		}
	}
}

func submitStreamer(ctx context.Context, client *http.Client, apiBase string, form model.SubmitFormState) (string, error) {
	if strings.TrimSpace(apiBase) == "" {
		return "", errors.New("API base URL is required")
	}
	payload := model.CreateStreamerRequest{
		Streamer: model.StreamerPayload{
			Alias:       strings.TrimSpace(form.Name),
			Description: forms.BuildStreamerDescription(form.Description, form.Platforms),
			Languages:   append([]string(nil), form.Languages...),
		},
	}
	if url := forms.FirstPlatformURL(form.Platforms); url != "" {
		payload.Platforms.URL = url
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	endpoint := strings.TrimSuffix(apiBase, "/") + "/api/streamers"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if trimmed := strings.TrimSpace(string(body)); trimmed != "" {
			return "", errors.New(trimmed)
		}
		return "", fmt.Errorf("submission failed: %s", resp.Status)
	}

	var created model.CreateStreamerResponse
	if err := json.Unmarshal(body, &created); err != nil {
		return "Streamer submitted successfully.", nil
	}
	alias := strings.TrimSpace(created.Streamer.Alias)
	id := strings.TrimSpace(created.Streamer.ID)
	switch {
	case alias != "" && id != "":
		return fmt.Sprintf("%s added with ID %s.", alias, id), nil
	case alias != "":
		return fmt.Sprintf("%s added to the roster.", alias), nil
	default:
		return "Streamer submitted successfully.", nil
	}
}

func statusClass(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" {
		return "offline"
	}
	return normalized
}

func statusLabel(status, label string) string {
	if strings.TrimSpace(label) != "" {
		return label
	}
	key := strings.ToLower(strings.TrimSpace(status))
	if mapped := model.StatusLabels[key]; mapped != "" {
		return mapped
	}
	if key == "" {
		return "Offline"
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

func apiProxyHandler(target *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = target.Host
		proxy.ServeHTTP(w, r)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Truncate(time.Millisecond))
	})
}

func (s *server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	adminIndex := filepath.Join(s.assetsDir, "admin", "index.html")
	http.ServeFile(w, r, adminIndex)
}

func singleProxyHandler(target *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = target.Host
		proxy.ServeHTTP(w, r)
	})
}
