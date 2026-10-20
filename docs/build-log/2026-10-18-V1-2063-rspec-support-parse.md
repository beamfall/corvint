# V1-2063: value-position Ruby blocks and support-file parse isolation

Date: 2026-10-20
Ticket: V1-2063 / GitHub #1312
Spec: `docs/specs/test-source-analysis-v0.md`, `TSA-V0-396` and `TSA-V0-397`

Decision: the owner's standing auto-accept of lane requirements (2026-10-11) accepts `TSA-V0-396`
(value-position `if`/`unless`/`case`/`begin`/`while`/`until` open a block in the Ruby line reader;
modifiers do not) and `TSA-V0-397` (an unreadable support file holds back only inclusions whose
shared group it could define, and the abstention anchors that file).

IDs: `TSA-V0-396..397`, after the highest ID in an open PR (`TSA-V0-390`, #1307 and #1311) plus a
gap of five.

Evidence: the #1312 reproduction table on the built binary moved from four `ABSTAINED` rows to all
seven `ELIGIBLE`; focused tests in `internal/testsource`. Real-suite qualification is NOT_RUN.
