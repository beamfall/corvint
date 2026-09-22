# Decision 0099 — wire the two experimental adapter call sites behind operator opt-ins

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

`docs/decisions/0089-agentic-completeness-wave-2026-09-11.md` left two harness call sites unwired:
`unplannedread.HookPostTool` had no post-tool caller because no planned set existed at that event,
and a persisted one waited on the invariant 4 amendment `docs/specs/unplanned-read-events-v0.md`
proposed; `compactionkernel.InjectionBlock` had no caller because injecting it by default would
promote an experimental slice (invariant 8). Both slices were also blocked on frozen case sets that
cannot be collected while neither call site runs.

The owner call: wire both, each behind an explicit operator opt-in that leaves the default adapter
output and worktree unchanged, and keep both slices `proposed` intent and `experimental` delivery.

- Unplanned reads (`URE-V0-008`, `URE-V0-009`). The Claude Code user-prompt adapter records the
  delivered packet as a packet row in the existing marker-gated ledger, carrying 16-hex hashes of
  the packet paths and of the session key, not the paths or the session; the post-tool adapter
  judges a read against the newest same-session packet row and abstains when none is retained. No
  second file is added. The amendment the spec proposed is applied to `AGENTS.md` invariant 4 word
  for word: it covers every write to `.corvint/unplanned-reads.jsonl`, and the marker remains the only
  way to cause one.
- Compaction kernel (`CKN-V0-009`). Both hosts' session-start and user-prompt adapters append the
  framed kernel only when `CORVINT_EXPERIMENTAL_COMPACTION_KERNEL=1`. The adapter reads a fresh index
  snapshot and never builds one; any refusal is a fixed `NOT_RUN` line. An environment variable,
  not a marker file, is the opt-in because the kernel writes nothing and needs no new state.

Neither opt-in is promotion. Promotion still requires the enabled-ledger sample and the frozen
compaction-survival case set named in each spec; the opt-ins exist so those can be collected.

Rollback: delete `cmd/corvint/host_adapter_experimental.go`, its three calls in
`cmd/corvint/host_adapter.go`, the packet-row functions in `internal/unplannedread`, and the
invariant 4 sentence; delete `.corvint/unplanned-reads.jsonl` where enabled. No index, receipt, or
wire contract of another verb changes.
