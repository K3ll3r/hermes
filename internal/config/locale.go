package config

import (
	"os"
	"strings"
)

// DetectLocale returns the user's preferred language as a BCP-47-ish
// two-letter code (e.g. "en", "ja", "de"). Checks HERMES_LOCALE first
// (explicit override), then standard POSIX locale variables.
// Returns "en" if nothing is set.
func DetectLocale() string {
	for _, env := range []string{"HERMES_LOCALE", "LANG", "LC_MESSAGES", "LANGUAGE"} {
		if val := os.Getenv(env); val != "" {
			return normalizeLocale(val)
		}
	}
	return "en"
}

// normalizeLocale extracts the two-letter language code from POSIX
// locale strings like "en_US.UTF-8", "ja_JP", "de", "C.UTF-8".
// Note: region is stripped, so zh_CN and zh_TW both normalize to "zh".
// To distinguish Simplified vs Traditional Chinese, use explicit locale
// keys like "zh" in the localized maps and set HERMES_LOCALE=zh.
func normalizeLocale(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "C" || s == "POSIX" {
		return "en"
	}
	// Strip encoding: "en_US.UTF-8" → "en_US"
	if i := strings.IndexByte(s, '.'); i > 0 {
		s = s[:i]
	}
	// Strip region: "en_US" → "en"
	if i := strings.IndexAny(s, "_-"); i > 0 {
		s = s[:i]
	}
	if len(s) < 2 {
		return "en"
	}
	return strings.ToLower(s[:2])
}
