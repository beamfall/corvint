# Decision 0512 — Flows traffic bounds, impact cancellation and CEM mark lock (AFU-V1-057..059, MCPV0-035, CEM-PILOT-013 amendment) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner answer in chat, 2026-10-11 UTC
("Those 8 PRs, finish them"), confirming acceptance of the listed requirements for pull requests
#758, #760, #761, #762, #764, #765, #766 and #767.

## Context

PR #765 proposed `AFU-V1-057` and `AFU-V1-058` (V1-0276: bounded navigation traffic indexing) and `AFU-V1-059` (V1-0279: cancellable impact graph construction) in `docs/specs/application-flow-understanding-v1.md`, `MCPV0-035` in `docs/specs/mcp-server-2026-07-28-v0.md` (V1-0279: cancellation of an active MCP impact request), and an amendment to `CEM-PILOT-013` in `docs/specs/cem-pilot-kit.md` (V1-0665: for in-place updates the map lock precedes the first bounded read). AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `AFU-V1-057`, `AFU-V1-058`, `AFU-V1-059`, `MCPV0-035` and the `CEM-PILOT-013` amendment as written in the spec text carried by this pull request. Their status text
changes from "proposed, pending owner acceptance" to "accepted, decision 0512".

## Limits

The evidence is the tests named in the three PR build logs. Live qualification and `make gate` are `NOT_RUN`. This decision is not a standing rule for other requirements.

## Rollback

Revert this decision and return the five status texts to proposed. To withdraw the behavior, also revert the corresponding `internal/appflows`, `internal/liveverify/affected`, MCP and `cem` output-lock changes of PR #765, then regenerate `docs/specs/REQUIREMENTS.tsv`. No stored state changes.
