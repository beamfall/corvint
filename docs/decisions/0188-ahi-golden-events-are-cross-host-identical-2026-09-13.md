# Decision 0188 — AHI golden event expectations are pinned as cross-host identical

Date: 2026-09-13. Status: accepted, lane call (wave16-ccahiread). Authority: `docs/agent-memory/
tests.md` backlog item filed by `docs/reviews/corpus-mutation-audit-2026-09-13.md`, resolved under
the standing delegation to make bounded design calls in the same lane that closes the finding.

The audit found that every per-host `events[].expected` value in
`conformance/harness-event-v0/common-logical-interaction.json` (`sessionIdSha256`, `task`, `paths`,
`verification`, `stopHookActive`, `openedPaths`) could be changed for a single host with every
consumer green, because no test reads those values. Closing this needed a call on what the fixture
means: the file has no per-host raw `input`, only a golden `expected` object per host, and
`cmd/corvint/host_adapter.go:464@12a7c6a4` hashes `claude-code` session IDs under a different domain
(`corvint-local-completion-session/claude-code/0`) than the other three hosts
(`corvint-local-completion-session/0`), so the identical `sessionIdSha256` already recorded for every
host cannot be a real per-host re-derivation from one shared raw session ID — a single SHA-256
output has no host-varying preimage. The fixture is therefore read as what its name says: one
common logical interaction, and its golden object states the same normalized outcome regardless of
which host reports it, not a captured per-host hash chain.

The call: `TestAHI014EventExpectationsAreHostConsistent`
(`conformance/harness-event-v0/host_schema_test.go`) pins, per event, the exact closed set of hosts
that must carry a golden entry and requires every present host's `expected` object to be
canonically byte-identical to the others. This catches exactly the seven event-field mutations and
the file-change host-set mutation the audit listed, without inventing a per-host hash derivation
the fixture format cannot support. `TestAHI016OverBoundPromptMatchesCrossHostBoundaryCases`
(`cmd/corvint/prompt_bound_test.go`) separately now asserts each boundary case's `corvintInvoked`
against `reason == ""` from `normalizeAdapterInput`, closing the other half of the same backlog
entry.

A later change to `common-logical-interaction.json` that legitimately needs two hosts to disagree
on one event's fields (a real host-specific divergence, not a copy-paste fixture) must update this
decision or file a narrower successor; until then, cross-host disagreement on the fixture is a
corpus bug, not a supported shape.

Rollback: revert `TestAHI014EventExpectationsAreHostConsistent`, the `corvintInvoked` assertion in
`TestAHI016OverBoundPromptMatchesCrossHostBoundaryCases`, and this file. No production code path
changes with this decision; only test coverage does.
