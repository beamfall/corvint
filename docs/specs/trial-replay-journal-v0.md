# Trial Replay Journal V0

Owner: Russell Lewis
Date: 2026-10-08
Intent status: proposed
Delivery status: not-started
Authoritative inputs: `confidently-wrong-trial-v0.md` (CWT-V0-004), `cem-reviewer-trial-v0.md`
(CRT-V0-010 and its rollback rule), `gate-ledger-v0.md` (the deterministic-step boundary), `AGENTS.md`
invariants 1, 2 and 4, and `docs/build-log/2026-10-08-trial-replay-journal-surface.md`

## Agent digest
- Claim: Both agent trial dispatchers replay a settled invocation only when a full pinned key matches and record where each replayed observation came from.
- Status: proposed / not-started; no code exists and no owner has accepted the intent.
- Exists: per-tool `--resume` and `--reuse` in `tools/cw-trial` and `tools/cem-trial` (`checkpointer`, `loadReuse`, `reuseSource.apply`), governed by CWT-V0-004 and CRT-V0-010.
- Blocked on: owner acceptance of the intent, then the unresolved legacy-report decision below.
- Read next: Verified current state; Requirements; Unresolved decisions.

## User and measurable job

The operator of a held-out or development agent trial edits one arm's context, prologue or scorer
input and reruns the trial. They want to pay only for invocations whose inputs changed, and they
need every reused observation to say exactly what produced it. Success means:

- a rerun invokes the agent only for steps whose replay key changed;
- a changed agent runner, reasoning effort, model, access mode or invocation contract always
  invokes the agent again;
- each replayed record names a retained source and the digest of the original record, so a reviewer
  can check the reuse from the repository alone.

The source of the idea is the workflow journal in the MIT package `@andrewjacop/pi-herdr` 0.6.0
(`src/workflow/journal.ts`, itself ported from `tintinweb/pi-subagents`). It appends one entry per
settled step, keyed by position and a hash of the step's inputs, and replays a run's unchanged
prefix. This spec takes the key-and-journal idea, not the code. It replaces the position prefix
with declared dependencies (TRJ-V0-011), because positional matching is only needed when steps
share hidden state, and trial steps do not.

## Verified current state and provenance

Measured at `3989698e` on 2026-10-08:

- `tools/cw-trial` and `tools/cem-trial` each implement reuse separately. Each keeps a
  `checkpointer` that rewrites the whole report as `OUTPUT.partial.json` after every finished
  invocation. Each has a `loadReuse` and `reuseSource.apply` that copy a prior record when the
  task, arm (and in cem-trial the repeat) and `prompt_sha256` match, and the prior record finished
  without failing. cw-trial also refuses a prior record with `error` set.
- The source checks differ:
  - cw-trial requires the same profile, model and access for both `--reuse` and `--resume`
    (`--resume` loads the checkpoint through `loadReuse`), and admits either only under
    `--access none`. In that mode the agent runs in an empty directory, so the snapshot reaches
    it only through the prompt;
  - cem-trial requires the same profile and model for `--reuse`. Its `--resume` merges the
    checkpoint with no check, and silently ignores a checkpoint it cannot load. It has no access
    mode, and records `model` as `NOT_OBSERVED` for a script agent.
- Both reports record an `agent` identity. For codex it holds the `codex --version` output, the
  argv template with placeholder paths, and `effort`; for a script agent it holds the script path
  and its sha256. Neither check compares it. So a reply produced by a different codex version, a
  different argv, a different reasoning effort or a different script is reused silently. The codex
  binary's own digest is not recorded, so the version string is the only identity it has. A
  script's sha256 covers only the script file, not any agent CLI it wraps.
- Both tools give the agent the dispatcher's full environment (`os.Environ()`) and record none of
  it. Ambient codex configuration (`CODEX_HOME`, its `config.toml` and global instructions) can
  therefore change a reply without changing anything the reports record.
- `reused_from` means different things in the two tools: cw-trial records the sha256 of the source
  file, and cem-trial records the source report's `manifest_sha256`, which identifies the task set,
  not the source. Across the 26 committed `corvint-cw-trial/0` and `corvint-cem-trial/0` reports
  under `benchmarks/results/`, 1,397 of 3,333 recorded invocations are reused. Every reuse is in
  cw-trial, and development reruns reuse 65–100% of their invocations. None of the 14 distinct
  source digests matches a JSON file tracked in the current tree. Those observations therefore
  cite sources that cannot be checked from the repository, which falls short of invariant 1.
- A reuse source is read whole under the 8 MiB `maxOutputBytes` bound, so a larger report cannot
  be reused at all; scoring reads the final report under the same bound. Each checkpoint rewrites
  the full report, so a run writes roughly n/2 times the final report size in total. For the
  1.3 MB, 252-invocation `cw-trial-unseen-corvint-v2-run-1` that is about 165 MB, an estimate
  rather than a measurement.
- `gate-ledger-v0.md` already reuses deterministic `make gate` steps, keyed on their exact inputs.
  An agent invocation is not a deterministic function of its inputs, so it needs a different
  rule: replay the recorded observation and label it as replayed.

## Requirements

- `TRJ-V0-001`: Both dispatchers MUST implement `--resume` and `--reuse` through one shared package
  that owns the replay key, the journal, eligibility and provenance. Each dispatcher keeps its own
  record shape, scoring, failure rule (`failedInvocation`, `laneFailed`) and validity rules (CWT,
  CRT). cw-trial's `--access none` admission for `--workers`, `--reuse` and `--resume` MUST stay
  unchanged.
- `TRJ-V0-002`: The replay key MUST be the SHA-256 of the canonical JSON (`gokernel.CanonicalJSON`)
  of this object:
  - `schema` `corvint-trial-replay-key/0` and the dispatcher's report `profile`;
  - `step`: `{task, arm, repeat}`, with `repeat` 0 where the dispatcher has no repeats;
  - `prompt_sha256`, `model` and `access`, where cem-trial's `access` is the fixed value `none`;
  - `agent`: the runner identity object the report records, with a script agent's `command` path
    removed (its sha256 identifies it). For codex this includes the placeholder argv template,
    which already carries the sandbox mode, and `effort`;
  - `ambient`: for a codex agent, the sha256 of each of `config.toml` and `AGENTS.md` under the
    resolved `CODEX_HOME` (default `~/.codex`), with a missing file as the fixed value `absent`;
    for a script agent, the fixed value `script`;
  - `inputs`: the immutable identity of what the agent can read beyond the prompt. For cw-trial
    under `--access none` this is the fixed value `none`, because the agent runs in an empty
    directory. For cem-trial it is the clone's `base_commit`; the change's patch reaches the agent
    through the prompt;
  - `upstream`: an array (TRJ-V0-011).
  Some `NOT_OBSERVED` values are fixed by construction: `effort` with no override, and `model` for a
  cem-trial script agent. Those are values, not missing members. Apart from them, if any member is
  `NOT_OBSERVED` or cannot be computed (for example, an unreadable codex config file), the step
  MUST run live and be journaled with `replayable: false`. It MUST NOT match anything. The key
  does not cover the rest of the agent's environment or a CLI that a script agent wraps; both
  limits MUST be stated in each report as `replay_key_limits`.
- `TRJ-V0-003`: The key MUST NOT include a filesystem path, host, user, clock time, worker count, run
  id or completion order; the placeholder tokens in the codex argv template are not paths. Two runs
  with the same inputs therefore produce the same key on any machine.
- `TRJ-V0-004`: A run with `--output` MUST append one JSON Lines entry to `OUTPUT.journal.jsonl`
  for each settled step, after the step's record is final and under the dispatcher's existing
  record guard. The file MUST be append-only and MUST NOT be rewritten. An entry MUST hold:
  - `schema` `corvint-trial-replay-journal/0`;
  - `key`, `replayable` and `step`;
  - `outcome`: `FINISHED` when the record has no `error` and is not failed under the dispatcher's
    failure rule, otherwise `FAILED`;
  - `record`: the dispatcher's record, byte for byte as the final report will write it;
  - `record_sha256`.
  One line MUST NOT exceed `maxOutputBytes`. A step whose record would exceed it is journaled
  without `record` and is not replayable. The final report MUST still be written as today. The
  journal replaces `OUTPUT.partial.json`, and the dispatcher MUST NOT write that file any more.
- `TRJ-V0-005`: A step MAY be replayed only from an entry with an equal key, `replayable: true`,
  `outcome: FINISHED`, an observed wall time and a `record_sha256` that matches its `record`. A
  `FAILED` entry is kept but MUST never be replayed, so resuming retries it. When more than one
  entry is eligible for a key, the earliest line wins.
- `TRJ-V0-006`: When reading a journal, a final line without its newline terminator MUST be ignored
  with one warning: that is a torn append from a killed run. Any other unparseable line, unknown
  schema or digest mismatch MUST refuse the run before any agent invocation, naming the line
  number. The run MUST NOT rewrite or truncate the journal.
- `TRJ-V0-007`: `--resume` MUST read `OUTPUT.journal.jsonl`. `--reuse SOURCE` MUST accept a journal,
  or a final report whose records carry `replay_key` and `record_sha256` (TRJ-V0-008), and MUST
  match by key alone. The profile, model and access pre-checks in today's `loadReuse` become
  members of the key rather than separate gates. A source MUST be read line by line or record by
  record under the per-line bound, never whole under a file-size bound. This lifts the 8 MiB
  ceiling for replay sources only; scoring still reads the final report under `maxOutputBytes`. A report without
  `replay_key` MUST NOT be replayed (see Unresolved decisions).
- `TRJ-V0-008`: Every record in a final report MUST carry its `replay_key` and `record_sha256`. A
  replayed record MUST copy the original observations unchanged and add `replayed_from`:
  `{source_sha256, key, record_sha256}`, where `source_sha256` is always the source file's sha256.
  `reused_from` MUST keep each tool's current meaning for existing consumers: the source file's
  sha256 in cw-trial, and the source report's `manifest_sha256` in cem-trial. The report MUST add `live_invocations` and
  `replayed_invocations`, and MUST report token totals for live and replayed invocations
  separately, so a rerun's own cost is never inflated by replayed tokens.
- `TRJ-V0-009`: The report MUST list every distinct replay source as `replay_sources`:
  `[{sha256, kind: journal|report}]`. A focused test over `benchmarks/results/` MUST fail when a
  report carrying `replay_key` lists a source whose sha256 is not a tracked file, so a committed
  replayed observation always has a retained source. Reports without `replay_key`, including the
  26 measured above, are exempt and keep their provenance gap visible.
- `TRJ-V0-010`: Matching MUST be by key, never by position. With `--workers` above 1 the replay
  result MUST be identical for every completion order.
- `TRJ-V0-011`: The key's `upstream` MUST list the `{key, record_sha256}` of every step whose output
  feeds this step's prompt or inputs, sorted. Both trial dispatchers have no such steps, so
  `upstream` MUST be `[]` for them. A future dispatcher with dependent steps MUST declare them
  here. Replay by run position, as in pi-herdr, is not admitted.
- `TRJ-V0-012`: The journal is private derived state of the trial tools. No `corvint` command, and
  no ranking, evidence, learning or authority path, MAY read it (`AGENTS.md` invariant 4), and
  `corvint` read commands MUST NOT write it. Scoring and `rescore` MUST read only the final report,
  so a rescore stays byte-identical whether or not a journal exists.

## Non-goals and simpler baseline

Non-goals:

- reusing deterministic gate steps (owned by `gate-ledger-v0.md`);
- a shared or cross-user cache;
- a Core product command or MCP surface;
- relaxing cw-trial's `--access none` admission;
- replaying Tasks attempts or post-merge stages;
- automatic retry of failed invocations;
- positional prefix replay;
- changing what makes a CWT or CRT run invalid.

Simpler baseline: add the recorded `agent` identity to each tool's existing `loadReuse` check. That closes the silent-reuse holes for about ten lines per tool. It does not
fix:

- the unretained provenance (TRJ-V0-009);
- the 8 MiB reuse ceiling;
- the quadratic checkpoint rewrites;
- two implementations drifting apart, which has already happened.

If the owner rejects this spec, the baseline should still land as a CWT and CRT amendment.

## Trust boundary, limits and failure modes

The journal is a local file the operator controls, written next to the report with the same mode.
It can contain agent replies, exactly as reports already do. It is trusted only as far as its
`record_sha256` digests go: those detect corruption, not a malicious operator editing a record and
its digest together. That exposure is the same as editing a report today, and review of committed
sources (TRJ-V0-009) is the control.

| Failure | Behaviour |
|---|---|
| Run killed mid-append | Torn final line ignored with a warning; finished steps replay (TRJ-V0-006). |
| Corrupt or edited line | Run refuses before any invocation, naming the line (fail closed). |
| Runner, effort, model, access, sandbox or argv changed | Key differs; step runs live. |
| Key member `NOT_OBSERVED` | Step runs live, journaled `replayable: false`. |
| Source report has no `replay_key` | Nothing replays from it; the run pays in full. |
| Record exceeds the line bound | Journaled without a record and not replayable. |
| Disk full while appending | Same as today's checkpoint write failure: the run continues; that step is not resumable. |

## Acceptance criteria and test matrix

| Requirement | Planned focused test |
|---|---|
| TRJ-V0-002, 003 | `TestTRJV0002_KeyChangesWithEachMember`, `TestTRJV0003_KeyIgnoresPathsHostTimeAndWorkers` |
| TRJ-V0-002 | `TestTRJV0002_NotObservedMemberIsNotReplayable`, `TestTRJV0002_ScriptFixtureReplaysInBothTools`; a changed codex version, effort, codex config file or script digest forces a live run in both tools |
| TRJ-V0-008 | `TestTRJV0008_ReusedFromKeepsEachToolsMeaning` |
| TRJ-V0-004, 006 | `TestTRJV0004_AppendsOneLinePerSettledStep`, `TestTRJV0006_TornTailIgnored`, `TestTRJV0006_CorruptLineRefusesBeforeInvocation` |
| TRJ-V0-005 | `TestTRJV0005_FailedEntryRetries`, `TestTRJV0005_EarliestEligibleWins` |
| TRJ-V0-007, 008 | `TestTRJV0007_ReuseFromKeyedReport`, `TestTRJV0007_LegacyReportReplaysNothing`, `TestTRJV0008_LiveAndReplayedTotalsSeparate` |
| TRJ-V0-009 | `TestTRJV0009_CommittedReplaySourcesAreTracked` over `benchmarks/results/` |
| TRJ-V0-010 | `TestTRJV0010_WorkerOrderDoesNotChangeReplay` |
| TRJ-V0-012 | Existing `TestScoreRebuildsTheReportByteIdentically` (cem-trial) and `TestTrialFinalizeAndScoreAreByteIdentical` (cw-trial) pass with and without a journal present |

Each test runs with a `script` agent fixture, so no model is called. A recorded development rerun
that measures live versus replayed invocations and wall time is evidence for promotion, not for
implementation.

## Rollout, rollback, compatibility and drift

Both dispatchers bump their report profile to `/1` in the same change, because records gain fields,
and both readers accept `/0` and `/1`. cw-trial's `readReport` currently refuses any other profile,
so a full revert would leave `/1` reports unscoreable. Rollback therefore reverts the writer, the
shared package and the replay flags' new behaviour, and keeps the reader's `/1` acceptance;
journals are then simply ignored. CWT-V0-004 and CRT-V0-010 are amended in the same
change, to point at this spec for the mechanics. Any later change to the key's members bumps
`corvint-trial-replay-key/0`, which deliberately invalidates every older journal.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| TRJ-V0-001..012 | not started | none; this is a proposal |

## Unresolved decisions and promotion or kill criteria

- **Legacy reports.** 1,397 committed observations were produced under the old match rule. Should
  a one-time `--reuse-legacy` keep the exact `/0` rule, labelled `reused_legacy`, so that existing
  development runs need not be paid again? The default above is no, and pays once.
- **Access.** Replay under cw-trial `--access` other than `none` would need `inputs` to bind the
  materialised snapshot's Git tree id, plus evidence that the sandbox confines reads to it. That
  needs its own CWT amendment.
- **Environment.** `ambient` covers only the codex config files. Running the agent with a minimal
  declared environment, and putting that environment in the key, would close the remaining gap,
  but it changes how codex authenticates and finds its home. Is that worth doing in V0?
- **Location.** Should the shared package live in `internal/` or `tools/internal/`? It must not be
  reachable from a `corvint` product command (TRJ-V0-012).

Promotion to `implemented` needs every test above passing, plus independent review. Promotion to
`validated` needs one recorded development rerun showing that `live_invocations` equals the number
of steps whose inputs changed. Kill: if the shared package plus its tests is larger than the two
implementations it replaces plus the simpler baseline, without closing TRJ-V0-009, ship the baseline
instead.
