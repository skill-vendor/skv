package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z"
var version = "dev"

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
	var quiet bool

	cmd := &cobra.Command{
		Use:   "skv",
		Short: "Skill Vendor",
		Long:  "skv vendors skills into a repo and keeps skv.cue and skv.lock in sync.",
		Example: strings.TrimSpace(`
  skv init
  skv add https://github.com/acme/skill-foo
  skv sync
  skv verify
  skv list
  skv status
`),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			globalOutput.SetQuiet(quiet)
		},
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

	cmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-error output")

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newSyncCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newVerifyCmd())
	cmd.AddCommand(newImportCmd())
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newStatusCmd())

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(version)
			return nil
		},
	}
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
	var noSync bool
	cmd := &cobra.Command{
		Use:   "add <repo>[#ref][:path]",
		Short: "Add a skill and fetch it immediately",
		Long: "Add a skill source to skv.cue and fetch it immediately. " +
			"Use --no-sync to only modify skv.cue without fetching.",
		Example: strings.TrimSpace(`
  skv add https://github.com/acme/skill-foo
  skv add https://github.com/acme/skill-pack:skills/skill-foo
  skv add https://github.com/acme/skill-pack#v1.2.3:skills/skill-foo
  skv add https://github.com/acme/skill-pack:skills/skill-foo --name release-notes
  skv add https://github.com/acme/skill-foo --no-sync
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErrorf("add requires <repo>[#ref][:path]")
			}
			return runAdd(args[0], addOptions{name: name, noSync: noSync})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "override skill name")
	cmd.Flags().BoolVar(&noSync, "no-sync", false, "only add to skv.cue, don't fetch")
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

func newListCmd() *cobra.Command {
	var jsonOutput bool
	var namesOnly bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed skills",
		Long:  "Show all skills installed in skv.lock with their source, ref, and commit.",
		Example: strings.TrimSpace(`
  skv list
  skv list --json
  skv list --names
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usageErrorf("list does not accept arguments")
			}
			return runList(listOptions{json: jsonOutput, names: namesOnly})
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&namesOnly, "names", false, "output skill names only, one per line")
	return cmd
}

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a skill completely",
		Long: "Remove a skill from skv.cue, delete its vendored content from .skv/skills, " +
			"remove symlinks from tool directories, and rewrite skv.lock.",
		Example: strings.TrimSpace(`
  skv remove skill-foo
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErrorf("remove requires a skill name")
			}
			return runRemove(args[0])
		},
	}
	return cmd
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of installed skills",
		Long:  "Show the state and drift for each skill (ok, modified, missing).",
		Example: strings.TrimSpace(`
  skv status
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usageErrorf("status does not accept arguments")
			}
			return runStatus()
		},
	}
	return cmd
}
