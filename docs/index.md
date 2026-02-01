---
layout: default
title: SKV - Skill Vendor
---

<div class="hero">
  <div class="hero-copy">
    <h1>Deterministic, repo-local skill deps for your CLI workflow.</h1>
    <p class="hero-tagline">Repo-local · Deterministic · No registry</p>
    <p class="hero-links"><a href="https://github.com/skill-vendor/skv">GitHub</a> · <a href="https://github.com/skill-vendor/skv/blob/main/docs/skv.schema.cue">Schema</a></p>
  </div>
  <div class="hero-panel">
    <pre><code>brew install skill-vendor/tap/skv
skv init
skv add https://github.com/acme/skill-foo
skv sync</code></pre>
    <div class="panel-note">Commit <code>skv.cue</code>, <code>skv.lock</code>, and <code>.skv/</code>.</div>
  </div>
</div>

<nav class="toc">
  <strong>On this page:</strong>
  <a href="#why-skv">Why SKV</a> ·
  <a href="#workflow">Workflow</a> ·
  <a href="#commands">Commands</a> ·
  <a href="#configuration">Configuration</a> ·
  <a href="#lock-file">Lock File</a> ·
  <a href="#ci-integration">CI</a> ·
  <a href="#troubleshooting">Troubleshooting</a> ·
  <a href="#installation">Installation</a>
</nav>

---

## Why SKV?

Skills are becoming a shared standard across tools (Claude Code, Codex CLI, OpenCode), but there is no good ecosystem for dependency management. SKV brings a Go-modules-like workflow to skills:

<div class="grid two-col">
  <div class="card">
    <h3>The problem</h3>
    <ul>
      <li>Skills scattered across repos with no versioning</li>
      <li>No way to pin exact versions across a team</li>
      <li>Manual copy-paste leads to drift</li>
      <li>No CI verification that skills match expectations</li>
    </ul>
  </div>
  <div class="card">
    <h3>SKV's approach</h3>
    <ul>
      <li>CUE-based spec with JSON lock file</li>
      <li>Deterministic installs (pinned commits + checksums)</li>
      <li>Vendored skills committed to the repo</li>
      <li>Works with any Git host—no registry required</li>
    </ul>
  </div>
</div>

---

## Supported Tools

<div class="grid two-col">
  <div class="card">
    <h3>Native support</h3>
    <ul>
      <li>Claude Code (<code>.claude/skills/</code>)</li>
      <li>OpenAI Codex (<code>.codex/skills/</code>)</li>
      <li>OpenCode (<code>.opencode/skill/</code>)</li>
    </ul>
  </div>
  <div class="card">
    <h3>Compatible</h3>
    <ul>
      <li><a href="https://cursor.com/docs/context/skills">Cursor</a> (via Codex/Claude-compatible skills handling)</li>
    </ul>
  </div>
</div>

---

## Workflow

<p class="section-lede">Go from spec to vendored skills and tool links in one repeatable flow.</p>

<div class="flow">
  <div class="flow-step">
    <span class="flow-index">1</span>
    <h3>Spec</h3>
    <p>Declare skills in <code>skv.cue</code> with repo, ref, and path.</p>
  </div>
  <div class="flow-step">
    <span class="flow-index">2</span>
    <h3>Vendor</h3>
    <p><code>skv sync</code> fetches and pins exact commits into <code>.skv/skills/</code>.</p>
  </div>
  <div class="flow-step">
    <span class="flow-index">3</span>
    <h3>Link</h3>
    <p>SKV wires symlinks into each tool's skill directory automatically.</p>
  </div>
  <div class="flow-step">
    <span class="flow-index">4</span>
    <h3>Commit</h3>
    <p>Commit <code>skv.cue</code>, <code>skv.lock</code>, and <code>.skv/</code> to your repo.</p>
  </div>
  <div class="flow-step">
    <span class="flow-index">5</span>
    <h3>Verify</h3>
    <p>Run <code>skv verify</code> in CI to ensure skills match the lock.</p>
  </div>
</div>

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
| `name` | Always | Unique skill identifier |
| `repo` | For remote skills | Git repository URL |
| `path` | No | Subdirectory containing the skill |
| `ref` | No | Tag, branch, or commit (defaults to repo default branch) |
| `local` | For local skills | Path to local skill directory (mutually exclusive with `repo`) |

---

## Lock File

`skv.lock` is a machine-managed JSON file that ensures deterministic installs. It captures:

- **Resolved commit SHA** — the exact commit vendored, even if the spec uses a branch or tag
- **Checksum** — SHA-256 hash of the vendored directory contents
- **License metadata** — best-effort SPDX identifier and license file path

**Why checksums matter:**

Checksums use SHA-256 over a deterministic directory hash (sorted file paths + file mode + file contents). This provides:

- **Integrity** — detects local edits or tampering
- **Reproducibility** — ensures vendored contents match what was resolved
- **CI verification** — `skv verify` validates checksums and errors on mismatch

Tags are expected to be stable. If a tag resolves to a different commit, `skv update` warns and aborts unless you re-run with `--force`.

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

For ARM64 runners, use `skv-linux-arm64` instead.

If vendored content doesn't match the lock, `skv verify` exits non-zero with details about the mismatch.

---

## Troubleshooting

**Missing SKILL.md**

```
error: skill directory must contain SKILL.md at its root
```

Every skill directory must have a `SKILL.md` file. If adding a single-skill repo, ensure `SKILL.md` is at the repo root. For monorepos, specify the path to the skill directory.

**Lock mismatch**

```
error: vendored content does not match lock
```

The vendored files have changed since the lock was written. Options:

- `skv sync --refresh` — re-fetch from remote and update the lock
- `skv sync --accept-local` — accept local changes and rewrite the lock

**Network unavailable**

```
error: fetch required but running in offline mode
```

`skv sync --offline` can only verify and link already-vendored skills. If a skill hasn't been fetched yet, you'll need network access.

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
