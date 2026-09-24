# Decision 0377 — Trial reciprocal rank fusion against the corroboration count in `context`

Date: 2026-09-24. Status: proposed (ticket V1-0219; TCP-V0-048..050). Authority: repository
owner, "rescope it to RRF vs corroboration" and "start the spec amendment for V1-0219"
(2026-09-24). It amends nothing: it specifies an opt-in ordering and the reading that decides it,
and sets no default.

## Context

Ticket V1-0219 came from a review of vectorize-io/hindsight, which merges parallel retrieval
channels with reciprocal rank fusion (RRF). The ticket first proposed fusing lexical, structural
and git-recency channels. Recording the current state showed that the task-context packet is
built from slots, not from one ranked list. `compile` admits capped slots in evidence order
(`internal/contextindex/taskcontext.go:237-283@fc3cbfcb`), and `corroborate` (`taskcontext.go:338-352`,
decision 0027) already reorders rows by the number of relations that would have admitted each
path. Recency already exists as an opt-in reweighting inside the lexical and `cochange` slots
(decision 0369). The frozen bench cannot measure it, because it rebuilds each snapshot as one
commit. The owner rescoped the ticket to RRF against the corroboration count.

Decision 0027 left open "weighting relations differently, and the graph walk that subsumes both
slot order and corroboration". RRF is the smallest rank-aware form of that question: it reads the
same relations per path that corroboration reads and differs only in weighting each relation by
the rank at which it found the path.

## Decision

1. `CORVINT_CONTEXT_RRF=on` replaces the corroboration count sort with a sort by
   `sum 1/(60 + rank)` over the admitting and corroborating relations, where rank is the path's
   position in each relation's own examined candidates (TCP-V0-048). Ties keep the incoming slot
   order.
2. Admission, caps, evidence rows, score, summary and reserved and graph placement are unchanged.
   Each row's reason names every contributing relation and rank. Unset, the packet bytes are
   unchanged (TCP-V0-049).
3. The constant is 60, the value conventionally used for RRF, fixed before any reading so that it
   is not fitted to the evaluation sets. With caps of a few rows it makes the relation count
   dominate, so the trial mainly tests whether rank should break ties that slot order breaks
   today. A second constant is a new decision, not a sweep inside this one.
4. The flag is decided by the frozen `tools/retrieval-bench` `context` arm (no recall@20 loss on
   any of the four subsets) and by decision 0027's offline gold-rank reading on
   `unseen-corvint-v2`, with its `unseen-corvint-v1` subset also reported alone, off against on
   (TCP-V0-050). A loss keeps the flag opt-in or removes it. A gain at ranks 1-5 on both sets
   with no top-20 loss makes it a default-on candidate that still needs decision 0070's paired
   ladder and a separate owner decision.

## Consequences

The default product is unchanged until a later decision. Git recency as a fused channel is out of
scope and waits on a history-preserving corpus (ticket V1-0221). Decision 0027 records its
offline gold-rank reading but names no committed harness for it, so implementing V1-0219 either
reuses one that exists or adds a reproducible one whose report is kept with the BUILD-LOG entry.
