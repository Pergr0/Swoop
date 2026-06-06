package staging

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDirectoryTree(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(project, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "readme.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "src", "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := Scan([]string{project})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Kind != KindDir || entries[0].Name != "project" {
		t.Fatalf("root: %+v", entries)
	}
	if entries[0].FileCount != 2 {
		t.Fatalf("file count: %d", entries[0].FileCount)
	}
	items := CollectFiles(entries[0])
	if len(items) != 2 {
		t.Fatalf("items: %+v", items)
	}
	byRel := map[string]string{}
	for _, it := range items {
		byRel[it.RelPath] = it.Path
	}
	if byRel["project/readme.txt"] == "" || byRel["project/src/main.go"] == "" {
		t.Fatalf("rel paths: %+v", byRel)
	}
}

func TestSummarizeOffer(t *testing.T) {
	s := SummarizeOffer([]FileMetaLite{
		{Name: "a.txt", RelPath: "proj/a.txt", Size: 10},
		{Name: "b.txt", RelPath: "proj/sub/b.txt", Size: 20},
		{Name: "c.txt", RelPath: "docs/c.txt", Size: 5},
		{Name: "solo.txt", RelPath: "solo.txt", Size: 1},
	})
	if len(s.RootDirs) != 2 || s.LooseFiles != 1 || s.FileCount != 4 || s.TotalSize != 36 {
		t.Fatalf("summary: %+v", s)
	}
}
