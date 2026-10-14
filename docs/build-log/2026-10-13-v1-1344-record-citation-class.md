# 2026-10-13 V1-1344: a `record` citation class for evidence-record changes

Lane V1-1344, Corvint 1.0 scope `PRS-V1-018` (accepted, owner instruction 2026-10-13).
Requirements `DCW-V0-057..059` (`docs/specs/daily-change-evidence-workflow-v0.md`) and `CEM-V1-170`
with a dated `CEM-V1-106` amendment (`docs/specs/cem-stable-v1.md`), all accepted under the owner's
standing auto-accept (2026-10-11); decision 0527 gains a dated amendment.

## Intent

A change that only adds build-log entries or agent evidence records evidence about other work. The
bind loop treated it as a product change: its intent had no requirements section, so naming the
entry refused `invalid-requirements-section`; its plan usually cites one BASE decision for every
hunk, so `DCW-V0-048` refused it as uniform; and its map counted in the `change` class that
decision 0527's authored arm scores. V1-1278's post-S3 baseline (PR #855) shows the shape: four
hunks under `docs/build-log/`, each citing a separate span of the DCW spec only to avoid the uniform
refusal.

## Decisions

- Classification (`DCW-V0-057`): a change is a record when `git diff --no-renames --name-only BASE
  HEAD` names at least one path under `docs/build-log/` or `.agent-evidence/` and every other path
  is `.corvint/change.cem.json` or `.corvint/changes/NAME.cem.json`. A mixed change is a change; a
  sidecar-only change stays a chore. `internal/dogfoodflow/record.go` `RecordChange`.
- No `record` role. The stable `role` enum (`change`, `bind-chore`, `seal-chore`, `CEM-V1-095`) is
  closed in the wire, schema and portable consumer, so a record map carries no role, and
  `cem status --citation` derives `class: record` (role `change`) from the map's hunk paths
  (`CEM-V1-170`, `internal/cem/workflow/stable_read.go` `stableRecordPaths`). The predicate is
  duplicated there because `internal/cem` does not import `internal/dogfoodflow`; both comments
  name the other.
- Binding (`DCW-V0-058`): an intents manifest naming only `docs/build-log/NAME.md` entries the
  change touches or `docs/decisions/NAME.md` files present at the target, on a record change, is published as the `DCW-V0-024` no-intent
  declaration, so OCM steps are `no-intent-declared` and `dogfood check`, unchanged, accepts it.
  Drift is still checked against the source manifest bytes. The uniform-plan refusal skips record
  changes. Evidence is a BASE decision or build-log entry with relation `decision`: `cem cite`
  refuses evidence absent at BASE, so a record's new entry cannot cite itself, and the registered
  relations have no `evidence` member.
- Measurement (`DCW-V0-059`): `tools/cem-citation-baseline` adds an always-present `record`
  summary member after `chore`; decision 0527's authored arm excludes record maps; the kill
  threshold is unchanged.

## Failure modes and limits

- Existing committed record maps: classifying by hunk paths at BASE `523fa4ce6`, 12 of the 640
  committed maps now read class `record` (`01e2f0a3`, `070fcd63`, `29772b4a`, `36ae2f5e`, `6e5414b5`,
  `86675edc`, `a209e6a2`, `a4eb77ea`, `c38e6836`, `cbc4d327`, `e5a3ddd1`, `f3a6d467`). The 0527
  pre-registration baseline counted them as `change`. It is not rescored, because it is a baseline,
  not the test.
- Prospective registration: the registered scorer is pinned at blob
  `40b37c5e520881baf390e7547c7d505d9d487654` from the V1-1336 merge `dc6811176`. In that tree a
  record map reads class `change`, so registration criterion (c) still admits record maps.
  `01e2f0a3` (PR #855) is already one of the 9 maps added after M. This change gives the scorer a
  new blob, so by `DCW-V0-056` the exclusion reaches the prospective set only through a new dated
  registration made before its maps are read. That registration is left to the owner of V1-1337.
- The published intent snapshot is `#no-intent-declared`, so the record's entry name is not kept
  beyond the citation plan and the map's hunk paths.
- A record that also edits a decision, spec or code is a change and keeps every refusal.
- Independent review (Codex, one round) returned FAIL with three findings, all fixed in this lane:
  an absent or unrelated record intent was accepted (now the entry must be in the change's diff and
  a decision must be a blob at the target, else the manifest is assessed as path intents); the
  duplicated predicates had no parity test (now `TestRecordChangeDCWV0057MatchesStatusClass`); and
  drift on a record intent was untested (now a subtest rewrites the manifest during `cem cite` and
  requires `intent-scope-drift` with nothing published).

## Acceptance evidence

- `TestRecordChangeDCWV0057Classifies`, `TestRecordChangeDCWV0057MatchesStatusClass` and
  `TestChangeDCWV0058RecordBindsUniformPlanAndRecordIntent` (`internal/dogfoodflow`; six subtests:
  record, record naming a decision, mixed, absent decision, untouched entry, drift). A mutation that disables `recordChange` fails the latter.
- `TestCEMV1170StatusCitationClassRecord` (`internal/cem/cli`).
- `TestBaselineDCWV0059SplitsRecordRows` (`tools/cem-citation-baseline`).
- `TestDogfoodDCWV0058RecordChangeBindsChecksAndSeals` (`cmd/corvint`). It runs the built binary on a
  foreign fixture: change, bind, change, check PASS, seal, then `cem status --citation` on the
  sealed map reads `{"class":"record","role":"change"}`.

## Rollback

Revert the lane's change commit. Record changes become changes again: a uniform plan is refused, a
build-log intent is read as a path intent, and status and the baseline report record maps as
`change`. Nothing needs migrating, because no map, report, sealed CEM or wire format changed.
