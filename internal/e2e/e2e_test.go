package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestE2E(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(cwd, "..", ".."))

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "skv")

	build := exec.Command("go", "build", "-o", binPath, "./cmd/skv")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, string(out))
	}

	params := testscript.Params{
		Dir:         filepath.Join(repoRoot, "internal", "e2e", "testdata"),
		WorkdirRoot: t.TempDir(),
		Setup: func(env *testscript.Env) error {
			path := binDir + string(os.PathListSeparator) + env.Getenv("PATH")
			env.Setenv("PATH", path)
			return nil
		},
	}

	testscript.Run(t, params)
}
