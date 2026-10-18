# V1-1806: affected over-selection on a schema-ID bump

Owner input: ticket V1-1806 (no GitHub issue). `corvint affected --base 70a21da5d` on the
PR #1134 diff (branch `claude/ruby-verification-command-1796`, head `5c57314aa`) selected 157
units. AFP-V0-072 was added to `docs/specs/affected-plan-v0.md`; it is accepted under the owner's
standing auto-accept of lane requirements (2026-10-11). No CAL-V0 ID was added.

## Attribution

The retained plan (working tree of that branch, 12 dirty paths) had 157 units: `DEPENDENCY_PATH`
77, `PATH_LITERAL_READER` 46, `BINARY_EXEC` 22, `UNBOUNDED_READER` 6, `DECLARED_READ_SCOPE` 4,
`DIRECT_SOURCE_CHANGE` 2; unknowns `LANGUAGE_FRONTIER` 23 and `UNOWNED_DIRTY_PATH` 5. The
reproduction on the sealed range `70a21da5d..5c57314aa` (16 dirty paths, adding the sealed CEM,
the use-case ledger and receipt, and the build log) selected 163: `DEPENDENCY_PATH` 76,
`PATH_LITERAL_READER` 52, `BINARY_EXEC` 22, `UNBOUNDED_READER` 5, `DECLARED_READ_SCOPE` 4,
`DIRECT_SOURCE_CHANGE` 2, `ENCLOSING_PACKAGE` 2; scope `UNKNOWN` (every `docs/specs/*` path, the
build log, the conformance files and the CEM are unowned).

- Schema-ID constant alone. With only `internal/contextindex/analyzer_schema.go` dirty at the
  base (`corvint-analyzer/123` to `/124`), the plan selects 120: `DEPENDENCY_PATH` 78,
  `BINARY_EXEC` 22, `UNBOUNDED_READER` 14, `DECLARED_READ_SCOPE` 3, `PATH_LITERAL_READER` 2
  (`obscorpus`, `semescalate` through `../../internal`), `DIRECT_SOURCE_CHANGE` 1. This is
  justified at package granularity: the constant feeds `analyzerEngine()`, which names the pack
  that `snapshot.go` and `blob_shards.go` write and load, so every index consumer reads it; the
  dependency units are the reverse-import closure of `internal/contextindex`, and the binary units
  run binaries built from it. `AnalyzerSchemaID()` is also read by `internal/tasks/scopes`. A
  symbol-level narrowing would drop true dependents, so none is made.
- `PATH_LITERAL_READER` (52), classed by the tokens that name a dirty path: lone unanchored
  file names (V1-0290 class) 11 units; anchored lone tokens (`/corvint`, `/cmd/`, `/docs`,
  `/receipts`, `/.corvint/`) 8; multi-component runs 9; anchored multi-component 5; climbing 4;
  mixed 15. Of the 11 lone units, 10 are Go (`README.md` or `REQUIREMENTS.tsv` joined with
  `t.TempDir()` or a fixture root) and one is shell.
- Other notable readers: `internal/unplannedread` through the fragment run `s/a` matching
  `specs/affected-plan-v0.md`; 8 units through `.corvint/changes/`, the sealed CEM every dogfood
  change commits.
- `BINARY_EXEC` (22) are consumers of binaries built from changed packages; the data-literal
  binary edges of V1-1006 are not the driver here.

## Change

AFP-V0-072: a Go unit that names a dirty path only through a lone unanchored file-name token is a
reader only when the path is in its own subtree, has no directory, or every directory component
is a whole component of one of its tokens. Tokens that place a directory, non-Go units, the CEM
sidecar narrowing, rule (d) and declared read scopes are unchanged. A Go test runs in its package
directory; a unit that is not a rule (d) reader reaches another directory only through a literal
naming it or a run-time root, which no literal index bounds (AFP-V0-021).

## Evidence

- PR #1134 range, same binary source before and after: 163 to 156 units; `PATH_LITERAL_READER`
  52 to 45; nothing added. Dropped, all through `docs/specs/README.md`: `internal/depsource`,
  `internal/evidencereach`, `internal/gokernel`, `internal/liveverify/affected/cfamily`,
  `internal/update`, `tools/cem-stable-regen`, `tools/cw-trial`. Each source was read: temp-dir
  or fixture `README.md` files, a release-archive member list, and a packet README under
  `stableDir`; none reads `docs/specs/README.md`. `internal/genesis`, `internal/dashboard/roadmap`
  and `tools/docs-ci-plan` stay selected because their tokens name `docs` and `specs`.
- Schema-ID-only plan: 120 before and 120 after.
- Mutation: making `resolves` return true for every non-sidecar path fails three cases of
  `TestFileNameOnlyReadersMustPlaceThePath_AFPV0072`.

## Review round 1 (Codex)

Three findings, each fixed with a case in `TestFileNameOnlyReadersKeepWhatTheyCanReach_AFPV0072`:
a reader joining a parent-only `..` with a file name was dropped (P1; the bases now include every
ancestor of the unit's directory once a token has a `..` component); a directory component with a
space or non-ASCII letter, which the Go lexicon splits, could never be in the vocabulary (P2; such
a component now never narrows); and a narrowed unit's build-constraint frontier vanished, turning
an `UNKNOWN` plan `BOUNDED` against AFP-V0-072(4) (P2; `readers` returns the narrowed set and
`Select` names its unit frontier). Each clause reverted alone fails its case. The PR #1134 range
plan is unchanged by the repair: 156 units, the 32 unknown entries identical to the 163-unit
plan's.

## Residuals

Left open and named: anchored lone tokens and the shell lone reader (rest of V1-1766), V1-1006,
the `s/a` fragment match, and readers of `.corvint/changes/` sealed CEMs (an analogue of the
sidecar narrowing).
