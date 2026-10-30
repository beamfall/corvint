# Decision 0023 — `query` accepts tasks to 8,000 characters and `impact` admits root-package Go files

Date: 2026-09-02. Status: accepted. Authority: repository owner, verbatim instruction "do all
three, then rerun" (2026-09-02), in reply to the recommendation naming, as the next slice after the
trial's first observation, "removing the corvint arm's silence: it refused 20 of 50 tasks": the query
length cap (11 tasks), root-level changed files in `impact` (2 tasks), and `impact` for non-Go
changed files (7 tasks).

## What is decided

1. `GPK-V0-028`'s task bound rises from 2,000 to 8,000 characters (`maxQueryChars` in
   `internal/contextindex/query.go`; help topic `query`). A failure excerpt is a task, and every
   trace2code task over the old bound was refused whole. The tokenisers the query path runs are
   linear in the task, the packet budget is unchanged, and no ranking rule reads the task's length.
   The Python oracle keeps `MAX_QUERY_CHARS = 2000` and is deliberately unrepaired (DR-0016). The
   existing parity case `query-repository-task-oversized` carried a 2,001-character task, which the
   candidate now answers; it is re-captured with an 8,001-character task that both runtimes refuse
   and declared known-divergent on the one stderr difference, the bound each refusal names.
2. `GPK-V0-027` no longer rejects a root-package `.go` path. Rule (a) resolves a root-package
   file's import path to the module path itself (go.dev/ref/mod: the module path is the import
   path of the module's root directory), so an importer naming the module reaches it. The oracle
   resolves the same file to `module/.`, which no import names, and so answers with the changed
   path and its same-package tests but never a reverse importer (DR-0017); the new parity case
   `impact-go-root` (a root file with a sibling test and a nested importer) is known-divergent
   under five validated rewrites that remove exactly the importer row, its coverage counts, and its
   verification command from the candidate's bytes, and the register records the disagreement. `range_impact.go` keeps its own root refusal; it is not on this path and is
   left for its own change.
3. Item three of the instruction changes nothing: decision 0015 already admits `.py` and the web
   suffixes to `impact`, and the seven refused changed files were five Rust, one Java, and one
   reStructuredText, which `GPK-V0-027` refuses by design (decision 0007 D2). They stay refused;
   widening to Rust and Java is a separate ruling the owner has not made.
4. The rerun is a development observation over heldout-v1 (`benchmarks/README.md`: the set became
   development with the first observation), same harness settings as decision 0022, and is read
   beside the first observation, never in place of it.

## Consequences

The corvint arm can answer 13 of the 20 tasks it refused (11 query, 2 root-package). The help texts
and `GPK-V0-027`/`GPK-V0-028` are amended in this change; `conformance/cli-parity-v0` gains
`impact-go-root` (parity 133); the divergence register gains DR-0016 and DR-0017. `src/` is not
edited.
