# Build log

Append-only record of material design decisions, independent findings, failed evaluations, and
promotion evidence, newest entry first. Each entry carries a date heading and the requirement or
decision IDs it concerns, so `rg -n '^## ' docs/BUILD-LOG.md` is the index.

The public tree starts this log at the 0.4.0a4 alpha. Entries written before publication are internal
working records and are referenced from decisions and specifications as historical context only.

## 2026-09-21 GL-V0-001..GL-V0-008: gate ledger, one pass per distinct content

The owner asked that Corvint manage its own gate so parallel agents stop each running a full
`make gate` on content another worktree already proved. `docs/specs/gate-ledger-v0.md` is accepted
and implemented: `tools/gate-ledger` keys every step on a digest of its declared inputs over the
exact worktree (tracked and untracked, ignored excluded, written through a private Git index) plus
the gate's tooling, records only passes in a per-user `0700` directory, and skips a step only on
key equality. `go-archive-gate` always runs because GOC-V0-010 binds its witness to HEAD, and every
doubt (undeclared step, skip-worktree or assume-unchanged entry, ignored compiled `.go`, git
failure, shared ledger directory) runs the step and records nothing.

Independent finding that changed the design: Go's test cache never hits across worktrees, even
with `-trimpath`, because its test log hashes the absolute paths of files a test opens under the
module root (measured on this host with a second `git worktree` at the same commit). The proposal
had assumed the cache would deduplicate resolved packages across worktrees; it deduplicates only
same-worktree reruns. So `ledger/go-test` (GL-V0-004) runs the 93 packages the affected-plan index
resolves without `-count=1`, under Go's cache, and the 104 unresolved packages (`cmd/corvint`
among them, for one `os.Getwd`) with `-count=1` under one record keyed on the whole tree. A tree
change therefore still reruns the unresolved set; narrowing it is the affected tier's job
(`affected-plan-v0.md`), and a per-package cross-worktree key is recorded as a follow-up in
`agent-memory/ideas.md`. `make go-test`, `make gate-affected` and CI keep `-count=1` unchanged.

Measured on this host (Mac Studio, `-p 1`), from the baseline `go test -json ./...` at the base
commit: 197 packages, 2,167s of package time, of which the 104 unresolved packages take 1,634s and
the 93 resolved ones 533s; `cmd/corvint` alone is 174s. The partition on this tree lists 93
resolved and 105 unresolved packages (the module root counts once more than the baseline's
package list). Three cheap steps run through `ledger/` twice: the first pass ran and recorded all
three in 3.7s, the second hit all three in 2.0s, so the per-step ledger cost (worktree digest plus
`go run` start-up) is about 0.65s. `plan` prints `go-archive-gate: always runs` and `RUN` with
`no declared input scope` for an unknown step. A `go-test` run whose unresolved set failed (the
host-adapter test reading a pre-existing dirty `plugin.json`) recorded nothing, as GL-V0-002
requires. Full gate, measured twice in a clean `git worktree` at `d6626ae` with an empty ledger
directory: the first `make gate` ran and recorded all 28 keyed steps in 1492s and recorded the
receipt; the second hit all 28 (the full `ledger/go-test` among them), ran only `go-archive-gate`,
and recorded the receipt in 173s. The first attempt at `22208f6` found two defects the unit tests
had not: a linked worktree's index path is absolute, so the private-index copy was empty and no
step recorded (fixed, `TestRunStepRecordsFromLinkedWorktree`), and the Windows cross-vet rejected
`syscall.Stat_t` and `syscall.Flock` (fixed by build-tagged `platform_unix.go`/`platform_other.go`;
a non-Unix host refuses the ledger directory and records nothing).

## 2026-09-21 SEG-018..SEG-021: typed semantic choice decisions

The owner directed Corvint to adopt the useful typed-decision ideas from TypeSafe AI's System One
model announcement without adding Jev or another hosted dependency. The unwired
`internal/semescalate` experiment now has a separate provider-neutral choice path over mechanically
supplied anchored options. Providers return only an option ID and exact integer probability mass;
Core derives an explicitly uncalibrated winner margin, supports a reserved abstain option, rebuilds
the immutable proposal, and still requires the registered verifier before emitting an `INFERRED`
candidate. The legacy proposal request and schema are unchanged. No provider, network path, serving
integration, calibration corpus, authority, or product claim is added.

Focused provider-spy tests cover the successful end-to-end path, pre-call question refusal,
case-folded/duplicate/missing/fabricated distributions, non-unique maxima, low-confidence and
explicit abstention, schema separation, complete cache identity, and the unchanged authority ceiling.
Independent review identified shared-state races, mutable evidence aliases, and incomplete screening
of transmitted handles. The repaired gate serializes run reservations and ledger reuse, snapshots
selected evidence, and screens every transmitted caller-authored string; focused race tests cover
concurrent budget/cache behavior and mutation during provider latency.
The canonical gate then exposed one malformed wrapped Agent-digest bullet, which was repaired and
independently re-reviewed. Two subsequent exact-target gate runs failed only because the large
`contextindex` fixture used the benchmark Git helper, allowing detached auto-maintenance to recreate
`.git/info` during `t.TempDir` cleanup. A 20-run loop reproduced 16 failures; applying the existing
`testGit` synchronous-maintenance policy to benchmark fixtures made all 20 pass without changing
runtime behavior.
The frozen calibration, held-out replay, kill-gate, and first accepted extractor profile remain open;
this deterministic slice is not evidence that any model's probabilities are calibrated.

## 2026-09-20 LAC-V0-032: safe roadmap auto-recheck

The roadmap repeats its existing read-only request every 30 seconds. Eligibility remains derived by
Corvint Tasks: no mutation, approval, external/manual completion, unknown-evidence waiver,
admission, release candidacy, attestation, or promotion is added. Pages containing mutation forms
do not refresh automatically. A page-preserving pause/resume link prevents timed reloads from
interrupting deliberate inspection. `TestRoadmapSafeAutoRecheck` binds the refresh and hard-stop
notice on successful and refused reads and checks that paused roadmaps and the board remain stable.
The first full gate reached every package but failed three `internal/contextindex` tests during
`t.TempDir` cleanup: detached Git maintenance recreated `.git/objects/info/packs` and
`.git/info/refs` after removal began. The shared fixture Git helper now disables auto-gc and keeps
any maintenance synchronous; the exact combined reproducer and the full gate must pass after this
repair before the console change is qualified.

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
