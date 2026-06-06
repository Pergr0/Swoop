// Regenerate build/windows/icon.ico from build/appicon.png.
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/leaanthony/winicon"
)

func main() {
	repoRoot := os.Getenv("SWOOP_ROOT")
	if repoRoot == "" {
		var err error
		repoRoot, err = os.Getwd()
		if err != nil {
			fatal("getwd: %v", err)
		}
	}

	pngPath := filepath.Join(repoRoot, "build", "appicon.png")
	icoPath := filepath.Join(repoRoot, "build", "windows", "icon.ico")

	png, err := os.ReadFile(pngPath)
	if err != nil {
		fatal("read %s: %v", pngPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(icoPath), 0o755); err != nil {
		fatal("mkdir %s: %v", filepath.Dir(icoPath), err)
	}

	out, err := os.OpenFile(icoPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		fatal("create %s: %v", icoPath, err)
	}
	defer out.Close()

	if err := winicon.GenerateIcon(bytes.NewBuffer(png), out, []int{256, 128, 64, 48, 32, 16}); err != nil {
		fatal("generate icon: %v", err)
	}

	fmt.Printf("wrote %s\n", icoPath)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
