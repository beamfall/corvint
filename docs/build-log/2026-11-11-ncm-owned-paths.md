# 2026-11-11: non-code map, owned paths and review fixes (V1-5424)

Second build round of change `non-code-map` (`docs/specs/non-code-map-v1.md`), after the
independent review of the first round (`docs/build-log/2026-11-11-non-code-map.md`).

## Orchestrator decision

Orchestrator ruling, binding, under the owner's 2026-11-02 delegation (the same ruling went to lane
rnc): a non-code path that a Go rule already attributes is OWNED and must not force a full run. Go
attribution means the enclosing Go package directory owns the file (its tests and dependents are
selected, as `select.go` already does), go.mod/go.sum readers, a package's testdata, and declared
readers. A full run with reason `unowned-path` follows only when no runner attributes the path at
all. NCM-V1-001 and delivered rule 2.6 are amended (requirement IDs unchanged); NCM-V1-005 drops
go:embed inputs from the never-narrow list, because their embedding package owns them.

## How the ruling is applied

1. `noncodemap.Decide` takes one `Attributed` predicate. An `UNOWNED_DIRTY_PATH` that a proven entry
   covers narrows to the entry's tests. One that `Attributed` reports gets no line and keeps the
   AFP-V0-012 attribution. Any other one runs everything with `unowned-path`. The reason
   `non-code-unmapped` is retired.
2. `repositoryIndex.attributed` is the selector's own Go rules:
   - an enclosing package directory (rule (b): the enclosing packages, the nearest one's dependents
     and every embedding ancestor's dependents);
   - a package whose files name the path (rule (c));
   - a declared read scope that bears on the path (AFP-V0-023).

   The plan's declared-reader witnesses (`DECLARED_EXISTENCE_READER`, `DECLARED_READ_SCOPE`) on a
   selected Go unit also count. The plan that reports the path unowned is the same plan that names
   its declared reader, so trusting the witness adds no trust beyond the `UNOWNED_DIRTY_PATH` entry.
3. `UNOWNED_DIRTY_PATH` goes back on the AFP-V0-982 allowlist. The selector holds the Go index and
   decides attribution before the allowlist runs. `corvint test` (via the selector's FALLBACK
   verdict) and the CI selector therefore still run everything for an unattributed path.
   - The line-evidence plan (`affectedLineBounded`) reads only the allowlist, as it did before
     V1-5424.
   - The qualified selector's set-aside of Go-only unowned entries is removed. The allowlist admits
     those entries again, so nothing is set aside and no observation's uncertainty is dropped
     (review P3-2).
4. go:embed (review P2-1): the go:embed branch is removed, so declared documentation is reached
   first and adds no test. A non-Go file below an embedding package keeps the existing rule (b)
   attribution: the embedding package and its dependents. The index records that a package embeds,
   not which files, so this is a sound over-approximation. Recording exact embed patterns needs a
   gateselect scan-memo profile bump and is not done here.
5. Flow files (review P2-3): both `.corvint/flows.json` and the deprecated `.corvint/e2e-flows.json`
   are `flows-unreadable` when the plan carries no flow join.
6. An invalid map forces `FALLBACK data-map-invalid` on every plan. The full run is fail-closed by
   design (review P3-1, now stated in delivered rule 6).
7. `SelectQualified` returns a named NCM full run (`noncodemap.NamesFullRun`) before its generic
   input checks, so the reason is no longer replaced by `the selection input has no supported scope`.

## What stays a full run

The ruling covers Go attribution of non-code paths. It does not reopen the owner's list decisions,
so these keep their full-run assertions:

- `.corvint/` configuration (NCM-V1-006, G16 and R31): sealed CEMs (superseded on 2026-11-12 by
  `2026-11-12-ncm-sealed-cem-evidence.md`), `test-read-scopes.json` and
  `gated-root-locators.json`. This is why the AFP-V0-050 "unknown base" case in
  `tools/gate-affected-select` is not restored.
- The never-narrow list (NCM-V1-005): `.github/**`, including the Go packages under it.
- The unclaimed tracked addition `data/new.txt` in the AZU-V0-9900 dry run. It is genuinely
  unattributed and is now `unowned-path`.

## Tests restored to their pre-V1-5424 assertions

- `internal/sessiontest/plan_test.go`.
- `internal/liveverify/affected/swfix_test.go`.
- The fixture half of `TestUnindexedFixtureSession_AFPV09013`.
- `cmd/corvint`: `affected_existence_readers_unix_test.go`, `test_source_slices_tspv0006_unix_test.go`
  (the assembly file) and `test_typed_slices_unix_test.go` (embedded data and data beside a
  package).
- The CIN-V0-860 unowned rows, using an attributed `core/readme.xyz` because `Makefile` is now never
  narrowed. A new row asserts `unowned-path` for an unattributed path.
