# Contributing

Thanks for helping build SKV. This repo is test-first by design.

## Testing philosophy

- **End-to-end first.** The CLI is validated via real `go test` runs that execute the compiled `skv` binary and operate on temporary git repositories.
- **Deterministic and local.** Tests avoid the network by using `file://` repos created in the test workspace. This keeps results fast and repeatable.
- **Explicit expectations.** Each E2E test has an `expected.lock.tmpl` in the test script, and the test materializes it with runtime values (repo URL, commit, checksum). We compare the resulting `skv.lock` byte-for-byte.
- **Spec-driven coverage.** E2E tests cover `verify`, `update`, `local` skills, and offline mode. Failures indicate regressions against the spec.

## Running tests

```bash
go test ./...
```

To run only the end-to-end suite:

```bash
go test ./internal/e2e -run TestE2E
```

## Validating docs (GitHub Pages)

The docs site lives in `docs/` and is built by GitHub Pages. Use the GitHub Pages Docker image to preview without managing Ruby locally.

```bash
docker run --rm -p 4000:4000 -v "$PWD":/usr/src/app starefossen/github-pages \
  jekyll serve -s docs --livereload --host 0.0.0.0 --baseurl ""
```

Open `http://localhost:4000/`.

## Design principles

- Keep the experience simple; eliminate anything that doesn't help the user ship.
- DevEx is the priority. The CLI is the product surface area, and the docs should reflect that.

## Building

Build the CLI from the repo root:

```bash
go build -o skv ./cmd/skv
./skv version  # prints "dev"
```

To build with a specific version:

```bash
go build -ldflags "-X main.version=v1.0.0" -o skv ./cmd/skv
```

## Releasing

Releases are created via GitHub Actions:

1. Go to **Actions** → **release** → **Run workflow**
2. Enter version in semver format (e.g., `v1.0.0`)
3. Click **Run workflow**

The workflow validates the version format, runs tests, builds binaries for linux/darwin × amd64/arm64, and creates a GitHub release with the binaries.

## Updating expected locks

If the lock schema changes, update the `expected.lock.tmpl` blocks in the relevant test scripts under `internal/e2e/testdata/`. Keep them readable and explicit; only substitute runtime values like repo URL, commit SHA, and checksum.

## Git requirements

The E2E tests create temporary git repositories, so `git` must be available on PATH.

## CI

CI runs `go test ./...` on every push and pull request (unit + E2E).
