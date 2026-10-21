# LSP, editor and Pi batch: adapter decisions (V1-0450, V1-0448, V1-0438)

Date: 2026-10-20. Tickets: V1-0450, V1-0553, V1-0443, V1-0448, V1-0438. Base: origin/main 7d78a56b0.

## V1-0450: OpenCode untracked file-change refusals

The fix is in Core, not in the adapter. The adapter cannot see the indexed revision, so it cannot
tell an untracked path from a real failure. `harness event` file-change now answers an untracked
path with the structured code `untracked-impact-path` (exit 2, same message bytes). OpenCode records
that code at info level, with the other expected refusals.

The code is scoped to the harness. `corvint impact` and the MCP surface keep their uncoded refusal
bytes. The code is an expected state, not an unsupported capability, so it appends no SOL-V0-007
self-observation row. Evidence: `TestAHI022HarnessFileChangeCodesUntrackedPath` and the real-binary
OpenCode lifecycle test in `integrations/host-adapters.test.mjs`. OpenCode moves to 0.7.16, which
also fixes the README header (V1-0553).

## V1-0448: pi-protected audit items

- **Item 1.** The dead `willRetry` test is removed. Pi's extension `agent_end` carries only
  `messages`, and a retried turn ends with `stopReason: 'error'`.
- **Item 2.** `resizeImage` resolves null on any worker failure, which is the upstream omission
  contract. It never falls back to in-process decoding.
- **Item 3.** The qualified attempt and the ordinary fallback share one 2000 ms budget. An
  exhausted or expired budget reports `deadline`.
- **Item 4.** `QUALIFIED_FAULTS` is a closed set. Any unlisted fault folds as
  `qualified-lifecycle-unavailable`.
- **Item 5.** Closed with a reason, not fixed. The `null` to `0` coercion is in the ordinary Pi
  adapter's `corvint_expand` integer parameters (`integrations/pi/tools.js`). Pi's validator converts
  the value before the handler sees it. Fixing it changes the model-facing schema and needs a
  coupled Pi adapter version bump, so it belongs in a separate ticket.

## V1-0438: skipped

Removing the VS Code Import Test Observation command touches TypeScript in `extensions/vscode`.
This lane has no `node_modules`, `tsc` or `@types/vscode`, and new npm installs are out of bounds.
The change could not be compiled or tested, so it was not made.
