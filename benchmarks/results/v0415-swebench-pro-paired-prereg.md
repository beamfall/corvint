# V1-0415 paired SWE-bench Pro trial: preregistration

Status: preregistered 2026-10-25, before any agent run. Ticket V1-0415. This file is
committed and its sha256 is recorded in the trial receipt before the first agent session starts.
Any departure from it is reported as a deviation in the receipt; results are reported unrepaired,
including null and negative outcomes.

## Dataset and harness

- Dataset: `ScaleAI/SWE-bench_Pro` (Hugging Face) default config, 642 V2 tasks, exported to
  `ordered.jsonl` (sha256 `c2656e4b5763165f6e7b218227507d43dd7c85ac8c6fdd61f2d8f12b1440cffb`).
- Harness: `scaleapi/SWE-bench_Pro-os` at `66f92766bba642462d4bbe5479e83f91f9211862`
  (release v2.0.0), task directories `v2/tasks/<instance_id>/`.
- Images: `ghcr.io/scaleapi/swe-bench_pro-v2:<instance_id>`, pulled anonymously with every blob
  digest verified; manifest digests recorded per task.
- Grader: a local mirror of the V2 verifier. The patch is applied to a pristine image with the
  replay chain (`git apply`, then `git apply --3way`, then `patch --fuzz=3 -p1`), the task's
  `tests/` are copied to `/tests`, and `bash /tests/test.sh` writes `/logs/verifier/reward.txt`.
  A task is resolved when `reward.txt` is `1`. Controls: the gold patch must resolve and the
  empty patch must not.

## Subset rule

1. Order all 642 tasks by `sha256("v0415:" + instance_id)` ascending (the `_order` key in
   `ordered.jsonl`).
2. Walk the order and admit a task when its gold patch resolves under the local grader within one
   560 s grading attempt. A task whose gold patch fails, times out, or whose image cannot be
   pulled is skipped and recorded with its reason; the next task in order replaces it.
3. Stop at N = 10 admitted tasks. Pairs run in that order.

## Arms

Both arms use the same model, budget, charter, tool permissions, host, and repository snapshot.

- Agent: OpenCode 2.1.3, `opencode run --standalone --auto`, isolated `HOME` and
  `XDG_CONFIG_HOME` per arm, Claude Code compatibility and skill import disabled.
- Model: `anthropic/claude-opus-5-5`, provider default settings, existing machine credentials.
- Budget: at most 50 agent steps (`agent.build.steps = 50`) and 570 s wall clock per session
  (`timeout 570`). Whatever diff exists at the end of the session is graded.
- Tools: OpenCode built-ins with `shell`, `webfetch` and `websearch` denied in both arms. The
  agent can read, search, and edit files; it cannot run code or tests.
- `off` arm: no Corvint plugin, skill, or index.
- `on` arm: the Corvint OpenCode adapter from `integrations/opencode/src` at
  `866a7fd52a3cd979f9fd038ecfa998e15111421c` (tree `894e23eef521b7a36282936fc641dafbf20b456a`)
  with a pinned `corvint` binary built from the same commit
  (sha256 `9c8e01e19febd78e56b49f8355f41e4c39dc4c27278d2fb6b62429706cd0e111`). The index is built
  before the session with `corvint --root <repo> index`; build time is reported separately and
  is not part of the agent budget.
- Repository: `git archive HEAD` of the image's `/app` (or `/testbed`), committed as a single
  base commit in a fresh repository under a neutral path. `.corvint/` is excluded from Git.
- Charter: an identical fixed preamble (work in the current directory, which corresponds to
  `/app`; no shell or tests are available; edit files to make the change; do not commit) followed
  by the task's `instruction.md` verbatim.
- The two sessions of a pair run concurrently.

## Metrics

Per arm, over the admitted tasks:

- Primary: resolve rate (resolved / N).
- Tokens: input, output, cache read, cache write, summed from OpenCode's `session_v2` rows.
- Cost: OpenCode's recorded session cost in USD, plus cost per resolved task.
- Secondary: paired discordance (on-only and off-only resolves), patch bytes, sessions ending at
  the step or wall-clock limit, Corvint tool calls in the `on` arm.

With N = 10 this trial cannot detect small effects; no significance claim is made. The paired
counts are reported as observed.

## Spend control

- Estimate: about $3 per session, $60 for 20 sessions; planning bound $5 per session, $100 total.
- After the first pair, if the larger of its two session costs times 20 exceeds $130, N is
  reduced to `floor(130 / (2 * that cost))` and the cut tasks are reported as `NOT_RUN`.
- Hard stop: when cumulative recorded cost reaches $130, no further pair starts; remaining pairs
  are `NOT_RUN`.

## Known deviations from the official protocol

- The agent runs on the macOS host without a shell, not inside the task container, so it cannot
  run tests in either arm.
- Wall clock is 570 s per session instead of 50 minutes, because of the host tool-call limit.
- Grading uses a local docker mirror of `test.sh` with up to 4 CPUs and 8 GB, instead of Harbor or
  Modal (no account exists and none is created).
- N = 10 of 642, chosen by the deterministic rule above, not the full set.
