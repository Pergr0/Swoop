//go:build linux || freebsd || openbsd || netbsd

package paths

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Downloads resolves the XDG user Downloads directory. It honours a localized
// folder name (e.g. "Загрузки") by reading ~/.config/user-dirs.dirs, and falls
// back to ~/Downloads.
func Downloads() string {
	if p := os.Getenv("XDG_DOWNLOAD_DIR"); p != "" {
		return expandHome(p)
	}
	if p := fromUserDirs(); p != "" {
		return p
	}
	return homeJoin("Downloads")
}

func fromUserDirs() string {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	f, err := os.Open(filepath.Join(cfg, "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "XDG_DOWNLOAD_DIR") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		val := strings.Trim(strings.TrimSpace(line[eq+1:]), "\"")
		if val != "" {
			return expandHome(val)
		}
	}
	return ""
}

func expandHome(p string) string {
	switch {
	case strings.HasPrefix(p, "$HOME/"):
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[len("$HOME/"):])
		}
	case strings.HasPrefix(p, "~/"):
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
