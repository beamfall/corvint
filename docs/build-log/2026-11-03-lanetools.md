# Lane verification tooling measurements and local evidence

Owner intent: the 2026-11-03 lanetools charter, V1-4046, V1-4252, V1-3570 and
V1-4319. Continuation and measurements finished on 2026-11-04 (WSL2/Linux,
Ryzen 9 5950X, 32 logical CPUs, Go 1.27.3). Independent review belongs to the
orchestrator. This record does not close the Tasks tickets or qualify a remote default.

## Contract and decisions

`LVT-V0-001..010` live in the registered Lane Verification Tooling companion. The
workflow parent has 256 obligations and self-development has 254; the charter explicitly
permits a new-prefix companion at capacity. Existing requirement IDs are unchanged.
The companion remains experimental, with standing acceptance recorded per requirement.

- V1-4046 (`LVT-V0-001..004`): canonical four-digit ranges, local/remote open-ref
  checks, conservative missing/malformed-index refusal, shared owner reservations,
  atomic replacement and exclusive locking. The existing reservation owned by this lane
  refuses a check without OWNER and passes with OWNER=lanetools; no Tasks mutation is involved.
- V1-4252 (`LVT-V0-005..007`): staged temp/Mkfifo scans use one blob batch;
  error-code discovery uses staged scratch; line citations combine index classes in one
  tagged enumeration and share the blob process for historical citations. Each of those
  four ordinary scans uses three Git processes. Host versions retain separate per-path
  history traversal and first-parent comparisons, batch immutable content, and fall back
  to the original algorithm when the batch is unusable. Its absence filter enumerates
  all historical paths with NUL framing, including deleted and unusual filenames.
- V1-3570 (`LVT-V0-008`): retain the remote GOMAXPROCS default and override precedence.
  Instead shorten untimed live-budget candidate discovery with at most four workers.
  Every original candidate still runs full Select, and results retain original ordering
  before fanout sorting. Timed selection, 32 rounds, calibration and budget are unchanged.
- V1-4319 (`LVT-V0-009`): remove owned host scratch on exit/INT/TERM/HUP and recover
  marked private scratch older than 24 hours only when its owner is demonstrably absent.
  Preserve live, ambiguous, symlinked, foreign, malformed and unmarked scratch. Existing
  unmarked leaks require operator inspection. SIGKILL recovery uses the next sweep.
- Owner friction (`LVT-V0-010`): explicit CORVINT_BIN static precheck, path-bearing
  prechange stderr and a cheap worktree `make spec-claims-check` before staging/generation.

## Git process measurements

The same baseline checkout/index at each real first-parent origin/main revision ran each
baseline and updated script sequentially. Scripts ran from untracked alternate filenames
under script/, so changing the executable did not itself add an unstaged-citation note.
`GIT_TRACE2_EVENT` counts top-level Git `start` events, including fixture Git commands.
Every row has equal exit status (zero) and byte-identical combined stdout/stderr. Retained
source SHA-256, output SHA-256, loads and complete pins are in
`.agent-evidence/lanetools/measurements-final.json`. Retained Go logs trim trailing
whitespace; measured-output digests always name the original bytes. Reproduce from the repository with
`python3 .agent-evidence/lanetools/measure.txt /tmp/lanetools-measure-fresh`.

| Revision | Check | Git starts before → after | Seconds before → after | Output |
|---|---|---|---|---|
| f13f52b16 | check-host-package-versions.sh | 41 → 17 | 0.250 → 0.556 | identical |
| f13f52b16 | check-fixed-tmp-paths.sh | 117 → 3 | 0.815 → 0.279 | identical |
| f13f52b16 | check-mkfifo-build-tags.sh | 103 → 3 | 0.628 → 0.272 | identical |
| f13f52b16 | check-error-code-ownership.sh | 99 → 3 | 3.671 → 3.808 | identical |
| f13f52b16 | check-line-citations.sh | 68 → 3 | 1.511 → 1.068 | identical |
| 9fdc2f02e | check-host-package-versions.sh | 41 → 17 | 0.285 → 0.465 | identical |
| 9fdc2f02e | check-fixed-tmp-paths.sh | 117 → 3 | 0.687 → 0.245 | identical |
| 9fdc2f02e | check-mkfifo-build-tags.sh | 103 → 3 | 0.650 → 0.215 | identical |
| 9fdc2f02e | check-error-code-ownership.sh | 99 → 3 | 2.953 → 3.644 | identical |
| 9fdc2f02e | check-line-citations.sh | 68 → 3 | 1.025 → 1.021 | identical |
| 6f12f4704 | check-host-package-versions.sh | 41 → 17 | 0.276 → 0.428 | identical |
| 6f12f4704 | check-fixed-tmp-paths.sh | 117 → 3 | 0.698 → 0.247 | identical |
| 6f12f4704 | check-mkfifo-build-tags.sh | 103 → 3 | 0.651 → 0.214 | identical |
| 6f12f4704 | check-error-code-ownership.sh | 99 → 3 | 2.664 → 2.958 | identical |
| 6f12f4704 | check-line-citations.sh | 68 → 3 | 1.039 → 0.922 | identical |

The original host-version fixture suite on the same base dropped from 894 to 429 Git
starts (52.0%), with identical output and all original controls passing. Its wall time
regressed from 2.874 to 5.442 seconds. Some individual checks also regress in wall time;
these measurements establish process reduction, not universal latency improvement.
Loads and source digests are retained; Windows desktop load was NOT_OBSERVED, so none
of these are quiet-host qualification numbers. A complete cold batch-precheck run was
NOT_RUN: its parallel background scheduling and combined-package Go commands conflict
with this lane's explicit execution limits. No precheck receipt or CI speed claim is made.

## Go measurements and soundness controls

Same-build alternating solo samples of cem/verify, GOMAXPROCS=4 versus unset, took
8.626/6.070 versus 8.040/6.536 seconds through heavy.sh. The live-budget test took
35.78 versus 35.81 seconds (package times 35.796/35.823). Loads were 1.17–5.52;
these are local shape evidence, not remote or quiet/concurrent qualification.

After bounded parallel discovery the same live-budget test took 19.15 seconds. The
same-build complete serial/parallel discovery benchmark took 24.174910396 versus
7.218870121 seconds and proved exactly equal representative paths. It still measured
160 timed selections and reached the same widest plan of 248 units. An injected fault
that zeroed only parallel fanout counts was selected through the same package and
made the parity benchmark fail; the source was restored. Original and final logs,
command argv and expected-failure receipt are retained under `.agent-evidence/lanetools/`.
The rejected reach-count-only experiment preserved full-plan parity but barely reduced
runtime (34.71 seconds), so it was not retained.

Focused shell controls preserve staged verdicts under unstaged repairs; inject unsafe
/tmp writes, bad Mkfifo constraints, unowned codes, malformed/reserved IDs, unavailable
Git batches and long Claims. Line-citation tests cover immutable decisions, same-change
stale pins, unmerged stages and modified/untracked diagnostics, with three Git starts
for both one and 41 historical decisions. Pi fixtures use IPC readiness and a temporary
sweep root; they exercise normal exit, all three signals and recovery after SIGKILL.
The parallel discovery race check passed (78.480 seconds), and its injected fanout fault
failed as expected before restoration. The lane-script tools/workflow/offload groups
passed (38 groups). Focused prechange
Go tests passed, including the missing-path diagnostic.

## Continuation controls

The completed next-block query includes base tokens, open-ref definitions and reservations,
with owner-only exclusions. Its fixtures also refuse missing/malformed bases and ledgers and
prove read-only ledger bytes. Allocation now includes IDs defined only on the caller's committed
head; peer collision reporting still excludes it. Failed worktree/root enumeration and empty or
truncated successful Git batch output refuse allocation. Free adjacent IDs remain selectable.
The corresponding negative controls and injected failures pass in the reservation/collision suites.

Two separate executions observed different Linux PID namespaces (`pid:[4026532300]` and
`pid:[4026532309]`). An absent PID in one namespace cannot prove an owner absent from the other.
The scratch sweep now requires the marker's namespace to match on Linux; unreadable, missing or
mismatched namespace identity preserves the directory. Injected absent and mismatched identities
failed the scratch suite against the old implementation (exit 1); the guard restores its pass
while the same-namespace dead-owner control still removes stale scratch. Older Linux markers
without a namespace require operator inspection.

The final focused shell/Pi checks and all 38 tools/workflow/offload groups passed again.
Focused prechange Go tests passed in 0.437 seconds. The repeated live-budget plus parity command
passed in 60.567 seconds; serial/parallel discovery measured 29.021420246/7.675511824 seconds.
These are verification runs, not new quiet-host performance qualification. The retained earlier
measurement scripts and Go source bindings were checked byte-for-byte against their source
hashes and immutable trees. Commands and delivered source hashes are recorded in
`.agent-evidence/lanetools/completion-checks.json`.

The local Tasks projection cannot read these four tickets (`journal-absent`); the prior read
receipts' `items[0].record` are retained in `/tmp/lanetools-ticket-V1-*.json`. Ticket completion,
independent review, publication and sealing remain orchestrator-owned by the lane charter.

## Retained limits and rollback

Five-concurrent remote qualification remains NOT_PRODUCED under the no-background-job
charter; no remote host is configured here. Native Pi host qualification is NOT_RUN
because Pi 0.99.1 is absent. Unit scratch cleanup does not qualify the complete host
matrix, other operating systems or unmarked legacy cleanup. Independent review is
NOT_PRODUCED here by explicit owner instruction; the orchestrator owns it. Tasks writes,
publication, model review, make gate and seal are NOT_RUN by instruction.

Revert the script changes independently; release only explicitly owned reservation rows.
Revert parallel discovery to its serial mode if parity or race checks fail. No product
selection rule, enforced performance threshold or remote default was narrowed.
