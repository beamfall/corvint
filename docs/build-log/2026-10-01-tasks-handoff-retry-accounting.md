# Prospective retry accounting for clean handoffs — issue 412

Owner intent: Russell Lewis approved prospective handoffs on 2026-09-30 after the
bounded Gate A proposal. Ticket V1-0566 / GitHub #412. Implementation base is public
`9afd8313edad658d6c40b1abe454fe7417eb6cad`; prior Gate A at `25971bda` remains applicable
after inspection of the intervening CLI reservation-read and CAL-V0-036 context changes.

## Decision

CAL-V0-013 now delegates clean handoffs to CAL-V0-044. HANDOFF and REVIEW_RETURNED
request a writer check; reason strings and review-stage claims alone give no exemption.
The optional journal-bound accounting object retains a non-PASSED gate observation for
the entire generation. A later PASS or resubmitted candidate cannot clear it. A clean
handoff retains accumulated retry debt, including at three. Claim, preview and exhausted
OPEN owner recovery share that decision; CAL-V0-043's owner and safety fences stay intact.
Supervised attempts may retain inert metadata after attachment but receive no exemption.

No retrospective refund, counter reset, live queue migration, changed retry limit or
automatic reopen is included. The installed Tasks build202 remains unchanged. Upgraded
writes in qualification use disposable stores only. Metadata begins at the first new
claim; typed old readers refuse it. The old raw `attempt show` view can still display it,
so the compatibility witness uses decoding `plan preview`. Rollback after an accounting
write requires a compatible reader/writer and preserved journal, with admissions stopped.

## Evidence and limits

Corvint supplied the pre-change context with omitted results and withheld test candidates;
original immutable source supplied the accounting evidence. Baseline Go overlay tests
on the admitted public base reproduced exhaustion after staged clean releases and passed
the changed upstream context tests. The initial mandatory dogfood run at base equal to
target returned `NOT_PRODUCED: git-diff-failed` for CEM preparation, absent CEM/OCM and
`no-source-paths` for the local outcome. Coordination query and impact were produced;
agent-receipt discovery remained NOT_OBSERVED. No reserved shared CEM was written.

Focused CAL-V0-044 tests pass for clean debt0/2/3 transitions, ordinary failure and review
expiry, wrong-stage/missing-candidate refusals, FAIL-to-PASS replacement, timeout followed
by PASS/resubmit, legacy absence, schema rejection, request replay/conflict, and pure
preview/claim agreement at count three. Their source assertions are in the requirement's
test table. Initial independent review found no production correctness defect; two missing
coverage cases (legacy classification and preview at clean count three) were added, along
with timeout coverage. Dedicated handoff race/fault/pool tests are not claimed; existing
scope-wide concurrency, redo and quarantine tests exercise the unchanged machinery.

Compiled disposable CLI qualification passed, including clean count-three continuation and
old202 planner refusal at the first accounting-bearing claim without mutation. The final
scoped tests/vet and evidence checks are recorded against their exact targets in the local
dogfood workflow; native integration/completion remain separate closeout obligations.
The checkout was restored after external archival; all task source hashes matched the saved
checkpoint. Lost private enrollment is re-established from the same frozen plan and base.
No repository-wide gate or production migration is claimed. Recorded eligibility does not
prove actor authentication, unreported external failures, review independence or physical
quiescence. Missing runtime and cost telemetry remains NOT_OBSERVED.

## Public integration repair

Hosted PR 414 shard 3 exposed a missed wire-package check: the runtime had the two
accepted handoff codes, while `TestTMV0002_AS01_CommandResultEnvelope` still expected
TCP-00's original 69. A17 now records the closed-code extension explicitly and that
existing assertion checks 71. Accounting behavior is unchanged.

The final integration base is public `01f557cb47272d44c1316cc58d2b44529661756f`.
A normal merge preserves the published `d9800f8d` ancestry. The prior sealed map is
retained byte-for-byte as a local artifact and in that history, with SHA-256
`ca30a0faecaf6dcfe88a3047feed8fb9057a2aaec6753eedeb61534d8877b5df`.
It is retired from the new candidate so the public-base diff can receive one new
binding. The initial strict `sealed-cem-in-change` refusal is retained. Public-main
archives remain baseline content. A separate integration clone and enrollment
preserve the original completed workflow. The final-base pre-change query was
produced; path impact refused `unsupported-impact-path-suffix` for the requirement
TSV. That refusal and the range affected-plan fallback are retained. Final verification includes the wire
and spec-index packages, unchanged disposable compiled CLI inputs, a narrow review
and the required final evidence checks. Native closeout remains coordinator-owned.
