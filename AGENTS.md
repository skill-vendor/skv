# AGENTS

## Validation checklist

- Run the full test suite:
  - `go test ./...`
- If you are working on CLI behavior, run only the E2E tests:
  - `go test ./internal/e2e -run TestE2E`

## E2E expectations

- Tests live in `internal/e2e/testdata/*.txt` and execute the compiled `skv` binary.
- Each test defines an explicit `expected.lock.tmpl` and compares `skv.lock` byte-for-byte after substituting runtime values.
- Several tests are currently expected to fail until the production binary implements:
  - `skv verify`
  - `skv update`
  - `local` skills in `skv.cue`
  - `skv sync --offline` behavior

## When changing the spec

- Update the `expected.lock.tmpl` blocks to match the new lock format.
- Keep lock templates readable; only substitute dynamic values (repo, commit, checksum).
