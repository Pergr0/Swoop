package webpresence

import "strings"

// ParseBrowser returns a short browser label from an HTTP User-Agent header.
func ParseBrowser(ua string) string {
	if ua == "" {
		return "Браузер"
	}
	l := strings.ToLower(ua)
	switch {
	case strings.Contains(l, "edg/"):
		return "Edge"
	case strings.Contains(l, "samsungbrowser"):
		return "Samsung Internet"
	case strings.Contains(l, "firefox"):
		return "Firefox"
	case strings.Contains(l, "opr/") || strings.Contains(l, "opera"):
		return "Opera"
	case strings.Contains(l, "crios"):
		return "Chrome"
	case strings.Contains(l, "chrome"):
		return "Chrome"
	case strings.Contains(l, "safari"):
		return "Safari"
	default:
		return "Браузер"
	}
}
