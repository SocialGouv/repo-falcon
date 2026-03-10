# Coding Agent Integration

RepoFalcon helps coding agents such as Claude Code, Roo Code, Cline, and Cursor work from repository structure instead of discovering everything file by file.

It exposes a code knowledge graph that gives agents structured awareness of architecture, dependencies, and symbol relationships.

Two complementary integration methods are available:

1. **Static context file** (`falcon agent-context`) — a markdown or JSON summary agents can read
2. **MCP server** (`falcon mcp serve`) — a live server agents can query interactively

---

## Why Graph Knowledge Matters for Coding Agents

### The problem: agents operate blind

Coding agents today work with a flat view of the codebase. When asked to modify a file, an agent typically:

1. Reads the target file
2. Uses `grep` or `glob` to find related files — slow, noisy, and incomplete
3. Guesses at the dependency structure based on import statements it happens to see
4. Makes changes without understanding the full blast radius

This leads to common failure modes:
- **Breaking callers**: the agent changes a function signature without knowing what depends on it
- **Circular dependencies**: the agent adds an import that violates the dependency direction
- **Missed impact**: the agent fixes a bug in one place but doesn't realize three other modules re-export the affected symbol
- **Wasted context window**: the agent reads dozens of files via grep to build a mental model that the graph already has

### The solution: structured graph context

A code knowledge graph gives agents what they lack — a pre-computed, queryable map of relationships:

| Agent need | Without graph | With graph |
|---|---|---|
| "What depends on this file?" | Grep for import paths, miss indirect deps | `falcon_file_context` returns all dependents instantly |
| "What does this package contain?" | `ls` + read each file | `falcon_package_info` lists files, symbols, deps |
| "Who calls this function?" | Grep for the function name (noisy) | `falcon_symbol_lookup` returns callers via CALLS edges |
| "What's the architecture?" | Read README, hope it's accurate | `falcon_architecture` returns live package structure |
| "What will this change break?" | Manual review | Impact analysis via dependency traversal |

### Concrete improvements

**Safer refactoring**: Before renaming `ExtractGoFile`, the agent queries `falcon_symbol_lookup("ExtractGoFile")` and sees it's called by `IndexCommand` in `internal/cli/index.go`. It knows to update both files.

**Architecture compliance**: The agent queries `falcon_architecture` and sees the dependency direction is `cli → extract → graph`. When asked to add a utility to `graph/`, it knows not to import from `cli/`.

**Faster navigation**: Instead of running 5-10 grep queries to understand a module, one `falcon_package_info` call returns all files, symbols, and dependency relationships.

**Smaller context window usage**: The graph summary is compact (a few KB) compared to reading raw source files (potentially hundreds of KB). The agent spends its context budget on reasoning, not discovery.

### How agents discover and use the graph

There are two layers:

**Discovery** — The agent needs to know the graph exists:
- **MCP tools** are auto-discovered: when configured in agent settings, the agent sees all `falcon_*` tools and their descriptions at session start
- **Static context files** are auto-read when placed in agent-specific locations (`CLAUDE.md`, `.roo/rules/`, `.cursor/rules/`)

**Usage guidance** — The agent needs to know *when* to use the graph. Add instructions to your project's agent configuration (e.g., `CLAUDE.md`):

```markdown
## Code Knowledge Graph

This repo has a RepoFalcon knowledge graph available via MCP (`falcon_*` tools).

- Before modifying a file, call `falcon_file_context` to understand its dependencies and dependents
- Before refactoring, call `falcon_architecture` to understand package boundaries and dependency direction
- To find all usages of a symbol, use `falcon_symbol_lookup` instead of grep
- Use `falcon_search` for structural queries (finding packages, symbols by name)
```

This makes agents proactive about querying the graph rather than falling back to file-by-file discovery.

---

## Quick Start

If you want the fastest path for Claude Code or another coding agent, build the CLI and initialize the repository:

```bash
devbox run -- go build -o falcon ./cmd/falcon
./falcon init --repo .
```

This runs the full pipeline:
1. **Index** — parses the repository and extracts the code graph
2. **Snapshot** — materializes a deterministic snapshot as Parquet files
3. **Agent context** — generates `.falcon/CONTEXT.md` (markdown summary)
4. **Agent setup** — interactively asks which coding agents you use and configures them

This is the same workflow highlighted in [`README.md`](README.md) because it is usually the shortest path from “I want my agent to understand this repo” to a usable setup.

### Why this is useful for Claude Code

When Claude Code is asked to refactor a package, rename a symbol, or review the blast radius of a change, RepoFalcon gives it precomputed context instead of forcing it to infer architecture from raw file reads alone.

In practice, that means Claude Code can:

- inspect dependencies before editing a file
- understand package boundaries before adding imports
- look up symbol relationships structurally instead of relying only on text search
- refresh the graph after major refactors

### Interactive agent setup

When running in a terminal, `falcon init` prompts you to select your coding agents:

```
Which coding agents do you use? (select numbers, comma-separated)

  1) Claude Code
  2) Roo Code
  3) Cline

Enter selection (e.g. 1,2 or 'all'), or press Enter to skip:
```

For each selected agent, falcon automatically:
- Creates/updates the agent's instruction file with falcon MCP tool guidance
- Configures the MCP server in the agent's settings

### Non-interactive mode

Use the `--agents` flag to skip the prompt:

```bash
# Configure specific agents
./falcon init --repo . --agents claude,roo,cline

# Skip agent setup entirely
./falcon init --repo . --agents none
```

### What gets created

| Agent | Instruction file | MCP config |
|-------|-----------------|------------|
| Claude Code | `CLAUDE.md` (marker section) | `.mcp.json` |
| Roo Code | `.roo/rules/falcon.md` | `.roo/mcp.json` |
| Cline | `.clinerules` (marker section) | `.cline/mcp_settings.json` |

The instruction files contain guidance on when and how to use the `falcon_*` MCP tools. The MCP config files point to the falcon binary with the correct arguments.

Files that support marker sections (`CLAUDE.md`, `.clinerules`) use `<!-- BEGIN FALCON -->` / `<!-- END FALCON -->` markers for idempotent updates — running `falcon init` again replaces the falcon section without touching your other content.

### Example [`CLAUDE.md`](CLAUDE.md:1)

After running `falcon init --repo . --agents claude`, your `CLAUDE.md` will contain:

```markdown
<!-- BEGIN FALCON -->
## RepoFalcon Code Knowledge Graph

This repository has a code knowledge graph available via MCP tools (`falcon_*`).

**When to use:**
- Before modifying a file, call `falcon_file_context` to see its dependencies and dependents
- Before refactoring, call `falcon_architecture` for package boundaries and dependency direction
- To find all usages of a symbol, use `falcon_symbol_lookup` instead of grep
- To search structurally, use `falcon_search` for packages, files, or symbols by name
- After major refactoring (renamed packages, moved files), call `falcon_refresh` to re-index

**Static context:** See `.falcon/CONTEXT.md` for a full architecture summary.
<!-- END FALCON -->
```

And `.mcp.json` will contain a matching MCP server entry (merged with any existing settings):

```json
{
  "mcpServers": {
    "falcon": {
      "command": "/absolute/path/to/falcon",
      "args": ["mcp", "serve", "--snapshot", "/absolute/path/to/.falcon/artifacts", "--repo", "."]
    }
  }
}
```

---

## Method 1: Static Context File

Generate a markdown summary of the code knowledge graph:

```bash
./falcon agent-context --snapshot .falcon/artifacts --out .falcon/CONTEXT.md
```

Options:

| Flag | Default | Description |
|------|---------|-------------|
| `--snapshot` | `.falcon/artifacts` | Path to snapshot artifacts directory |
| `--out` | `.falcon/CONTEXT.md` | Output file path |
| `--format` | `markdown` | Output format: `markdown` or `json` |

The generated file contains:
- Language and file statistics
- Internal package map with symbols and dependencies
- Internal dependency graph
- External dependency listing

### When to regenerate

Regenerate the context file whenever the codebase changes significantly:
- After merging PRs
- As a pre-commit hook
- In CI (commit the file or upload as artifact)

### Example: pre-commit hook

```bash
#!/bin/sh
./falcon init --repo .
git add .falcon/CONTEXT.md
```

---

## Method 2: MCP Server

Start an MCP server that agents can query dynamically:

```bash
./falcon mcp serve --snapshot .falcon/artifacts
```

The server loads graph artifacts at startup and serves over stdio using the Model Context Protocol (JSON-RPC 2.0).

### Available Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `falcon_architecture` | High-level overview: languages, packages, dependency counts | none |
| `falcon_file_context` | Symbols, imports, and dependents for a file | `path` (required) |
| `falcon_symbol_lookup` | Symbol details and relationships | `name` (required), `kind` (optional) |
| `falcon_package_info` | Package contents, dependencies, dependents | `name` (required) |
| `falcon_search` | Search files, symbols, or packages by name | `query` (required), `scope` (optional) |
| `falcon_refresh` | Re-index the repository and reload the graph | none |

---

## Agent Setup Guides

> **Recommended**: Use `falcon init --repo .` for automatic setup. The manual instructions below are for reference or custom configurations.

### Claude Code

**Automatic** (recommended): `falcon init --repo . --agents claude`

**Manual setup:**

1. Add MCP server to `.mcp.json`:

```json
{
  "mcpServers": {
    "falcon": {
      "command": "/path/to/falcon",
      "args": ["mcp", "serve", "--snapshot", "/path/to/.falcon/artifacts", "--repo", "."]
    }
  }
}
```

2. Add usage instructions to `CLAUDE.md` (see example above)

3. Optionally reference the static context file:

```markdown
For repository architecture details, see .falcon/CONTEXT.md
```

### Roo Code

**Automatic** (recommended): `falcon init --repo . --agents roo`

**Manual setup:**

1. Create `.roo/rules/falcon.md` with falcon usage instructions
2. Configure MCP in `.roo/mcp.json`:

```json
{
  "mcpServers": {
    "falcon": {
      "command": "/path/to/falcon",
      "args": ["mcp", "serve", "--snapshot", "/path/to/.falcon/artifacts", "--repo", "."]
    }
  }
}
```

### Cline

**Automatic** (recommended): `falcon init --repo . --agents cline`

**Manual setup:**

1. Add falcon usage instructions to `.clinerules`
2. Configure MCP in `.cline/mcp_settings.json`:

```json
{
  "mcpServers": {
    "falcon": {
      "command": "/path/to/falcon",
      "args": ["mcp", "serve", "--snapshot", "/path/to/.falcon/artifacts", "--repo", "."]
    }
  }
}
```

### Cursor

**Static context file** (no automatic setup yet):

```bash
mkdir -p .cursor/rules
./falcon agent-context --snapshot .falcon/artifacts --out .cursor/rules/architecture.mdc
```

### Generic / Other Agents

Any agent that reads markdown files can use the static context output. Any MCP-compatible agent can use the MCP server.

---

## CI Integration

### Generate context in CI

```yaml
- name: Generate agent context
  run: ./falcon init --repo .

- name: Upload context
  uses: actions/upload-artifact@v4
  with:
    name: falcon-context
    path: .falcon/
```

### Commit context file automatically

```yaml
- name: Update agent context
  run: |
    ./falcon init --repo .
    git add .falcon/CONTEXT.md
    git diff --cached --quiet || git commit -m "chore: update code knowledge graph context"
```

---

## Output Example

The markdown output looks like:

```
# Code Knowledge Graph

## Overview
- Total files: 25
- Languages: go (20), python (5)
- Internal packages: 8
- External dependencies: 12
- Total symbols: 156 (function: 80, type: 30, method: 25, ...)

## Package Map

### internal/extract
- Files: go.go, regex.go, types.go
- Symbols: `ExtractGoFile` (function), `ExtractJSImportTargets` (function), ...
- Imports: internal/graph, go/parser, regexp
- Imported by: internal/cli

## Internal Dependency Graph
internal/cli -> internal/extract, internal/artifacts, internal/prpack
internal/extract -> internal/graph
...

## External Dependencies
- `github.com/apache/arrow-go/v18` [go] (used by: internal/artifacts)
- `github.com/spf13/cobra` [go] (used by: internal/cli)
```
