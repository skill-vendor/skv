package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/skill-vendor/skv/internal/dirhash"
	"github.com/skill-vendor/skv/internal/fsutil"
	"github.com/skill-vendor/skv/internal/lock"
	"github.com/skill-vendor/skv/internal/spec"
)

const (
	maxCheckoutBytes = 50 * 1024 * 1024
	maxSkillBytes    = 20 * 1024 * 1024
	maxSkillFiles    = 5000
	maxFileBytes     = 5 * 1024 * 1024

	gitTimeout  = 2 * time.Minute
	hashTimeout = 30 * time.Second
)

func loadLockOptional(path string) (*lock.Lock, map[string]lock.Skill, error) {
	lockData, err := lock.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &lock.Lock{Skills: []lock.Skill{}}, map[string]lock.Skill{}, nil
		}
		return nil, nil, err
	}
	return lockData, indexLock(lockData), nil
}

func loadLockRequired(path string) (*lock.Lock, map[string]lock.Skill, error) {
	lockData, err := lock.Load(path)
	if err != nil {
		return nil, nil, err
	}
	return lockData, indexLock(lockData), nil
}

func indexLock(lockData *lock.Lock) map[string]lock.Skill {
	index := make(map[string]lock.Skill, len(lockData.Skills))
	for _, skill := range lockData.Skills {
		index[skill.Name] = skill
	}
	return index
}

func buildExcluded(specData *spec.Spec) map[string]struct{} {
	excluded := make(map[string]struct{})
	if specData.Tools != nil {
		for _, tool := range specData.Tools.Exclude {
			excluded[strings.ToLower(tool)] = struct{}{}
		}
	}
	return excluded
}

func verifyOffline(specData *spec.Spec, lockData *lock.Lock, repoRoot string, excluded map[string]struct{}) error {
	lockMap := indexLock(lockData)
	seen := make(map[string]struct{})
	for _, skill := range specData.Skills {
		if skill.Name == "" {
			return fmt.Errorf("skill missing name")
		}
		if _, dup := seen[skill.Name]; dup {
			return fmt.Errorf("duplicate skill name %q", skill.Name)
		}
		seen[skill.Name] = struct{}{}

		entry, ok := lockMap[skill.Name]
		if !ok {
			return fmt.Errorf("offline mode requires existing lock entry for %q", skill.Name)
		}
		if skill.Local != "" {
			if entry.Local == "" || entry.Local != skill.Local {
				return fmt.Errorf("offline mode requires lock entry for local skill %q to match spec", skill.Name)
			}
		} else {
			if entry.Repo != skill.Repo || entry.Path != skill.Path || entry.Ref != skill.Ref {
				return fmt.Errorf("offline mode requires lock entry for %q to match spec", skill.Name)
			}
		}
		if err := verifySkill(entry, repoRoot); err != nil {
			return err
		}
		if err := linkSkill(repoRoot, skill.Name, excluded); err != nil {
			return err
		}
	}
	return nil
}

func verifyLock(lockData *lock.Lock, repoRoot string) error {
	seen := make(map[string]struct{})
	for _, skill := range lockData.Skills {
		if skill.Name == "" {
			return fmt.Errorf("lock entry missing name")
		}
		if _, dup := seen[skill.Name]; dup {
			return fmt.Errorf("duplicate lock entry %q", skill.Name)
		}
		seen[skill.Name] = struct{}{}

		if err := verifySkill(skill, repoRoot); err != nil {
			return err
		}
	}
	return nil
}

func verifySkill(entry lock.Skill, repoRoot string) error {
	vendorPath := filepath.Join(repoRoot, ".skv", "skills", entry.Name)
	if err := ensureSkill(vendorPath); err != nil {
		return err
	}
	if err := validateSkillDir(vendorPath); err != nil {
		return err
	}
	checksum, err := hashDirWithTimeout(vendorPath)
	if err != nil {
		return err
	}
	if checksum != entry.Checksum {
		return fmt.Errorf("vendored content mismatch for %q (expected %s, got %s)", entry.Name, entry.Checksum, checksum)
	}
	return nil
}

func syncLocalSkill(repoRoot string, skill spec.SkillEntry, opts syncOptions, lockMap map[string]lock.Skill) (lock.Skill, error) {
	if skill.Local == "" {
		return lock.Skill{}, fmt.Errorf("local skill %q missing local path", skill.Name)
	}
	if skill.Repo != "" {
		return lock.Skill{}, fmt.Errorf("local skill %q should not include repo", skill.Name)
	}

	srcPath, err := resolveLocalPath(repoRoot, skill.Local)
	if err != nil {
		return lock.Skill{}, err
	}
	vendorPath := filepath.Join(repoRoot, ".skv", "skills", skill.Name)

	if opts.acceptLocal {
		if _, err := os.Stat(vendorPath); err != nil {
			return lock.Skill{}, fmt.Errorf("accept-local requires existing vendor for %q", skill.Name)
		}
		if err := ensureSkill(vendorPath); err != nil {
			return lock.Skill{}, err
		}
		if err := validateSkillDir(vendorPath); err != nil {
			return lock.Skill{}, err
		}
		checksum, err := hashDirWithTimeout(vendorPath)
		if err != nil {
			return lock.Skill{}, err
		}
		license := detectLicense(vendorPath, repoRoot)
		return lock.Skill{
			Name:     skill.Name,
			Local:    skill.Local,
			Checksum: checksum,
			License:  license,
		}, nil
	}

	if err := ensureSkill(srcPath); err != nil {
		return lock.Skill{}, err
	}
	if err := validateSkillDir(srcPath); err != nil {
		return lock.Skill{}, err
	}

	same, err := samePath(srcPath, vendorPath)
	if err != nil {
		return lock.Skill{}, err
	}
	if !same {
		if err := copyDirAtomic(srcPath, vendorPath); err != nil {
			return lock.Skill{}, err
		}
	}

	if err := validateSkillDir(vendorPath); err != nil {
		return lock.Skill{}, err
	}
	checksum, err := hashDirWithTimeout(vendorPath)
	if err != nil {
		return lock.Skill{}, err
	}
	license := detectLicense(vendorPath, repoRoot)
	return lock.Skill{
		Name:     skill.Name,
		Local:    skill.Local,
		Checksum: checksum,
		License:  license,
	}, nil
}

func syncRemoteSkill(repoRoot string, skill spec.SkillEntry, opts syncOptions, lockMap map[string]lock.Skill) (lock.Skill, error) {
	if skill.Repo == "" {
		return lock.Skill{}, fmt.Errorf("skill %q missing repo", skill.Name)
	}
	if skill.Local != "" {
		return lock.Skill{}, fmt.Errorf("skill %q cannot set both repo and local", skill.Name)
	}
	cleanPath, err := cleanSubpath(skill.Path)
	if err != nil {
		return lock.Skill{}, err
	}
	skill.Path = cleanPath

	vendorPath := filepath.Join(repoRoot, ".skv", "skills", skill.Name)
	existing, hasLock := lockMap[skill.Name]

	if opts.acceptLocal {
		if !hasLock {
			return lock.Skill{}, fmt.Errorf("accept-local requires existing lock entry for %q", skill.Name)
		}
		if !lockMatchesSpec(existing, skill) {
			return lock.Skill{}, fmt.Errorf("accept-local requires lock entry for %q to match spec", skill.Name)
		}
		if _, err := os.Stat(vendorPath); err != nil {
			return lock.Skill{}, fmt.Errorf("accept-local requires existing vendor for %q", skill.Name)
		}
		if err := ensureSkill(vendorPath); err != nil {
			return lock.Skill{}, err
		}
		if err := validateSkillDir(vendorPath); err != nil {
			return lock.Skill{}, err
		}
		checksum, err := hashDirWithTimeout(vendorPath)
		if err != nil {
			return lock.Skill{}, err
		}
		license := detectLicense(vendorPath, vendorPath)
		if license == nil {
			license = existing.License
		}
		return lock.Skill{
			Name:     skill.Name,
			Repo:     skill.Repo,
			Path:     skill.Path,
			Ref:      skill.Ref,
			Commit:   existing.Commit,
			Checksum: checksum,
			License:  license,
		}, nil
	}

	if !opts.refresh && hasLock && lockMatchesSpec(existing, skill) {
		if err := ensureSkill(vendorPath); err != nil {
			return lock.Skill{}, fmt.Errorf("vendored content for %q is missing; use --refresh or --accept-local", skill.Name)
		}
		if err := validateSkillDir(vendorPath); err != nil {
			return lock.Skill{}, err
		}
		checksum, err := hashDirWithTimeout(vendorPath)
		if err != nil {
			return lock.Skill{}, err
		}
		if checksum == existing.Checksum {
			return existing, nil
		}
		return lock.Skill{}, fmt.Errorf("vendored content for %q differs from lock; use --refresh or --accept-local", skill.Name)
	}

	return fetchAndVendorRemote(repoRoot, skill, vendorPath)
}

func updateRemoteSkill(repoRoot string, skill spec.SkillEntry, lockMap map[string]lock.Skill, force bool) (lock.Skill, error) {
	cleanPath, err := cleanSubpath(skill.Path)
	if err != nil {
		return lock.Skill{}, err
	}
	skill.Path = cleanPath

	existing, hasLock := lockMap[skill.Name]
	cloneDir, err := cloneRepo(skill.Repo, skill.Ref, skill.Path)
	if err != nil {
		return lock.Skill{}, err
	}
	defer os.RemoveAll(cloneDir)

	if err := validateCheckoutSize(cloneDir); err != nil {
		return lock.Skill{}, err
	}

	commit, err := gitHead(cloneDir)
	if err != nil {
		return lock.Skill{}, err
	}

	if skill.Ref != "" {
		isTag, err := gitIsTag(cloneDir, skill.Ref)
		if err != nil {
			return lock.Skill{}, err
		}
		if isTag && hasLock && existing.Commit != "" && existing.Commit != commit && !force {
			return lock.Skill{}, fmt.Errorf("tag %q moved for %q; re-run with --force to accept", skill.Ref, skill.Name)
		}
	}

	srcPath := cloneDir
	if skill.Path != "" {
		srcPath = filepath.Join(cloneDir, skill.Path)
	}
	if err := ensureSkill(srcPath); err != nil {
		return lock.Skill{}, err
	}
	if err := validateSkillDir(srcPath); err != nil {
		return lock.Skill{}, err
	}

	vendorPath := filepath.Join(repoRoot, ".skv", "skills", skill.Name)
	if err := copyDirAtomic(srcPath, vendorPath); err != nil {
		return lock.Skill{}, err
	}
	if err := validateSkillDir(vendorPath); err != nil {
		return lock.Skill{}, err
	}

	checksum, err := hashDirWithTimeout(vendorPath)
	if err != nil {
		return lock.Skill{}, err
	}

	license := detectLicense(srcPath, cloneDir)
	return lock.Skill{
		Name:     skill.Name,
		Repo:     skill.Repo,
		Path:     skill.Path,
		Ref:      skill.Ref,
		Commit:   commit,
		Checksum: checksum,
		License:  license,
	}, nil
}

func fetchAndVendorRemote(repoRoot string, skill spec.SkillEntry, vendorPath string) (lock.Skill, error) {
	cloneDir, err := cloneRepo(skill.Repo, skill.Ref, skill.Path)
	if err != nil {
		return lock.Skill{}, err
	}
	defer os.RemoveAll(cloneDir)

	if err := validateCheckoutSize(cloneDir); err != nil {
		return lock.Skill{}, err
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
	if err := validateSkillDir(srcPath); err != nil {
		return lock.Skill{}, err
	}

	if err := copyDirAtomic(srcPath, vendorPath); err != nil {
		return lock.Skill{}, err
	}
	if err := validateSkillDir(vendorPath); err != nil {
		return lock.Skill{}, err
	}

	checksum, err := hashDirWithTimeout(vendorPath)
	if err != nil {
		return lock.Skill{}, err
	}

	license := detectLicense(srcPath, cloneDir)
	return lock.Skill{
		Name:     skill.Name,
		Repo:     skill.Repo,
		Path:     skill.Path,
		Ref:      skill.Ref,
		Commit:   commit,
		Checksum: checksum,
		License:  license,
	}, nil
}

func ensureRepoHasSkill(repo, ref string) error {
	cloneDir, err := cloneRepo(repo, ref, "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cloneDir)

	if err := validateCheckoutSize(cloneDir); err != nil {
		return err
	}

	if err := ensureSkill(cloneDir); err != nil {
		if strings.Contains(err.Error(), "missing SKILL.md") {
			return fmt.Errorf("repo root missing SKILL.md; specify a :path")
		}
		return err
	}
	return nil
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

func validateSkillDir(root string) error {
	var total int64
	var count int
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed: %s", path)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		count++
		if count > maxSkillFiles {
			return fmt.Errorf("skill exceeds max file count (%d)", maxSkillFiles)
		}
		if info.Size() > maxFileBytes {
			return fmt.Errorf("file %s exceeds max size (%d bytes)", path, maxFileBytes)
		}
		total += info.Size()
		if total > maxSkillBytes {
			return fmt.Errorf("skill exceeds max size (%d bytes)", maxSkillBytes)
		}
		isLFS, err := isLFSPointer(path)
		if err != nil {
			return err
		}
		if isLFS {
			return fmt.Errorf("git lfs pointer detected: %s", path)
		}
		return nil
	})
}

func validateCheckoutSize(root string) error {
	var total int64
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		total += info.Size()
		if total > maxCheckoutBytes {
			return fmt.Errorf("checkout exceeds max size (%d bytes)", maxCheckoutBytes)
		}
		return nil
	})
}

func isLFSPointer(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	buf := make([]byte, 200)
	n, err := file.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	content := string(buf[:n])
	return strings.HasPrefix(content, "version https://git-lfs.github.com/spec/v1"), nil
}

func hashDirWithTimeout(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hashTimeout)
	defer cancel()
	return dirhash.HashDirWithContext(ctx, path)
}

func detectLicense(skillDir, repoDir string) *lock.License {
	candidates := []string{"LICENSE", "LICENSE.txt", "COPYING", "NOTICE"}
	path := findFirstFile(skillDir, candidates)
	if path == "" {
		path = findFirstFile(repoDir, candidates)
	}
	if path == "" {
		for _, name := range candidates {
			content, ok, err := gitShowFile(repoDir, name)
			if err != nil {
				return nil
			}
			if ok {
				return &lock.License{
					SPDX: detectSPDXText(content),
					Path: dirhash.NormalizePath(name),
				}
			}
		}
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
	return detectSPDXText(string(data))
}

func detectSPDXText(text string) string {
	if strings.Contains(text, "MIT License") {
		return "MIT"
	}
	if strings.Contains(text, "Apache License") {
		return "Apache-2.0"
	}
	if strings.Contains(text, "BSD 3-Clause") {
		return "BSD-3-Clause"
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
	if info, err := os.Lstat(linkPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || info.Mode().IsRegular() {
			if err := os.Remove(linkPath); err != nil {
				return err
			}
		} else if info.IsDir() {
			return fmt.Errorf("refusing to replace directory %s", linkPath)
		} else if err := os.Remove(linkPath); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
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

func gitCheckout(dir, ref string) error {
	return runGitCommand(dir, "checkout", ref)
}

func gitHead(dir string) (string, error) {
	out, err := runGitCommandOutput(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitIsTag(dir, ref string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "-q", "--verify", "refs/tags/"+ref)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return false, fmt.Errorf("git rev-parse timed out")
	}
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, fmt.Errorf("git rev-parse failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return true, nil
}

func runGitCommand(dir string, args ...string) error {
	_, err := runGitCommandOutput(dir, args...)
	return err
}

func runGitCommandOutput(dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmdArgs := append([]string{}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("git %s timed out", strings.Join(args, " "))
	}
	if err != nil {
		return nil, fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func cloneRepo(repo, ref, sparsePath string) (string, error) {
	cloneDir, err := os.MkdirTemp("", "skv-clone-*")
	if err != nil {
		return "", err
	}

	if err := runGitCommand("", "clone", "--no-checkout", repo, cloneDir); err != nil {
		_ = os.RemoveAll(cloneDir)
		return "", err
	}
	if sparsePath != "" {
		if err := gitSparseCheckout(cloneDir, sparsePath); err != nil {
			_ = os.RemoveAll(cloneDir)
			return "", err
		}
	}

	checkoutRef := ref
	if checkoutRef == "" {
		defaultBranch, err := gitDefaultBranch(cloneDir)
		if err == nil && defaultBranch != "" {
			checkoutRef = defaultBranch
		} else {
			checkoutRef = "HEAD"
		}
	}
	if err := gitCheckout(cloneDir, checkoutRef); err != nil {
		_ = os.RemoveAll(cloneDir)
		return "", err
	}
	return cloneDir, nil
}

func gitDefaultBranch(dir string) (string, error) {
	out, err := runGitCommandOutput(dir, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(out))
	ref = strings.TrimPrefix(ref, "refs/remotes/origin/")
	return ref, nil
}

func gitSparseCheckout(dir, path string) error {
	clean := filepath.ToSlash(path)
	if err := runGitCommand(dir, "sparse-checkout", "init", "--cone"); err != nil {
		return err
	}
	return runGitCommand(dir, "sparse-checkout", "set", clean)
}

func gitShowFile(dir, path string) (string, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "show", "HEAD:"+path)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", false, fmt.Errorf("git show timed out")
	}
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return "", false, nil
		}
		return "", false, fmt.Errorf("git show failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), true, nil
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

func cleanSubpath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be relative: %s", path)
	}
	cleaned := filepath.Clean(path)
	if cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes repo: %s", path)
	}
	return cleaned, nil
}

func resolveLocalPath(repoRoot, local string) (string, error) {
	if local == "" {
		return "", fmt.Errorf("local path is required")
	}
	abs := local
	if !filepath.IsAbs(local) {
		abs = filepath.Join(repoRoot, local)
	}
	abs = filepath.Clean(abs)
	rel, err := filepath.Rel(repoRoot, abs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("local path escapes repo: %s", local)
	}
	return abs, nil
}

func lockMatchesSpec(entry lock.Skill, skill spec.SkillEntry) bool {
	return entry.Repo == skill.Repo && entry.Path == skill.Path && entry.Ref == skill.Ref && entry.Local == skill.Local
}

func samePath(a, b string) (bool, error) {
	absA, err := filepath.Abs(a)
	if err != nil {
		return false, err
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		return false, err
	}
	return absA == absB, nil
}

func copyDirAtomic(src, dst string) error {
	parent := filepath.Dir(dst)
	tmp, err := os.MkdirTemp(parent, ".skv-tmp-")
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmp)
		}
	}()

	if err := fsutil.CopyDir(src, tmp); err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func isCommitRef(ref string) bool {
	if len(ref) != 40 {
		return false
	}
	for _, r := range ref {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func findSkill(specData *spec.Spec, name string) (spec.SkillEntry, bool) {
	for _, skill := range specData.Skills {
		if skill.Name == name {
			return skill, true
		}
	}
	return spec.SkillEntry{}, false
}
