# falcon — evaluation, improvements & observations (2026-06-28)

A field report from putting falcon side-by-side with **graphify** on a real
~2.8k-file Go+TS codebase (iterion), the improvements that followed, and what we
learned testing the optional LLM layer. Everything here is reproducible; numbers
are from `falcon v0.6.4` and a local Ollama.

## TL;DR

- falcon started this evaluation **without a symbol-level call graph** — its
  `falcon_symbol_lookup` advertised callers/callees but no `CALLS`/`REFERENCES`
  edges were ever produced. That made it a *package-level* tool, behind graphify
  for "who calls X / what does X call".
- We added the missing call graph (Go via `go/ast`, JS/TS/Python/Java via
  tree-sitter, with receiver typing), then ported graphify's best **deterministic**
  ideas (Louvain communities, confidence rubric, a work-memory reflection loop,
  insights, benchmark) and added an **opt-in, local-first LLM** enrichment layer
  for community labels.
- Result: for **agent-driven code understanding**, falcon now matches graphify's
  call-graph value and keeps its own edge — MCP-native, deterministic, Go, fast,
  Parquet/CI, PR-impact, fleet. graphify stays ahead only on **multi-modal** and
  **NL-query**, which are LLM-bound and out of falcon's lane by design.

## 1. The gap that started this

On iterion, `falcon_symbol_lookup` returned only a symbol's location — no
relationships — even though the rendering code and the `edges.parquet` schema
supported them. Root cause: the extractors emitted only `CONTAINS`/`IMPORTS`
(file/package level); `EdgeCalls`/`EdgeReferences` were defined but never
produced. So the "knowledge graph" had no symbol→symbol edges — the single most
useful thing for code navigation.

graphify, by contrast, builds a symbol-level call graph (tree-sitter + a
second-pass for inferred calls) and exposes `explain`/`path`. So on a like-for-like
call-graph question, graphify answered and falcon did not.

## 2. What was built

### 2.1 Robust, gitignore-aware scanning
falcon's walker ignored `.gitignore`; `falcon sync` at iterion's root walked into
a gitignored vendored Go toolchain and **crashed after ~2 min** on an
intentionally-unparseable test file. Fixes:
- respect `.gitignore` + `.falconignore` (own deterministic matcher, no git
  shell-out), expanded skip-set (`.iterion`, caches, agent state), and **skip
  nested git repos/worktrees** (a subdir with its own `.git`) — which excludes
  sibling worktrees without naming them;
- **resilient extraction**: a single unparseable file is logged and skipped, not
  fatal;
- drop minified/bundled JS by name marker and by structural signature.

Effect: `falcon index` at iterion root went from **2-min crash → 2.9s, 0 skipped**.

### 2.2 Symbol-level call graph (the core fix)
- **Go**: `go/ast` walk of every function body → `CALLS` edges, with
  intra-function **receiver typing** (`x.M()` resolves to `T.M` from
  receiver/params/`x := T{}`/`var x T`).
- **JS/TS/Python/Java**: a shared tree-sitter pass captures call sites and
  attributes them to the enclosing symbol by position; class methods are now
  captured as `Class.method` symbols. Receiver typing covers `this`/`self` → the
  enclosing class, plus typed params/locals (Java types, TS `: T`, Python hints)
  and `new T()`.
- A name-based resolver assigns a **confidence rubric** (see 2.4).

Effect on iterion/pkg: edges grew from ~35k (file/package only) to ~79k with the
symbol graph; `symbol_lookup`, and a new `path`, now answer real call-graph
questions.

### 2.3 Deterministic query surface (CLI + MCP)
New, no-LLM, exposed both as CLI one-liners (scriptable/CI) and MCP tools:
`falcon symbol`, `falcon path`, `falcon hubs`, `falcon communities`,
`falcon insights`, `falcon benchmark` (+ `falcon_path/hubs/communities/insights/
benchmark` MCP tools). This closes the gap with graphify's `explain`/`path` while
staying daemon-free and deterministic.

### 2.4 Lessons ported from graphify (deterministic only)
After studying graphify's source/docs we backported the ideas that fit falcon's
"deterministic, no-LLM-by-default" doctrine:
- **Louvain communities** (modularity) replacing label propagation — LPA
  collapsed dense hubs into one giant 2678-symbol blob; Louvain gives a balanced
  117-cluster partition, deterministically.
- **Confidence rubric** EXTRACTED / INFERRED / AMBIGUOUS on every edge; small
  ambiguities are emitted to all candidates and **flagged** (with `(?)` in
  lookups) instead of silently dropped.
- **Work-memory reflection loop** (`falcon remember` / `falcon reflect`,
  + `falcon_remember`): records query outcomes (useful/dead_end/corrected),
  time-decayed and corroboration-gated, into a `LESSONS.md` the next session
  preloads — the graph *learns which sources pay off*, with no LLM.
- **Insights** (surprising cross-cluster bridges + suggested questions) and
  **benchmark** (token-reduction estimate).
- **Centralized validation** (`internal/secure`): git-ref option-injection guard
  on `pr-pack`, label sanitization, path containment.
- **Worked examples** (`docs/worked/`): committed, reproducible case studies.

### 2.5 Optional local-first LLM layer
A new `internal/llm` (OpenAI-compatible client, default = local Ollama) powers
`falcon label`, which names the deterministic communities and writes them to a
**separate** `community_labels.json` — the Parquet core is never touched. It is
opt-in, needs no API key, fails soft if the backend is down, and any
OpenAI-compatible endpoint works (Ollama, or Gemini/OpenAI via their compat
endpoints + a key).

## 3. Comparison with graphify (iterion)

| Axis | falcon | graphify |
|---|---|---|
| Index speed (pkg) | ~7s (13k sym, 79k edges) | ~9s + ~5s cluster |
| Symbol-level call graph | ✅ Go (`go/ast`) + tree-sitter, receiver-typed | ✅ tree-sitter + inferred 2nd pass |
| Call-set agreement (Jaccard, 50 Go funcs) | ~0.38–0.41 vs graphify* | (reference) |
| Communities | Louvain (deterministic) | Leiden (finer) |
| Confidence model | EXTRACTED/INFERRED/AMBIGUOUS | same doctrine |
| Agent integration | **MCP-native** (13 tools) | CLI + skill |
| Determinism / Parquet / PR-impact / fleet | ✅ | partial |
| Work-memory learning loop | ✅ (deterministic) | ✅ (the inspiration) |
| Multi-modal (docs/PDF/images), NL-query | ✗ by design | ✅ (LLM) |
| LLM dependency | opt-in, local-first, separate artifact | required for NL/labels/multimodal |

\* *Jaccard measures agreement, not correctness — neither tool is ground truth.
It dipped slightly after falcon began emitting flagged AMBIGUOUS edges (higher
recall → larger call-sets → lower overlap with graphify's conservative sets).
Restricted to high-confidence edges, agreement is higher.*

Cross-language call graph was validated on real repos: **Python** (psf/requests:
`get`→`request`→`Session`) and **Java** (google/gson: `Gson.fromJson`/`toJson`
per class).

## 4. LLM-layer observations (qwen vs gemma4)

We labeled the same 8 iterion communities (identical deterministic clustering)
with two local models via Ollama:

| Cluster core | qwen2.5-coder:7b (code, 4.7 GB) | gemma4:latest (general, 9.6 GB) |
|---|---|---|
| `FakeClock.Now` | **Clock Management** ✓ | Build And Deployment Workflow ✗ |
| `chanMu.Lock` (mutex) | **Concurrency Control** ✓ | Configuration Management System ✗ |
| `Logger.Warn` | **Logging and Error Handling** ✓ | Assistant Interaction Management ✗ |
| `Workflow` | Workflow Management | **Agent Workflow Management** (more specific) |
| `ClawExecutor` | ClawExecutorAPI (echoes symbol) | **Backend Execution Services** (more natural) |
| `Spec` | BuildToolsFor (echoes) | **System Resource Definitions** |

Observations / feedback:
- **A small code-specialized model (qwen, 4.7 GB) was more *accurate*** on
  clearly-typed clusters (clock, mutex→concurrency, logging) where the larger
  general model (gemma4, 9.6 GB) drifted. gemma4 produced more **polished,
  narrative** labels where qwen just echoed a symbol name. Caveat: 8-sample,
  some giant/noisy clusters — a tendency, not a rigorous eval.
  → **Product takeaway:** `qwen2.5-coder` is a sensible default for code-label
  quality; reach for a general model when you want human-readable phrasing.
- **Fail-soft works as designed.** When a model was absent, falcon emitted
  `WARN llm labeling failed; keeping unlabeled clusters` and a clear error,
  leaving the deterministic clusters and Parquet intact — no crash.
- **Environment, not falcon, is the only friction.** `gemma4` needs a recent
  Ollama: on 0.9.6 the pull returned `412 Precondition Failed`; after
  `curl -fsSL https://ollama.com/install.sh | sh` (→ 0.30.11) it pulled (9.6 GB)
  and labeled fine. `--model` makes swapping models a one-flag change.
- **Claude Code OAuth was deliberately not used**: the Anthropic forfait is
  scoped to Claude Code by the Consumer Terms, so it isn't a legitimate backend
  for a separate tool. falcon stays local-first / bring-your-own-key.

## 5. Verdict & recommendations

- **For an agent understanding code (the primary use case), falcon now leads**:
  MCP-native querying in the agent's loop, a multi-language call graph at parity,
  plus deterministic capabilities graphify lacks out-of-the-box here
  (work-memory, insights, benchmark, hubs/communities) and an optional
  local-first LLM that absorbs the rest without sacrificing determinism.
- **Keep the boundary**: don't chase parity for its own sake. Multi-modal and
  LLM-NL are graphify's lane; pulling them into falcon would dilute its
  deterministic/CI identity. If you ever need them, run the two tools side by
  side.
- **One optional LLM, local-first, never in the core** is the right design: the
  Parquet graph is always built without an LLM; enrichment is opt-in and lives in
  separate artifacts.

## 6. Roadmap / backport process

- Stay deterministic-first; add LLM only as opt-in enrichment in separate
  artifacts.
- **Backport from graphify "à termes"**: periodically `git pull` the upstream
  clone, diff its CHANGELOG, and port only fixes that are deterministic,
  no-LLM-by-default, and useful to the code call graph — citing the upstream
  commit. Multi-modal / NL-LLM stays upstream.
- Possible next deterministic wins: receiver typing for chained/returned
  receivers; an inferred second-pass for calls; finer community resolution.
