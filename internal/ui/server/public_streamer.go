package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

func (s *server) streamerStructuredData(canonical string, streamer model.Streamer) template.JS {
	if strings.TrimSpace(canonical) == "" {
		return ""
	}
	if strings.TrimSpace(streamer.Name) == "" {
		return ""
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

	var sameAs []string
	for _, p := range streamer.Platforms {
		if u := strings.TrimSpace(p.ChannelURL); u != "" {
			sameAs = append(sameAs, u)
		}
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

func (s *server) handleStreamer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	alias := strings.TrimPrefix(r.URL.Path, "/streamers/")
	if alias == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	streamer := model.Streamer{}
	if s.streamersStore != nil {
		records, err := s.streamersStore.List()
		if err != nil {
			log.Printf("render streamer detail: %v", err)
			http.Error(w, "failed to load streamer", http.StatusInternalServerError)
			return
		}
		for _, rec := range mapStreamerRecords(records) {
			if strings.EqualFold(rec.Name, alias) || strings.EqualFold(rec.ID, alias) {
				streamer = rec
				break
			}
		}
	}
	if streamer.ID == "" && streamer.Name == "" {
		http.NotFound(w, r)
		return
	}

	page := s.buildBasePageData(r, fmt.Sprintf("%s – Sharpen.Live", streamer.Name), s.siteDescription, r.URL.Path)
	page.StructuredData = s.streamerStructuredData(s.absoluteURL(r, r.URL.Path), streamer)
	data := struct {
		basePageData
		Streamer model.Streamer
	}{
		basePageData: page,
		Streamer:     streamer,
	}

	tmpl := s.templates["streamer"]
	if tmpl == nil {
		http.Error(w, "template missing", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "streamer", data); err != nil {
		log.Printf("execute streamer template: %v", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
	}
}
