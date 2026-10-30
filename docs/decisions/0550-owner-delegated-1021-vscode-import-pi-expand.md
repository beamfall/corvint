# Decision 0550 — VS Code import command removal and Pi expand null-index refusal

Date: 2026-10-21. Status: accepted by the orchestrator under the owner's delegation. On
2026-10-21 the owner wrote "you can resolve those for me" in chat about the two tickets below.
The delegation covers only these two tickets. It is not a standing grant. New requirement IDs are
accepted under the owner's standing auto-accept of 2026-10-11.

## V1-0438 — VS Code "Import Test Observation" always fails

The command always refused with `UNVERIFIED_IMPORT` because no shared Corvint go-live verifier is
linked. A command that can only fail is not a user capability.

Decision: remove `corvint.importTestObservation` from the extension's command contributions,
activation events, registration and README, and remove the `corvint.testObservationPath` setting
that only that command read. `VSC-V0-079` amends `VSC-V0-021`, `VSC-V0-028` and the section 5
failure classes: import is not offered until a shared verifier is linked (post-1.0). The internal
importer (`src/observations.ts`) keeps its fail-closed `UNVERIFIED_IMPORT` refusal and its unit
test, and the conformance import vectors stay unchanged as internal-path safety assertions.
Restoring the command needs an accepted amendment that links the verifier, plus a test that imports
a verified observation file.

## V1-2150 — Pi `corvint_expand` null index expands row 0

Pi 0.99.1 converts a null integer argument to 0 before schema validation. As a result, a null
`result` or `evidence` index silently expanded row 0.

Decision: approved fix. `AHI-071` requires the typed refusal `invalid-expansion-index` for a null
or missing index, both in the tool's `prepareArguments` (before host conversion) and in the
handler, with no native dispatch. The Pi adapter is bumped from 0.3.2 to 0.3.3 together with this
schema change. That supersedes the accepted Pi tuple (host 0.99.1 / adapter 0.3.2, decision 0452)
in AHI-024, LCP-V0-008/009, the PWV-V0 inventory, the package, both compatibility manifests and
the native `adapter pi` and `dogfood event` admission. Adapter 0.3.2 now refuses visibly.
Qualification is unchanged in kind: FALLBACK only. The live-host result for 0.3.3 is recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-decide-1021.md`.

## Rollback

Revert the change. Restoring 0.3.2 requires replacing the Pi package and the Corvint binary as a
pair. Restoring the VS Code command requires the verifier amendment described above.
