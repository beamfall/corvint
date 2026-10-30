# Decision 0462 — repository-qualified know-how notes (KHN-V0-024..027) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("accept the specs for batch H too"), covering native ticket V1-1029 (GitHub #687).

## Context

V1-1029 lets a program whose tickets span several Git repositories pin know-how notes to files
in the other repositories. The proposed requirements are `KHN-V0-024..027` in
`docs/specs/corvint-tasks-know-how-notes-v0.md`: `--repo ALIAS=ROOT` on add, the optional
`repository` entry key, qualified-anchor matching against touchPaths, and per-alias freshness on
list and claim. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `KHN-V0-024..027` as written. Their status lines change from
`proposed (V1-1029; GitHub #687)` to `accepted (decision 0462; V1-1029; GitHub #687)` in the spec
header, digest, preamble, `docs/specs/README.md` and `INDEX.json`, and V1-1029 leaves the digest's
"Blocked on" list. `KHN-V0-008..015`, `KHN-V0-016..020` and `KHN-V0-021..023` stay `proposed`.

## Limits

This decision settles intent only. Delivery stays `experimental`, and V1-1029 is not completed
by this decision.

## Rollback

Revert this decision, restore the `proposed (V1-1029; GitHub #687)` text, re-add V1-1029 to the
digest's "Blocked on" list, then regenerate `docs/specs/REQUIREMENTS.tsv`.
