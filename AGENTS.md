# AGENTS

## Building

```bash
go build -o skv ./cmd/skv
```

## Validation checklist

- Run the full test suite:
  - `go test ./...`
- Make sure the tests pass before committing.

## E2E expectations

- Tests live in `internal/e2e/testdata/*.txt` and execute the compiled `skv` binary.
- Each test defines an explicit `expected.lock.tmpl` and compares `skv.lock` byte-for-byte after substituting runtime values.
- The suite covers `verify`, `update`, local skills, and offline sync behavior; failures should be treated as regressions.

## When changing the spec

- Update the `expected.lock.tmpl` blocks to match the new lock format.
- Keep lock templates readable; only substitute dynamic values (repo, commit, checksum).

## Homebrew formula

- Located at `skill-vendor/homebrew-tap` repo in `Formula/skv.rb`
- Distributes pre-built binaries for macOS/Linux (ARM/Intel)
