//go:build windows

package i18n

import (
	"os"

	"golang.org/x/sys/windows"
)

func userLocaleRussian() bool {
	id, err := windows.GetUserDefaultUILanguage()
	if err == nil && id&0x3ff == 0x19 { // LANG_RUSSIAN
		return true
	}
	return locale_unix_fallback()
}

// locale_unix_fallback checks env vars when the Windows API is unavailable.
func locale_unix_fallback() bool {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if v := os.Getenv(key); v != "" && isRussianTag(v) {
			return true
		}
	}
	return false
}
