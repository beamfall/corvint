# 2026-10-23 V1-2449: racing lease verbs and a torn head.json guard read

Lane `claude/cloud-v1-2449`, base `368b80d438669f618ac4cf1884ac19dd65fbc4a5`. Go 1.27.2, Linux, 4 CPUs.

## Question

`TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHead` failed on release-gates macos-15 with
`racer 2: SNAPSHOT_MOVED: .../.git/taskman/head.json: file replaced ...`. Is `SNAPSHOT_MOVED` an
accepted contention outcome the test must tolerate, or is a lease verb missing a bounded retry?

## Decision

A missing retry, and the test stays strict. CAL-V0-019 requires each racing verb to commit or
refuse with a report. `SNAPSHOT_MOVED` returned as an error from a single torn observation is
neither. CAL-V0-078 calls the code retryable. CAL-V0-372 requires the lease write's unlocked
pre-round guard read to spend a round on it, not return it while rounds remain. No requirement
defines a returned `SNAPSHOT_MOVED` as an accepted outcome of one torn read, so loosening the
assertion would hide the defect.

- Cause. The only reader that turns a stat/open inode mismatch on `head.json` into the
  absolute-path message (`intent.ReadFile`: `file replaced between stat and open`) and returned it
  to a racing verb was `leaseWrite`'s unlocked pre-round `writerGuards` → `readHead`. Every other
  unlocked read on the gate-run and lease paths (`readLeaseProof`, `prepareLease`,
  `prepareLeaseRecovery`, `settleLease`) already retries the code within its sixteen rounds; the
  locked reads cannot see a rename. The quoted wording "replaced before read" exists nowhere in
  any branch's history, so the ticket text paraphrases this message.
- The fix is already on main: #1551 (`4d1568242`, merged 2026-10-23 20:25 UTC, CAL-V0-372) spends
  the round. The failing nightly release-gates run started before that merge (run 30, head
  `77a49dfa`, 11:39 UTC). This lane adds no production change.
- What this lane adds: `TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHeadThroughTornGuardReads`.
  It runs the same nine-verb race (the body is now the shared `raceLeaseVerbs` helper) with the
  store's first guard read torn through `SetLeaseGuardFaultForTest`, so the racing
  qualification itself proves the retry instead of relying on host timing. The test name extends
  the original, so `-run TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHead` selects both.

## Evidence

- Natural reproduction on Linux: not reached. 30 runs of the unchanged test at `nice -n 15` beside
  six busy loops passed. With an instrumented build that slept 0–3 ms between the `head.json`
  stat and open, 470 head reads in 2 runs produced no torn read. At 0–20 ms, 10 runs produced one
  torn read, inside `prepareLease` → `journalBytes`, which retries. The macOS fsync-latency window
  is NEEDS-HOST.
- New test without the retry (the CAL-V0-372 `continue` disabled locally): 3 of 3 runs fail,
  `racer 8`, `racer 4` and `racer 3: SNAPSHOT_MOVED: head.json: file replaced between stat and
  open`. With the retry the same 3 runs pass.
- Tear count. A first version tore nine reads. Under `nice -n 15` beside six busy loops it
  failed once in 93 runs with `racer 3: SNAPSHOT_MOVED: lease: store changed during all
  preparation attempts`: the tears can pile onto one racer and spend its sixteen rounds. One tear
  still proves the retry, so the test tears one.
- With the retry, under load: `nice -n 15 go test -v -count=50 -timeout 90m -run
  TestCALV0019_RacingLeaseVerbsLeaveOneConsistentHead ./internal/tasks/store`, run beside four busy
  loops (one per CPU, load average 7.4–7.7): 50 of 50 runs of each test passed (100 of 100), `ok`
  in 3202.7 s. A loaded run takes 28.7–36.8 s (median 31.7 s; about 1.6 s idle), so 100 runs do
  not fit the 30-minute per-package hang detector. The same command at `-timeout 30m` with six
  busy loops (nine-tear version) panicked on the timeout after 1801 s; only its last lines were
  kept, so its per-run results before the timeout are not known.

## Rollback

Revert the commit: the racing test body returns inline and the torn-read variant goes away. No
product behaviour changes either way.
