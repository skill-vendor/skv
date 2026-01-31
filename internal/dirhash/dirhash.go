package dirhash

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// HashDir computes a deterministic hash based on the hashes of files in the directory.
// It ignores .git directories and only considers regular files.
func HashDir(root string) (string, error) {
	return HashDirWithContext(context.Background(), root)
}

// HashDirWithContext is like HashDir but allows cancellation.
func HashDirWithContext(ctx context.Context, root string) (string, error) {
	type entry struct {
		path string
		mode fs.FileMode
	}
	var files []entry

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		files = append(files, entry{path: rel, mode: info.Mode().Perm()})
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	h := sha256.New()
	for _, entry := range files {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.path)))
		if err != nil {
			return "", err
		}
		fh := sha256.Sum256(data)
		line := fmt.Sprintf("%x  %o  %s\n", fh, entry.mode, entry.path)
		h.Write([]byte(line))
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func NormalizePath(path string) string {
	return strings.ReplaceAll(path, string(filepath.Separator), "/")
}
