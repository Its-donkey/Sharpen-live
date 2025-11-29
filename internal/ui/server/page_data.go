// file name — /internal/ui/server/page_data.go - NEW
package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// canonicalHostFromURL extracts the host component from a URL string.
// It returns an empty string if parsing fails.
func canonicalHostFromURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Host)
}

// absoluteURL builds an absolute URL for the given path using the request
// and server configuration as hints.
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

// socialImageURL constructs the absolute URL for the configured social image.
func (s *server) socialImageURL(r *http.Request) string {
	if strings.TrimSpace(s.socialImagePath) == "" {
		return ""
	}
	return s.absoluteURL(r, s.socialImagePath)
}

// defaultDescription returns the site description configured for this server.
func (s *server) defaultDescription() string {
	return s.siteDescription
}

// truncateWithEllipsis trims a string to a maximum number of runes,
// attempting to cut on a word boundary, and appends an ellipsis if trimmed.
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
	for i := limit - 1; i >= 0 && i >= limit-20; i-- {
		if runes[i] == ' ' {
			cut = i
			break
		}
	}

	trimmed := strings.TrimSpace(string(runes[:cut]))
	if trimmed == "" {
		trimmed = strings.TrimSpace(string(runes[:limit]))
	}

	return trimmed + "…"
}

// normalizeDescription normalises a meta description string, falling back to
// the default site description and truncating to a search-friendly length.
func (s *server) normalizeDescription(desc string) string {
	trimmed := strings.TrimSpace(desc)
	if trimmed == "" {
		trimmed = s.defaultDescription()
	}
	return truncateWithEllipsis(trimmed, 155)
}

// buildBasePageData constructs the basePageData used by most templates.
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
		Robots:          "",
		FallbackErrors:  s.fallbackErrors,
	}
}
