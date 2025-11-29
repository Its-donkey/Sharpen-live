package server

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
)

type urlSet struct {
	XMLName xml.Name   `xml:"urlset"`
	Xmlns   string     `xml:"xmlns,attr"`
	URLs    []urlEntry `xml:"url"`
}

type urlEntry struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

func (s *server) handleRobots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	fmt.Fprintln(w, "User-agent: *")
	fmt.Fprintln(w, "Allow: /")
	fmt.Fprintf(w, "Sitemap: %s\n", s.absoluteURL(r, "/sitemap.xml"))
}

func (s *server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	homeURL := s.absoluteURL(r, "/")
	entries := []urlEntry{{
		Loc:        homeURL,
		ChangeFreq: "daily",
		Priority:   "1.0",
	}}

	var records []streamers.Record
	if s.streamersStore != nil {
		var err error
		records, err = s.streamersStore.List()
		if err != nil {
			log.Printf("render sitemap: %v", err)
		}
	}

	for _, rec := range records {
		alias := strings.TrimSpace(rec.Streamer.Alias)
		if alias == "" {
			alias = rec.Streamer.ID
		}
		path := fmt.Sprintf("/streamers/%s", alias)
		lastMod := rec.UpdatedAt.Format("2006-01-02")
		entries = append(entries, urlEntry{
			Loc:        s.absoluteURL(r, path),
			LastMod:    lastMod,
			ChangeFreq: "weekly",
			Priority:   "0.8",
		})
	}

	smap := urlSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  entries,
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(smap); err != nil {
		log.Printf("encode sitemap: %v", err)
	}
}
