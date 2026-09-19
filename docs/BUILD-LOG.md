# Build log

Append-only record of material design decisions, independent findings, failed evaluations, and
promotion evidence, newest entry first. Each entry carries a date heading and the requirement or
decision IDs it concerns, so `rg -n '^## ' docs/BUILD-LOG.md` is the index.

The public tree starts this log at the 0.4.0a4 alpha. Entries written before publication are internal
working records and are referenced from decisions and specifications as historical context only.

## 2026-09-19 DCP-V1: experimental revision-pinned documentation corpus

Issue 31 was explicitly scoped to all six phases. The change supplies a native immutable corpus,
separately attributed native read integrations and CEM citation sidecar, capability-gated stdio MCP,
Markdown/JSON rendering and explicit block maintenance, and an independent flow adapter outside Core.
Gate A required real native receipt decoding rather than trusting normalized verification labels,
separate source/provider commits, complete inventory before analyzer admission, and same-call
maintenance rederivation. Those boundaries govern the implementation. Generated intent remains
proposed; opaque Go run identity, E2E served-build identity, assertion adequacy and external utility
remain unknown. Passing synthetic journey fixtures do not claim real browser execution.

Independent review found cross-binary engine identity, scope additions, native range impact,
excluded-source handling, maintenance publication races and a CEM base-mode gap. The consolidated
repair uses shared compiler identity, native admission/receipt metadata and descriptor-confined
no-clobber publication with retained recovery inodes. Actual two-binary and concurrency regressions
cover the previously hidden boundaries.

Focused corpus, provider, maintenance, adapter, native/CEM and transport tests passed during development.
A new receipt-join regression exposed the host's `/var` versus `/private/var` root alias; reads now use
the caller's lexical receipt root and the same confined file reader. An E2E freshness review exposed
that source identity alone was insufficient; unknown served builds now remain unknown. These failed
iterations were repaired before final verification. Frozen synthetic labels report precision, recall,
abstention, false relationships, latency and bytes, without a savings or universal-quality claim.

The first full gate found help usage lines being interpreted as executable draft examples, an
unupdated analyzer-input fingerprint, and a README claim that differed from the spec digest. Usage
now includes the optional root prefix, the analyzer schema advances to 69 for the added immutable
reader inputs, and the README repeats the contract claim. These native gate failures are retained;
the source must be rebound and the full gate rerun before local completion.

Self-development routes used: actual pre-change query/context, dirty affected advice (UNKNOWN with
explicit frontiers), and the committed self-corpus fixture. Draft maintenance was exercised by the
new corpus path; existing draft host adoption and external provider/host interoperability remain
NOT_OBSERVED. CEM/OCM and clean selected-check receipts are retained by the enrolled local completion
session. Independent review and final gate results must be inspected before completion; the build
log records the contract, not an assertion that a pending gate already passed.

Integration with current main preserves the native work-init and fixture-planning additions.
Independent delta review identified three new native option values that the corpus wrapper must
leave untouched; an option-isolation regression covers that compatibility boundary. The merged
source requires fresh canonical verification and newly bound evidence against current main.

## 2026-09-19 NTP-V0 integration with repository work-queue adoption

The owner authorized merging the verified fixture extension. Current main `d9144000` also
implements WQO repository adoption; the combined command keeps native fixture dispatch separate
from adoption and shadow observation. Both build-log entries are preserved. The fixture decision
is renumbered from 0321 to 0322 because the adoption decision already owns 0321 on main; accepted
requirements and the original prerequisite approval are unchanged. Historical review receipts keep
their original identifiers. The completed `1f639857` gate and `66c33c02` seal remain prior evidence;
integration requires a new current-main CEM/OCM binding and checks on the combined clean target.
GP, live reservations, CONFIG_PIN, admission and production promotion remain held.

## 2026-09-19 NTP-V0 native taskman fixture planning

Decision 0322 records the owner's accepted prerequisite amendment and pre-edit baseline freeze.
The opt-in `work plan-fixture` reads a native fixture journal through fixed audit/status/export
commands, then emits a source-bound priority-first plan. WQO shadow selection is unchanged.
A built task-store fixture selected P0 touching A+B over the larger P1(A)/P2(B) wave; two complete
reads were byte-identical and all 57 source/store files stayed unchanged. Exact binary, snapshot,
source hashes and output are in `.agent-evidence/native-taskman/fixture-smoke-final.json` and its
adjacent receipt/source manifest. Fixture reservations/history remain caller-owned observations.

Independent review reproduced whole-repository exclusion escaping empty resource sets, native
Code enum drift and malformed observation/history acceptance. The repairs check whole scope before
pairwise resources, preserve the closed native Code enum (DEVELOPMENT_MODE for fixture SELECTED),
and reject contradictory identity/revision/resource/history input without releasing reservations.
`internal/taskman/review_test.go` retains the failure regressions; focused planner, adapter, WQO
boundary and cancellation tests passed. The exact committed native gate and CEM/OCM/local completion
reports are post-commit evidence; these focused results alone do not close that gate.
The first full gate at `865e554b4e5a601282423dbdbbf6264dfdda1b6e` failed only
`internal/specindex`: header/digest/README wording did not exactly match the registry. The
metadata was aligned and the focused registry check rerun before rebinding; the failed gate is
retained in the enrolled check history, never represented as a passing full gate.

Self-development routes used: original prechange query (including omissions), native fixture
planning, `affected` (two Go units advised; full gate mandatory), tracked-path `prove` (97 omissions),
and the enrolled keyed dogfood workflow. New untracked-path `prove` refused; no substitute success
is claimed for it. Snapshot indexing was used by fixture planning. Batch/mutation/learning/service,
foreign adapters and live-provider qualification were not applicable to this bounded fixture slice.
Exact ATCP history remains unrecovered. GP is NOT_RUN: retired harness replacement is diagnostic
only, complete workloads/allocation/I/O/environment witnesses and real runtime conditions are absent.
No executor admission, CONFIG_PIN, production completion, real-queue cutover or performance promotion
is claimed. The explicit executor dependency handoff remains open.

## 2026-09-19 WQO-V0-046..048 repository work-queue adoption (decision 0321, Beamfall/corvint#20)

Before this change, `corvint work observe` could never return `VALIDATED_AT`. `workManifest.complete`
was never set, so WQO-V0-017 always added `SOURCE_UNQUALIFIED` and `propose-wave` always abstained.
`corvint work init` and `corvint work adapter` now give any repository a committed policy,
worklist, and adapter. The observer qualifies store scope only when it reproduces the
`repository-worklist-v0` documents byte for byte.

A first cut qualified both closed mappings. It turned Corvint's own `decision-0046-v0` observations
`VALIDATED_AT` and broke the WQO-V0-021/025/032 final-check witnesses in `cmd/corvint`, which rely
on an incomplete initial capture ("initial capture did not retain incomplete scope"; three
`WQO-V0-032` codes became `MALFORMED_INPUT`). Restricting qualification to the adoption mapping kept
those witnesses and the self-dogfood contract unchanged.

Pre-landing review reproduced a symlink escape and an interrupted-write residue in `work init`.
The repair roots every write with `os.Root`, rejects a symlinked `.corvint`, delays success output,
and rolls back files created by a failed or racing initialization.

A second independent review found that init still accepted a plain directory or repository
subdirectory, a partial committed adoption without the worklist surfaced `ADAPTER_FAILED` instead
of `SOURCE_UNQUALIFIED`, and the generated adapter unnecessarily required Bash. The repair refuses
non-root targets before writing, preflights the committed adoption worklist with the other source
inputs, and emits the POSIX-only adapter with `/bin/sh`.

The first canonical-gate attempt after that repair intentionally did not qualify: the release
artifact conformance test refused the staged repair as a dirty worktree. The repair was committed
unchanged before rerunning the gate from clean, frozen source.

After the final evidence seal, current `main` advanced with the accepted corvid artwork. Merging it
correctly invalidated the pinned README workflow citation because the themed logo moved that span by
one line. The canonical gate refused the stale pin; the repair relocates it to
`README.md:188-198@3297e31e` without changing the cited workflow.

`TestWorkAdoptedRepositoryWorklist` starts from a clean fixture and runs init, a refused second
init, commit, observe, and propose-wave over four verification tickets. One is a suite batch, one a
failure-classification repair, one a test-validity receipt, and one a cleanup/retry that shares
`internal/parser`. The result is `VALIDATED_AT/UNCHANGED_OBSERVED`, `ELIGIBLE_AT` with three
tickets selected, the cleanup/retry ticket `EXCLUDED` with its collision group, and a byte-identical
repository manifest. A scratch-repository probe of the built binary recorded the unknowns:
`ERROR/SOURCE_UNQUALIFIED` with no policy, with an uncommitted adoption, and with a tracked
modification, and `ERROR/ADAPTER_FAILED` when `corvint` is absent from the fixed `PATH`.

## 2026-09-19 CRB-V0-014: owner-selected corvid artwork

The owner selected the first, corvid direction from three generated concepts and then approved
adoption ("look good. use it"). The chosen silhouette
was redrawn as editable SVG with a custom lowercase wordmark; the light, dark and universal mark
and lockup variants share geometry. The README selects a theme-appropriate lockup; the 24 px editor
icon inherits its host color. `assets/brand/README.md` records usage, raster dimensions and rollback.

Manual asset checks passed: SVG parsing, accessible titles/descriptions, no font or external-image
dependencies, PNG dimensions/alpha, shared variant geometry and README references. Independent
Codex review passed with no material findings, including comparison to the selected concept,
24 px legibility and exact SVG-to-PNG rendering parity. Requirement-index regeneration and focused
spec-requirement, requirement-definition and decision-number checks passed using an isolated index
containing the updated spec; `git diff --check` passed. Go/runtime code is unchanged; the full Go
and release gates were not run for this manual artwork slice, and installed editor qualification
remains unclaimed.

Pre-change Corvint query ran against `6a423ac091d848b8ac5b49e8002c61c252993ac3`, with three ranked
results omitted and two test-path candidates withheld. Dirty-Go advice, mutation and retrieval
evaluations are not applicable. The initial `make dogfood-change` returned `not-complete`
(`cem-prepare: git-diff-failed`, missing intent scope and outcome input); it is not passing evidence.
Post-commit CEM/OCM qualification is NOT_PRODUCED for this manual asset slice; native
CEM's PNG binary-patch limitation remains explicit. Original query, generation prompts and manual
asset hashes are retained in `/tmp/corvint-logo-20260919/` for this task.
## 2026-09-18 EEP-V0 provider-to-impact workflow: synthetic fixture evaluation, and unsupported cases

`TestImpactProviderEvaluation` (`internal/extevidence/extevidence_test.go`) runs the
provider-to-`impact` composition path (`Section`) over the 5-relation mock fixture
`internal/extevidence/testdata/mock-provider.json`: 3 relations that must be admitted (one
`declared` `implements`, one `inferred` `mockdocs:enables`, one `observed` `verifies`) and 2 that
must be excluded (one `learned` relation, `EEP-V0-007`; one relation from a foreign provider
endpoint, `EEP-V0-006`). Results on commit `4a2c00f` (`Russells-Mac-Studio.local`, `go1.27.1`):
precision 1.000 (3 of 3 admitted rows expected), recall 1.000 (3 of 3 expected relations admitted),
zero false-positive relationships, zero `learned`-evidence admission, abstention accuracy 2 of 2
(the learned relation reports `excluded` and the foreign-provider relation reports `unresolved`,
both under `unknowns`, neither admitted), latency about 0.68 ms, and a 3022-byte `context.external`
section. `TestSelectionEvaluation` was rerun the same day over its existing corpora (63 cases:
see the two entries below) with the same results already on record. The fixture used here is one
record with five relations, not an independent corpus; the entries below already exercise a larger,
independent labelled set for the fail-closed selection profile. This shows the exclusion and
authority-assignment rules hold on the documented worked example; it is not an adopter outcome.

Unsupported in this slice, per `external-evidence-provider-v0.md` and `external-test-selection-v0.md`:
command, MCP, and remote provider transports (file transport only); multi-hop obligations beyond one
relation hop; and checkout worktree inspection for a V1 checkout binding (a checkout's canonical path
is echoed, never opened). None of these are measured above; none are estimated. The Change Frontier
sidecar for external obligations landed separately (EFO-V0, entry below) and is not measured here.

## 2026-09-18 EFO-V0 external obligations sidecar: reference-only join to the frontier

Decision 0313. `corvint obligations --cem FILE --impact FILE` writes `external-frontier-obligations/0`
(`docs/specs/external-frontier-obligations-v0.md`). The CEM and frontier wires are unchanged; the
sidecar cites the frontier through `binding.cem_sha256` (the `CF-V0-019` raw-copy digest) and CEM hunk
ids, and nothing reads it. Tested by `TestObligations*` in `internal/extevidence` and `cmd/corvint`;
no adopter receipt or labelled review sample exists yet, so promotion stays open.

## 2026-09-18 EEP-V2 path-to-path relations: synthetic conformance evaluation

Decision 0312. `TestSelectionEvaluation` now runs over both labelled corpora: the 23 ETS-V0 cases,
plus 40 cases in the independent two-repository fixture
`internal/extevidence/testdata/conformance-path/cases.json`, 63 in all. Ten of the path cases
cover directory scopes (`EEP-V2-012`, `EEP-V2-013`): a held path, a changed test inside the scope,
a scope that widens, and missing, dirty, other-repository, and slash-less cases that must not
narrow. Results: precision 1.000 (36 of 36 selected tests expected), unsafe narrowing 0 of 49 cases
that must not narrow, abstention accuracy 6 of 6, latency p50 about 98 ms and max about 181 ms per
case on one loaded development host, and a largest `test_selection` member of 5393 bytes. The V1 and no-provider CLI
tests keep their bytes. A V1 record carrying the same path relation stays an `unsupported`
unknown, which is why `TestAffectedSelectionPathRelation` gives `full` under V1 and `narrow` under
V2. The corpus is synthetic and was authored with the feature. It shows that the per-side and
worst-side rules hold. It is not an adopter outcome.

## 2026-09-18 ETS-V0 external test selection: synthetic conformance evaluation

Decision 0311. `TestSelectionEvaluation` over the 23 labelled cases in
`internal/extevidence/testdata/conformance-selection/cases.json`: precision 1.000 (16 of 16 selected
tests expected), unsafe-narrowing 0 of 20 cases that must not narrow, abstention accuracy 3 of 3,
latency p50 about 35 ms and max about 125 ms per case on one development host, and a largest
`test_selection` member of 4470 bytes. The corpus is synthetic and authored with the feature, so
these numbers show the fail-closed rules hold. They are not an adopter outcome, and promotion needs
a corpus drawn from a real change history.
