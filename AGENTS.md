# CLAUDE

## Validation

- Full test suite:
  - `go test ./...`
- End-to-end suite:
  - `go test ./internal/e2e -run TestE2E`

## Notes

- E2E tests run the compiled `skv` binary built during `go test`.
- Several E2E cases are expected to fail until `verify`, `update`, `local` skills, and offline behavior are implemented.
