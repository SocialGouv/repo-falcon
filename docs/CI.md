# CI

RepoFalcon is designed to run deterministically in CI. The goal is that repeated runs over the same input repository produce the same logical artifacts.

## Recommended CI steps

1. Build (or `go run`) the CLI.
2. Index the repository.
3. Run `snapshot` to normalize ordering and make outputs deterministic.
4. Upload/publish artifacts.

Example:

```bash
go run ./cmd/falcon index --repo . --out artifacts
go run ./cmd/falcon snapshot --in artifacts --out artifacts
```

## Deterministic output notes

RepoFalcon aims for stable outputs suitable for CI caching, diffing, and downstream automation:

- Paths are written repository-relative and canonicalized.
- Snapshotting materializes `nodes.parquet` and rewrites tables so row ordering is stable.
- JSON outputs are written without timestamps and with stable field ordering.

If you need reproducible-bytes outputs, prefer running `snapshot` and comparing the snapshot outputs.

## GitHub Actions

This repository includes a minimal GitHub Actions workflow at [`/.github/workflows/ci.yml`](.github/workflows/ci.yml:1) that runs:

- `gofmt` (check)
- `go vet ./...`
- `go test ./...`
- a small smoke test against `testdata/`

