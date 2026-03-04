# RepoFalcon 🦅

[![CI](https://github.com/OWNER/REPO/actions/workflows/ci.yml/badge.svg)](https://github.com/OWNER/REPO/actions/workflows/ci.yml)

**RepoFalcon** gives you a bird’s‑eye view of your codebase.

It analyzes a repository, extracts structural relationships, and builds a **code knowledge graph** that helps developers understand large codebases, review pull requests more effectively, and detect architectural or security risks.

RepoFalcon is designed to run **entirely in CI**, producing structured artifacts that can be used by humans, automation, or AI tools.

---

## What RepoFalcon Does

Modern codebases are complex networks of relationships:

- files import other files
- functions call other functions
- modules depend on external libraries
- services interact across layers

RepoFalcon maps these relationships and turns them into a **repository graph** that can be queried and analyzed.

This enables:

- pull‑request impact analysis
- architecture exploration
- dependency mapping
- security finding correlation
- AI‑assisted code review

---

## Key Features

- 🦅 Repository knowledge graph
- 🔍 Pull request impact analysis
- 🧩 Dependency mapping
- 🧠 AI‑ready context generation
- 🔐 Security findings correlation
- ⚡ CI‑friendly design (no database required)

RepoFalcon generates artifacts that make large repositories easier to reason about.

---

## Supported Languages

RepoFalcon currently supports:

- TypeScript / JavaScript
- Python
- Java
- Go

Support for additional languages can be added through additional parsers.

---

## How It Works

RepoFalcon combines multiple analysis layers to build a graph of your repository:

1. Parse source files to extract structure
2. Identify symbols and relationships between them
3. Detect imports and dependencies
4. Incorporate security findings (if available)
5. Build a repository knowledge graph
6. Use that graph to analyze pull requests

The resulting graph can power architecture analysis, dependency exploration, and smarter reviews.

---

## Quickstart

Build the CLI:

```bash
go build -o falcon ./cmd/falcon
```

Index a repository (writes initial Parquet tables + `metadata.json`):

```bash
./falcon index --repo . --out artifacts
```

Materialize a deterministic snapshot (normalizes ordering, materializes `nodes.parquet`, rewrites `edges.parquet`):

```bash
./falcon snapshot --in artifacts --out artifacts
```

Generate a PR context pack between two git refs:

```bash
./falcon pr-pack --repo . --snapshot artifacts --base <base-sha-or-ref> --head <head-sha-or-ref>
```

Artifacts produced:

```
artifacts/
  metadata.json
  files.parquet
  symbols.parquet
  packages.parquet
  edges.parquet
  findings.parquet
  nodes.parquet
  pr_context_pack.json
  review_report.md
```

These artifacts can be consumed by CI pipelines, developer tools, or AI systems.

---

## Determinism

RepoFalcon is designed to be CI-friendly and reproducible:

- repository-relative paths are canonicalized
- Parquet row ordering is stable (tables are sorted and deduped)
- JSON outputs are written with stable field ordering and without timestamps

Integration tests run the CLI twice on the same fixture repos and compare logical Parquet table contents plus byte-compare `pr_context_pack.json` and `review_report.md`.

---

## Architecture

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md:1).

---

## Typical Use Cases

### Pull Request Reviews

RepoFalcon identifies which parts of the codebase are affected by a change and provides structured context for reviewers.

### Architecture Analysis

Explore how modules depend on each other and detect structural issues.

### Dependency Insights

Understand how external packages are used across the repository.

### Security Correlation

Combine static analysis findings with repository context to highlight relevant risks.

---

## Running in CI

RepoFalcon is designed for CI pipelines.

Typical pipeline steps:

1. Index the repository
2. Build a graph snapshot
3. Generate pull‑request context
4. Publish artifacts or comments

This approach keeps the pipeline **stateless and reproducible**.

### CI usage (GitHub Actions)

This repo ships a minimal GitHub Actions workflow at [`/.github/workflows/ci.yml`](.github/workflows/ci.yml:1).

For local parity with CI, run:

```bash
make ci
```

Or run the steps directly:

```bash
gofmt -w .
go vet ./...
go test ./...
```

For deterministic artifacts in CI, run `snapshot` after `index`. Details: [`docs/CI.md`](docs/CI.md:1).

---

## Philosophy

RepoFalcon treats a codebase not as a collection of files but as a **graph of relationships**.

By exposing this structure, developers and tools gain a much clearer understanding of how software systems evolve and interact.

---

## Status

RepoFalcon is under active development.

Contributions, feedback, and ideas are welcome.

---

## License

MIT License
