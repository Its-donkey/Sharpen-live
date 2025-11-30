package logging

import (
	"encoding/xml"
	"strings"
	"time"
)

// YouTubeAtomFeed represents the Atom XML feed from YouTube WebSub notifications.
type YouTubeAtomFeed struct {
	XMLName xml.Name       `xml:"feed"`
	Entries []YouTubeEntry `xml:"entry"`
}

// YouTubeEntry represents a single video entry in the Atom feed.
type YouTubeEntry struct {
	VideoID     string    `xml:"videoId"`
	ChannelID   string    `xml:"channelId"`
	Title       string    `xml:"title"`
	Link        Link      `xml:"link"`
	Author      Author    `xml:"author"`
	Published   time.Time `xml:"published"`
	Updated     time.Time `xml:"updated"`
}

// Link represents the link element in Atom XML.
type Link struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

// Author represents the author element in Atom XML.
type Author struct {
	Name string `xml:"name"`
	URI  string `xml:"uri"`
}

// ParseYouTubeWebSub parses YouTube WebSub Atom XML and logs structured events.
func (l *Logger) ParseYouTubeWebSub(xmlBody string, requestID string) {
	if xmlBody == "" {
		return
	}

	var feed YouTubeAtomFeed
	if err := xml.Unmarshal([]byte(xmlBody), &feed); err != nil {
		l.Warn("youtube", "failed to parse websub XML", map[string]any{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Log each video entry as a separate structured event
	for _, entry := range feed.Entries {
		fields := map[string]any{
			"video_id":      entry.VideoID,
			"channel_id":    entry.ChannelID,
			"video_title":   entry.Title,
			"channel_name":  entry.Author.Name,
			"channel_url":   entry.Author.URI,
			"video_url":     entry.Link.Href,
			"published_at":  entry.Published.Format(time.RFC3339),
			"updated_at":    entry.Updated.Format(time.RFC3339),
			"request_id":    requestID,
		}

		l.Info("youtube", "video notification received", fields)
	}
}

// IsYouTubeWebSubNotification checks if the request body looks like YouTube WebSub XML.
func IsYouTubeWebSubNotification(body string, contentType string) bool {
	if body == "" {
		return false
	}

	// Check content type
	if !strings.Contains(strings.ToLower(contentType), "atom") &&
		!strings.Contains(strings.ToLower(contentType), "xml") {
		return false
	}

	// Check for YouTube-specific XML markers
	return strings.Contains(body, "<feed") &&
		strings.Contains(body, "youtube.com") &&
		(strings.Contains(body, "<yt:videoId>") || strings.Contains(body, "videoId"))
}
