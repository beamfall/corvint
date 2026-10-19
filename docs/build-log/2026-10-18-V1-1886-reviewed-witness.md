# V1-1886: reviewer-verdict obligation witness

Date: 2026-10-18
Ticket: V1-1886 (GitHub issue beamfall/corvint#1170).
Requirements: TOL-V0-073..078 (`docs/specs/corvint-tasks-obligation-ledger-v0.md`). They are
accepted under the owner's standing auto-accept of lane requirements (2026-10-11).

## Decisions

- **Reuse, no new profile.** `obligations witness --reviewed ID --review FILE [--exclude-author ...]`
  writes the existing OBLIGATIONS_WITNESS event with source `REVIEWED` and one optional `review`
  member; payloads and entries without it keep their exact bytes.
- **Independence.** Authors come from the CAL-V0-107 derivation (mode ALL) plus `--exclude-author`;
  a reviewer among them refuses. An unverified derivation without `--exclude-author` refuses
  INDEPENDENCE_UNVERIFIED. Distinct labels still do not prove independent people.
- **Agreement policy.** CREDITED needs K `correct` verdicts within `maxStubRisk` (`on-path` never
  admitted); otherwise HELD keeps every verdict and reason and returns a REVIEWED witness to OPEN.
- **Staleness is a read.** `obligations show` reports `witnessReview.stale` when the cited blob at
  the store worktree's HEAD differs; stored state changes only through a new review.
- REVIEWED is OWNER-only, like DECLARED.

## Residuals

- Read-time staleness is not seen by counts, gates or the ledger fingerprint.
- Staleness is read at the store worktree's HEAD, not at a reviewed candidate.
- The writer trusts the CLI-derived author list; the derived path is covered only by the existing
  DeriveAuthors tests.
