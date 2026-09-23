# Decision 0372 — external evaluation slice in the context evolution program

Date: 2026-09-23. Status: accepted scope; experimental delivery. Authority: repository owner
delegation to make owner calls and record them; ticket V1-0097 names
`docs/specs/context-evolution-program-v0.md` as the owning spec and reserves `CEP-V0-001`..`006`.

Context: ticket V1-0097 asks that Corvint packets be scored on external retrieval benchmarks
(ContextBench, arXiv 2602.05892, and the Agent Retrieval Bench already driven by
`tools/retrieval-bench`) and that the confidently-wrong trial (arXiv 2603.25764) gain an
already-fixed abstention control. Decision 0179 holds the context evolution program's hypothesis
catalog prose-only and allows numbered clauses only through a separate accepting slice's own
requirements, never by relabelling catalog prose.

The call: the owning spec gains one separate section, the external evaluation slice, with its own
`## Requirements` (`CEP-V0-001`..`006`), non-goals, failure modes, traceability and rollback. The
clauses govern only the two evaluation harnesses; none of the five hypotheses is relabelled, and
their prose stays the catalog. This satisfies 0179 in the form it names (a separate slice with its
own requirements) while keeping the owning spec the ticket fixed.

- ContextBench rows are read from a local JSON-lines export of the dataset's own columns; Parquet,
  repository cloning and the network stay out of the tool. File and line coverage/precision follow
  ContextBench's `coverage_precision` definitions with whole-file line prediction, because a packet
  names files, not spans. Symbol and byte-span granularities are not measured.
- The already-fixed control carries no gold; any `certain` claim, and any produced corvint packet
  that does not abstain, is a recorded control failure. New report fields appear only for
  ContextBench rows and control tasks, so committed reports keep their bytes.
- Results are evidence a later promotion may cite, never a promotion (`CEP-V0-006`).

Consequences: `docs/specs/INDEX.json` gains `reqPrefix` `CEP-V0` for the spec and the two tools as
implementation; `REQUIREMENTS.tsv` gains six rows. A full ContextBench run needs the 66 upstream
repositories at their base commits, which are not shipped with the dataset; until they are local it
is `NOT_RUN` (BUILD-LOG V1-0097).

Rollback: revert the V1-0097 change; the slice section, this file, the six TSV rows, and the new
tool files go together, and decision 0179's catalog rule is unchanged.
