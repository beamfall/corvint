# Decision 0455 — navigation execution profile (NEX-V0) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("approve"), answering the question raised for native ticket V1-0252.

## Context

V1-0252 asked for a working navigation agent. The proposed answer is the optional companion
profile `docs/specs/application-flow-navigation-execution-v0.md` (`NEX-V0-001..007`) under
`AFU-V1-029`, implemented in `internal/appflows/navigation_execution.go` and
`tools/web-flows/navigation.mjs` and locally qualified by `script/web-flows-gate`
(`docs/build-log/2026-10-08-navigation-execution-integration.md`). The spec digest named owner
acceptance as its blocker; AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `NEX-V0-001..007` as written. The spec intent changes from `proposed technical
profile under AFU-V1-029` to `accepted technical profile under AFU-V1-029 (decision 0455; V1-0252)`
in the header, digest, `docs/specs/README.md` and `INDEX.json`.

## Limits

This decision settles intent only. Delivery stays `experimental; qualification pending`. External
or hosted qualification remains unobserved, the profile gains no API-observation or per-test
evidence authority, and V1-0252 is not completed by this decision; its required full gate is
separate.

## Rollback

Revert this decision and restore the `proposed technical profile under AFU-V1-029` intent in the
spec header, digest, README row and INDEX entry, then regenerate `docs/specs/REQUIREMENTS.tsv`.
