package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skill-vendor/skv/internal/lock"
	"github.com/skill-vendor/skv/internal/spec"
)

func TestRunUpdateRefRequiresName(t *testing.T) {
	withTempDir(t, func(_ string) {
		err := runUpdate([]string{"--ref", "v1"})
		if err == nil || !strings.Contains(err.Error(), "--ref requires a skill name") {
			t.Fatalf("expected ref error, got %v", err)
		}
	})
}

func TestRunUpdateTagMovedRequiresForce(t *testing.T) {
	withTempDir(t, func(dir string) {
		repoDir := filepath.Join(dir, "skillrepo")
		initGitRepo(t, repoDir)
		writeFile(t, filepath.Join(repoDir, "SKILL.md"), "---\nname: skill-foo\ndescription: demo\n---\n")
		writeFile(t, filepath.Join(repoDir, "notes.txt"), "v1")
		gitCmd(t, repoDir, "add", ".")
		gitCmd(t, repoDir, "commit", "-m", "add skill")
		gitCmd(t, repoDir, "tag", "v1")

		repoURL := "file://" + repoDir
		if err := spec.Write(filepath.Join(dir, "skv.cue"), &spec.Spec{
			Skills: []spec.SkillEntry{{
				Name: "skill-foo",
				Repo: repoURL,
				Ref:  "v1",
			}},
		}); err != nil {
			t.Fatalf("write spec: %v", err)
		}
		if err := lock.Write(filepath.Join(dir, "skv.lock"), &lock.Lock{Skills: []lock.Skill{}}); err != nil {
			t.Fatalf("write lock: %v", err)
		}

		if err := runSync(nil); err != nil {
			t.Fatalf("sync: %v", err)
		}

		writeFile(t, filepath.Join(repoDir, "notes.txt"), "v2")
		gitCmd(t, repoDir, "add", ".")
		gitCmd(t, repoDir, "commit", "-m", "update skill")
		gitCmd(t, repoDir, "tag", "-f", "v1")

		err := runUpdate([]string{"skill-foo"})
		if err == nil || !strings.Contains(err.Error(), "tag \"v1\" moved") {
			t.Fatalf("expected tag moved error, got %v", err)
		}

		if err := runUpdate([]string{"--force", "skill-foo"}); err != nil {
			t.Fatalf("update with force: %v", err)
		}
	})
}

func TestRunSyncAcceptLocalRequiresLockForRemote(t *testing.T) {
	withTempDir(t, func(dir string) {
		if err := spec.Write(filepath.Join(dir, "skv.cue"), &spec.Spec{
			Skills: []spec.SkillEntry{{
				Name: "skill-foo",
				Repo: "https://example.com/repo",
				Path: "skill-foo",
			}},
		}); err != nil {
			t.Fatalf("write spec: %v", err)
		}

		err := runSync([]string{"--accept-local"})
		if err == nil || !strings.Contains(err.Error(), "accept-local requires existing lock entry") {
			t.Fatalf("expected accept-local lock error, got %v", err)
		}
	})
}

func TestRunSyncAcceptLocalRequiresVendorForRemote(t *testing.T) {
	withTempDir(t, func(dir string) {
		repo := "https://example.com/repo"
		if err := spec.Write(filepath.Join(dir, "skv.cue"), &spec.Spec{
			Skills: []spec.SkillEntry{{
				Name: "skill-foo",
				Repo: repo,
				Path: "skill-foo",
				Ref:  "main",
			}},
		}); err != nil {
			t.Fatalf("write spec: %v", err)
		}

		if err := lock.Write(filepath.Join(dir, "skv.lock"), &lock.Lock{Skills: []lock.Skill{{
			Name:   "skill-foo",
			Repo:   repo,
			Path:   "skill-foo",
			Ref:    "main",
			Commit: "deadbeef",
		}}}); err != nil {
			t.Fatalf("write lock: %v", err)
		}

		err := runSync([]string{"--accept-local"})
		if err == nil || !strings.Contains(err.Error(), "accept-local requires existing vendor") {
			t.Fatalf("expected accept-local vendor error, got %v", err)
		}
	})
}

func TestRunSyncAcceptLocalLocalSkillRequiresVendor(t *testing.T) {
	withTempDir(t, func(dir string) {
		if err := spec.Write(filepath.Join(dir, "skv.cue"), &spec.Spec{
			Skills: []spec.SkillEntry{{
				Name:  "local-skill",
				Local: "./local-skill",
			}},
		}); err != nil {
			t.Fatalf("write spec: %v", err)
		}
		if err := lock.Write(filepath.Join(dir, "skv.lock"), &lock.Lock{Skills: []lock.Skill{}}); err != nil {
			t.Fatalf("write lock: %v", err)
		}

		err := runSync([]string{"--accept-local"})
		if err == nil || !strings.Contains(err.Error(), "accept-local requires existing vendor") {
			t.Fatalf("expected accept-local vendor error, got %v", err)
		}
	})
}

func withTempDir(t *testing.T, fn func(dir string)) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()
	fn(dir)
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	gitCmd(t, dir, "-c", "init.defaultBranch=main", "init")
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), gitEnv()...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
}

func gitEnv() []string {
	return []string{
		"GIT_AUTHOR_NAME=TestUser",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=TestUser",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE=2000-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2000-01-01T00:00:00Z",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0",
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
