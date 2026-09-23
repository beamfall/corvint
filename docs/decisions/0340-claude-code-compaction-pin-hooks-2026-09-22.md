# Decision 0340 — Claude Code compaction pin hooks

Date: 2026-09-22 (UTC). Status: accepted; registration static-verified, live cycle NOT_RUN.
Authority: ticket V1-0094 ("adapter: compaction re-pinning through PreCompact and PostCompact
hooks"), governed by `docs/specs/agent-harness-integration-v0.md` AHI-026 to AHI-030 and the
prose of `docs/specs/context-evolution-program-v0.md` §3.

## Decision

The Claude Code plugin registers the two compaction hook events the installed host documents,
`PreCompact` and `PostCompact`, as matcherless command groups on the existing native adapter
(`corvint adapter claude-code pre-compact|post-compact`, `cmd/corvint/host_adapter_compaction.go`).
The event names, payload fields and stdout handling were read from the installed Claude Code
2.1.267 hook runner, not from memory: `PreCompact` receives `trigger` and `custom_instructions`
and its raw stdout is joined into the compactor's custom instructions; `PostCompact` receives
`trigger` and `compact_summary` and its stdout is shown to the user only; neither reaches model
context. The host ignores hook names it does not know, so no other compaction event exists to
register at this version.

`pre-compact` emits, as text rather than hook JSON, one instruction line plus one
`corvint-compaction-pin/0` line: the compact `SessionStart` receipt's `context.compaction`
revision (the `HEAD` tree), its tracked and untracked dirty-path counts, at most 24 admitted
project-relative tracked dirty paths within 1500 bytes, and the elided count. `post-compact`
re-reads the last such line from the untrusted summary, re-validates every field, verifies the
pinned tree and each pinned path with one hermetic 500 ms `git cat-file --batch-check`, rebuilds
the current compaction block through the same read-only `session-start`/`compact` call AHI-003
uses, and prints one `corvint-compaction-report/0` line naming the non-rehydratable paths and
whether the revision moved. Degradations keep the `systemMessage` envelope and the SOL-V0-010
self-observation row; that row is the only write on either path (AHI-029).

## Why the model-facing re-emission stays SessionStart(source=compact)

The host offers no post-compaction channel into model context except `SessionStart` with source
`compact`, which AHI-003 already serves. The pin hooks therefore add a user-visible verdict and a
compactor-side anchor, not a second packet. Every compact `SessionStart` context now opens with a
fixed trusted disclosure that says so (AHI-030); on a host that lacks the compaction events the
registration is silently ignored and that disclosure, plus `compatibility.json` `compactionHooks`
(`verification` STATIC_ONLY, live cycle NOT_RUN), is how the gap stays visible.

## Alternatives set aside

- Reading the transcript to recover pins: refused by the AHI transcript boundary.
- Blocking compaction (`PreCompact` exit 2) when the pin cannot be built: coding continues on a
  degraded Corvint, never on a blocked host.
- Bumping the plugin version: `integrations/host-adapters.test.mjs` binds it to the shared
  `integrations/compatibility.json` matrix, which this change does not own; follow-up.

## Evidence and rollback

`TestAHI026ClaudeCompactionHooksRegisteredAgainstHostAPI`,
`TestAHI027ClaudePreCompactEmitsPinFromCompactionBlock`,
`TestAHI028ClaudePostCompactReportsNonRehydratablePaths`,
`TestAHI029ClaudeCompactionHooksMutateNothing`, the AHI-030 assertion inside
`TestAHI003ClaudeCompactSessionStartRehydratesDirtyPaths`, and the extended
`TestAHI017AdapterHostKillMatchesDeclaredHooks`. No frozen evaluation fits: the CEP §3 gate is an
unrun 30-task three-cycle trial and this change does not claim it. Rollback is removing the two
`hooks.json` groups, the `compactionHooks` block, the two adapter events and the disclosure; the
existing events and receipts are unchanged by this decision.
