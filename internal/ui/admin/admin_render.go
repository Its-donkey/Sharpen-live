//go:build js && wasm

package admin

import (
	"bytes"
	"encoding/json"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"github.com/Its-donkey/Sharpen-live/internal/ui/state"
)

// RenderAdminConsole draws the entire admin UI panel into the DOM.
func RenderAdminConsole() {
	container := getDocument().Call("getElementById", "admin-view")
	if !container.Truthy() {
		return
	}

	releaseAdminHandlers()

	isAuthenticated := strings.TrimSpace(state.AdminConsole.Token) != ""

	var builder strings.Builder
	builder.WriteString(`<section class="admin-panel" aria-live="polite">`)
	builder.WriteString(`<div class="admin-header"><div><h2 id="admin-console-title">Admin Dashboard</h2><p class="admin-help">Review submissions, manage the roster, and adjust server settings.</p></div>`)
	if isAuthenticated {
		refreshLabel := "Refresh data"
		if state.AdminConsole.Loading {
			refreshLabel = "Refreshing…"
		}
		builder.WriteString(`<div class="admin-header-actions">`)
		builder.WriteString(`<button type="button" class="admin-tab admin-logout-button" id="admin-refresh"`)
		if state.AdminConsole.Loading {
			builder.WriteString(` disabled`)
		}
		builder.WriteString(`>` + refreshLabel + `</button>`)
		builder.WriteString(`<button type="button" class="admin-tab admin-logout-button" id="admin-logout">Log out</button>`)
		builder.WriteString(`</div>`)
	}
	builder.WriteString(`</div>`)

	if state.AdminConsole.Status.Message != "" {
		tone := state.AdminConsole.Status.Tone
		if tone == "" {
			tone = "info"
		}
		builder.WriteString(`<div class="admin-status" data-state="` + html.EscapeString(tone) + `">` + html.EscapeString(state.AdminConsole.Status.Message) + `</div>`)
	}

	if !isAuthenticated {
		builder.WriteString(renderAdminLoginForm())
	} else {
		builder.WriteString(renderAdminTabs())
		switch state.AdminConsole.ActiveTab {
		case "activity":
			builder.WriteString(renderAdminActivityTab())
		case "settings":
			builder.WriteString(renderAdminSettingsTab())
		default:
			builder.WriteString(renderAdminStreamersTab())
		}
	}

	builder.WriteString(`</section>`)
	container.Set("innerHTML", builder.String())
	bindAdminEvents()
}

//----------------------------------------------------------------------------------
// The following lines need to be put back in below prior to going to production:
//
// <input type="password" id="admin-password" value="` + html.EscapeString(state.AdminConsole.LoginPassword) + `" placeholder="Enter your password" autocomplete="current-password" required />
// <input type="email" id="admin-email" value="` + html.EscapeString(state.AdminConsole.LoginEmail) + `" placeholder="you@example.com" autocomplete="username" required />
//
//----------------------------------------------------------------------------------

func renderAdminLoginForm() string {
	var builder strings.Builder
	builder.WriteString(`
<form id="admin-login-form" class="admin-auth">
  <div class="form-field form-field-wide">
    <span>Email</span>
    <input type="email" id="admin-email" value="admin@sharpen.live" autocomplete="username" required />
  </div>
  <div class="form-field form-field-wide">
    <span>Password</span>
    <input type="password" id="admin-password" value="change-me" autocomplete="current-password" required />
  </div>
  <div class="submit-streamer-actions">
    <button type="submit" class="submit-streamer-submit">Log in</button>
  </div>
</form>
<p class="admin-help">Use your admin credentials from the alert server configuration.</p>
`)
	return builder.String()
}

func renderAdminTabs() string {
	if state.AdminConsole.ActiveTab == "" {
		state.AdminConsole.ActiveTab = "streamers"
	}
	type tab struct {
		Key   string
		Label string
	}
	tabs := []tab{
		{Key: "streamers", Label: "Streamers"},
		{Key: "activity", Label: "Activity"},
		{Key: "settings", Label: "Settings"},
	}
	var builder strings.Builder
	builder.WriteString(`<div class="admin-tabs" role="tablist">`)
	for _, t := range tabs {
		className := "admin-tab"
		if state.AdminConsole.ActiveTab == t.Key {
			className += " active"
		}
		builder.WriteString(`<button type="button" class="` + className + `" data-admin-tab="` + t.Key + `">` + t.Label + `</button>`)
	}
	builder.WriteString(`</div>`)
	return builder.String()
}

func renderAdminStreamersTab() string {
	var builder strings.Builder
	builder.WriteString(`<div class="admin-grid" role="tabpanel" data-tab="streamers">`)
	builder.WriteString(renderAdminSubmissionsSection())
	builder.WriteString(renderAdminRosterSection())
	builder.WriteString(`</div>`)
	return builder.String()
}

func renderAdminActivityTab() string {
	state.AdminConsole.ActivityTab = "website"
	var builder strings.Builder
	builder.WriteString(`<section class="admin-activity" role="tabpanel" data-tab="activity">`)
	builder.WriteString(renderWebsiteActivityPanel())
	builder.WriteString(`</section>`)
	return builder.String()
}

func renderAdminActivitySubTabs() string {
	return ""
}

func renderWebsiteActivityPanel() string {
	var builder strings.Builder
	builder.WriteString(`<div class="admin-activity-panel" data-activity="website">`)
	builder.WriteString(`<div class="admin-streamers-header"><h4>Website server activity</h4><div class="admin-log-actions"><button type="button" class="secondary-button" id="admin-logs-reconnect">Reconnect feed</button><button type="button" class="secondary-button" id="admin-logs-clear">Clear feed</button></div></div>`)
	builder.WriteString(`<p class="admin-help">Live stdout/stderr entries from the helper server appear below.</p>`)
	if state.AdminConsole.ActivityLogsError != "" {
		builder.WriteString(`<div class="admin-log-error">` + html.EscapeString(state.AdminConsole.ActivityLogsError) + `</div>`)
	}
	builder.WriteString(`<div class="admin-log-feed" id="admin-log-feed">`)
	if len(state.AdminConsole.ActivityLogs) == 0 {
		builder.WriteString(`<div class="admin-empty admin-log-placeholder">Connecting to log stream…</div>`)
	} else {
		for i := len(state.AdminConsole.ActivityLogs) - 1; i >= 0; i-- {
			builder.WriteString(renderLogEntry(state.AdminConsole.ActivityLogs[i]))
		}
	}
	builder.WriteString(`</div></div>`)
	return builder.String()
}

func renderLogEntry(entry model.AdminActivityLog) string {
	var builder strings.Builder
	message := strings.TrimSpace(entry.Message)
	raw := strings.TrimSpace(entry.Raw)
	snippet := message
	if snippet == "" {
		snippet = summarizeLogMessage(raw)
	}
	if snippet == "" {
		snippet = "Log message"
	}
	displayRaw := formatLogRaw(raw, message)

	builder.WriteString(`<details class="admin-log-entry">`)
	builder.WriteString(`<summary>`)
	builder.WriteString(`<div class="admin-log-summary">`)
	if entry.Time != "" {
		builder.WriteString(`<div class="admin-log-time">` + html.EscapeString(entry.Time) + `</div>`)
	}
	builder.WriteString(`<div class="admin-log-snippet">` + html.EscapeString(snippet) + `</div>`)
	builder.WriteString(`</div>`)
	builder.WriteString(`</summary>`)
	builder.WriteString(`<div class="admin-log-body">`)
	if message != "" && message != raw {
		builder.WriteString(`<div class="admin-log-full">` + html.EscapeString(message) + `</div>`)
	}
	builder.WriteString(`<pre class="admin-log-raw">` + html.EscapeString(displayRaw) + `</pre>`)
	builder.WriteString(`</div>`)
	builder.WriteString(`</details>`)
	return builder.String()
}

func renderApiActivityPanel() string {
	var builder strings.Builder
	builder.WriteString(`<div class="admin-activity-panel" data-activity="api">`)
	builder.WriteString(`<h4>API server activity</h4>`)
	builder.WriteString(`<p class="admin-help">Request volume, error spikes, and PubSub callbacks will be summarized in this feed.</p>`)
	builder.WriteString(`<div class="admin-empty">No API telemetry connected yet.</div>`)
	builder.WriteString(`</div>`)
	return builder.String()
}

func renderAdminSubmissionsSection() string {
	var builder strings.Builder
	builder.WriteString(`<section aria-labelledby="admin-submissions-title">`)
	builder.WriteString(`<h3 id="admin-submissions-title">Pending submissions</h3>`)
	switch {
	case state.AdminConsole.Loading && len(state.AdminConsole.Submissions) == 0:
		builder.WriteString(`<div class="admin-empty">Loading submissions…</div>`)
	case len(state.AdminConsole.Submissions) == 0:
		builder.WriteString(`<div class="admin-empty">No pending submissions at the moment.</div>`)
	default:
		builder.WriteString(`<div class="admin-submissions">`)
		for _, submission := range state.AdminConsole.Submissions {
			builder.WriteString(renderSubmissionCard(submission))
		}
		builder.WriteString(`</div>`)
	}
	builder.WriteString(`</section>`)
	return builder.String()
}

func renderSubmissionCard(sub model.AdminSubmission) string {
	var builder strings.Builder
	builder.WriteString(`<article class="admin-card" data-submission-id="` + html.EscapeString(sub.ID) + `">`)
	builder.WriteString(`<div class="admin-card-header"><h4>` + html.EscapeString(sub.Alias) + `</h4><span class="admin-card-meta">Submitted ` + html.EscapeString(sub.SubmittedAt) + `</span></div>`)
	builder.WriteString(`<section><strong>Description</strong><p>` + html.EscapeString(sub.Description) + `</p></section>`)
	if len(sub.Languages) > 0 {
		builder.WriteString(`<section><strong>Languages</strong><p>` + html.EscapeString(strings.Join(sub.Languages, " · ")) + `</p></section>`)
	}
	if strings.TrimSpace(sub.PlatformURL) != "" {
		builder.WriteString(`<section><strong>Platform</strong><p>` + html.EscapeString(sub.PlatformURL) + `</p></section>`)
	}
	builder.WriteString(`<div class="admin-card-actions">
      <button type="button" data-moderate-action="approve" data-submission-id="` + html.EscapeString(sub.ID) + `">Approve</button>
      <button type="button" data-moderate-action="reject" data-submission-id="` + html.EscapeString(sub.ID) + `">Reject</button>
    </div>`)
	builder.WriteString(`</article>`)
	return builder.String()
}

func renderAdminRosterSection() string {
	var builder strings.Builder
	builder.WriteString(`<section aria-labelledby="admin-streamers-title">`)
	statusCheckLabel := "Check online status"
	if state.AdminConsole.StatusCheckRunning {
		statusCheckLabel = "Checking…"
	}
	disableCheck := ""
	if state.AdminConsole.StatusCheckRunning || state.AdminConsole.Loading {
		disableCheck = " disabled"
	}
	builder.WriteString(`<div class="admin-streamers-header"><h3 id="admin-streamers-title">Current roster</h3><button type="button" class="secondary-button" id="admin-status-check"` + disableCheck + `>` + html.EscapeString(statusCheckLabel) + `</button></div>`)
	builder.WriteString(`<p class="admin-help">Use the public submission form on the main dashboard to add new streamers. Pending submissions will appear above for approval.</p>`)
	if len(state.AdminConsole.Streamers) == 0 {
		if state.AdminConsole.Loading {
			builder.WriteString(`<div class="admin-empty">Loading roster…</div>`)
		} else {
			builder.WriteString(`<div class="admin-empty">No streamers found. Submit a streamer from the public form to add one.</div>`)
		}
	} else {
		builder.WriteString(`<div class="admin-streamers">`)
		for _, entry := range state.AdminConsole.Streamers {
			form := state.AdminConsole.StreamerForms[entry.ID]
			if form == nil {
				form = newStreamerFormFromStreamer(entry)
				state.AdminConsole.StreamerForms[entry.ID] = form
			}
			builder.WriteString(renderStreamerCard(entry, form))
		}
		builder.WriteString(`</div>`)
	}
	builder.WriteString(`</section>`)
	return builder.String()
}

func renderStreamerCard(s model.Streamer, form *model.AdminStreamerForm) string {
	var builder strings.Builder
	builder.WriteString(`<article class="admin-card" data-streamer-id="` + html.EscapeString(s.ID) + `">`)
	builder.WriteString(`<div class="admin-card-header">`)
	builder.WriteString(`<div class="admin-card-heading"><h4>` + html.EscapeString(s.Name) + `</h4><span class="admin-card-meta">Status: ` + html.EscapeString(strings.Title(form.Status)) + ` · Languages: ` + html.EscapeString(strings.Join(s.Languages, " · ")) + `</span></div>`)
	builder.WriteString(`<div class="admin-card-actions admin-card-actions--streamer">`)
	editLabel := "Edit"
	if form.Visible {
		editLabel = "Cancel edit"
	}
	builder.WriteString(`<button type="button" data-edit-streamer="` + html.EscapeString(s.ID) + `">` + editLabel + `</button>`)
	builder.WriteString(`<button type="button" class="remove-platform-button" data-delete-streamer="` + html.EscapeString(s.ID) + `">Delete</button>`)
	builder.WriteString(`</div></div>`)
	if form.Visible {
		builder.WriteString(renderStreamerForm(form, s.ID, "Update streamer", "Save changes", true))
	} else {
		builder.WriteString(`<div class="admin-card-body"><p>` + html.EscapeString(s.Description) + `</p>`)
		if len(s.Platforms) > 0 {
			builder.WriteString(`<div class="admin-card-meta"><strong>Platforms</strong><ul class="platform-list">`)
			for _, p := range s.Platforms {
				badge := ""
				if strings.EqualFold(p.Name, "youtube") {
					badge = renderYouTubeLeaseBadge(s, p)
				}
				builder.WriteString(`<li>` + html.EscapeString(p.Name) + ` · ` + html.EscapeString(p.ChannelURL))
				if badge != "" {
					builder.WriteString(` ` + badge)
				}
				builder.WriteString(`</li>`)
			}
			builder.WriteString(`</ul></div>`)
		}
		builder.WriteString(`</div>`)
	}
	builder.WriteString(`</article>`)
	return builder.String()
}

func renderStreamerForm(form *model.AdminStreamerForm, key, heading, submitLabel string, includeCancel bool) string {
	var builder strings.Builder
	ensurePlatformRows(form)
	formID := "create"
	if key != "" {
		formID = key
	}
	builder.WriteString(`<form class="admin-streamer-form" data-streamer-form="` + html.EscapeString(formID) + `">`)
	builder.WriteString(`<h4>` + html.EscapeString(heading) + `</h4>`)
	builder.WriteString(`<div class="form-grid">
    <label class="form-field">
      <span>Name</span>
      <input type="text" data-streamer-field="name" data-streamer-id="` + html.EscapeString(formID) + `" value="` + html.EscapeString(form.Name) + `" required />
    </label>
    <label class="form-field">
      <span>Status</span>
      <select data-streamer-field="status" data-streamer-id="` + html.EscapeString(formID) + `">
        <option value="online"` + selectedAttr(form.Status == "online") + `>Online</option>
        <option value="busy"` + selectedAttr(form.Status == "busy") + `>Workshop</option>
        <option value="offline"` + selectedAttr(form.Status == "offline") + `>Offline</option>
      </select>
    </label>
    <label class="form-field">
      <span>Status label</span>
      <input type="text" data-streamer-field="statusLabel" data-streamer-id="` + html.EscapeString(formID) + `" value="` + html.EscapeString(form.StatusLabel) + `" />
    </label>
    <div class="form-field form-field-wide">
      <span>Description</span>
      <textarea rows="3" data-streamer-field="description" data-streamer-id="` + html.EscapeString(formID) + `">` + html.EscapeString(form.Description) + `</textarea>
    </div>
    <div class="form-field form-field-wide">
      <span>Languages</span>
      <input type="text" placeholder="English, Japanese" data-streamer-field="languages" data-streamer-id="` + html.EscapeString(formID) + `" value="` + html.EscapeString(form.LanguagesInput) + `" />
    </div>
  </div>`)
	builder.WriteString(`<fieldset class="platform-fieldset">
    <legend>Platforms</legend>
    <div class="platform-rows">`)
	for _, row := range form.Platforms {
		leaseHint := renderLeaseExpiryHint(formID, row)
		builder.WriteString(`<div class="platform-row" data-platform-row="` + html.EscapeString(row.ID) + `">
        <label class="form-field form-field-inline">
          <span>Platform name</span>
          <input type="text" data-platform-field="name" data-streamer-id="` + html.EscapeString(formID) + `" data-row-id="` + html.EscapeString(row.ID) + `" value="` + html.EscapeString(row.Name) + `" placeholder="YouTube" required />
        </label>
        <label class="form-field form-field-inline">
          <span>Channel URL</span>
          <input type="url" data-platform-field="channel" data-streamer-id="` + html.EscapeString(formID) + `" data-row-id="` + html.EscapeString(row.ID) + `" value="` + html.EscapeString(row.ChannelURL) + `" placeholder="https://example.com" required />
        </label>
        `)
		if leaseHint != "" {
			builder.WriteString(`<div class="platform-lease-hint">Subscription lease expiry ` + html.EscapeString(leaseHint) + `</div>`)
		}
		builder.WriteString(`
        <button type="button" class="remove-platform-button" data-remove-platform="` + html.EscapeString(row.ID) + `" data-platform-owner="` + html.EscapeString(formID) + `">Remove</button>
      </div>`)
	}
	builder.WriteString(`</div>
    <button type="button" class="add-platform-button" data-add-platform="` + html.EscapeString(formID) + `">+ Add another platform</button>
  </fieldset>`)
	if form.Error != "" {
		builder.WriteString(`<div class="form-error">` + html.EscapeString(form.Error) + `</div>`)
	}
	builder.WriteString(`<div class="submit-streamer-actions">
    <button type="submit" class="submit-streamer-submit">` + html.EscapeString(submitLabel) + `</button>`)
	if includeCancel {
		builder.WriteString(`<button type="button" class="submit-streamer-cancel" data-cancel-edit="` + html.EscapeString(formID) + `">Cancel</button>`)
	}
	builder.WriteString(`</div>`)
	builder.WriteString(`</form>`)
	return builder.String()
}

func renderYouTubeLeaseBadge(streamer model.Streamer, platform model.Platform) string {
	lease := lookupLeaseStatus(streamer, platform)
	if lease == nil {
		return ""
	}
	class, label := leaseBadgePresentation(*lease)
	var builder strings.Builder
	builder.WriteString(`<span class="yt-lease-badge ` + class + `">`)
	builder.WriteString(`<span class="yt-lease-dot"></span>`)
	builder.WriteString(`<span class="yt-lease-text">` + html.EscapeString(label) + `</span>`)
	if lease.Expired && lease.StartDate != "" {
		builder.WriteString(`<span class="yt-lease-date">Leased ` + html.EscapeString(lease.StartDate) + `</span>`)
	}
	builder.WriteString(`</span>`)
	return builder.String()
}

func lookupLeaseStatus(streamer model.Streamer, platform model.Platform) *model.YouTubeLeaseStatus {
	leases := state.AdminConsole.YouTubeLeases
	if len(leases) == 0 {
		return nil
	}
	keys := []string{
		normalizeLeaseLookupKey(streamer.Name),
		normalizeLeaseLookupKey(streamer.ID),
		normalizeLeaseLookupKey(platform.ID),
		normalizeLeaseLookupKey(extractYouTubeHandle(platform.ChannelURL)),
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if lease, ok := leases[key]; ok {
			return &lease
		}
	}
	return nil
}

func leaseBadgePresentation(status model.YouTubeLeaseStatus) (class string, label string) {
	switch {
	case status.Expired:
		return "expired", "Expired"
	case status.ExpiringSoon || strings.EqualFold(status.Status, "expiring"):
		return "expiring", "Expiring soon"
	default:
		return "valid", "Valid"
	}
}

func extractYouTubeHandle(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "@") {
		return raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for _, part := range segments {
		if strings.HasPrefix(part, "@") {
			return part
		}
	}
	return ""
}

func normalizeLeaseLookupKey(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.TrimPrefix(raw, "@")
	return raw
}

func renderLeaseExpiryHint(formID string, row model.PlatformFormRow) string {
	if !strings.EqualFold(strings.TrimSpace(row.Name), "youtube") {
		return ""
	}
	lease := leaseForRow(formID, row)
	if lease == nil {
		return ""
	}
	expiry := strings.TrimSpace(lease.LeaseExpires)
	if t, err := time.Parse(time.RFC3339, expiry); err == nil {
		expiry = t.UTC().Format("2006-01-02 15:04 MST")
	}
	if expiry == "" {
		expiry = "unknown"
	}
	return expiry
}

func leaseForRow(formID string, row model.PlatformFormRow) *model.YouTubeLeaseStatus {
	leases := state.AdminConsole.YouTubeLeases
	if len(leases) == 0 {
		return nil
	}
	keys := []string{
		normalizeLeaseLookupKey(formID),
		normalizeLeaseLookupKey(row.ChannelID),
		normalizeLeaseLookupKey(row.Handle),
		normalizeLeaseLookupKey(extractYouTubeHandle(row.ChannelURL)),
	}
	if streamer := findStreamerByID(formID); streamer != nil {
		keys = append(keys, normalizeLeaseLookupKey(streamer.Name))
		keys = append(keys, normalizeLeaseLookupKey(streamer.ID))
		for _, p := range streamer.Platforms {
			if strings.EqualFold(p.Name, "youtube") {
				keys = append(keys, normalizeLeaseLookupKey(extractYouTubeHandle(p.ChannelURL)))
			}
		}
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if lease, ok := leases[key]; ok {
			return &lease
		}
	}
	return nil
}

func selectedAttr(ok bool) string {
	if ok {
		return ` selected`
	}
	return ""
}

func summarizeLogMessage(message string) string {
	message = strings.TrimSpace(strings.ReplaceAll(message, "\r\n", "\n"))
	if message == "" {
		return "Log message"
	}
	lines := strings.Split(message, "\n")
	summary := strings.TrimSpace(lines[0])
	if summary == "" && len(lines) > 1 {
		summary = strings.TrimSpace(lines[1])
	}
	const maxSummaryLen = 160
	runes := []rune(summary)
	if len(runes) > maxSummaryLen {
		runes = append(runes[:maxSummaryLen-1], '…')
	}
	return string(runes)
}

func formatLogRaw(raw string, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = strings.TrimSpace(fallback)
	}
	if raw == "" {
		return "(no message)"
	}

	if formatted, ok := indentJSON(raw); ok {
		return formatted
	}
	return unescapeLogNewlines(raw)
}

func indentJSON(raw string) (string, bool) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(raw), "", "  "); err != nil {
		return "", false
	}
	return buf.String(), true
}

func unescapeLogNewlines(raw string) string {
	raw = strings.ReplaceAll(raw, `\r\n`, "\n")
	raw = strings.ReplaceAll(raw, `\n`, "\n")
	return raw
}

func renderAdminSettingsTab() string {
	var builder strings.Builder
	builder.WriteString(`<section class="admin-settings" role="tabpanel" data-tab="settings">`)
	builder.WriteString(`<h3>Environment settings</h3>`)
	builder.WriteString(`<p class="admin-help">Update runtime configuration. Some changes may require restarting the server.</p>`)
	draft := state.AdminConsole.SettingsDraft
	if draft == nil {
		builder.WriteString(`<div class="admin-empty">Settings unavailable.</div>`)
	} else {
		saveLabel := "Save settings"
		disabled := ""
		if state.AdminConsole.SettingsSaving {
			saveLabel = "Saving…"
			disabled = " disabled"
		}
		builder.WriteString(`<form class="admin-settings-form" id="admin-settings-form">
      <div class="form-grid">
        `)
		builder.WriteString(settingsInput("Admin email", "email", "adminEmail", draft.AdminEmail))
		builder.WriteString(settingsInput("Admin password", "password", "adminPassword", draft.AdminPassword))
		builder.WriteString(settingsInput("Admin token", "text", "adminToken", draft.AdminToken))
		builder.WriteString(settingsInput("YouTube API key", "password", "youtubeApiKey", draft.YouTubeAPIKey))
		builder.WriteString(settingsInput("YouTube alerts callback URL", "url", "youtubeAlertsCallback", draft.YouTubeAlertsCallback))
		builder.WriteString(settingsInput("YouTube alerts secret", "password", "youtubeAlertsSecret", draft.YouTubeAlertsSecret))
		builder.WriteString(settingsInput("YouTube alerts verify prefix", "text", "youtubeAlertsVerifyPrefix", draft.YouTubeAlertsVerifyPref))
		builder.WriteString(settingsInput("YouTube alerts verify suffix", "text", "youtubeAlertsVerifySuffix", draft.YouTubeAlertsVerifySuff))
		builder.WriteString(settingsInput("YouTube alerts hub URL", "url", "youtubeAlertsHubUrl", draft.YouTubeAlertsHubURL))
		builder.WriteString(settingsInput("Listen address", "text", "listenAddr", draft.ListenAddr))
		builder.WriteString(settingsInput("Data directory", "text", "dataDir", draft.DataDir))
		builder.WriteString(settingsInput("Static directory", "text", "staticDir", draft.StaticDir))
		builder.WriteString(settingsInput("Streamers file", "text", "streamersFile", draft.StreamersFile))
		builder.WriteString(settingsInput("Submissions file", "text", "submissionsFile", draft.SubmissionsFile))
		builder.WriteString(`</div>
        <div class="submit-streamer-actions">
          <button type="submit" class="submit-streamer-submit" id="admin-save-settings"` + disabled + `>` + html.EscapeString(saveLabel) + `</button>
        </div>
      </form>`)
	}
	builder.WriteString(`</section>`)
	return builder.String()
}

func settingsInput(label, inputType, field, value string) string {
	return `<label class="form-field">
    <span>` + html.EscapeString(label) + `</span>
    <input type="` + inputType + `" data-settings-field="` + field + `" value="` + html.EscapeString(value) + `" />
  </label>`
}

func renderAdminMonitorTab() string {
	var builder strings.Builder
	builder.WriteString(`<section class="admin-monitor" role="tabpanel" data-tab="monitor">`)
	builder.WriteString(`<div class="admin-streamers-header"><h3>YouTube alerts monitor</h3><button type="button" class="secondary-button" id="admin-monitor-refresh">Refresh monitor</button></div>`)
	if state.AdminConsole.MonitorLoading && len(state.AdminConsole.MonitorEvents) == 0 {
		builder.WriteString(`<div class="admin-empty">Loading monitor events…</div>`)
	} else if len(state.AdminConsole.MonitorEvents) == 0 {
		builder.WriteString(`<div class="admin-empty">No PubSub events recorded yet.</div>`)
	} else {
		builder.WriteString(`<div class="admin-monitor-log">`)
		for _, event := range state.AdminConsole.MonitorEvents {
			builder.WriteString(`<article class="admin-monitor-entry">
        <p><strong>` + html.EscapeString(strings.Title(event.Platform)) + `</strong> · ` + html.EscapeString(event.Timestamp) + `<br/>` + html.EscapeString(event.Message) + `</p>
      </article>`)
		}
		builder.WriteString(`</div>`)
	}
	builder.WriteString(`</section>`)
	return builder.String()
}
