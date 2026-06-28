# Follow-ups

Prioritized next steps, captured after the 2026-06-28 evaluation vs graphify
(see [evaluation-and-observations.md](evaluation-and-observations.md)). The bar
for "do it": deterministic, no-LLM-by-default, useful to the code call graph.

## P1 — the one real, on-brand gap: language coverage

falcon supports Go, TS/JS, Python, Java. graphify covers ~25 languages. This is
the single most valuable catch-up and it is deterministic (mostly wiring
tree-sitter grammars + a call/symbol walk per language).

- [ ] Add tree-sitter language support, roughly by demand: **Rust, C/C++, C#,
      Ruby, PHP**, then Kotlin/Swift/Scala, etc.
- [ ] Per language: symbol capture (funcs/methods/types), `IMPORTS`, call-site
      capture + receiver typing where the grammar allows, fixtures + tests.
- [ ] Keep the shared tree-sitter ref pass (`collectRefs`) generic; add per-grammar
      node-name tables rather than bespoke walkers.

## P2 — deterministic precision wins

- [ ] Receiver typing for **chained/returned receivers** (`a.b().c()`) and field
      accesses, beyond locals/params/`this`.
- [ ] Optional **inferred second pass** for calls (graphify-style) to raise recall
      on cross-file references, kept behind the AMBIGUOUS/confidence rubric.
- [ ] Finer **community resolution** on very dense graphs (the iterion `Context`
      cluster is still large); consider Leiden-style refinement.

## P3 — maturity / adoption

- [ ] Battle-test on more real repos; grow `docs/worked/` case studies.
- [ ] Keep the deterministic core stable; community/adoption follows usage.

## Out of scope by design (leave to graphify; do NOT chase parity)

- Multi-modal indexing (docs/PDF/images/video) — not falcon's lane.
- LLM-native NL query and semantic-similarity edges — falcon relies on the agent
  for NL and keeps the LLM strictly opt-in/enrichment. If ever needed, run
  graphify alongside rather than merging the roles.

## Process — backport from graphify "à termes"

- [ ] Periodically `git -C .repos/graphify pull`, diff its CHANGELOG, and port
      only fixes that are deterministic + no-LLM-by-default + call-graph-relevant,
      citing the upstream commit. Multi-modal / NL-LLM stays upstream.
