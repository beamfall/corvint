# Decision 0024 — the trial's `corvint` arm supplies a task-context packet that keeps the subject out and admits the relations the gold sits in

Date: 2026-09-02. Status: accepted. Authority: repository owner, verbatim instructions "you need
to find ways to improve this. it must be a step up from everything else", "you can summon experts
to find some breakthroughs. We need to make sure that we are noticably better than grep and other
alternaticves", and "you can use experts not from the catalog. they need to be the best to find
the breakthrough" (2026-09-02), given after the trial's first observation (decision 0022's run)
showed the corvint arm at 32/13 success/confidently-wrong tasks against grep's 30/11 and none's
29/10.

## What was measured

Four expert charters ran read-only over the first-observation report, the 50-task corpus, and the
harness (scratch under the session's `experts/` directory; summaries reproduced here because the
scratch is not preserved):

1. Retrieval (ad hoc). A faithful reimplementation of the harness `grep` arm reaches gold in the
   top 20 for 33/50 (19 trace2code, 7 comment2context, 7 edit2ripple). No rescoring of the same
   query (IDF, BM25, length normalisation, identifier-only queries) beats it. Structural sources
   placed ahead of the grep tail do: test/source pairing by stem (+4 in ablation), a directory
   prior ordered by identifier evidence (+2), symbol-definition lookup for identifiers in the text
   (+1 to +2), paths the task names with their partners (+1), reverse imports (+1). The best
   combined packet reaches 42/50 (20/12/10) with no task lost; the slot caps were tuned once.
2. Program analysis (ad hoc). Of the 30 change tasks, today's `impact` relation (reverse imports
   plus same-package Go tests) reaches gold in 10; forward imports would add 4, cross-directory
   definition-to-reference 3, stem pairing anywhere in the tree 1, re-export following 2. The
   corpus truncates file text at about 8 KB (53 of the 85 given and gold files), so
   definition-level measurements are lower bounds.
3. Calibration (ad hoc). Every confidently-wrong claim in every arm was classified. All 13 corvint,
   11 grep, and 10 none confidently-wrong tasks but three are the task's own given file claimed as
   the answer; 9 tasks fail identically in all three arms, so between-arm tests cannot see them.
   Every READY `impact` packet ranks the changed path first (`score` 1000, `authoritative`, "direct
   changed path"), 9 of 21 packets hold only that row, and 6 of those 9 were confidently wrong.
   Gold was a result row in 6/50 corvint packets; the 11/50 figure counted mentions in exclusion and
   unparsed samples. Paired exact McNemar on the first run: confidently-wrong 2 corvint-only against
   0 grep-only (p = 0.5); a one-directional fix needs 6 discordant tasks to reach p < 0.05.
4. Competitive (`competitive-market-analyst`). With no repository access the none arm scores
   within one task of grep, so the model knows these public repositories and no retriever's value
   is readable until that is controlled. Proposed headline: paired clean success (gold found and
   no wrong `certain`), corvint at least five tasks above grep with a paired test; guardrails:
   gold-in-context not below grep per kind and zero packet-caused errors; stop-loss: pivot to
   Corvint as the verifier behind the agent's own retriever.

A direct probe of the shipped compiler over the 50 snapshots reaches gold as a result row for
38/50 (retrieval 20/20, change 18/30) against grep's 33; the one refusal is a change subject
absent from its snapshot.

## What is decided

1. A new experimental, Go-only command `corvint context --task TEXT [--subject PATH] [--limit N]`
   compiles the task-context packet (`docs/specs/task-context-packet-v0.md`): slots for pairing,
   mentioned paths, symbol definitions, reverse imports, and siblings ahead of a lexical fill, the
   subject carried beside the results and never among them, `NO_CANDIDATES` when nothing is
   admitted, and no exclusion or unparsed samples in the wire. The `query` and `impact` wires and
   their parity cases are unchanged.
2. The trial's corvint arm supplies this packet for both task shapes. The corvint prologue describes
   it and says a `NO_CANDIDATES` packet backs at most `unsure`.
3. The shared prompt skeleton, in both access modes, states that a path the task presents as its
   own subject is not one of its answers. It is shared so that the subject-claim error, which is a
   prompt artifact common to every arm, is removed from every arm alike; a drop that appeared only
   in the corvint arm would not be Corvint's.
4. The harness scores placement beside claims: `gold_in_context` (anywhere in the text),
   `gold_as_result` (a result row), and `context_failed` (a producer failure, never pooled
   silently), each summarised per arm.
5. The next reading is a paired one on the same 50 tasks with the same model, effort, limit, and
   access. Forward imports and cross-directory definition-to-reference edges are the next slots,
   after that reading, not before it.

## What is not decided

Whether the packet is a step up is the rerun's to say, not this record's; the 42/50 and 38/50
figures are gold placement on the corpus, not agent outcomes, and the none-arm memorisation
finding means a favourable outcome on these public repositories still needs a task set the model
has not seen before it can be called a product result.
