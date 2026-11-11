# Release catalog aligned to PROD-INT-006; one-tag release run; change-folder acceptance rows

Lane relrest, tickets V1-5295, V1-5236 and V1-5296. Requirements: `ARTIFACT-BIN-V0-520` to
`-524` in `docs/specs/release-artifact-integrity-v0.md`, `ACR-V1-013` to `-015` in
`docs/specs/acceptance-record-v1.md`. All proposed.

## Decisions

- Binaries the plan does not ship stay in the catalog under a new role, `withheld` (decision
  0596), instead of being deleted from it or relabelled `support`. The catalog must still classify
  every `cmd/` directory (`ARTIFACT-BIN-V0-001`), and `support` means release tooling that is
  built on purpose; neither fits an analyzer awaiting removal. `corvint-cli` and
  `corvint-web-flows` have no 1.0 verdict yet and are withheld too: the ticket asks that every
  product binary be one the plan ships, and an open verdict is not a ship.
- The drift test reads the plan text (the `PROD-INT-006` "archives for ... with" clause and the
  "Binaries per product" note) rather than a list copied into the test, so a plan amendment fails
  the test until the catalog follows. An unreadable block fails; it never skips.
- The release candidate's app archives follow the catalog's targets (`AppPlatforms`), not the
  four core archive platforms. Otherwise a three-target release manifest could never assemble
  into a candidate. The core archive gate keeps its five targets and the candidate its four core
  archives; narrowing those is the `release-process` change (`REL-V1-008`). Consequence: an install
  on darwin/amd64 refuses at the app step.
- `release --tag` requires an absent or empty output directory. `manifest` merges every entry in
  the directory, so a stale entry from another run would otherwise enter the manifest. On failure
  the run removes only the files it recorded as written, then the directory if it created it.
- No change folder was auto-accepted under G14. Deciding that a proposal "only implements an
  accepted requirement" is a classification the owner has not delegated to lanes, so all 45 folders
  without a row got `pending-owner` rows. `acceptance-record` keeps its `auto` row; its proposal
  header was the one header that disagreed with the log.

## Left open

- The `.github/` workflow that runs `release-binary release` per tag and publishes the output
  (V1-5236 remainder).
- The plan file's header still says it "proposes a plan" and "accepts nothing". The text is pinned
  by decision 0596 (`script/acceptance-plan-pin-check.sh`), so only the owner can change it.
- G14 classification of the 45 `pending-owner` folders.
