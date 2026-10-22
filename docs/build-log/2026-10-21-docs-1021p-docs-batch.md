# Docs batch 1021p: V1-2161, V1-2175, V1-2183, V1-2181, V1-2154, V1-2234

## Problem

Six documentation gaps. V1-2161: lane guidance lacked a no-`git stash` rule (shared `refs/stash`).
V1-2175: `CAL-V0-060` did not say administrative writes skip checkpoint retention. V1-2183:
`STA-V0-007` did not say Spec Kit presence uses admitted sources. V1-2181: a symlinked root
`AGENTS.md` was refused with an unexplained message. V1-2154: shell-test registration was
undocumented. V1-2234: a lane entry said "see the outcome below" with no outcome section.

## Decisions

- Requirements are accepted under the owner's standing auto-accept (2026-10-11).
- `STA-V0-014` (new) states the symlink rule. No `STA-V0-` ID was reserved or open elsewhere
  (checked origin/main and open PRs). The refusal code stays `unsupported-query-authority`; only the
  message names the symlink (`selectQueryAuthority`, `internal/contextindex/query.go`). Resolving
  in-tree symlinks stays post-1.0.
- `CAL-V0-060` and `STA-V0-007` are amended in place, no new IDs.
- Shell-test registration lives in `docs/SHELL-TEST-REGISTRATION.md`, routed from
  `docs/AGENT-ROUTES.md`; the `unbounded-readers` failure message and the `STR-V0-004` test message
  point to it.
- Lane entry shape (problem, decisions, evidence, NOT_RUN, rollback, Outcome) is documented in
  `docs/LANE-ORCHESTRATION.md`; the one existing dangling "see the outcome below" now cites the
  section that exists. Optional lane-ops stash detection (V1-2161 criterion 2) is not built;
  `script/lane-clean.sh` already keeps checkouts with stash entries.

## Evidence

- `TestSTAV0014_SymlinkedRootGuideIsNotAuthority` failed before the message change (generic
  message) and passes after.
- Doc gates and focused package tests: see Outcome.

## NOT_RUN

Tier 2 affected tests (batch CI); `make gate`.

## Rollback

Revert the commit; no data or wire format changed.

## Outcome

All six tickets delivered. Dogfood result and NOT_PRODUCED reasons are in the PR body.
