# 2026-10-20 V1-0812: PR narrowing decisions (AFP-V0-097 to AFP-V0-099)

Ticket: V1-0812 (narrow PR race tests behind the merge queue). Spec: `AFP-V0-029`, `AFP-V0-030`
(amended) and `AFP-V0-097` to `AFP-V0-099` (new) in `docs/specs/affected-plan-v0.md`. The three
forks recorded in `2026-10-20-v1-0812-whole-epoch-shadow-bar.md` were delegated by the owner
("you decide the narrowing questions", owner chat, 2026-10-20) and decided by the orchestrator
under that delegation. Narrowing stays off: nothing here sets `CORVINT_PR_NARROWING`.

## Decisions

1. **D1, whole-epoch evidence (`AFP-V0-097`).** A trusted workflow,
   `.github/workflows/ci-forward-bar.yml`, runs every 6 hours and on dispatch, from `main` only,
   with `actions: read` and `contents: read`. It lists the whole epoch (up to 400 pages) once and
   retains `afp-shadow-bar-epoch-<N>` with an `afp-shadow-bar-verdict/0` document. A pull
   request's `narrowing-verdict` job runs `accept.sh` from the PR's base commit and narrows only
   when the newest such artifact by `created_at`:
   - (a) came from a schedule or dispatch run of that workflow path on `main` of this repository;
   - (b) says `MET` with a clear kill switch;
   - (c) is at most 7 days old and unexpired;
   - (d) names the current epoch.

   It then re-reads the newest 5 pages of row names live, so a `MISS` or a tripped kill switch
   since the verdict still forces FULL. Anything else runs full CI with the reason in the summary.
   *Rationale:* one bounded trusted computation replaces per-PR paging of the whole epoch, and
   the verdict cannot be minted by a PR or fork run.
2. **D2, selector identity (`AFP-V0-098`).** The driver gains `--mode plan`, which writes the
   selection and executes nothing. A shard narrows only when that selection is a superset of the
   `corvint test --dry-run` shadow selection for the same base and tested merge, so every package
   the epoch's rows bound to S is still run at PR time. Every other unsafe input (non-`main` base,
   `.github/` change, no merge-queue rule, driver fallback, shadow incompleteness, identity
   mismatch) also forces FULL, and each reason is retained in `corvint-pr-narrowing/0`.
   *Rationale:* keeps the epoch-1 rows valid without advancing the epoch.
3. **D3, rows of narrowed runs (`AFP-V0-099`).** A narrowed PR run's row is class `NARROWED`,
   outside the bar and the window, so it neither inflates `CLEAN` nor trips the switch as
   `INCOMPLETE`. `merge_group` always runs full: a narrowed shard writes no tested-tree record,
   so the queue cannot reuse it. The collector downloads the selection run's `ci-narrowing-*`
   records, and a queue failure of a narrowed-out package is a `MISS`, even when its rerun passes.

## Failure modes

- A stale verdict (up to 7 days) is bounded by the live 5-page kill-switch read.
- An epoch advance leaves no verdict for the new epoch: every PR runs full until the schedule
  computes one.
- A narrowed-out flaky failure is a `MISS` and can trip the switch: deliberate, since the PR did
  not test it.
- A record saying `NARROWED` while the shard ran whole (filter mismatch) is still read as
  narrowed: conservative.
- Rows and records expire with artifact retention (90 days), so an old `MISS` can drop out of a
  later verdict; re-arming by epoch (`AFP-V0-030`) bounds this.
- Activation needs `CORVINT_PR_TOOL_SOURCE` to pin a commit carrying `narrow.sh` and the plan
  mode; until then the gate cannot run and every shard is full.
- Hosted behavior, fork visibility of `vars`, and the `GITHUB_TOKEN` budget of a 400-page verdict
  are `NOT_OBSERVED`.

## Evidence

`tools/ci-forward-row/narrowing_test.go`: `TestAFPV0097VerdictWrittenOnMain`,
`TestAFPV0097VerdictAcceptance` (each of (a) to (d) failing gives FULL),
`TestAFPV0097WorkflowBindings`, `TestAFPV0098SupersetGuard`, `TestAFPV0099NarrowedRows`,
`TestAFPV0099CollectorFetchesNarrowing`, `TestAFPV0099BarExcludesNarrowedRows`;
`TestAFPV0098PlanModeExecutesNothing` in `tools/corvint-pr-tests`. Local only.

## Rollback

Unset (or set to anything but `on`) the repository variable `CORVINT_PR_NARROWING`: every PR is
full at once. Or revert the change; the verdict workflow and records are then inert.
