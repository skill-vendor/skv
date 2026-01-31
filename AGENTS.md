# AGENTS

## Validation checklist

- Run the full test suite:
  - `go test ./...`
- If you are working on CLI behavior, run only the E2E tests:
  - `go test ./internal/e2e -run TestE2E`

## E2E expectations

- Tests live in `internal/e2e/testdata/*.txt` and execute the compiled `skv` binary.
- Each test defines an explicit `expected.lock.tmpl` and compares `skv.lock` byte-for-byte after substituting runtime values.
- The suite covers `verify`, `update`, local skills, and offline sync behavior; failures should be treated as regressions.

## When changing the spec

- Update the `expected.lock.tmpl` blocks to match the new lock format.
- Keep lock templates readable; only substitute dynamic values (repo, commit, checksum).
