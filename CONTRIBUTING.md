# Contributing

Thanks for helping build SKV. This repo is test-first by design.

## Testing philosophy

- **End-to-end first.** The CLI is validated via real `go test` runs that execute the compiled `skv` binary and operate on temporary git repositories.
- **Deterministic and local.** Tests avoid the network by using `file://` repos created in the test workspace. This keeps results fast and repeatable.
- **Explicit expectations.** Each E2E test has an `expected.lock.tmpl` in the test script, and the test materializes it with runtime values (repo URL, commit, checksum). We compare the resulting `skv.lock` byte-for-byte.
- **Spec-driven failures.** Some tests are expected to fail until features are implemented (e.g., `verify`, `update`, `local` skills, offline mode). These define the contract for the production binary.

## Running tests

```bash
go test ./...
```

To run only the end-to-end suite:

```bash
go test ./internal/e2e -run TestE2E
```

## Updating expected locks

If the lock schema changes, update the `expected.lock.tmpl` blocks in the relevant test scripts under `internal/e2e/testdata/`. Keep them readable and explicit; only substitute runtime values like repo URL, commit SHA, and checksum.

## Git requirements

The E2E tests create temporary git repositories, so `git` must be available on PATH.
