package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/diff"
	"github.com/rogpeppe/go-internal/testscript"

	"github.com/skill-vendor/skv/internal/dirhash"
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
			env.Setenv("GIT_AUTHOR_NAME", "TestUser")
			env.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
			env.Setenv("GIT_COMMITTER_NAME", "TestUser")
			env.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
			env.Setenv("GIT_AUTHOR_DATE", "2000-01-01T00:00:00Z")
			env.Setenv("GIT_COMMITTER_DATE", "2000-01-01T00:00:00Z")
			return nil
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"lockcmp": cmdLockCmp,
			"render":  cmdRender,
		},
	}

	testscript.Run(t, params)
}

func cmdLockCmp(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("unsupported: ! lockcmp")
	}
	if len(args) < 2 {
		ts.Fatalf("usage: lockcmp lockfile template [key=value...]")
	}

	lockName := args[0]
	templateName := args[1]
	lockPath := ts.MkAbs(lockName)
	templatePath := ts.MkAbs(templateName)

	templateData, err := os.ReadFile(templatePath)
	ts.Check(err)
	expected := string(templateData)

	values := map[string]string{}
	var repoPath string
	var commitPath string
	haveRepo := false
	haveCommit := false

	for _, arg := range args[2:] {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			ts.Fatalf("lockcmp arg must be key=value: %q", arg)
		}
		switch {
		case key == "repo":
			repoPath = value
			haveRepo = true
		case key == "commit":
			commitPath = value
			haveCommit = true
		case key == "local":
			values["__LOCAL__"] = value
		case key == "checksum":
			values["__CHECKSUM__"] = mustHashDir(ts, value)
		case strings.HasPrefix(key, "checksum."):
			name := strings.TrimPrefix(key, "checksum.")
			if name == "" {
				ts.Fatalf("lockcmp checksum key must include a name: %q", key)
			}
			token := "__CHECKSUM_" + strings.ToUpper(name) + "__"
			values[token] = mustHashDir(ts, value)
		default:
			ts.Fatalf("unknown lockcmp key %q", key)
		}
	}

	if haveRepo {
		values["__REPO__"] = repoURL(ts, repoPath)
		if !haveCommit {
			commitPath = repoPath
			haveCommit = true
		}
	}
	if haveCommit {
		values["__COMMIT__"] = gitHead(ts, commitPath)
	}

	for token, value := range values {
		expected = strings.ReplaceAll(expected, token, value)
	}

	actual, err := os.ReadFile(lockPath)
	ts.Check(err)
	if string(actual) == expected {
		return
	}

	unifiedDiff := diff.Diff(lockName, actual, templateName, []byte(expected))
	ts.Logf("%s", unifiedDiff)
	ts.Fatalf("%s and %s differ", lockName, templateName)
}

func cmdRender(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("unsupported: ! render")
	}
	if len(args) < 2 {
		ts.Fatalf("usage: render template output [key=value...]")
	}

	templatePath := ts.MkAbs(args[0])
	outputPath := ts.MkAbs(args[1])

	templateData, err := os.ReadFile(templatePath)
	ts.Check(err)
	rendered := string(templateData)

	for _, arg := range args[2:] {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			ts.Fatalf("render arg must be key=value: %q", arg)
		}
		token := "__" + strings.ToUpper(key) + "__"
		if key == "repo" {
			value = repoURL(ts, value)
		}
		rendered = strings.ReplaceAll(rendered, token, value)
	}

	ts.Check(os.WriteFile(outputPath, []byte(rendered), 0o644))
}

func mustHashDir(ts *testscript.TestScript, dir string) string {
	sum, err := dirhash.HashDir(ts.MkAbs(dir))
	ts.Check(err)
	return sum
}

func repoURL(ts *testscript.TestScript, repo string) string {
	if strings.HasPrefix(repo, "file://") {
		return repo
	}
	abs := ts.MkAbs(repo)
	return "file://" + filepath.ToSlash(abs)
}

func gitHead(ts *testscript.TestScript, repo string) string {
	repoPath := repo
	if strings.HasPrefix(repoPath, "file://") {
		repoPath = strings.TrimPrefix(repoPath, "file://")
	}
	repoPath = ts.MkAbs(repoPath)
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			ts.Fatalf("git rev-parse HEAD failed: %s", msg)
		}
		ts.Fatalf("git rev-parse HEAD failed: %v", err)
	}
	return strings.TrimSpace(string(out))
}
