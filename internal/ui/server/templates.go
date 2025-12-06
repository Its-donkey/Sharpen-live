// file name — /internal/ui/server/templates.go - NEW
package server

import (
	"fmt"
	"html/template"
	"path/filepath"
	"strings"

	"github.com/Its-donkey/Sharpen-live/internal/ui/forms"
)

// loadTemplates loads and wires all HTML templates used by the UI server.
// It returns a map keyed by logical template name (e.g. "home", "streamer").
func loadTemplates(dir string) (map[string]*template.Template, error) {
	funcs := template.FuncMap{
		"join":            strings.Join,
		"contains":        forms.ContainsString,
		"displayLanguage": forms.DisplayLanguage,
		"statusClass":     statusClass,
		"statusLabel":     statusLabel,
		"lower":           strings.ToLower,
		"formatDays":      formatDays,
	}

	base := filepath.Join(dir, "base.tmpl")
	home := filepath.Join(dir, "home.tmpl")
	streamer := filepath.Join(dir, "streamer.tmpl")
	submit := filepath.Join(dir, "submit_form.tmpl")
	admin := filepath.Join(dir, "admin.tmpl")
	logs := filepath.Join(dir, "logs.tmpl")
	config := filepath.Join(dir, "config.tmpl")

	homeTmpl, err := template.New("home").Funcs(funcs).ParseFiles(base, home, submit)
	if err != nil {
		return nil, fmt.Errorf("parse home templates: %w", err)
	}

	streamerTmpl, err := template.New("streamer").Funcs(funcs).ParseFiles(base, streamer)
	if err != nil {
		return nil, fmt.Errorf("parse streamer templates: %w", err)
	}

	adminTmpl, err := template.New("admin").Funcs(funcs).ParseFiles(base, admin)
	if err != nil {
		return nil, fmt.Errorf("parse admin templates: %w", err)
	}

	logsTmpl, err := template.New("logs").Funcs(funcs).ParseFiles(base, logs)
	if err != nil {
		return nil, fmt.Errorf("parse logs templates: %w", err)
	}

	configTmpl, err := template.New("config").Funcs(funcs).ParseFiles(base, config)
	if err != nil {
		return nil, fmt.Errorf("parse config templates: %w", err)
	}

	templates := map[string]*template.Template{
		"home":     homeTmpl,
		"streamer": streamerTmpl,
		"admin":    adminTmpl,
		"logs":     logsTmpl,
		"config":   configTmpl,
	}

	return templates, nil
}

// formatDays converts seconds to days for display
func formatDays(seconds int) string {
	if seconds == 0 {
		return "0"
	}
	days := seconds / 86400
	return fmt.Sprintf("%d", days)
}
