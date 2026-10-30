# AI co-author trailer guard (V1-3227)

Lane rc3fix-w7, 2026-10-30, base `93b9f5b08ce841875c9c81d17905908d91346f99`. The owner rule of
2026-10-30 is that Claude must never appear as a GitHub contributor. GitHub credits a contributor
for every co-author trailer on a commit, so a single trailer from a lane host whose Claude Code
attribution setting was unset is enough to break the rule. Spec: `docs/specs/corvint-self-development-v0.md`
ASD-V0-560 (checker, batch-precheck step, opt-in commit-msg hook) and ASD-V0-561 (preflight report).

## Decision

- One script, `script/check-ai-coauthor-trailers.sh`, holds the rule. The range form serves
  `make batch-precheck` and CI, `--message` serves a commit-msg hook, and `--strip` is the rewrite
  step that the refusal prints.
- The rule keys on the address first. An Anthropic or OpenAI no-reply address (subdomains
  included), or a GitHub no-reply address of a Claude, Codex, ChatGPT or Copilot account, is
  decisive. Names count only when they are unambiguous agent names (Claude Code, Claude plus a
  model family, Codex, ChatGPT, Copilot and their bot forms). "Claude" is also a human first name,
  so a bare "Claude" is refused only with an AI address or with no address. "Claude Monet" and
  "Claude <claude@example.org>" pass, and so does a trailer quoted in commit prose. The test pins
  each of these cases.
- The batch-precheck step names the short HEAD SHA. Its ledger salt covers the step name, so a
  pass is never reused after a message-only amend. It also uses `--batch`: with no batch base, and
  in a checkout where `origin/main` does not resolve (the batch-precheck test fixtures), it skips
  and says so. Any explicit base that does not resolve is refused. Under `--base`, the base
  worktree's own range is empty, so a trailer classifies as introduced.
- The hook is opt-in (`--install-hook`). It writes to `git rev-parse --git-path hooks/commit-msg`,
  so it honours `core.hooksPath` and is shared by every worktree of the clone. It never replaces a
  different hook; instead it prints the line to add to that hook. Nothing installs it
  automatically.
- `script/agent-preflight.sh` reports `claude-attribution` from `${CLAUDE_CONFIG_DIR:-~/.claude}/settings.json`
  and the checkout's `.claude/settings{,.local}.json`; the last one that sets it wins. It fails
  when the setting is unset, because Claude Code adds the trailer by default. A host with no
  Claude Code directory is only noted.

## Evidence

- `make ai-coauthor-trailers-test`: cases 1 to 6, including running the printed rebase recovery
  over a range with a merge. Each assertion was mutation-checked.
- The lane's self-check found that the first version of the recovery called the checkout's own
  `script/check-ai-coauthor-trailers.sh` from inside the rebase. That fails on any commit that
  predates the script, which includes every commit in the batch that introduces it. The test hid
  this by giving the fixture an untracked copy. The recovery now copies the script to a temporary
  file first, and the fixture tracks no script until case 6. A mutation back to the checkout path
  fails case 4.
- `script/agent-preflight_test.sh`: 16 assertions. Case 4 covers attribution, and every case now
  pins `CLAUDE_CONFIG_DIR`, so the test no longer reads the real host settings.
- `make batch-precheck-test` passes with `TMPDIR=${TMPDIR%/}`. Its step counts in groups 3, 4 and
  4b go up by one. Those edits go beyond the single step line the dispatch allowed, and the test
  cannot pass without them. Group 10 fails on macOS when TMPDIR has a trailing slash. That failure
  is pre-existing at `dc89ab41b` and was filed as friction.
- Live: commits `384377e7e`, `465a7ec1a` and `bb7cfc04e` (other branches of this repository, each
  carrying an Anthropic no-reply co-author) are refused. All of `origin/main` from its root is
  clean (about 2 s). `make test-commands-check` passes, and TestSTRV0004 passes with the new
  `shell-test` and `GATE_STEPS` entry.

## CI change needed (not made; `.github/**` is out of lane scope)

CI already runs `make shell-test`, which now includes `ai-coauthor-trailers-test`. To refuse the
trailer on PR heads as well, add a doc-gates step
`script/check-ai-coauthor-trailers.sh "$BASE_SHA" HEAD` with `fetch-depth: 0`, or another way to
fetch the base. batch-precheck derives its doc-gate steps from `ci.yml`, so once that lands the
explicit batch-precheck step would run twice and can be dropped.

## Rollback

Follow the spec's Failure and rollback section. An installed hook is removed with
`rm "$(git rev-parse --git-path hooks/commit-msg)"`.
