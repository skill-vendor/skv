# AGENTS

This file is for contributors and automation agents; it defines required checks and conventions.

## Design principles

- Keep it simple; avoid unnecessary layers or abstractions.
- DevEx comes first. The CLI is the primary UI, the docs should reinforce that.

## Local dev loop

```bash
# 1. Make changes
# 2. Run E2E tests (fast feedback)
go test ./internal/e2e -run TestE2E

# 3. Run full suite before committing
go test ./...
```

## Building

```bash
go build -o skv ./cmd/skv
```

## Validation checklist

### Always run

- `go test ./...` — full test suite must pass before committing

### If docs changed

Preview locally with Jekyll:

```bash
docker run --rm -p 4000:4000 -v "$PWD":/usr/src/app starefossen/github-pages \
  jekyll serve -s docs --livereload --host 0.0.0.0 --baseurl ""
```

Key doc files:
- `docs/index.md` — homepage
- `docs/skv.schema.cue` — CUE schema reference
- `docs/examples/skv.cue` — example spec

### If spec/lock format changed

- Update `expected.lock.tmpl` blocks in `internal/e2e/testdata/*.txt`
- Keep lock templates readable; only substitute dynamic values (repo, commit, checksum)

## E2E expectations

- Tests live in `internal/e2e/testdata/*.txt` and execute the compiled `skv` binary.
- Each test defines an explicit `expected.lock.tmpl` and compares `skv.lock` byte-for-byte after substituting runtime values.
- The suite covers `verify`, `update`, local skills, and offline sync behavior; failures should be treated as regressions.

## Release

See CONTRIBUTING.md for release steps. The Homebrew formula is at `skill-vendor/homebrew-tap` in `Formula/skv.rb`.
