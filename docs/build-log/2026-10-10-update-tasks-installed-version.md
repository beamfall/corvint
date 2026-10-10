# corvint-update reports the Tasks version string (V1-1106)

Date: 2026-10-10

`corvint-update check --component tasks` reported `installedVersion` as the whole
`corvint-tasks version` command result, a `taskman-command-result/0` JSON envelope. Core reports
a plain version line, so the two components read differently and the Tasks value was hard to
compare with a release tag.

`internal/update.version` now decodes the Tasks envelope and uses its single item's `version`
field, for example `0.0.0-tcp01-unverified+build.407`. The build number comes from that string.
The identity check is stricter than the old substring test. Output must be JSON with profile
`taskman-command-result/0` and exactly one item with a non-empty version. Anything else is refused
as `unexpected Tasks version identity`, and check reports the installed identity as unknown
(UPD-V0-001). The same parse applies to a candidate's version smoke during apply (UPD-V0-002).

Evidence: `TestUPDV0001TasksInstalledVersion` covers the envelope form. It refuses a bare text
line, a wrong profile, an empty item list, two items and an empty version. The test is cited in
the UPD-V0-001 acceptance row of `docs/specs/operator-update-v0.md`. The requirement text is unchanged.
