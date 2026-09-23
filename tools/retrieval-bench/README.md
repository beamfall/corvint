# retrieval-bench

Runs [Agent Retrieval Bench](https://github.com/eyuansu62/agent-retrieval-bench) (arXiv
2607.24882: 427 samples, 25 repositories, frozen base commits, a selective subset that scores
abstention) against `corvint query`, `corvint context`, `corvint impact`, and `corvint affected` beside a
deterministic grep baseline and two stronger lexical baselines (`grep-ident`, `bm25`). This is the control arm the bet in
`docs/plans/BREAKTHROUGH-BET-2026-09-01.md` and audit finding F1 ask for. The tool downloads
nothing: it reads the bench's JSONL and corpus snapshots from disk, writes one report, and removes
the temporary snapshot copies it makes.

## Protocol

- **Query text** is the bench's own `query_text_for_eval`, byte for byte: the sample's `query`
  object as Python `json.dumps(..., sort_keys=True)` writes it. The query, context, and grep arms see it;
  the impact and affected arms see only `query.changed_file`. `corvint query` bounds a task at 2,000 characters, so the
  Corvint arm sees at most that prefix; every sample records `query_chars` and `query_truncated`.
- **Gold** follows the bench's `target_gold_files`: `gold.files` when present, else
  `related_tests` (code2test), `must_context_files`/`context_files` (comment2context), else
  `root_cause_files`, else `related_tests`. `gold.no_gold: true` means no gold. A sample with neither gold nor that label is skipped and
  counted under `skipped.no_gold_unlabeled`, as the bench's selective evaluator skips it.
- **Given** context (`gold.given_files`, or the reviewed `path`/`given_file` for
  comment2context) is removed from every arm's ranking and is never a success.
- **Snapshots.** A release ships each base commit as one chunk file
  (`corpus/<release>/OWNER__NAME/COMMIT.chunks.jsonl`) whose `kind: file` rows carry every
  file's full text; the tree the bench evaluated over is rebuilt from those rows in a temporary
  directory and committed once, so every arm sees exactly the bench's corpus. A `--snapshot` may
  also name a directory: a Git worktree must be clean at the base commit, and it is copied like a
  plain tree rather than used in place, so no arm can write into the caller's snapshot (an
  unsupported `impact` call would otherwise append to its `.corvint/self-observations.jsonl`). Temporary copies are removed when the run ends, or
  on SIGINT/SIGTERM; a snapshot is never modified.
- **Corvint arm:** one `corvint query --task TEXT --limit K` per sample; the ranking is each
  result's own path in packet order, distinct. `state: OUT_OF_SCOPE` or an empty packet is an
  abstention. A failed invocation is recorded as an error, not an abstention.
- **Context arm:** one `corvint context --task TEXT --limit K` per sample over the full query
  text. Unlike query, the harness does not truncate at 2,000 characters; context's own 32,000-character
  bound applies, and a longer input is an arm error. The ranking is each task-context result ID in
  packet order, distinct. `state: NO_CANDIDATES`, an empty packet, or a packet whose
  `coverage.answerability.verdict` is `unsupported-conjunction` (TCP-V0-016; the verdict is
  appended to the arm's `state`) is an abstention; a failed invocation is an error, not an
  abstention.
- **Impact arm:** one `corvint impact CHANGED_FILE --limit K` per sample whose `query` carries
  a string `changed_file` (code2test samples do); the ranking is every evidence path in packet
  order, distinct, with the changed file itself removed (it is the query's own subject) along
  with the given files. `state: OUT_OF_SCOPE` or a ranking left empty is an abstention. A sample
  without `changed_file` records the error `no changed_file in query`, and a refusal (a path
  suffix the command has no reverse-import rule for, a repository without a Go module, a path
  not tracked at the base commit) is recorded as an error, never an abstention. `--limit` above
  50 is refused by the command itself.
- **Affected arm:** one `corvint --root COPY affected` per sample whose `query` carries a string
  `changed_file`. The command takes no path: the change is whatever is dirty, so the arm dirties
  the changed file in the temporary copy by appending two newlines (a dirty edit in every
  language), runs the command, and restores the file's exact previous bytes in a deferred step
  that also runs after a failure; the copy must then be clean under `git status --porcelain`, or
  the sample records an error. The ranking is every test path in `plan.selected[].tests[]` in
  plan order, distinct, with the changed file and the given files removed and truncated to k;
  the plan's `scope` (`BOUNDED` or `UNKNOWN`) is the arm's `state` and the plan's length its
  `packet_bytes`. A plan that selects no other test is an abstention, and the command exits 0
  for a clean worktree or an unowned language (an empty `selected` with an
  `UNOWNED_DIRTY_PATH` unknown), so those are abstentions too. A sample without `changed_file`
  records the error `no changed_file in query`; a non-zero exit (no resolvable HEAD, a status
  the command cannot read, a worktree that drifted while the plan was compiled), a changed file
  the copy does not hold, or a copy left dirty is an error, never an abstention.
- **Grep arm:** every text file under 1 MiB scored by the distinct query terms it contains
  (tokens as the bench tokenizes them: camelCase split, alphanumeric runs, lowercase, at least
  two characters; a term in the path counts double), ties broken by occurrences then path. It
  abstains only when no file matches any term. It is unchanged since the first run so old reports
  stay comparable; it is a control, not what an agent types.
- **Grep-ident arm** ("what an agent types into ripgrep"): the query terms are the identifiers in
  the query's string *values* only, never its JSON keys: every ASCII identifier of at least four
  characters with an inner underscore or a camel-case boundary, plus the stem of every code file
  name (`.go .py .rs .ts .tsx .js .java .kt .rb .cs .cpp .c .h`), distinct and sorted. Every text
  file under 1 MiB is scored by the distinct identifiers it holds as whole words (case-sensitive,
  no identifier byte on either side) plus one for every identifier the lowercased path contains;
  ties break by occurrences then path. No identifier, or no file holding one, is an abstention.
  This rule is frozen here before any tuning; it must never read JSON keys or `context`'s own term
  table.
- **BM25 arms** (`bm25:all`, `bm25:ident`): whole-file BM25 with k1 1.2 and b 0.75, fixed and
  never tuned, over the bench tokens of every text file under 1 MiB; inverse document frequency
  `ln(1 + (N - n + 0.5) / (n + 0.5))`, length normalised against the mean token count of the copy
  (given files stay in those statistics and are excluded from the ranking). `all` uses every grep
  term of the query text; `ident` uses the bench tokens of the grep-ident identifiers with no
  length floor. Ties break by path; no file scoring above zero is an abstention.
- **Metrics** per positive sample: `recall@5/10/20` (only those with k ≤ `--limit`), `mrr@k`
  (over the k-long ranking, so it is a floor on the bench's MRR), `precision@k` (hits over the
  answered length, at most k, as the bench divides), `f1@k`, `hit@k`, and `hard_negative_hits@k`
  (from `gold.negative_distractors`). Every sample carries
  `selective_success`: a no-gold sample succeeds by abstaining, a positive one by answering with
  a gold file in the top k. Strata: `positive`, `natural_no_gold` (`metadata.organic`), and
  `counterfactual_no_gold`, which the bench reports separately. A sample whose full query text
  carries at least one TCP-V0-022 repository anchor (`contextindex.TaskHasAnchors`, the five
  classes `context` extracts under `CORVINT_CONTEXT_ANCHORS=on`) records `anchor_bearing: true`
  (absent otherwise, so reports without anchors keep their bytes) and is also averaged under
  `stratum:anchor-bearing`, whatever the flag's value in the run. Means are reported over all
  samples, per task type, and per stratum (`n` counts the group, `positives` the samples the
  positive-only metrics average over), with 95% Wilson intervals for `hit@k` over positives
  and `selective_success` over everything. Each sample also carries `partition`, its repository's
  fold (`A`, `B`, or `unassigned`) from the frozen map in `benchmarks/README.md`, and means are
  reported per `fold:` group too.
- **Paired statistics** (`paired`): per group (`all`, `task:`, `fold:`, `task:/fold:`), for each
  retrieval arm (`corvint`, `context`) against each baseline arm present (`grep`, `grep-ident`,
  `bm25:all`, `bm25:ident`), the mean paired difference of `recall@5`, `recall@10`, `recall@20`,
  and `mrr@k` over the positive samples both arms scored, its bootstrap 95% interval (percentile
  method, 4000 resamples with 100 cut from each tail, one fixed-seed generator per interval so output is deterministic),
  win/loss/tie counts, and per-repository mean differences for repositories with at least five
  samples in the group. Recall is not a Bernoulli rate, so no Wilson interval is attached to it;
  a claim that one arm beats another is read from the interval's lower bound, never the point
  estimate.
- **Latency** (`latency`): every arm records its wall time per sample (`wall_ms`); the section
  reports, per arm and per cache-state label, nearest-rank p50 and p95, the maximum, and the
  median absolute deviation, never a mean, and `NOT_RUN` below 30 samples. The label is observed,
  not inferred: before each Corvint verb the harness checks the copy for `.corvint/index` and records
  `COLD_UNIQUE` when absent (no prime; every call rebuilds from the tree) or `PRIMED_SHARED` when
  present; the lexical arms are `NOT_APPLICABLE`, and reports written before this field are
  `UNRECORDED`. Tree copies are shared across a snapshot's samples, so the operating-system page
  cache is warm after the first sample, and the lexical arms read and tokenize the copy once on a
  snapshot's first sample (that sample's `wall_ms` carries the read); the label describes the
  Corvint cache only.
- **Registration** (`registration`): the samples file digest, the `corvint` digest (`NOT_RUN`
  when no Corvint verb was selected), the fold map digest, and the arms run. Record these before a
  first run; a run whose registration differs is a different run.
- **Not measured:** budgeted context yield (needs the bench's chunk tokenizer) and token cost;
  `packet_bytes` per query, context, impact, and affected answer is the context-cost proxy.

## Running

Fetch a release from the bench's Hugging Face dataset (`eyuansu71/agent_retrieval_bench`,
`releases/v2_<task>/agent_retrieval_bench_v2_<task>.tar.zst`; `v2_code2test` is 444 MB and
3.7 GB extracted) and extract it; the JSONL sits under `benchmark/<release>/` and the chunk
files under `corpus/<release>/`. The `v2_code2test` release holds 106 positive samples and no
no-gold samples; abstention is measured by the `v2_abstention` and
`v2_selective_retrieval_*` releases, which this tool reads the same way. Then:

```console
go build -o /tmp/retrieval-bench ./tools/retrieval-bench
go build -o /tmp/corvint ./cmd/corvint
/tmp/retrieval-bench --samples benchmark/v2_code2test/code2test.jsonl \
  --corpus corpus/v2_code2test --corvint /tmp/corvint \
  --output benchmarks/results/agent-retrieval-bench-code2test.json
```

`--snapshot OWNER/NAME@COMMIT=PATH` names a snapshot explicitly; `--task-type` and
`--max-samples` bound a run; `--limit` is k (default 20, at most 50). `--arms` selects the arms
(`corvint`, `context`, `grep`, `grep-ident`, `bm25`, `impact`, `affected`; comma-separated or
repeated; default all); a selection without a Corvint verb spawns no `corvint` at all, so the
lexical baselines run on any machine that holds the corpus. The report is canonical JSON, byte
identical across reruns over the same inputs and binary except for the measured `wall_ms` values
and the `latency` section, and names the samples file digest and the `corvint` version and
digest it measured.

`--summarize REPORT` (repeatable, no `--samples`/`--corpus`) re-reads written reports, merges
their details by sample id (a later report's arm of the same name replaces an earlier one's),
refuses reports over different samples files or naming an arm the bench does not define, and
rebuilds every summary section, so a report written before the paired, fold, latency, or
registration sections gains them, and a lexical-only run can be paired with an earlier `context`
run over the same samples:

```console
/tmp/retrieval-bench --samples benchmark/v2_trace2code/trace2code.jsonl \
  --corpus corpus/v2_trace2code --arms grep-ident,bm25 --output lexical-trace2code.json
/tmp/retrieval-bench --summarize first-run-trace2code.json --summarize lexical-trace2code.json \
  --output paired-trace2code.json
```

## Reading a result

The bet's kill criteria are stated before any run: Corvint must show a materially lower
confidently-wrong rate at non-inferior success. Here that is `selective_success` on the
`natural_no_gold` stratum (an answer on a no-gold sample is a confident wrong answer) read
beside `hit@k` on positives, both against the grep arm and both with intervals. A first run is
first-observation evidence under `benchmarks/README.md` and is preserved unrepaired.

The impact arm is read beside its `errors` count: an errored sample scores zero on every
metric, and most impact errors are structural (no `changed_file` in the query, a changed file
with no reverse-import rule, a repository without a Go module), so its means understate what
the command does on the samples it accepts. Read `hit@k` and `mrr@k` over the details whose
`arms.impact.error` is empty before comparing it with the other arms.

The affected arm answers only code2test-shaped samples (those with a `changed_file`) and is
read beside the impact arm: impact refuses every changed file its reverse-import rules do not
cover, while affected builds a multi-language unit graph and selects test files, so it answers
where impact errors. Its `state` is the plan's scope; `UNKNOWN` means the selector widened
(an unowned dirty path, a language frontier) and says nothing about the ranking's quality. Its
abstentions include changed files no language plugin owns, so compare `hit@k` over the details
whose `arms.affected.abstained` is false with the same slice of the other arms.

## Matched snapshot latency (proposed TCP-V0-021)

Add `--snapshot-latency --output REPORT.json` to measure an explicit cold and hit `context`
call for every sample. `context` must be selected. The bench materializes one private Git copy
per `(repo, base_commit)` and runs `index` once in each copy. It times cold calls with the scratch
snapshot temporarily moved outside the copy, restores it on every return path, and requires the
subsequent hit to return the same ranking, state, abstention and top score. Copy/index work is
outside the spans. Source corpus files and `.corvint-benchmark-cache` are never modified.

`details[].arms.context.wall_ms` is the observed hit, `cold_wall_ms` is the matched miss, and
`latency.context.OBSERVED_HIT` / `OBSERVED_MISS` report min, p50, p95, max and MAD (the existing
30-sample floor still applies). Hit/miss are the loader's opt-in stderr observation, not directory
presence: missing/malformed diagnostics or a warm miss fail the measurement. Use a binary with
`CORVINT_BENCH_SNAPSHOT_TRACE` support. Product packet bytes are unchanged. Both spans include the
CLI process, packet parse and result extraction; neither is an OS-page-cache cold claim.

The bench exclusively creates and syncs `REPORT.json.registration.json` before the first
retriever. Override the path with `--registration PATH`; a stdout-only latency run requires it.
Registration and report must name distinct files: equivalent paths and aliases refuse before
retrieval or writes, with another check before report publication. The file binds sample-file,
target-binary, bench-binary and frozen fold-map digests, selected IDs,
limit, arms, mode and ranking/storage environment. It survives failure; each rerun needs a new
path. All normal runs with `--output` also gain the pre-run sidecar. Binaries/environment are
checked again before the final report. Corpus runs require the owner's quiet-host schedule;
register the falsifier externally before execution as well. The L3 target is observed-hit p50
below 100 ms over full v2_code2test, with no ranking difference, at load below 20.

## Optional exact context diagnostics (experimental RBD-V0)

Add `--context-packets /absolute/new/capture.jsonl` with a context arm and `--output`
or `--registration` to retain the original bounded context stdout and stderr. The destination
must be new, outside corpus/snapshot inputs, and have no symlink component. The file is private
(mode 0600); existing artifacts are never overwritten. This cannot be combined with summarize.

The JSONL header has profile `corvint-retrieval-context-capture-v0`, the unchanged registration and
ordered invocation descriptors. Each invocation records zero-based sample/invocation ordinals,
sample ID, repository/base, ordinary/cold/hit phase, and actual task bytes/SHA-256. Stream fields
`base64`, `bytes` and `sha256` retain the exact available bytes. Stdout is COMPLETE or NOT_PRODUCED;
unavailable stdout has no payload/hash. Stderr is COMPLETE or TRUNCATED when observed. Parse status
is PARSED, MALFORMED or NOT_RUN, so malformed JSON can still be captured completely.

The terminal footer binds `invocations`, `total_bytes` (the entire encoded file), `prior_sha256`
(the exact preceding header/record lines) and COMPLETE/PARTIAL status. Validate sequence, descriptors,
counts, digest, complete lines and absence of trailing bytes before trusting capture completeness.
No footer means incomplete. COMPLETE attests retained diagnostic bytes only, independently of
packet meaning, report publication, or task success. A later report-output failure does not erase
those bytes.

Capture is limited to 1,024 invocations, a 1 MiB header, 64 KiB record metadata, existing 8 MiB
stdout/64 KiB stderr per call, and 64 MiB for the entire encoded file. Capture integrity/write/budget
failure stops normal report publication and preserves the artifact. Disk writes happen outside
retrieval timing; bookkeeping still has overhead. Default report, registration and scoring remain
unchanged. Do not use capture-on/off timing to claim a speed improvement.
