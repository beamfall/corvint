# Decision 0032 — `corvint index` writes the tree's index snapshot; `context` reads it and never writes

Date: 2026-09-02. Status: accepted. Authority: repository owner, verbatim instruction "go ahead
and build the index command" (2026-09-02), given on the storage engineer's finding that the whole
index rebuild (about 900 ms of a packet call on a 3,233-file repository) is a pure function of the
committed tree and loads from a file in milliseconds, and on the owner's goal, "I want this to
destroy the other tools in speed".

## What is decided

1. A new verb, `corvint index`, builds the committed tree's index and writes it under
   `.corvint/index/`, keyed by object format, tree OID, and a digest of the running executable
   (index-snapshot-v0). It is the only writer. `context` reads a matching snapshot, applies the
   worktree's dirty paths from `git status`, and otherwise builds as before. Invariant 4 stays
   literally true: no read verb writes derived state. The alternative, a read verb that caches with
   `mutates:true` for one file, was rejected because it amends a product invariant to fix a latency
   problem and re-opens the open defect class of read verbs writing `.corvint/`.
2. The encoding is `encoding/gob` in this slice: the load is about 20 ms against a 150 ms target
   and the file is private to one tree and one binary. The flat memory-mapped encoding the storage
   engineer prototyped (0.3 ms load, byte-deterministic) is the candidate for the invariant 7
   benchmark/format gate, not a requirement of this slice.
3. Hit and miss must produce identical packet bytes; the packet says nothing about which path
   served it.

## What was measured

On the Beamfall checkout (3,233 files): `context` without a snapshot 750 to 770 ms warm (one
1.10 s outlier); `index` 450 ms; `context` with a snapshot 390 to 410 ms, of which loading is
57 ms (two Git observations plus decode) and compiling the packet 330 ms. Output byte-identical
between hit and miss. The remaining 330 ms is the packet compiler itself, now the whole cost.

With the profiler's remaining items in the same change (parallel lexical scan, memoised
`identifierEvidence`, overlapped `git log`, decode-once sources, `BuildContext` import narrowing,
`adoptStatus`), on a 145-term review comment against `internal/plugin/plugin.go`, unloaded:
`main` 4.06 to 4.39 s, this change without a snapshot 1.26 to 1.47 s, `index` 434 to 446 ms, with
a snapshot 948 to 1,019 ms, output byte-identical to `main`. Of that hit, `siblingRows` is about
545 ms and `lexicalRows` about 360 ms, both one pass per term per source; the term table `index`
could write is the next slice (`docs/agent-memory/optimizations.md`, 2026-09-02).

## What is not decided

Who runs `index` in a session (a Claude Code hook on session start is the obvious candidate and
needs the plugin change); whether `query`, `impact`, and `prove` should read the snapshot; the
promoted encoding.
