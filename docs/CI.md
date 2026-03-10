# CI

RepoFalcon is designed to run deterministically in CI so downstream automation, artifact publishing, and AI review jobs all receive stable inputs.

The most common pipeline shape is:

1. index the repository
2. normalize artifacts with `snapshot`
3. generate PR review context
4. upload or hand off the results to another job

## Recommended CI steps

1. Build (or `go run`) the CLI.
2. Index the repository.
3. Run `snapshot` to normalize ordering and make outputs deterministic.
4. Optionally run `pr-pack` to generate review-oriented outputs.
5. Upload or publish artifacts.

Example:

```bash
go run ./cmd/falcon index --repo . --out artifacts
go run ./cmd/falcon snapshot --in artifacts --out artifacts
go run ./cmd/falcon pr-pack --repo . --snapshot artifacts --base "$BASE_SHA" --head "$HEAD_SHA" --out artifacts
```

This produces a deterministic artifact set that can include:

- `nodes.parquet`, `edges.parquet`, and related graph tables for downstream automation
- `pr_context_pack.json` for machine-readable PR context
- `review_report.md` for human-readable review material

## Why `snapshot` matters

If you only remember one rule for CI, make it this one: run `snapshot` after `index`.

That normalization step makes artifact ordering stable and is the basis for reproducible downstream consumption.

## Deterministic output notes

RepoFalcon aims for stable outputs suitable for CI caching, diffing, and downstream automation:

- Paths are written repository-relative and canonicalized.
- Snapshotting materializes `nodes.parquet` and rewrites tables so row ordering is stable.
- JSON outputs are written without timestamps and with stable field ordering.

If you need reproducible-bytes outputs, prefer running `snapshot` and comparing the snapshot outputs.

## GitHub Actions

For GitHub Actions, RepoFalcon can be used either by calling the CLI directly or by reusing the composite action defined in [`action.yml`](action.yml:1).

That action is the easiest way to build PR context artifacts before a later review step.

This repository includes a minimal GitHub Actions workflow at [`/.github/workflows/ci.yml`](.github/workflows/ci.yml:1) that runs:

- `gofmt` (check)
- `go vet ./...`
- `go test ./...`
- a small smoke test against `testdata/`
