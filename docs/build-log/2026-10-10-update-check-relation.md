# corvint-update check states the installed-to-available relation (V1-0639)

`corvint-update check` reported `installedVersion` and `availableTag` and left the caller to compare
them. The two do not compare as strings: the Tasks developer tag (`tasks-dev-20260929.2`) has no
build, and the installed Tasks build 407 was newer than the latest published release (build 202),
so a naive reader would call an apply an update when it is a refused downgrade.

Decision: `check` adds `relation` (`CURRENT`, `UPDATE_AVAILABLE`, `LOCAL_NEWER`, `UNKNOWN`) with
`relationBasis` and `relationReason` (`UPD-V0-009`, proposed). Tasks compares on the integer build
declared by the release's `build-verification.json`, accepted only when its archive digest equals the
`SHA256SUMS` entry and its target is the host; this costs two bounded metadata reads per Tasks check.
Core release metadata (`verification-report.json`) declares no build number, and the release body's
build is free text, so Core compares the installed version label with the tag by semantic-version
precedence. Anything unbound or unparseable is `UNKNOWN` with a reason. The relation is advisory:
`apply` still decides from the verified candidate (`UPD-V0-004`).

Live before the change (2026-10-10): Core installed `1.0.0-rc.3 (build 407)` against `v1.0.0-rc.2`;
Tasks installed build 407 against `tasks-dev-20260929.2` declaring build 202. Both are expected to
report `LOCAL_NEWER`.

Evidence: `TestUPDV0009CheckStatesRelation`; the Tasks installed-version half of the ticket was
delivered by V1-1106 (`TestUPDV0001TasksInstalledVersion`).
