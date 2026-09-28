# HLQ missing-envelope diagnostics — V1-0397

Base: `86fe981154b53f388bec6c4378eeef97649566e9`.
Owning intent: HLQ-V1-002 and HLQ-V1-007 in
`docs/specs/host-lifecycle-qualification-v1.md`.

## Finding and correction

The deterministic received-hook fixture reproduces two losses: `envelopedReceipt`
reports a missing envelope without its additionalContext, and `contextHook` returns
an empty string on receipt-validation errors. Both new regressions fail before the fix.

Missing-envelope errors now carry a Go-quoted prefix of the received text, bounded
at 2048 input bytes with an explicit omitted-byte count. Quotes and control characters
cannot forge report rows or columns. The helper still returns the full received string
separately; contextHook preserves it while adding the hook event to the error. The
case report therefore retains a visible `adapter-host-kill-deadline` fallback cause.
No adapter timeout, deadline or success predicate changes.

The historical upgrade error's received text was not recorded, so its cause remains
UNKNOWN. Reported historical load ranges are retained in the HLQ spec without claiming
causation or a safe threshold. The deterministic watchdog tests establish fallback
behavior, not the cause of that lost event.

## Evidence and remaining work

HLQ units, including fallback, quoting/control characters, truncation and contextHook
propagation, pass. Existing watchdog/degradation/declared-host-kill checks pass; HLQ
vet and regenerated requirement locators pass. Private logs and original context
receipts: `/private/tmp/corvint-bugfix-20260928/v1-0397/`.

The exact regression patch was transferred from the TCP worktree to this independent
HLQ checkout before production edits; the TCP checkout was restored and its enrollment
left intact. This checkout has its own HLQ intent/check enrollment. Pre-production
query and impact receipts were retained after the regression-only edit. A separate
initial no-diff dogfood run was NOT_RUN; final binding and closure remain pending.
The unchanged owned-process wrapper is used with bash because it is not executable.

Three consecutive live Claude qualifications on the frozen committed candidate,
independent review, CEM/OCM and final dogfood closure remain pending at this source
commit. Actual load will be recorded, never manufactured. No cobra cases were opened.
The full repository gate is NOT_RUN under the owner's scoped-check instruction.

Rollback reverts diagnostic rendering and text propagation; it does not reclassify
any failed or unobserved qualification. The separately requested AGENTS.md workflow
edit was rejected by automatic approval review and remains unchanged pending explicit
approval through the coordinator; it is not part of this source change.
