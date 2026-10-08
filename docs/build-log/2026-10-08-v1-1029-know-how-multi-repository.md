# 2026-10-08: repository-qualified know-how notes (V1-1029)

## Intent

GitHub issue beamfall/corvint#687 (ticket V1-1029) reports that a WORKER cannot add know-how in a
program whose tickets span several Git repositories. The store lives in one checkout, the
touchPaths name the other repositories' files under a prefix such as `e2e/`, and a note could pin
only files of the store checkout. The WORKER scope check (KHN-V0-022) then refused every anchor
the worker could actually pin. The amendment is
`docs/specs/corvint-tasks-know-how-notes-v0.md` KHN-V0-024..027. It is proposed and awaits owner
acceptance. Delivery stays experimental. The adopter's repository and product names stay in the
issue, and the tests use the generic aliases `e2e` and `work`.

## Decisions

- **One repository per note, stored qualified.** `add --repo ALIAS=ROOT` pins in ROOT and stores
  each anchor as `ALIAS/PATH`, with `repository: ALIAS` on the entry. Because the stored paths are
  qualified, the unchanged KHN-V0-006 rule matches them against touchPaths, `list --path` and the
  WORKER scope. Selection and the scope check needed no code change (KHN-V0-026).
- **Optional key, omitted when absent.** The key follows the CREATE `localToken` pattern. The
  payload's closed set widens only when the key is present, so legacy payloads, records, receipts
  and replay keep their bytes. The Tasks codec, the payload decoder and the Core reader share one
  alias-and-prefix rule (`wire.KnowHowRepositoryRefusal`). RECONFIRM carries no key and inherits
  the repository; reconfirm needs `--repo` with the note's own alias.
- **Fail closed on ambiguity.** ROOT must be the work-tree top level, compared after following
  symbolic links. A subdirectory would otherwise pin paths relative to the wrong directory. A
  repeated alias is refused. On reads, an unmapped alias makes the note UNKNOWN with a
  `KNOWHOW_REPOSITORY:` warning. It never falls back to the store checkout: a test shows that
  checkout holding an identical blob still reads UNKNOWN.
- **Claim replay.** The claim's `--repo` is validated before anything commits and kept out of the
  lease request. Replaying a request id therefore keeps working, and freshness follows the
  replay's own mapping, like the existing read-time freshness rule.
- **Rejected alternatives.**
  - A persistent alias registry or environment variable would add configuration state.
  - Automatically mapping touchPath prefixes to checkouts would guess the repository.
  - A store-location flag is not needed: the store is still found from the working directory.
  - Multi-repository notes were left out to keep the ledger and pin semantics unchanged.

## Evidence

The traceability rows for KHN-V0-024..027 in the spec list the focused tests. Durable
qualification on a real multi-repository program is NOT_RUN. `make dogfood-change` is NOT_RUN in
this lane.

## Limits

An alias is an agreed name, not a repository identity. A reader that maps it to another
repository holding the identical blob at the same path sees CURRENT. A store that holds a
repository entry cannot be read by older binaries, which refuse it closed.
