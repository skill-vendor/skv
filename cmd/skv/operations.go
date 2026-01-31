package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/skill-vendor/skv/internal/fsutil"
	"github.com/skill-vendor/skv/internal/lock"
	"github.com/skill-vendor/skv/internal/spec"
)

type addOptions struct {
	name string
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
	return spec.Write("skv.cue", specData)
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
		return verifyOffline(specData, lockData, repoRoot, excluded)
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
	return lock.Write("skv.lock", &lock.Lock{Skills: lockSkills})
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
		return nil
	}

	for _, skill := range targets {
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
	return lock.Write("skv.lock", lockData)
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

	return verifyLock(lockData, repoRoot)
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
	return linkSkill(repoRoot, name, excluded)
}
