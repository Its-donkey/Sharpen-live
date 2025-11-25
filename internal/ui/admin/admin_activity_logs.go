//go:build js && wasm

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"syscall/js"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"github.com/Its-donkey/Sharpen-live/internal/ui/state"
)

const (
	maxActivityLogEntries     = 300
	logStreamAuthCheckTimeout = 5 * time.Second
)

var (
	websiteLogSource   js.Value
	websiteLogHandlers []js.Func
	logTimestampRegex  = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})`)
)

func ensureWebsiteLogStream() {
	if websiteLogSource.Truthy() {
		return
	}
	token := strings.TrimSpace(state.AdminConsole.Token)
	if token == "" {
		state.AdminConsole.ActivityLogsError = "Log in to view website logs."
		scheduleAdminRender()
		return
	}
	eventSource := js.Global().Get("EventSource")
	if !eventSource.Truthy() {
		state.AdminConsole.ActivityLogsError = "EventSource unsupported by this browser."
		scheduleAdminRender()
		return
	}
	streamURL := "/admin/logs?token=" + url.QueryEscape(token)
	source := eventSource.New(streamURL)
	messageHandler := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		data := args[0].Get("data").String()
		handleWebsiteLogMessage(data)
		return nil
	})
	errorHandler := js.FuncOf(func(this js.Value, args []js.Value) any {
		state.AdminConsole.ActivityLogsError = "Log stream interrupted. Click Reconnect."
		go verifyLogStreamAuthorization()
		scheduleAdminRender()
		return nil
	})
	source.Call("addEventListener", "message", messageHandler)
	source.Call("addEventListener", "error", errorHandler)
	websiteLogSource = source
	websiteLogHandlers = []js.Func{messageHandler, errorHandler}
	state.AdminConsole.ActivityLogsError = ""
}

func reconnectWebsiteLogStream() {
	closeWebsiteLogStream()
	ensureWebsiteLogStream()
}

func closeWebsiteLogStream() {
	if !websiteLogSource.Truthy() {
		return
	}
	websiteLogSource.Call("close")
	websiteLogSource = js.Value{}
	for _, fn := range websiteLogHandlers {
		fn.Release()
	}
	websiteLogHandlers = nil
}

func handleWebsiteLogMessage(payload string) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return
	}
	entries := parseWebsiteLogEntries(payload)
	if len(entries) == 0 {
		return
	}
	state.AdminConsole.ActivityLogs = append(state.AdminConsole.ActivityLogs, entries...)
	if len(state.AdminConsole.ActivityLogs) > maxActivityLogEntries {
		state.AdminConsole.ActivityLogs = state.AdminConsole.ActivityLogs[len(state.AdminConsole.ActivityLogs)-maxActivityLogEntries:]
	}
	state.AdminConsole.ActivityLogsShouldScroll = true
	state.AdminConsole.ActivityLogsError = ""
	scheduleAdminRender()
}

func handleAdminLogsClear() {
	state.AdminConsole.ActivityLogs = nil
	scheduleAdminRender()
}

func handleAdminLogsReconnect() {
	reconnectWebsiteLogStream()
}

func scrollWebsiteLogFeed() {
	feed := getDocument().Call("getElementById", "admin-log-feed")
	if feed.Truthy() {
		feed.Set("scrollTop", 0)
	}
}

func verifyLogStreamAuthorization() {
	if strings.TrimSpace(state.AdminConsole.Token) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), logStreamAuthCheckTimeout)
	defer cancel()
	_, status, err := adminAPIRequest(ctx, http.MethodGet, "/api/admin/submissions", nil, true)
	if err != nil && status != http.StatusUnauthorized {
		return
	}
	if status == http.StatusUnauthorized {
		handleAdminUnauthorizedResponse()
	}
}

func parseTimestampFromLog(payload string) string {
	matches := logTimestampRegex.FindStringSubmatch(payload)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func parseWebsiteLogEntries(payload string) []model.AdminActivityLog {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(payload), &parsed); err == nil {
		if entries := extractLogEvents(parsed, payload); len(entries) > 0 {
			return entries
		}
		return []model.AdminActivityLog{buildAdminLogEntry(parsed, payload)}
	}
	return []model.AdminActivityLog{buildAdminLogEntry(nil, payload)}
}

func extractLogEvents(parsed map[string]any, fallbackRaw string) []model.AdminActivityLog {
	rawEvents, ok := parsed["logevents"]
	if !ok {
		return nil
	}
	events, ok := rawEvents.([]any)
	if !ok || len(events) == 0 {
		return nil
	}
	entries := make([]model.AdminActivityLog, 0, len(events))
	for _, rawEvent := range events {
		var (
			eventMap map[string]any
			eventRaw = fallbackRaw
		)
		switch ev := rawEvent.(type) {
		case map[string]any:
			eventMap = ev
			if marshalled, err := json.Marshal(ev); err == nil {
				eventRaw = string(marshalled)
			}
		case string:
			eventRaw = ev
			if strings.TrimSpace(ev) != "" {
				var parsedEvent map[string]any
				if err := json.Unmarshal([]byte(ev), &parsedEvent); err == nil {
					eventMap = parsedEvent
				}
			}
		}
		entries = append(entries, buildAdminLogEntry(eventMap, eventRaw))
	}
	return entries
}

func buildAdminLogEntry(parsed map[string]any, raw string) model.AdminActivityLog {
	entry := model.AdminActivityLog{Raw: raw}
	if parsed != nil {
		if ts, ok := parsed["time"].(string); ok && ts != "" {
			entry.Time = ts
		} else if ts, ok := parsed["datetime"].(string); ok && ts != "" {
			entry.Time = ts
		}
		if msg, ok := parsed["message"].(string); ok && msg != "" {
			entry.Message = msg
		} else if rawMessage, ok := parsed["raw"].(map[string]any); ok {
			if pretty, err := json.MarshalIndent(rawMessage, "", "  "); err == nil {
				entry.Message = string(pretty)
			}
		}
	}
	if entry.Message == "" {
		entry.Message = raw
	}
	if formatted := formatLogRawJSON(entry.Raw); formatted != "" {
		entry.Raw = formatted
		if entry.Message == raw || entry.Message == entry.Raw {
			entry.Message = formatted
		}
	}
	if entry.Time == "" {
		entry.Time = parseTimestampFromLog(raw)
	}
	if entry.Time == "" {
		entry.Time = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return entry
}

func formatLogRawJSON(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}
	pretty, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ""
	}
	return string(pretty)
}
