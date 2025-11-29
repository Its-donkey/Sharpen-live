package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	adminservice "github.com/Its-donkey/Sharpen-live/internal/alert/admin/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	streamersvc "github.com/Its-donkey/Sharpen-live/internal/alert/streamers/service"
	"github.com/Its-donkey/Sharpen-live/internal/alert/submissions"
)

func adminSubmissionsErrorMessage(err error) string {
	switch {
	case errors.Is(err, adminservice.ErrInvalidAction):
		return "Invalid submission action."
	case errors.Is(err, adminservice.ErrMissingIdentifier):
		return "Submission ID is required."
	case errors.Is(err, submissions.ErrNotFound):
		return "Submission not found."
	default:
		if err != nil {
			return err.Error()
		}
		return "An unexpected error occurred."
	}
}

func adminStreamersErrorMessage(err error) string {
	switch {
	case errors.Is(err, streamers.ErrStreamerNotFound):
		return "Streamer not found."
	case errors.Is(err, streamers.ErrDuplicateAlias):
		return "Streamer alias already exists."
	case errors.Is(err, streamersvc.ErrValidation):
		return "Invalid streamer update."
	default:
		if err != nil {
			return err.Error()
		}
		return "An unexpected error occurred."
	}
}

func pastTense(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "approve":
		return "approved"
	case "reject":
		return "rejected"
	default:
		if action == "" {
			return action
		}
		if strings.HasSuffix(action, "e") {
			return action + "d"
		}
		return action + "ed"
	}
}

func (s *server) loadAdminLogs(limit int) ([]logCategoryView, error) {
	if limit <= 0 {
		limit = adminLogLimit
	}
	if strings.TrimSpace(s.logDir) == "" {
		return nil, errors.New("log directory not configured")
	}
	categories := []struct {
		File  string
		Title string
	}{
		{File: "general.json", Title: "General"},
		{File: "http.json", Title: "HTTP"},
		{File: "websub.json", Title: "WebSub"},
	}
	views := make([]logCategoryView, 0, len(categories))
	for _, cat := range categories {
		view := logCategoryView{Title: cat.Title}
		entries, err := s.readLogFile(filepath.Join(s.logDir, cat.File), limit)
		if err != nil {
			view.Error = err.Error()
		} else {
			view.Entries = entries
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *server) readLogFile(path string, limit int) ([]logEntryView, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	var payload struct {
		LogEvents []json.RawMessage `json:"logevents"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	if len(payload.LogEvents) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = adminLogLimit
	}
	entries := make([]logEntryView, 0, min(limit, len(payload.LogEvents)))
	for i := len(payload.LogEvents) - 1; i >= 0 && len(entries) < limit; i-- {
		entries = append(entries, mapLogEntry(payload.LogEvents[i]))
	}
	return entries, nil
}

func mapLogEntry(raw json.RawMessage) logEntryView {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return logEntryView{
			Timestamp: "(unknown time)",
			Message:   "Unparseable log entry",
			Meta:      err.Error(),
		}
	}
	entry := logEntryView{
		Category:  stringVal(payload, "category"),
		RequestID: stringVal(payload, "id"),
		Message:   stringVal(payload, "message"),
		Timestamp: formatLogTime(stringVal(payload, "time")),
	}
	method := stringVal(payload, "method")
	path := stringVal(payload, "path")
	requestURI := stringVal(payload, "requestUri")
	query := stringVal(payload, "query")
	direction := stringVal(payload, "direction")
	status := intVal(payload, "status")
	remote := stringVal(payload, "remote")
	forwarded := stringSliceVal(payload, "forwardedFor")
	realIP := stringVal(payload, "realIp")
	duration := intVal(payload, "durationMs")
	responseBytes := int64Val(payload, "responseBytes")
	bodyBytes := int64Val(payload, "bodyBytes")
	contentLength := int64Val(payload, "contentLength")
	contentType := stringVal(payload, "contentType")
	rawBytes := int64Val(payload, "rawBytes")
	rawEncoding := stringVal(payload, "rawEncoding")
	body := stringVal(payload, "body")
	bodyEncoding := stringVal(payload, "bodyEncoding")
	headers := mapVal(payload, "headers")
	trailer := mapVal(payload, "trailer")
	target := requestURI
	if target == "" {
		target = path
		if target != "" && query != "" && !strings.Contains(target, "?") {
			target = target + "?" + query
		}
	}
	if method != "" || target != "" {
		var parts []string
		if direction != "" {
			parts = append(parts, titleCase(direction))
		}
		methodPath := strings.TrimSpace(strings.TrimSpace(method + " " + target))
		if methodPath != "" {
			parts = append(parts, methodPath)
		}
		if status > 0 {
			parts = append(parts, fmt.Sprintf("(%d)", status))
		}
		if len(parts) > 0 {
			entry.Message = strings.Join(parts, " ")
		}
	}
	var meta []string
	if duration > 0 {
		meta = append(meta, fmt.Sprintf("%dms", duration))
	}
	if responseBytes > 0 {
		meta = append(meta, fmt.Sprintf("resp %d bytes", responseBytes))
	}
	if contentLength > 0 {
		meta = append(meta, fmt.Sprintf("content %d bytes", contentLength))
	}
	if contentType != "" {
		meta = append(meta, contentType)
	}
	if remote != "" {
		meta = append(meta, "from "+remote)
	}
	if len(forwarded) > 0 {
		meta = append(meta, "xff: "+strings.Join(forwarded, ", "))
	}
	if realIP != "" {
		meta = append(meta, "real-ip: "+realIP)
	}
	if entry.RequestID != "" {
		meta = append(meta, "id "+entry.RequestID)
	}
	entry.Meta = strings.Join(meta, " • ")
	var details []string
	if rawBytes > 0 {
		if rawEncoding != "" {
			details = append(details, fmt.Sprintf("Raw dump: %d bytes (%s)", rawBytes, rawEncoding))
		} else {
			details = append(details, fmt.Sprintf("Raw dump: %d bytes", rawBytes))
		}
	}
	if body != "" || bodyBytes > 0 {
		bodyLabel := fmt.Sprintf("Body: %d bytes", bodyBytes)
		if bodyEncoding != "" {
			bodyLabel = fmt.Sprintf("Body: %d bytes (%s)", bodyBytes, bodyEncoding)
		}
		details = append(details, bodyLabel)
		preview := trimPreview(body, adminBodyPreviewLimit)
		if preview != "" {
			details = append(details, preview)
		}
	}
	if len(headers) > 0 {
		details = append(details, "Headers:\n"+formatHeaders(headers))
	}
	if len(trailer) > 0 {
		details = append(details, "Trailers:\n"+formatHeaders(trailer))
	}
	entry.Details = strings.Join(details, "\n")
	if entry.Message == "" {
		entry.Message = "(no message)"
	}
	if entry.Timestamp == "" {
		entry.Timestamp = "(unknown time)"
	}
	return entry
}

func formatHeaders(values map[string]any) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var lines []string
	var total int
	for _, key := range keys {
		val := values[key]
		var parts []string
		switch v := val.(type) {
		case []any:
			parts = append(parts, stringSlice(v)...)
		case string:
			parts = append(parts, strings.TrimSpace(v))
		case float64:
			parts = append(parts, fmt.Sprintf("%g", v))
		default:
			parts = append(parts, fmt.Sprintf("%v", v))
		}
		line := fmt.Sprintf("%s: %s", key, strings.Join(parts, ", "))
		total += len(line)
		lines = append(lines, line)
		if total > adminHeadersPreviewLimit {
			lines = append(lines, "...")
			break
		}
	}
	return strings.Join(lines, "\n")
}

func trimPreview(body string, limit int) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return string(runes[:limit]) + "..."
}

func stringSliceVal(values map[string]any, key string) []string {
	raw, ok := values[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		return stringSlice(v)
	case []string:
		out := make([]string, 0, len(v))
		for _, s := range v {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	default:
		return nil
	}
}

func stringSlice(values []any) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, val := range values {
		if v, ok := val.(string); ok {
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mapVal(values map[string]any, key string) map[string]any {
	if raw, ok := values[key]; ok {
		if m, ok := raw.(map[string]any); ok {
			return m
		}
	}
	return nil
}

func int64Val(values map[string]any, key string) int64 {
	if raw, ok := values[key]; ok {
		switch v := raw.(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		case json.Number:
			if parsed, err := v.Int64(); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func stringVal(values map[string]any, key string) string {
	if raw, ok := values[key]; ok {
		if v, ok := raw.(string); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func intVal(values map[string]any, key string) int {
	if raw, ok := values[key]; ok {
		switch v := raw.(type) {
		case float64:
			return int(v)
		case int64:
			return int(v)
		case int:
			return v
		}
	}
	return 0
}

func formatLogTime(raw string) string {
	if raw == "" {
		return ""
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts.Local().Format("2006-01-02 15:04:05")
	}
	return strings.TrimSpace(raw)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func titleCase(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	return strings.ToUpper(lower[:1]) + lower[1:]
}
