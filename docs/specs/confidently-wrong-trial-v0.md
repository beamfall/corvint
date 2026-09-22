# Confidently-Wrong Trial V0

Owner: Russell Lewis
Date: 2026-09-01
Requirement prefix: `CWT-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/plans/BREAKTHROUGH-BET-2026-09-01.md` ("How this beats the
competition, measurably"; "Ninety days"), `benchmarks/README.md` (partitions, first-observation
rule, the external-dispatcher clause), `tools/retrieval-bench` (arms, snapshot materialisation,
Wilson intervals, canonical output), `AGENTS.md` invariants 2 and 4.

## Agent digest
- Claim: `tools/cw-trial` runs one agent over a frozen task set under `none`, `grep`, and `corvint` arms and scores confidently-wrong claims per task with intervals.
- Status: proposed/experimental
- Exists: `tools/cw-trial` (`run`, `score`), `tools/cw-trial/testdata/pilot/tasks.json` (five code2test tasks, `partition: pilot`), `benchmarks/results/cw-trial-pilot-first-run.json`.
- Blocked on: the judged run's first observation over `tools/cw-trial/testdata/heldout-v1` under `--access none` with the pilot's model (decision 0022).
- Read next: Requirements (claim grammar, definition, arms); Non-goals; Promotion or kill.

## Intent and scope

The bet is judged on confidently-wrong actions per task across three arms, and nothing in the tree
measured one: `tools/retrieval-bench` scores rankings, `benchmarks/dogfood_measure.py` keeps
outcome metrics `NOT_OBSERVED`, and `benchmarks/README.md` says only an external agent-harness
dispatcher may supply session measurements. This slice is that dispatcher plus its scorer.
Affected user: the repository owner reading the bet's kill criteria. Measurable job: for one
frozen task set, produce per arm the task-success rate, the confidently-wrong rate, the abstention
rate, and the token cost, each with an interval, from replies an agent gave inside a disposable
copy of the task's repository at its pinned revision.

Definition encoded: a **confidently-wrong action** is a claim in the agent's reply that names a
test file, source file, or answer the task's frozen gold marks false, with confidence `certain`.
The claim grammar exists so that judgment is mechanical; an agent that does not emit the block
has committed to nothing and is scored as abstaining.

## Requirements

- `CWT-V0-001`: Claim grammar. Every reply MUST end with one fenced JSON block
  `{"claims":[{"kind":"test-file|source-file|answer","value":"...","confidence":"certain|likely|unsure","evidence":"..."}]}`.
  The scorer takes the last fenced block whose text contains `"claims"`, or a reply that is itself
  one JSON object naming `"claims"`. No such block is `ABSENT`; a block that does not parse into a
  `claims` array is `MALFORMED`; a claim outside the grammar (unknown kind or confidence, empty
  value) is `INVALID` and never counts as an action. Path values are compared after trimming
  whitespace, a leading `./`, and a trailing `/`; `answer` values compare case-insensitively.
- `CWT-V0-002`: Verdicts. A claim whose kind the task's gold covers is `TRUE` when its value is
  in gold, else `FALSE`; a claim of a kind gold does not cover is `UNJUDGED`. Per task and arm:
  `success` is 1 when any claim is `TRUE`; `confidently_wrong` counts `FALSE` claims with
  confidence `certain` (and `confidently_wrong_task` is 1 when that count is positive);
  `wrong_likely` counts `FALSE` claims with confidence `likely`; `abstained` is 1 when the reply
  holds no valid claim (`ABSENT`, `MALFORMED`, or an empty array). `UNJUDGED` and `INVALID`
  claims never move success or the confidently-wrong count in either direction. Placement is
  scored beside the claims: `context_failed` is 1 when the producer failed (and the task is
  otherwise scored as a no-context task, never pooled silently); otherwise `gold_in_context` is
  1 when any gold value appears anywhere in the context text and `gold_as_result` is 1 only when
  a gold value is a result row (a `results[].id` or result evidence path of a packet, or the path
  field of a plain listing line), so a packet that merely mentions the gold in an exclusion or
  unparsed sample does not count as having supplied it. `utilisation_gap` is 1 when
  `gold_as_result` is 1 and `success` is 0: the context supplied the gold as a result row and
  the agent still did not claim it, so the failure is the agent's use of the context, not the
  retrieval.
- `CWT-V0-003`: Arms. `none` supplies the task text only; `grep` adds the top-K paths of
  `tools/retrieval-bench`'s term-overlap scorer reimplemented in this tool (distinct task terms
  per file, a path match counts double, ties by occurrences then path; K = `--limit`, default 20,
  at most 50); `corvint` adds the `corvint` task-context packet verbatim (task-context-packet-v0):
  `context --task TEXT --limit K`, with `--subject CHANGED_FILE` for a `change` task; `aider`
  adds the rank listing of an independently provided aider repository-map executable
  given by `--aider-command` (the shipped Python bridge is retired under GOC-V0-008): a personalised PageRank over identifier
  definitions and references seeded with the subject path and the task's code-shaped
  identifiers, the same seeds the corvint arm gets, top-K files as `rank=N PATH` lines, the
  subject excluded, run inside the copy with twice the agent timeout. Every arm receives the one
  prompt skeleton, which states that a path the task presents as its own subject is not one of its
  answers; only the arm name, the arm's fixed prologue, and the context body differ. The
  `corvint` prologue states that each row's `action` says what to do with the file and is to be
  acted on, that a row's evidence `confidence` bounds the claim it can back
  (`high` may back `certain`; `medium` or `low` at most `likely`) and that a `NO_CANDIDATES`
  packet backs at most `unsure`. The
  context body is bounded at 64 KiB in every arm and truncation is recorded. A context producer
  that fails does not skip the arm: the failure text becomes the context and `context_error` is
  recorded, so the agent is dispatched the same number of times in every arm. The arms differ in
  supplied context, never in access: `--access` (CWT-V0-010) applies to every arm alike.
- `CWT-V0-004`: Dispatch. One invocation per task per arm, inside the task's materialised copy
  under `--access read-only` or inside an empty directory under `--access none` (CWT-V0-010),
  never in the source snapshot. The `codex` agent runs `codex exec --json --ephemeral -s read-only
  --skip-git-repo-check -C DIR -m MODEL [-c model_reasoning_effort=E] -o REPLY PROMPT` with stdin
  closed; the reply is the `-o` file (falling back to the last `agent_message` event), token
  counts come from the `turn.completed` usage event, and `tool_calls` counts the completed
  `command_execution`, `file_change`, `mcp_tool_call`, and `web_search` items (their command
  text kept under `commands`, bounded); a `script` agent's `tool_calls` is `NOT_OBSERVED`. A `script` agent is any executable given the
  prompt as its argument that replies on stdout. Wall time is measured by the dispatcher; the exit
  code and token counts are recorded only when the CLI reports them and are otherwise the string
  `NOT_OBSERVED`, never zero. A timeout or launch failure is recorded as the arm's `error`; the
  task counts under `errors` and scores nothing. So is an observed non-zero exit code whose reply
  holds no well-formed claims block (`agent exited with status N and left no claims block`): a
  crashed or refused invocation is an error, never an abstention, and `score --report` applies the
  rule to the recorded exit code; a non-zero exit that still left a well-formed block is scored.
  A reply cut at the reply bound (`reply_truncated`) whose claims block did not survive is an error
  too (`reply exceeded the reply bound and its claims block was cut`), never an abstention.
  Under `--access none` only: `--workers N` runs up to N invocations concurrently, each in its own
  empty directory, and changes timings and nothing else because records are written only by their
  own invocation and the report keeps manifest order; `--reuse REPORT` copies the reply and observations of any arm whose prompt
  sha256 is byte-identical in a prior report of the same profile, model, and access and whose
  record is not an error under this clause (read from the recorded exit code, so an older report's
  failed invocation without an `error` reruns), marking the
  record `reused_from` with that report's sha256, so a change to one arm's context reruns only
  that arm. A run with `--output` writes the report so far to `OUTPUT.partial.json` (atomically,
  marked `partial`, unscored) after every finished invocation and removes it when the final
  report is written; `--resume` reuses that checkpoint, and only a record whose wall time was
  observed is reused, so a killed run keeps every finished invocation and reruns the rest.
- `CWT-V0-005`: Materialisation. A snapshot is a bench chunk file
  (`--corpus/OWNER__NAME/COMMIT.chunks.jsonl`, `kind: file` rows only, unsafe paths skipped) or a
  task's explicit `snapshot` directory (copied, `.git` excluded). It is rebuilt in a temporary
  directory and committed once so every arm sees one immutable tree; the source is never written
  or used in place; the copies are removed when the run ends. With `--history DIR` (a local Git
  repository), a task whose `base_commit` that repository holds is instead materialised as a
  shared, no-checkout clone checked out detached at the base commit, so the copy carries every
  commit behind it and history-reading slots (decision 0025) can run; the clone reads the
  source's objects and writes nothing into the source (no worktree entry, no object); the record
  carries `history_commits`; a task whose base commit is unknown there falls back to the rebuild.
- `CWT-V0-006`: Identity. The report MUST record `profile` (`corvint-cw-trial/0`), `partition`,
  `pilot` (true iff the partition is `pilot`), `tasks_sha256` of the manifest bytes, `model`,
  `access` (`read-only` or `none`; a report without the field is read-only), the agent identity (`kind`, `version`, the command shape, `effort` or `NOT_OBSERVED`), the
  `corvint` version and sha256 (`NOT_OBSERVED` when the corvint arm did not run), `skeleton_sha256`,
  and per arm the prologue text and its sha256; per task and arm the prompt's sha256 and byte
  length, the context, the full reply (bounded at 1 MiB, truncation recorded), and every claim
  with its verdict and whether its `evidence` string appears in the supplied context
  (`NOT_APPLICABLE` when no context was supplied).
- `CWT-V0-007`: Summary and determinism. Per arm the report carries `tasks`, `errors`, and 95%
  Wilson intervals (`n`, `count`, `rate`, `low`, `high`) for `success`, `confidently_wrong_tasks`,
  `abstained`, `gold_in_context`, `gold_as_result`, and the CWT-V0-013 rates over the scored
  tasks, `mean_f1` (`n`, `mean`), plus the
  `context_failed`, `utilisation_gap`, and `reused` counts, `confidently_wrong_claims`, `wrong_likely_claims`,
  `wall_ms`, summed `tokens` (or `NOT_OBSERVED` when any task lacks them), summed `tool_calls`,
  and `explored_tasks` (invocations with at least one tool call; both `NOT_OBSERVED` when any
  scored invocation did not report them). Output is canonical
  JSON (`gokernel.CanonicalJSON`), byte-identical over identical inputs; `cw-trial score --report`
  rebuilds every derived field from the raw ones and reproduces a report's bytes unchanged.
- `CWT-V0-008`: Task manifest. `partition` is required; a task needs `id`, `kind`
  (`retrieval` or `change`), `text`, `gold` keyed by claim kind, and `changed_file` for a change
  task; `repo` and `base_commit` locate the corpus snapshot unless `snapshot` is explicit; a
  manifest that lists one task id twice is refused. A
  `pilot` partition is never held-out evidence and never feeds the bet's verdict. The tool cannot
  verify that a held-out set was frozen before its author saw Corvint output; it records only the
  manifest digest, and the first-observation rule in `benchmarks/README.md` governs the rest.
- `CWT-V0-010`: Access. `--access read-only` (the pilot's mode) lets the agent read the copy in
  every arm, so a capable agent finds gold by exploring and the arms measure nothing (the pilot:
  5/5 in each arm). `--access none` is the judged mode (decision 0022): for each task every arm's
  context and prompt are produced from the copy first, the copy is then removed, and each arm is
  invoked from one fresh empty directory under a second skeleton that states the repository is not
  available and forbids commands; `skeleton_sha256` is that skeleton's digest. The agent's shell
  is not disabled, so compliance is observed, not enforced: `tool_calls` and `commands` per
  invocation and `explored_tasks` per arm record every command the agent ran, and a judged run's
  reading MUST state them. The mode is recorded as `access` and applies to every arm alike.
- `CWT-V0-011`: Held-out import. `cw-trial import-heldout --tasks tasks.jsonl --manifest
  manifest.json [--chunk-root DIR]` maps the frozen set under `tools/cw-trial/testdata/heldout-v1`
  onto a run manifest one field to one field (`mode` to `kind`, `repository` `OWNER/NAME@COMMIT`
  to `repo` and `base_commit`, `gold` and `gold_kind` to the gold map, the sample's verbatim query
  fields to `text` by release, the row's `source` carried verbatim), checks the file's sha256
  against the manifest's `tasks_sha256` and, with `--chunk-root`, every row's chunk-file digest,
  refuses any row it cannot map, and never writes the frozen files. The produced `source` names
  the frozen set and its digest. The text names the gold kind so a path claim is judged.
- `CWT-V0-012`: Unseen set generation. `cw-trial generate --repo DIR --name OWNER/NAME --since
  DATE --output DIR [--limit N] [--min-files N] [--max-files N]` builds change tasks from a local
  Git history without any download: every non-merge commit after the date that touched between
  the bounds of text files, at least one a modified non-test source file, becomes one co-change
  task whose subject is the modified non-test source file with the largest diff and whose gold is
  every other touched file (`test-file` or `source-file` by path) plus the subject's
  same-directory test counterpart by stem when the parent tree holds one (`test-file`, counted
  under `counterpart_gold_paths`, so the set answers "what to read", not only "what changed");
  the text carries the commit
  message as the intent and the subject's hunks bounded at 8 KiB; the parent tree is written whole
  as `corpus/OWNER__NAME/PARENT.chunks.jsonl`; candidates are selected by the released sets' rule
  (index floor(i*N/n) in commit order); the manifest records the population, the sampling rule,
  and how many gold paths the commit created (`novel_gold_paths`), which no model can have seen.
  A repository younger than the model's training data, such as this one, is the intended input;
  the none arm's success on such a set is the memorisation check the first observation could not
  make.
- `CWT-V0-009`: No verdict field. The report carries the rates the bet's kill criteria read
  (`confidently_wrong_tasks` and `tokens` for `corvint` against `grep`, `success` for
  non-inferiority) and never a pass/fail judgment; that reading is human, over a non-pilot
  partition, and recorded in `docs/BUILD-LOG.md`.

- `CWT-V0-013`: Retrieval-native metrics (decision 0034). Beside the agent's outcome, each
  scored arm carries metrics derived from the record alone, so `score` reproduces them:
  `packet_top_1`, `_3`, `_5`, `_10` (a gold path among the context's first k rows, read in the
  consumer's order: a packet's `results[].id`, else the path of each `key=value PATH` listing
  line, the subject skipped); `f1` of the arm's distinct valid claims against the gold paths (a
  claim is correct only when it is `TRUE` and names a gold path, so a `TRUE` `answer` claim never
  lifts `f1` above 1);
  `hard_gold_in_context` and `hard_success` (a context row, or a `TRUE` claim, that is gold beyond
  the subject's stem, `testStem` with a trailing `tests` also dropped, so the counterpart a naming
  convention guesses does not count), present only when the record carries `subject`; and
  `success_retrievable` (`success` over the tasks with at least one gold path in the copy at the
  base), present only when the record carries `gold_checked`. A run records `subject` and
  `gold_at_base` at dispatch; `score --tasks FILE --history DIR` fills them for an older report
  and never overwrites what a run observed. The summary carries each as a Wilson interval and
  `mean_f1` as a mean; a metric no record carries reports `n` 0. A committed report that carries
  these metrics MUST reproduce byte for byte under a flag-free `score` of the committed tool
  (decision 0211). `cw-trial-unseen-beamfall-apple-run-1.json`, `-apple-run-2.json`, and
  `cw-trial-unseen-beamfall-run-1.json` do not: their `grep` arm's `packet_top_1/3/5` were
  scored without skipping the subject, so they are non-reproducible, kept as recorded, and cited
  only with their rescored values beside them.

- `CWT-V0-014`: Invocation lifecycle (proposed). On Darwin and Linux, `runCommand`
  MUST use the shared `internal/procgroup` supervisor to own the invocation's process group.
  Cancellation, deadline and normal leader exit MUST terminate remaining members before
  reaping the leader, bound pipe draining and cleanup to five seconds, and refuse an incomplete
  exit or cleanup observation as an invocation error. Real runner INT/TERM signals MUST reach
  this cancellation path. Observed normal and nonzero exit codes retain their existing shapes;
  timeout and cancellation remain errors. The invocation receives EOF stdin and the inherited
  environment; an empty or relative working directory and a PATH or root-relative executable
  resolve to absolute paths before dispatch.
  Capture retains the first 8 MiB of stdout and 64 KiB of stderr while draining excess bytes;
  truncation alone MUST NOT stop an invocation or change its observed exit. Shared supervisor
  callers that omit the explicit truncation policy retain fail-on-overflow behavior; a zero
  stderr limit inherits the stdout limit, and an unknown policy refuses before spawn.
  This qualifies only the owned group: a child that changes process group or session, and
  other platforms, remain unsupported. Other trial subprocess helpers are outside this slice.
  The clause does not qualify hostile external execution or change a frozen outcome trial.

Simpler baseline: `tools/retrieval-bench`'s ranking metrics, which cannot see what an agent
commits to. This slice measures the agent's action, not the retriever's list; CWT-V0-013 reports
both, because a metric saturated by the counterpart guess cannot separate retrievers (BUILD-LOG
2026-09-02).

Trust boundary and resource limits: the tool trusts the manifest, the corpus, the `corvint` and
agent executables it is given, and the local Git; it downloads nothing. Manifest 16 MiB; chunk
line 64 MiB; grep files 1 MiB each; context 64 KiB; reply 1 MiB; agent stdout 8 MiB; `corvint`
call 2 min; agent call `--timeout` (default 5 min); one process at a time.

## Non-goals and authority

Authoring the fifty-task set (human-owned, frozen before any Corvint output); running arms in
parallel; a `prove --base` change arm (needs a diff the pilot tasks do not carry; deferred); any
agent CLI beyond `codex` and the `script` adapter; a judge model; multi-turn dialogue; enforcing a
token budget beyond the equal context bound; declaring the bet won or dead.

## Failure modes

`codex` or `corvint` missing: the run fails before the first task, no report. Agent timeout:
`error` on that arm, the task counts under `errors` and is excluded from the rates. Reply without a
block: `ABSENT`, abstained. Unparseable block: `MALFORMED`, abstained. `corvint` refusal (a
changed file with no reverse-import rule, no Go module): `context_error` on the corvint arm, the
agent still runs with the failure text as its context. Missing snapshot: the run fails at that
task, no partial report. An agent that tries to write: the sandbox is read-only and the copy is
disposable; the source snapshot is never reachable.
A committed report whose derived fields came from a scorer other than the committed tool: the
committed-report reproduction test fails, and the report is either rescored before commit or
listed as non-reproducing with its rescored values cited beside the recorded ones (decision 0211).

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| CWT-V0-001 | `extractClaims`, `claimsBlock`, `verdict`, `normalize` in `tools/cw-trial/main.go` | `TestExtractClaimsPresentAbsentMalformed`; `TestJudgeClaimsGroundsEvidenceInContext` |
| CWT-V0-002 | `scoreArm`, `judgeClaims` | `TestScoreArmSuccessConfidentlyWrongAbstain` (success, confidently wrong with likely/unjudged/invalid, abstain by silence, empty block, malformed) |
| CWT-V0-003 | `skeleton`, `prologues`, `buildPrompt`, `contextBody`, `grepBody`, `aiderBody`, `corvintPacket` | `TestTrialWithScriptAgentScoresEveryArm` (none carries no context, grep lists the gold hit and skips binaries, prompt digests differ); `TestAiderArmRunsTheProducerWithTheSameSeeds`; pilot run `benchmarks/results/cw-trial-pilot-first-run.json` (corvint arm) |
| CWT-V0-004 | `codexAgent`, `scriptAgent`, `runCommand`, `failedInvocation`, `dispatchTask`, `invokePending`, `checkpointer`, `loadReuse`, `reuseSource.apply` | `TestParseCodexEventsReadsUsageAndLastMessage`; `TestTrialWithScriptAgentScoresEveryArm` (tokens `NOT_OBSERVED`, wall time observed); `TestTrialCountsANonZeroExitWithoutClaimsAsAnError`; `TestTrialReusesIdenticalPromptsAndRunsWorkersUnderNoAccess`; `TestTrialCheckpointsEveryInvocationAndResumesOnlyTheUnfinished`; pilot run (codex tokens observed) |
| CWT-V0-005 | `workspaces`, `materialize`, `materializeHistory`, `commitKnown`, `writeChunkRows`, `copyTree`, `safeRelative` | `TestMaterializationNeverTouchesTheSnapshot`; `TestChunkFileMaterializesOnlyFileRows`; `TestHistorySnapshotCarriesCommitsAndLeavesTheSourceUntouched` |
| CWT-V0-006 | `trial`, `prologueIdentity`, `corvintIdentity`, agent `identity` | `TestTrialWithScriptAgentScoresEveryArm` (digests, pilot flag, corvint `NOT_OBSERVED`) |
| CWT-V0-007 | `finalize`, `summarizeArm`, `wilson`, `sum`, `goldPlacement`, `resultRows`, `rescore`, `write` | `TestTrialFinalizeAndScoreAreByteIdentical`, `TestGoldPlacementSeparatesResultRowsFromMentions` |
| CWT-V0-008 | `readManifest`, `validateTask`, `resolveSnapshot` | `TestRunOptionsRejectIncompleteArms`; `testdata/pilot/tasks.json` carries `partition: pilot` |
| CWT-V0-009 | the report struct has no verdict member | this spec; BUILD-LOG 2026-09-01 entry reads the pilot without a verdict |
| CWT-V0-010 | `skeletons`, `skeletonNoAccess`, `dispatchTask`, `prepareArm`, `invokeArm`, `workspaces.discard`, `parseCodexEvents` (`toolItemTypes`), `summarizeArm` (`tool_calls`, `explored_tasks`) | `TestAccessNoneRemovesTheCopyBeforeDispatch` (grep context from the copy, empty cwd, no-access skeleton digest, `NOT_OBSERVED` tool calls for a script agent), `TestParseCodexEventsReadsUsageAndLastMessage` (a completed command is one tool call, an in-progress one is not) |
| CWT-V0-011 | `importHeldout`, `heldoutToManifest`, `heldoutTask`, `heldoutText`, `verifyChunk` (`tools/cw-trial/heldout.go`) | `TestImportHeldoutMapsRowsOneToOne`, `TestImportHeldoutRefusesWhatItCannotMap` (five refusals and the digest check), `TestImportHeldoutVerifiesTheChunkFileDigest` |
| CWT-V0-013 | `retrievalMetrics`, `orderedRows`, `hardGold`, `hardStem`, `goldPresent`, `f1`, `mean`, `backfillRecords`, `dispatchTask` (`subject`, `gold_at_base`) | `TestRetrievalMetricsReadTheContextRowsAndTheHardGold`; `TestTrialFinalizeAndScoreAreByteIdentical` (the recorded fields reproduce); `TestCommittedReportsReproduceTheirRetrievalMetricsUnderScore` (every committed report carrying the metrics reproduces, except the three listed non-reproducing reports, which must still differ) |
| CWT-V0-014 | `runCommand`; `internal/procgroup.Run`, `OverflowPolicy`, `Spec.StderrLimit` | `TestRunProcessLifecycleMatrix`; `TestRunProcessTruncateOverflowPolicy`; `TestRunProcessDefaultOverflowPolicyStillTerminates`; `TestRunProcessRejectsInvalidOverflowPolicyBeforeSpawn`; `TestTruncateCaptureConsumesCrossingWrite`; `TestRunCommandPreservesExitShapes`; `TestRunCommandNormalizesPathsAndEnvironment`; `TestRunCommandRefusesBeforeDispatch`; `TestRunCommandTruncatesOutput`; `TestRunCommandCancellationKillsDescendant`; `TestTrialMainSignalsCleanOwnedAgentGroup` |
| CWT-V0-012 | `generate`, `generateManifest`, `candidateCommits`, `inspectCommit`, `generateTask`, `writeSnapshot`, `evenlySpaced` (`tools/cw-trial/generate.go`) | `TestGenerateBuildsCoChangeTasksFromLocalHistory` |

Compatibility and drift: the grep scorer duplicates `tools/retrieval-bench`'s rules on purpose
(the tool does not import that package); if the bench's tokenizer changes, both change in one
commit. The `codex exec` event names (`item.completed`/`agent_message`, `turn.completed`/`usage`)
were observed on codex-cli 0.149.0; a CLI that stops emitting them degrades to `NOT_OBSERVED`
tokens, never to zero.

Unresolved decisions: which model and reasoning effort the judged run uses (the pilot used
`gpt-5.6-sol` at `medium`); whether `likely` wrong claims join the kill criterion or stay a
side count; whether a `prove --base` arm replaces `impact` once tasks carry diffs; whether the
agent may explore the copy in every arm (the pilot allowed it: the arms differ in supplied
context, not in tool access).

Rollout: the tool ships under `tools/`; no binary, adapter, or benchmark manifest calls it.
The proposed lifecycle slice rolls back by disabling dispatch while reverting its adapters,
supervisor options and clauses together; existing frozen-run artifacts remain preserved.
Rollback: delete `tools/cw-trial`, `benchmarks/results/cw-trial-pilot-first-run.json`, this spec,
and its README/INDEX rows. Promotion or kill: promote to `implemented` when a non-pilot task set
of at least fifty tasks, authored and frozen under the first-observation rule, has run once with
every arm and its first report is preserved unrepaired; the bet's own kill criteria (a third fewer
confidently-wrong tasks than `grep` at non-inferior success and at most 25% more tokens) are read
from that report by the owner, never from the pilot.
