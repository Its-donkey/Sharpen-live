package server

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Its-donkey/Sharpen-live/backend/platforms/youtube/internal/alerts"
)

// AlertProcessor processes incoming stream alerts.
type AlertProcessor interface {
	Handle(ctx context.Context, alert alerts.StreamAlert) error
}

// Logger mirrors the log.Printf signature.
type Logger interface {
	Printf(format string, v ...any)
}

// Config configures a Server instance.
type Config struct {
	Processor AlertProcessor
	Logger    Logger
	Streamers StreamerDirectory
	QueueSize int
	Immediate bool
}

// Server exposes HTTP endpoints for alert delivery.
type Server struct {
	processor     AlertProcessor
	logger        Logger
	streamerNames StreamerDirectory
	queue         chan alerts.StreamAlert
	immediate     bool
}

const defaultQueueSize = 32
const youtubeTopicTemplate = "https://www.youtube.com/xml/feeds/videos.xml?channel_id=%s"

// New constructs a Server.
func New(cfg Config) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = httpLogger{}
	}

	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = defaultQueueSize
	}

	s := &Server{
		processor:     cfg.Processor,
		logger:        logger,
		streamerNames: cfg.Streamers,
		immediate:     cfg.Immediate,
	}

	if !s.immediate {
		s.queue = make(chan alerts.StreamAlert, queueSize)
		go s.drainQueue()
	}

	return s
}

func (s *Server) drainQueue() {
	for alert := range s.queue {
		s.processAlert(alert)
	}
}

func (s *Server) processAlert(alert alerts.StreamAlert) {
	if s.processor == nil {
		s.logger.Printf("alert dropped: processor unavailable channel=%s streamer=%q video=%s", alert.ChannelID, alert.StreamerName, alert.StreamID)
		return
	}

	if err := s.processor.Handle(context.Background(), alert); err != nil {
		if errors.Is(err, alerts.ErrMissingChannelID) {
			s.logger.Printf("alert rejected: missing channel id streamer=%q video=%s", alert.StreamerName, alert.StreamID)
			return
		}
		s.logger.Printf("alert processing failed: channel=%s streamer=%q video=%s err=%v", alert.ChannelID, alert.StreamerName, alert.StreamID, err)
		return
	}

	s.logger.Printf("alert processed: channel=%s streamer=%q video=%s", alert.ChannelID, alert.StreamerName, alert.StreamID)
}

func (s *Server) enqueueAlert(alert alerts.StreamAlert) {
	if s.immediate {
		s.processAlert(alert)
		return
	}

	s.queue <- alert
}

// Routes returns the HTTP handler for the server.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/alerts", s.handleAlerts())
	return mux
}

func (s *Server) handleAlerts() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && hasVerificationParams(r):
			s.handleVerification(w, r)
		case r.Method == http.MethodPost:
			s.handleNotification(w, r)
		case r.Method == http.MethodGet:
			http.Error(w, "missing verification parameters", http.StatusBadRequest)
		default:
			http.Error(w, "unsupported request", http.StatusBadRequest)
		}
	})
}

func (s *Server) handleVerification(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	challenge := strings.TrimSpace(query.Get("hub.challenge"))
	mode := strings.TrimSpace(query.Get("hub.mode"))
	lease := strings.TrimSpace(query.Get("hub.lease_seconds"))
	topic := strings.TrimSpace(query.Get("hub.topic"))
	verifyToken := strings.TrimSpace(query.Get("hub.verify_token"))
	userAgent := strings.TrimSpace(r.Header.Get("User-Agent"))
	requestID := requestIDFrom(r)

	if challenge == "" {
		http.Error(w, "missing hub.challenge", http.StatusBadRequest)
		return
	}

	s.logger.Printf(
		"verification received: request_id=%q user_agent=%q mode=%s topic=%s lease_seconds=%s verify_token=%s",
		requestID,
		userAgent,
		mode,
		topic,
		lease,
		verifyToken,
	)

	channelID, err := channelIDFromTopic(topic)
	if err != nil {
		s.logger.Printf("verification topic invalid: topic=%s err=%v", topic, err)
		http.Error(w, "invalid topic", http.StatusBadRequest)
		return
	}

	if name, ok := s.streamerNames.Name(channelID); ok {
		s.logger.Printf("verification channel matched roster: channel_id=%s streamer=%q", channelID, name)
	} else {
		s.logger.Printf("verification channel not yet registered: channel_id=%s (allowing challenge)", channelID)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(challenge))
	s.logger.Printf("verification response sent: request_id=%s challenge=%s", requestID, challenge)
}

func (s *Server) handleNotification(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		s.logger.Printf("notification read error: %v", err)
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	userAgent := strings.TrimSpace(r.Header.Get("User-Agent"))
	requestID := requestIDFrom(r)
	trimmedBody := strings.TrimSpace(string(body))
	bodyPreview := trimmedBody
	if len(bodyPreview) > 2048 {
		bodyPreview = bodyPreview[:2048] + "...(truncated)"
	}

	s.logger.Printf("notification raw body (request_id=%s):\n%s", requestID, string(body))

	if !isAtomPayload(contentType, body) {
		s.logger.Printf(
			"notification rejected: request_id=%q user_agent=%q content_type=%q body_preview=%q",
			requestID,
			userAgent,
			contentType,
			bodyPreview,
		)
		http.Error(w, "unsupported notification payload", http.StatusBadRequest)
		return
	}

	entries, err := parseAtomFeed(body)
	if err != nil {
		s.logger.Printf("notification atom parse error: request_id=%q err=%v", requestID, err)
		http.Error(w, "invalid atom payload", http.StatusBadRequest)
		return
	}

	if len(entries) == 0 {
		s.logger.Printf("notification contained no entries: request_id=%q", requestID)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	for _, entry := range entries {
		channelID := strings.TrimSpace(entry.ChannelID)
		videoID := strings.TrimSpace(entry.VideoID)
		if channelID == "" || videoID == "" {
			s.logger.Printf("notification entry skipped: request_id=%q channel=%q video=%q", requestID, channelID, videoID)
			continue
		}

		streamerName, known := s.streamerNames.Name(channelID)
		if s.streamerNames != nil {
			if !known {
				s.logger.Printf("notification entry channel unknown: request_id=%q channel=%s", requestID, channelID)
			}
		}

		link := entry.AlternateURL()
		s.logger.Printf(
			"notification queued: request_id=%q user_agent=%q channel=%s streamer=%q video=%s title=%q published=%s updated=%s link=%s",
			requestID,
			userAgent,
			channelID,
			streamerName,
			videoID,
			strings.TrimSpace(entry.Title),
			strings.TrimSpace(entry.Published),
			strings.TrimSpace(entry.Updated),
			link,
		)

		alert := alerts.StreamAlert{
			ChannelID:    channelID,
			StreamID:     videoID,
			Status:       "online",
			StreamerName: streamerName,
		}
		s.enqueueAlert(alert)
	}

	w.WriteHeader(http.StatusNoContent)
}

func hasVerificationParams(r *http.Request) bool {
	query := r.URL.Query()
	return strings.TrimSpace(query.Get("hub.mode")) != "" && strings.TrimSpace(query.Get("hub.challenge")) != ""
}

func isAtomPayload(contentType string, body []byte) bool {
	lower := strings.ToLower(contentType)
	if strings.Contains(lower, "application/atom+xml") {
		return true
	}
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "<?xml") {
		if idx := strings.Index(trimmed, "?>"); idx != -1 && idx+2 < len(trimmed) {
			trimmed = strings.TrimSpace(trimmed[idx+2:])
		}
	}
	return strings.HasPrefix(trimmed, "<feed")
}

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	VideoID   string     `xml:"{http://www.youtube.com/xml/schemas/2015}videoId"`
	ChannelID string     `xml:"{http://www.youtube.com/xml/schemas/2015}channelId"`
	Title     string     `xml:"title"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Links     []atomLink `xml:"link"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

func parseAtomFeed(body []byte) ([]atomEntry, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	var (
		entries []atomEntry
		current *atomEntry
	)

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "entry":
				current = &atomEntry{}
			case "videoId":
				if current != nil {
					var value string
					if err := decoder.DecodeElement(&value, &t); err != nil {
						return nil, err
					}
					current.VideoID = strings.TrimSpace(value)
				}
			case "channelId":
				if current != nil {
					var value string
					if err := decoder.DecodeElement(&value, &t); err != nil {
						return nil, err
					}
					current.ChannelID = strings.TrimSpace(value)
				}
			case "title":
				if current != nil {
					var value string
					if err := decoder.DecodeElement(&value, &t); err != nil {
						return nil, err
					}
					current.Title = strings.TrimSpace(value)
				}
			case "published":
				if current != nil {
					var value string
					if err := decoder.DecodeElement(&value, &t); err != nil {
						return nil, err
					}
					current.Published = strings.TrimSpace(value)
				}
			case "updated":
				if current != nil {
					var value string
					if err := decoder.DecodeElement(&value, &t); err != nil {
						return nil, err
					}
					current.Updated = strings.TrimSpace(value)
				}
			case "link":
				var rel, href string
				for _, attr := range t.Attr {
					switch attr.Name.Local {
					case "rel":
						rel = strings.TrimSpace(attr.Value)
					case "href":
						href = strings.TrimSpace(attr.Value)
					}
				}
				if current != nil && (rel != "" || href != "") {
					current.Links = append(current.Links, atomLink{Rel: rel, Href: href})
				}
				if err := decoder.Skip(); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if t.Name.Local == "entry" && current != nil {
				entries = append(entries, *current)
				current = nil
			}
		}
	}

	return entries, nil
}

func (e atomEntry) AlternateURL() string {
	for _, link := range e.Links {
		if strings.EqualFold(strings.TrimSpace(link.Rel), "alternate") {
			return strings.TrimSpace(link.Href)
		}
	}
	return ""
}

func requestIDFrom(r *http.Request) string {
	headers := []string{"X-Request-Id", "X-Goog-Request-Id", "X-Cloud-Trace-Context"}
	for _, key := range headers {
		if value := strings.TrimSpace(r.Header.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func channelIDFromTopic(topic string) (string, error) {
	trimmed := strings.TrimSpace(topic)
	if trimmed == "" {
		return "", errors.New("empty topic")
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}

	if id := strings.TrimSpace(u.Query().Get("channel_id")); id != "" {
		return id, nil
	}

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(segments); i++ {
		segment := strings.TrimSpace(segments[i])
		if !strings.EqualFold(segment, "channel") {
			continue
		}
		if i+1 < len(segments) {
			candidate := strings.TrimSpace(segments[i+1])
			if candidate != "" {
				return candidate, nil
			}
		}
	}

	return "", errors.New("channel id not found in topic")
}

type httpLogger struct{}

func (httpLogger) Printf(string, ...any) {}
