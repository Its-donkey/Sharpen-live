package server

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	adminauth "github.com/Its-donkey/Sharpen-live/internal/alert/admin/auth"
	adminservice "github.com/Its-donkey/Sharpen-live/internal/alert/admin/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
	youtubeapi "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/api"
	youtubeservice "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/subscriptions"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	streamersvc "github.com/Its-donkey/Sharpen-live/internal/alert/streamers/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/submissions"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	youtubeui "github.com/Its-donkey/Sharpen-live/internal/ui/platforms/youtube"
	"github.com/Its-donkey/Sharpen-live/logging"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	// Options configures the UI HTTP server.
)

type Options struct {
	Listen         string
	TemplatesDir   string
	AssetsDir      string
	LogDir         string
	DataDir        string
	ConfigPath     string
	Site           string
	FallbackErrors []string
	Templates      map[string]*template.Template

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
	logger           *logging.SiteLogger
	adminSubmissions AdminSubmissions
	statusChecker    StatusChecker
	adminManager     AdminManager
	adminEmail       string
	metadataFetcher  MetadataFetcher
	siteName         string
	siteKey          string
	siteDescription  string
	primaryHost      string
	youtubeConfig    config.YouTubeConfig
	fallbackErrors   []string
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
	FallbackErrors  []string
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

	appendFallback := func(msg string) {
		for _, existing := range opts.FallbackErrors {
			if existing == msg {
				return
			}
		}
		opts.FallbackErrors = append(opts.FallbackErrors, msg)
	}

	appConfig, err := config.Load(opts.ConfigPath)
	usingCatchAll := false
	if err != nil {
		appendFallback(fmt.Sprintf("failed to load config: %v", err))
		appConfig = config.DefaultConfig()
		usingCatchAll = true
	}
	var siteConfig config.SiteConfig
	if opts.Site == config.CatchAllSiteKey {
		siteConfig = config.CatchAllSite(appConfig)
		usingCatchAll = true
	} else {
		siteConfig, err = config.ResolveSite(opts.Site, appConfig)
		if err != nil {
			appendFallback(fmt.Sprintf("site %q not found: %v", opts.Site, err))
			siteConfig = config.CatchAllSite(appConfig)
			usingCatchAll = true
		}
	}
	if siteConfig.Key == config.CatchAllSiteKey || strings.EqualFold(siteConfig.Name, config.CatchAllSiteKey) || strings.EqualFold(siteConfig.Name, "catch-all") {
		usingCatchAll = true
	}
	opts = applyDefaults(opts, siteConfig)

	templateRoot, err := filepath.Abs(opts.TemplatesDir)
	if err != nil {
		if usingCatchAll {
			return fmt.Errorf("resolve templates dir: %w", err)
		}
		appendFallback(fmt.Sprintf("template path %q failed: %v", opts.TemplatesDir, err))
		siteConfig, opts = switchToCatchAll(appConfig, opts)
		usingCatchAll = true
		templateRoot, err = filepath.Abs(opts.TemplatesDir)
		if err != nil {
			return fmt.Errorf("resolve catch-all templates dir: %w", err)
		}
	}

	tmpl := opts.Templates
	if tmpl == nil {
		loaded, err := loadTemplates(templateRoot)
		if err != nil {
			if usingCatchAll {
				return fmt.Errorf("load catch-all templates: %w", err)
			}
			appendFallback(fmt.Sprintf("failed to load templates from %s: %v", templateRoot, err))
			siteConfig, opts = switchToCatchAll(appConfig, opts)
			usingCatchAll = true
			templateRoot, err = filepath.Abs(opts.TemplatesDir)
			if err != nil {
				return fmt.Errorf("resolve catch-all templates dir: %w", err)
			}
			loaded, err = loadTemplates(templateRoot)
			if err != nil {
				return fmt.Errorf("load catch-all templates: %w", err)
			}
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
	logger, err := logging.NewSiteLogger(logDir, siteConfig.Key)
	if err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}

	dataDir := opts.DataDir
	if dataDir == "" {
		dataDir = "data"
	}
	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("resolve data dir: %w", err)
	}

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
			Search: youtubeapi.SearchClient{
				APIKey:     strings.TrimSpace(appConfig.YouTube.APIKey),
				HTTPClient: &http.Client{Timeout: 5 * time.Second},
			},
		}
	}

	primaryHost := canonicalHostFromURL(appConfig.YouTube.CallbackURL)

	siteDescription := "Sharpen.Live tracks live knife sharpeners and bladesmith streams across YouTube, Twitch, and Facebook - find makers, tutorials, and sharpening resources."
	switch {
	case strings.EqualFold(siteConfig.Key, config.CatchAllSiteKey) || strings.EqualFold(siteConfig.Name, config.CatchAllSiteKey) || strings.EqualFold(siteConfig.Name, "catch-all"):
		siteDescription = "This catch-all page appears when a requested site cannot be served. Review the errors below to restore the site configuration."
	case strings.EqualFold(siteConfig.Key, "synth-wave") || strings.EqualFold(siteConfig.Name, "synth.wave"):
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
		logger:           logger,
		adminSubmissions: adminSubSvc,
		statusChecker:    statusChecker,
		adminManager:     adminMgr,
		adminEmail:       appConfig.Admin.Email,
		metadataFetcher:  metadataSvc,
		siteName:         siteConfig.Name,
		siteKey:          siteConfig.Key,
		siteDescription:  siteDescription,
		primaryHost:      primaryHost,
		youtubeConfig:    appConfig.YouTube,
		fallbackErrors:   opts.FallbackErrors,
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
				Mode:         "subscribe",
				Verify:       appConfig.YouTube.Verify,
				LeaseSeconds: appConfig.YouTube.LeaseSeconds,
			},
			OnError: func(err error) {

			},
		})
		defer monitor.Stop()
	}

	alertPaths := youtubeui.CallbackPaths(appConfig.YouTube.CallbackURL)

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
	})
	mux.Handle("/streamers/watch", streamersWatch)
	mux.Handle("/api/streamers/watch", streamersWatch)
	mux.HandleFunc("/api/youtube/metadata", srv.handleMetadata)
	if baseStore, ok := streamersStore.(*streamers.Store); ok {
		alertsHandler := youtubeui.NewAlertsHandler(youtubeui.AlertsHandlerOptions{
			StreamersStore: baseStore,
			Logger:         srv.logger,
		})
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
		Handler: logging.WithHTTPLogging(srv.logger, mux),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	siteLabel := siteConfig.Name
	if siteConfig.Key != "" {
		siteLabel = fmt.Sprintf("%s (%s)", siteConfig.Name, siteConfig.Key)
	}

	logLine := fmt.Sprintf("Serving %s UI on http://%s", siteLabel, opts.Listen)
	if len(opts.FallbackErrors) > 0 {
		logLine = fmt.Sprintf("%s (fallback: %s)", logLine, strings.Join(opts.FallbackErrors, "; "))
	}
	logging.Logf(srv.logger, "%s", logLine)

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

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type sitemapSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}
