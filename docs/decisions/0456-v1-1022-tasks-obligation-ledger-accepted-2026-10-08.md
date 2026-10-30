# Decision 0456 — Tasks obligation ledger (TOL-V0) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("accept the specs for V1-1022/1023/1024"), covering native ticket V1-1022 (GitHub #680).

## Context

V1-1022 asks for a native per-ticket obligation ledger with step-level Playwright witnessing. The
proposal is `docs/specs/corvint-tasks-obligation-ledger-v0.md` (`TOL-V0-001..021`), which listed
eight Unresolved decisions. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `TOL-V0-001..021` as written and resolves the eight former Unresolved decisions by
accepting the position the spec text takes:

1. `complete` and gates do NOT require every core obligation `WITNESSED` in V0.
2. Retry 0 only (`TOL-V0-010`).
3. OWNER by default; OPERATOR only through an explicit `policy.roles.OPERATOR` row; a WORKER report
   witness only by opt-in policy.
4. Bounds as stated: 256 obligations, 1,024 ledger events, 64 KiB events, 64 MiB reports.
5. HELD tickets admit ledger writes.
6. Report-embedded commit metadata (`captureGitInfo`) is not required in V0; the source presence check
   alone binds the commit.
7. The `TOL-V0-021` stall restart and its dispatcher ledger version are kept, with the rollback limit
   the spec records.
8. The workState `State` stays in the `TOL-V0-017` fingerprint; it is not dropped.

The spec's Unresolved decisions section is replaced by these resolutions, and its header, digest,
`docs/specs/README.md` and `INDEX.json` change from `proposed` to
`accepted (decision 0456; V1-1022)`.

## Limits

This decision settles intent only. Delivery stays `not-started`: no code, store member or command
exists, no live Playwright 1.63 fixture has run, and V1-1022 is not completed by this decision.

## Rollback

Revert this decision, restore `proposed` in the spec header, digest, README row and INDEX entry and
the Unresolved decisions section, then regenerate `docs/specs/REQUIREMENTS.tsv`.
