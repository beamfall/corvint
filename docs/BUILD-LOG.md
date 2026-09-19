# Build log

Append-only record of material design decisions, independent findings, failed evaluations, and
promotion evidence, newest entry first. Each entry carries a date heading and the requirement or
decision IDs it concerns, so `rg -n '^## ' docs/BUILD-LOG.md` is the index.

The public tree starts this log at the 0.4.0a4 alpha. Entries written before publication are internal
working records and are referenced from decisions and specifications as historical context only.

## 2026-09-19 NTP-V0 native taskman fixture planning

Decision 0321 records the owner's accepted prerequisite amendment and pre-edit baseline freeze.
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
