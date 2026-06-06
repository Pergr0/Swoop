//go:build !linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func revealPathInShell(path string) error {
	path = filepath.Clean(path)
	if st, err := os.Stat(path); err != nil {
		return fmt.Errorf("downloads folder not found: %s: %w", path, err)
	} else if !st.IsDir() {
		return fmt.Errorf("downloads path is not a folder: %s", path)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	cmd.Env = os.Environ()
	return cmd.Start()
}
