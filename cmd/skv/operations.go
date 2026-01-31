package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/skill-vendor/skv/internal/fsutil"
	"github.com/skill-vendor/skv/internal/lock"
	"github.com/skill-vendor/skv/internal/spec"
)

type addOptions struct {
	name   string
	noSync bool
}

type listOptions struct {
	json  bool
	names bool
}

type syncOptions struct {
	offline     bool
	refresh     bool
	acceptLocal bool
}

type updateOptions struct {
	all   bool
	ref   string
	force bool
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
	globalOutput.Success("Initialized skv in current directory")
	return nil
}

func runAdd(repoArg string, opts addOptions) error {
	if repoArg == "" {
		return usageErrorf("add requires <repo>[#ref][:path]")
	}

	repo, ref, path := parseRepoArg(repoArg)
	if repo == "" {
		return fmt.Errorf("invalid repo argument")
	}
	var err error
	path, err = cleanSubpath(path)
	if err != nil {
		return err
	}

	name := opts.name
	if name == "" {
		if path != "" {
			name = filepath.Base(path)
		} else {
			name = deriveName(repo)
		}
	}

	specData, err := spec.Load("skv.cue")
	if err != nil {
		return err
	}
	for _, skill := range specData.Skills {
		if skill.Name == name {
			return fmt.Errorf("skill %q already exists", name)
		}
	}

	if path == "" {
		if err := ensureRepoHasSkill(repo, ref); err != nil {
			return err
		}
	}

	entry := spec.SkillEntry{
		Name: name,
		Repo: repo,
		Path: path,
		Ref:  ref,
	}

	specData.Skills = append(specData.Skills, entry)
	if err := spec.Write("skv.cue", specData); err != nil {
		return err
	}
	globalOutput.Success("Added %s from %s", name, repo)

	if opts.noSync {
		return nil
	}

	// Auto-sync the newly added skill
	return runSyncSingle(entry)
}

func runSync(opts syncOptions) error {
	if opts.offline && (opts.refresh || opts.acceptLocal) {
		return usageErrorf("offline mode is incompatible with --refresh or --accept-local")
	}
	if opts.refresh && opts.acceptLocal {
		return usageErrorf("--refresh and --accept-local are mutually exclusive")
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

	excluded := buildExcluded(specData)

	if opts.offline {
		lockData, err := lock.Load("skv.lock")
		if err != nil {
			return err
		}
		if err := verifyOffline(specData, lockData, repoRoot, excluded); err != nil {
			return err
		}
		globalOutput.Success("Verified %d skill(s) in offline mode", len(specData.Skills))
		return nil
	}

	lockData, lockMap, err := loadLockOptional("skv.lock")
	if err != nil {
		return err
	}

	_ = lockData
	seen := make(map[string]struct{})
	var lockSkills []lock.Skill
	passThrough := syncOptions{refresh: opts.refresh, acceptLocal: opts.acceptLocal}

	for _, skill := range specData.Skills {
		if skill.Name == "" {
			return fmt.Errorf("skill missing name")
		}
		if _, dup := seen[skill.Name]; dup {
			return fmt.Errorf("duplicate skill name %q", skill.Name)
		}
		seen[skill.Name] = struct{}{}

		globalOutput.Info("Syncing %s...", skill.Name)

		var entry lock.Skill
		if skill.Local != "" {
			entry, err = syncLocalSkill(repoRoot, skill, passThrough, lockMap)
		} else {
			entry, err = syncRemoteSkill(repoRoot, skill, passThrough, lockMap)
		}
		if err != nil {
			return err
		}

		lockSkills = append(lockSkills, entry)
		if err := linkSkill(repoRoot, skill.Name, excluded); err != nil {
			return err
		}
	}

	sort.Slice(lockSkills, func(i, j int) bool { return lockSkills[i].Name < lockSkills[j].Name })
	if err := lock.Write("skv.lock", &lock.Lock{Skills: lockSkills}); err != nil {
		return err
	}
	globalOutput.Success("Synced %d skill(s)", len(lockSkills))
	return nil
}

func runUpdate(name string, opts updateOptions) error {
	if opts.ref != "" && name == "" {
		return usageErrorf("--ref requires a skill name")
	}
	if opts.ref != "" && opts.all {
		return usageErrorf("--ref cannot be used with --all")
	}
	if name == "" && !opts.all {
		opts.all = true
	}
	if name != "" && opts.all {
		return usageErrorf("cannot combine a skill name with --all")
	}

	specData, err := spec.Load("skv.cue")
	if err != nil {
		return err
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := fsutil.EnsureDir(filepath.Join(".skv", "skills")); err != nil {
		return err
	}

	excluded := buildExcluded(specData)

	lockData, lockMap, err := loadLockRequired("skv.lock")
	if err != nil {
		return err
	}

	var targets []spec.SkillEntry
	if name != "" {
		skill, ok := findSkill(specData, name)
		if !ok {
			return fmt.Errorf("skill %q not found in spec", name)
		}
		if skill.Local != "" {
			return fmt.Errorf("cannot update local skill %q", name)
		}
		if isCommitRef(skill.Ref) && opts.ref == "" {
			return fmt.Errorf("skill %q is pinned to a commit", name)
		}
		if opts.ref != "" {
			skill.Ref = opts.ref
		}
		targets = append(targets, skill)
	} else {
		for _, skill := range specData.Skills {
			if skill.Local != "" {
				continue
			}
			if isCommitRef(skill.Ref) {
				continue
			}
			targets = append(targets, skill)
		}
	}

	if len(targets) == 0 {
		globalOutput.Info("No skills to update")
		return nil
	}

	for _, skill := range targets {
		globalOutput.Info("Updating %s...", skill.Name)
		entry, err := updateRemoteSkill(repoRoot, skill, lockMap, opts.force)
		if err != nil {
			return err
		}
		lockMap[skill.Name] = entry
		if err := linkSkill(repoRoot, skill.Name, excluded); err != nil {
			return err
		}
	}

	var lockSkills []lock.Skill
	for _, skill := range specData.Skills {
		entry, ok := lockMap[skill.Name]
		if !ok {
			continue
		}
		lockSkills = append(lockSkills, entry)
	}
	if len(lockSkills) == 0 {
		return fmt.Errorf("no lock entries found after update")
	}

	sort.Slice(lockSkills, func(i, j int) bool { return lockSkills[i].Name < lockSkills[j].Name })
	lockData.Skills = lockSkills
	if err := lock.Write("skv.lock", lockData); err != nil {
		return err
	}
	globalOutput.Success("Updated %d skill(s)", len(targets))
	return nil
}

func runVerify() error {
	lockData, err := lock.Load("skv.lock")
	if err != nil {
		return err
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := verifyLock(lockData, repoRoot); err != nil {
		return err
	}
	globalOutput.Success("Verified %d skill(s)", len(lockData.Skills))
	return nil
}

func runImport(inputPath string) error {
	if inputPath == "" {
		return usageErrorf("import requires <agentDir>/<skill>")
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	absPath, err := resolveLocalPath(repoRoot, inputPath)
	if err != nil {
		return err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("import path is not a directory: %s", absPath)
	}

	name := filepath.Base(absPath)
	vendorPath := filepath.Join(repoRoot, ".skv", "skills", name)

	if err := fsutil.EnsureDir(filepath.Dir(vendorPath)); err != nil {
		return err
	}
	if _, err := os.Stat(vendorPath); err == nil {
		return fmt.Errorf("vendored skill already exists: %s", vendorPath)
	}

	if err := os.Rename(absPath, vendorPath); err != nil {
		return err
	}

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

	localPath := filepath.ToSlash(filepath.Join(".", ".skv", "skills", name))
	entry := spec.SkillEntry{
		Name:  name,
		Local: localPath,
	}

	specData, err := spec.Load("skv.cue")
	if err != nil {
		return err
	}
	for _, skill := range specData.Skills {
		if skill.Name == name {
			return fmt.Errorf("skill %q already exists in spec", name)
		}
	}
	specData.Skills = append(specData.Skills, entry)
	if err := spec.Write("skv.cue", specData); err != nil {
		return err
	}

	lockData, lockMap, err := loadLockOptional("skv.lock")
	if err != nil {
		return err
	}

	license := detectLicense(vendorPath, repoRoot)
	lockMap[name] = lock.Skill{
		Name:     name,
		Local:    localPath,
		Checksum: checksum,
		License:  license,
	}

	var lockSkills []lock.Skill
	for _, skill := range specData.Skills {
		if entry, ok := lockMap[skill.Name]; ok {
			lockSkills = append(lockSkills, entry)
		}
	}
	sort.Slice(lockSkills, func(i, j int) bool { return lockSkills[i].Name < lockSkills[j].Name })
	lockData.Skills = lockSkills
	if err := lock.Write("skv.lock", lockData); err != nil {
		return err
	}

	excluded := buildExcluded(specData)
	if err := linkSkill(repoRoot, name, excluded); err != nil {
		return err
	}
	globalOutput.Success("Imported %s", name)
	return nil
}

// runSyncSingle syncs a single skill entry (used by add with auto-sync).
func runSyncSingle(skill spec.SkillEntry) error {
	if err := fsutil.EnsureDir(filepath.Join(".skv", "skills")); err != nil {
		return err
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	specData, err := spec.Load("skv.cue")
	if err != nil {
		return err
	}
	excluded := buildExcluded(specData)

	lockData, lockMap, err := loadLockOptional("skv.lock")
	if err != nil {
		return err
	}

	globalOutput.Info("Fetching %s...", skill.Name)

	var entry lock.Skill
	if skill.Local != "" {
		entry, err = syncLocalSkill(repoRoot, skill, syncOptions{}, lockMap)
	} else {
		entry, err = syncRemoteSkill(repoRoot, skill, syncOptions{}, lockMap)
	}
	if err != nil {
		return err
	}

	lockMap[skill.Name] = entry

	// Rebuild lock preserving spec order
	var lockSkills []lock.Skill
	for _, s := range specData.Skills {
		if e, ok := lockMap[s.Name]; ok {
			lockSkills = append(lockSkills, e)
		}
	}
	sort.Slice(lockSkills, func(i, j int) bool { return lockSkills[i].Name < lockSkills[j].Name })
	lockData.Skills = lockSkills

	if err := lock.Write("skv.lock", lockData); err != nil {
		return err
	}

	if err := linkSkill(repoRoot, skill.Name, excluded); err != nil {
		return err
	}

	commit := entry.Commit
	if commit == "" {
		commit = "(local)"
	} else if len(commit) > 7 {
		commit = commit[:7]
	}
	globalOutput.Success("Vendored %s (%s)", skill.Name, commit)

	return nil
}

func runList(opts listOptions) error {
	lockData, err := lock.Load("skv.lock")
	if err != nil {
		return err
	}

	if len(lockData.Skills) == 0 {
		if !opts.json && !opts.names {
			globalOutput.Info("No skills installed")
		}
		if opts.json {
			fmt.Println("[]")
		}
		return nil
	}

	if opts.json {
		data, err := json.MarshalIndent(lockData.Skills, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if opts.names {
		for _, skill := range lockData.Skills {
			fmt.Println(skill.Name)
		}
		return nil
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSOURCE\tREF\tCOMMIT")
	for _, skill := range lockData.Skills {
		source := skill.Repo
		ref := skill.Ref
		commit := skill.Commit

		if skill.Local != "" {
			source = skill.Local
			ref = "(local)"
			commit = "-"
		} else {
			// Shorten source for display
			source = strings.TrimPrefix(source, "https://")
			source = strings.TrimPrefix(source, "http://")
			source = strings.TrimSuffix(source, ".git")
			if skill.Path != "" {
				source += ":" + skill.Path
			}
			if ref == "" {
				ref = "(default)"
			}
			if len(commit) > 7 {
				commit = commit[:7]
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", skill.Name, source, ref, commit)
	}
	return w.Flush()
}

func runRemove(name string) error {
	specData, err := spec.Load("skv.cue")
	if err != nil {
		return err
	}

	// Find and remove from spec
	found := false
	var newSkills []spec.SkillEntry
	for _, skill := range specData.Skills {
		if skill.Name == name {
			found = true
			continue
		}
		newSkills = append(newSkills, skill)
	}
	if !found {
		return fmt.Errorf("skill %q not found in skv.cue", name)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	// Remove vendored content
	vendorPath := filepath.Join(repoRoot, ".skv", "skills", name)
	if err := os.RemoveAll(vendorPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove vendor directory: %w", err)
	}

	// Remove symlinks
	links := []string{
		filepath.Join(repoRoot, ".claude", "skills", name),
		filepath.Join(repoRoot, ".codex", "skills", name),
		filepath.Join(repoRoot, ".opencode", "skill", name),
	}
	for _, link := range links {
		if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove symlink %s: %w", link, err)
		}
	}

	// Update spec
	specData.Skills = newSkills
	if err := spec.Write("skv.cue", specData); err != nil {
		return err
	}

	// Update lock file
	lockData, lockMap, err := loadLockOptional("skv.lock")
	if err != nil {
		return err
	}
	delete(lockMap, name)

	var lockSkills []lock.Skill
	for _, skill := range newSkills {
		if entry, ok := lockMap[skill.Name]; ok {
			lockSkills = append(lockSkills, entry)
		}
	}
	sort.Slice(lockSkills, func(i, j int) bool { return lockSkills[i].Name < lockSkills[j].Name })
	lockData.Skills = lockSkills

	if err := lock.Write("skv.lock", lockData); err != nil {
		return err
	}

	globalOutput.Success("Removed %s", name)
	return nil
}

func runStatus() error {
	specData, err := spec.Load("skv.cue")
	if err != nil {
		return err
	}

	if len(specData.Skills) == 0 {
		globalOutput.Info("No skills defined in skv.cue")
		return nil
	}

	lockData, lockMap, err := loadLockOptional("skv.lock")
	if err != nil {
		return err
	}
	_ = lockData

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, skill := range specData.Skills {
		status := "ok"
		detail := ""

		entry, inLock := lockMap[skill.Name]
		vendorPath := filepath.Join(repoRoot, ".skv", "skills", skill.Name)

		if !inLock {
			status = "missing"
			detail = "not in lock file"
		} else {
			// Check if vendored content exists
			if _, err := os.Stat(vendorPath); os.IsNotExist(err) {
				status = "missing"
				detail = "vendor directory missing"
			} else if err != nil {
				status = "error"
				detail = err.Error()
			} else {
				// Check checksum
				checksum, err := hashDirWithTimeout(vendorPath)
				if err != nil {
					status = "error"
					detail = err.Error()
				} else if checksum != entry.Checksum {
					status = "modified"
					detail = "local changes detected"
				} else {
					// Format detail with ref info
					if entry.Local != "" {
						detail = "local"
					} else {
						ref := entry.Ref
						if ref == "" {
							ref = "default"
						}
						commit := entry.Commit
						if len(commit) > 7 {
							commit = commit[:7]
						}
						detail = fmt.Sprintf("%s @ %s", ref, commit)
					}
				}
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", skill.Name, status, detail)
	}
	return w.Flush()
}
