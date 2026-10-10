# GitHub 715: `corvint-tasks doctor` intent accepted

Date: 2026-10-10

The owner accepted `TQD-V0-001..012` in chat on 2026-10-10, recorded as
[decision 0481](../decisions/0481-tqd-v0-001-012-tasks-doctor-accepted-2026-10-10.md). The
[doctor contract](../specs/corvint-tasks-doctor-v0.md), its `docs/specs/README.md` row and its
`docs/specs/INDEX.json` entry now read accepted; delivery stays experimental and
`docs/specs/REQUIREMENTS.tsv` was regenerated.

The implementation is unchanged by this note. It passed five Codex review rounds; round 5 passed
at `7b6b5036`. Open: PREFLIGHT_FAILURE waits for #713, the dispatcher ledger is not read, the
Linux process walk is unqualified, live qualification is `NOT_RUN`, and native ticket V1-1076 is
not completed here.
