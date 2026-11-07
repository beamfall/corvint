# Dogfood rebind, refresh, external review and cumulative handbacks

SPDX-License-Identifier: Apache-2.0

Owner intent: V1-4545, V1-4546, V1-4548 and V1-4541, read in full from the
host's read-only queue snapshot on 2026-11-04. Requirements DWC-V0-044..047
are accepted under the owner's standing auto-accept (2026-10-11), without an
owner read-back claim. The original workflow spec is full (256 obligations),
so this slice extends its existing DWC companion. Origin/main's next free
DWC ID was 004; this lane leaves the requested 40-ID gap and starts at 044.

## Decisions and boundaries

- V1-4545 / DWC-V0-044: a successful standalone prepare/cite that dirties only
  an already committed CEM stops before OCM, status and local-outcome recording.
  It retains an incomplete report and prints one commit-and-rerun remediation.
  Nested finish-lane passes retain their existing commit coordinator. A different
  dirty path or preceding failure keeps the full diagnostics.
- V1-4546 / DWC-V0-045: matched refresh rows still carry and repin; unmatched
  hunks use ordinary content-based drafting. Same-file membership cannot justify
  copying the first carried citation. This explicitly amends DCW-V0-842's
  carried-forward branch; its cross-mainline repin behavior remains covered.
- V1-4548 / DWC-V0-046: use DOGFOOD_REVIEW with an external report containing
  exactly one `dogfood-review: reviewed-head=FULL_SHA verdict=PASS` header.
  External reports require exact pre-seal HEAD, while the existing pinned
  reviewer-output format retains its metadata-only descendant allowance.
  Native and shell paths record report path, digest, head and verdict in output
  and the seal commit message, keeping the tree change rename-only. The wrapper
  hashes and validates one bounded snapshot. A head movement during checking
  refuses sealing. No report stays NOT_OBSERVED; supplied evidence authenticates
  neither reviewer independence nor correctness. No reviewer was launched here.
- V1-4541 / DWC-V0-047: the template/lint helper requires distinct base, fix,
  bind and sealed heads plus delivered/superseded tickets and GitHub issues
  closed. A review round carries cumulative tickets from its prior handback;
  lint refuses losing them. Delegated bind/seal steps require explicit
  NOT_PRODUCED reasons. This helper performs no ticket or issue mutations.

Rollback: remove the standalone rebind shortcut, restore the carried-forward
proposal branch, remove external-review parsing/commit evidence, and remove the
handback helper and references, respectively. Ordinary CEM/check semantics and
existing legacy review admission remain the baseline.

## Evidence and retained limits

Focused regressions `TestDWCV0044RebindStopsBeforeCascadingFailures`,
`TestDWCV0045RefreshDraftsNewHunkFromItsOwnContent`,
`TestDWCV0046ExternalReviewSeal`, `TestDWCV0047HandbackCumulativeTickets`, and
`TestProposeCitationsDCWV0842RefreshAcrossAMainlineMerge` passed through
script/heavy.sh, one package per invocation. Review refusal cases include a
wrong head, failed verdict, duplicate/malformed header, missing/nonregular or
oversized report and head movement during checking. Fixture seals retain
OBSERVED in the commit message and change only the CEM's path.
The shell `make review-head-check-test` regression passed, including an actual
fixture seal. Focused cmd/corvint use-case and help checks passed. Logs and the
exact affected plan are retained in the lane's external evidence directory;
the final handback names the implementation and bind heads and final check.

Self-development routes used: dogfood change-start, bound prechange context,
query and impact, affected/test dry-run selection, proposal inspection and the
final reviewer-facing CEM report. Optional LSP, corpus, remote provider, browser
falsification and learning routes are not applicable to these workflow fixes.
There is no new wire profile or changed frozen CEM format. Frozen evaluations,
repository-wide tests/vet and live third-party qualification are NOT_RUN: this
is a scoped lane using focused verification, not a release qualification.
Independent review is NOT_RUN here: the orchestrator owns it. Native ticket
completion, publication and sealing are NOT_RUN under the lane charter.

Observed process friction: plain ticket reads lack the host queue journal and
need the advertised CORVINT_TASKS_SNAPSHOT; prechange impact refuses directory
or Markdown paths and succeeds with the two concrete Go files; the host heavy
configuration selects an unwritable /home/russell/bench/rt/.locks, so this run
uses /home/russell/lanes/.heavy-locks. One capability-catalog-check invocation
expanded to direct multi-package Go tests, contrary to the lane rule; its
focused checks were then rerun one package at a time through script/heavy.sh.
The handback retains the exact commands, failed attempts and coverage unknowns.
No token-saving or independent outcome claim is made.

## Observed final-check blocker: separate sandbox mounts

The first real final check reached verifier agreement but refused dogfood-report-write.
Both paths were writable. A bounded rename probe confirmed errno 18 EXDEV from
`/home/russell/projects/corvint/.git/worktrees/cx-dogfood/corvint/` to the worktree's
`.corvint/` despite matching device numbers. DWC-V0-048 records the necessary integration
repair: keep the existing Git-directory stage, and only on EXDEV publish the same checked
bytes through a private destination-directory temporary file. All temporary files are
removed and an unsuccessful publication leaves the old report intact. Other failures
still refuse. `TestDWCV0048ReportPublicationAcrossMounts` covers success and a refused
second rename without truncation. This fixes the observed lane blocker; it grants no
verification exception and does not change CEM wire semantics.

The repair rebind also exercised V1-4545 live: input preflight had already rewritten the
committed CEM, so coordination-time impact refused its clean-worktree requirement before
the rebind shortcut. The shortcut now admits that one exact refusal when only the CEM is
dirty, retaining it in the incomplete report. All unrelated prior failures still keep their
diagnostics. The focused regression now reproduces this refusal and requires exactly one
printed remediation line, not only absence of OCM/status calls.
