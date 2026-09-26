# cw-trial

The external agent-harness dispatcher for the confidently-wrong trial
(`docs/specs/confidently-wrong-trial-v0.md`, `docs/plans/BREAKTHROUGH-BET-2026-09-01.md`). It runs
one agent over one frozen task set under three context arms, inside a disposable copy of each
task's repository snapshot, and scores what the agent committed to. It downloads nothing and
writes only the report and the temporary copies it removes.

## Protocol

- **Arms.** `none`: the task text only. `grep`: the task text plus the top-K paths of
  `tools/retrieval-bench`'s term-overlap scorer, reimplemented here (distinct task terms per
  file, a path match counts double, ties by occurrences then path; `rg` is not used). `corvint`:
  the task text plus the `corvint` task-context packet verbatim, `context --task TEXT --limit K`,
  with `--subject CHANGED_FILE` for a `change` task (task-context-packet-v0). Every arm gets the one
  prompt skeleton; only the arm name, its fixed prologue, and the context body (bounded at
  64 KiB) differ. The arms differ in supplied context, never in access. `aider`: the task text plus the
  rank listing of an independently provided aider repository-map executable (`--aider-command`),
  seeded with the subject path and the task's code-shaped identifiers. The former shipped
  Python bridge is retired under GOC-V0-008; historical results remain unchanged. Corvint does
  not install that external toolchain or claim a native reimplementation of aider's ranking.
- **Access.** `--access read-only` (the pilot) runs the agent inside the copy, where it can find
  gold by exploring. `--access none` (the judged mode, decision 0022) produces every arm's context
  from the copy first, removes the copy, and runs each arm from a fresh empty directory under a
  skeleton that says the repository is unavailable and forbids commands; `tool_calls` and
  `commands` per invocation and `explored_tasks` per arm record what the agent ran anyway.
- **Claim grammar.** The reply must end with a fenced JSON block
  `{"claims":[{"kind":"test-file|source-file|answer","value":"...","confidence":"certain|likely|unsure","evidence":"..."}]}`.
  The scorer takes the last fenced block naming `"claims"` (or a bare JSON object reply). No
  block is `ABSENT`, an unparseable one `MALFORMED`; both abstain. `evidence` is a packet row
  `id`, a grep hit path, or `none`; the report records whether it appears in the supplied context.
- **Scoring.** A claim whose kind the gold covers is `TRUE` or `FALSE`; other kinds are
  `UNJUDGED`; claims outside the grammar are `INVALID`. `success`: any `TRUE` claim.
  `confidently_wrong`: `FALSE` claims at `certain` (`confidently_wrong_task` when any).
  `abstained`: no valid claim. Placement: `gold_in_context` (gold anywhere in the context),
  `gold_as_result` (gold as a result row), `utilisation_gap` (gold as a result row and no
  success: the agent, not the retrieval, failed), `context_failed`. Per arm: 95% Wilson
  intervals for `success`, `confidently_wrong_tasks`, `abstained`, and the two gold placements,
  the `utilisation_gap` and `context_failed` counts, claim counts, summed `wall_ms`, and summed
  codex `tokens` (or `NOT_OBSERVED`). Errored invocations (timeout, launch failure, non-zero exit with no claims block) count under
  `errors` and score nothing.
- **Dispatch.** `codex exec --json --ephemeral -s read-only --skip-git-repo-check -C COPY -m MODEL
  [-c model_reasoning_effort=E] -o REPLY PROMPT` with stdin closed; token counts come from the
  `turn.completed` usage event. `--agent script --agent-command PATH` runs any executable with the
  prompt as its argument and reads stdout (`testdata/fake-agent.sh` proves the pipeline without a
  model). Every unobserved field is `NOT_OBSERVED`, never zero. Under `--access none`,
  `--workers N` runs up to N invocations at once and `--reuse REPORT` copies any arm whose prompt
  is byte-identical in a prior report (same model and access), so a packet change reruns only the
  corvint arm; reused records carry `reused_from` and each arm's summary counts `reused`. A run
  with `--output` checkpoints the report so far to `OUTPUT.partial.json` after every finished
  invocation (removed on completion); a killed run is resumed with the same command plus
  `--resume`, which reruns only the invocations that never finished.
- **Identity.** The report records the model, agent version and command shape, `corvint`
  version and sha256, the manifest sha256, the skeleton sha256, and each arm's prologue text and
  sha256. Output is canonical JSON; `cw-trial score --report FILE` rebuilds every derived field
  from the preserved replies and reproduces the bytes.

## Task manifest

```json
{"partition": "pilot", "source": "...", "tasks": [
  {"id": "...", "kind": "change", "repo": "etcd-io/etcd", "base_commit": "<sha>",
   "changed_file": "cache/cache.go", "text": "Which test file(s) cover ...",
   "gold": {"test-file": ["cache/demux_test.go"]}}]}
```

`kind` is `retrieval` or `change`; `snapshot` may name a directory instead of a corpus chunk
file. `cw-trial import-heldout --tasks tasks.jsonl --manifest manifest.json [--chunk-root DIR]
[--output tasks.json]` produces this manifest from the frozen set under `testdata/heldout-v1`,
verifying its digest (and each chunk file's under `--chunk-root`) and refusing any row it cannot
map one field to one field. A `pilot` partition can never count as held-out. A held-out set must be authored and
frozen before its author sees any Corvint output (`benchmarks/README.md`); the tool records only
the manifest digest.

A task may carry `"control": "already-fixed"` and no gold (CEP-V0-004/005): its issue is already
resolved at the pinned revision, so every valid claim is `FALSE` and a `certain` claim sets
`control_failed` on any arm. Whether a produced corvint packet abstained is recorded apart as
`packet_abstained`. Controls stay out of the arm's other aggregates and are counted only in its
`already_fixed` block. `testdata/already-fixed` holds a synthetic fixture (never held-out
evidence).

`cw-trial generate --repo DIR --name OWNER/NAME --since DATE --output DIR [--limit N]
[--min-files N] [--max-files N]` builds an `unseen` change-task set from a local Git history
without any download (CWT-V0-012): each qualifying non-merge commit after the date becomes one
co-change task (subject: the modified non-test source file with the largest diff; gold: every other
touched file plus the subject's same-directory test counterpart when the parent tree holds one;
text: the commit message as the intent plus the subject's hunks), and the parent tree
is written whole under `DIR/corpus/OWNER__NAME/PARENT.chunks.jsonl` in the shape `run` reads. The
manifest records the population, the sampling rule, and how many gold paths the commit created.
A repository younger than the model's training data is the intended input: the none arm's success
on it is the memorisation check a public corpus cannot make.

## Running

```console
go build -o /tmp/cw-trial ./tools/cw-trial
go build -o /tmp/corvint ./cmd/corvint
/tmp/cw-trial run --tasks tools/cw-trial/testdata/pilot/tasks.json \
  --corpus corpus/v2_code2test --corvint /tmp/corvint \
  --agent codex --model gpt-5.6-sol --effort medium --timeout 5m \
  --output benchmarks/results/cw-trial-pilot-first-run.json
/tmp/cw-trial run --tasks ... --arms none,grep --agent script --agent-command tools/cw-trial/testdata/fake-agent.sh
/tmp/cw-trial score --report benchmarks/results/cw-trial-pilot-first-run.json
```

The pilot manifest holds five Agent Retrieval Bench `v2_code2test` samples (etcd, one changed
file each) with the samples' `related_tests` as gold; the corpus is the bench's own chunk files,
materialised as `tools/retrieval-bench` does. `--history DIR` names a local repository whose
history holds the tasks' base commits (the unseen set's own repository): such a task is
materialised as a shared clone at its base commit, with history, so the packet's `cochange` slot
has something to read; the source is never written, and the record carries `history_commits`.

## Unseen task sets

Frozen under `testdata/`, each generated by `generate` from a private local repository the model
cannot have seen; snapshots are not stored, `run --history REPO` materialises them from the
repository. `unseen-corvint-v1` (34 tasks, this repository to `ed7f5ed^`, `--max-files 10`);
`unseen-corvint-v2` (63 tasks, same tip, `--max-files 40`, v1 is a subset; sha256
0c04367bc67479da32865ed08c294effa3b1bb23073333aae44a50abea42672b); `unseen-beamfall` (60 tasks sampled evenly from 1,444 candidates in
Beamfall's Go/TSX history since 2026-06-01; sha256 255f6835fa822b758cf3dbe953972ce3f42f3eebb355bd46a9f455a8caccdfaf);
`unseen-beamfall-apple` (40 tasks from 389 candidates in Beamfall's Swift history; sha256
d3eddb6d37e39045919f15c98413bcf247fca4a5bf805780e10850b6cb4d6f89). `beamfall-roadmap-v1` (50 roadmap-ticket-to-diff `retrieval` tasks
from Beamfall/core at `02835b1be`, built by its own Git-only script, not `generate`; frozen and not
yet run, V1-0319) carries its trial protocol in its README. Task files embed commit messages and up to 8 KiB of the subject's
diff from those repositories.
