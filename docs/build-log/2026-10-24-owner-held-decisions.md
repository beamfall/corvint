# Owner-held ticket decisions (2026-10-24)

Date: 2026-10-24. The owner delegated the open owner queue to the orchestrator. This entry covers
the 13 tickets held with `owner-decision`. Several were already decided by decisions 0548, 0549 or
0552 and were held again on 2026-10-22 only because a later lane skipped the work as "needs owner
decision". Each entry below is the recorded decision, its reason and what implements it. Each one can
be reversed by a later owner decision that replaces the entry. Measurements this entry does not have
are marked NOT_OBSERVED.

## V1-1702: pool release pays a complete audit

**Decision: decline reaffirmed, build narrowed.** Keep the CAL-V0-264 decline (a5827b719): pool
verbs do not move to the writer route. Meet AC1 through the "equivalent bounded path" clause with
decision 0549 S6b, the cross-process resume of sealed segments (R11). The target is the T3 class:
pool release at 47,000 receipts takes at most 2 s.

**Reason:** decision 0552 already chose this route
(`docs/decisions/0552-owner-delegated-ticket-decisions-2026-10-21.md:98-104`). Decision 0549 F3
shows that R11 re-hashes every sealed segment, so rewritten-prefix detection (PSR-V0-053) is
preserved by bytes (`docs/decisions/0549-tasks-lease-write-performance-design-2026-10-20.md:196-206`).
No new evidence justifies losing that detection, so there is no reversal. S6a landed in 1521b2382;
S6b is still open (`docs/build-log/2026-10-21-journal-1021h-sealed-resume.md`,
`docs/build-log/2026-10-23-tasks-linux-qualification-b3-harness.md`). Pool release time at 47,000
receipts on current main: NOT_OBSERVED. Implemented by a lane in `internal/tasks/journal` and
`internal/tasks/store`, which records the 47,000-receipt measurement as AC evidence.

## V1-1895: complete after a merge-queue merge commit

**Decision: Option B reaffirmed.** `complete` keeps tree equality on the submitted candidate
(CAL-V0-017 is not loosened). Both refusal branches of `completionBlocker` name the post-merge
sequence: submit the merge commit, run the gate, then complete. AC1 changes from "one command" to
"one named sequence".

**Reason:** decision 0548 chose this
(`docs/decisions/0548-pre-1-0-delegated-ticket-dispositions-2026-10-20.md:36`). Accepting a merge
commit whose tree was never gated would bind evidence to content nothing checked (invariant 1). The
work was skipped, not built (`docs/build-log/2026-10-21-taskscli-1021r-refusal-messages.md`). The
current text is wrong for a merge-queue merge commit: `notIntegrated`
(`internal/tasks/transaction/lease_gate.go:457`) says to fetch and complete again, and the
tree-mismatch branch of `completionBlocker` (`internal/tasks/transaction/lease_gate.go:434`) names
no route. Implemented by a lane in `internal/tasks/transaction`, with a refusal-text test per
branch. The orchestrator closes the ticket after that lane merges.

## V1-2030: hook-reported server identity in falsification receipts

**Decision: accepted as decision 0552 framed it.** `ServerIdentity` carries its source, observed or
hook-reported. An identity known only from the hook caps the verdict below complete. An independent
server probe is optional and is not 1.0 work. The legacy wire without isolation stays byte-identical
(BBF-V0-023).

**Reason:** decision 0552 already chose this
(`docs/decisions/0552-owner-delegated-ticket-decisions-2026-10-21.md:122-126`). The field still has
only `server_url` and `application_identity` (`internal/behaviorfalsify/profiles.go:93`), so a
forged hook report would read as observed, which is invented certainty (invariant 2). Implemented
by a lane in `internal/behaviorfalsify`. It adds a requirement row to
`docs/specs/browser-behavior-falsification-v0.md` and a forged-hook test.

## V1-0332: parallel PRs conflict on shared index files

**Decision: accepted, with these bounds.** Add the built-in `merge=union` attribute for
`docs/decisions/README.md` only. The build log is already conflict-free (decision 0423, one file per
entry, 9fedba554). The generated spec files keep `make spec-requirements-resolve` (decision 0543,
SRG-V0-041). Do not use union for any generated or ordered file.

**Reason:** `docs/decisions/README.md` gains new rows at its end, so every parallel decision PR
conflicts there. A union merge keeps both rows, and `decision-numbers-check` fails on a duplicate
or a missing row (V1-0265), so a bad union cannot pass silently. `.gitattributes` today is only
`* -text`, and `script/check-eol-policy.sh` checks only `-text` and CRLF, so adding the attribute
keeps the EOL policy green. The REQUIREMENTS.tsv side was settled by
`docs/build-log/2026-10-18-V1-1525-spec-index-conflicts.md`, and the remaining question was raised in
`docs/build-log/2026-10-21-testprov-1021w-reuse-trace-obligations.md`. Whether GitHub's own merge
honors `merge=union`: NOT_OBSERVED. Local batch merges do honor it, and that is where these
conflicts occur. Implemented by a docs-tooling lane with a script test that merges two branches
which each add a decision row.

## V1-0418: dirty-worktree overlay

**Decision: stays deferred post-1.0.** No change to decision 0552.

**Reason:** it is a new freshness capability (kind FEATURE, milestone post-1-0) that changes evidence
pinning, and current behavior fails closed
(`docs/decisions/0552-owner-delegated-ticket-decisions-2026-10-21.md:174-178`). The all-known-issues
rule covers bugs, fixes, optimizations and tests, not post-1-0 features. The attached evidence is a
suspected stale-base labeling report from the 2026-10-11 audit. That report is a separate suspected
defect: the orchestrator files it as its own `bugs` ticket after deduplicating. No code change.

## V1-0480: shared immutable and editor-overlay evidence join

**Decision: stays deferred post-1.0.** No change to decision 0552.

**Reason:** decision 0373 cut editor qualification from 1.0
(`docs/decisions/0373-corvint-1.0-scope-ratified-2026-09-23.md`), and decision 0552 applied that cut
to this LSP chain (`docs/decisions/0552-owner-delegated-ticket-decisions-2026-10-21.md:192-196`).
Its parent V1-0478 is still open post-1-0, with LQP-V0 profiles not yet accepted. No code change.

## V1-0953: staged begin after a satisfied stage

**Decision: already delivered, close with 699a59187.** Same-clone supersession of a satisfied stage
is rejected for 1.0.

**Reason:** the supersession fix e56d9808b was reverted by 1649aa2c4 because it could archive
invalidated evidence (`docs/build-log/2026-10-21-v1-0953-staged-begin-reverted.md`). 699a59187 then
met the ticket's documented-refusal branch. The `prior-completion-stale` refusal names the staged
route (`cmd/corvint/local_completion.go:285`), and `docs/DOGFOOD.md:1110` documents it. Both tests
cover it: `internal/localcompletion/lifecycle_test.go:924` and
`cmd/corvint/local_completion_test.go:602`. AC1 to AC3 are met. Closes as decided, with no further
code change.

## V1-1003: MCP impact and context budgets

**Decision: accepted, with these bounds.**
- MCP `impact` gains `budgetBytes`, with the same semantics and refusals as the CLI path-impact
  `--budget-bytes` (`cmd/corvint/diagnostic_refusals.go:92`).
- MCP `context` gains `summaryBytes`, with the same semantics as `--summary-bytes`
  (`cmd/corvint/context_summary.go:45`). There is no new budget on the context packet. AC1 is
  amended to match.
- Omissions caused by `maxEvidence` are counted separately from budget omissions, each with its
  own reason text. If the analyzer pin on `impact.go` requires it, the pin is bumped.
- Rejected: deduplicating schemas across tools in `tools/list`.

**Reason:** a shared external `$ref` breaks offline validators, while the duplicated schema costs
about 600 B once per listing (the ticket's figure; this entry has no measurement of its own). The
lanes that skipped this ticket recorded the `maxEvidence` and `$ref` constraints
(`docs/build-log/2026-10-21-index-1021h-query-miss-notice.md`,
`docs/build-log/2026-10-21-affected-src-1021g-source-golist-search.md`). Reusing the CLI semantics
adds no new budget contract next to MCPV0-031. Implemented by a lane in the MCP server and the
contextindex impact code, which records the listing's size before and after.

## V1-1091: btime jitter in cross-process identity

**Decision: decisions 0548 and 0552 reaffirmed.** Move the pool's tolerant comparison
(`compareLinuxIdentity`, `internal/tasks/store/pool_process_unix.go:101`) into
`internal/tasks/supervisor`. Use it at every remaining cross-process identity comparison: a 1 s btime
difference counts as the same boot only when the pid and the exact start ticks also match. There is
no format migration, and boot_id stays post-1.0.

**Reason:** the decisions are recorded at
`docs/decisions/0548-pre-1-0-delegated-ticket-dispositions-2026-10-20.md:65` and
`docs/decisions/0552-owner-delegated-ticket-decisions-2026-10-21.md:82-86`. Two pieces are built:
the in-process cache (05fa42d95, `internal/tasks/supervisor/identity_linux.go:53`) and the pool-only
tolerance (1ffbb44c4). `docs/build-log/2026-10-23-tasks-lane-1023.md` scopes that tolerance to pool
records only. The remaining comparison sites are listed in
`docs/build-log/2026-10-20-v1-1091-btime-jitter.md`. The AC1 evidence is a measurement on the WSL2
Linux host plus CI's Linux run of the tolerance test. How often GitHub runners show the jitter:
NOT_OBSERVED unless CI logs record it. Implemented by a lane in `internal/tasks` (the supervisor and
its call sites).

## V1-1478: JS/TS registry-reader shapes that may fail open

**Decision: BUILD-NOW reaffirmed, as decision 0552 framed it.**
1. Resolve an accessor inside `${}` in `registryCallSites`, with a setup-select fixture and a
   TSA-V0 trace row.
2. Widen writes that happen after the declaration, with a named reason.
3. Widen an eager `new X(args)`, with a named reason.
4. Record the NGI-V0-008 amendment (`docs/specs/non-go-impact-v0.md:98`).

**Reason:** the decision is recorded at
`docs/decisions/0552-owner-delegated-ticket-decisions-2026-10-21.md:90-96`. No commit has touched
`internal/contextindex/registry_dialect.go` since then, so the work is not delivered. Widening with a
named reason keeps uncertainty visible instead of failing open (invariant 2). Implemented by a
js-analysis lane in `internal/contextindex/registry_dialect.go`.

## V1-1638: resealing after merging main moves citation ordinals

**Decision: accepted, with these bounds.**
- When the citation map meets a hunk that belongs to a sealed CEM admitted under DCW-V0-042
  (`admitSealedCEMs`, `internal/dogfoodflow/change.go:206`), it records the hunk as cited by that
  verified admission, not as unknown.
- `citationPlanMismatch` (`internal/dogfoodflow/change.go:1371`) and `ordinalsMoved`
  (`internal/dogfoodflow/change.go:1444`) can find that the only ordinal shifts come from inserted,
  admitted sealed-CEM hunks. In that case they remap the plan's ordinals through the hunk IDs
  recorded at first cite and print a NOTE, instead of refusing.
- Any other shift still refuses, and the plan file is never rewritten.

**Reason:** admitted sealed CEMs are already re-verified by bytes, so their hunks carry pinned
evidence (invariant 1). Refusing a reseal only because merged-in sealed hunks moved the ordinals
costs a full re-cite and adds no detection. Every other shift still refuses, so evidence a plan never
cited is not adopted. The open design question was recorded in
`docs/build-log/2026-10-21-dogfood-flow-1021t-propose-citations.md`. Implemented by a lane in
`internal/dogfoodflow`, with a test that merges main and then reseals.

## V1-2069: a projection path named by a selected reader keeps the plan UNKNOWN

**Decision: accepted as an AFP-V0-088 (2) amendment.** Consider a `.taskman/` projection path whose
namers (path literals or declared read scopes) are all already selected. That path moves to the
non-source `TASK_STORE_PROJECTION` class, with no `UNOWNED_DIRTY_PATH`. Every one of those readers
stays selected, and literal matching does not change.

**Reason:** clause (3) already trusts a projection path that no reader names
(`docs/specs/affected-plan-v0.md:1986`; rationale in
`docs/build-log/2026-10-20-V1-2049-affected-taskman-projection.md`). Finding and selecting the reader
should not make the plan worse than finding none. Decision 0548 made project authority, not a
syntax heuristic, bound the reader
(`docs/decisions/0548-pre-1-0-delegated-ticket-dispositions-2026-10-20.md:61`), and the declared
scope (`.corvint/test-read-scopes.json:388`, b06d0bc4c) is that authority. The five path-literal
readers that remain (`docs/build-log/2026-10-21-testscopes-1021w-confinable-test-readers.md`) stay
selected. Earlier work did not meet the acceptance criteria
(`docs/build-log/2026-10-21-design-1021-documentation-and-scoped-readers.md`). The fast-tier NOTHING
plan (AFP-V0-153) is unaffected because the selection is not empty. The remaining risk is a run-time
root that no literal names (AFP-V0-072). Implemented by a lane in `internal/liveverify/affected`,
with the spec amendment in `docs/specs/affected-plan-v0.md`.

## V1-0910: a stat cache for the intent tree digest

**Decision: rejected.** Decision 0549 F1 stands: no ctime-keyed stat cache.

**Reason:** the cache would treat file stamps as evidence, which CAL-V0-116 declines
(`docs/decisions/0549-tasks-lease-write-performance-design-2026-10-20.md:187-192`). Liveness writes
already skip the intent tree (JSG-V0-001, 12c06de28), and JSG S3 already uses exact listings
(356c96747). The remaining full `TreeDigest` cost is 119 ms at 3,000 tickets
(`docs/decisions/0549-tasks-lease-write-performance-design-2026-10-20.md:40`), well inside T3's 2 s,
so no 1.0 target is missed and the perf-before-1.0 rule is met. A per-write cost at the store's
current 2,629 tickets: NOT_OBSERVED. The lane notes that skipped this ticket are in
`docs/build-log/2026-10-21-storeperf-1021u-lease-commit-staged-checkpoint.md`. Closes as decided,
with no code change.
