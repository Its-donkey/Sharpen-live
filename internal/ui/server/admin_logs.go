package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/logging"
)

// handleLogs displays the log viewer UI
func (s *server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	level := strings.ToUpper(r.URL.Query().Get("level"))
	category := r.URL.Query().Get("category")

	// Read recent logs
	logPath := filepath.Join(s.logDir, "app.log")
	entries, err := logging.ReadRecent(logPath, limit*2) // Read more to allow for filtering
	if err != nil {
		s.logger.Error("logs", "Failed to read logs", err, nil)
		http.Error(w, "Failed to read logs", http.StatusInternalServerError)
		return
	}

	// Filter entries
	filtered := make([]logging.Entry, 0, len(entries))
	for _, entry := range entries {
		if level != "" && entry.Level != level {
			continue
		}
		if category != "" && entry.Category != category {
			continue
		}
		filtered = append(filtered, entry)
	}

	// Take only the requested limit
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	// Get unique categories and levels for filters
	categories := make(map[string]bool)
	levels := make(map[string]bool)
	for _, entry := range entries {
		categories[entry.Category] = true
		levels[entry.Level] = true
	}

	data := logsPageData{
		basePageData: s.buildBasePageData(r, "Logs - "+s.siteDisplayName(), "View application logs", "/logs"),
		Entries:      filtered,
		Limit:        limit,
		Level:        level,
		Category:     category,
		Categories:   mapKeys(categories),
		Levels:       mapKeys(levels),
	}

	tmpl := s.templates["logs"]
	if tmpl == nil {
		// Render inline HTML if template not found
		s.renderLogsHTML(w, data)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "logs", data); err != nil {
		s.logger.Error("logs", "Failed to render logs template", err, nil)
		s.renderLogsHTML(w, data)
	}
}

// handleLogsStream provides a real-time SSE stream of log entries
func (s *server) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Parse filters
	level := strings.ToUpper(r.URL.Query().Get("level"))
	category := r.URL.Query().Get("category")

	// Create channel for log entries
	entryCh := make(chan logging.Entry, 100)
	unsubscribe := s.logger.Subscribe(entryCh)
	defer unsubscribe()

	// Send initial connection message
	fmt.Fprintf(w, "data: {\"type\":\"connected\",\"timestamp\":\"%s\"}\n\n", time.Now().UTC().Format(time.RFC3339))
	flusher.Flush()

	// Stream log entries
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			// Send keepalive
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case entry := <-entryCh:
			// Filter by level and category
			if level != "" && entry.Level != level {
				continue
			}
			if category != "" && entry.Category != category {
				continue
			}

			// Send entry as JSON
			data, err := json.Marshal(entry)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

type logsPageData struct {
	basePageData
	Entries    []logging.Entry
	Limit      int
	Level      string
	Category   string
	Categories []string
	Levels     []string
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// renderLogsHTML renders a simple inline HTML log viewer
func (s *server) renderLogsHTML(w http.ResponseWriter, data logsPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Application Logs</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0a0a0a; color: #e0e0e0; }
        .container { max-width: 1400px; margin: 0 auto; padding: 20px; }
        h1 { font-size: 24px; margin-bottom: 20px; color: #fff; }
        .controls { display: flex; gap: 15px; margin-bottom: 20px; flex-wrap: wrap; }
        .controls select, .controls input { padding: 8px 12px; background: #1a1a1a; border: 1px solid #333; color: #e0e0e0; border-radius: 4px; }
        .controls button { padding: 8px 16px; background: #2563eb; color: white; border: none; border-radius: 4px; cursor: pointer; }
        .controls button:hover { background: #1d4ed8; }
        .stats { display: flex; gap: 20px; margin-bottom: 20px; }
        .stat { background: #1a1a1a; padding: 12px 20px; border-radius: 6px; border-left: 3px solid #2563eb; }
        .stat-label { font-size: 12px; color: #888; margin-bottom: 4px; }
        .stat-value { font-size: 20px; font-weight: bold; }
        .logs { background: #0f0f0f; border: 1px solid #222; border-radius: 8px; overflow: hidden; }
        .log-entry { padding: 12px 16px; border-bottom: 1px solid #1a1a1a; font-family: "SF Mono", Monaco, monospace; font-size: 13px; }
        .log-entry:hover { background: #1a1a1a; }
        .log-header { display: flex; gap: 15px; margin-bottom: 6px; align-items: center; }
        .log-time { color: #666; }
        .log-level { padding: 2px 8px; border-radius: 3px; font-size: 11px; font-weight: 600; }
        .level-DEBUG { background: #374151; color: #9ca3af; }
        .level-INFO { background: #1e40af; color: #93c5fd; }
        .level-WARN { background: #b45309; color: #fcd34d; }
        .level-ERROR { background: #991b1b; color: #fca5a5; }
        .level-FATAL { background: #7f1d1d; color: #fca5a5; }
        .log-category { color: #60a5fa; }
        .log-request-id { color: #a78bfa; font-size: 11px; }
        .log-message { color: #e0e0e0; margin-bottom: 4px; }
        .log-fields { color: #9ca3af; font-size: 12px; }
        .log-error { color: #f87171; margin-top: 4px; }
        .log-duration { color: #34d399; font-size: 11px; }
        .toggle-fields { cursor: pointer; color: #60a5fa; font-size: 11px; margin-top: 4px; user-select: none; }
        .fields-detail { margin-top: 8px; padding: 10px; background: #1a1a1a; border-radius: 4px; display: none; }
        .fields-detail.show { display: block; }
        .fields-detail pre { color: #9ca3af; overflow-x: auto; }
        .stream-indicator { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-left: 10px; }
        .stream-indicator.connected { background: #10b981; }
        .stream-indicator.disconnected { background: #ef4444; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Application Logs <span class="stream-indicator" id="stream-status"></span></h1>

        <div class="controls">
            <select id="level" onchange="updateFilters()">
                <option value="">All Levels</option>`)

	for _, lvl := range data.Levels {
		selected := ""
		if lvl == data.Level {
			selected = " selected"
		}
		fmt.Fprintf(w, `<option value="%s"%s>%s</option>`, lvl, selected, lvl)
	}

	fmt.Fprint(w, `</select>
            <select id="category" onchange="updateFilters()">
                <option value="">All Categories</option>`)

	for _, cat := range data.Categories {
		selected := ""
		if cat == data.Category {
			selected = " selected"
		}
		fmt.Fprintf(w, `<option value="%s"%s>%s</option>`, cat, selected, cat)
	}

	fmt.Fprint(w, `</select>
            <input type="number" id="limit" value="100" min="10" max="1000" step="10" onchange="updateFilters()">
            <button onclick="updateFilters()">Apply Filters</button>
            <button onclick="toggleStream()">Toggle Live Stream</button>
            <button onclick="clearLogs()">Clear Display</button>
        </div>

        <div class="stats">
            <div class="stat">
                <div class="stat-label">Total Entries</div>
                <div class="stat-value" id="entry-count">`)
	fmt.Fprintf(w, "%d", len(data.Entries))
	fmt.Fprint(w, `</div>
            </div>
            <div class="stat">
                <div class="stat-label">Displaying</div>
                <div class="stat-value" id="display-count">`)
	fmt.Fprintf(w, "%d", len(data.Entries))
	fmt.Fprint(w, `</div>
            </div>
        </div>

        <div class="logs" id="logs-container">`)

	for _, entry := range data.Entries {
		renderLogEntry(w, entry)
	}

	fmt.Fprint(w, `</div>
    </div>

    <script>
        let eventSource = null;
        let isStreaming = false;
        const logsContainer = document.getElementById('logs-container');
        const streamStatus = document.getElementById('stream-status');
        const displayCount = document.getElementById('display-count');

        function updateFilters() {
            const level = document.getElementById('level').value;
            const category = document.getElementById('category').value;
            const limit = document.getElementById('limit').value;
            const params = new URLSearchParams();
            if (level) params.append('level', level);
            if (category) params.append('category', category);
            if (limit) params.append('limit', limit);
            window.location.href = '/logs?' + params.toString();
        }

        function toggleStream() {
            if (isStreaming) {
                stopStream();
            } else {
                startStream();
            }
        }

        function startStream() {
            const level = document.getElementById('level').value;
            const category = document.getElementById('category').value;
            const params = new URLSearchParams();
            if (level) params.append('level', level);
            if (category) params.append('category', category);

            eventSource = new EventSource('/logs/stream?' + params.toString());

            eventSource.onopen = () => {
                isStreaming = true;
                streamStatus.classList.add('connected');
                streamStatus.classList.remove('disconnected');
            };

            eventSource.onerror = () => {
                isStreaming = false;
                streamStatus.classList.remove('connected');
                streamStatus.classList.add('disconnected');
                eventSource.close();
                eventSource = null;
            };

            eventSource.onmessage = (event) => {
                try {
                    const entry = JSON.parse(event.data);
                    if (entry.type === 'connected') return;
                    appendLogEntry(entry);
                } catch (e) {
                    console.error('Failed to parse log entry:', e);
                }
            };
        }

        function stopStream() {
            if (eventSource) {
                eventSource.close();
                eventSource = null;
            }
            isStreaming = false;
            streamStatus.classList.remove('connected');
            streamStatus.classList.add('disconnected');
        }

        function appendLogEntry(entry) {
            const div = document.createElement('div');
            div.className = 'log-entry';
            div.innerHTML = formatLogEntry(entry);
            logsContainer.insertBefore(div, logsContainer.firstChild);

            // Keep only last 100 entries
            while (logsContainer.children.length > 100) {
                logsContainer.removeChild(logsContainer.lastChild);
            }

            updateDisplayCount();
        }

        function formatLogEntry(entry) {
            let html = '<div class="log-header">';
            html += '<span class="log-time">' + new Date(entry.timestamp).toLocaleString() + '</span>';
            html += '<span class="log-level level-' + entry.level + '">' + entry.level + '</span>';
            html += '<span class="log-category">' + entry.category + '</span>';
            if (entry.request_id) {
                html += '<span class="log-request-id">' + entry.request_id.substring(0, 8) + '</span>';
            }
            if (entry.duration_ms) {
                html += '<span class="log-duration">' + entry.duration_ms + 'ms</span>';
            }
            html += '</div>';
            html += '<div class="log-message">' + escapeHtml(entry.message) + '</div>';
            if (entry.error) {
                html += '<div class="log-error">Error: ' + escapeHtml(entry.error) + '</div>';
            }
            if (entry.fields && Object.keys(entry.fields).length > 0) {
                const id = 'fields-' + Math.random().toString(36).substring(7);
                html += '<div class="toggle-fields" onclick="toggleFields(\'' + id + '\')">⊕ Show Details</div>';
                html += '<div class="fields-detail" id="' + id + '"><pre>' + JSON.stringify(entry.fields, null, 2) + '</pre></div>';
            }
            return html;
        }

        function toggleFields(id) {
            const el = document.getElementById(id);
            el.classList.toggle('show');
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        function clearLogs() {
            logsContainer.innerHTML = '';
            updateDisplayCount();
        }

        function updateDisplayCount() {
            displayCount.textContent = logsContainer.children.length;
        }

        // Auto-start stream on page load
        setTimeout(startStream, 100);
    </script>
</body>
</html>`)
}

func renderLogEntry(w http.ResponseWriter, entry logging.Entry) {
	fmt.Fprint(w, `<div class="log-entry"><div class="log-header">`)
	fmt.Fprintf(w, `<span class="log-time">%s</span>`, entry.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, `<span class="log-level level-%s">%s</span>`, entry.Level, entry.Level)
	fmt.Fprintf(w, `<span class="log-category">%s</span>`, entry.Category)
	if entry.RequestID != "" {
		fmt.Fprintf(w, `<span class="log-request-id">%s</span>`, entry.RequestID[:min(8, len(entry.RequestID))])
	}
	if entry.Duration != nil {
		fmt.Fprintf(w, `<span class="log-duration">%dms</span>`, *entry.Duration)
	}
	fmt.Fprint(w, `</div>`)
	fmt.Fprintf(w, `<div class="log-message">%s</div>`, htmlEscape(entry.Message))
	if entry.Error != "" {
		fmt.Fprintf(w, `<div class="log-error">Error: %s</div>`, htmlEscape(entry.Error))
	}
	if entry.Fields != nil && len(entry.Fields) > 0 {
		id := fmt.Sprintf("fields-%d", time.Now().UnixNano())
		fmt.Fprintf(w, `<div class="toggle-fields" onclick="toggleFields('%s')">⊕ Show Details</div>`, id)
		fieldsJSON, _ := json.MarshalIndent(entry.Fields, "", "  ")
		fmt.Fprintf(w, `<div class="fields-detail" id="%s"><pre>%s</pre></div>`, id, htmlEscape(string(fieldsJSON)))
	}
	fmt.Fprint(w, `</div>`)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
