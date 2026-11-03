# Operator note: the 2026-10-30 Tasks barrier

The available repository record does **not establish how this specific barrier was lifted**.
Do not infer an UNPAUSE receipt, request ID, actor, command or time from later implementation work.
The live journal and ticket runtime were excluded from this lane by its no-Tasks-command charter.

The bounded evidence search was:

- `rg -n -i 'pause|barrier|unpause' docs/build-log/2026-10-30*.md docs/build-log/2026-10-31*.md`.
  The [October 30 widen/resume entry](build-log/2026-10-30-widen-barrier-and-resume-scope.md)
  records the supported recovery choices (operator unpause/widen/pause, or release/restoring claim)
  and fixture tests. It does not record executing either against the incident's store.
- The [October 31 Tasks core entry](build-log/2026-10-31-tasks-core-w2.md) records the named-ticket
  `--admit-under-barrier` exemption. This admits one scoped operation without lifting the barrier;
  it is not evidence of the incident's recovery.
- `git log --all --grep V1-4261` found `91ebd15bd` (guard spec) and `face50c0a` (writer guard),
  both dated November 2. The [November 2 entry](build-log/2026-11-02-gld-w44.md) reports protection
  from tests accidentally writing the source checkout, with fixture-only tests. None identifies
  the October 30 UNPAUSE transaction.

**Gaps:** the actual barrier bytes and pausing request ID, its removal receipt/request ID,
operator identity, recovery timestamp and whether recovery used the supported unpause writer
are NOT_OBSERVED in those records. An operator with the original journal can append a separately
pinned receipt audit; this note does not authorize a live write or claim that deleting a barrier
file is supported recovery.

For future incidents, inspect `queue status`: an old barrier now warns with the pausing request ID
(or `NOT_OBSERVED` when receipt evidence is unavailable), age and policy limit. The default limit is
one hour; optional policy `barrierWarningSeconds` admits 1..604800 seconds. Session-start hooks for
Claude/Codex show the same advisory warning. A warning never lifts an operator's barrier. Preserve
the receipt evidence before an authorized `unpause --request-id ID`; interrupted unpause uses the
[existing recovery contract](specs/corvint-tasks-pending-unpause-recovery-v0.md).

Qualification scripts invoking pause/unpause must set `CORVINT_TASKS_FORBID_COMMON_DIR` to
`git rev-parse --path-format=absolute --git-common-dir` in their source checkout **before** changing
into a disposable fixture. Go tests export this guard for their children automatically. Never
exercise recovery against the primary live store for test evidence.
