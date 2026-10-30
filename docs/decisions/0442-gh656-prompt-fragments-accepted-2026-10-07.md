# Decision 0442 — issue 656 shared dispatch prompt fragments accepted

Date: 2026-10-07. Status: accepted by the owner. On 2026-10-07 the owner replied "accept" to the
orchestrator's report of the requirements proposed for GitHub issue 656.

## Context

Issue [656](https://github.com/beamfall/corvint/issues/656) reported a dispatch config refused at the
256 KiB file limit because a shared rules block was repeated in every role prompt. The V1-0954 lane
proposed a top-level `prompts` fragment map that role prompts reference and that expands at decode
time. Codex reviewed the lane in three rounds until it reported no remaining finding. AGENTS.md
invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts these requirements as written in their spec at branch `claude/gh656`:

| Requirement | Spec | Ticket | Summary |
|---|---|---|---|
| `CAL-V0-175`..`178` | `corvint-tasks-agent-leases-v0.md` | V1-0954 (issue 656) | Shared dispatch prompt fragments: declared once, expanded to a byte-identical inline prompt, fail-closed refusals naming role and fragment, every existing prompt check applied to the expanded prompt |

The orchestrator's in-task choice to refuse unreferenced fragments (fail closed) stands with this
acceptance.

## Limits

This decision settles intent only. Delivery status is unchanged:

- A live dispatcher run with an adopter-sized config was not run.
- `make gate` and the full `go test ./...` were not run.

## Rollback

Revert this decision and restore the "proposed, pending owner acceptance" markers in
`corvint-tasks-agent-leases-v0.md`, `docs/specs/README.md` and `docs/specs/INDEX.json`, then
regenerate `docs/specs/REQUIREMENTS.tsv`.
