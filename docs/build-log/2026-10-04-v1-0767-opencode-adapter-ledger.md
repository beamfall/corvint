## 2026-10-04 V1-0767: OpenCode path abstentions reach the SOL-V0-010 ledger

Human-owned intent: the owner asked to start V1-0767. V1-0746 named two OpenCode path
abstentions at `console.info`, but the plugin had no `SOL-V0-010` writer, so they never reached
`.corvint/self-observations.jsonl` as the Codex and Claude Code abstentions do.

### Decision

This extends decision 0169's adapter-degradation writer to OpenCode through the existing
`harness event` call. It adds no new `corvint` command, because a separate adapter-report command
would cost one more process spawn per abstention for the same closed row.

- An `opencode` `file-change` or `post-tool` input may carry `adapterCodes`: a sorted,
  duplicate-free list of `changed-paths-truncated` and `post-tool-path-not-project-relative`.
  - Any other host, event, value or shape refuses the whole event as `invalid-harness-input`.
    Native hooks, authority events and the pi adapter are therefore unchanged.
  - Core removes the field before input validation, so the receipt and the response are identical
    with or without it.
- After `--root` resolves, Core appends one row per code through `observations.Append`, whether or
  not the event later succeeds. Rows use host `opencode`, the event, and `corvintVersion` (new
  `EventRequest.CorvintVersion`, set by `cmd/corvint`).
  - The append is synchronous and fail-open like the `SOL-V0-001` append in the same handler. It
    is bounded by the plugin's process deadline, not by an adapter-specific one.
  - Rows inherit the writer's content-free contract and per-hour deduplication.
- `internal/observations` admits host `opencode` and code `changed-paths-truncated`.
- The plugin sends the codes it names at `console.info`:
  - on the `post-tool` call that dropped a path or saw an out-of-project path;
  - on a truncated `file-change` batch.
- The plugin's receipt check (`receiptIdentity`) and the stub fixture also leave `adapterCodes` out
  of the basis. Without that, every coded `post-tool` failed as `incompatible-corvint-output`
  against the real binary. The real-binary test found this; the stub alone hid it.
- Specs amended in place: `SOL-V0-010` (requirement, failure modes, traceability and rollback) and
  `AHI-022`.

### Evidence

- `TestOpenCodeAdapterCodesAreLedgeredOutsideTheReceipt`:
  - an uncoded event writes no ledger;
  - adding codes leaves the receipt and response unchanged;
  - two identical coded events leave exactly one content-free row per code.
- `TestAdapterCodesAreClosedToOpenCodePathEvents`: another host, another event, an unknown code,
  unsorted, duplicate, empty and non-list input are all refused, and `.corvint` is untouched.
- `AHI-022 V1-0746 OpenCode names the path cap and an out-of-project path at info level` now also
  asserts the `adapterCodes` each captured event carries, and that no path reaches them.
- `SOL-V0-010 AHI-022 V1-0767 OpenCode path abstentions reach the self-observation ledger against
  the real binary`. Against a real `cmd/corvint` build it asserts:
  - three `opencode` rows (`file-change`/`changed-paths-truncated`, and `post-tool` with each code);
  - two identical out-of-project calls deduplicated to one row;
  - no path, session or host event name in the rows;
  - no warning except the one below.

### Limits and follow-up

- Core refuses a `file-change` batch over 100 paths ("impact paths exceed 100-path bound"), with
  no code, while the plugin batches up to 256. The plugin warns `corvint-command-failed` for such a
  batch. This predates V1-0767 and is filed separately; the real-binary test tolerates only that
  warning. The truncation row is written before the refusal.
- A new plugin against an older Core gets `invalid-harness-input` (a warning) on the rare coded
  event, because older Core refuses the unknown field. The plugin and Core ship together, and no
  package version bump was made, matching V1-0746.
- Live OpenCode host qualification: `NOT_OBSERVED`.

### Rollback

Remove the plugin's `adapterCodes` field first, then `takeAdapterCodes` and its append, then the
`opencode` host and `changed-paths-truncated` admissions. Retained rows age out under the cap.
