//go:build darwin

package i18n

import (
	"os"
	"os/exec"
	"strings"
)

func userLocaleRussian() bool {
	if out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output(); err == nil {
		if strings.Contains(strings.ToLower(string(out)), "ru") {
			return true
		}
	}
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if v := os.Getenv(key); v != "" && isRussianTag(v) {
			return true
		}
	}
	return false
}
