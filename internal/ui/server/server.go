package server

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	adminauth "github.com/Its-donkey/Sharpen-live/internal/alert/admin/auth"
	adminservice "github.com/Its-donkey/Sharpen-live/internal/alert/admin/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
	"github.com/Its-donkey/Sharpen-live/internal/alert/logging"
	youtubeapi "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/api"
	youtubehandlers "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/handlers"
	liveinfo "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/liveinfo"
	youtubeservice "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/subscriptions"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	streamersvc "github.com/Its-donkey/Sharpen-live/internal/alert/streamers/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/submissions"
	"github.com/Its-donkey/Sharpen-live/internal/ui/forms"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	uistreamers "github.com/Its-donkey/Sharpen-live/internal/ui/streamers"
)

// Options configures the UI HTTP server.
type Options struct {
	Listen       string
	TemplatesDir string
	AssetsDir    string
	LogDir       string
	DataDir      string
	ConfigPath   string
	Site         string
	Logger       logging.Logger
	Templates    map[string]*template.Template

	StreamersStore   StreamersStore
	StreamerService  StreamerService
	SubmissionsStore *submissions.Store
	AdminSubmissions AdminSubmissions
	AdminManager     AdminManager
	MetadataFetcher  MetadataFetcher
	StatusChecker    StatusChecker
	NewLeaseMonitor  LeaseMonitorFactory
}

// StreamersStore exposes the subset of streamer store behaviour required by the UI.
type StreamersStore interface {
	List() ([]streamers.Record, error)
	Path() string
}

// StreamerService is the subset of streamer service methods required by submit/admin flows.
type StreamerService interface {
	Create(context.Context, streamersvc.CreateRequest) (streamersvc.CreateResult, error)
	Update(context.Context, streamersvc.UpdateRequest) (streamers.Record, error)
	Delete(context.Context, streamersvc.DeleteRequest) error
}

// AdminSubmissions abstracts the admin submissions service.
type AdminSubmissions interface {
	List(context.Context) ([]submissions.Submission, error)
	Process(context.Context, adminservice.ActionRequest) (adminservice.ActionResult, error)
}

// AdminManager represents the admin authentication manager.
type AdminManager interface {
	Login(email, password string) (adminauth.Token, error)
	Validate(token string) bool
}

// MetadataFetcher is implemented by YouTube metadata clients.
type MetadataFetcher interface {
	Fetch(ctx context.Context, url string) (youtubeservice.Metadata, error)
}

// StatusChecker performs streamer status checks.
type StatusChecker interface {
	CheckAll(ctx context.Context) (adminservice.StatusCheckResult, error)
}

// LeaseMonitorFactory constructs a lease monitor. Useful for tests that avoid background goroutines.
type LeaseMonitorFactory func(context.Context, subscriptions.LeaseMonitorConfig) *subscriptions.LeaseMonitor

type server struct {
	assetsDir        string
	stylesPath       string
	socialImagePath  string
	logDir           string
	templates        map[string]*template.Template
	currentYear      int
	submitEndpoint   string
	streamersStore   StreamersStore
	streamerService  StreamerService
	submissionsStore *submissions.Store
	adminSubmissions AdminSubmissions
	statusChecker    StatusChecker
	adminManager     AdminManager
	logger           logging.Logger
	adminEmail       string
	metadataFetcher  MetadataFetcher
	siteName         string
	siteDescription  string
	primaryHost      string
	youtubeConfig    config.YouTubeConfig
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
	SiteName        string
	MetaDescription string
	CanonicalURL    string
	SocialImage     string
	OGType          string
	Robots          string
	StructuredData  template.JS
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

// Run starts the UI HTTP server using the provided context and options.
func Run(ctx context.Context, opts Options) error {
	if opts.ConfigPath == "" {
		opts.ConfigPath = "config.json"
	}

	appConfig, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	siteConfig, err := config.ResolveSite(opts.Site, appConfig)
	if err != nil {
		return fmt.Errorf("resolve site config: %w", err)
	}
	opts = applyDefaults(opts, siteConfig)

	templateRoot, err := filepath.Abs(opts.TemplatesDir)
	if err != nil {
		return fmt.Errorf("resolve templates dir: %w", err)
	}

	tmpl := opts.Templates
	if tmpl == nil {
		loaded, err := loadTemplates(templateRoot)
		if err != nil {
			return fmt.Errorf("load templates: %w", err)
		}
		tmpl = loaded
	}

	assetsPath, err := filepath.Abs(opts.AssetsDir)
	if err != nil {
		return fmt.Errorf("resolve assets dir: %w", err)
	}

	logDir, err := filepath.Abs(opts.LogDir)
	if err != nil {
		return fmt.Errorf("resolve log dir: %w", err)
	}

	dataDir := opts.DataDir
	if dataDir == "" {
		dataDir = "data"
	}
	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("resolve data dir: %w", err)
	}

	logger := opts.Logger
	if logger == nil {
		logger = logging.New()
	}

	// --- HTTP category log: single-envelope JSON, one file per run ---
	httpLogFile, err := prepareLogFile(logDir, "http.json")
	if err != nil {
		return fmt.Errorf("prepare http log file: %w", err)
	}
	httpLogWriter := logging.NewCategoryLogFileWriter(httpLogFile)
	logging.SetCategoryWriter("http", httpLogWriter)
	defer httpLogWriter.Close()

	generalLogFile, err := prepareLogFile(logDir, "general.json")
	if err != nil {
		return fmt.Errorf("prepare general log file: %w", err)
	}
	generalLogWriter := logging.NewCategoryLogFileWriter(generalLogFile)
	logging.SetCategoryWriter("general", generalLogWriter)
	defer generalLogWriter.Close()

	websubLogFile, err := prepareLogFile(logDir, "websub.json")
	if err != nil {
		return fmt.Errorf("prepare websub log file: %w", err)
	}
	websubLogWriter := logging.NewCategoryLogFileWriter(websubLogFile)
	logging.SetCategoryWriter("websub", websubLogWriter)
	defer websubLogWriter.Close()

	streamersStore := opts.StreamersStore
	if streamersStore == nil {
		streamersStore = streamers.NewStore(filepath.Join(dataDir, "streamers.json"))
	}
	submissionsStore := opts.SubmissionsStore
	if submissionsStore == nil {
		submissionsStore = submissions.NewStore(filepath.Join(dataDir, "submissions.json"))
	}
	streamerSvc := opts.StreamerService
	if streamerSvc == nil {
		baseStore, ok := streamersStore.(*streamers.Store)
		if !ok {
			return fmt.Errorf("streamer service requires *streamers.Store when not injected")
		}
		streamerSvc = streamersvc.New(streamersvc.Options{
			Streamers:     baseStore,
			Submissions:   submissionsStore,
			YouTubeClient: &http.Client{Timeout: 10 * time.Second},
			YouTubeHubURL: appConfig.YouTube.HubURL,
		})
	}
	metadataSvc := opts.MetadataFetcher
	if metadataSvc == nil {
		metadataSvc = youtubeservice.MetadataService{
			Client:  &http.Client{Timeout: 5 * time.Second},
			Timeout: 5 * time.Second,
		}
	}
	adminSubSvc := opts.AdminSubmissions
	if adminSubSvc == nil {
		baseStore, ok := streamersStore.(*streamers.Store)
		if !ok {
			return fmt.Errorf("admin submissions requires *streamers.Store when not injected")
		}
		adminSubSvc = adminservice.NewSubmissionsService(adminservice.SubmissionsOptions{
			SubmissionsStore: submissionsStore,
			StreamersStore:   baseStore,
			YouTubeClient:    &http.Client{Timeout: 10 * time.Second},
			YouTube:          appConfig.YouTube,
			Logger:           logger,
		})
	}
	adminMgr := opts.AdminManager
	if adminMgr == nil {
		adminMgr = adminauth.NewManager(adminauth.Config{
			Email:    appConfig.Admin.Email,
			Password: appConfig.Admin.Password,
			TokenTTL: time.Duration(appConfig.Admin.TokenTTLSeconds) * time.Second,
		})
	}

	statusChecker := opts.StatusChecker
	if statusChecker == nil {
		baseStore, ok := streamersStore.(*streamers.Store)
		if !ok {
			return fmt.Errorf("status checker requires *streamers.Store when not injected")
		}
		statusChecker = adminservice.StatusChecker{
			Streamers: baseStore,
			Logger:    logger,
			Search: youtubeapi.SearchClient{
				APIKey:     strings.TrimSpace(appConfig.YouTube.APIKey),
				HTTPClient: &http.Client{Timeout: 5 * time.Second},
			},
		}
	}

	primaryHost := canonicalHostFromURL(appConfig.YouTube.CallbackURL)

	siteDescription := "Sharpen.Live tracks live knife sharpeners and bladesmith streams across YouTube, Twitch, and Facebook - find makers, tutorials, and sharpening resources."
	if strings.EqualFold(siteConfig.Key, "synth-wave") || strings.EqualFold(siteConfig.Name, "synth.wave") {
		siteDescription = "synth.wave tracks live synthwave, chillwave, and electronic music streams so you can ride the neon frequencies in real time."
	}

	srv := &server{
		assetsDir:        assetsPath,
		stylesPath:       "/styles.css",
		socialImagePath:  "/og-image.png",
		logDir:           logDir,
		templates:        tmpl,
		currentYear:      time.Now().Year(),
		submitEndpoint:   "/submit",
		streamersStore:   streamersStore,
		streamerService:  streamerSvc,
		submissionsStore: submissionsStore,
		adminSubmissions: adminSubSvc,
		statusChecker:    statusChecker,
		adminManager:     adminMgr,
		logger:           logger,
		adminEmail:       appConfig.Admin.Email,
		metadataFetcher:  metadataSvc,
		siteName:         siteConfig.Name,
		siteDescription:  siteDescription,
		primaryHost:      primaryHost,
		youtubeConfig:    appConfig.YouTube,
	}

	monitorFactory := opts.NewLeaseMonitor
	if monitorFactory == nil {
		monitorFactory = subscriptions.StartLeaseMonitor
	}
	var monitor *subscriptions.LeaseMonitor
	if streamersStore.Path() != "" {
		monitor = monitorFactory(ctx, subscriptions.LeaseMonitorConfig{
			StreamersPath: streamersStore.Path(),
			Interval:      time.Minute,
			Options: subscriptions.Options{
				Client:       &http.Client{Timeout: 10 * time.Second},
				HubURL:       appConfig.YouTube.HubURL,
				Logger:       logger,
				Mode:         "subscribe",
				Verify:       appConfig.YouTube.Verify,
				LeaseSeconds: appConfig.YouTube.LeaseSeconds,
			},
			OnError: func(err error) {
				if logger != nil {
					logger.Printf("lease monitor: %v", err)
				}
			},
		})
		defer monitor.Stop()
	}

	alertPaths := alertCallbackPaths(appConfig.YouTube.CallbackURL)

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleHome)
	mux.HandleFunc("/streamers/", srv.handleStreamer)
	mux.HandleFunc("/submit", srv.handleSubmit)
	mux.Handle("/styles.css", srv.assetHandler("styles.css", "text/css"))
	mux.Handle("/submit.js", srv.assetHandler("submit.js", "application/javascript"))
	mux.Handle("/og-image.png", srv.assetHandler("og-image.png", "image/png"))
	mux.HandleFunc("/robots.txt", srv.handleRobots)
	mux.HandleFunc("/sitemap.xml", srv.handleSitemap)
	mux.HandleFunc("/admin", srv.handleAdmin)
	mux.HandleFunc("/admin/", srv.handleAdmin)
	mux.HandleFunc("/admin/login", srv.handleAdminLogin)
	mux.HandleFunc("/admin/logout", srv.handleAdminLogout)
	mux.HandleFunc("/admin/submissions", srv.handleAdminSubmission)
	mux.HandleFunc("/admin/streamers/update", srv.handleAdminStreamerUpdate)
	mux.HandleFunc("/admin/streamers/delete", srv.handleAdminStreamerDelete)
	mux.HandleFunc("/admin/status-check", srv.handleAdminStatusCheck)
	streamersWatch := streamersWatchHandler(streamersWatchOptions{
		FilePath: srv.streamersStore.Path(),
		Logger:   srv.logger,
	})
	mux.Handle("/streamers/watch", streamersWatch)
	mux.Handle("/api/streamers/watch", streamersWatch)
	mux.HandleFunc("/api/youtube/metadata", srv.handleMetadata)
	if baseStore, ok := streamersStore.(*streamers.Store); ok {
		alertsHandler := buildAlertsHandler(logger, baseStore)
		for _, path := range alertPaths {
			mux.Handle(path, alertsHandler)
		}
	}
	mux.HandleFunc("/streamers.json", srv.serveStreamersJSON)
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	server := &http.Server{
		Addr:    opts.Listen,
		Handler: logging.WithHTTPLogging(logRequests(logger, mux), logger),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	siteLabel := siteConfig.Name
	if siteConfig.Key != "" {
		siteLabel = fmt.Sprintf("%s (%s)", siteConfig.Name, siteConfig.Key)
	}

	log.Printf("Serving %s UI on http://%s", siteLabel, opts.Listen)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server shutdown: %w", err)
		}
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("server error: %w", err)
	}
}

func applyDefaults(opts Options, site config.SiteConfig) Options {
	if opts.Listen == "" {
		addr := strings.TrimSpace(site.Server.Addr)
		port := strings.TrimSpace(site.Server.Port)
		if addr != "" || port != "" {
			if port != "" && !strings.HasPrefix(port, ":") {
				opts.Listen = addr + ":" + port
			} else {
				opts.Listen = addr + port
			}
		}
		if opts.Listen == "" {
			opts.Listen = "127.0.0.1:4173"
		}
	}
	if opts.TemplatesDir == "" {
		if site.App.Templates != "" {
			opts.TemplatesDir = site.App.Templates
		} else {
			opts.TemplatesDir = "ui/templates"
		}
	}
	if opts.AssetsDir == "" {
		if site.App.Assets != "" {
			opts.AssetsDir = site.App.Assets
		} else {
			opts.AssetsDir = "ui"
		}
	}
	if opts.LogDir == "" {
		if site.App.Logs != "" {
			opts.LogDir = site.App.Logs
		} else {
			opts.LogDir = "data/logs"
		}
	}
	if opts.DataDir == "" {
		if site.App.Data != "" {
			opts.DataDir = site.App.Data
		} else {
			opts.DataDir = "data"
		}
	}
	if opts.ConfigPath == "" {
		opts.ConfigPath = "config.json"
	}
	return opts
}

func prepareLogFile(dir, name string) (*os.File, error) {
	if dir == "" {
		dir = "data/logs"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	target := filepath.Join(dir, name)

	if _, err := os.Stat(target); err == nil {
		archiveDir := filepath.Join(dir, "archive")
		if err := os.MkdirAll(archiveDir, 0o755); err != nil {
			return nil, fmt.Errorf("create log archive dir: %w", err)
		}
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		timestamp := time.Now().UTC().Format("20060102-150405")
		archived := filepath.Join(archiveDir, fmt.Sprintf("%s-%s%s", base, timestamp, ext))
		if err := os.Rename(target, archived); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("archive log file: %w", err)
		}
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return file, nil
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
	admin := filepath.Join(dir, "admin.tmpl")

	homeTmpl, err := template.New("home").Funcs(funcs).ParseFiles(base, home, submit)
	if err != nil {
		return nil, fmt.Errorf("parse home templates: %w", err)
	}
	streamerTmpl, err := template.New("streamer").Funcs(funcs).ParseFiles(base, streamer)
	if err != nil {
		return nil, fmt.Errorf("parse streamer templates: %w", err)
	}
	adminTmpl, err := template.New("admin").Funcs(funcs).ParseFiles(base, admin)
	if err != nil {
		return nil, fmt.Errorf("parse admin templates: %w", err)
	}
	return map[string]*template.Template{
		"home":     homeTmpl,
		"streamer": streamerTmpl,
		"admin":    adminTmpl,
	}, nil
}

func canonicalHostFromURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Host)
}

func (s *server) absoluteURL(r *http.Request, path string) string {
	clean := strings.TrimSpace(path)
	if clean == "" {
		clean = "/"
	}
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	scheme := "https"
	if r != nil {
		if proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))); proto != "" {
			scheme = proto
		} else if r.TLS == nil {
			scheme = "http"
		}
		if host := strings.TrimSpace(r.Host); host != "" {
			return fmt.Sprintf("%s://%s%s", scheme, host, clean)
		}
	}
	host := strings.TrimSpace(s.primaryHost)
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, clean)
}

func (s *server) socialImageURL(r *http.Request) string {
	if strings.TrimSpace(s.socialImagePath) == "" {
		return ""
	}
	return s.absoluteURL(r, s.socialImagePath)
}

func (s *server) defaultDescription() string {
	return s.siteDescription
}

func truncateWithEllipsis(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	limit := max
	if limit > len(runes) {
		limit = len(runes)
	}
	cut := limit
	for i := limit; i >= 0 && i >= limit-20; i-- {
		if runes[i] == ' ' {
			cut = i
			break
		}
	}
	trimmed := strings.TrimSpace(string(runes[:cut]))
	if trimmed == "" {
		trimmed = strings.TrimSpace(string(runes[:limit]))
	}
	return trimmed + "..."
}

func (s *server) normalizeDescription(desc string) string {
	trimmed := strings.TrimSpace(desc)
	if trimmed == "" {
		trimmed = s.defaultDescription()
	}
	return truncateWithEllipsis(trimmed, 155)
}

func (s *server) buildBasePageData(r *http.Request, title, description, canonicalPath string) basePageData {
	if strings.TrimSpace(title) == "" {
		title = s.siteName
	}
	canonical := s.absoluteURL(r, canonicalPath)
	return basePageData{
		PageTitle:       title,
		StylesheetPath:  s.stylesPath,
		SubmitLink:      "/#submit",
		CurrentYear:     s.currentYear,
		SiteName:        s.siteName,
		MetaDescription: s.normalizeDescription(description),
		CanonicalURL:    canonical,
		SocialImage:     s.socialImageURL(r),
		OGType:          "website",
	}
}

func (s *server) homeStructuredData(homeURL string) template.JS {
	if strings.TrimSpace(homeURL) == "" {
		return ""
	}
	org := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Organization",
		"name":        s.siteName,
		"url":         homeURL,
		"description": s.defaultDescription(),
	}
	payload, err := json.Marshal(org)
	if err != nil {
		return ""
	}
	return template.JS(payload)
}

func (s *server) streamerStructuredData(streamer model.Streamer, canonical string) template.JS {
	if strings.TrimSpace(streamer.Name) == "" || strings.TrimSpace(canonical) == "" {
		return ""
	}
	sameAs := make([]string, 0, len(streamer.Platforms))
	for _, platform := range streamer.Platforms {
		if url := strings.TrimSpace(platform.ChannelURL); url != "" {
			sameAs = append(sameAs, url)
		}
	}
	schema := map[string]any{
		"@context": "https://schema.org",
		"@type":    "Person",
		"name":     streamer.Name,
		"url":      canonical,
	}
	if desc := strings.TrimSpace(streamer.Description); desc != "" {
		schema["description"] = desc
	}
	if len(streamer.Languages) > 0 {
		schema["knowsLanguage"] = streamer.Languages
	}
	if len(sameAs) > 0 {
		schema["sameAs"] = sameAs
	}
	payload, err := json.Marshal(schema)
	if err != nil {
		return ""
	}
	return template.JS(payload)
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

func (s *server) handleRobots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sitemapURL := s.absoluteURL(r, "/sitemap.xml")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s\n", sitemapURL)
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type sitemapSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

func (s *server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	entries := []sitemapURL{{Loc: s.absoluteURL(r, "/")}}
	seen := map[string]struct{}{entries[0].Loc: {}}

	if s.streamersStore != nil {
		records, err := s.streamersStore.List()
		if err != nil && s.logger != nil {
			s.logger.Printf("sitemap: failed to list streamers: %v", err)
		}
		if err == nil {
			for _, rec := range records {
				id := strings.TrimSpace(rec.Streamer.ID)
				if id == "" {
					continue
				}
				loc := s.absoluteURL(r, "/streamers/"+url.PathEscape(id))
				if _, exists := seen[loc]; exists {
					continue
				}
				lastMod := rec.UpdatedAt
				if lastMod.IsZero() {
					lastMod = rec.CreatedAt
				}
				entry := sitemapURL{Loc: loc}
				if !lastMod.IsZero() {
					entry.LastMod = lastMod.UTC().Format(time.RFC3339)
				}
				entries = append(entries, entry)
				seen[loc] = struct{}{}
			}
		}
	}

	payload := sitemapSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  entries,
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))

	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		log.Printf("render sitemap: %v", err)
	}
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

	s.renderHome(w, r, formState, streamersList, rosterErr, http.StatusOK)
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
	streamersList, rosterErr := s.fetchRoster(ctx)
	if rosterErr != "" && len(streamersList) == 0 {
		http.Error(w, fmt.Sprintf("failed to load streamer roster: %v", rosterErr), http.StatusBadGateway)
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

	data := streamerPageData{Streamer: *match}
	title := fmt.Sprintf("%s · %s", match.Name, s.siteName)
	description := truncateWithEllipsis(strings.TrimSpace(match.Description), 155)
	escapedID := url.PathEscape(match.ID)
	base := s.buildBasePageData(r, title, description, "/streamers/"+escapedID)
	base.OGType = "profile"
	base.SecondaryAction = &navAction{Label: "Back to roster", Href: "/"}
	base.StructuredData = s.streamerStructuredData(*match, base.CanonicalURL)
	data.basePageData = base

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
	formState.Open = true
	removeID := strings.TrimSpace(r.FormValue("remove_platform"))
	action := strings.TrimSpace(r.FormValue("action"))

	switch {
	case removeID != "":
		formState.Platforms = removePlatformRow(formState.Platforms, removeID)
		formState.Errors.Platforms = make(map[string]model.PlatformFieldError)
		formState.Open = true
		s.renderHomeWithRoster(w, r, formState, http.StatusOK)
		return
	case action == "add-platform":
		if len(formState.Platforms) < model.MaxPlatforms {
			formState.Platforms = append(formState.Platforms, forms.NewPlatformRow())
		}
		formState.Errors.Platforms = make(map[string]model.PlatformFieldError)
		formState.Open = true
		s.renderHomeWithRoster(w, r, formState, http.StatusOK)
		return
	}

	errors := forms.ValidateSubmitForm(&formState)
	formState.Errors = errors
	if errors.Name || errors.Description || errors.Languages || len(errors.Platforms) > 0 {
		s.renderHomeWithRoster(w, r, formState, http.StatusUnprocessableEntity)
		return
	}

	s.maybeEnrichMetadata(r.Context(), &formState)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	message, err := submitStreamer(ctx, s.streamerService, formState)
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

func (s *server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var req model.MetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	data, err := s.metadataFetcher.Fetch(ctx, req.URL)
	if err != nil {
		if errors.Is(err, youtubeservice.ErrValidation) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to fetch metadata", http.StatusBadGateway)
		return
	}
	resp := model.MetadataResponse{
		Description: data.Description,
		Title:       data.Title,
		Handle:      data.Handle,
		ChannelID:   data.ChannelID,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *server) renderHome(w http.ResponseWriter, r *http.Request, formState model.SubmitFormState, roster []model.Streamer, rosterErr string, status int) {
	if !formState.Open && (formState.ResultState != "" || formState.ResultMessage != "" || hasSubmitErrors(formState.Errors)) {
		formState.Open = true
	}
	ensureSubmitDefaults(&formState)
	homeTitle := fmt.Sprintf("%s · Live knife sharpeners & bladesmith streams", s.siteName)
	base := s.buildBasePageData(r, homeTitle, s.defaultDescription(), "/")
	base.StructuredData = s.homeStructuredData(base.CanonicalURL)
	data := homePageData{
		basePageData: base,
		Streamers:    roster,
		RosterError:  rosterErr,
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
	s.renderHome(w, r, formState, streamersList, rosterErr, status)
}

func (s *server) fetchRoster(ctx context.Context) ([]model.Streamer, string) {
	if s.streamersStore != nil {
		records, err := s.streamersStore.List()
		if err != nil {
			return nil, err.Error()
		}
		return mapStreamerRecords(records), ""
	}

	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	streamersList, err := uistreamers.FetchStreamers(ctx)
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

func hasSubmitErrors(errs model.SubmitFormErrors) bool {
	if errs.Name || errs.Description || errs.Languages {
		return true
	}
	for _, p := range errs.Platforms {
		if p.Channel {
			return true
		}
	}
	return false
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

func (s *server) maybeEnrichMetadata(ctx context.Context, form *model.SubmitFormState) {
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

	metadata, err := s.metadataFetcher.Fetch(ctx, target)
	if err != nil {
		return
	}

	if desc == "" {
		if trimmed := strings.TrimSpace(metadata.Description); trimmed != "" {
			form.Description = trimmed
			desc = trimmed
		} else if title := strings.TrimSpace(metadata.Title); desc == "" && title != "" {
			form.Description = title
		}
	}
	if name == "" {
		if title := strings.TrimSpace(metadata.Title); title != "" {
			form.Name = title
		}
	}
}

func submitStreamer(ctx context.Context, streamerSvc StreamerService, form model.SubmitFormState) (string, error) {
	if streamerSvc == nil {
		return "", errors.New("streamer service unavailable")
	}
	req := streamersvc.CreateRequest{
		Alias:       strings.TrimSpace(form.Name),
		Description: forms.BuildStreamerDescription(form.Description, form.Platforms),
		Languages:   append([]string(nil), form.Languages...),
		PlatformURL: forms.FirstPlatformURL(form.Platforms),
	}
	result, err := streamerSvc.Create(ctx, req)
	if err != nil {
		return "", err
	}
	alias := strings.TrimSpace(result.Submission.Alias)
	id := strings.TrimSpace(result.Submission.ID)
	switch {
	case alias != "" && id != "":
		return fmt.Sprintf("%s queued with submission %s.", alias, id), nil
	case alias != "":
		return fmt.Sprintf("%s submitted for review.", alias), nil
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

func streamersWatchHandler(opts streamersWatchOptions) http.Handler {
	interval := opts.PollInterval
	if interval <= 0 {
		interval = defaultWatchPollInterval
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeMessage := func(ts time.Time, flusher http.Flusher) {
			fmt.Fprintf(w, "data: %d\n\n", ts.UnixMilli())
			flusher.Flush()
		}

		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if opts.FilePath == "" {
			http.Error(w, "streamers path not configured", http.StatusInternalServerError)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		lastMod, _ := fileModTime(opts.FilePath)
		writeMessage(lastMod, flusher)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				mod, err := fileModTime(opts.FilePath)
				if err != nil {
					if !errors.Is(err, os.ErrNotExist) && opts.Logger != nil {
						opts.Logger.Printf("streamers watch: stat failed: %v", err)
					}
					continue
				}
				if mod.After(lastMod) {
					lastMod = mod
					writeMessage(mod, flusher)
				}
			}
		}
	})
}

type streamersWatchOptions struct {
	FilePath     string
	Logger       logging.Logger
	PollInterval time.Duration
}

const defaultWatchPollInterval = 2 * time.Second

func fileModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func (s *server) serveStreamersJSON(w http.ResponseWriter, r *http.Request) {
	if s.streamersStore == nil {
		http.Error(w, "streamers store unavailable", http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, s.streamersStore.Path())
}

func mapStreamerRecords(records []streamers.Record) []model.Streamer {
	if len(records) == 0 {
		return nil
	}
	online := make([]model.Streamer, 0, len(records))
	offline := make([]model.Streamer, 0, len(records))
	for _, rec := range records {
		mapped := mapStreamerRecord(rec)
		if strings.EqualFold(mapped.Status, "online") {
			online = append(online, mapped)
		} else {
			offline = append(offline, mapped)
		}
	}
	return append(online, offline...)
}

func mapStreamerRecord(rec streamers.Record) model.Streamer {
	name := strings.TrimSpace(rec.Streamer.Alias)
	if name == "" {
		name = strings.TrimSpace(rec.Streamer.ID)
	}
	state, label := deriveStatusFromRecord(rec.Status)
	return model.Streamer{
		ID:          rec.Streamer.ID,
		Name:        name,
		Description: strings.TrimSpace(rec.Streamer.Description),
		Status:      state,
		StatusLabel: label,
		Languages:   append([]string(nil), rec.Streamer.Languages...),
		Platforms:   collectPlatformsFromRecord(rec.Platforms, rec.Status),
	}
}

func deriveStatusFromRecord(status *streamers.Status) (string, string) {
	if status == nil {
		return "offline", "Offline"
	}
	if status.Live {
		return "online", "Online"
	}
	if len(status.Platforms) > 0 {
		return "busy", "Workshop"
	}
	return "offline", "Offline"
}

func collectPlatformsFromRecord(details streamers.Platforms, status *streamers.Status) []model.Platform {
	var platforms []model.Platform
	if yt := details.YouTube; yt != nil {
		id := strings.TrimSpace(yt.ChannelID)
		handle := strings.TrimPrefix(strings.TrimSpace(yt.Handle), "@")
		if id == "" {
			id = handle
		}
		url := youtubeLiveURL(yt, status)
		if url == "" {
			url = youtubeChannelURL(yt)
		}
		if url != "" {
			platforms = append(platforms, model.Platform{
				ID:         id,
				Name:       "YouTube",
				ChannelURL: url,
			})
		}
	}
	if tw := details.Twitch; tw != nil {
		if url := twitchChannelURL(tw); url != "" {
			platforms = append(platforms, model.Platform{
				Name:       "Twitch",
				ChannelURL: url,
			})
		}
	}
	if fb := details.Facebook; fb != nil {
		if url := facebookPageURL(fb); url != "" {
			platforms = append(platforms, model.Platform{
				Name:       "Facebook",
				ChannelURL: url,
			})
		}
	}
	return platforms
}

func youtubeChannelURL(details *streamers.YouTubePlatform) string {
	handle := strings.TrimSpace(details.Handle)
	if handle != "" {
		if !strings.HasPrefix(handle, "@") {
			handle = "@" + handle
		}
		return "https://www.youtube.com/" + handle
	}
	channel := strings.TrimSpace(details.ChannelID)
	if channel != "" {
		return "https://www.youtube.com/channel/" + channel
	}
	const feedPrefix = "https://www.youtube.com/xml/feeds/videos.xml?channel_id="
	if topic := strings.TrimSpace(details.Topic); strings.HasPrefix(topic, feedPrefix) {
		return "https://www.youtube.com/channel/" + topic[len(feedPrefix):]
	}
	return ""
}

func youtubeLiveURL(details *streamers.YouTubePlatform, status *streamers.Status) string {
	if status == nil || status.YouTube == nil || !status.YouTube.Live {
		return ""
	}
	if videoID := strings.TrimSpace(status.YouTube.VideoID); videoID != "" {
		return "https://www.youtube.com/watch?v=" + videoID
	}
	return youtubeChannelURL(details)
}

func twitchChannelURL(details *streamers.TwitchPlatform) string {
	username := strings.TrimSpace(details.Username)
	if username == "" {
		return ""
	}
	return "https://www.twitch.tv/" + username
}

func facebookPageURL(details *streamers.FacebookPlatform) string {
	pageID := strings.TrimSpace(details.PageID)
	if pageID == "" {
		return ""
	}
	return "https://www.facebook.com/" + pageID
}

func logRequests(logger logging.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start).Truncate(time.Millisecond)
		msg := fmt.Sprintf("[http-request] %s %s host=%s duration=%s ua=%q", r.Method, r.URL.Path, r.Host, duration, r.UserAgent())
		if logger == nil {
			log.Print(msg)
			return
		}
		requestID := logging.RequestIDFromContext(r.Context())
		if strings.TrimSpace(requestID) != "" {
			logging.LogWithID(logger, "general", requestID, msg)
			return
		}
		logger.Printf("%s", msg)
	})
}

func buildAlertsHandler(logger logging.Logger, streamersStore *streamers.Store) http.Handler {
	opts := youtubehandlers.AlertNotificationOptions{
		Logger:         logger,
		StreamersStore: streamersStore,
		VideoLookup:    &liveinfo.Client{Logger: logger},
	}
	return handleAlerts(opts)
}

// handleAlerts mirrors the alertserver webhook endpoint for YouTube WebSub.
func handleAlerts(notificationOpts youtubehandlers.AlertNotificationOptions) http.Handler {
	allowedMethods := strings.Join([]string{http.MethodGet, http.MethodPost}, ", ")
	logger := notificationOpts.Logger
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !youtubehandlers.IsAlertPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}

		userAgent := r.Header.Get("User-Agent")
		from := r.Header.Get("From")
		forwardedFor := r.Header.Get("X-Forwarded-For")
		platform := alertPlatform(userAgent, from)

		switch r.Method {
		case http.MethodGet:
			if platform == "youtube" {
				if youtubehandlers.HandleSubscriptionConfirmation(w, r, youtubehandlers.SubscriptionConfirmationOptions{
					Logger:         logger,
					StreamersStore: notificationOpts.StreamersStore,
				}) {
					return
				}
				http.Error(w, "invalid subscription confirmation", http.StatusBadRequest)
				return
			}
			if logger != nil {
				logger.Printf("suspicious /alerts GET request: platform=%q ua=%q from=%q xff=%q", platform, userAgent, from, forwardedFor)
			}
			w.Header().Set("Allow", allowedMethods)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		case http.MethodPost:
			if platform != "youtube" {
				if logger != nil {
					logger.Printf("suspicious /alerts POST request: platform=%q ua=%q from=%q xff=%q", platform, userAgent, from, forwardedFor)
				}
				w.Header().Set("Allow", allowedMethods)
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if youtubehandlers.HandleAlertNotification(w, r, notificationOpts) {
				return
			}
			http.Error(w, "failed to process notification", http.StatusInternalServerError)
		default:
			w.Header().Set("Allow", allowedMethods)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func alertPlatform(userAgent, from string) string {
	ua := strings.ToLower(userAgent)
	from = strings.ToLower(from)
	switch {
	case strings.Contains(ua, "google"):
		return "youtube"
	case strings.Contains(ua, "youtube"):
		return "youtube"
	case strings.Contains(from, "google.com"):
		return "youtube"
	case strings.Contains(from, "youtube.com"):
		return "youtube"
	default:
		return ""
	}
}

func alertCallbackPaths(callbackURL string) []string {
	paths := []string{"/alerts", "/alert"}
	path := resolveCallbackPath(callbackURL)
	if path != "" {
		paths = append(paths, path)
		switch {
		case strings.HasSuffix(path, "/alerts"):
			paths = append(paths, strings.TrimSuffix(path, "s"))
		case strings.HasSuffix(path, "/alert"):
			paths = append(paths, path+"s")
		}
	}
	return dedupePaths(paths)
}

func resolveCallbackPath(callbackURL string) string {
	u, err := url.Parse(strings.TrimSpace(callbackURL))
	if err != nil {
		return ""
	}
	path := strings.TrimRight(u.Path, "/")
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func dedupePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		result = append(result, p)
	}
	return result
}
