// Package paths resolves OS-standard user directories using native rules, so
// received files land where each platform expects them. Platform specifics live
// in build-tagged files (downloads_*.go).
package paths

import (
	"os"
	"path/filepath"
)

// homeJoin returns $HOME/sub, with sensible fallbacks if the home dir is
// unknown.
func homeJoin(sub string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, sub)
	}
	if cfg, err := os.UserConfigDir(); err == nil && cfg != "" {
		return filepath.Join(cfg, sub)
	}
	return sub
}
