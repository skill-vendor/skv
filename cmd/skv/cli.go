package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var errUsage = errors.New("usage")

type usageError struct {
	err error
}

func (u usageError) Error() string {
	return u.err.Error()
}

func (u usageError) Unwrap() error {
	return errUsage
}

func usageErrorf(format string, args ...any) error {
	return usageError{err: fmt.Errorf(format, args...)}
}

func Execute() error {
	cmd := newRootCmd()
	return cmd.Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skv",
		Short: "Skill Vendor",
		Long:  "skv vendors skills into a repo and keeps skv.cue and skv.lock in sync.",
		Example: strings.TrimSpace(`
  skv init
  skv add https://github.com/acme/skill-foo
  skv sync
  skv verify
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	cmd.CompletionOptions.DisableDefaultCmd = true

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newSyncCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newVerifyCmd())
	cmd.AddCommand(newImportCmd())

	return cmd
}

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Initialize skv in the current repo",
		Long:    "Create skv.cue, skv.lock, and .skv/skills in the current repository.",
		Example: "  skv init",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usageErrorf("init does not accept arguments")
			}
			return runInit()
		},
	}
	return cmd
}

func newAddCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "add <repo>[#ref][:path]",
		Short: "Add a skill source to skv.cue",
		Long:  "Add a skill source to skv.cue. If :path is omitted, the repo root must contain SKILL.md.",
		Example: strings.TrimSpace(`
  skv add https://github.com/acme/skill-foo
  skv add https://github.com/acme/skill-pack:skills/skill-foo
  skv add https://github.com/acme/skill-pack#v1.2.3:skills/skill-foo
  skv add https://github.com/acme/skill-pack:skills/skill-foo --name release-notes
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErrorf("add requires <repo>[#ref][:path]")
			}
			return runAdd(args[0], addOptions{name: name})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "override skill name")
	return cmd
}

func newSyncCmd() *cobra.Command {
	var offline bool
	var refresh bool
	var acceptLocal bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Vendor skills, update the lock, and link into tools",
		Long: "Fetch and vendor skills, update skv.lock, and refresh tool links. " +
			"Errors on mismatched vendored content unless you re-fetch or accept local changes.",
		Example: strings.TrimSpace(`
  skv sync
  skv sync --offline
  skv sync --refresh
  skv sync --accept-local
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usageErrorf("sync does not accept arguments")
			}
			opts := syncOptions{offline: offline, refresh: refresh, acceptLocal: acceptLocal}
			return runSync(opts)
		},
	}
	cmd.Flags().BoolVar(&offline, "offline", false, "verify and link using existing lock/vendor data")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "re-fetch remote skills and rewrite checksums")
	cmd.Flags().BoolVar(&acceptLocal, "accept-local", false, "trust local vendored content and rewrite checksums")
	return cmd
}

func newUpdateCmd() *cobra.Command {
	var updateAll bool
	var ref string
	var force bool
	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Update floating refs and rewrite skv.lock",
		Long: "Update floating refs (branches/tags/default branch) and rewrite skv.lock. " +
			"Commit-pinned skills are skipped unless a temporary ref is provided.",
		Example: strings.TrimSpace(`
  skv update
  skv update --all
  skv update skill-foo
  skv update skill-foo --ref v1.3.0
  skv update skill-foo --force
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return usageErrorf("update accepts at most one skill name")
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			opts := updateOptions{all: updateAll, ref: ref, force: force}
			return runUpdate(name, opts)
		},
	}
	cmd.Flags().BoolVar(&updateAll, "all", false, "update all non-commit refs")
	cmd.Flags().StringVar(&ref, "ref", "", "temporary ref for this update")
	cmd.Flags().BoolVar(&force, "force", false, "allow tag ref to move")
	return cmd
}

func newVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "verify",
		Short:   "Verify vendored skills match skv.lock",
		Long:    "Verify that vendored skill content matches the checksums recorded in skv.lock.",
		Example: "  skv verify",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usageErrorf("verify does not accept arguments")
			}
			return runVerify()
		},
	}
	return cmd
}

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <agentDir>/<skill>",
		Short: "Import a local skill into .skv/skills",
		Long: "Move a local skill into .skv/skills, add it as a local entry in skv.cue, " +
			"and link it into supported tool directories.",
		Example: strings.TrimSpace(`
  skv import .codex/skills/skill-foo
  skv import .claude/skills/skill-foo
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErrorf("import requires <agentDir>/<skill>")
			}
			return runImport(args[0])
		},
	}
	return cmd
}
