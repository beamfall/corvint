# Decision 0481 — `corvint-tasks doctor` (TQD-V0-001..012) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accepted").

## Context

GitHub issue 715 asked for one read that names stuck-queue patterns. The doctor lane proposed
`TQD-V0-001..012` in `docs/specs/corvint-tasks-doctor-v0.md`: a pure-read `corvint-tasks doctor`
verb with five built-in detectors (NO_PROGRESS_HANDOFF, REPEAT_REFUSAL, SLOW_LANE_RECOVERY,
SETUP_ONLY_PROOF, and FALSE_IDLE over detached-run supervisors only), bounded project plugins, an
explicit `--refresh` cache under the git common directory and a lock-free `--line` status line.
The wording passed five Codex review rounds. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `TQD-V0-001..012` as written. The intent status, the agent digest, each
requirement's status, the matching entries in `docs/specs/README.md` and `docs/specs/INDEX.json`,
and the build log record the acceptance. The delivery status stays experimental.

## Limits

This decision settles intent only. The evidence is the focused CLI tests named in the spec's
traceability table. Live qualification is `NOT_RUN`. The listed open limits remain: PREFLIGHT_FAILURE
waits for #713's recorded preflight evidence, the dispatcher ledger is not read, and the Linux
process walk is unqualified. Native ticket V1-1076 is not completed by this decision.

## Rollback

Revert this decision and return the TQD-V0-001..012 status text to proposed. To withdraw the
behavior, also revert the doctor change (the `doctor` verb, the `taskman-doctor/0` item profile
and the `taskman-doctor-cache/0` cache profile), regenerate `docs/specs/REQUIREMENTS.tsv`, and
delete `<git common dir>/taskman-doctor/`, which nothing else reads. No stored format changes.
