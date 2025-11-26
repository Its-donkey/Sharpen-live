package handlers

import "strings"

// IsAlertPath reports whether the request path targets the alerts webhook.
// It accepts optional prefixes (e.g. /dev/alerts) and tolerates trailing slashes.
func IsAlertPath(path string) bool {
	trimmed := strings.TrimRight(strings.TrimSpace(path), "/")
	return strings.HasSuffix(trimmed, "/alerts") || strings.HasSuffix(trimmed, "/alert")
}
