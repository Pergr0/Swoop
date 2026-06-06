package transfer

import (
	"os"
	"path/filepath"
	"testing"

	"swoop/core/protocol"
)

func TestResolveSendFilesNested(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(project, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	f1 := filepath.Join(project, "a.txt")
	f2 := filepath.Join(project, "sub", "b.txt")
	if err := os.WriteFile(f1, []byte("aa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("bbb"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, paths, total, err := resolveSendFiles([]protocol.SendItem{
		{Path: f1, RelPath: "project/a.txt"},
		{Path: f2, RelPath: "project/sub/b.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || len(paths) != 2 || total != 5 {
		t.Fatalf("files=%d paths=%d total=%d", len(files), len(paths), total)
	}
	if files[0].RelPath != "project/a.txt" || files[1].RelPath != "project/sub/b.txt" {
		t.Fatalf("rel: %+v", files)
	}
}

func TestSanitizeRelPath(t *testing.T) {
	got := sanitizeRelPath(`project/sub/file.txt`)
	want := filepath.Join("project", "sub", "file.txt")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
