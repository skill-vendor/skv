---
layout: default
title: SKV - Skill Vendor
---

<div class="hero">
  <div class="hero-copy">
    <p class="eyebrow">SKV</p>
    <h1>Repo-local, deterministic dependency management for agent skills.</h1>
    <p class="lede">Vendor skills directly into your repository, pin them by commit, and link them for the tools your team uses. Clone the repo and skills are ready - no registry, no global cache.</p>
    <div class="hero-actions">
      <a class="button" href="https://github.com/skill-vendor/skv">GitHub</a>
      <a class="button ghost" href="https://github.com/skill-vendor/skv/blob/main/docs/skv.schema.cue">Schema</a>
    </div>
    <div class="hero-meta">
      <span class="meta-label">Supported</span>
      <span>Claude Code, OpenAI Codex, OpenCode, Cursor</span>
    </div>
  </div>
  <div class="hero-panel">
    <div class="panel-title">Quick start</div>
    <pre><code>brew install skill-vendor/tap/skv
skv init
skv add https://github.com/acme/skill-foo
skv sync</code></pre>
    <div class="panel-note">Commit <code>skv.cue</code>, <code>skv.lock</code>, and <code>.skv/</code>.</div>
  </div>
</div>

## What is SKV?

SKV vendors agent skills into your repository, pins them with a lock file, and creates symlinks for each supported tool. Clone the repo and skills are ready - no registry, no global cache.

<div class="grid two-col">
  <div class="card">
    <h3>Key features</h3>
    <ul>
      <li>CUE-based spec with JSON lock file</li>
      <li>Deterministic installs (pinned commits + checksums)</li>
      <li>Vendored dependencies committed to the repo</li>
      <li>Works with any Git host (GitHub, GitLab, Codeberg, etc.)</li>
    </ul>
  </div>
  <div class="card">
    <h3>Supported tools</h3>
    <ul>
      <li>Claude Code (<code>.claude/skills/</code>)</li>
      <li>OpenAI Codex (<code>.codex/skills/</code>)</li>
      <li>OpenCode (<code>.opencode/skill/</code>)</li>
      <li>Cursor (via Codex/Claude-compatible handling)</li>
    </ul>
  </div>
</div>

---

## Quick Start

```bash
# 1. Install
brew install skill-vendor/tap/skv

# 2. Initialize
skv init

# 3. Add a skill
skv add https://github.com/acme/skill-foo

# 4. Sync (vendor + link)
skv sync

# 5. Commit everything
git add skv.cue skv.lock .skv .claude .codex .opencode
git commit -m "Add skill-foo"
```

---

## How It Works

```
skv.cue          skv sync         .skv/skills/
(your spec)  ──────────────►  (vendored content)
                    │
                    ▼
               skv.lock
           (commits + checksums)
                    │
                    ▼
    .claude/skills/  .codex/skills/  .opencode/skill/
                  (symlinks)
```

1. `skv sync` reads your `skv.cue` spec
2. Skills are vendored into `.skv/skills/<name>/`
3. `skv.lock` records the resolved commit SHA and checksum
4. Symlinks are created in each tool's skill directory

---

## Commands

| Command | Description |
|---------|-------------|
| `skv init` | Scaffold spec, lock, and `.skv/skills/` directory |
| `skv add <repo>[#ref][:path]` | Add a skill to the spec |
| `skv sync` | Vendor skills, update lock, refresh symlinks |
| `skv sync --offline` | Verify and link without network access |
| `skv sync --refresh` | Re-fetch and overwrite vendored content |
| `skv sync --accept-local` | Treat local content as source of truth |
| `skv update [name]` | Update floating refs (branches/tags) |
| `skv update --all` | Update all non-commit-pinned skills |
| `skv verify` | Check vendored skills match the lock (CI-friendly) |
| `skv list` | List all skills with their status |
| `skv remove <name>` | Remove a skill from spec, lock, and disk |
| `skv import <path>` | Move a local skill into SKV management |

---

## Adding Skills

```bash
# Single-skill repo (expects SKILL.md at root)
skv add https://github.com/acme/skill-foo

# Skill from a monorepo path
skv add https://github.com/acme/skill-pack:skills/skill-foo

# Pinned to a tag
skv add https://github.com/acme/skill-pack#v1.2.3:skills/skill-foo

# Override the skill name
skv add https://github.com/acme/skill-pack:skills/skill-foo --name release-notes
```

---

## Managing Skills

**List installed skills:**

```bash
$ skv list
skill-foo      https://github.com/acme/skill-foo (abc1234)
release-notes  https://github.com/acme/skill-pack:skills/release-notes#v1.2.3 (def5678)
local-helper   local:.skv/skills/local-helper
```

**Remove a skill:**

```bash
$ skv remove skill-foo
Removed skill-foo from spec, lock, and .skv/skills/
```

---

## Configuration

The spec file `skv.cue` defines your skill dependencies:

```cue
skv: {
  // Exclude specific tools (all enabled by default)
  tools: {
    exclude: ["opencode"]
  }

  skills: [
    {
      name: "skill-foo"
      repo: "https://github.com/acme/skill-pack"
      path: "skills/skill-foo"   // subdirectory in repo
      ref:  "v1.2.3"             // optional: tag, branch, or commit
    },
    {
      name:  "local-helper"
      local: "./.skv/skills/local-helper"  // local skill, not fetched
    },
  ]
}
```

**Fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique skill identifier |
| `repo` | Remote | Git repository URL |
| `path` | No | Subdirectory containing the skill |
| `ref` | No | Tag, branch, or commit (defaults to repo default branch) |
| `local` | Local | Path to local skill directory |

---

## CI Integration

Add `skv verify` to your CI pipeline to ensure vendored skills match the lock file:

```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]

jobs:
  verify-skills:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install skv
        run: |
          curl -sL https://github.com/skill-vendor/skv/releases/latest/download/skv-linux-amd64 -o skv
          chmod +x skv
          sudo mv skv /usr/local/bin/

      - name: Verify skills
        run: skv verify
```

If vendored content doesn't match the lock, `skv verify` exits non-zero with details about the mismatch.

---

## Installation

**Homebrew (macOS and Linux):**

```bash
brew install skill-vendor/tap/skv
```

**Go:**

```bash
go install github.com/skill-vendor/skv/cmd/skv@latest
```

**Binary:**

Download from [GitHub Releases](https://github.com/skill-vendor/skv/releases/latest):

```bash
curl -sL https://github.com/skill-vendor/skv/releases/latest/download/skv-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m) -o skv
chmod +x skv
sudo mv skv /usr/local/bin/
```

---

## Links

- [GitHub Repository](https://github.com/skill-vendor/skv)
- [Schema Reference](https://github.com/skill-vendor/skv/blob/main/docs/skv.schema.cue)
- [Example Config](https://github.com/skill-vendor/skv/blob/main/docs/examples/skv.cue)
- [Contributing](https://github.com/skill-vendor/skv/blob/main/CONTRIBUTING.md)

---

<small>Built with [Jekyll](https://jekyllrb.com/) and hosted on [GitHub Pages](https://pages.github.com/)</small>
