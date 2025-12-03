package server

import (
	"context"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	adminauth "github.com/Its-donkey/Sharpen-live/internal/alert/admin/auth"
	adminservice "github.com/Its-donkey/Sharpen-live/internal/alert/admin/service"
	youtubeservice "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	streamersvc "github.com/Its-donkey/Sharpen-live/internal/alert/streamers/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/submissions"
	"github.com/Its-donkey/Sharpen-live/internal/metadata"
	"github.com/Its-donkey/Sharpen-live/logging"
)

func TestHandleHomeRendersStreamers(t *testing.T) {
	srv := newTestServer()
	srv.streamersStore = &stubStreamersStore{
		records: []streamers.Record{
			{Streamer: streamers.Streamer{ID: "one", Alias: "First"}},
			{Streamer: streamers.Streamer{ID: "two", Alias: "Second"}},
		},
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	srv.handleHome(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "Sharpen.Live") {
		t.Fatalf("expected page title, got %q", body)
	}
}

func TestHandleSubmitValidatesAndRedirects(t *testing.T) {
	streamerSvc := &stubStreamerService{
		createResult: streamersvc.CreateResult{
			Submission: submissions.Submission{Alias: "New", ID: "sub123"},
		},
	}
	srv := newTestServer()
	srv.streamerService = streamerSvc

	form := url.Values{
		"name":         {"New"},
		"description":  {"Desc"},
		"languages":    {"English"},
		"platform_url": {"https://example.com"},
	}
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	srv.handleSubmit(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", rr.Code)
	}
	if !streamerSvc.created {
		t.Fatalf("expected create to be called")
	}
	location := rr.Header().Get("Location")
	if !strings.Contains(location, "submitted=1") {
		t.Fatalf("expected success redirect, got %q", location)
	}
}

func TestHandleSubmitValidationError(t *testing.T) {
	srv := newTestServer()
	form := url.Values{
		"name":         {""},
		"description":  {""},
		"languages":    {""},
		"platform_url": {""},
	}
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	srv.handleSubmit(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestHandleAdminRequiresValidToken(t *testing.T) {
	adminMgr := &stubAdminManager{
		token: adminauth.Token{Value: "token"},
		valid: true,
	}
	srv := newTestServer()
	srv.adminManager = adminMgr
	srv.adminSubmissions = &stubAdminSubmissions{
		list: []submissions.Submission{{ID: "a", Alias: "X"}},
	}

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: "token"})
	rr := httptest.NewRecorder()

	srv.handleAdmin(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, "LoggedIn:true") {
		t.Fatalf("expected logged-in indicator, got %q", body)
	}
}

func TestHandleAdminLoginSetsSession(t *testing.T) {
	adminMgr := &stubAdminManager{
		token: adminauth.Token{Value: "tok", ExpiresAt: time.Now().Add(time.Hour)},
		valid: true,
	}
	srv := newTestServer()
	srv.adminManager = adminMgr

	form := url.Values{
		"email":    {"admin@example.com"},
		"password": {"secret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	srv.handleAdminLogin(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", rr.Code)
	}
	found := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == adminCookieName && c.Value == "tok" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected session cookie to be set")
	}
}

func TestHandleAdminSubmissionApprove(t *testing.T) {
	adminMgr := &stubAdminManager{valid: true, token: adminauth.Token{Value: "tok"}}
	adminSubs := &stubAdminSubmissions{}
	srv := newTestServer()
	srv.adminManager = adminMgr
	srv.adminSubmissions = adminSubs

	form := url.Values{"id": {"123"}, "action": {"approve"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/submissions", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: "tok"})
	rr := httptest.NewRecorder()

	srv.handleAdminSubmission(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", rr.Code)
	}
	if adminSubs.lastReq.ID != "123" || adminSubs.lastReq.Action != adminservice.ActionApprove {
		t.Fatalf("unexpected admin action: %+v", adminSubs.lastReq)
	}
	if loc := rr.Header().Get("Location"); !strings.Contains(loc, "msg=Submission+approved.") {
		t.Fatalf("expected success message, got %q", loc)
	}
}

func TestHandleAdminStreamerUpdate(t *testing.T) {
	adminMgr := &stubAdminManager{valid: true, token: adminauth.Token{Value: "tok"}}
	streamerSvc := &stubStreamerService{}
	srv := newTestServer()
	srv.adminManager = adminMgr
	srv.streamerService = streamerSvc

	form := url.Values{
		"id":          {"id-1"},
		"alias":       {"Alias"},
		"description": {"Desc"},
		"languages":   {"en, fr"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/streamers/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: "tok"})
	rr := httptest.NewRecorder()

	srv.handleAdminStreamerUpdate(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", rr.Code)
	}
	if streamerSvc.lastUpdate.ID != "id-1" {
		t.Fatalf("expected update ID to be set, got %+v", streamerSvc.lastUpdate)
	}
	if streamerSvc.lastUpdate.Alias == nil || *streamerSvc.lastUpdate.Alias != "Alias" {
		t.Fatalf("expected alias pointer set, got %+v", streamerSvc.lastUpdate.Alias)
	}
	if streamerSvc.lastUpdate.Languages == nil || len(*streamerSvc.lastUpdate.Languages) != 2 {
		t.Fatalf("expected languages parsed, got %+v", streamerSvc.lastUpdate.Languages)
	}
}

func TestHandleAdminStreamerDelete(t *testing.T) {
	adminMgr := &stubAdminManager{valid: true, token: adminauth.Token{Value: "tok"}}
	streamerSvc := &stubStreamerService{}
	srv := newTestServer()
	srv.adminManager = adminMgr
	srv.streamerService = streamerSvc

	form := url.Values{"id": {"deadbeef"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/streamers/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: adminCookieName, Value: "tok"})
	rr := httptest.NewRecorder()

	srv.handleAdminStreamerDelete(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", rr.Code)
	}
	if streamerSvc.lastDelete.ID != "deadbeef" {
		t.Fatalf("expected delete to receive id, got %+v", streamerSvc.lastDelete)
	}
}

func TestHandleMetadata(t *testing.T) {
	srv := newTestServer()
	srv.metadataService = stubMetadataService{data: metadata.Metadata{
		Title:       "t",
		Description: "d",
		Handle:      "@h",
		ChannelID:   "cid",
		Languages:   []string{"English", "es"},
	}}

	req := httptest.NewRequest(http.MethodPost, "/api/metadata", strings.NewReader(`{"url":"https://example.com"}`))
	rr := httptest.NewRecorder()

	srv.handleMetadata(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, `"description":"d"`) {
		t.Fatalf("expected metadata JSON, got %q", body)
	}
}

func TestHandleMetadataMethodNotAllowed(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/metadata", nil)
	rr := httptest.NewRecorder()

	srv.handleMetadata(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
	if allow := rr.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("expected Allow header, got %q", allow)
	}
}

func TestHandleSitemap(t *testing.T) {
	srv := newTestServer()
	srv.streamersStore = &stubStreamersStore{
		records: []streamers.Record{
			{
				Streamer:  streamers.Streamer{ID: "alpha"},
				UpdatedAt: time.Date(2024, time.March, 1, 12, 30, 0, 0, time.UTC),
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	req.Host = "sharpen.live"
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()

	srv.handleSitemap(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "<loc>https://sharpen.live/</loc>") {
		t.Fatalf("expected home loc, got %q", body)
	}
	if !strings.Contains(body, "<loc>https://sharpen.live/streamers/alpha</loc>") {
		t.Fatalf("expected streamer loc, got %q", body)
	}
	if !strings.Contains(body, "2024-03-01") {
		t.Fatalf("expected lastmod timestamp, got %q", body)
	}
}

func TestHandleRobots(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	req.Host = "sharpen.live"
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()

	srv.handleRobots(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Sitemap: https://sharpen.live/sitemap.xml") {
		t.Fatalf("expected sitemap link, got %q", body)
	}
	if !strings.Contains(body, "Allow: /") {
		t.Fatalf("expected allow all agents, got %q", body)
	}
}

// TestLoadAdminLogs has been removed because logging functionality was removed from the codebase

func TestStreamersWatchAlias(t *testing.T) {
	path := filepath.Join(t.TempDir(), "streamers.json")
	handler := streamersWatchHandler(streamersWatchOptions{
		FilePath: path,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/streamers/watch", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", ct)
	}
	if body := strings.TrimSpace(rr.Body.String()); !strings.HasPrefix(body, "data:") {
		t.Fatalf("expected initial SSE payload, got %q", body)
	}
}

// helpers and stubs

func newTestServer() *server {
	templates := map[string]*template.Template{
		"home":     template.Must(template.New("home").Parse("Sharpen.Live {{len .Streamers}}")),
		"streamer": template.Must(template.New("streamer").Parse("streamer")),
		"admin":    template.Must(template.New("admin").Parse("admin {{.AdminEmail}} LoggedIn:{{.LoggedIn}}")),
	}
	// Create a test logger that discards output
	logger := logging.New("test", logging.INFO, io.Discard)
	return &server{
		assetsDir:        ".",
		stylesPath:       "/styles.css",
		templates:        templates,
		currentYear:      2024,
		submitEndpoint:   "/submit",
		streamersStore:   &stubStreamersStore{},
		streamerService:  &stubStreamerService{},
		submissionsStore: submissions.NewStore(""),
		adminSubmissions: &stubAdminSubmissions{},
		statusChecker:    &stubStatusChecker{},
		adminManager:     &stubAdminManager{valid: true},
		adminEmail:       "admin@example.com",
		metadataService:  stubMetadataService{},
		metadataFetcher:  stubMetadataFetcher{},
		socialImagePath:  "/og-image.png",
		siteName:         "Sharpen.Live",
		primaryHost:      "example.com",
		logger:           logger,
	}
}

type stubStreamersStore struct {
	records []streamers.Record
	err     error
	path    string
}

func (s *stubStreamersStore) List() ([]streamers.Record, error) {
	return s.records, s.err
}

func (s *stubStreamersStore) Path() string {
	if s.path != "" {
		return s.path
	}
	return "streamers.json"
}

type stubStreamerService struct {
	createResult streamersvc.CreateResult
	createErr    error
	created      bool
	lastUpdate   streamersvc.UpdateRequest
	lastDelete   streamersvc.DeleteRequest
}

func (s *stubStreamerService) Create(ctx context.Context, req streamersvc.CreateRequest) (streamersvc.CreateResult, error) {
	s.created = true
	if s.createErr != nil {
		return streamersvc.CreateResult{}, s.createErr
	}
	return s.createResult, nil
}

func (s *stubStreamerService) Update(ctx context.Context, req streamersvc.UpdateRequest) (streamers.Record, error) {
	s.lastUpdate = req
	return streamers.Record{}, nil
}

func (s *stubStreamerService) Delete(ctx context.Context, req streamersvc.DeleteRequest) error {
	s.lastDelete = req
	return nil
}

type stubAdminSubmissions struct {
	list       []submissions.Submission
	listErr    error
	processErr error
	lastReq    adminservice.ActionRequest
}

func (s *stubAdminSubmissions) List(context.Context) ([]submissions.Submission, error) {
	return s.list, s.listErr
}

func (s *stubAdminSubmissions) Process(ctx context.Context, req adminservice.ActionRequest) (adminservice.ActionResult, error) {
	s.lastReq = req
	if s.processErr != nil {
		return adminservice.ActionResult{}, s.processErr
	}
	return adminservice.ActionResult{Status: req.Action, Submission: submissions.Submission{ID: req.ID}}, nil
}

type stubAdminManager struct {
	token adminauth.Token
	err   error
	valid bool
}

func (s *stubAdminManager) Login(email, password string) (adminauth.Token, error) {
	return s.token, s.err
}

func (s *stubAdminManager) Validate(token string) bool {
	return s.valid && token == s.token.Value
}

type stubStatusChecker struct {
	result adminservice.StatusCheckResult
	err    error
}

func (s *stubStatusChecker) CheckAll(context.Context) (adminservice.StatusCheckResult, error) {
	return s.result, s.err
}

type stubMetadataFetcher struct {
	data youtubeservice.Metadata
	err  error
}

func (s stubMetadataFetcher) Fetch(context.Context, string) (youtubeservice.Metadata, error) {
	return s.data, s.err
}

type stubMetadataService struct {
	data metadata.Metadata
	err  error
}

func (s stubMetadataService) Fetch(context.Context, string) (*metadata.Metadata, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &s.data, nil
}
