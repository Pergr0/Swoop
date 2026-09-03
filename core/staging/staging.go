// Package staging scans local files and directories into a tree for the
// sender UI and builds wire metadata (relative paths) for transfer.
package staging

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"swoop/core/protocol"
)

// Kind marks a staging tree node.
type Kind string

const (
	KindFile Kind = "file"
	KindDir  Kind = "dir"
)

// Entry is one node in the sender staging tree (a root file/dir or a child).
type Entry struct {
	Path      string  `json:"path"`
	Name      string  `json:"name"`
	Kind      Kind    `json:"kind"`
	RelPath   string  `json:"relPath"`
	Size      int64   `json:"size"`
	FileCount int     `json:"fileCount"`
	Children  []Entry `json:"children,omitempty"`
}

// SendItem is a single file the sender chose to transfer.
type SendItem struct {
	Path    string `json:"path"`
	RelPath string `json:"relPath"`
}

// RootDirInfo summarizes one top-level directory in an incoming offer.
type RootDirInfo struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	FileCount int    `json:"fileCount"`
}

// OfferSummary is derived from the file list shown to the receiver.
type OfferSummary struct {
	RootDirs   []RootDirInfo `json:"rootDirs"`
	LooseFiles int           `json:"looseFiles"`
	FileCount  int           `json:"fileCount"`
	TotalSize  int64         `json:"totalSize"`
}

type dirBuilder struct {
	entry    Entry
	children map[string]*dirBuilder
}

// Scan builds staging trees for the given absolute paths (files or directories).
// Returns an error if the total file count would exceed the transfer limit.
func Scan(paths []string) ([]Entry, error) {
	seen := make(map[string]struct{})
	var roots []Entry
	var totalFiles int
	for _, p := range paths {
		p = filepath.Clean(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		e, err := scanOne(p)
		if err != nil {
			return nil, err
		}
		totalFiles += e.FileCount
		if totalFiles > protocol.MaxTransferFiles {
			return nil, fmt.Errorf("слишком много файлов (макс %d)", protocol.MaxTransferFiles)
		}
		roots = append(roots, e)
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Name < roots[j].Name })
	return roots, nil
}

func scanOne(abs string) (Entry, error) {
	fi, err := os.Stat(abs)
	if err != nil {
		return Entry{}, err
	}
	if !fi.IsDir() {
		name := filepath.Base(abs)
		return Entry{
			Path: abs, Name: name, Kind: KindFile,
			RelPath: name, Size: fi.Size(), FileCount: 1,
		}, nil
	}
	rootName := filepath.Base(abs)
	children, err := buildDirTree(abs, rootName)
	if err != nil {
		return Entry{}, err
	}
	e := Entry{
		Path: abs, Name: rootName, Kind: KindDir,
		RelPath: rootName, Children: children,
	}
	aggregate(&e)
	return e, nil
}

func buildDirTree(dir, rootName string) ([]Entry, error) {
	root := &dirBuilder{children: map[string]*dirBuilder{}}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dir {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")

		cur := root
		for i, part := range parts {
			b, ok := cur.children[part]
			if !ok {
				abs := filepath.Join(dir, filepath.FromSlash(strings.Join(parts[:i+1], "/")))
				kind := KindDir
				if i == len(parts)-1 && !d.IsDir() {
					kind = KindFile
				}
				b = &dirBuilder{
					entry: Entry{
						Path: abs, Name: part, Kind: kind,
						RelPath: rootName + "/" + strings.Join(parts[:i+1], "/"),
					},
					children: map[string]*dirBuilder{},
				}
				cur.children[part] = b
			}
			cur = b
		}
		if !d.IsDir() {
			cur.entry.Kind = KindFile
			cur.entry.Size = info.Size()
			cur.entry.FileCount = 1
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return materialize(root), nil
}

func materialize(b *dirBuilder) []Entry {
	out := make([]Entry, 0, len(b.children))
	for _, child := range b.children {
		e := child.entry
		if e.Kind == KindDir {
			e.Children = materialize(child)
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind == KindDir
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func aggregate(e *Entry) {
	if e.Kind == KindFile {
		return
	}
	var size int64
	var count int
	for i := range e.Children {
		aggregate(&e.Children[i])
		size += e.Children[i].Size
		count += e.Children[i].FileCount
	}
	e.Size = size
	e.FileCount = count
}

// CollectFiles returns absolute paths and relative paths for every file under e.
func CollectFiles(e Entry) []SendItem {
	var out []SendItem
	var walk func(Entry)
	walk = func(ent Entry) {
		if ent.Kind == KindFile {
			out = append(out, SendItem{Path: ent.Path, RelPath: ent.RelPath})
			return
		}
		for _, c := range ent.Children {
			walk(c)
		}
	}
	walk(e)
	return out
}

// SummarizeOffer builds receiver-facing stats from wire file metadata.
func SummarizeOffer(files []FileMetaLite) OfferSummary {
	dirSize := map[string]int64{}
	dirCount := map[string]int{}
	var loose int
	var total int64
	for _, f := range files {
		total += f.Size
		rel := filepath.ToSlash(f.RelPath)
		if rel == "" {
			rel = f.Name
		}
		if i := strings.Index(rel, "/"); i >= 0 {
			root := rel[:i]
			dirSize[root] += f.Size
			dirCount[root]++
		} else {
			loose++
		}
	}
	roots := make([]RootDirInfo, 0, len(dirSize))
	for name, size := range dirSize {
		roots = append(roots, RootDirInfo{Name: name, Size: size, FileCount: dirCount[name]})
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Name < roots[j].Name })
	return OfferSummary{
		RootDirs: roots, LooseFiles: loose,
		FileCount: len(files), TotalSize: total,
	}
}

// FileMetaLite is the minimal file fields needed for offer summaries.
type FileMetaLite struct {
	Name    string
	RelPath string
	Size    int64
}

// SelectedItems returns send items for the chosen absolute file paths.
func SelectedItems(entries []Entry, selectedFiles map[string]bool) ([]SendItem, error) {
	relByPath := map[string]string{}
	var walk func([]Entry)
	walk = func(list []Entry) {
		for _, e := range list {
			if e.Kind == KindFile {
				relByPath[e.Path] = e.RelPath
			} else {
				walk(e.Children)
			}
		}
	}
	walk(entries)

	var items []SendItem
	for path, on := range selectedFiles {
		if !on {
			continue
		}
		rel, ok := relByPath[path]
		if !ok {
			return nil, errors.New("неизвестный файл: " + path)
		}
		items = append(items, SendItem{Path: path, RelPath: rel})
	}
	if len(items) == 0 {
		return nil, errors.New("нет выбранных файлов")
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RelPath < items[j].RelPath })
	return items, nil
}
