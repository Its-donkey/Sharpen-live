package logging

import (
	"io"
	"testing"
)

func TestParseYouTubeWebSub(t *testing.T) {
	// Sample YouTube WebSub Atom feed (based on real format)
	xmlBody := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:yt="http://www.youtube.com/xml/schemas/2015">
  <entry>
    <yt:videoId>dQw4w9WgXcQ</yt:videoId>
    <yt:channelId>UCuAXFkgsw1L7xaCfnd5JJOw</yt:channelId>
    <title>Test Video Title</title>
    <link rel="alternate" href="https://www.youtube.com/watch?v=dQw4w9WgXcQ"/>
    <author>
      <name>Test Channel</name>
      <uri>https://www.youtube.com/channel/UCuAXFkgsw1L7xaCfnd5JJOw</uri>
    </author>
    <published>2025-01-01T12:00:00+00:00</published>
    <updated>2025-01-01T12:30:00+00:00</updated>
  </entry>
</feed>`

	logger := New("test", DEBUG, io.Discard)

	// This should not panic or error
	logger.ParseYouTubeWebSub(xmlBody, "test-request-id")
}

func TestIsYouTubeWebSubNotification(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		want        bool
	}{
		{
			name:        "valid youtube websub",
			body:        `<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015"><yt:videoId>abc</yt:videoId></feed>`,
			contentType: "application/atom+xml",
			want:        true,
		},
		{
			name:        "valid youtube websub with xml content type",
			body:        `<feed>youtube.com<yt:videoId>abc</yt:videoId></feed>`,
			contentType: "application/xml",
			want:        true,
		},
		{
			name:        "missing youtube markers",
			body:        `<feed><entry>test</entry></feed>`,
			contentType: "application/atom+xml",
			want:        false,
		},
		{
			name:        "wrong content type",
			body:        `<feed>youtube.com<yt:videoId>abc</yt:videoId></feed>`,
			contentType: "application/json",
			want:        false,
		},
		{
			name:        "empty body",
			body:        "",
			contentType: "application/atom+xml",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsYouTubeWebSubNotification(tt.body, tt.contentType)
			if got != tt.want {
				t.Errorf("IsYouTubeWebSubNotification() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestYouTubeAtomParsing(t *testing.T) {
	xmlBody := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:yt="http://www.youtube.com/xml/schemas/2015">
  <entry>
    <yt:videoId>testVideoId123</yt:videoId>
    <yt:channelId>testChannelId456</yt:channelId>
    <title>My Test Video</title>
    <link rel="alternate" href="https://www.youtube.com/watch?v=testVideoId123"/>
    <author>
      <name>Test Channel Name</name>
      <uri>https://www.youtube.com/channel/testChannelId456</uri>
    </author>
    <published>2025-01-15T10:00:00+00:00</published>
    <updated>2025-01-15T10:30:00+00:00</updated>
  </entry>
</feed>`

	// Basic smoke test - if this doesn't panic, we're good
	logger := New("test", DEBUG, io.Discard)
	logger.ParseYouTubeWebSub(xmlBody, "test-id")
}
