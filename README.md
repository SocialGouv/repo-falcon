# RepoFalcon 🦅

**RepoFalcon** turns a repository into deterministic artifacts and a queryable code knowledge graph that developers, CI pipelines, and coding agents can use immediately.

It is especially useful when you want to:

- give **Claude Code** or another coding agent a structural view of the repository instead of making it work file by file
- generate **pull request context** in CI before an automated review step
- publish **stable artifacts** that other tools can consume without needing a database or a long-running service

---

## Why teams use RepoFalcon

Modern repositories are harder to review than they are to compile. A pull request may change only a few files while affecting callers, packages, or architectural boundaries elsewhere.

RepoFalcon maps repository structure into a graph so you can answer practical questions faster:

- What files and symbols are impacted by this PR?
- Which packages depend on this module?
- What context should I provide to Claude Code before asking for a refactor?
- What deterministic artifacts should CI publish for downstream automation?

RepoFalcon currently supports:

- TypeScript / JavaScript
- Python
- Java
- Go

---

## Key features

- 🦅 **Repository knowledge graph** — map files, symbols, packages, and relationships across the codebase
- 🤖 **Agent-ready context** — give Claude Code and other coding agents a structural view of the repository
- 🔍 **Pull request impact analysis** — understand what a change touches before review or refactoring
- ⚙️ **CI-friendly artifacts** — generate deterministic outputs that are easy to upload, diff, and reuse
- 🧩 **Dependency visibility** — inspect package boundaries, imports, and downstream impact
- 📦 **PR review outputs** — produce `pr_context_pack.json` and `review_report.md` for downstream review steps
- 🔐 **Finding correlation** — connect repository structure with analysis findings when available

---

## Quickstart

Download the latest binary for your platform:

```bash
curl -fsSL "https://github.com/SocialGouv/repo-falcon/releases/latest/download/falcon-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')" -o ./falcon && chmod +x ./falcon
```

Or install it globally:

```bash
curl -fsSL "https://github.com/SocialGouv/repo-falcon/releases/latest/download/falcon-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')" | sudo tee /usr/local/bin/falcon > /dev/null && sudo chmod +x /usr/local/bin/falcon
```

**Windows** (PowerShell):

```powershell
Invoke-WebRequest -Uri "https://github.com/SocialGouv/repo-falcon/releases/latest/download/falcon-windows-amd64.exe" -OutFile falcon.exe
```

The fastest end-to-end setup is:

```bash
./falcon init --repo .
```

This command:

1. indexes the repository
2. builds a deterministic snapshot
3. generates [`.falcon/CONTEXT.md`](.falcon/)
4. can configure coding-agent integrations

If you want to run the steps manually:

```bash
./falcon index --repo . --out .falcon/artifacts
./falcon snapshot --in .falcon/artifacts --out .falcon/artifacts
./falcon pr-pack --repo . --snapshot .falcon/artifacts --base <base-sha-or-ref> --head <head-sha-or-ref> --out .falcon/artifacts
```

Typical generated outputs:

```text
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

---

## Use case: Claude Code and coding agents

One of the clearest use cases for RepoFalcon is preparing a repository for **Claude Code** or another coding agent.

Without RepoFalcon, an agent usually has to discover architecture by reading files, grepping imports, and guessing dependency direction. With RepoFalcon, the repository already has a graph-backed summary and MCP tools available.

### Typical workflow

```bash
./falcon init --repo . --agents claude
```

That setup gives you two useful layers:

1. **Static context** via [`.falcon/CONTEXT.md`](.falcon/)
2. **Live graph queries** via [`falcon mcp serve`](cmd/falcon/main.go:1)

RepoFalcon can also update Claude Code files such as [`CLAUDE.md`](CLAUDE.md:1) and MCP configuration so the agent knows when to query repository structure instead of relying only on raw file search.

### Concrete example

Imagine you ask Claude Code to refactor a package boundary or rename a function used across multiple modules.

With RepoFalcon in place, Claude Code can:

- inspect file dependencies before editing
- understand package boundaries before introducing a new import
- search symbol relationships structurally instead of relying only on grep
- refresh graph artifacts after a major refactor

This makes refactors safer and reduces wasted context-window usage.

For the full integration guide, agent-specific setup, and MCP details, see [`docs/AGENT_INTEGRATION.md`](docs/AGENT_INTEGRATION.md:1).

---

## Use case: pull request analysis in CI

RepoFalcon is designed to run well in CI because its outputs are deterministic and easy to publish as artifacts.

A common workflow is:

1. index the repository
2. snapshot the graph into normalized artifacts
3. generate a PR context pack
4. pass those artifacts to another review or automation step

### Why this matters

This is useful when you want to run an AI reviewer, publish a review summary, or keep machine-readable artifacts for later analysis.

Instead of asking downstream tooling to rediscover the repository from scratch, CI can hand over:

- `pr_context_pack.json` for structured PR context
- `review_report.md` for human-readable review material
- Parquet graph artifacts for deeper analysis

### Manual CI commands

```bash
./falcon index --repo . --out artifacts
./falcon snapshot --in artifacts --out artifacts
./falcon pr-pack --repo . --snapshot artifacts --base "$BASE_SHA" --head "$HEAD_SHA" --out artifacts
```

The key step for reproducibility is running [`snapshot`](cmd/falcon/main.go:1) after indexing. More details are available in [`docs/CI.md`](docs/CI.md:1).

---

## GitHub Action usage

RepoFalcon ships as a reusable composite action via [`action.yml`](action.yml:1).

Example workflows are included in this repository:

- [`/.github/workflows/repofalcon_pr_context.yml`](.github/workflows/repofalcon_pr_context.yml:1)
- [`/.github/workflows/repofalcon_then_claude_review.yml`](.github/workflows/repofalcon_then_claude_review.yml:1)

### Example: generate PR context artifacts

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

      - name: Upload RepoFalcon artifacts
        uses: actions/upload-artifact@v4
        with:
          name: repofalcon-artifacts
          path: ${{ steps.falcon.outputs.artifacts-dir }}
          if-no-files-found: error
```

### Example: run RepoFalcon before Claude review

The typical pattern is:

1. generate RepoFalcon artifacts in one job
2. upload or pass them to a downstream AI review job
3. let the reviewer consume structured outputs instead of rediscovering the repo

See [`/.github/workflows/repofalcon_then_claude_review.yml`](.github/workflows/repofalcon_then_claude_review.yml:1) for a concrete example of this pipeline shape.

---

## What RepoFalcon produces

RepoFalcon generates artifacts that can be consumed by humans, scripts, CI systems, or coding agents.

### Core graph artifacts

- `files.parquet`
- `symbols.parquet`
- `packages.parquet`
- `edges.parquet`
- `nodes.parquet`
- `metadata.json`

### Review-oriented artifacts

- `pr_context_pack.json` — structured machine-readable context for a PR
- `review_report.md` — a human-readable review summary
- [`.falcon/CONTEXT.md`](.falcon/) — a compact repository overview for coding agents

---

## How it works

RepoFalcon combines several analysis steps to build a repository graph:

1. parse source files to extract structure
2. identify symbols and relationships
3. detect imports and dependencies
4. optionally incorporate findings
5. materialize a repository snapshot
6. generate PR-oriented or agent-oriented outputs from that graph

This structure enables:

- pull-request impact analysis
- architecture exploration
- dependency mapping
- security finding correlation
- AI-assisted code review

---

## Determinism

RepoFalcon is designed to be CI-friendly and reproducible:

- repository-relative paths are canonicalized
- Parquet row ordering is stable after snapshotting
- JSON outputs are written with stable field ordering and without timestamps

Integration tests run the CLI twice on the same fixture repositories and compare logical Parquet contents plus byte-level outputs for generated review artifacts.

---

## Architecture and additional docs

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md:1) — internal architecture and design overview
- [`docs/AGENT_INTEGRATION.md`](docs/AGENT_INTEGRATION.md:1) — coding-agent setup, MCP, and static context files
- [`docs/CI.md`](docs/CI.md:1) — deterministic CI guidance and pipeline recommendations

---

## Development

### Prerequisites

- **direnv** (recommended) — automatically loads the repo environment
- **devbox** — provides a reproducible toolchain for local development

Build locally:

```bash
devbox run -- go build -o bin/falcon ./cmd/falcon
```

Run the local CI equivalent:

```bash
devbox run -- make ci
```

Or run the main checks directly:

```bash
devbox run -- gofmt -w .
devbox run -- go vet ./...
devbox run -- go test ./...
```

---

## Status

RepoFalcon is under active development.

Contributions, feedback, and ideas are welcome.

---

## License

MIT License
