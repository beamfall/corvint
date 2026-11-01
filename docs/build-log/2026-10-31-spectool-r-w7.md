# Lane spectool-r (wave 7): lane flow guide checks, SSH lane channel, friction metrics

Native tickets V1-3239, V1-3605, V1-3620 and V1-0822 (V1-2563 skipped: a `.github` change);
immutable base `5ffe8ed4a37eb67af5f5176ff17eb6df6e8911bb`; Unraid lane host, branch
`claude/lane-spectool-r-1101`.

## Problem

- V1-3239 (P1): lanes rediscover recoveries after a refusal. The guide (section 13 of
  `docs/LANE-ORCHESTRATION.md`) already existed from an earlier docs lane, but nothing checked it
  against the CLI (AC2), no refusal audit was retained (AC3), and nothing stopped a new refusal code
  without a recovery hint (AC4).
- V1-3605: the guide said the Unraid `~/lanes/heavy.sh` was the delegating wrapper on 2026-10-31,
  while lane scripts-w5 saw the prototype.
- V1-3620: no documented closeout channel from an SSH lane host to the orchestrator.
- V1-0822: no deterministic, local friction metrics for comparing context-packet arms.

## Decisions

- V1-3239 "shipped skill/guide": the guide stays section 13 of `docs/LANE-ORCHESTRATION.md`,
  versioned with the product; a thin repository skill `.claude/skills/corvint-lane-flow/SKILL.md`
  points to it so Claude Code lanes discover it. It is not added to the installed host plugins:
  the flow (heavy.sh, dogfood closeout) is Corvint self-development, not user product behavior.
- The checks are one new AGPL tool, `tools/flow-guide-check`, in `DOC_GATE_TARGETS` (rule
  `flow-guide-check`). `tools/capabilities-check` was not extended because its files are
  Apache-2.0 and a new Apache file belongs only under `protocol/**`.
- Guide drift (ASD-V0-996) resolves `corvint`/`corvint-tasks` spans against the
  `internal/capcatalog/trees` snapshots (longest literal-word prefix, then flags), `make` spans
  against Makefile rules, `script/` spans against files (including `script/heavy.sh make T`), and
  table codes against non-test source. Backtick spans are read across line breaks. A bare binary
  name, a glob or a directory is prose, not a command.
- Refusal surfaces (ASD-V0-997) are closed and statically parseable: `refuse "CODE ..."` in
  non-test `script/*.sh` and `.fail`/`.refuse` with a literal code in non-test
  `internal/dogfoodflow`. The hint rule is a closed marker set (`fix:`, `want `, `run `, `rerun`,
  `set `, `use `, `usage:`), a `failRecoveries` key for `.fail`, or a guide table code. The rest of
  `corvint` and `corvint-tasks` is reported as `NOT_CLASSIFIED` counts rather than guessed at;
  corvint-tasks results already carry structured `nextAction` and `alternativeActions`.
- The ratchet keys on file and code, so a reused code in another file is a new pair. The baseline
  holds the 76 pairs found unhinted after the fixes, and only shrinks: a stale row fails.
- Top offenders (ASD-V0-998): the dogfood-check FAIL codes. One map, `failRecoveries`, gives each
  code a default `fix:` line, printed by `check.fail` only when the call names none; calls that
  build their fix at runtime (`dogfood-report-missing`, `aggregate-schema-invalid`) keep it, and the
  map is their fallback. `refuse` codes were not given defaults: their recoveries
  (aggregate enrollment, sealed replay) need owner wording, so they stay listed.
- V1-0822 lives under the unplanned-read spec (`URE-V0-996..998`): it measures the same packet
  quality. The record is caller-supplied JSON (`corvint-friction-sessions/0`), not a new ledger, and
  `corvint eval --friction FILE` is the only emitter. The report carries counts only, never a path or
  digest. Missing input is `NOT_OBSERVED` with a closed reason; an unread relevant path is
  `NOT_REACHED`. The import-boundary test lives in `cmd/corvint` (already an unbounded reader) and
  not in the new package, so the package keeps a bounded read scope.
- Order: V1-0822 was built first only because the other tickets' text was unreadable on this host
  until the orchestrator staged it.

## Evidence

- V1-3239: `go test ./tools/flow-guide-check` (`TestASDV0996RepositoryGuideResolves`,
  `TestASDV0996DriftFails`, `TestASDV0997RepositoryRefusalsAreHintedOrListed`,
  `TestASDV0997NewUnhintedRefusalFails`, `TestASDV0997StaleRowFails`) passed;
  `go test -run 'TestCheck|TestASDV0998' ./internal/dogfoodflow` passed;
  `go test -run TestDogfoodDailyPathCompletesWithDeclaredNoIntent ./cmd/corvint` passed;
  `script/dogfood-change_test.sh` passed, and failed when its new `fix:` assertion was mutated (so
  the assertion runs). `make flow-guide-check`: ok (33 guide spans, 14 mapped refusal codes, 159
  refusal sites).
- Refusal audit (`go run ./tools/flow-guide-check --audit`), base `5ffe8ed4a` versus this lane:

  | Surface | Sites | Hinted, base | Hinted, lane | Unhinted codes, base | Unhinted codes, lane |
  |---|---:|---:|---:|---:|---:|
  | `script/*.sh` | 93 | 25 | 25 | 53 | 53 |
  | `internal/dogfoodflow` | 66 | 5 | 33 | 33 | 20 |
  | `corvint`, rest of `cmd/` and `internal/` | 663 refusal call sites | `NOT_CLASSIFIED` | | | |
  | `corvint-tasks` (`internal/tasks`, `cmd/corvint-tasks`) | 458 refusal call sites | `NOT_CLASSIFIED` | | | |

  Base top offenders were all dogfood-check: `verifier-disagreement` 7 sites,
  `aggregate-enrollment-required` 4, `context-abstention-evidence-drift` 4, `dogfood-report-drift`
  4, `aggregate-binding-drift` 3. The lane gave 13 codes a default recovery, which fixed 28 sites.
  The largest remaining unhinted script groups are `script/install-cem-verifiers.sh`,
  `script/install-local-bin.sh` and `script/core-n1-source.sh`.
- V1-3605: on 2026-11-01 `~/lanes/heavy.sh` is the delegating form (mtime 2026-11-01T01:16Z); the
  prototype is `~/lanes/heavy.sh.bak-20261101-prototype` (mtime 2026-10-31T18:36Z). Every heavy run
  in this lane printed `heavy.sh: script sha256=09d1871e... <worktree>/script/heavy.sh` and took
  slots in `HEAVY_LOCK_DIR=/home/agent/lanes/.flock-slots` (`~/.config/corvint/heavy.env`,
  `HEAVY_SLOTS=2`). The ASD-V0-920 host line is not on this base, so it cannot print. The guide's
  2026-10-31 claim was corrected.
- V1-3620: from this Unraid SSH lane, SendMessage to the orchestrator returned
  `Forwarded to Claude Desktop's session messaging: Message queued` (message
  `669b8253-8df9-46a9-9d6d-540b796bf037`), and the orchestrator answered by staging
  `~/lanes/spectool-r-tickets.txt` at 2026-11-01T03:23Z. The lane's final friction list goes by the
  same route. Section 14 now names SendMessage as the channel and keeps the file fallback.
- V1-0822: `go test ./internal/frictionmetric` (`TestURE996MetricDefinitions`,
  `TestURE996MissingInputIsNotObserved`, `TestURE996RefusesMalformedRecords`,
  `TestURE998BenchmarkPrecisionRecall`) and
  `go test -run 'TestURE997|Help|Eval|Catalog' ./cmd/corvint` passed; `make capabilities-check`
  and `make capability-catalog-check` passed. Benchmark
  (`internal/frictionmetric/testdata/benchmark.json`, 5 labelled sessions):

  | Metric | TP | FP | FN | Precision | Recall |
  |---|---:|---:|---:|---:|---:|
  | `rereads` | 2 | 1 | 0 | 0.667 | 1.0 |
  | `unplanned_reads` | 4 | 0 | 0 | 1.0 | 1.0 |
  | `retried_commands` | 2 | 0 | 1 | 1.0 | 0.667 |
  | `reverted_edits` | 1 | 0 | 1 | 1.0 | 0.5 |
  | `first_relevant_read_ms` | 5 | 0 | 0 | 1.0 | 1.0 |

  The misses are the definitions' known limits: a re-read after a generator command rewrote the
  file (false positive), a retry with changed flags (a new digest), and a partial revert to a third
  state.

Gates before the final commit: `make doc-gates` targets (without `selection-baseline-check`, run
on its own: OK), `go run ./conformance/use-cases-v0` (valid), `gofmt -l` (clean), `make windows-vet`
(rc 0) and `make test-read-scopes-check unbounded-readers-check` passed. `make line-citations-check`
fails on one citation, `docs/AUTOMATION.md:97` (`cmd/corvint/main.go:1266-1349@b2b338b9`, cited
content changed), which fails identically on the base `5ffe8ed4a`; this lane does not touch
`cmd/corvint/main.go` and left it for the owning lane, after repinning the 8 moved citations its
own edits caused.

## NOT_RUN

- The exhaustive gate (`go test ./...`), by the owner's standing preference for scoped issue work;
  `corvint affected --base 5ffe8ed4a` reported scope `UNKNOWN` (114 selected, 85 unknown), so only
  focused tests ran.
- V1-0822's benchmark labels were written by this lane, not taken from reviewed real agent
  sessions, so precision and recall describe the definitions on the fixture only. Labelling
  retained real sessions stays open as follow-up work, not as a ticket criterion.
- No converter from a host transcript (for example Claude Code JSONL) to the friction record.

## Rollback

Revert the lane's commits. `tools/flow-guide-check` and its `DOC_GATE_TARGETS` entry, the
repository skill, `internal/dogfoodflow/recovery.go` with its three-line `check.fail` hook,
`internal/frictionmetric` and `cmd/corvint/eval_friction.go` store nothing and migrate nothing.

## Outcome

- Delivers V1-3239: guide drift check, refusal-hint ratchet with an audit, and default recoveries
  for 13 dogfood-check codes (ASD-V0-996..998).
- Delivers V1-3605: the guide records the observed delegating Unraid wrapper (AC "the doc states
  the real behaviour"); the host line waits for ASD-V0-920 to land.
- Delivers V1-3620: SendMessage from the Unraid SSH lane reached the orchestrator, and section 14
  names it as the channel, with a file fallback.
- Delivers V1-0822: `corvint eval --friction` deterministic metrics with `NOT_OBSERVED`, a labelled
  benchmark and an import-boundary test (URE-V0-996..998).
- Dogfood: pre-change receipts recorded by `make dogfood-change`; inputs checked with
  `make dogfood-inputs-check`; the seal is left to the orchestrator (review-before-seal).

## Friction

1. `corvint-tasks ticket show V1-3239` in a lane worktree on Unraid refuses `journal-absent` for
   any ticket newer than the committed `.taskman` snapshot, and the brief staged no
   `~/lane-inputs/ticket-bodies-<lane>/` files (section 14), so three of four tickets were
   unreadable until the orchestrator staged `~/lanes/spectool-r-tickets.txt`.
2. `script/heavy.sh go test ./internal/frictionmetric/` (a brand-new package, not in any selection
   base) refuses `whole-reason-missing`; the small-package exemption covers only `tools/` and
   `conformance/`, so a new `internal/` package always needs `HEAVY_WHOLE_REASON`.
3. `corvint affected --base 5ffe8ed4a` prints `DEGRADED self-binary-stale` (the installed binary
   was built from `f1680433b107`; 199 Go paths differ) on the lane host.
4. `corvint affected` selects `internal/tasks/cli`, `internal/releasegate` and others as
   `PATH_LITERAL_READER <- internal/frictionmetric/testdata/benchmark.json`: a new testdata file
   matches unrelated packages' path literals (probably the bare `testdata/benchmark.json` suffix).
5. After an `evalHelp` change, `TestMCPV0100HelpToolMatchesTheCLI` fails with
   `corvint.help {} differs from corvint help --json` and does not name `make capability-catalog`
   as the fix.
6. SendMessage from the SSH host reports only "queued", and the orchestrator's answer arrived as a
   file under `~/lanes/` with no notification, so the lane learned of it only by listing the
   directory again.
7. A test that reads the whole repository cannot sit in a new package without an
   `unbounded-readers.json` entry and reason; the lane moved its import-boundary test into
   `cmd/corvint` instead. The charter does not mention the rule.
8. The shell plugin refuses `.corvint/shell-foreign-callers.json` when a `reason` exceeds 512
   bytes, but `make shell-foreign-callers-check` prints only `the shell plugin refused
   .corvint/shell-foreign-callers.json`, with no cap and no script named. Repro: lengthen the
   `script/heavy.sh` reason past 512 bytes and run the check.
9. `make line-citations-check` fails on the base `5ffe8ed4a` itself (`docs/AUTOMATION.md:97`,
   `cmd/corvint/main.go:1266-1349@b2b338b9`), so every lane on this base sees one failure it did
   not cause.
