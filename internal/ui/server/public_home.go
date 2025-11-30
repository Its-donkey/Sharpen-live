package server

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
	"github.com/Its-donkey/Sharpen-live/internal/ui/forms"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	youtubeui "github.com/Its-donkey/Sharpen-live/internal/ui/platforms/youtube"
)

func (s *server) homeStructuredData(homeURL string) template.JS {
	if strings.TrimSpace(homeURL) == "" {
		return ""
	}
	org := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Organization",
		"name":        s.siteDisplayName(),
		"url":         homeURL,
		"description": s.defaultDescription(),
	}
	payload, err := json.Marshal(org)
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

func (s *server) siteDisplayName() string {
	if name := strings.TrimSpace(s.siteName); name != "" {
		return name
	}
	return "Sharpen.Live"
}

func (s *server) homePageTitle() string {
	name := s.siteDisplayName()
	switch {
	case strings.EqualFold(s.siteKey, config.CatchAllSiteKey) || strings.EqualFold(name, config.CatchAllSiteKey):
		return "Site unavailable - review configuration"
	case strings.EqualFold(s.siteKey, "synth-wave") || strings.EqualFold(name, "synth.wave"):
		return name + " - Live synthwave streams"
	default:
		return name + " - Live knife sharpening streams"
	}
}

func (s *server) submitPageTitle() string {
	name := s.siteDisplayName()
	if strings.EqualFold(s.siteKey, config.CatchAllSiteKey) || strings.EqualFold(name, config.CatchAllSiteKey) {
		return "Submit a streamer"
	}
	return "Submit a streamer - " + name
}

func (s *server) streamerPageTitle(streamerName string) string {
	name := s.siteDisplayName()
	if strings.TrimSpace(streamerName) == "" {
		return name
	}
	return strings.TrimSpace(streamerName) + " - " + name
}

func (s *server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	state, rosterErr := s.fetchRoster(ctx)

	submit := defaultSubmitState(r)
	page := s.buildBasePageData(r, s.homePageTitle(), s.siteDescription, "/")
	s.renderHomeWithRoster(w, r, page, state, rosterErr, submit)
}

func (s *server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	title := s.submitPageTitle()
	switch r.Method {
	case http.MethodGet:
		state := defaultSubmitState(r)
		page := s.buildBasePageData(r, title, s.siteDescription, "/submit")
		s.renderHome(w, r, page, state)
	case http.MethodPost:
		state, removedRows, err := parseSubmitForm(r)
		if err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		for _, rowID := range removedRows {
			state.Platforms = removePlatformRow(state.Platforms, rowID)
		}

		state.Errors = forms.ValidateSubmitForm(&state)
		if hasSubmitErrors(state.Errors) {
			ensureSubmitDefaults(&state)
			w.WriteHeader(http.StatusUnprocessableEntity)
			page := s.buildBasePageData(r, title, s.siteDescription, "/submit")
			s.renderHome(w, r, page, state)
			return
		}

		youtubeui.MaybeEnrichMetadata(ctx, &state, http.DefaultClient)
		if _, err := submitStreamer(ctx, s.streamerService, state); err != nil {
			state.Errors.General = append(state.Errors.General, "failed to submit streamer, please try again")
			ensureSubmitDefaults(&state)
			page := s.buildBasePageData(r, title, s.siteDescription, "/submit")
			s.renderHome(w, r, page, state)
			return
		}

		http.Redirect(w, r, "/?submitted=1", http.StatusSeeOther)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	target := strings.TrimSpace(payload.URL)
	u, err := url.Parse(target)
	if err != nil || !u.IsAbs() {
		http.Error(w, "invalid url", http.StatusBadRequest)
		return
	}

	if s.metadataFetcher == nil {
		http.Error(w, "metadata service unavailable", http.StatusServiceUnavailable)
		return
	}

	meta, err := s.metadataFetcher.Fetch(ctx, u.String())
	if err != nil {
		http.Error(w, "failed to fetch metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]string{
		"description": meta.Description,
		"title":       meta.Title,
		"handle":      meta.Handle,
		"channelId":   meta.ChannelID,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *server) renderHome(w http.ResponseWriter, r *http.Request, page basePageData, submit model.SubmitFormState) {
	state, rosterErr := s.fetchRoster(r.Context())
	s.renderHomeWithRoster(w, r, page, state, rosterErr, submit)
}

func (s *server) renderHomeWithRoster(w http.ResponseWriter, r *http.Request, page basePageData, state []model.Streamer, rosterErr string, submit model.SubmitFormState) {
	page.StructuredData = s.homeStructuredData(s.absoluteURL(r, "/"))
	submitView := submitFormView{
		State:           submit,
		LanguageOptions: forms.AvailableLanguageOptions(submit.Languages),
		FormAction:      "",
		MaxPlatforms:    model.MaxPlatforms,
	}
	data := struct {
		basePageData
		Roster      []model.Streamer
		Streamers   []model.Streamer
		RosterError string
		Submit      submitFormView
	}{
		basePageData: page,
		Roster:       state,
		Streamers:    state,
		RosterError:  rosterErr,
		Submit:       submitView,
	}

	tmpl := s.templates["home"]
	if tmpl == nil {
		http.Error(w, "template missing", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "home", data); err != nil {
		http.Error(w, "failed to render page", http.StatusInternalServerError)
	}
}

func (s *server) fetchRoster(ctx context.Context) ([]model.Streamer, string) {
	if s.streamersStore == nil {
		return nil, "streamers store unavailable"
	}
	records, err := s.streamersStore.List()
	if err != nil {
		return nil, "failed to load roster"
	}
	return mapStreamerRecords(records), ""
}
