# Tree read scope predicates (V1-5174)

Owner request: narrow guard selection without hiding verdict-changing edits. Requirements
AFP-V0-3300 through AFP-V0-3303 govern this slice. Delivery is one local commit with the
requested focused checks; no Tasks mutation or publication.

## Design

Existing `corvint-test-read-scopes/0` string entries keep their meaning. Additive entries are
`{"tree":"cmd/","contentPredicate":"REGEX","samples":["POSITIVE LINE"]}` and
`{"tree":"cmd/","listingOnly":true}`. Entries remain sorted/unique by path. The Go adapter
retains the filesystem bound separately from selection metadata; both enter the graph digest
and cache. `corvint-affected-graph-cache/1` rebuilds stale caches. Confinement and the independent
CI selector accept the objects as full access trees, without claiming predicate narrowing.
The declaration compatibility check's closed top-level member set is unchanged.

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

Baseline: `ebeb5a643539faca17e390ad3fe99370dd6f8872`. Append only
`// V1-5174 behavior-neutral planner measurement.` to `internal/tasks/journal/anchor.go`, then
revert. The baseline used the source-built CLI with `affected --base FULL_SHA --format json`.
This revision rejects `--json`; `--format json` is its supported equivalent.

To exclude implementation edits from the experiment, the after run uses the candidate CLI and
candidate declarations over the same source inventory in an isolated checkout. Its clean fixture
commit is `89c32e679d1d3d35112fb72f75113e550c24b96d`; only the same comment is dirty. Command:
`go run ./cmd/corvint --root /tmp/v1-5174/qualified-measurement affected --format json`.
Worktree-only mode uses implicit HEAD, equivalent here to the baseline's empty committed range.
A preliminary shared clone correctly retained all guards because verified Git reads reject object
alternates. The self-contained shallow fixture admits immutable reads but refuses explicit
`--base`; implicit HEAD supplies the equivalent measured endpoint. Preliminary refusals are not
counted as successful narrowing. Original and scratch comment edits are restored afterward.

| Metric | Before | After |
|---|---:|---:|
| Whole-tree reasons in `.corvint/test-read-scope-trees.json` | 37 | 37 |
| Plain whole top-level tree entries | 37 | 23 |
| Predicate entries | 0 | 14 |
| Listing-only entries | 0 | 0 |
| Packages with plain whole top-level trees | 15 | 13 |
| Selected units | 280 | 279 |
| Excluded units | 609 | 610 |
| `LANGUAGE_FRONTIER` unknowns | 67 | 67 |

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
`/tmp/v1-5174/`.

## Verification and review

- PASS: `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'AFP_V0_33|AFP_V0_315|AFP_V0_301' ./internal/liveverify/affected/` on Darwin. Covers endpoints, binary/unreadable/missing/bad-regex fallbacks, listing transitions, cache round-trip, declaration identities and overlapping witnesses.
- PASS: the same focused command for `./tools/test-read-scopes-check/`, on wrapper-selected WSL/Linux. Covers samples, reasons, empty matches and existing AFP-V0-3010/3150/3151 cases.
- PASS: `make test-read-scopes-check`.
- PASS: `GOTOOLCHAIN=local go vet ./internal/liveverify/affected/... ./tools/test-read-scopes-check/ ./tools/declaration-compat-check/`.
- PASS: `make spec-requirements` after staging the spec as required; only reserved AFP-V0-3300 through AFP-V0-3303 added to `REQUIREMENTS.tsv`.
- PASS: `gofmt -l` on changed Go files, no output.

Independent native reviewer found no remaining blocking issues after repairs to the unsafe CLI
listing conversion, overlapping witness attribution, deterministic observation order and the
fault-injection fixture. The permission fixture initially lacked `OpenBounded`; its reach assertion
caught this before coverage was claimed. The overlap fixture initially exercised an earlier
literal-reader witness; it now directly exercises `DECLARED_READ_SCOPE`. A temporary Git-reader
import into the affected library caused two consumer-closure checker findings; moving admission
to the CLI restored the original closure instead of changing unrelated declarations.

Corvint context was consulted first, but its snapshot revision was stale; original source reads,
the source-built planner and independent review supplied the task evidence. Installed Core:
`1.0.0-rc.3.dev`, build 133. Official update check: freshness UNKNOWN after its version smoke could not prove process-group cleanup in the sandbox; no upgrade claimed.
Usage/billed telemetry: NOT_OBSERVED. Tasks, publication, exhaustive/Windows gates and multi-commit
CEM/seal closeout are NOT_RUN under the owner's focused-command, single-local-commit scope.
Rollback restores the object entries to their original string trees; all reasons remain available.
