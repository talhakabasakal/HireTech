package permissions

import "strings"

// Matches reports whether a granted permission covers a required permission.
// Supported patterns are exact grants, the global wildcard, a resource
// wildcard (resource:*), and the read-only wildcard (*:read).
func Matches(granted, required string) bool {
	granted = strings.ToLower(strings.TrimSpace(granted))
	required = strings.ToLower(strings.TrimSpace(required))
	if granted == "" || required == "" {
		return false
	}
	if granted == required || granted == "*" {
		return true
	}
	if strings.HasSuffix(granted, ":*") {
		prefix := strings.TrimSuffix(granted, ":*")
		return required == prefix || strings.HasPrefix(required, prefix+":")
	}
	return granted == "*:read" && strings.HasSuffix(required, ":read")
}
