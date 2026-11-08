# Tree read scope predicates (V1-5174)

Owner request: narrow guard selection without hiding verdict-changing edits. Requirements
AFP-V0-3300 through AFP-V0-3303 govern this slice. Delivery is one local commit with the
requested focused checks; no Tasks mutation or publication.

## Design

Fix round 1 corrects a confirmed P1 in `934f91e20`: putting object entries in
`.corvint/test-read-scopes.json` broke its many string-only readers (including test reuse,
gate-ledger and repository tests). Every entry is again a plain string. Each converted guard
keeps its whole-tree string as the conservative superset. The only net addition to that file
against `ebeb5a643` is the plain `internal/dogfoodflow/` overlap for `tools/flow-guide-check`.
The changes to `.github/testconfine/scopes.go`, `internal/gateselect/readscopes.go` and
`internal/liveverify/affected/golang/readscopes.go` are reverted exactly to that base; there
is no net `.github/` change or change to any reader of the string-only declaration.

Narrowing now lives solely in `.corvint/test-read-scope-trees.json`. Each package/tree value
can remain a reason string or become `{"reason":"GUARD READ","contentPredicate":"REGEX",
"samples":["POSITIVE LINE"]}` or `{"reason":"GUARD READ","listingOnly":true}`. The two existing
checker consumers accept these alternatives without a profile bump; top-level shape stays
`corvint-test-read-scope-trees/0`. The compatibility checker admits the known metadata members
and refuses unknown object members. Old binaries and all other readers still see the same plain
whole-tree strings, so they keep conservative selection and filesystem access.

The affected graph loads metadata separately from the unchanged Go declaration reader. Missing,
unreadable, oversized or malformed trees files, invalid predicates/samples, and absent objects
leave the plain whole-tree scopes intact. Graph units and digest retain admitted rules.
`corvint-affected-graph-cache/2` binds the trees file's exact SHA-256 content and mandatory read
stamp (including absence); metadata edits, deletion and reappearance invalidate cached graphs.
Edits to the string declaration keep the existing package comparison; edits to the trees file
select declared readers even when its metadata cannot be admitted.
Content predicates inspect bounded immutable BASE and planned HEAD/worktree bytes. Either match
selects. Missing/invalid predicates, empty matches, missing/uncovered samples, binary text,
unreadable/oversized/missing endpoints, sparse checkouts and all uncertainty retain selection.
Add/delete/rename endpoints without both readable sides therefore select. Listing scopes compare
direct-child names/types, detecting a new directory even when Git names only its first file.
Unknown listings/types and tree/ancestor records select; existing-child content changes and
nested changes leaving direct children identical do not. Own-package edits, ordinary dependencies
and overlapping plain entries retain their existing authority.

Observation uses stable graph/path order. Revision data never survives in the reusable graph
cache. Git admission stays in `cmd/corvint/affected_readscopes.go`, preserving the graph library's
dependency closure. Literal/glob readers respect the same predicate. Declared-scope witnesses name
the actual bearing entry, including a plain subtree overlapping a nonmatching predicate tree.
The checker retains whole-tree reasons and refuses missing/bad regexes, empty matches, missing
samples, uncovered samples and mixed predicate/listing forms, naming package and entry.
Samples are regression checks, not proof of an arbitrary semantic superset.

## Source audit and conversions

| Guard | Evidence and disposition |
|---|---|
| `internal/obscorpus` | `obscorpus_test.go`, `TestPackageIsPureAndUnwired`: six production-Go trees use `bytes.Contains` for the quoted `github.com/Beamfall/corvint/internal/obscorpus` import. Six predicate entries cover that exact path string; own-package checks stay unconditional. |
| `tools/fixture-maintenance-check` | `guard_test.go`, `TestRepositoryGitFixturesDisableDetachedMaintenance`: files without raw substring `git` are ignored before lexing/accumulating facts. Five entries use `git`, also preserving scanner failures and `github.com` matches. |
| `tools/flow-guide-check` | `main.go`, `sourceText`, `checkRefusals`, `unclassifiedCounts`: three entries cover the unanchored kebab/upper code grammar and `refuse`/`Refuse` names. `dogfoodSites` parses every package file, so `internal/dogfoodflow/` is a plain overlapping subtree. `script/` remains plain for its listing/content/existence reads. |
| `internal/diagnostic` | Retained both trees: `gate_test.go`'s `scanFile` fails on any Go parse error before checking diagnostic imports. An import predicate would hide failures. |
| `tools/capabilities-check` | Retained all four trees: `Collect`, `parseSubcommands`, `ScanLiterals`, `treeDrift`/`clitree.ScanDirs` consume arbitrary literals, tables and dispatch source, including parse-error verdicts. Other reads include link existence and integration skills. No narrow marker is a proven superset. |
| `internal/analyzercap` | Retained `cmd/`: `source_ceiling_test.go`, `TestCandidateSourceCeilingPerExecutable`, lists commands then calls `reachableProductionSource` to measure their source contents. |
| `internal/langplugin` | Retained `cmd/` and `internal/`: besides listing, `TestCoreNameLiterals_LGPV0009` scans `cmd/corvint`, and `TestPluginIsolation_LGPV0009` follows imports from plugin directories including command packages. |
| `tools/cli-discovery-gen` | Retained `cmd/`: `install_test.go` only lists it, but `help_test.go`, `TestCAPV07700RootInputs`, calls `rootTree`, which parses actual command source in `main.go`. Independent review caught this additional read. |

All three proposed listing candidates read contents too; none is converted. The listing-only
kind is implemented/tested without weakening these guards. `cmd/corvint-test`,
`conformance/cli-errors-v0`, `internal/companionrelease`, `tools/corvint-pr-tests` (V1-5028)
and `internal/proctrace` remain unchanged as requested.

## Before/after measurement

Fix-round measurement repeats the original behavior-neutral edit:
append `// V1-5174 behavior-neutral planner measurement.` to
`internal/tasks/journal/anchor.go`, then restore the original bytes in a `finally` block.
Both runs use the same branch-built binary and source inventory in the self-contained shallow
fixture `/tmp/v1-5174-fix1/measurement`. Before uses the two declarations from
`ebeb5a643539faca17e390ad3fe99370dd6f8872`; after uses this repair's declarations. Clean fixture
commits are `5f41844610b6f7db563f87ba02d57b1d9bcc78f1` and
`0d9f11c07f7f092f279c013a12c7a5bc978f09b3` respectively. Only the comment is dirty during each run;
the fixture and task worktree have no remaining anchor edit.

Build: `script/heavy.sh --admit-only env GOTOOLCHAIN=local go build -o /tmp/v1-5174-fix1/corvint ./cmd/corvint`.
Binary SHA-256: `4808e32e90393ba082b90372296cb87212f38def250a9bad18cfa67cfe007fc9`.
Command for each run:
`/tmp/v1-5174-fix1/corvint --root /tmp/v1-5174-fix1/measurement affected --format json`.
Implicit HEAD supplies the measured endpoint without an explicit range over shallow history.
Both runs retain `self-binary-unverified` because the source build's VCS commit is absent from
the isolated fixture; this is a provenance limitation, not a selection failure. The binary was
built from the repaired worktree immediately before measurement. Original round artifacts remain
in `/tmp/v1-5174/`; fix-round raw plans, stderr, argv and summary are in `/tmp/v1-5174-fix1/`.

| Metric | Before | After fix round 1 |
|---|---:|---:|
| Whole-tree reasons in the trees file | 37 | 37 |
| Plain whole top-level strings in the scopes file | 37 | 37 |
| Predicate objects in the trees file | 0 | 14 |
| Listing-only objects | 0 | 0 |
| Packages with plain whole top-level strings | 15 | 15 |
| Selected units | 280 | 279 |
| Excluded units | 609 | 610 |
| `LANGUAGE_FRONTIER` unknowns | 67 | 67 |
| One observed invocation, seconds | 4.608 | 3.675 |

The elapsed times are individual observations, not a performance benchmark.

| Converted guard | Before | After |
|---|---|---|
| `internal/obscorpus` | selected (`PATH_LITERAL_READER`) | excluded |
| `tools/fixture-maintenance-check` | selected (`PATH_LITERAL_READER`) | selected (`PATH_LITERAL_READER`) |
| `tools/flow-guide-check` | selected (`PATH_LITERAL_READER`) | selected (`PATH_LITERAL_READER`) |

The latter two retain selection because `anchor.go` contains `github.com` and
`taskman-writer-checkpoint/1`, respectively. This is a measured one-unit reduction, not a claim
that all comment edits or all converted guards are excluded. Retained `diagnostic`, `langplugin`
and `capabilities-check` stay selected; `analyzercap` and `cli-discovery-gen` remain unselected.
Both plans retain UNKNOWN scope and all 67 frontier observations. Raw receipts/command logs:
`/tmp/v1-5174-fix1/`.

## Verification and review

Fix round 1, on Darwin with Go 1.27.3:

- PASS: `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'AFP_V0_33|AFP_V0_315|AFP_V0_301' ./internal/liveverify/affected/`.
- PASS: `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'AFP_V0_33|AFP_V0_315|AFP_V0_301|Tasks' ./tools/test-read-scopes-check/`.
- PASS: `HEAVY_WHOLE_REASON='small tool package' script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/declaration-compat-check/`.
- PASS: `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'ReadScope|Declaration' ./internal/liveverify/affected/`.
- PASS: `make test-read-scopes-check`.
- PASS: `make changed-checks BASE=ebeb5a643539faca17e390ad3fe99370dd6f8872` (line citations, unbounded-reader check and read-scope check). The unbounded-reader checker reports the advisory that `internal/tasks/cli` is recorded but not currently unbounded; this repair leaves that separate ratchet entry unchanged. The line-citation checker retains its 180 skipped external-basename citations.
- PASS: `GOTOOLCHAIN=local go vet` on `cmd/corvint`, `internal/liveverify/affected`, its `golang` adapter, `internal/gateselect`, `.github/testconfine`, and both changed tools, through `script/heavy.sh --admit-only` for the writable Go cache. The first direct attempt was blocked by sandbox permissions on the default Go cache; no source error was reported.
- PASS: `make spec-requirements`, after staging the spec; AFP-V0-3300 through AFP-V0-3303 retain their IDs.
- PASS: `gofmt -l` on every changed Go file, no output; `git diff --check`.
- PASS: `git diff ebeb5a643 -- .corvint/test-read-scopes.json` shows only the added plain overlap string. JSON validation confirms all 3,156 entries are strings and all 14 metadata objects retain their whole-tree scope string. `.github/` and the restored readers have no net diff against the base.

Coverage includes both content endpoints, binary/missing/unreadable/invalid-regex failures,
missing/unreadable/malformed metadata and absent package/tree objects, listing transitions,
cache round-trip and metadata-only invalidation, declaration edits and overlapping witnesses.
The first checker run caught a diagnostic-wording regression in existing AFP-V0-3010 tests;
restoring the `want exactly` prefix fixed it, and the requested checker command then passed.
No tests or acceptance assertions were weakened.

An independent native read-only reviewer found no blocking issues in the repaired code, metadata,
cache bindings or regression coverage. The earlier round's review missed the shared-format P1;
this repair restores the format instead of attempting to update its many independent readers.

Corvint context and `affected --base ebeb5a643539faca17e390ad3fe99370dd6f8872 --format json`
were collected before focused tests. The context snapshot was stale and the installed affected
binary reported `self-binary-stale`; source inspection, the branch-built measurement and the
independent review supply the repair evidence. Installed Core is `1.0.0-rc.3.dev`, build 133.
Official Core update freshness remains UNKNOWN: its version smoke could not prove process-group
cleanup in the sandbox. No tool upgrade is claimed. Usage/billed telemetry: NOT_OBSERVED.
Tasks, publication, exhaustive/Windows gates and multi-commit CEM/seal closeout are NOT_RUN under
the owner's single-local-commit repair scope. Dogfood status also refused ambiguous Claude/Codex
session identities without an enrollment key. No lifecycle completion is claimed.
Rollback replaces metadata objects in the trees file with their reason strings; the main
read-scope declaration stays string-only and conservative.
