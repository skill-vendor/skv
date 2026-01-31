# skv — Skill Vendor

Repo-local, deterministic dependency management for agent skills, built in Go.

SKV vendors skills into your repo, pins them with a lock file, and exposes them via per-skill symlinks into agent directories. The result: clone the repo and skills are ready, without a registry or centralized cache.

## Installation

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

## Why

Skills are becoming a shared standard across tools (Claude Code, Codex CLI, OpenCode), but there is no good ecosystem for dependency management. SKV brings a Go-modules-like workflow to skills:

- CUE spec + lock file
- Deterministic installs (pinned commits + checksums)
- Vendored dependencies committed to the repo
- Convention over configuration, with optional overrides

## How it works

1. `skv sync` reads `skv.cue`.
2. Skills are vendored into `./.skv/skills/<name>`.
3. `skv.lock` is updated with resolved commit SHA and checksum.
4. Per-skill symlinks are created in supported tool directories.

## Supported tools (default)

- Claude Code: `./.claude/skills/<name>`
- OpenAI Codex: `./.codex/skills/<name>`
- OpenCode: `./.opencode/skill/<name>`
- Cursor is supported via its Codex/Claude-compatible skills handling: https://cursor.com/docs/context/skills

All are enabled by default; you can exclude any subset in the spec.

## Files and directories

- `skv.cue`: human-authored spec
- `skv.lock`: machine-managed lock file
- `./.skv/skills/`: vendored skills (committed)
- `./.claude/skills/`, `./.codex/skills/`, `./.opencode/skill/`: symlink targets

## Commands

- `skv init` — scaffold spec + lock + `./.skv/skills`.
- `skv add <repo>[#ref][:path]` — add a skill source to the spec.
  - If `ref` is omitted, the repo default branch is used (lock pins the resolved commit).
  - If `path` is omitted, the repo root must contain `SKILL.md` (single-skill repo); otherwise `skv add` errors and asks for an explicit path.
  - Name defaults to the last segment of `path`, or the repo name if `path` is omitted. Override with `--name`.
- `skv sync` — primary command: vendor skills, update lock, refresh symlinks; verifies vendored content matches the lock and errors on mismatch.
  - `--refresh` re-fetches and overwrites vendored content, then rewrites the lock.
  - `--accept-local` treats current vendored content as the source of truth and rewrites the lock.
  - `--offline` never hits the network; verifies and links only (errors if a fetch is required).
- `skv update` — update floating refs (tags/branches/default branch) and rewrite lock + vendored content. The spec is unchanged.
  - `skv update` (no args) updates all non-commit-pinned skills (same as `--all`).
  - `skv update <name>` updates a single skill.
  - `skv update --all` updates all non-commit-pinned skills.
  - `skv update <name> --ref <ref>` updates one skill to a temporary ref for this run (lock + vendored only).
  - `--ref` requires a skill name and is incompatible with `--all`.
  - If a tag now resolves to a different commit, `skv update` warns and aborts; re-run with `--force` to accept the change.
- `skv verify` — check that vendored skills match `skv.lock` (CI-friendly).
- `skv import <agentDir>/<skill>` — move a local skill into `./.skv/skills`, add to spec, and link.
  - Moves the skill into `./.skv/skills/<name>`.
  - Adds a `local` entry in `skv.cue`.
  - Writes/updates the lock (checksum + license) for the moved skill.
  - Creates symlinks in managed tool directories.

Examples:

```bash
# Add a single-skill repo (expects SKILL.md at repo root).
skv add https://github.com/acme/skill-foo

# Add a specific skill from a monorepo path.
skv add https://github.com/acme/skill-pack:skills/skill-foo

# Add pinned to a tag.
skv add https://github.com/acme/skill-pack#v1.2.3:skills/skill-foo

# Add and override the skill name.
skv add https://github.com/acme/skill-pack:skills/skill-foo --name release-notes

# Update a single skill that tracks a branch/tag/default branch.
skv update skill-foo

# Update all non-commit-pinned skills.
skv update --all

# Temporarily update one skill to a specific ref.
skv update skill-foo --ref v1.3.0

# Verify vendored content against the lock.
skv verify

# Offline mode: no network access (errors if a fetch is required).
skv sync --offline
```

## Spec (CUE)

See `docs/skv.schema.cue` for the schema and `docs/examples/skv.cue` for a complete example.

```cue
// Skill dependencies for skv
// Vendors agent skills into your repo
// https://github.com/skill-vendor/skv
skv: {
  // Exclude tools; all are enabled by default.
  tools: {
    exclude: ["opencode"]
  }

  skills: [
    {
      name: "skill-foo"
      repo: "https://github.com/acme/skill-pack"
      path: "skills/skill-foo"
      ref:  "v1.2.3" // optional; defaults to repo default branch
    },
    {
      name: "local-bar"
      local: "./.skv/skills/local-bar"
    },
  ]
}
```

## SKILL.md format

Each skill directory must contain a `SKILL.md` with YAML frontmatter and an optional Markdown body. SKV only validates that `SKILL.md` exists; content rules (naming and length constraints) are tool-specific. See the official docs for details:

- [Claude Agent Skills overview](https://docs.claude.com/en/docs/agents/skills/)
- [OpenAI Codex skills (create skill)](https://developers.openai.com/codex/skills/create-skill)
- [OpenCode Agent Skills](https://opencode.ai/docs/skills)

## Local skills

Use `local` entries in `skv.cue` for skills that already live in the repo or should not be fetched from a remote. `skv sync` verifies the local directory (including `SKILL.md`), computes the checksum, and links the skill into tool directories. `skv update` skips local skills.

## Lock file

`skv.lock` is a machine-managed JSON file that captures resolved metadata per skill:

- repo URL
- path
- resolved commit SHA
- checksum of the vendored directory
- best-effort license metadata (SPDX identifier and/or license file path)

Checksums use SHA-256 over a deterministic directory hash (sorted file paths + file mode + file bytes; excludes `.git/` and OS metadata). This adds integrity and reproducibility: it detects local edits or tampering, and ensures the vendored contents match what was resolved when the lock was written. `skv sync` and `skv verify` validate checksums and error on mismatches. `skv update` rewrites checksums for moving refs (branches/default branch). Tags are expected to be stable—if a tag resolves to a different commit, `skv update` warns and aborts unless you re-run with `--force`.

License metadata is best-effort: SKV scans for license files (e.g., `LICENSE`, `COPYING`, `NOTICE`) in the skill directory and, if not found there, in the repo root. The lock records the SPDX identifier when detectable and the path to the license file when present.

## Safety and limits

SKV treats remote repos and inputs as adversarial and enforces strict limits:

- Max checkout size: 50 MB (warns and aborts if exceeded).
- Max skill directory size: 20 MB (warns and aborts if exceeded).
- Max file count per skill: 5,000 files (warns and aborts if exceeded).
- Max individual file size: 5 MB (warns and aborts if exceeded).
- Timeouts for clone/checkout and hashing to avoid hangs.
- No execution of repo code or hooks.
- Sparse checkout of the requested path only.
- Reject path traversal and repo-escaping symlinks.
- Skip Git LFS content; if LFS pointers are detected, SKV errors.

## Edge cases

- Missing `SKILL.md`: hard error (the vendored path must contain `SKILL.md` at its root).
- Vendored content diverges from lock: hard error; use `skv sync --refresh` to re-fetch or `skv sync --accept-local` to re-lock local changes.
- Network unavailable: `skv sync` hard errors. Use `skv sync --offline` to verify/link only; it still errors if a fetch is required.

## Design decisions

- No registry: decentralized, works with any Git host (GitHub, GitLab, Codeberg, etc.), skills are explicitly listed in the spec.
- Per-skill linking: avoids clobbering existing skills in tool directories.

## Status

Early WIP. Feedback welcome.
