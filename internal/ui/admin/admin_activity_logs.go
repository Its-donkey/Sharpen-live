//go:build js && wasm

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
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
	entry := model.AdminActivityLog{Raw: payload}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(payload), &parsed); err == nil {
		if ts, ok := parsed["time"].(string); ok && ts != "" {
			entry.Timestamp = ts
		} else if ts, ok := parsed["datetime"].(string); ok && ts != "" {
			entry.Timestamp = ts
		}
		if msg, ok := parsed["message"].(string); ok && msg != "" {
			entry.Message = msg
		}
	}
	if entry.Message == "" {
		entry.Message = payload
	}
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	state.AdminConsole.ActivityLogs = append(state.AdminConsole.ActivityLogs, entry)
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
		feed.Set("scrollTop", feed.Get("scrollHeight"))
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
