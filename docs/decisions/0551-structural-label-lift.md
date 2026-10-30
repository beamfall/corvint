# Decision 0551 — the structural label is lifted by one go1.27.1 corpus and parser-closure admission

Date: 2026-10-21. Status: accepted. Authority: repository owner, in chat on 2026-10-21. The owner
first said "fix the limit" and chose "Lift it before 1.0". The owner then approved this exact
method: "qualify the corpus once on go1.27.1 (darwin + Linux, with the go1.24 comparison), and admit
patch releases by parser-closure identity with no corpus rerun; runtime/os differences are outside
the closure." Ticket: V1-2327. It amends decisions 0522 and 0547 without superseding them.

## Context

Both stable verifiers reported `runtime.structuralQualification`
`EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS` on every admitted tuple, because no full structural corpus
had run (`CEM-V1-050`). Each tuple packet qualified a patch release by comparing a 70-package
`go list -deps` closure against its predecessor. That closure includes `runtime`, `os`,
`internal/poll` and `internal/godebugs`, so it differs on every patch release for reasons the
predicates cannot observe. Taken at face value, it would demand a corpus rerun on every release.

## Decision

- The structural predicates are qualified once, on `go1.27.1`, the tuple the frozen S0E packet
  records. The qualifying corpus is sealed under `protocol/cem-1.0/structural-corpus/`
  (`CEM-V1-369`). It contains:
  - every tracked `.go` file of the Corvint tree at `ef2a01db5` (4824) and its 12
    `internal/cem/verify/testdata/structural` fixtures, 4836 files in all;
  - 26 version-sensitive cases. 25 are committed. The zero-byte `empty-file.go.txt` is declared in
    `CORPUS.json` and in both harnesses instead, because CEM 1.0 cannot bind an empty-file
    creation (`CEM-V1-300`).

  Each file yields scanner, parser, printer and formatter digests plus up to five derived
  perturbation pairs. The four portable predicates judge each pair in both directions, and the
  native verifier must reach the same verdicts.
- A Go release is admitted without a corpus run only when its parser-closure digest equals the
  qualified tuple's (`CEM-V1-370`, `tools/cem-structural-closure`). The closure is `go list -deps`
  of the six `go/*` roots for four platforms, minus the runtime, scheduler, system-call, file-system
  and GODEBUG packages. The predicates read only in-memory bytes, so those packages are outside what
  they observe. The lexical path packages `path/filepath` and `internal/filepathlite` stay in,
  because `go/scanner` cleans and joins `//line` file names with them. go1.27.0 through go1.27.3
  share one closure digest, `83e19e1d…877ec7` (35 packages, 323 files). go1.24.13 has another,
  `e11da95b…3a232c` (34 packages, 313 files).
- Both verifiers now report `QUALIFIED_EXACT_TUPLE` on an admitted tuple (`CEM-V1-371`). The frozen
  packet's `runtime` values are re-emitted through `make cem-stable-regen-replace` (`CEM-V1-091`).
  The capture ran on go1.27.3, and `goVersion` stays `go1.27.1`: the tool maps a captured version
  back through its admitted substitution and refuses any other.
- `structuralTuples` stays `go1.27.1`, `go1.27.2` and `go1.27.3`. Both tuple packets record the
  closure digest and the new label.

## Evidence

The runs, hosts and digests are in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-v1-2327-structural-label-lift.md` and
`protocol/cem-1.0/structural-corpus/QUALIFICATION.json`.

## Limits

- Windows: NOT_RUN. No Windows host ran the corpus.
- The Linux host is a colima VM (linux/arm64) on the darwin/arm64 host, running static test binaries
  cross-compiled on darwin. Its observations are byte-identical to darwin's for go1.27.1 and
  go1.24.13. linux/amd64 is NOT_RUN.
- The frozen `cancelledContextExpectedResult` snapshot in the two envelope cell files keeps the old
  label. It is not a regenerated value and no code reads it. The `INPUTS.json` note that it
  "equals" the full result case was already stale before this change.
- `R1-EXPECTED.json` and historical decisions and build logs keep the old label as history.

## Rollback

Revert the re-emission, the three emission sites and the corpus packet. The label returns to
`EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS`, and the tuple list is unchanged.
