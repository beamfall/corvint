# Incremental release membership edit — V1-3193

Decision: add a closed `RELEASE_MEMBERSHIP` release mutation (`corvint-tasks release membership
--add/--remove`) rather than a partial `RELEASE_UPDATE` payload. The partial-update route would
have made every update field optional and lost the closed-payload refusal of a mistyped field;
a separate operation keeps `RELEASE_UPDATE` closed and lets membership refuse no-op edits
(re-adding a member, removing a non-member) that a whole-payload update cannot detect.

- TRM-V0-001/002: only `ticketIds` changes, under the same stale-revision, absent and promoted
  refusals as update; candidate and attestations clear, as an update clears them.
- TRM-V0-003: the role matrix maps membership onto `RELEASE_UPDATE`, so no policy file needs a
  new operation name and a policy that removes update also removes membership edits.
- TRM-V0-004: the CLI builds the membership payload from `--add`/`--remove` flags and sorts it
  (`release.CanonicalPayload`), so the operator-note set-order refusal cannot arise on the new
  route. Sorting caller-serialized create/update payloads was tried and reverted: V1-0750
  (`TestV10750_ReleasePayloadsCanonicalizeFramingButKeepSetOrderStrict`) deliberately keeps set
  order strict there, with a fix hint, so that contract stays.

Evidence: `TestTRMV0001_ReleaseMembershipEditsOnlyMembership` (WSL2, go1.27.3, PASS).
