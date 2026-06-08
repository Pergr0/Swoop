//go:build unix && !darwin

package i18n

import "os"

func userLocaleRussian() bool {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if v := os.Getenv(key); v != "" && isRussianTag(v) {
			return true
		}
	}
	return false
}
