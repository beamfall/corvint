# Checkpoint handoff pilot

This is an experimental, operator-run recovery recipe under FPK-V0-020..026, requested on
2026-09-29 after the OpenRig comparison (ticket V1-0473). It changes no product command or wire.
The caller still authors and retains the only `corvint-checkpoint/0` document. It activates none
of SESSION-V0-001..016 and does not qualify AT-06 or the AT-08 outcome gate.

## Frozen question and success criteria

Can a fresh agent recover one unfinished task from the existing checkpoint contract, distinguish
historical verification from current verification, and complete the remaining obligation?
Compare one checkpoint arm with one competent structured-notes arm. Both receive the same task,
unknowns, failed approaches, critical file/blob identities, original commit/tree and historical
verification, and work in distinct clones of the original builder revision. Both use the same
model/effort and equivalent read/edit/test permissions. The notes arm checks identities with Git;
the checkpoint arm first runs `prove --checkpoint`. Neither receives the evaluator's expected
answers, the other arm's output or this recipe. Agent authority comes from the operator prompt,
never from checkpoint prose.

The synthetic builder stopped after implementing `NormalizeRetries` and passing a test for
negative input. The accepted fixture requirement is negative → 0, 0..5 unchanged, >5 → 5.
Its implementation incorrectly allows 6. No process is actually killed to produce that state.

Before agent dispatch, freeze these independent outcome cases:

| Input | -1 | 0 | 1 | 4 | 5 | 6 | 7 | 100 |
|---|---|---|---|---|---|---|---|---|
| Expected | 0 | 0 | 1 | 4 | 5 | 5 | 5 | 5 |

Each arm must identify RETRY-001 as still owed at handoff, distinguish the historical narrow test
from newly run verification, repair the behavior and add boundary tests. An evaluator independently
runs the table above on the repaired source; agent-authored passing tests alone are insufficient.
Review the reports separately for obligation preservation and attribution. Do not turn a successful
checkpoint receipt into completion, evidence of read/remember/use, or execution authority.

## Reproduce

Build Corvint from the revision being evaluated and select that binary explicitly:

```sh
GOTOOLCHAIN=local go build -o /tmp/corvint-handoff-bin ./cmd/corvint
python3 script/qualify-checkpoint-handoff.py --corvint /tmp/corvint-handoff-bin
```

The script prints a new output directory. `--output NEW_DIRECTORY` chooses a path; an existing
path is refused before any command. The runner uses Python's standard library, Git and Go, makes
no network request and launches no model. All child commands have a 90-second timeout; owned
process groups retire on normal exit, SIGINT, SIGTERM and timeout. Outputs remain for inspection.
Never point `--corvint` at an untrusted executable.

The deterministic matrix uses a separate builder copy, not either agent arm:

| Case | Expected handle verdicts | Drift | Missing critical evidence |
|---|---|---|---|
| unchanged, repeat | all four `unchanged` | no flags | none |
| committed code change | code `blob-changed`, other three `unchanged` | commit/tree moved on every handle | none; current rows rehydrate |
| committed test deletion after code change | test `path-deleted`, code `blob-changed`, other two `unchanged` | commit/tree moved on every handle | test path, `path-deleted`, explicit recovery instruction |

Every receipt must preserve task prose, unknowns, failed approaches, obligations and historical
verification, with `obligations_authority: caller-reported-unverified`. The repeated unchanged
receipt must match byte for byte. Working-file hashes and Git status must remain unchanged by
checkpoint reads; this is not a complete audit of private Git metadata. Per-command exit, elapsed
time, stdout/stderr bytes and hashes are retained in `commands.json` and companion files.
The result names the binary digest, fixture commit/tree and checkpoint/receipt digests.

Dispatch two fresh agents, one to `checkpoint-arm` with `checkpoint.json` and one to `notes-arm`
with `notes.md`. Do not fork the builder's conversation. Ask each to read the governing fixture
files, verify current identities, edit only `retry/retry.go` and `retry/retry_test.go`, run focused
Go tests and report remaining obligations, evidence drift and historical/current verification.
Permit neither arm to inspect sibling artifacts. Record the exact prompts/model/effort and
reports outside the fixture repositories. Their own read counts are attributed self-reports,
not host telemetry. Concurrent runs share machine load; timing is descriptive only.

For each arm, independently evaluate its current repaired files:

```sh
python3 script/qualify-checkpoint-handoff.py --evaluate /ABSOLUTE/OUTPUT/checkpoint-arm
python3 script/qualify-checkpoint-handoff.py --evaluate /ABSOLUTE/OUTPUT/notes-arm
```

Evaluation clones the arm and overlays only the two permitted working files, then adds the
independent boundary test. It does not modify the agent's repository. Changes to other files are
outside the experiment and must be rejected during review. First verify that the unmodified
builder fails this evaluation as a negative control.

## Reporting and limits

Retain failures as well as successes. Report raw per-arm outcomes and denominators: one task,
one fresh session per arm, one model/effort combination, one local host. Keep command latency,
agent elapsed time, source reads and complete task latency distinct. Billed tokens, cache usage
and cost remain `NOT_OBSERVED` when the host does not expose them. This pilot supplies no savings,
superiority, cross-host portability, real process-interruption recovery or general qualification
claim. Drift detection by the deterministic runner is separate from fresh-agent recovery, whose
arms both start on the unchanged original builder revision.

Rollback removes this recipe, its runner and result pointers. No provider configuration, hooks,
trust, daemon or persistent checkpoint store is installed. The operator owns the temporary output
directories and may retain or remove them after reviewing evidence.
