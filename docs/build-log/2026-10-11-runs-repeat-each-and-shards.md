# #809 and #810: repeat-each observations and shard sets for `corvint runs`

Date: 2026-10-11. Base: `645304a2fb71065f9061d8d8d866600d703e84c6` (`claude/issues-809-810-runs`).
Scope: `PWR-V0-011` to `PWR-V0-023` in `docs/specs/playwright-run-evidence-v0.md`, experimental,
in `internal/playwrightruns` and the `corvint runs` help. No decisions, ticket-store writes, PRs
or CEM binding in this lane.

## Observed reporter shape (Playwright 1.61.1)

- `--repeat-each N` sets `config.projects[].repeatEach` and emits one spec entry per repeat with
  the same title, line and column, in repeat order; there is no repeat index in the JSON output.
- `--shard i/N` sets `config.shard {current,total}`. Combined with `--repeat-each`, Playwright
  shards the repeat instances: in a 2-shard, repeat-each-2 run the same test sits once in each
  shard, so repeats can only be counted over the merged set.
- `merge-reports --reporter json` has `config.shard: null`, lists each project once per merged
  shard, and merges `config.metadata` shallowly.

## Design

- Logical runs (`shards.go`): an unsharded report is its own logical run; shard reports group by
  shard total, Playwright version, declared application revision, config digest and test-file
  blobs, and the `repeatEach` declaration; shard total and `repeatEach` are bounded at 1024. A
  group is admitted only when shards 1..total are each present once at declared inputs, else
  refused with `SHARD_INPUTS_NOT_DECLARED`, `SHARD_SET_INCOMPLETE` or `SHARD_DUPLICATE` (that
  precedence; `missing` and `duplicate` always listed).
- Repeats (`reader.go`): occurrences are numbered in report order; a duplicated identity is
  ambiguous unless all occurrences share one reported spec location and the count equals
  `repeatEach`.
- Stability counts each repeat as an observation of its logical run (`repeats[]` beside `runs[]`);
  compare counts per-run repeat slots (absent slots counted, never allocated), so k/N matches N
  single-repeat reports.
- Byte identity: every new field is `omitempty`; goldens taken at the base pin unsharded
  single-repeat stability and compare documents (`TestSingleRepeatOutputByteIdentical`).

## Fixtures

`internal/playwrightruns/testdata/runs/*.json` are real `@playwright/test` 1.61.1 json-reporter
outputs from a three-file scratch suite (one deliberately failing on repeat 1), sanitized to a
`/work` root with identity metadata `app-1`/`cfg-1`. Synthetic derivatives: `shard-3-of-3-overlap`
(shard 3 plus shard 1's file), `repeat3-count-mismatch` (one repeat removed); `shard-3-of-3-app-2`
is a real run at another declared revision (`APP_REV`); `shard-2-of-3-again` is a real rerun;
`whole-adds-failed` is a real single-repeat run with the cart test forced to fail (`FAIL_ADDS`),
declaring the same test-file blob as a fixture convenience so it pools with `whole`. The reports
were produced with an existing local Playwright 1.61.1 install, read-only; no network install.

## Limits

- Two complete shard sets at the same inputs are one group with every shard duplicated and are
  refused as `SHARD_DUPLICATE`; pass such runs through `--repeat-each` or `merge-reports` instead.
- Repeat order is report order; a reporter that reorders specs would renumber repeats.
- Shard sets with undeclared inputs are refused rather than grouped on version alone.
- `merge-reports` identity inputs are whatever the shard reports shared (shallow metadata merge).

## Review

Codex round 1 (`/private/tmp/claude-501/w13-runs-codex-r1.log`) found three P2s, all fixed with
tests: shards declaring different test-file blobs were merged (blobs are now in the group key);
repeats without a reported spec line counted as one location (now ambiguous); declared shard totals
and repeat counts were unbounded (now 1024, `REPORT_BOUND_EXCEEDED`, and compare no longer
allocates absent slots). The reviewer also filed V1-1248 to V1-1250 for these findings in the
native queue from its read-only run; this lane wrote no tickets.

## Evidence

`corvint --root <worktree> context` (Corvint 1.0.0-rc.3 build 407, revision `bd944d2f`, read-only)
routed AGENTS.md, the four package files, the CLI and its tests and the owning spec. Focused
`go test` of `internal/playwrightruns` and `-run TestRuns` in `cmd/corvint`, plus `go vet` of both,
pass. Live qualification against an adopter suite and the repository-wide gate are NOT_RUN.
