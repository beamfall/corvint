# V1-1590 batch: preflight cache, repeat-failure refusal and the run baseline gate

One batch for GitHub #1037 (V1-1594), #1048 (V1-1605) and #1033 (V1-1590), plus V1-1609 and
V1-1611 on the `corvint runs reusable` reader of V1-1578 (#1051). Every requirement below is
agent-drafted and accepted under the owner's standing auto-accept of lane requirements
(2026-10-11). IDs were checked against origin/main and the #1051 branch before numbering.

## V1-1594: preflight result cache (TOL-V0-047..050)

- Opt-in `preflight --cache` instead of a default: an unchanged default keeps every existing
  output byte-identical and keeps the read honest for callers that never asked for replay.
- The key covers what the static checks read (record, ledger, policy, commit) and what the issue
  names even though preflight reads Git objects only: `HEAD`, the tracked diff, untracked files
  and the executable digest. An over-wide key costs a miss; a narrow one would replay a stale
  "ready".
- The cache lives in the user cache directory, never in the repository or `.corvint/`, so
  `preflight` remains a read under product invariant 4 and nothing can mistake an entry for
  evidence. Any keying failure runs the checks (`SKIPPED`), never replays.
- `--cache --deep` is refused: gate commands are not static and depend on the environment.

## V1-1605: refuse identical failing re-runs (TOL-V0-051..054)

- Off by default (`--refuse-repeat N`), as the issue asks; TOL-V0-031's per-`--out` refusal is
  unchanged and still applies.
- The input digest reuses the preflight cache key, and for `run-batch` the clean commit, fixture
  digest, config, specs, ledger and scope, plus the executable digest. A rebuilt tool, a new
  commit or a ledger change is a new input, so honest progress is never refused.
- Signatures fold digit runs and whitespace so durations and ports do not split one failure.
- A remediation is a single-use reference audited before the run, not a judgement that the fix
  is real; reusing it is refused, so an agent cannot replay one excuse.
- An unreadable ledger runs the job unguarded (`SKIPPED`): the guard saves effort and must never
  block honest work on a corrupt private file.

## V1-1590: revision-pinned failure baseline (PWR-V0-041..045)

- Deviation from the issue's proposal: `corvint runs baseline record` does not run the suite or
  the isolated re-run. It reads the retained full and isolated reports and prints the baseline
  for the caller to retain, so `corvint runs` stays read-only under product invariant 4 and the
  baseline remains a caller file, never Corvint state.
- Known requires a failure in both runs, and every excuse re-reads both reports at diff time
  against their recorded sha256; a moved, rewritten or now-passing report un-excuses. Full-run-only
  failures, including flaky results, are suspected flakes and are never excused.
- The specs live in PWR-V0 rather than CAL-V0 or the obligation ledger: the gate consumes
  Playwright report identities, and the existing `runs` reader already owns identity, ambiguity
  and report-error rules, which the diff reuses.
- Staleness is reported, not blocking: a stale baseline still excuses only verified failures, and
  making age a verdict input would turn an unrelated base move into a red gate.
- Accepted on 2026-10-17 under the owner's standing auto-accept (2026-10-11).

## V1-1611: reports from another machine (PWR-V0-036, no new requirement)

- #1051 already resolves a test file through a declared `testRootPath` and returns
  `TEST_ROOT_UNRESOLVED` otherwise, but no CLI test drove a report whose `config.rootDir` is another
  machine's path. The added `TestRunsReusableCLI` subtest covers `FRESH_PASS`, `STALE`, a missing
  root and an escaping root; the behaviour needed no code change.

## V1-1609: not delivered in this batch

- Letting `runs reusable` classify `corvint test` units needs `corvint test` receipts to feed a
  reuse decision, which STR-V0-011 (decision 0524) forbids: receipts "MUST NOT feed ranking,
  selection, learning or authority", enforced by `TestSTRV0011ReceiptsHaveNoOtherReader`. That is an
  amendment to an owner-accepted requirement, outside the standing auto-accept for new lane
  requirements, so V1-1609 stays open for an owner decision on the STR-V0-011 amendment.
