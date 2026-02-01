# skv — Skill Vendor

Repo-local, deterministic dependency management for agent skills, built in Go.

> **Status:** Early WIP. Feedback welcome.

## Who is this for?

SKV is for teams who want **repo-local, deterministic skills**—vendored into the repo, pinned by commit, and verified in CI. If you want a global cache or a centralized registry, SKV isn't the right fit.

## Quick Start

```bash
# 1. Install
brew install skill-vendor/tap/skv

# 2. Initialize
skv init

# 3. Add a skill
skv add https://github.com/acme/skill-foo

# 4. Vendor and link
skv sync

# 5. Commit
git add skv.cue skv.lock .skv/
git commit -m "Add skill-foo"
```

**Commit these files:** `skv.cue` (your spec), `skv.lock` (pinned versions), and `.skv/` (vendored skills).

## How it works

1. `skv sync` reads `skv.cue`.
2. Skills are vendored into `.skv/skills/<name>`.
3. `skv.lock` is updated with resolved commit SHA and checksum.
4. Per-skill symlinks are created in supported tool directories.

## Why

Skills are becoming a shared standard across tools (Claude Code, Codex CLI, OpenCode), but there is no good ecosystem for dependency management. SKV brings a Go-modules-like workflow to skills:

- CUE spec + lock file
- Deterministic installs (pinned commits + checksums)
- Vendored skills committed to the repo
- Convention over configuration, with optional overrides

## Installation

**Using Homebrew (macOS and Linux):**

```bash
brew install skill-vendor/tap/skv
```

**Using Go:**

```bash
go install github.com/skill-vendor/skv/cmd/skv@latest
```

**From GitHub Releases:**

Download the latest binary for your platform from [GitHub Releases](https://github.com/skill-vendor/skv/releases/latest), then:

```bash
chmod +x skv-*
sudo mv skv-* /usr/local/bin/skv
```

## Supported tools

- Claude Code: `.claude/skills/<name>`
- OpenAI Codex: `.codex/skills/<name>`
- OpenCode: `.opencode/skill/<name>`
- [Cursor](https://cursor.com/docs/context/skills) (via Codex/Claude-compatible skills handling)

All are enabled by default; you can exclude any subset in the spec.

## Commands

| Command | Description |
|---------|-------------|
| `skv init` | Scaffold spec, lock, and `.skv/skills/` |
| `skv add <repo>[#ref][:path]` | Add a skill to the spec |
| `skv sync` | Vendor skills, update lock, refresh symlinks |
| `skv update [name]` | Update floating refs (branches/tags) |
| `skv verify` | Check vendored skills match the lock (CI-friendly) |
| `skv list` | List all skills with their status |
| `skv remove <name>` | Remove a skill |
| `skv import <path>` | Move a local skill into SKV management |

See the [docs site](https://skill-vendor.github.io/skv/) for full command reference and options.

## Examples

```bash
# Add a single-skill repo (expects SKILL.md at repo root)
skv add https://github.com/acme/skill-foo

# Add a skill from a monorepo path
skv add https://github.com/acme/skill-pack:skills/skill-foo

# Add pinned to a tag
skv add https://github.com/acme/skill-pack#v1.2.3:skills/skill-foo

# Update all non-commit-pinned skills
skv update --all

# Verify in CI
skv verify

# Offline mode (no network)
skv sync --offline
```

## Spec (CUE)

See the [schema](https://github.com/skill-vendor/skv/blob/main/docs/skv.schema.cue) and [example](https://github.com/skill-vendor/skv/blob/main/docs/examples/skv.cue) for full details.

```cue
skv: {
  tools: {
    exclude: ["opencode"]
  }
  skills: [
    {
      name: "skill-foo"
      repo: "https://github.com/acme/skill-pack"
      path: "skills/skill-foo"
      ref:  "v1.2.3"
    },
    {
      name:  "local-bar"
      local: "./.skv/skills/local-bar"
    },
  ]
}
```

## CI verification

```yaml
- name: Verify skills
  run: skv verify
```

If vendored content doesn't match the lock, `skv verify` exits non-zero.

## Reference

For detailed documentation on the lock file format, safety limits, edge cases, and more, see the [docs site](https://skill-vendor.github.io/skv/).
