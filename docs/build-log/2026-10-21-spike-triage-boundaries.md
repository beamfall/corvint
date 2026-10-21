# Spike triage: V1-0468, V1-0581, V1-0610, V1-0635, ci-shadow-historical-compat

Five P1–P3 spikes that need a recorded boundary or procedure rather than a code change. Decisions
delegated to the orchestrator (decision 0548).

## V1-0468: the fixture task-context reader accepts only policy/0

`internal/taskman/capture.go:278` requires `profile == "taskman-policy/0"`, and `:269` closes the
policy field set. The only caller of `taskman.Preview` is `cmd/corvint/taskman_fixture.go:15`,
the fixture task-planning path. `cem_gate_verdict.go` uses only `EvaluateCEMVerdictGate`, which does
not read the policy document. So the policy/1 refusal is limited to the fixture path, as the ticket's
2026-09-29 correction states.

**Proposed recommendation:** keep the explicit /0-only fixture contract and its fail-closed
refusal. Record "queues opted into taskman-policy/1 are unsupported by the fixture task-context
preview" as a named limitation. Upgrade the reader only if a supported product path starts calling
`Preview`.

## V1-0581: cancelling an ACTIVE unsatisfied enrollment for a new base

The ticket describes a procedural deviation, not a Core defect. `docs/DOGFOOD.md:985` already says
not to cancel or clear state to resume, and `:1027` says `cancel` records non-success. Neither says
what to do when the base must move while an enrollment is ACTIVE and unsatisfied.

**Proposed recommendation:** add one rule to DOGFOOD.md:

> A new base needs a fresh worktree and a new enrollment. Leave the original enrollment ACTIVE in
> its own checkout until its coordinator disposes of it. Never cancel it in place to make room.

Follow-up: have `dogfood cancel` on an ACTIVE unsatisfied enrollment print the check and observation
counts it will close, and require `--reason`. This is a small CLI build with a spec requirement.

## V1-0610: Python closeout guards and swallowed interruption

The ticket's failure class was a handler that raised `InterruptedError`, which Python retries under
PEP 475, so SIGINT was swallowed. I audited every signal-handling script in the tree:

| Script | Handler raises | Spawn and cleanup |
|---|---|---|
| `script/qualify-checkpoint-handoff.py:47` | `SystemExit(128+signum)` | `start_new_session=True`, `os.killpg` |
| `script/qualify-lsp.py:52` | `KeyboardInterrupt` | `start_new_session=True`, `os.killpg` |
| `script/measure-lsp-quality.py:73` | `KeyboardInterrupt`, after retiring the active group | `start_new_session=True`, `os.killpg` |
| `script/qualify-lsp-editors.py:237` | a dedicated exception | `start_new_session=True`, `os.killpg` |

None raises `InterruptedError`. The temporary closeout wrapper from the ticket is not in the
repository. **Result: not reproduced in tracked tooling.**

Proposed follow-up (tests): a docs or lint check that refuses `raise InterruptedError` inside a
`signal.signal` handler under `script/` and `tools/`.

## V1-0635: hidden skips and stale entries in test read scopes

- **Stale entries (local, static).** All 156 packages declared in `.corvint/test-read-scopes.json`
  exist at base `90ab9f33e` and contain `_test.go` files. Zero stale entries.
- **Hidden skips and over-broad declarations.** These need Landlock in a Linux clone that has `.git`
  and CI's environment, and the ticket's methodology note requires the same. That is not available
  on this macOS host, so this part is `NOT_RUN`.

**Proposed recommendation:** a scheduled or manually dispatched CI job. It runs each declared
package confined and unconfined with `go test -json` and diffs per-test outcomes, so a test that
skips or falls back under confinement shows up. Track it as a follow-up tests ticket.

## ci-shadow-historical-compat: retained historical selection misses

This needs a hosted 50-minute `-race -p 1` historical run (the recorded run took 2451.7 s), so it
was `NOT_RUN` here. The retained row is already correctly refused: the two omitted failing packages,
`internal/frontiernextrepo` and `internal/tracerecordrepo`, kept the driver from promoting.

**Proposed recommendation:** add the Git executable version to the frozen runtime identity before any
promotion campaign. The recorded run used Git 2.55.0, and both missed packages read Git directly.
Then re-run one frozen pair and record whether the misses are selection gaps or runtime drift.
