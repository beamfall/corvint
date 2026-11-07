<!-- SPDX-License-Identifier: Apache-2.0 -->
# mbrescue review fix round 4: closeout resume keeps its start, a failed repin refresh stays pending

Lane `claude/mbrescue` (PR #126), sealed at `a3dcb1bda10c184993dc7b85807ee8cc9925c941`. Greptile
raised two P2 findings on that head. This round verified each with a failing regression first,
then fixed the cause. The commits are new; nothing was rebased or amended. Requirement IDs come
from the round's block `ASD-V0-3730..3739`, which `make requirement-ids-free` reported free on
origin/main and on 40 open refs.

## Finding 1: a resume loses the unwind point (`Makefile`, ASD-V0-3730)

`make dogfood-closeout` wrote `git rev-parse HEAD` to `$(git rev-parse --git-path
dogfood-closeout-start)` before step 5 on every run. Step 5 (`dogfood-finish`) can fail after its
bind commit. Its printed resume command is `CLOSEOUT_FROM=5`, and the resumed run then recorded
the bind commit as the start. When the separate sealed-head precheck (`DCW-V0-923`) refused, the
unwind (`DWC-V0-746`) dropped only the seal and left the bind commit on the branch.

- Regression (`script/dogfood-cache_test.sh`, the closeout fixture): the fake `dogfood-finish`
  fails right after its bind commit (`TEST_FAIL_AFTER_BIND=1`) and, on a resume, reuses a bind
  commit already at HEAD. The case runs the closeout, then the printed resume, then the refusing
  precheck (`TEST_SEALED_RC=1`), and expects HEAD back at the original start. Before the fix,
  `make dogfood-cache-test` failed with `unwind: dropped 1 bind and seal commit(s) …; HEAD is
  <bind commit> again`.
- Fix: only a fresh run (`CLOSEOUT_FROM=1`) records the start. It empties the record before step 1
  and writes HEAD there before step 5, after its own repin commit. A resumed run keeps a non-empty
  record and says so (`resume keeps the start head SHA the first run recorded (ASD-V0-3730)`). It
  writes HEAD only where the record is missing or empty, which is the case when the fresh run
  stopped before step 5.
- Emptying the record, rather than deleting it, keeps the recipe free of file removal. The record
  line already treats an empty record as missing, and the unwind reads an empty start as
  `NOT_RUN no recorded start head`.
- Considered and not taken: checking a found record against the commits since it, as the unwind
  does. The fresh-run reset already keeps a previous round's record out of the resume chain. The
  remaining case is a run started above step 1 with no fresh run before it. The unwind then
  refuses safely with `NOT_RUN` because the range holds the lane's own commits.

## Finding 2: a retry skips the failed refresh (`script/dogfood-closeout-repin.sh`, ASD-V0-3731)

When `corvint dogfood propose-citations --refresh` failed after the repin commit, the script
exited 1 with the repin committed. The retry found nothing left to repin, returned early with
`nothing to repin` and exit 0, and so never refreshed the plan.

- Regression (`script/dogfood-closeout-repin_test.sh` case 5d): a fake corvint whose refresh fails
  on its first call only. The first run exits 1 with the repin committed and the plan unchanged.
  The rerun must call the refresh again, install the refreshed plan (the old one kept as
  `.pre-repin`), print the `refreshed` line last and exit 0. A third run must be a plain no-op.
  Before the fix, the rerun printed `nothing to repin` and the refresh ran once in total.
- Fix: the commit's SHA is recorded at `$(git rev-parse --git-path
  dogfood-closeout-repin-pending)` right after the repin commit. The record is emptied only after
  the refreshed plan is installed. A run with nothing to repin and a record naming an ancestor of
  HEAD says it is retrying the refresh, runs it and reports success after it. A record whose
  commit is no longer an ancestor (the repin was dropped) is emptied and nothing runs.
- The refresh-failure `fix:` line now names the rerun that retries the refresh, and the hand
  refresh as the alternative.
- Considered and not taken: detecting a stale plan by comparing it with the committed repin's
  hunks. That needs the plan-to-patch map the bind owns. The pending record is explicit and local
  to the Git directory.

## Specs

- `docs/specs/lane-check-tooling-v0.md` adds `ASD-V0-3730` and `ASD-V0-3731` with their evidence
  rows. `ASD-V0-567` and `ASD-V0-568` each point to the new row.
- `DWC-V0-746` in `docs/specs/daily-change-evidence-workflow-companion-v0.md` says that a resume
  keeps the start record.
- `make spec-requirements` regenerated `REQUIREMENTS.tsv` and the README range. No line citation
  moved.

Verification, the reseal and the batch check are recorded in the lane handback for this round.
