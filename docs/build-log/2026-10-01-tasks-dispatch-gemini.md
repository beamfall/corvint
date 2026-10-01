# Tasks dispatcher: Gemini CLI host

The owner asked for Gemini support after the Claude Code and Codex hosts. It belongs to the
multi-host supervision goal of V1-0475. The dispatcher can already launch Gemini CLI as an ordinary
argv host. Its `finished` summary (CAL-V0-058), however, did not recognize Gemini's output.

- `gemini -p -o stream-json` streams each assistant reply as several
  `{"type":"message","role":"assistant","content":...,"delta":true}` chunks.
  - The old reader kept only the last chunk.
  - Gemini's `result` event carries no text.
- `-o json` prints one pretty-printed `{"session_id","response","stats"}` object. Because it spans
  several lines, the old reader fell back to the raw tail.

Decisions:

- `extractText` now handles both Gemini output formats:
  - **Stream:** consecutive assistant delta chunks are joined into one reply, and any other event
    ends that reply. The last reply wins, as for the other hosts. Later `error` and `result` events
    do not replace it.
  - **`json`:** when the whole tail parses as one object with a non-empty `response` string, the
    reader uses that string.
- Thought parts never reach the stream (the CLI emits only text parts), and user messages are not
  summarized.
- No new config key or host kind. CAL-V0-058 names the recognized streams.
- `docs/TASKS-SUPERVISION.md` gives a host example:
  - flags: `-p {prompt} -o stream-json --approval-mode default --policy <file> --skip-trust -e none`;
  - a policy TOML that allow-lists the claim, submit, gate and test commands plus the file tools.
  - Headless Gemini denies a call no rule matches and fails one a rule asks about, so the policy
    works as an allow-list.
  - The allow rules use priority 999. Rules from Gemini settings rank above a priority-100
    `--policy` rule.
  - The argv `PATH` includes `/opt/homebrew/bin`, because the `gemini` launcher is a
    `#!/usr/bin/env node` script.
  - `--skip-trust` makes `workRoot/.gemini/settings.json` and `.env` load. A worker allowed to
    write files can widen its next run there. The doc says so.
  - Like the other hosts' allow-lists, the policy is host enforcement, not containment.

Evidence:

- The event shapes come from the installed Gemini CLI 0.54.0 bundle (Homebrew `@google/gemini-cli`):
  - the stream formatter writes one `JSON.stringify(event)` line per event;
  - the non-interactive runner emits `init`, the user `message`, assistant `message` deltas (text
    parts only), `tool_use`, `tool_result`, `error` and `result`;
  - `JsonFormatter.format` prints `response` with two-space indentation;
  - the policy engine's non-interactive default decision is `DENY`, and a matched `ASK_USER` throws
    "requires user confirmation", for example on a redirected shell command.
- `TestCALV0058_SummaryReadsHostFinalText` gains two cases:
  - a Gemini stream-json transcript (init, user message, chunked reply, tool use and result, a
    second chunked reply, a warning, result);
  - a pretty-printed Gemini json object.

An independent review found no code defects. It found these doc defects, now fixed:

- the example `PATH` lacked `node`;
- settings rules outranked the policy;
- the doc did not say that workspace settings load;
- the ASK_USER wording was wrong.

Accepted limit: a `-o json` output over the 64 KiB tail loses `response`, so its summary is the raw
tail.

Live qualification is BLOCKED. On this machine the Gemini CLI is signed in with personal Google
OAuth. A headless probe failed with `IneligibleTierError`: that client is no longer supported for
Gemini Code Assist for individuals. No `GEMINI_API_KEY`, `GOOGLE_API_KEY` or Vertex configuration
is present. Live qualification needs owner-provided non-interactive authentication. It would run
the same disposable-fixture flow as the Codex host (claim, handoff, `finished` summary and
SIGINT stop).

Not qualified: any live Gemini run, the documented argv and policy, wall/idle kill (this is
host-independent; CAL-V0-056 tests), and Linux.
