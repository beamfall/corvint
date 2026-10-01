# Tasks dispatcher: Claude Code host

The owner asked for Claude Code support after the continuous dispatcher (issue 431, S11). It belongs
to the multi-host supervision goal of V1-0475. The dispatcher already launched any argv host. The
only host-aware part was the `finished` summary (CAL-V0-058), which recognized only OpenCode and
Codex event streams. A Claude Code worker run with `-p --output-format stream-json --verbose`
therefore summarized as a raw JSONL tail.

Decisions:

- `textOf` now also reads the Claude Code shapes: `{"type":"result","result":...}`, and the last
  non-empty text block of `{"type":"assistant","message":{"content":[...]}}`.
  - The terminal `result` normally wins.
  - An error result has no `result` field (for example `error_max_turns`). In that case the
    summary falls back to the last assistant text, not the raw tail.
  - Thinking, tool-use and tool-result blocks are never summarized.
- No new config key or host kind. Claude Code is configured as an ordinary host argv. The
  CAL-V0-058 text names the recognized streams.
  - `docs/TASKS-SUPERVISION.md` gives a host example: `--permission-mode dontAsk`, an
    `--allowedTools` allow-list, `--setting-sources project` and `--strict-mcp-config`.
  - With this setup, a headless worker is refused rather than prompted, and does not run the
    operator's user-level hooks.
  - The allow-list is host enforcement, not containment. This matches the existing non-goal.

Evidence:

- `TestCALV0058_SummaryReadsHostFinalText` covers these cases:
  - OpenCode and Codex;
  - a Claude stream-json transcript (init, thinking, text and tool use, tool result, rate-limit
    event, result);
  - a single Claude json object;
  - a Claude error result without `result`;
  - plain text;
  - mixed JSON and text.
- Live qualification on macOS used claude 2.1.267 (`--model haiku`) on a disposable fixture
  store, with the dispatcher built from this change.
  - **Run 1:** the model mistyped the long absolute path in the claim command. `dontAsk` refused
    the non-allow-listed command, and the worker exited 0 with no claim. The `finished` summary was
    the worker's own final sentence, and the ticket cooled down (1 of 2).
  - **Run 2:** `corvint-tasks` was on the worker `PATH` and allow-listed as
    `Bash(ct claim *)`. Worker `qual.impl.1.63077bd8-1` claimed AT-0002, wrote `src/hello.txt` and
    exited 0. The dispatcher handed it off: `attempt show` gives cause `HANDOFF` and
    handoffEvidence `dispatch:qual.impl.1.63077bd8-1`. The `finished` event carried the worker's
    final text, and the ticket cooled down.
  - `status` reported RUNNING while the dispatcher ran. After SIGINT it reported `stopped`, with
    0 workers left running, and status reported NOT_RUNNING.
- Not qualified:
  - Claude Code wall/idle kill of a long-running worker. This is host-independent and covered
    by `TestCALV0056_*`.
  - Models other than haiku.
  - Linux.
