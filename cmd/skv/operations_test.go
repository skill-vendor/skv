package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/skill-vendor/skv/internal/lock"
	"github.com/skill-vendor/skv/internal/spec"
)

func TestUpdateRefRequiresName(t *testing.T) {
	withTempDir(t, func(_ string) {
		err := runUpdate("", updateOptions{ref: "v1"})
		if err == nil || !strings.Contains(err.Error(), "--ref requires a skill name") {
			t.Fatalf("expected ref error, got %v", err)
		}
	})
}

func TestUpdateTagMovedRequiresForce(t *testing.T) {
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

		if err := runSync(syncOptions{}); err != nil {
			t.Fatalf("sync: %v", err)
		}

		writeFile(t, filepath.Join(repoDir, "notes.txt"), "v2")
		gitCmd(t, repoDir, "add", ".")
		gitCmd(t, repoDir, "commit", "-m", "update skill")
		gitCmd(t, repoDir, "tag", "-f", "v1")

		err := runUpdate("skill-foo", updateOptions{})
		if err == nil || !strings.Contains(err.Error(), "tag \"v1\" moved") {
			t.Fatalf("expected tag moved error, got %v", err)
		}

		if err := runUpdate("skill-foo", updateOptions{force: true}); err != nil {
			t.Fatalf("update with force: %v", err)
		}
	})
}

func TestSyncAcceptLocalRequiresLockForRemote(t *testing.T) {
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

		err := runSync(syncOptions{acceptLocal: true})
		if err == nil || !strings.Contains(err.Error(), "accept-local requires existing lock entry") {
			t.Fatalf("expected accept-local lock error, got %v", err)
		}
	})
}

func TestSyncAcceptLocalRequiresVendorForRemote(t *testing.T) {
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

		err := runSync(syncOptions{acceptLocal: true})
		if err == nil || !strings.Contains(err.Error(), "accept-local requires existing vendor") {
			t.Fatalf("expected accept-local vendor error, got %v", err)
		}
	})
}

func TestSyncAcceptLocalLocalSkillRequiresVendor(t *testing.T) {
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

		err := runSync(syncOptions{acceptLocal: true})
		if err == nil || !strings.Contains(err.Error(), "accept-local requires existing vendor") {
			t.Fatalf("expected accept-local vendor error, got %v", err)
		}
	})
}
