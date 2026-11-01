# CEM 2.0 Discovery Replay V0

Owner: Russell Lewis
Date: 2026-11-01
Requirement prefix: `VRP-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: ticket V1-2600 (CEM 2.0 Phase 2 kill-or-continue value replay) and V1-2602
(typed per-unit label vocabulary); `benchmarks/README.md` (partitions and the first-observation
rule); `AGENTS.md` invariants 2, 4 and 8; the existing machinery this replay reuses unchanged:
`cem cover`, `cem verify --hunk-revert`, `cem discriminate` and the Go mutator behind
`prove --mutate` (`internal/liveverify/mutate`). `docs/specs/cem-2.0-intent.md` is cited, not edited.

## Agent digest
- Claim: A pre-registered replay of 40 later-fixed agent changes decides if typed per-unit labels beat coverage-diff at flagging the unit the fix touched.
- Status: proposed/experimental; **pre-registration only, not run**. The owner set X = 0.15 and Y = 0.60 on 2026-11-01.
- Exists: `benchmarks/cem2-discovery-replay-v0/` — `PROTOCOL.md`, `select.py`, `selection-manifest.json` (+ `.sha256`, `5a51e33a...29cda`), `thresholds.json`, `harness.py` (`check`, `plan`, `run`, `score`, `selftest`).
- Blocked on: the owner setting X and Y; an external-repository clone (beamfall/core selection is NOT_PRODUCED); a host with capacity for about 53 h worst-case wall-clock.
- Read next: `benchmarks/cem2-discovery-replay-v0/PROTOCOL.md` sections 2, 4 and 6; Requirements below.

## Intent and scope

CEM 2.0 proposes typed per-unit discrimination labels. Before any more of it is built, the owner
wants a kill gate: on real history, do those labels find the units that later turned out wrong
more often than plain coverage-diff, at an acceptable false-alarm rate? This slice freezes the
question, population, measurement, decision rule and runbook so the answer cannot be shaped by the
result. It ships no product behaviour and writes no Go.

## Requirements

### Pre-registration
- `VRP-V0-001`: The protocol, selection manifest, its sha256 and the threshold file live under `benchmarks/cem2-discovery-replay-v0/` and are committed before any replay output exists; the partition is held-out under the first-observation rule.
- `VRP-V0-002`: Thresholds X and Y are recorded in `thresholds.json` and the protocol table; until the owner commits numeric values both are the literal `OWNER-BLOCKER` and the proposed defaults are labelled non-binding.
- `VRP-V0-003`: `harness.py run` and `score` refuse (exit 4) when the protocol, threshold file, manifest or digest is untracked or differs from HEAD, and refuse (exit 3) while either threshold is not numeric.
- `VRP-V0-004`: `harness.py check` verifies the manifest sha256 (exit 2 on mismatch) and reports the threshold status.
- `VRP-V0-005`: `run` keeps the first observation: an existing per-change output directory is skipped, never overwritten; a rerun is permitted only under `VRP-V0-052`.

### Selection
- `VRP-V0-010`: The change set is produced by `select.py` from Git history alone (log, diff, blame), with the rule text frozen verbatim in the manifest's `selectionRule`; the same repository at the same ref yields byte-identical output.
- `VRP-V0-011`: A change qualifies only when a later `fix(...)` commit sharing a ticket or requirement ID blames at least one of its touched non-test Go code lines to it, comment-only and ID-renumbering hunks excluded, and it has 1..60 non-test Go source hunks.
- `VRP-V0-012`: The set holds 30..50 changes (40 when more than 50 qualify, by deterministic index sampling); each row records pre-fix change, base, fix, shared IDs, touched lines in the change's numbering, later fixes and the landing merges.
- `VRP-V0-013`: Each row records an escape stratum (`post-merge` or `same-merge`) from first-parent landing; strata are reported, not pooled silently.
- `VRP-V0-014`: At least one external repository is selected by the same rule into its own digested manifest before any run; while no clone is available its selection is recorded `NOT_PRODUCED`.

### Measurement
- `VRP-V0-020`: The unit is one non-test Go source hunk of the change's canonical CEM patch (`cem prepare`); touched units are those whose new range holds a manifest touched line.
- `VRP-V0-021`: The oracle test set is every `_test.go` file at the change in a changed source file's directory; one coverage run (`go test -count=1 -coverprofile -coverpkg`) at the change feeds both arms.
- `VRP-V0-022`: Units cite base-existing oracle files as whole-file `test-claim`s; units without one, and non-unit hunks, are marked `unknown`; tests added by the change enter only as `--same-change-test` and are counted separately.
- `VRP-V0-023`: One Corvint build runs every change; its version and the Go version are recorded per change.
- `VRP-V0-024`: Hunk-revert runs at its shipped limits (300 s per run, 1200 s total).
- `VRP-V0-025`: Discrimination runs with every unit eligible (`--max-hunks` = source hunks + 64), `--max-mutants 8` and `--wall-time 1800s`, using the shipped Go operator set only.
- `VRP-V0-026`: CEM maps stay repository-relative inside a disposable worktree (CEM-V1-647) and are copied out after each step; the worktree is removed after the change.

### Labels
- `VRP-V0-030`: The baseline flags a unit when at least one executable added line was not executed; the all-unexecuted variant is reported as secondary.
- `VRP-V0-031`: The typed label is folded first-match from hunk-revert, discrimination and coverage witnesses into the V1-2602 vocabulary (protocol section 4.3); the typed flag is `weak-test` or `uncovered(no-executed-line)`.
- `VRP-V0-032`: `likely-equivalent` is `NOT_PRODUCED` while no equivalence screen exists; its absence is disclosed as a bias toward typed false alarms.
- `VRP-V0-033`: A unit whose witnesses are missing, timed out or not run is `NOT_OBSERVED(reason)`, never estimated.
- `VRP-V0-034`: The runbook commands in protocol section 7 are complete: `check`, `plan`, `run`, `score` start the replay with no further design work once X and Y are set.

### Results
- `VRP-V0-040`: `score` emits one row per change: per-unit flags and labels, capture per arm (`true`, `false`, or `NOT_OBSERVED`), flagged units, false alarms and replay seconds.
- `VRP-V0-041`: A false alarm is a flagged unit touched by neither the fix nor a later fix.
- `VRP-V0-042`: The CEM 1.0 dogfood cost comparator is `NOT_OBSERVED` unless measured on the same host and changes; no cost ratio is claimed without it.

### Decision
- `VRP-V0-050`: Over paired changes, `delta = (b - c) / paired`, where b counts typed-only captures and c baseline-only captures; `FA_typed` is pooled typed false alarms over typed-flagged units.
- `VRP-V0-051`: The verdict is CONTINUE iff `delta >= X`, `FA_typed <= Y` and `b > c`; otherwise KILL.
- `VRP-V0-052`: A run with fewer than 80% of selected changes paired is `INVALID_RUN`; one rerun after an infrastructure-only repair is allowed, and a second `INVALID_RUN` is recorded as KILL.
- `VRP-V0-053`: Secondary measures never change the verdict.
- `VRP-V0-054`: The verdict and every per-change row are recorded in a build-log entry; a KILL is recorded as a failed evaluation, not softened.

## Non-goals
- No new mutation operator, equivalence screen, label producer or CEM wire field.
- No product claim, ranking input or shipped default from this slice.
- No run, and no reading of any replay output, before X and Y are committed.

## Failure modes
- Selection bias from blame linkage: lines rewritten by other commits drop the pair; disclosed, not corrected.
- No post-merge stratum on public `origin/main` (all 40 are same-merge); the result speaks for review-caught defects only.
- Historical changes judged by today's verifier: recorded per change (`VRP-V0-023`).
- Surviving equivalent mutants inflate `weak-test` (`VRP-V0-032`).
- Host overload or timeouts turn units into `NOT_OBSERVED`; enough of them yields `INVALID_RUN`, not a verdict.

## Acceptance evidence
- Pre-registration (this slice): committed protocol, manifest digest `5a51e33a42dc6a329e9e3f90350653b395b6270f21953df0b90eee5b5cd29cda` reproduced twice byte-identically, `harness.py selftest` passing on synthetic witnesses, and `harness.py check` reporting OWNER-BLOCKER (build log `docs/build-log/2026-11-01-drp-u15.md`).
- Run (later): per-change rows and the verdict in a dated build-log entry, under `VRP-V0-054`.

## Rollback
Delete `benchmarks/cem2-discovery-replay-v0/`, this spec and its index row; nothing else depends on them.
