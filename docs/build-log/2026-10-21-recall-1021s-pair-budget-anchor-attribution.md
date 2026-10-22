# 2026-10-21 recall-1021s: subjectless pairs fill only free budget; trace2code drop attributed

Lane recall-1021s (pre-1.0 ticket sweep), base `ef2a01db5`. The requirements below are accepted
under the owner's standing auto-accept (2026-10-11). The V1-2288 shape (variant E with disclosure)
is the owner-delegated orchestrator decision of 2026-10-21.

## Problem

- V1-2288: frozen code2test recall@20 fell from the V1-0186/V1-0205 baseline 0.5116 to 0.4642
  and the abstention rate from 0.1707 to 0.0366. `2026-10-21-recall-1021k-code2test-attribution.md`
  (PR #1467) attributed it to subjectless counterpart tests displacing selected lexical tests
  (V1-0431 `de427c57f`, then the rank-relative rework `366c3756a` in #444).
- V1-2295: trace2code recall@20 moved 0.7937 (242fde6) to 0.8036 (8f4b866) to 0.7690 (b8fe75d)
  to 0.7640 (fc90abe and today's base).
- V1-1088: the index did not admit `.mts`/`.cts`, so the AMAP-V0-025 side-effect import closure
  failed closed on any TypeScript module of those suffixes even when it only read the table.

## Decisions

- TCP-V0-004 amended, TCP-V0-161 added: a subjectless counterpart test is promoted when already
  selected (unchanged), otherwise it only fills unfilled budget. One that does not fit is listed
  in the optional `coverage.omitted_pairs` member with reason `paired test omitted: budget`.
  TCP-V0-011's `withheld` count is unchanged.
- IDX-SNAP-V0-108: `.mts` and `.cts` join the text-only suffix allowlist (no symbol extractor).
- Analyzer schema `corvint-analyzer/143`. 140 is on main (V1-2285), and #1483, #1502 and #1509
  take 141. Batch #1505 takes 142. Batch 1021v renumbered it to `corvint-analyzer/146`, after
  #1509 (143), #1512 (144) and #1514 (145).
- V1-2295 is attributed and not changed here. The drop is the TCP-V0-022 anchors default flip in
  `366c3756a` (#444), not the pair rule. Choosing between the chi orientation gain and trace2code
  is an owner fork, so the baseline is not moved.

## Evidence

Frozen Agent Retrieval Bench v2 (`tools/retrieval-bench --arms context`, frozen samples) gives
r20/r10/r5 (abstention: rate).

| subset | base ef2a01db | this change | V1-0186 baseline 242fde6 |
|---|---|---|---|
| code2test | 0.4642/0.3962/0.3160 | 0.5022/0.4013/0.3160 | 0.5116/0.3994/0.2877 |
| trace2code | 0.7640/0.5165/0.4356 | 0.7640/0.5165/0.4356 | 0.7937/0.5083/0.4010 |
| abstention | 0.0366 | 0.1707 | 0.1707 |
| edit2ripple | 0.6336/0.5517/0.3937 | 0.6394/0.5517/0.4109 | 0.6293/0.5101/0.3621 |
| comment2context | 0.5062/0.3771/0.2687 | 0.5042/0.3771/0.2687 | 0.5042/0.3438/0.2562 |

- code2test differs from the 242fde6 baseline on one sample only, diffusers `c7ea7d6f`. The gold
  `tests/modular_pipelines/test_modular_pipelines_common.py` (rank 16 at 242fde6) is pushed out by
  five `docs/source/*/modular_diffusers/*.md` rows. A one-sample bisect of first-parent merges
  places them first at `76f7f2ac4` (#639, rc3-batch-2: the V1-0859 documentation share for the
  lexical fill). Its first parent `3dc863bd7` carries two docs rows, as 242fde6 does. The new
  code2test r20 of 0.5022 is that cause, and the re-baseline is recorded against it.
- trace2code: `d761a403f` (b8fe75d's first parent) gives 0.8036/0.4637/0.3779 in full, and
  b8fe75d gives 0.7690. b8fe75d with `CORVINT_CONTEXT_ANCHORS=off` gives 0.8036/0.5033/0.4059, the
  first parent's r20 exactly. Five pallets/click samples lose `src/click/core.py` to rows matched
  on quoted strings pasted from pytest output. This change with anchors off gives 0.7987, with one
  sample abstaining. The last step, 0.7690 to 0.7640, is gin `2097c0a5` losing `path.go` at the
  same #639. The 242fde6 to 8f4b866 gain is gin `ddc4c496`, which pairs moved off abstention.
- chi orientation corpus (20 frozen cases, `benchmarks/untouched-repository-v1/corpus.json`,
  limit 20):

  | case | base | this change |
  |---|---|---|
  | `record response status when flushing` | carries `wrap_writer_test.go` | names it in `omitted_pairs` (falsifier met) |
  | `Bump minimum Go...` | carries `tree_test.go` | names it in `omitted_pairs` |
  | `Remove superfluous import aliases` | misses `clean_path_test.go` | carries it |

  Critical rows carried: base 44/55, this change 44/55, with 3 of the 11 misses named.
- Tests: `TestTaskContextSelectedLexicalPairs` and `TestRunTaskContextSubjectlessCounterparts`
  pin carried-or-named. The first fails on base (`counterpart source1_test.go carried=false
  omitted=false`). `TestAMAPV0025ChainBindingsFailClosed` `.mts`/`.cts` subtests fail without the
  suffixes.

## NOT_RUN

`make gate` (owner preference for scoped work). Affected tier 2 is left to batch CI. Daily-loop
UC-TASK-ORIENTATION is not run. The host toolchain is go1.27.3 (V1-2118).

## Rollback

Revert the lane's commits. That restores displacement, drops `omitted_pairs` and the two suffixes,
and returns the analyzer schema to the base number. No stored state or wire profile version
changed. `omitted_pairs` is additive.
