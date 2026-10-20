# 2026-10-20 V1-1266: K1 measured over the dogfood changes after P2 (kill criterion fired)

V1-1266 is decision 0523 P2. Its code and producer step are already on `main`:

- the consumer falsifier `cem verify --hunk-revert` (`CEM-V1-088`/`089`, #893);
- the offered citation step (`TCQ-V0-060`/`061`, `docs/DOGFOOD.md` §4, #893);
- the observed guarding-test search `cem verify --guarding-tests` (`TCQ-V0-062`, #944);
- the shared temporary-directory refusal (`CEM-V1-250`, V1-1452).

The ticket text proposes `CEM-V1-068` and `CEM-V1-069`. Decision 0523 renumbered every draft ID by
+20, and the P2 rows landed as `CEM-V1-088..089` under the owner's standing auto-accept of lane
requirements (2026-10-11). This change adds no requirement, test or code. It records the K1
measurement that acceptance criterion 2 left open, and updates the K1 lines of the rc.3 release
notes. Earlier entries: `2026-10-14-v1-1266-hunk-revert-held.md` (design, latency) and
`2026-10-15-v1-1266-guarding-tests-k1.md` (tally definition, first observations).

## Acceptance re-check at base `c27d54e92`

| Criterion | State | Evidence |
| --- | --- | --- |
| 1. `held` only from consumer re-execution; build break and fail-without-revert are `not-run` | Delivered | `go test -run 'TestCEMV108[89]'` passes in `internal/cem/verify` (9 tests), `internal/cem/workflow` (3, including the real-`go test` fixture `TestCEMV1088HeldOnlyFromConsumerReexecution`) and `internal/cem/cli`, run at this base on 2026-10-20 |
| 2. Producer step plus K1 held share over the next 30 dogfood changes | Step delivered; K1 measured here | See below |
| 3. Non-Go runners recorded as limits; none reported qualified | Delivered | `docs/RELEASE-NOTES.md` "1.0.0-rc.3 (unreleased): recorded limits" |

## K1 measurement

K1 (decision 0523 §5): fewer than 25% of non-mechanical Go code hunks are `held` across the next 30
dogfood changes after P2 lands. Then the hunk-revert headline is dropped from rc.3 text, and P0 ships
alone.

Method: the 2026-10-15 tally definition, unchanged.

- **Window.** A map counts when its sealed file `.corvint/changes/<bind>.cem.json` is absent at
  `f81114713` (the #944 merge, 2026-10-16T01:57:07Z) and present at base `c27d54e92`. Maps are
  ordered by the first-parent commit of `main` that added them. That gives 350 maps, the last
  landing 2026-10-20T03:10:39Z. All 350 declare `cem/1.0`.
- **Denominator.** Hunks whose path ends in `.go` but not `_test.go` and whose disposition is not
  `mechanical`.
- **Upper bound.** Supported hunks in the denominator with a `test-claim` basis on a `_test.go`
  evidence path. Only these can be `held`.
- **Numerator.** `.hunkRevert.held` from
  `corvint cem verify --map .corvint/change.cem.json --expected-base <base> --target <bind>^ --artifacts "$PWD" --hunk-revert`,
  run at `<bind>` in a scratch clone (`corvint 1.0.0-rc.3 build 676`). The map at `<bind>` is
  byte-identical to its sealed copy in every run.

| Window | Maps | Maps with Go code hunks | Denominator | `test-claim` upper bound | `held` | Share |
| --- | --- | --- | --- | --- | --- | --- |
| First 30 maps (to `11a235c3a`, 2026-10-16T09:12:57Z) | 30 | 11 | 95 | 0 | 0 | 0% |
| First 30 maps with Go code hunks (to `d4b1e2039`, 2026-10-16T23:08:29Z) | 74 | 30 | 270 | 0 | 0 | 0% |
| All maps to base | 350 | 236 | 3,790 | 49 | 0 | 0% |

**K1 fired.** Under either reading of "30 dogfood changes" no hunk could be `held`, because no map
cited a guarding test. The full window holds 49 candidate hunks in 10 maps, and the consumer held
none of them. Following decision 0523 §5, the hunk-revert headline is not carried in rc.3 text, and
P0 (consumer-derived tiers and the `citation` block) is the rc.3 support claim. `--hunk-revert` and
`--guarding-tests` stay shipped as opt-in steps with their recorded limits. Whether to keep them,
retire them, or rerun K1 after making the step automatic is not decided here.

### The 49 candidates

Each map was run once on a shared host (load average 52 to 116).

| Bind | Landed in | Candidates | `held` | `survived` | `not-run` (reason) | `elapsedSeconds` |
| --- | --- | --- | --- | --- | --- | --- |
| `31be1c53` | `70a21da5d` | 15 | 0 | 4 | 6 `revert-breaks-build`, 5 `no-named-test` | 33.8 |
| `a2b2036f` | `70a21da5d` | 15 | 0 | 4 | 6 `revert-breaks-build`, 5 `no-named-test` | 26.9 |
| `b81ce430` | `70a21da5d` | 2 | 0 | 2 | none | 6.1 |
| `b9c835c8` | `2c5c01be7` | 4 | 0 | 3 | 1 `revert-breaks-build` | 12.1 |
| `6c980ddc` | `b3db71094` | 3 | 0 | 1 | 2 `revert-breaks-build` | 9.0 |
| `38b05710` | `70a1a9a02` | 4 | 0 | 0 | 4 `revert-breaks-build` | 16.7 |
| `7971a87e` | `70a1a9a02` | 1 | 0 | 0 | 1 `revert-breaks-build` | 11.7 |
| `8166fc95` | `70a1a9a02` | 3 | 0 | 3 | none | 46.6 |
| `9ae89fcd` | `70a1a9a02` | 1 | 0 | 1 | none | 23.0 |
| `b02a3a36` | `fa8afe679` | 1 | 0 | 0 | 1 `not-a-modification` | 0.02 |
| Total | | 49 | 0 | 18 | 31 (20 `revert-breaks-build`, 10 `no-named-test`, 1 `not-a-modification`) | |

`31be1c53` and `a2b2036f` give identical counts and appear to be two binds of one change; they are
counted as recorded.

### Why the share is zero

- **The step was not used.** `--guarding-tests` emits a plan row only for a test it saw fail with
  that one hunk reverted. Re-run at the same target, such a row is `held` unless the test is flaky.
  None of the 49 rows is `held`, so, by inference, none came from the step. The citations were
  written by hand.
- **Hand citations name the change's own new tests.** Ten rows cite a span with no base test
  declaration (`no-named-test`). Twenty fail to compile with the hunk reverted, typically because a
  new test calls the new code. Eighteen name a test that still passes reverted (`survived`), so the
  citation is weaker than it claims. That is the outcome the falsifier exists to expose.
- **No automatic run.** `internal/dogfoodflow` reports `guarding-tests` `NOT_PRODUCED`
  `guarding-tests-not-run` and does not block, so producers skip the search. Its probe cost
  (1200 s on a 42-hunk change, `2026-10-15` entry) also discourages manual use.

## Other kill criteria touched

- **K3.** No `held` witness exists to audit. `NOT_OBSERVED`.
- **K5.** The falsifier's own `elapsedSeconds` on the 10 maps ranges from 0.02 to 46.6 s, median
  14.4 s. No dogfood command passes the flag, so default dogfood latency is unchanged. A
  with-and-without median over dogfood changes stays `NOT_OBSERVED`.

## Release notes

`docs/RELEASE-NOTES.md` rc.3 section: the K1 bullet under "recorded limits" now records the measured
share and points here, and the kill-criteria list records K1 as met. Each edit keeps its line count,
so the decision 0375 line citations further down the file stay pinned.

## Residuals

- **Ticket state.** All three acceptance criteria now have retained evidence. K1 is a kill outcome,
  not a pass, and the orchestrator decides closure.
- **Follow-up.** Whether to rerun K1 once the step runs automatically, or to retire the P2 surfaces,
  needs an owner or orchestrator decision.
- **Outside this lane's paths.** `docs/CHANGE-EVIDENCE-MAP.md` lists K1 without its outcome. It
  is not edited here. The spec needs no change: no requirement text claims the headline.
