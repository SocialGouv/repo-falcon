# RepoFalcon 🦅

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

## Install

Download the latest binary for your platform:

```bash
curl -fsSL "https://github.com/SocialGouv/repo-falcon/releases/latest/download/falcon-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')" | sudo tee /usr/local/bin/falcon > /dev/null && sudo chmod +x /usr/local/bin/falcon
```

Or install to a local directory:

```bash
curl -fsSL "https://github.com/SocialGouv/repo-falcon/releases/latest/download/falcon-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')" -o ./falcon && chmod +x ./falcon
```

**Windows** (PowerShell):

```powershell
Invoke-WebRequest -Uri "https://github.com/SocialGouv/repo-falcon/releases/latest/download/falcon-windows-amd64.exe" -OutFile falcon.exe
```

Verify the download (optional):

```bash
curl -fsSL "https://github.com/SocialGouv/repo-falcon/releases/latest/download/falcon-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/').sha256" | sha256sum -c
```

---

## Quickstart

Build the CLI:

```bash
go build -o bin/falcon ./cmd/falcon
```

One-command setup (index + snapshot + agent context):

```bash
./falcon init --repo .
```

Or run the steps individually:

```bash
./falcon index --repo . --out .falcon/artifacts
./falcon snapshot --in .falcon/artifacts --out .falcon/artifacts
./falcon pr-pack --repo . --snapshot .falcon/artifacts --base <base-sha-or-ref> --head <head-sha-or-ref>
```

Artifacts produced:

```
.falcon/
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
  CONTEXT.md
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

---

## GitHub Action usage

RepoFalcon ships as a reusable composite action via [`action.yml`](action.yml:1).

Example workflows are included in this repo:

- [`/.github/workflows/repofalcon_pr_context.yml`](.github/workflows/repofalcon_pr_context.yml:1)
- [`/.github/workflows/repofalcon_then_claude_review.yml`](.github/workflows/repofalcon_then_claude_review.yml:1)

### Basic PR workflow (generate PR context artifacts)

```yaml
name: RepoFalcon PR context

on:
  pull_request:

jobs:
  repofalcon:
    runs-on: ubuntu-latest
    steps:
      - name: Generate RepoFalcon artifacts
        id: falcon
        uses: <owner>/repo-falcon@<ref>
        with:
          mode: pr
          repo: .
          out: artifacts
          base: ${{ github.event.pull_request.base.sha }}
          head: ${{ github.event.pull_request.head.sha }}

      - name: Inspect outputs
        shell: bash
        run: |
          set -euo pipefail
          echo "Artifacts dir: ${{ steps.falcon.outputs.artifacts-dir }}"
          echo "Context pack:  ${{ steps.falcon.outputs.context-pack-path }}"
          echo "Report:        ${{ steps.falcon.outputs.review-report-path }}"
```

### Upload generated artifacts

```yaml
- name: Upload RepoFalcon artifacts
  uses: actions/upload-artifact@v4
  with:
    name: repofalcon-artifacts
    path: ${{ steps.falcon.outputs.artifacts-dir }}
    if-no-files-found: error
```

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

## Coding Agent Integration

RepoFalcon can expose its code knowledge graph to coding agents (Claude Code, Roo Code, Cline, Cursor, etc.) via two methods:

1. **One command**: `./falcon init --repo .` (generates everything under `.falcon/`)
2. **MCP server**: `./falcon mcp serve --snapshot .falcon/artifacts`

See [`docs/AGENT_INTEGRATION.md`](docs/AGENT_INTEGRATION.md) for setup guides.

---

## Philosophy

RepoFalcon treats a codebase not as a collection of files but as a **graph of relationships**.

By exposing this structure, developers and tools gain a much clearer understanding of how software systems evolve and interact.

---

## Development

### Prerequisites

- **direnv** (recommended) — automatically loads the repo environment.
  - Install: https://direnv.net/docs/installation.html
  - After install (once):
    - `direnv allow`
- **devbox** — provides a reproducible toolchain (Go, Node, pnpm, task, etc.).
  - Install: https://www.jetify.com/devbox/docs/installing_devbox/
  - Note: devbox uses **Nix** under the hood; the devbox installer will guide you.

---

## Status

RepoFalcon is under active development.

Contributions, feedback, and ideas are welcome.

---

## License

MIT License
