# Decision 0464 — dot specifiers as relative web imports (GPK-V0-082) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("accept GPK-V0-082 too"), covering native ticket V1-0967.

## Context

V1-0967 makes the web import resolver treat a specifier as relative only when it is `.` or `..` alone
or followed by `/` or `\`, as TypeScript's `pathIsRelative` does (`webRelativeSpecifier` in
`internal/contextindex/webresolve.go`); any other specifier that starts with a dot is bare and
goes through `paths`, `baseUrl` and the package test. The change is query-time only; the analyzer schema stays `corvint-analyzer/112` and only its input SHA pin moves. The proposed requirement is `GPK-V0-082` in `docs/specs/go-production-kernel-migration-v0.md`. AGENTS.md invariant 8 keeps acceptance
human-owned.

## Decision

The owner accepts `GPK-V0-082` as written. Its status changes from `(proposed 2026-10-08, not accepted; V1-0967)` to `(accepted 2026-10-08, decision 0464; V1-0967)`, together with
any status summary in the spec header that names it.

## Limits

This decision settles intent only. Delivery stays `experimental`, and V1-0967 is not completed by this
decision.

## Rollback

Revert this decision, restore the `(proposed 2026-10-08, not accepted; V1-0967)` text, then regenerate `docs/specs/REQUIREMENTS.tsv`.
