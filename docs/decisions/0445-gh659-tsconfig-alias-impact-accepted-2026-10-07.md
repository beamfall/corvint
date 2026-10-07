# Decision 0445 — issue 659 tsconfig alias impact requirements accepted

Date: 2026-10-07. Status: accepted by the owner. On 2026-10-07 the owner replied "accept" to the
orchestrator's summary of the issue-659 requirements and the lane's eight default choices.

## Context

Issue [659](https://github.com/beamfall/corvint/issues/659) reported that `impact` on a TypeScript
page object imported through tsconfig `baseUrl`/`paths` selected no spec. The V1-0958 lane proposed
`GPK-V0-077`..`081` in `docs/specs/go-production-kernel-migration-v0.md` and recorded its choices in
`docs/build-log/2026-10-07-tsconfig-path-alias-impact.md`. Codex reviewed eight rounds; one P2
finding remains open as V1-0967. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `GPK-V0-077`..`081` as written: the alias arm of rule (c) for `baseUrl`, `paths`
and `extends`; config reading that never guesses; type-only imports as weaker edges; unresolved
bare specifiers disclosed as UNKNOWN; and the per-call lazy graph with memoised resolution.

It also accepts the lane's defaults:

1. With no package.json, tsconfig or jsconfig in the index, the default profile adds no disclosure.
2. Repository-wide UNKNOWN disclosure for unresolved bare specifiers is acceptable.
3. The nearest config applies regardless of its `include`, `files` and `references`.
4. A config that extends a package resolves nothing unless the leaf declares `baseUrl`, `paths`,
   the resolution mode and `moduleSuffixes` itself.
5. In classic resolution every bare specifier is unresolved.
6. CSS and JSON run-time edges stay declined.
7. An UNKNOWN impact keeps its READY/OUT_OF_SCOPE state with an uncertainty line.
8. Range impact is out of scope for now.

## Limits

This decision settles intent only. Delivery stays experimental. `make gate` was not run, and
the dot-prefixed alias gap (V1-0967) and alias resolution in `affected` (V1-0970) remain open.

## Rollback

Revert this decision and restore the "proposed, pending owner acceptance" markers and the
"Proposed amendment" heading for `GPK-V0-077`..`081`, then regenerate
`docs/specs/REQUIREMENTS.tsv`.
