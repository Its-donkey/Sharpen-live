package server

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	adminauth "github.com/Its-donkey/Sharpen-live/internal/alert/admin/auth"
	adminservice "github.com/Its-donkey/Sharpen-live/internal/alert/admin/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
	twitchhandlers "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/twitch/handlers"
	youtubeapi "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/api"
	youtubeservice "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/subscriptions"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	streamersvc "github.com/Its-donkey/Sharpen-live/internal/alert/streamers/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/submissions"
	"github.com/Its-donkey/Sharpen-live/internal/metadata"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	youtubeui "github.com/Its-donkey/Sharpen-live/internal/ui/platforms/youtube"
	"github.com/Its-donkey/Sharpen-live/logging"
)

type Options struct {
	Listen         string
	TemplatesDir   string
	AssetsDir      string
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

// MetadataService fetches channel metadata for a given URL.
type MetadataService interface {
	Fetch(ctx context.Context, url string) (*metadata.Metadata, error)
}

// LeaseMonitorFactory constructs a lease monitor. Useful for tests that avoid background goroutines.
type LeaseMonitorFactory func(context.Context, subscriptions.LeaseMonitorConfig) *subscriptions.LeaseMonitor

type server struct {
	assetsDir        string
	stylesPath       string
	socialImagePath  string
	templates        map[string]*template.Template
	currentYear      int
	submitEndpoint   string
	streamersStore   StreamersStore
	streamerService  StreamerService
	submissionsStore *submissions.Store
	adminSubmissions AdminSubmissions
	statusChecker    StatusChecker
	adminManager     AdminManager
	metadataService  MetadataService
	adminEmail       string
	metadataFetcher  MetadataFetcher
	siteName         string
	siteKey          string
	siteDescription  string
	primaryHost      string
	youtubeConfig    config.YouTubeConfig
	twitchConfig     config.TwitchConfig
	configPath       string
	fallbackErrors   []string
	logger           *logging.Logger
	logDir           string
	availableSites   []string

	// Store cache for multi-site WebSub support
	storeCache   map[string]*streamers.Store
	storeCacheMu sync.RWMutex
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
	usingAlertserver := false
	if err != nil {
		appendFallback(fmt.Sprintf("failed to load config: %v", err))
		appConfig = config.DefaultConfig()
		usingAlertserver = true
	}
	var siteConfig config.SiteConfig
	if opts.Site == config.AlertserverKey {
		siteConfig = config.Alertserver(appConfig)
		usingAlertserver = true
	} else {
		siteConfig, err = config.ResolveSite(opts.Site, appConfig)
		if err != nil {
			appendFallback(fmt.Sprintf("site %q not found: %v", opts.Site, err))
			siteConfig = config.Alertserver(appConfig)
			usingAlertserver = true
		}
	}
	if siteConfig.Key == config.AlertserverKey || strings.EqualFold(siteConfig.Name, config.AlertserverKey) || strings.EqualFold(siteConfig.Name, "default-site") {
		usingAlertserver = true
	}
	opts = applyDefaults(opts, siteConfig)

	templateRoot, err := filepath.Abs(opts.TemplatesDir)
	if err != nil {
		if usingAlertserver {
			return fmt.Errorf("resolve templates dir: %w", err)
		}
		appendFallback(fmt.Sprintf("template path %q failed: %v", opts.TemplatesDir, err))
		siteConfig, opts = switchToAlertserver(appConfig, opts)
		usingAlertserver = true
		templateRoot, err = filepath.Abs(opts.TemplatesDir)
		if err != nil {
			return fmt.Errorf("resolve default-site templates dir: %w", err)
		}
	}

	tmpl := opts.Templates
	if tmpl == nil {
		loaded, err := loadTemplates(templateRoot)
		if err != nil {
			if usingAlertserver {
				return fmt.Errorf("load default-site templates: %w", err)
			}
			appendFallback(fmt.Sprintf("failed to load templates from %s: %v", templateRoot, err))
			siteConfig, opts = switchToAlertserver(appConfig, opts)
			usingAlertserver = true
			templateRoot, err = filepath.Abs(opts.TemplatesDir)
			if err != nil {
				return fmt.Errorf("resolve default-site templates dir: %w", err)
			}
			loaded, err = loadTemplates(templateRoot)
			if err != nil {
				return fmt.Errorf("load default-site templates: %w", err)
			}
		}
		tmpl = loaded
	}

	assetsPath, err := filepath.Abs(opts.AssetsDir)
	if err != nil {
		return fmt.Errorf("resolve assets dir: %w", err)
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
			Streamers:          baseStore,
			Submissions:        submissionsStore,
			YouTubeHubURL:      siteConfig.YouTube.HubURL,
			YouTubeCallbackURL: siteConfig.YouTube.CallbackURL,
		})
	}
	metadataSvc := opts.MetadataFetcher
	if metadataSvc == nil {
		metadataSvc = youtubeservice.MetadataService{
			Client:  &http.Client{Timeout: 5 * time.Second},
			Timeout: 5 * time.Second,
		}
	}

	// Initialize logging
	logDir := filepath.Join(dataDir, "logs")
	fileWriter, err := logging.NewFileWriter(logDir, "app.log", 50, 10)
	if err != nil {
		return fmt.Errorf("create log file writer: %w", err)
	}
	defer fileWriter.Close()

	logger := logging.New(siteConfig.Key, logging.INFO, fileWriter, os.Stdout)
	logger.Info("server", "Starting server", map[string]any{
		"site":   siteConfig.Name,
		"listen": opts.Listen,
	})

	// Initialize metadata service
	metadataService := metadata.NewService(&http.Client{
		Timeout: 10 * time.Second,
	}, logger, siteConfig.YouTube.APIKey)

	// Resolve WebSub callback URL and path - prioritize config.json over env var
	websubCallbackURL := strings.TrimSpace(siteConfig.YouTube.CallbackURL)
	websubCallbackSource := ""
	if websubCallbackURL != "" {
		websubCallbackSource = "config.json (youtube.callback_url)"
	} else {
		websubCallbackURL = strings.TrimSpace(os.Getenv("WEBSUB_CALLBACK_BASE_URL"))
		if websubCallbackURL != "" {
			websubCallbackSource = "environment variable (WEBSUB_CALLBACK_BASE_URL)"
		}
	}

	// Always register the handler at /alerts (the path after reverse proxy stripping)
	// The full callback URL (e.g., https://sharpen.live/dev/alerts) is sent to YouTube,
	// but the reverse proxy strips prefixes before forwarding to the Go app
	websubCallbackPath := "/alerts"

	if websubCallbackURL != "" {
		logger.Info("websub", "YouTube WebSub configured", map[string]any{
			"callbackUrl":      websubCallbackURL,
			"localHandlerPath": websubCallbackPath,
			"source":           websubCallbackSource,
			"note":             "Handler registered at local path; reverse proxy handles URL rewriting",
		})
	} else {
		logger.Warn("websub", "YouTube WebSub not configured - set config.json youtube.callback_url or WEBSUB_CALLBACK_BASE_URL env var", nil)
	}

	adminSubSvc := opts.AdminSubmissions
	if adminSubSvc == nil {
		baseStore, ok := streamersStore.(*streamers.Store)
		if !ok {
			return fmt.Errorf("admin submissions requires *streamers.Store when not injected")
		}
		adminSubSvc = adminservice.NewSubmissionsService(adminservice.SubmissionsOptions{
			SubmissionsStore:      submissionsStore,
			StreamersStore:        baseStore,
			WebSubCallbackBaseURL: websubCallbackURL,
			MetadataService:       metadataService,
			YouTubeAPIKey:         siteConfig.YouTube.APIKey,
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
				APIKey:     strings.TrimSpace(siteConfig.YouTube.APIKey),
				HTTPClient: &http.Client{Timeout: 5 * time.Second},
			},
		}
	}

	primaryHost := canonicalHostFromURL(websubCallbackURL)

	siteDescription := "Sharpen.Live tracks live knife sharpeners and bladesmith streams across YouTube, Twitch, and Facebook - find makers, tutorials, and sharpening resources."
	switch {
	case strings.EqualFold(siteConfig.Key, config.AlertserverKey) || strings.EqualFold(siteConfig.Name, config.AlertserverKey) || strings.EqualFold(siteConfig.Name, "default-site"):
		siteDescription = "Alertserver Admin appears when a requested site cannot be served. Review the errors below to restore the site configuration."
	case strings.EqualFold(siteConfig.Key, "synth-wave") || strings.EqualFold(siteConfig.Name, "synth.wave"):
		siteDescription = "synth.wave tracks live synthwave, chillwave, and electronic music streams so you can ride the neon frequencies in real time."
	}

	srv := &server{
		assetsDir:        assetsPath,
		stylesPath:       "/styles.css",
		socialImagePath:  "/og-image.png",
		templates:        tmpl,
		currentYear:      time.Now().Year(),
		submitEndpoint:   "/submit",
		streamersStore:   streamersStore,
		streamerService:  streamerSvc,
		submissionsStore: submissionsStore,
		adminSubmissions: adminSubSvc,
		statusChecker:    statusChecker,
		adminManager:     adminMgr,
		adminEmail:       appConfig.Admin.Email,
		metadataService:  metadataService,
		metadataFetcher:  metadataSvc,
		siteName:         siteConfig.Name,
		siteKey:          siteConfig.Key,
		siteDescription:  siteDescription,
		primaryHost:      primaryHost,
		youtubeConfig:    siteConfig.YouTube,
		twitchConfig:     siteConfig.Twitch,
		configPath:       opts.ConfigPath,
		fallbackErrors:   opts.FallbackErrors,
		logger:           logger,
		logDir:           logDir,
		availableSites:   configuredSiteKeys(appConfig),
		storeCache:       make(map[string]*streamers.Store),
	}

	// Check initial live status for all streamers in background
	logger.Info("startup", "Starting initial live status check for all streamers in background", nil)
	go func() {
		// Use a fresh context with generous timeout for initial checks
		checkCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		srv.checkAllStreamersLiveStatus(checkCtx)
	}()

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
				HubURL:       siteConfig.YouTube.HubURL,
				Mode:         "subscribe",
				Verify:       siteConfig.YouTube.Verify,
				CallbackURL:  siteConfig.YouTube.CallbackURL,
				LeaseSeconds: siteConfig.YouTube.LeaseSeconds,
			},
			OnError: func(err error) {

			},
		})
		defer monitor.Stop()
	}

	alertPaths := youtubeui.CallbackPaths(siteConfig.YouTube.CallbackURL)

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
	mux.HandleFunc("/admin/youtube/settings", srv.handleAdminYouTubeSettings)
	mux.HandleFunc("/admin/config", srv.handleAdminConfig)
	mux.HandleFunc("/admin/platform/settings", srv.handleAdminPlatformSettings)
	streamersWatch := streamersWatchHandler(streamersWatchOptions{
		FilePath: srv.streamersStore.Path(),
	})
	mux.Handle("/streamers/watch", streamersWatch)
	mux.Handle("/api/streamers/watch", streamersWatch)
	mux.HandleFunc("/api/metadata", srv.handleMetadata)
	// mux.HandleFunc("/api/youtube/metadata", srv.handleMetadata)
	websubRegistered := false
	if websubCallbackURL != "" {
		mux.HandleFunc(websubCallbackPath, srv.handleYouTubeWebSub)
		websubRegistered = true
	}
	if baseStore, ok := streamersStore.(*streamers.Store); ok {
		alertsHandler := youtubeui.NewAlertsHandler(youtubeui.AlertsHandlerOptions{
			StreamersStore: baseStore,
		})
		for _, path := range alertPaths {
			if websubRegistered && path == websubCallbackPath {
				continue
			}
			mux.Handle(path, alertsHandler)
		}
	}
	mux.HandleFunc("/streamers.json", srv.serveStreamersJSON)

	// Twitch EventSub webhook handler
	if baseStore, ok := streamersStore.(*streamers.Store); ok {
		twitchEventSubHandler := &twitchhandlers.EventSubHandler{
			Secret:         siteConfig.Twitch.EventSubSecret,
			StreamersStore: baseStore,
			Logger:         logger,
			GetAllStores:   srv.getAllStreamerStores,
		}
		mux.Handle("/twitch/eventsub", twitchEventSubHandler)
	}

	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	mux.HandleFunc("/logs", srv.handleLogs)
	mux.HandleFunc("/logs/stream", srv.handleLogsStream)
	mux.HandleFunc("/oglogs", srv.handleLogs)

	// Wrap with logging middleware
	httpLogger := logging.NewHTTPLogger(logger, 10*1024)
	handler := httpLogger.Middleware(mux)

	server := &http.Server{
		Addr:    opts.Listen,
		Handler: handler,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

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
