# 2026-11-02 — merge-queue narrowing verdict cadence and cache prune (V1-4360)

Owner report: the merge queue "seems pretty broken and slow" and ran every test.

Finding: queue run 41576741930 (#70, a one-line comment change) printed `AFP-V0-097 narrowing
verdict: FULL (the newest 25 artifact page(s) do not reach 2026-11-02T23:58:13Z, an hour before the
verdict's listing)`. The repository held 8,102 artifacts. At that rate 25 pages of 100 covered about
3.4 hours, but the `17 */6 * * *` ci-forward-bar schedule ran 8-9 hours apart (00:58, 15:53, 06:04).
For most of each cycle, every pull request and merge-queue entry therefore ran all 16 shards in full
(10-15 minutes each). Separately, CodeQL default setup kept one
`codeql-overlay-base-database-*` entry per language for each main push (the Go entry is about
425 MB) and restores only the newest. That filled the 10 GB cache and evicted go-build.

Decision (orchestrator, under owner delegation):
- ci-forward-bar runs hourly (about 30 s and at most 400 API pages per run).
- `ci.yml` passes `KILL_PAGES` 60 (about 8 hours) to accept.sh, so a delayed schedule still
  covers its listing. A fresh verdict still stops after a few pages.
- accept.sh keeps its pinned default of 25.
- A new `cache-prune.yml` (AFP-V0-2185) holds the only `actions: write` grant. It deletes superseded
  overlay entries hourly, which keeps ci-forward-bar read-only.
- I deleted the 52 superseded overlay entries by hand with the owner's permission ("you have my
  permission to fix the caching").

Evidence: `TestAFPV0097WorkflowBindings` and `TestAFPV02185CachePruneKeepsNewestOverlay` pass. The
least-privilege check and actionlint are clean. Hosted narrowing after merge is `NOT_OBSERVED` until
a dispatched verdict and a later queue run show NARROWED shards.

Rollback: restore the 6-hourly cron, drop `KILL_PAGES`, and delete `cache-prune.yml` and its exception row.
