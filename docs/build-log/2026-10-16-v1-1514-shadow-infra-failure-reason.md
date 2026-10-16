# 2026-10-16 V1-1514 forward rows: a hosted set-up failure is `INFRA`, not `INCOMPLETE`

Ticket V1-1514 (P2 BUG). Work started at `653048415716e71a305091957bd321c2120a3b20` (origin/main) and
was merged forward to `449c9d1a2a0ae2395577b819b85b3db3c5f61cae`, the bind base. The owner approved
option (a) in chat on 2026-10-16: a new forward-row class `INFRA` for hosted pre-step failures,
excluded from the AFP-V0-030 `INCOMPLETE` trip count and from `complete`, with AFP-V0-016 consent for
the `.github/forward-rows/` edits. The AFP-V0-029/030 amendment is recorded as decision 0537;
AFP-V0-052 is a new lane requirement accepted under the owner's standing auto-accept of 2026-10-11.
`.github/workflows/ci-control-plane.yml` needed no edit: the `forward-rows` job already runs
`collect.sh` with `GH_TOKEN` and `actions: read`, which covers the jobs API.

## Finding

Merge queue run 38686348902 (PR #908, queue sha `cab1c638a36038aa06847550717158f9d1050026`) lost
`go-product-shard (5)` at its only step, `Set up job`. The real forward row was
`ci-forward-INCOMPLETE-pr908-cab1c638…-e1-f0`; its only incomplete reason was that the outcomes
documents covered shards 0,1,2,3,4,6,7 of 8. The shard count is 8 (`FORWARD_SHARDS`), not the 9 the
audit plan named.

## Contract decisions

- `collect.sh` reads the outcomes run's job list (`filter=latest`) and names each
  `go-product-shard (N)` job concluded `failure` whose first step, `Set up job`, failed. An unreadable
  job list names nothing and says so in the step summary, so the row stays `INCOMPLETE`.
- `row.jq` keeps a separate `infrastructure` array (also listed in `incomplete`) and sets `INFRA` only
  when the named shards are below the shard count, the outcomes documents cover exactly the other
  shards, the selection documents cover every shard (or, when the selection run is the outcomes run,
  exactly the other shards), no other incomplete reason remains, and no missed package failed its
  rerun. Anything else stays `INCOMPLETE`; a hosted failure never makes a row `CLEAN`, `FLAKY` or
  `MISS`. The non-`FLAKY` miss rule is a conservative reading: a row with a real miss and a lost shard
  keeps the `INCOMPLETE` it had before.
- `bar.jq` accepts `INFRA` names, gives an `INFRA` row no place in the latest-20 window (it is skipped,
  so the window still holds 20 classified rows), never counts it as complete or `INCOMPLETE`, and
  reports how many `INFRA` rows lie between the window's rows.
- AFP-V0-047's closed row-key list gains `infrastructure`; this follows from option (a) and is recorded
  in AFP-V0-052 rather than as a separate owner amendment.
- Selection-run analogue: trivial, not a limit. A queue row's selection run is the latest successful
  pull-request run, which has no failed job; a pull-request row's selection run is its outcomes run, and
  the selection coverage gap of exactly the named shards is explained in the same way.

## Evidence

Loads are the 1/5/15-minute averages of the shared 12-core host.

- BASE scripts with the new tests: `TestAFPV0029ForwardRowsAndShadowBar` 29 subtests pass, 5 fail (CLEAN
  row keys, both `INFRA` subtests, the no-hosted-failure-named subtest) and the run then panics on the
  missing `infrastructure` key; `TestAFPV0047CollectorNamesRows` and `TestAFPV0047BarCountsArtifactNames`
  fail (load about 72 before, 59 after).
- Fixed: `go test -count=1 ./tools/ci-forward-row/...` passes (179 s, load about 67 to 74); the collector
  test alone passes after the set-up-only fix (125.7 s, load about 60 to 65).
- The collector test replays the real job list of run 38686348902 (trimmed, `testdata/jobs/`) and names
  `ci-forward-INFRA-pr868-…-e1-f0`; the same job list with shard 5's outcomes present, and a copy where
  shard 5 set up and then failed `Go tests`, stay `INCOMPLETE`.
- Mutations, each caught: dropping the `$unexplained` check (load 68.8), dropping the non-`FLAKY` miss
  check (66.5) and making the outcomes coverage always true (64.3) fail
  `TestAFPV0029ForwardRowsAndShadowBar`; counting `INFRA` as a window row fails
  `TestAFPV0047BarCountsArtifactNames` (62.5); removing `INFRA` from the class whitelist (61.5) and
  dropping the `Set up job` conclusion test (71.5) fail `TestAFPV0047CollectorNamesRows`. A first version
  also required the job to have exactly one step; that mutation survived because the conclusion test
  already decides it, so the length test was removed and a set-up-then-tests-failed case was added.

## Limits

- A hosted `INFRA` row is `NOT_OBSERVED` until a set-up failure recurs on main after this change.
- A queue run whose every shard failed at set-up retained no outcomes artifact and stays `INCOMPLETE`
  on the collector's reason.
- The job list is read with `per_page=100`; the shard workflow has far fewer jobs.
- A cancelled job, a later-step failure and a runner lost mid-job stay `INCOMPLETE` by design.
