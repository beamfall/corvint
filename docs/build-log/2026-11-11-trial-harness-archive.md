# 2026-11-11: trial and replay harness results archived; deletion blocked by CI tables (V1-4825)

V1-4825 (owner accepted 2026-11-06, restructuring plan step D1) asks to archive the results of
twelve unwired trial and replay harnesses, then delete them. Base:
`b55635262788687c3eea9a08a94378e4ab8856a4`. This entry is the archive step. The deletion step
did not happen: every harness still on `main` is named by a `.github/**` file, and this lane may
not edit `.github`. Nothing under `tools/` changed.

## The twelve and their state

The set is the four the ticket names plus the eight the earlier `claude/deadcode` lane found by
importer check (2026-11-07). Each was re-checked on the base: no Go package imports any of them
(`git grep 'corvint/tools/NAME'` outside the harness), and no Makefile target or `script/**` file
runs one.

| Harness | State on the base | Files | Lines | Go lines |
|---|---|---:|---:|---:|
| `tools/cem-trial` | deleted by 8307671c7b98cb20146e0423b6dfa7cd02d23c37 (2026-11-09, decision 0594) | - | - | 7114 |
| `tools/native-hook-observer` | deleted by bed00466323db80525ea084b3f7b9889e0e72598 (2026-11-09, decision 0594) | - | - | 3180 |
| `tools/cem-citation-baseline` | deleted by 389f59ba89cf842328ff346c8e3a013016eaa27f (2026-11-09, decision 0594) | - | - | 1867 |
| `tools/beamfall-shadow` | present; blocked | 59 | 3270 | 3104 |
| `tools/compat-trial` | present; blocked | 17 | 2153 | 2082 |
| `tools/cw-trial` | present; blocked | 31 | 12272 | 5251 |
| `tools/docs-blinded-task` | present; blocked | 1 | 127 | 127 |
| `tools/heading-nav-bench` | present; blocked | 27 | 2103 | 657 |
| `tools/paired-trial` | present; blocked | 3 | 471 | 424 |
| `tools/plan-delta` | present; blocked | 2 | 233 | 233 |
| `tools/str-parity` | present; blocked | 5 | 1194 | 1133 |
| `tools/tiers-replay` | present; blocked | 3 | 362 | 362 |

The nine still present hold 148 files, 22185 lines, 13373 of them Go. Go line counts of the three
already deleted come from the 2026-11-07 archive. No harness tree changed between that archive and
this base (`git log c3b29cbb5..b55635262 -- tools/NAME` is empty for each), so its file and line
counts still hold.

## What blocks the deletion

Every present harness is a package entry in the CI shard cost table, which `.github/cishards`
embeds for shard partitioning; the earlier decision-0594 deletions were followed by a separate
`.github` change (9d5a70ca3, "Drop cishards table entries for packages cut by decision 0594").
The `walltime_test.go` lines are a frozen measured string that still names deleted packages
(`native-hook-observer`), so they alone would not block.

| Harness | `.github` references (file:line) | Other references a deletion must also clear |
|---|---|---|
| beamfall-shadow | `.github/cishards/package-costs.json:247-250`, `.github/cishards/test-commands.json:11`, `.github/cishards/walltime_test.go:541-542` | `.corvint/test-selection.json`, `test-read-scopes.json`, `test-read-scope-literals.json`, `requirement-links.json`; `docs/specs/process-group-owner-v0.md`, `affected-edit-oracle-v0.md` |
| compat-trial | `.github/cishards/package-costs.json:261-264`, `.github/cishards/walltime_test.go:547-549` | the same four `.corvint` files; `docs/specs/compat-trial-v0.md`, `compat-replay-runner-v0.md`, `affected-plan-v0.md`; `tools/test-read-scopes-check/main_test.go` |
| cw-trial | `.github/cishards/package-costs.json:267`, `.github/cishards/walltime_test.go:549` | `script/measure-context-packet.py:296-297` reads `tools/cw-trial/testdata/unseen-corvint-v{1,2}/tasks.json` at run time; `script/line-citation-unpinned-inventory.txt:116`; the four `.corvint` files; `docs/specs/confidently-wrong-trial-v0.md` |
| docs-blinded-task | `.github/cishards/package-costs.json:269`, `.github/cishards/walltime_test.go:550` | `.corvint/test-read-scope-literals.json`; `docs/specs/source-documentation-draft-v0.md` |
| heading-nav-bench | `.github/cishards/package-costs.json:276`, `.github/cishards/walltime_test.go:554` | three `.corvint` files; `docs/specs/heading-navigation-experiment-v0.md` |
| paired-trial | `.github/cishards/package-costs.json:279`, `.github/cishards/walltime_test.go:554` | two `.corvint` files; `docs/specs/agent-harness-integration-v0.md` |
| plan-delta | `.github/cishards/package-costs.json:281`, `.github/cishards/walltime_test.go:555`, `.github/linecov/qualification.json:27`, `.github/linecov/qualification.json:46` | `.corvint/test-read-scope-literals.json`; `docs/specs/test-selection-precision-v0.md` |
| str-parity | `.github/cishards/package-costs.json:287`, `.github/cishards/walltime_test.go:557` | `.corvint/non-source-paths.json` and four more `.corvint` files; `docs/specs/session-test-runner-v0.md` |
| tiers-replay | `.github/cishards/package-costs.json:293`, `.github/cishards/walltime_test.go:560`, `.github/forward-rows/github-guard.sh:113` | `tools/selection-baseline/guard_test.go:172-194`, `tools/ci-forward-row/narrowing_reasons_test.go`; `docs/specs/ci-narrowing-v0.md`, `affected-plan-v0.md` |

`tools/tiers-replay` is not unwired: `.github/forward-rows/github-guard.sh:113` maps a change to
`.github/forward-rows/tiers.sh` or `tiers.jq` to it, and `tools/selection-baseline` tests that
mapping. Deleting it needs a CI change first, not just a table edit.

The unblock is one owner-reviewed `.github` change that drops the eight remaining packages from
`package-costs.json` (and `test-commands.json:11`, `linecov/qualification.json`), plus a decision
on `tiers-replay`; then the deletions, their `.corvint` inventories, spec retirement notes and
`docs/specs/README.md` rows can land as one change. `script/measure-context-packet.py` keeps the
two cw-trial task files it reads unless it is changed too.

## Last results, kept here before deletion

The exact sources are recoverable from the base with `git show b55635262:tools/NAME/...`, and the
three already deleted from their deleting commit's parent.

- cem-trial: `benchmarks/results/cem-reviewer-trial-first-run.json` (kept). Held-out: 57 pairs
  scored, none dropped. Treatment versus control mean miss 0.554094 versus 0.573099, delta 0.019006,
  95% BCa interval [-0.047515, 0.101608], relative reduction 0.033163. Seeded arm mean miss
  0.649123, delta -0.076023, interval [-0.146199, -0.004386], relative reduction -0.132653, verdict
  "not met: treatment missed more evidence than control". Neither arm supports the 20% promotion.
  The development report `cem-reviewer-trial-development-2026-10-24.json` (10 pairs) is also kept.
- cw-trial: latest numbered development replay `benchmarks/results/cw-trial-heldout-v1-development-run-8.json`
  (kept): 50 tasks, held-out partition, repository access none; development observations, never a
  fresh held-out result. Decision 0211 keeps the three Beamfall reports that did not reproduce.
- native-hook-observer: three source-provenance evidence files only; host notification delivery,
  a completed task trial and pre-arm benefit are `NOT_OBSERVED`.
- beamfall-shadow: four-family fixture harness; network and repository-read denial
  `NOT_OBSERVED`; no dated run result exists (`NOT_PRODUCED`, not a passing trial).
- compat-trial: `benchmarks/compat-trial-v0.json` and `.md` preregister the experiment; no qualified
  outcome result exists (`NOT_OBSERVED`).
- paired-trial: offline paired-receipt analyzer; no real-session paired result retained; cost and
  resolution benefit `NOT_OBSERVED`.
- tiers-replay: `benchmarks/results/tiers-replay-2026-10-31.jsonl` (kept): 20 merges ending at
  8daec98ce4349c1583186692726d2c3a782ac647, 16 FULL and 4 NARROWED, 60.2 s. Planning evidence, not
  test execution.
- docs-blinded-task: blinded task preparation utility; no completed blinded trial result
  (`NOT_OBSERVED`).
- heading-nav-bench: pilot of eight questions in `tools/heading-nav-bench/results/pilot/`:
  baseline versus heading gold hits 3/13 versus 7/13, answer scores 10/16 versus 11/16, fully
  answered 4 versus 5, packet bytes 47664 versus 47704, reader seconds 70.95 versus 63.32,
  unsupported conclusions 0 versus 1; unequal cache hits, so no speed or cost promotion.
- str-parity: substitute corpus of 50 first-parent pairs ending at
  645304a2fb71065f9061d8d8d866600d703e84c6 (decision 0526); the original AFP-V0-011 pairs and
  AFP-V0-029 shadow rows `NOT_OBSERVED`; the last focused run is fixture validation, not new parity.
- plan-delta: offline two-receipt comparison utility; no standalone benefit measurement
  (`NOT_OBSERVED`).
- cem-citation-baseline: the 593-map baseline is archived in `beamfall/corvint-history`
  (2026-10-12 entry); decision 0527 preregisters the comparison; no new frozen run.

## Verification

- Reference survey: `git grep -n 'tools/NAME' -- . ':!.agent-evidence' ':!.corvint/changes'` per
  harness, and `git grep -l 'corvint/tools/NAME' -- '*.go'` for importers (none).
- Deleting commits: `git log --diff-filter=D -1 -- tools/NAME` for the three absent harnesses.
- Results: the cem-trial and tiers-replay figures above were re-read from the kept files on this
  base; the others are carried from the 2026-11-07 archive over unchanged trees.
