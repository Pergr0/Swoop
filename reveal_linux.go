//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func revealPathInShell(path string) error {
	path = filepath.Clean(path)
	if st, err := os.Stat(path); err != nil {
		return fmt.Errorf("downloads folder not found: %s: %w", path, err)
	} else if !st.IsDir() {
		return fmt.Errorf("downloads path is not a folder: %s", path)
	}

	try := func(name string, args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Env = os.Environ()
		return cmd.Start()
	}

	// gio is reliable in GTK/WebKit environments; xdg-open is the generic fallback.
	candidates := []struct {
		bin  string
		args []string
	}{
		{"gio", []string{"open", path}},
		{"xdg-open", []string{path}},
		{"/usr/bin/gio", []string{"open", path}},
		{"/usr/bin/xdg-open", []string{path}},
		{"/bin/xdg-open", []string{path}},
	}

	var lastErr error
	for _, c := range candidates {
		if c.bin[0] == '/' {
			if _, err := os.Stat(c.bin); err != nil {
				continue
			}
		} else if _, err := exec.LookPath(c.bin); err != nil {
			continue
		}
		if err := try(c.bin, c.args...); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return fmt.Errorf("open downloads folder: %w (install gio or xdg-utils)", lastErr)
	}
	return fmt.Errorf("open downloads folder: no opener found (install gio or xdg-utils)")
}
