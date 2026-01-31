package dirhash

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHashDirIgnoresGit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	hash1, err := HashDir(dir)
	if err != nil {
		t.Fatalf("hash dir: %v", err)
	}

	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("write git file: %v", err)
	}

	hash2, err := HashDir(dir)
	if err != nil {
		t.Fatalf("hash dir: %v", err)
	}
	if hash1 != hash2 {
		t.Fatalf("expected hash to ignore .git; got %q and %q", hash1, hash2)
	}
}

func TestHashDirDetectsContentChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	hash1, err := HashDir(dir)
	if err != nil {
		t.Fatalf("hash dir: %v", err)
	}

	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	hash2, err := HashDir(dir)
	if err != nil {
		t.Fatalf("hash dir: %v", err)
	}

	if hash1 == hash2 {
		t.Fatalf("expected hash to change when content changes")
	}
}

func TestHashDirDetectsModeChange(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode bits are not stable on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "a.sh")
	if err := os.WriteFile(path, []byte("echo ok"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	hash1, err := HashDir(dir)
	if err != nil {
		t.Fatalf("hash dir: %v", err)
	}

	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	hash2, err := HashDir(dir)
	if err != nil {
		t.Fatalf("hash dir: %v", err)
	}

	if hash1 == hash2 {
		t.Fatalf("expected hash to change when mode changes")
	}
}
