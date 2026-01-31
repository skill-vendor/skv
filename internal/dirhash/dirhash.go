package dirhash

import (
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
	var files []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
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
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(files)
	h := sha256.New()
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		fh := sha256.Sum256(data)
		line := fmt.Sprintf("%x  %s\n", fh, rel)
		h.Write([]byte(line))
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func NormalizePath(path string) string {
	return strings.ReplaceAll(path, string(filepath.Separator), "/")
}
