package metadata

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// FacebookScraper scrapes metadata from Facebook pages.
type FacebookScraper struct {
	client *http.Client
}

// Matches returns true if the URL is a Facebook URL.
func (s *FacebookScraper) Matches(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "facebook.com") || strings.Contains(lower, "fb.com")
}

// Scrape fetches metadata from a Facebook page.
func (s *FacebookScraper) Collect(ctx context.Context, url string) (*Metadata, error) {
	// Create request with context and timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Facebook requires a realistic user agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Facebook URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(io.LimitReader(resp.Body, 2*1024*1024)) // 2MB limit
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	metadata := &Metadata{
		Platform: "facebook",
	}

	// Extract title from Open Graph
	if title, exists := doc.Find(`meta[property="og:title"]`).Attr("content"); exists {
		metadata.Title = strings.TrimSpace(title)
	}

	// Fallback to <title> tag
	if metadata.Title == "" {
		metadata.Title = strings.TrimSpace(doc.Find("title").Text())
		// Remove " | Facebook" suffix if present
		metadata.Title = strings.TrimSuffix(metadata.Title, " | Facebook")
	}

	// Extract description from Open Graph
	if desc, exists := doc.Find(`meta[property="og:description"]`).Attr("content"); exists {
		metadata.Description = strings.TrimSpace(desc)
	}

	// Fallback to meta description
	if metadata.Description == "" {
		if desc, exists := doc.Find(`meta[name="description"]`).Attr("content"); exists {
			metadata.Description = strings.TrimSpace(desc)
		}
	}

	// Extract handle from URL
	metadata.Handle = extractFacebookHandle(url)
	metadata.ChannelID = metadata.Handle // Facebook uses page name/ID as identifier

	metadata.Languages = []string{"English"}
	return metadata, nil
}

// extractFacebookHandle extracts the page name or ID from a Facebook URL.
func extractFacebookHandle(url string) string {
	// Match facebook.com/pagename pattern
	re := regexp.MustCompile(`facebook\.com/([a-zA-Z0-9._]+)`)
	if matches := re.FindStringSubmatch(url); len(matches) > 1 {
		handle := matches[1]
		// Skip common non-page paths
		if handle != "pages" && handle != "profile.php" && handle != "groups" {
			return handle
		}
	}

	// Match facebook.com/pages/name/ID pattern
	re = regexp.MustCompile(`facebook\.com/pages/[^/]+/(\d+)`)
	if matches := re.FindStringSubmatch(url); len(matches) > 1 {
		return matches[1]
	}

	return ""
}
