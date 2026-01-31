package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/skill-vendor/skv/internal/dirhash"
	"github.com/skill-vendor/skv/internal/fsutil"
	"github.com/skill-vendor/skv/internal/lock"
	"github.com/skill-vendor/skv/internal/spec"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	switch cmd {
	case "init":
		if err := runInit(); err != nil {
			fatal(err)
		}
	case "add":
		if err := runAdd(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "sync":
		if err := runSync(os.Args[2:]); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: skv <init|add|sync>")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "skv:", err)
	os.Exit(1)
}

func runInit() error {
	if _, err := os.Stat("skv.cue"); err == nil {
		return fmt.Errorf("skv.cue already exists")
	}
	if err := fsutil.EnsureDir(filepath.Join(".skv", "skills")); err != nil {
		return err
	}
	if err := spec.Write("skv.cue", &spec.Spec{Skills: []spec.SkillEntry{}}); err != nil {
		return err
	}
	if err := lock.Write("skv.lock", &lock.Lock{Skills: []lock.Skill{}}); err != nil {
		return err
	}
	return nil
}

func runAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	nameFlag := fs.String("name", "", "override skill name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("add requires <repo>[#ref][:path]")
	}

	repo, ref, path := parseRepoArg(fs.Arg(0))
	if repo == "" {
		return fmt.Errorf("invalid repo argument")
	}

	name := *nameFlag
	if name == "" {
		if path != "" {
			name = filepath.Base(path)
		} else {
			name = deriveName(repo)
		}
	}

	entry := spec.SkillEntry{
		Name: name,
		Repo: repo,
		Path: path,
		Ref:  ref,
	}
	return spec.AppendSkill("skv.cue", entry)
}

func runSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	_ = fs.Bool("offline", false, "offline mode (not implemented)")
	_ = fs.Bool("refresh", false, "refresh mode (not implemented)")
	_ = fs.Bool("accept-local", false, "accept local mode (not implemented)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	specData, err := spec.Load("skv.cue")
	if err != nil {
		return err
	}

	if err := fsutil.EnsureDir(filepath.Join(".skv", "skills")); err != nil {
		return err
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	excluded := make(map[string]struct{})
	if specData.Tools != nil {
		for _, tool := range specData.Tools.Exclude {
			excluded[strings.ToLower(tool)] = struct{}{}
		}
	}

	var lockSkills []lock.Skill
	for _, skill := range specData.Skills {
		if skill.Name == "" {
			return fmt.Errorf("skill missing name")
		}
		if skill.Local != "" {
			return fmt.Errorf("local skills not supported yet")
		}
		if skill.Repo == "" {
			return fmt.Errorf("skill %q missing repo", skill.Name)
		}

		entry, err := syncSkill(repoRoot, skill)
		if err != nil {
			return err
		}
		lockSkills = append(lockSkills, entry)

		if err := linkSkill(repoRoot, skill.Name, excluded); err != nil {
			return err
		}
	}

	sort.Slice(lockSkills, func(i, j int) bool { return lockSkills[i].Name < lockSkills[j].Name })
	return lock.Write("skv.lock", &lock.Lock{Skills: lockSkills})
}

func syncSkill(repoRoot string, skill spec.SkillEntry) (lock.Skill, error) {
	cloneDir, err := os.MkdirTemp("", "skv-clone-*")
	if err != nil {
		return lock.Skill{}, err
	}
	defer os.RemoveAll(cloneDir)

	if err := gitClone(skill.Repo, cloneDir); err != nil {
		return lock.Skill{}, err
	}
	if skill.Ref != "" {
		if err := gitCheckout(cloneDir, skill.Ref); err != nil {
			return lock.Skill{}, err
		}
	}
	commit, err := gitHead(cloneDir)
	if err != nil {
		return lock.Skill{}, err
	}

	srcPath := cloneDir
	if skill.Path != "" {
		srcPath = filepath.Join(cloneDir, skill.Path)
	}
	if err := ensureSkill(srcPath); err != nil {
		return lock.Skill{}, err
	}

	vendorPath := filepath.Join(repoRoot, ".skv", "skills", skill.Name)
	if err := os.RemoveAll(vendorPath); err != nil {
		return lock.Skill{}, err
	}
	if err := fsutil.CopyDir(srcPath, vendorPath); err != nil {
		return lock.Skill{}, err
	}

	checksum, err := dirhash.HashDir(vendorPath)
	if err != nil {
		return lock.Skill{}, err
	}

	license := detectLicense(srcPath, cloneDir)

	entry := lock.Skill{
		Name:     skill.Name,
		Repo:     skill.Repo,
		Path:     skill.Path,
		Ref:      skill.Ref,
		Commit:   commit,
		Checksum: checksum,
		License:  license,
	}
	return entry, nil
}

func ensureSkill(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("skill path is not a directory: %s", path)
	}
	if _, err := os.Stat(filepath.Join(path, "SKILL.md")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("missing SKILL.md in %s", path)
		}
		return err
	}
	return nil
}

func detectLicense(skillDir, repoDir string) *lock.License {
	candidates := []string{"LICENSE", "LICENSE.txt", "COPYING", "NOTICE"}
	path := findFirstFile(skillDir, candidates)
	if path == "" {
		path = findFirstFile(repoDir, candidates)
	}
	if path == "" {
		return nil
	}
	spdx := detectSPDX(path)
	rel, err := filepath.Rel(repoDir, path)
	if err != nil {
		rel = path
	}
	return &lock.License{
		SPDX: spdx,
		Path: dirhash.NormalizePath(rel),
	}
}

func findFirstFile(dir string, names []string) string {
	for _, name := range names {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return ""
}

func detectSPDX(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if strings.Contains(string(data), "MIT License") {
		return "MIT"
	}
	return ""
}

func linkSkill(repoRoot, name string, excluded map[string]struct{}) error {
	target := filepath.Join(repoRoot, ".skv", "skills", name)
	links := map[string]string{
		"claude":   filepath.Join(repoRoot, ".claude", "skills", name),
		"codex":    filepath.Join(repoRoot, ".codex", "skills", name),
		"opencode": filepath.Join(repoRoot, ".opencode", "skill", name),
	}

	for tool, linkPath := range links {
		if _, skip := excluded[tool]; skip {
			continue
		}
		if err := ensureLink(target, linkPath); err != nil {
			return err
		}
	}
	return nil
}

func ensureLink(target, linkPath string) error {
	if err := os.RemoveAll(linkPath); err != nil {
		return err
	}
	if err := fsutil.EnsureDir(filepath.Dir(linkPath)); err != nil {
		return err
	}

	rel, err := filepath.Rel(filepath.Dir(linkPath), target)
	if err != nil {
		rel = target
	}
	return os.Symlink(rel, linkPath)
}

func gitClone(repo, dir string) error {
	cmd := exec.Command("git", "clone", repo, dir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitCheckout(dir, ref string) error {
	cmd := exec.Command("git", "-C", dir, "checkout", ref)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitHead(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func parseRepoArg(arg string) (repo, ref, path string) {
	repo = arg
	if hash := strings.Index(arg, "#"); hash != -1 {
		repo = arg[:hash]
		refPath := arg[hash+1:]
		ref, path = splitRefPath(refPath)
		return repo, ref, path
	}
	repo, path = splitRepoPath(repo)
	return repo, "", path
}

func splitRefPath(refPath string) (ref string, path string) {
	if colon := splitColon(refPath); colon != -1 {
		ref = refPath[:colon]
		path = refPath[colon+1:]
		return ref, path
	}
	return refPath, ""
}

func splitRepoPath(repo string) (string, string) {
	if colon := splitColon(repo); colon != -1 {
		return repo[:colon], repo[colon+1:]
	}
	return repo, ""
}

func splitColon(value string) int {
	if value == "" {
		return -1
	}
	idx := strings.LastIndex(value, ":")
	if idx == -1 {
		return -1
	}
	if strings.Contains(value[:idx], "://") {
		return idx
	}
	return idx
}

func deriveName(repo string) string {
	trimmed := strings.TrimSuffix(repo, "/")
	if idx := strings.LastIndex(trimmed, "/"); idx != -1 {
		trimmed = trimmed[idx+1:]
	}
	return strings.TrimSuffix(trimmed, ".git")
}
