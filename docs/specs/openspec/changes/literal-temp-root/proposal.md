# Change proposal: prove temp-root and bounded-climb literals, and scope the three hub packages

Change name: `literal-temp-root`
Ticket: V1-5201 (P1, milestone `v1-0`)
Product requirement served: PROD-TEST-001 and PROD-TEST-002 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/affected-plan-selection-v0.md` (prefix `AFP-V0`)
Requirement IDs this change adds: AFP-V0-107 to AFP-V0-114 (reserved for V1-5201 on 2026-11-10)
Intent status: proposed (written 2026-11-10; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No product code changes with this proposal.
Layout: `proposal.md`, `design.md`, `tasks.md`, `specs/literal-temp-root/spec.md`.

## Why

V1-5201 reports that a docs-only change selects the Go packages `cmd/corvint`,
`internal/contextindex` and `internal/liveverify/affected`, which sets a cost floor of 0.32. The
ticket assumes the cause is path literals in tests that write fixtures under a temporary root.
The measurement below shows that assumption is only part of the cause.

### What exists today (measured at 2913fdc7e)

- Collection: `internal/liveverify/affected/golang/golang.go:521` lexes every string literal after
  the imports (`pathTokens`, :774) into path tokens (AFP-V0-021). A token is dropped only by
  test metadata (AFP-V0-3334), a bounded `os.Getwd` join (AFP-V0-2950) or the per-line marker
  `//corvint:fixture-path` (AFP-V0-551). No hub uses the marker.
- Rule (d) is applied per token, not per use: `escapesPackage` (`golang/unbounded.go:110`) marks a
  unit an unbounded reader when any token climbs out of the package with `..`, plus `os.Getwd` and
  `--show-toplevel` (`golang.go:575`). Such a unit is selected for every dirty path
  (`readers.go:313`).
- Resolution: `Graph.resolves` (`readers.go:550`) matches any token for Go units. Lone `docs` or
  `specs` tokens match any path below those directories.
- Declared read scopes (AFP-V0-023, AFP-V0-161) already remove a unit from rule (d)
  (`graph.go:771`) and from rule (c) outside the scope. Full CI enforces a scope with the Landlock
  wrapper. None of the three hubs declares one in `.corvint/test-read-scopes.json`.

### What the measurement found

Dirty paths: `AGENTS.md`, `Makefile` and `docs/specs/eol-policy-gate-v0.md`. The classes are:
(a) joined to a temporary directory, (b) an in-memory fixture map or struct, (c) content or a
message, (d) a real repository read, (e) cannot tell. Production occurrences are counted
separately.

| Hub | Path | Prod | a | b | c | d | e |
|---|---|---|---|---|---|---|---|
| cmd/corvint | AGENTS.md | 9 | 4 | 9 | 7 | 0 | 3 |
| cmd/corvint | Makefile | 4 | 4 | 12 | 9 | 0 | 6 |
| cmd/corvint | eol-policy-gate | 13 | 2 | 34 | 15 | 4 | 18 |
| contextindex | AGENTS.md | 6 | 1 | 66 | 32 | 0 | 11 |
| contextindex | Makefile | 7 | 0 | 18 | 4 | 0 | 0 |
| contextindex | eol-policy-gate | 7 | 0 | 1 | 18 | 0 | 0 |
| affected | AGENTS.md | 1 | 0 | 10 | 0 | 0 | 2 |
| affected | Makefile | 3 | 0 | 135 | 5 | 1 | 75 |
| affected | eol-policy-gate | 0 | 0 | 23 | 2 | 1 | 13 |

- Class (a) includes same-package helpers that only return a temporary root. Only 2 occurrences
  are joined directly to `t.TempDir()`.
- Production code in every hub holds these literals as reads of a root that a caller supplies.
- For the docs path, no literal names the file. The matches are `docs`, `specs` and `../docs`.
- A temp-root rule frees 0 of the 30 PATH_LITERAL_READER (unit, path) pairs in the three plans.
  The reason: each pair also has an occurrence in class (b), (c), (d) or (e), or in production.
- A plan for `qzx-probe-nothing.md`, a path no literal names, still selects all three hubs as
  UNBOUNDED_READER. The causes are an `os.Getwd` in `cmd/corvint/init_adopt.go:44`, and `".."`
  literals in `internal/contextindex/analyzer_schema_test.go` and
  `internal/liveverify/affected/affprec_repo_test.go`. Removing every path token would not
  deselect any hub.

## What Changes

1. A temp-root literal proof that works on each occurrence (AFP-V0-107 to AFP-V0-110). It is
   needed for precision but frees no hub by itself.
2. A bounded-climb proof that extends AFP-V0-2950 from `os.Getwd` to literal `..` joins, climbing
   literals and repository-root locals, plus a comparison-only exemption (AFP-V0-111 to
   AFP-V0-113). This is the step that addresses rule (d) for `internal/contextindex` and
   `internal/liveverify/affected`.
3. Every selection reason is reported, not only the first (AFP-V0-114). The first-witness report is
   what hid rule (d) behind PATH_LITERAL_READER in V1-5201.
4. Declared read scopes for the three hubs, measured under the Landlock wrapper (task 6). This is
   the only lever that can free `cmd/corvint`, whose production code finds the repository root at
   run time.

## Capabilities

### New Capabilities

- None. The rules extend the Go plugin's literal collection.

### Modified Capabilities

- `affected-plan-selection-v0.md`: AFP-V0-021 collection and AFP-V0-012 rule (d) gain the
  occurrence-level proofs. AFP-V0-2950 is generalized.

## Non-goals

- No new marker or configuration file. AFP-V0-551 and AFP-V0-023 stay as they are.
- No rule for class (c) content literals. These remain path tokens, so some hubs stay selected for
  docs paths until they declare a scope.
- No type checking (go/types) and no call graph across packages. Proofs use one package's
  untyped ASTs, like AFP-V0-828 and AFP-V0-2950.
- No change to production-literal handling. A production occurrence always keeps its token.
- The hub test sources are not edited so that the analyzer stops seeing a literal.

## Impact

- Code: one new file in `internal/liveverify/affected/golang/`, hooks in `pathTokens` and
  `escapesPackage`, and the witness list in `readers.go`.
- Plans: equal or smaller selections, never a new miss. Each AFP-V0-114 plan lists more reasons.
- Ticket: V1-5201 closes only when task 7's re-measurement shows a hub deselected for a docs-only
  change, or it records which hub stays selected and why.
