# Worked examples

Reproducible case studies of falcon run against real code. Each captures the
exact commands and their (deterministic) output, so anyone can re-run and get
the same numbers — the committed evidence that the graph adds value.

This mirrors the practice of keeping a durable, reviewable record rather than
relying on artifacts that are gitignored and vanish.

- [repofalcon-self.md](repofalcon-self.md) — falcon indexing its own source
  (the self-host case). Fully deterministic; no LLM.

To add one: run the falcon commands against a target, paste the commands and
output verbatim, and note the falcon version (`falcon --version`). Prefer
targets others can obtain (a public repo, or falcon itself).
