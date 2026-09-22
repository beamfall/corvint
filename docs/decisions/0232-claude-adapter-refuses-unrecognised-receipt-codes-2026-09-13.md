# Decision 0232 — the Claude Code adapter refuses a receipt code outside the recognised set

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`integrations/compatibility.json` sets `receiptDegradationPolicy.onUnrecognised` to `refuse` and
lists `claude-code` among its conforming adapters. The Gemini CLI and OpenCode adapters refuse with
`corvint-degradations-unrecognised`. The Go Claude Code adapter's `claudeNamedCodes` named safe codes
and counted the rest as `+N more`, but never refused. `docs/specs/agent-harness-integration-v0.md`
has no clause exempting the in-process adapter, so the declaration and the code disagreed.

The call (`AHI-019`):

- `claudeReceiptOutput` refuses a receipt whose degradation list holds any entry outside the
  recognised set, on every event, with the fault `corvint-degradations-unrecognised` in a
  `systemMessage`. It is a fault under `AHI-021`, not an expected degradation, so it stays
  user-visible. The message names no code: the offending code is unvalidated text.
- Any subset of the recognised set, including the empty list, is still accepted and rendered as
  before, so core closing a gap is never an adapter break.
- The out-of-root `post-tool` receipt stays silent (`AHI-019`): the check runs only where a receipt
  would otherwise be rendered.

Rejected alternative:

- Record an exemption because the adapter and the kernel ship in the same binary. The kernel emits
  only recognised codes today, but the published policy is a per-adapter contract. An exemption
  would leave a new core code shown as `+N more` or as a name the adapter was never validated
  against, which is the silent surface change the policy forbids.

Scope: the refusal also covers the whole-receipt renderers in `renderAdapterResult`, the Codex
adapter (in its `codexDegraded` fault shape) and the Claude Code dogfood envelope (the same
`systemMessage`), on every event that frames a receipt; `stop` and `session-end` frame none.
