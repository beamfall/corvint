# Decision 0441 — rc3 issue 650 to 653 requirements accepted

Date: 2026-10-07. Status: accepted by the owner. On 2026-10-07 the owner replied "accept" to the
orchestrator's list of the requirements proposed for GitHub issues 650 to 653, the multi-root
`corvint-mcp` proxy and the token-usage audit.

## Context

The owner asked to fix and merge issues
[650](https://github.com/beamfall/corvint/issues/650),
[651](https://github.com/beamfall/corvint/issues/651),
[652](https://github.com/beamfall/corvint/issues/652) and
[653](https://github.com/beamfall/corvint/issues/653), to audit Corvint and Corvint Tasks for token
usage, and to serve several repositories from one `corvint-mcp` ("do it"). Each delivery lane drafted
proposed requirements, and Codex reviewed each lane and the integrated batch until it reported no
blocking finding. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts these requirements as written in their specs at the integration head:

| Requirement | Spec | Ticket | Summary |
|---|---|---|---|
| `CAL-V0-145`..`154` | `corvint-tasks-agent-leases-v0.md` | V1-0936, V1-0858 (issue 651) | The dispatcher integrates detached attempt runs and relaunches a finished run's ticket under its own role |
| `CAL-V0-155`..`161` | `corvint-tasks-agent-leases-v0.md` | V1-0937 (issue 652) | Role and ticket budgets, worker token accounting with `UNKNOWN` never read as 0, declared effort, dispatch state `/2` |
| `CAL-V0-165`..`174` | `corvint-tasks-agent-leases-v0.md` | V1-0935, V1-0941 (issue 650) | Compact agent output: `--fields`, `--summary`, opt-in retries, terse help, compact list items |
| `PSR-V0-016`..`021` | `corvint-tasks-pool-safe-reuse-v0.md` | V1-0946 (issue 653) | Up to 4 attempts share one `ALLOCATED` pool member, which quarantines once after its last attempt |
| `MMR-V0-001`..`008` | `mcp-multi-root-v0.md` | V1-0938 | One `corvint-mcp` serves up to 32 declared repository roots, each addressed by alias |
| `AHI-045`..`047` | `agent-harness-integration-v0.md` | V1-0939, V1-0942 | Hooks inject a small `corvint-hook-context/0` projection, stay silent when nothing is actionable, and give workflow guidance only at a main-thread SessionStart |
| `DCW-V0-033` | `daily-change-evidence-workflow-v0.md` | V1-0940 | A passing `dogfood check` or `dogfood seal` prints one line |

The owner also accepts the matching amendment notes on `LCP-V0-011`
(`local-completion-policy-v0.md`) and `URE-V0-008` (`unplanned-read-events-v0.md`).

## Limits

This decision settles intent only. Delivery status is unchanged:

- Live dispatcher, fleet and host qualification were not run.
- An older-binary run against a store that holds share bindings was not run; its refusal is
  inferred from the closed decoders.
- `make gate` was not run.

## Rollback

Revert this decision and restore the "proposed, pending owner acceptance" markers in the affected
specs, `docs/specs/README.md` and `docs/specs/INDEX.json`, then regenerate
`docs/specs/REQUIREMENTS.tsv`.
