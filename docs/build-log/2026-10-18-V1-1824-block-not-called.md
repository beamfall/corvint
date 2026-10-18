# V1-1824: BLOCK-NOT-CALLED test-validity rule for RSpec block helpers

Owner input: [issue 1140](https://github.com/beamfall/corvint/issues/1140). A spec helper that
takes a block and hands an assertion a block that only references it (`assert_no_enqueued_jobs do
block end`) makes every example that uses it vacuous. TSA-V0-053..056 were added to
`docs/specs/test-source-analysis-v0.md`, after TSA-V0-052 on origin/main; no open PR held a TSA-V0
ID. No CAL-V0 ID was added. The requirements are accepted under the owner's standing auto-accept
of lane requirements (2026-10-11). Delivery stays experimental, like the rest of TSA-V0.

## Design

- **Same reader, same vocabulary.** The rule extends the RSpec reader behind
  `MEMOIZED-BLOCK-EXPECTATION`. It records each `def` directly in an example group or at file
  level, and reuses the reader's balance, depth and size abstention (TSA-V0-050). The rule ID
  and the `block-not-called` hygiene reason sit beside the existing ones in
  `internal/testvalidity/source_rules.go`.
- **Conservative by construction.** A block helper is reported only when every use of the block
  parameter is a bare reference that is the whole body of a literal block, or when the block is
  never used. `yield`, `super`, `binding`, `&block` forwarding and any use the reader cannot
  classify count as calls. A bare reference at the def's own level may be the return value, so it
  also counts as a call. A def in a class or module, an anonymous `&` or `...` parameter and a
  nested def are not read.
- **Callers inherit.** An example inherits the finding only when it passes the helper a block and
  its group chain resolves the name to that def, so an override in a nested group, or a call
  without a block, is not reported.

## Residuals

- A helper that calls its block on some paths but not others is not reported (no path-sensitive
  reading).
- Helpers in `spec/support` or another file are not read; only same-file defs are.
- No CLI, preflight or run-receipt surface reports these findings yet, as for TSA-V0-045..052.
