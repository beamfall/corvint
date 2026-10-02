# Explicit dispatcher progress: proposed contract

Issue 468 requests explicit progress independently of work-state role matching. Gate A revision 1
accepted the bounded opaque command-token plan at proposal level; it did not admit implementation.
The proposed CAL-V0-064 contract retains replay history, lifetime caps, checked admission/accounting,
strict grammar/load validation, pending UNKNOWN accounting and fail-closed downgrade. Tokens remain
producer assertions, never proof of referenced work. S12 CAL059..061 and reserved S13 CAL062..063
remain separate. The exact reservation proof is integrate456 commit 14eaf5e251691976b6cc81421c9d07a44c701dbe,
CAL spec blob 74aff5bd11116bab050dfb7731b5fdbb232fd0fb.

At public base 1fda1b94984245d0cd0ac0a6d17cc72572ce6619, private controls demonstrate that object output
is rejected by the old reader and ordinary file changes alone leave its fingerprint unchanged.
These are local source observations; the historical INV-1 report is not fresh production qualification.
The genuine initial dogfood pass ran before tracked edits and refused empty BASE..HEAD: CEM prepare
reported git-diff-failed, CEM/OCM remained NOT_PRODUCED, local outcome no-source-paths. Its original
prechange agent-receipt-absent notes are retained; actual prechange receipts were independently
captured and preserved. No implementation test, final CEM, CI or ticket completion is claimed.

Remaining work includes implementation, real explicit-signal replay, focused checks, independent
implementation review, required docs gate, clean binding/seal, integration and supported native
completion. Rollback never restores pre-feature ledger history over later worker/accounting activity.
