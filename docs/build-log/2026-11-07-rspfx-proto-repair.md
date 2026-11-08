# Runner protocol repair round

Base: `fd95d92413829a09718c323ddbb00c5980d8ae40` (integration repair base).
The earlier lane bootstrap base is `9761ea1f459c0d3b3c963953e24a76eaeda16370`;
this repair does not review or rewrite its intervening integration commits.

## Decisions and source findings

- RSP-V0-029: canonicalize the repository before containment and snapshots. A
  symlinked checkout previously let WalkDir hash only the link; a second write
  to an already dirty file could keep the same status and escape detection.
- RSP-V0-028: only an object-reader error wrapping `fs.ErrNotExist` proves an
  absent pins path. Normalize that case to an empty discovery set on either CI
  revision, preserving addition/removal claim unions. Empty bytes, malformed
  JSON and other read errors remain FULL. The caller owns the absence proof;
  no error-string matching or guessed Git failure interpretation is added.
- RSP-V0-025: refuse registered compiled-in runner names and the internal
  `script` and `make` command kinds with `compiled-in-runner-claim:<runner>`.
  The reference twin claims `reference-go-test`; it explicitly uses the existing
  `go-test` parser internally. The legacy hot-path assertion still checks
  `go-test`. Protocol shapes and existing assertions are preserved.
- RSP-V0-027: NUL environment values take the existing
  `schema-violation:environment` failure path, discarding staged evidence.
- RSP-V0-029: snapshot common Git config, config.worktree and hooks with the
  existing entry/byte/time bounds, including common metadata of linked worktrees.
  Hash the linked checkout's `.git` pointer too. Objects/packs stay excluded.
  The review's execution claim was incorrect: the existing `gitstatus.Status`
  uses private metadata and overrides `core.hooksPath` and `core.fsmonitor`.
  Keep that protection and test that planted fsmonitor code never executes.
- Item 6's duplicate-walk claim was incorrect: one working-tree content walk
  runs at Open and another after shutdown at Close. These are the before/after
  mutation comparison, not redundant reads. Ignored trees remain covered and
  retain fail-closed bounds; the added ignored-tree regression records this.

## Evidence and limits

Requirement-to-test mappings are in the existing RSP spec Traceability rows
025, 027, 028 and 029. New tests are in `internal/runnerprotocol/repair_test.go`;
process fixtures remain in `protocol_test.go`. Package vet passed for
`internal/runnerprotocol` and `tools/runner-reference`; formatting is clean.
Regression execution, live qualification and independent review are NOT_RUN:
the owner's final BUILD-ONLY instruction assigns them to the host verifier.
No performance improvement or runtime pass is claimed.

Dogfood start, context/affected selection, CEM binding/check/seal and frozen
selection evaluations are deferred under that same final check-only instruction
and the explicit reservation of `.corvint/change.cem.json` for integration.
No CEM, generated spec index, requirement TSV, use-case receipt, golden or
core-freeze output is changed. The spec's illustrative `go-test` pin remains
unchanged because the repair charter permits only Traceability row edits;
integration should reconcile that example with RSP-V0-025.

Rollback: revert this repair commit; no record-store migration or release
manifest change is involved. Removing the fixes restores the reported gaps and
must not be interpreted as qualification evidence.
