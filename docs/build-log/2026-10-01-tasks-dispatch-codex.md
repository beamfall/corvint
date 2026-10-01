# Tasks dispatcher: Codex host

The owner asked for Codex support after the Claude Code host. It belongs to the multi-host
supervision goal of V1-0475. The dispatcher launched Codex as an ordinary argv host. Its summary
reader already matched the documented `codex exec --json` agent-message shape, but only a synthetic
unit case covered it, and nothing showed that a sandboxed Codex worker could claim.

Findings:

- A real `codex exec --json` stream (codex-cli 0.153.2) matches the existing reader. The stream
  runs: thread and turn events; then `item.started` and `item.completed` events for
  `agent_message`, `command_execution` and `file_change`; then `turn.completed`. The reader keeps
  the last `agent_message` text. No reader change was needed.
  `TestCALV0058_SummaryReadsHostFinalText` gains a case built from the observed shape.
- Under `--sandbox workspace-write`, `.git` is read-only, so the worker's `claim` cannot write.
  - **Run 1:** the claim failed, and Codex logged no `command_execution` event for it. The worker
    reported "filesystem permissions blocked it".
  - **Run 2:** made `.git/taskman` a writable root (`-c sandbox_workspace_write.writable_roots=`).
    The claim still failed `UNSUPPORTED_FILESYSTEM`. On a disposable copy of the fixture, the
    store's filesystem qualification probe was refused: it creates
    `.git/.taskman-qualify-*.tmp` in the git common directory itself, not under `taskman/`.
  - **Run 3:** made the whole common directory (`{workRoot}/.git`) a writable root. Worker
    `qual.impl.1.5d2c91ee-1` claimed AT-0002, wrote `src/hello.txt` and exited 0. The dispatcher
    handed it off: `attempt show` gives cause `HANDOFF` and handoffEvidence
    `dispatch:qual.impl.1.5d2c91ee-1`. The `finished` event carried Codex's final agent message,
    and the ticket cooled down (1 of 2).
  - `status` reported RUNNING. After SIGINT the dispatcher stopped with 0 workers left, and status
    reported NOT_RUNNING.
- `docs/TASKS-SUPERVISION.md` documents the Codex argv and the writable-root requirement,
  including the common-directory form for a linked worktree.
  - Granting the common directory also lets a worker write refs, objects, hooks, `config` and the
    store journal, so it can plant commands for later unsandboxed Git runs. The sandbox is host
    enforcement, not containment, which matches the existing non-goal.
  - Narrowing the store's writable footprint, so that only `taskman/` needs to be writable, is
    filed as V1-0637.

Not qualified:

- `--approve-for-me` and other approval routing.
- Codex wall/idle kill of a long-running worker. This is host-independent and covered by the
  CAL-V0-056 tests.
- Models other than the account default.
- Linux (Landlock sandbox).
