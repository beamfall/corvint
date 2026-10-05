# V1-0773: OpenCode file-change batches stop at Core's 100-path impact bound

Date: 2026-10-05. Ticket: V1-0773 (P2 BUG). Contract: `AHI-022` in
`docs/specs/agent-harness-integration-v0.md`. This change is stacked on V1-0767 (PR 580).

## Decision

The OpenCode plugin queued up to 256 paths per `file-change` batch. Core's `Impact`
(`maxImpactPaths = 100`, `internal/contextindex/index.go`) refuses more than 100 paths with
`impact paths exceed 100-path bound` and no `code`. The plugin turned that refusal into a
`corvint-command-failed` warning on any edit touching more than 100 files.

The ticket allowed three fixes: split batches, cap them, or have Core abstain with a code. The
plugin now caps each batch at `MAX_FILE_CHANGE_PATHS = 100`, matching Core's bound. A dropped
path names `changed-paths-truncated` once per batch at info level, and V1-0767's `adapterCodes`
records it as a content-free SOL-V0-010 row.

- **Why not split:** each extra batch is another Core subprocess of up to 2 s, and `afterTool` awaits
  the drain. Splitting would lengthen a large edit's tool completion by up to two more deadlines.
- **Why not change Core:** a coded Core abstention would add an error code to the shared `Impact`
  refusal used by every caller.

Post-tool `changedPaths` keep their 256-path cap; Core does not run impact on them. A 120-path
post-tool was observed to succeed against the real binary.

## Evidence

- **Reproduction** against a binary built from the base: 100 paths were answered with a coded refusal
  (`unsupported-impact-repository`), and 101 paths with the refusal that has no code.
- **New test:** `AHI-022 V1-0773 OpenCode file-change of 101 to 256 paths stays within Core's impact
  bound against the real binary`. A 150-path patch raises no warning except a disclosed timeout. A
  file-change timeout observes nothing about Core, so that attempt is retried, up to three times. The
  test passes only on an invocation Core completed. That invocation names truncation only for
  `file-change` and leaves exactly one `opencode file-change changed-paths-truncated` ledger row.
- **Tolerance removed:** the V1-0767 real-binary test no longer tolerates the file-change
  `corvint-command-failed` warning.
- **Updated tests:** the fixture tests now expect a 100-path batch, and a separately named batch for
  a 300-path call.
- **Negative control:** with the cap set back to 256, the new test, the V1-0767 test and the V1-0746
  test all fail. With the cap at 100, `TestHostAdapterJavaScriptHosts` passes.

## Limits

- Paths beyond 100 in one batch receive no impact evidence or SOL-V0-001 touched-path record. The
  truncation row discloses the loss.
- No specification has a traceability row that owns `Impact`'s 100-path bound. The `AHI-022` row
  names `MAX_FILE_CHANGE_PATHS` and the new test instead.
- The JS constant duplicates the Go constant. If Core lowers its bound, the real-binary test fails.
  If Core raises it, the plugin keeps the smaller cap.
- If all three attempts time out, the test fails as inconclusive. It does not pass.
- Live OpenCode qualification: NOT_OBSERVED.

## Package version

The package version moves from 0.7.7 to 0.7.8 in `package.json`, `runtime.js` and
`compatibility.json` (`AHI-020`). This also covers the unbumped V1-0767 `adapterCodes` change below
this commit. `hostVersionEvidenceAdapterVersion` stays 0.7.3, because no new native qualification ran.

## Rollback

Revert the commits. Batches go back to 256 paths and the warning returns above 100 paths.
