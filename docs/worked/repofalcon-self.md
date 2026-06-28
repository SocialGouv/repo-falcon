# Worked: falcon on its own source (self-host)

falcon version `v0.6.4`. Deterministic, no LLM. Reproduce from the repo root:

```bash
make build
./bin/falcon index --repo . --out /tmp/falcon-self
```

```
index complete  files=185 packages=106 symbols=584 edges=2667 skipped=0
```

## Token benchmark

```bash
./bin/falcon benchmark --snapshot /tmp/falcon-self
```

```
- Corpus: 185 files, ~144015 tokens (naive full-context)
- Graph: 584 symbols, 2667 edges
- Avg query cost: ~156 tokens (symbol lookup, 10-symbol sample)
- Reduction: 923.2x fewer tokens per query
```

## Hubs (degree centrality)

```bash
./bin/falcon hubs --top 6 --snapshot /tmp/falcon-self
```

```
1. newIndexCmd  (func)  — 29 edges — internal/cli/index.go:20
2. GraphIndex   (type)  — 27 edges — internal/mcp/graph.go:15
3. NewRootCommand (func) — 22 edges — internal/cli/root.go:20
4. loadTestFleetGraph (func) — 20 edges — integration/fleet_integration_test.go:584
5. ExtractPythonFile (func) — 19 edges — internal/extract/python.go:22
6. Detect (func) — 18 edges — internal/workspace/detect.go:28
```

The hubs are exactly falcon's core abstractions: the command wiring
(`newIndexCmd`, `NewRootCommand`), the in-memory graph (`GraphIndex`), and the
per-language extractors (`ExtractPythonFile`, `Detect`).

## Communities (Louvain)

```bash
./bin/falcon communities --top 6 --snapshot /tmp/falcon-self
```

```
1. Detect (49 symbols)
2. newIndexCmd (45 symbols)
3. GraphIndex (43 symbols)
4. loadTestFleetGraph (42 symbols)
5. ExtractPythonFile (36 symbols)
6. falconBin (28 symbols)
```

## Symbol lookup (call graph)

```bash
./bin/falcon symbol ChangedFiles --snapshot /tmp/falcon-self
```

```
- File: internal/git/changed_files.go:33
- Called by: TestChangedFiles_BasicAndRename, newPRPackCmd
```

`ChangedFiles` is reached from `newPRPackCmd` — the very path the `secure`
git-ref validation guards.
