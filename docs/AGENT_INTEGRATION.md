# Coding Agent Integration

RepoFalcon can expose its code knowledge graph to coding agents like Claude Code, Roo Code, Cline, and Cursor. This gives agents structured awareness of your repository's architecture, dependencies, and symbol relationships.

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

## Prerequisites

Generate the graph artifacts first:

```bash
# Build
go build -o falcon ./cmd/falcon

# One command does everything (index + snapshot + context file)
./falcon init --repo .
```

This creates `.falcon/artifacts/` (Parquet graph data) and `.falcon/CONTEXT.md` (markdown summary).

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

---

## Agent Setup Guides

### Claude Code

**Option A: Static context file**

Place the context file where Claude Code reads project context:

```bash
# Alongside CLAUDE.md (auto-loaded)
./falcon agent-context --snapshot .falcon/artifacts --out AGENTS.md
```

Or reference it from your `CLAUDE.md`:

```markdown
For repository architecture details, see .falcon/CONTEXT.md
```

**Option B: MCP server**

Add to `.claude/settings.json`:

```json
{
  "mcpServers": {
    "falcon": {
      "command": "./falcon",
      "args": ["mcp", "serve", "--snapshot", ".falcon/artifacts"]
    }
  }
}
```

Claude Code will then have access to all `falcon_*` tools for querying the graph.

### Roo Code

**Option A: Static context file**

```bash
mkdir -p .roo/rules
./falcon agent-context --snapshot .falcon/artifacts --out .roo/rules/architecture.md
```

**Option B: MCP server**

Configure the MCP server in Roo Code's settings following its MCP configuration format, pointing to `./falcon mcp serve --snapshot .falcon/artifacts`.

### Cline

**Option A: Static context file**

```bash
./falcon agent-context --snapshot .falcon/artifacts --out .clinerules/architecture.md
```

**Option B: MCP server**

Configure in Cline's MCP settings:

```json
{
  "falcon": {
    "command": "./falcon",
    "args": ["mcp", "serve", "--snapshot", ".falcon/artifacts"]
  }
}
```

### Cursor

**Static context file:**

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
