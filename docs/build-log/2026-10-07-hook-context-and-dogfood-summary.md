# 2026-10-07: actionable-only hook context and a one-line dogfood PASS

## Intent

These changes come from the owner-requested token-usage audit ("ensure Corvint and corvint-tasks are
optimal with token usage at all times"). Three tickets cut model-visible bytes that the agent rarely
acts on. The evidence and the abstentions stay the same.

- **V1-0939.** The workflow argv guidance now appears only at a main-thread SessionStart, including
  `source=compact`. A Claude Code subagent SessionStart (a payload with an `agent_id`) gets no
  guidance.
- **V1-0942.** Hooks inject the `corvint-hook-context/0` projection in place of the whole
  `corvint-dogfood-event/0` receipt, and a prompt with nothing actionable injects nothing.
- **V1-0940.** A passing `dogfood check` or `dogfood seal` prints one summary line in place of the
  status JSON.

The requirements are AHI-045, AHI-046 and AHI-047, plus DCW-V0-033. Each is proposed and pending
owner acceptance. There are also proposed amendment notes on LCP-V0-011 and URE-V0-008.

## Design

- **Projection (AHI-045).**
  - The projection is derived from the receipt that the same invocation produced, inside the
    unchanged `internal/repoenvelope` frame.
  - **Kept:** task evidence with its blob pins, non-current declared scope, critical omissions,
    unavailable selectors and new degradations. A reduced compaction block is also kept. Once a
    packet is emitted, it also carries governance, an unresolved anchor resolution, a non-clean
    freshness and a non-inactive policy.
  - **Dropped:** the adapter block, repository identity, `requestSha256`, `resultDigest`, coverage
    counters, the constant Frontier result and the completion decision.
- **Unchanged.** The engine receipt and its profile are unchanged, and so are
  `localCompletionProfile`, `compatibility.json` and the Pi `workflow.js` validator. Stop gating,
  PreCompact/PostCompact, the degraded outputs and the AHI-016 disclosure are also unchanged.
- **Silence (AHI-046).**
  - Governance, an unresolved anchor count and the two per-installation degradations
    (`frontier-authority-unavailable`, `host-version-unknown`) never trigger a packet on their own.
  - `explicit-task-anchor-required` stays in the receipt.
  - The URE-V0-008 ledger records a silent prompt as an empty planned set, so later reads count as
    unplanned. A degraded or undelivered packet still refuses.
- **Compatibility.** An old plugin with a new binary gets the projection. A new plugin with an old
  binary gets the full receipt. Both arrive in the same envelope.
- **Dogfood summary (DCW-V0-033).**
  - On PASS the output is `dogfood-check: SUMMARY cem=… hunks=… supported=… unknown=… mechanical=…
    ocm=… [requirements=… linked=… unlinked=…] report=.corvint/dogfood-report.json detail=<git-dir>/corvint/dogfood-check.stdout`,
    then any no-intent note, then the unchanged `dogfood-check: PASS`.
  - The terminator stays because `internal/localcompletion` (`validateAggregateCheckCapture`) and the
    daily-loop harnesses read only that suffix.
  - The detail file holds the former JSON lines. It is written owner-only after removing whatever was
    at that path.
  - `DOGFOOD_VERBOSE=1` restores the old bytes, and so does an unwritable detail file. A failing CEM
    policy still prints its JSON as the repair detail.
  - A field that is missing or is not a plain token prints `NOT_OBSERVED`.

## Measurement

**Hook adapters.** A replay ran in a clean detached worktree of `79536edd`, indexed by each binary.
It drove 80 recent real user prompts plus four SessionStart sources through both adapters. The
before binary is `79536edd`; the after binary is this branch.

| Event | before additionalContext (B) | after (B) |
|---|---|---|
| Claude SessionStart startup/resume/clear (main thread) | 2862 | 1129 |
| Claude SessionStart compact (clean tree, disclosure included) | 3151 | 1418 |
| Claude SessionStart with `agent_id` | 2862 | 0 (stdout `{}`) |
| Codex SessionStart | 2296 | 584 |
| Claude UserPromptSubmit, mean (median) | 2036 (2847) | 318 (0) |
| Codex UserPromptSubmit, mean | 1772 | 324 |

After the change, the Claude prompts split into 41 silent, 15 with context and 24 degraded. The
degraded ones were deadline or `dogfood-event-unavailable` outcomes, and that count varied by 1
between runs with host load.

**Dogfood.** The portable fixture has one hunk and two requirements.

| Output | before (B) | after (B) |
|---|---|---|
| `dogfood check` PASS stdout | 2144 | 273 |
| `dogfood seal` PASS stdout | 2237 | about 366 |
| no-intent check PASS stdout | about 1355 | 301 |

The after figures are computed from the exact asserted lines with a 69-byte detail path. A real CEM
with more hunks prints more JSON before the change, so these savings are a lower bound (inference).

**Daily estimate.** This uses the audit's rates:

- UserPromptSubmit: 420 prompts a day × 1718 B saved, about 722 KB.
- SessionStart: 154 a day, about 98 of them in subagents (295 of 463). The saving is 98 × 2862 plus
  56 × 1733, about 378 KB.
- dogfood check and seal: 35 runs a day × 1871 B, about 65 KB.

The total is about 1.16 MB a day of model-visible text. At an assumed 4 bytes per token that is
roughly 290k input tokens a day. The tokenizer was not measured, and digest-heavy JSON probably
tokenizes worse, so this figure is an inference. Billed-token and cache effects are NOT_OBSERVED.

## Limits

- **Subagent detection.**
  - The `agent_id` semantics come only from the hook schema strings in the Claude Code 2.1.267
    binary. A live subagent run is NOT_OBSERVED.
  - Codex has no subagent signal, so every Codex SessionStart projects in full.
- **Unchanged consumers.** `tools/native-hook-observer` (experimental, UNQUALIFIED) still compares
  full receipts. The prompt-over-bound quiet notices are unchanged.
- **AHI-004 receipt link.** The receipt digests are no longer model-visible. AHI-045 records this as
  an amendment.
- **Context-bounds faults.** About six replay prompts fault `dogfood-event-unavailable`
  (`unsupported-dogfood-context-bounds`, LCP-V0-011). This is pre-existing friction and out of
  scope.
- **Snapshot eviction.** Each rebuilt binary needs `index --if-stale` on the replay root, because
  the shared snapshot store evicts older non-live snapshots.
- **Not run.** `make gate` (owner policy) and a live Claude Code or Codex session.

## Rollback

- **AHI-045 to AHI-047.** Revert `cmd/corvint/host_adapter_projection.go` and its call sites.
- **DCW-V0-033.** Revert it, or set `DOGFOOD_VERBOSE=1` for a single run.

No store, ledger, receipt, report or capture format changes.
