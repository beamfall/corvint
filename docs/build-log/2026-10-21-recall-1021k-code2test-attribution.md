# code2test recall@20 regression attribution — V1-2288, V1-0205 re-bench

Base: `58d4ed2d309d5dc1afc21c127647d2e22b1bed3f` (origin/main). Lane: recall-1021k.
Owning intent: TCP-V0-004 subjectless pair clause and TCP-V0-047 in
`docs/specs/task-context-packet-v0.md`; baseline from the 2026-09-23 V1-0205/V1-0206
entry in `docs/BUILD-LOG.md`. This entry records evidence for an owner decision. It
changes no code, spec or baseline.

## Problem

Agent Retrieval Bench v2 (`tools/retrieval-bench --arms context`, `corvint context
--task TEXT --limit 20`, frozen samples, code2test samples sha256 `712b2699…`
unchanged) measured code2test recall@20 0.4642 on main against the V1-0186/V1-0205
baseline 0.5116.

## Evidence

All numbers are recall@20/@10/@5; abstention is the abstain rate. Each run used a
binary built from the named first-parent commit with the harness built from base.

| binary | code2test | comment2context | trace2code | edit2ripple | abstention |
|---|---|---|---|---|---|
| baseline 242fde6 (#126) | 0.5116/0.3994/0.2877 | 0.5042/0.3438/0.2562 | 0.7937/0.5083/0.4010 | 0.6293/0.5101/0.3621 | 0.1707 |
| 1fd76c1 (parent of #320) | 0.5116 | — | 0.7937/0.5083/0.4010 | — | 0.1707 |
| de427c5 / e9a261d (#320) | 0.4481/0.3632/0.2642 | — | 0.8036/0.4637/0.3779 | — | 0.0244 |
| b8fe75d | 0.4528/0.3868/0.2877 | — | 0.7690/0.4884/0.4158 | — | 0.0244 |
| fc90abe = base 58d4ed2 | 0.4642/0.3962/0.3160 | 0.5062/0.3771/0.2687 | 0.7640/0.5165/0.4356 | 0.6336/0.5517/0.3937 | 0.0366 |
| base, pair admission off | 0.5022/0.4107/0.3160 | — | 0.7541/0.5116/0.4307 | — | 0.1707 |
| base, variant E (below) | 0.5022/0.4013/0.3160 | 0.5042/0.3771/0.2687 | 0.7640/0.5165/0.4356 | 0.6394/0.5517/0.4109 | 0.1707 |

The current harness reproduces the baseline binary exactly, so the harness is not the
cause. Bisection over first-parent merges since #126 (87e1044, 94ca556 and 1fd76c1
good) isolates **de427c57f** ("retain paired tests for subjectless lexical sources",
V1-0431, merged by e9a261d26 / #320). Its own entry
(`2026-09-28-subjectless-lexical-counterparts.md`) lists the frozen retrieval
evaluations as pending; they were never run. The 2026-10-01 rank-relative rework
(366c3756a, #444, amendment still marked "proposed") recovered r10/r5 but not r20,
and abstention stayed collapsed from 0.1707 to 0.0244–0.0366. No change after fc90abe
moves these numbers.

Mechanism: a subjectless counterpart test displaces a selected lexical test row. On
the frozen corpora the displaced rows are often the gold test (etcd 1dd2bdfd, diffusers
81051ce9/c7ea7d6f, tokio 3e1ace6b, vite 9cf0a50a/a5663509/c81f5402, vuejs e3045841
lost; etcd 0c60e782, tokio 7ae166d7 gained), and a non-empty packet replaces the
abstention outcome.

Variant E (lane experiment, not delivered) appends a counterpart only into unfilled
budget and never displaces a selected lexical test. It restores abstention and most of
code2test r20, improves edit2ripple and keeps trace2code. It still misses 0.5116 by one
sample (diffusers c7ea7d6f, where five `docs/source/*/modular_diffusers/*.md` rows from
a separate later change take the slots; not attributed). It fails
`TestTaskContextSelectedLexicalPairs`, which pins the rank-relative displacement
contract for the chi "record response status when flushing" orientation case.

## Decision needed (owner)

The drop is a consequence of V1-0431/366c3756a, which accepted displacement of
unrelated lexical tests for orientation without frozen evaluation. Options:

1. Restore: amend TCP-V0-004 so subjectless counterparts never displace selected
   lexical tests (variant E), update the chi pins, bump the analyzer schema; code2test
   r20 0.5022 then needs the diffusers docs-row cause fixed or accepted.
2. Re-baseline: accept code2test r20 0.4642 and abstention 0.0366 as the cost of
   orientation pairs, recording this cause.

The lane does not change the baseline.

## V1-0205

The merged binary (242fde6, TCP-V0-047 floor 2.0, analyzer/79) reproduces every
floor-2.0 number recorded on 2026-09-23 across all five subsets, including the guards
that entry had not re-benched. Criteria 1 and the bench half of criterion 2 were met
at merge. The UC-TASK-ORIENTATION treatment case and the 7-task unrelated-routing probe
(criterion 3, not a frozen fixture) were NOT_RUN here.

## NOT_RUN and rollback

NOT_RUN: daily-loop UC-TASK-ORIENTATION, the 7-task probe, go tests beyond the variant
E pin check. Host Go is go1.27.3 (contract expects go1.27.2). Rollback: delete this
file.
