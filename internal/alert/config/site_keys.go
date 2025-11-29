// file name — /internal/alert/config/site_keys.go
package config

import "strings"

// NormaliseSiteKey trims and canonicalises a site key for lookups.
func NormaliseSiteKey(siteArg string) string {
	key := strings.TrimSpace(siteArg)
	if key == "" {
		return ""
	}
	return NormaliseNameKey(key)
}

// NormaliseNameKey converts a free-form name into a normalised key.
//
// It replaces ".", "_" and spaces with "-", and converts to lowercase.
func NormaliseNameKey(name string) string {
	replacer := strings.NewReplacer(".", "-", "_", "-", " ", "-")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(name)))
}
