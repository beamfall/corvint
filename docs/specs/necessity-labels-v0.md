# Necessity Labels V0

Owner: Russell Lewis
Frozen: 2026-09-11
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `AGENTS.md` (invariants 1, 2, 4),
`docs/specs/task-context-packet-v0.md`, `docs/DOGFOOD.md`, `LICENSING.md`

## Agent digest
- Claim: `corvint necessity` labels each packet file `load-bearing` or `supporting` by removing it from an index copy and recompiling the packet.
- Status: proposed/experimental
- Exists: `internal/necessity`, `cmd/corvint/necessity.go`, and their tests.
- Blocked on: a CEM reviewer trial measuring whether the labels help a reviewer reach decisive evidence, and dogfood receipts; dispatch from `cmd/corvint/main.go` landed with decision 0089.
- Read next: Requirements; Labelling rule; Gate and kill.

## User and measurable job

A packet of twelve files does not say which two a reviewer must open. The reviewer either reads all
twelve or trusts rank order, and rank order is a score, not a dependency. This verb answers the
dependency question the only way a deterministic ranker can be asked it: remove one included file
from the index and recompile the same packet. If the packet then loses a critical anchor, gains a
missing critical selector, or leaves its resolved state or verdict, the file carried something the
packet depended on.

The job is measurable on the CEM reviewer trial: with labels shown, a reviewer should locate the
decisive evidence for a change in fewer opened files than with the packet alone. It is killed if the
labels agree with reviewer judgement no better than plain rank order does.

This contract is proposed and the implementation is experimental. It is not a claim that a
`supporting` file is unnecessary to a human, nor that a `load-bearing` file is causally relevant to
the task. It is a counterfactual over one ranker at one immutable revision, and nothing else.

## Labelling rule

The baseline is the compiled packet's `state`, its `coverage.answerability.verdict`, its
`coverage.critical` selector set, and the size of `coverage.critical_missing`. For one included path
the counterfactual packet is recompiled over an index copy without that path. The removal degrades
the packet when, in this fixed precedence, a baseline critical selector is absent, `critical_missing`
grew, `state` changed, or `verdict` changed to an abstaining one. The first matching clause supplies
the emitted `change` string, so the answer is deterministic for one revision, task, subject and
limit.

## Requirements

- **NEC-V0-001:** `corvint [--root PATH] necessity --task TEXT [--subject PATH] [--limit N]` MUST
  parse exactly the `context` verb's arguments, MUST default `--limit` to 12, MUST refuse a limit
  outside 1..50, an unrecognized flag, or a missing `--task` with `invalid-arguments` before any
  repository read, and MUST NOT claim any other invocation shape.
- **NEC-V0-002:** The verb MUST load the index exactly as `context --task` does (the tree's snapshot
  when one exists, else one observed build over the committed tree), MUST compile the packet through
  `contextindex.TaskContext` with the caller's task, subject and limit, and MUST emit that packet's
  members unchanged plus one added `necessity` member on a single canonical JSON line.
- **NEC-V0-003:** Each counterfactual MUST run over a shallow copy of the index whose `Sources`,
  `Tracked`, `Documents`, `Features`, `Scenarios`, `Markers` and `Symbols` tables are copied before
  the path is dropped and whose derived term table is rebuilt. The loaded index MUST NOT be mutated
  on any path, so the emitted packet and every label answer for the same revision.
- **NEC-V0-004:** For each included path the verb MUST label it (a) `load-bearing` when the
  counterfactual packet degrades under the Labelling rule, naming the degradation in a short
  deterministic `change` string, or (b) `supporting` when it does not. A label MUST NOT be inferred
  from score, relation, rank, or any input other than the recompiled packet.
- **NEC-V0-005:** Counterfactual work MUST be bounded by `min(--limit, 12)` recompiles for one
  invocation. Every included path past that budget MUST be emitted as `unlabelled` with reason
  `budget`, never as a guessed or defaulted label.
- **NEC-V0-006:** When the packet carries no answer — `state` is not `READY`, or the answerability
  verdict is `unsupported-conjunction` — the verb MUST emit the packet unchanged, run zero
  counterfactuals, and emit an empty label list marked `abstained`.
- **NEC-V0-007:** Every emitted `necessity` block MUST carry the fixed disclosure stating that the
  labels are counterfactual under the current ranker and are not proof of relevance, on the
  abstaining path as well as the labelled one.
- **NEC-V0-008:** The verb MUST be read-only: no index, snapshot, trace, ledger or self-observation
  write on any path, including failure. Exit status MUST be 0 for any compiled answer including an
  abstention, and 2 with one error envelope on stderr and empty stdout for an argument or repository
  failure. Clarifying amendment (2026-09-13, bug hunt): the load runs under the process signal
  context, so SIGINT or SIGTERM before the packet compiles is that repository failure (the loader's
  cancellation envelope, exit 2), never an ignored signal followed by an exit-0 answer.

## Emitted shape

```json
{"...": "the ordinary task-context packet members",
 "necessity": {
   "abstained": false,
   "baseline": {"state": "READY", "verdict": "no-specific-terms", "anchors": 1, "critical_missing": 0},
   "budget": 12, "reruns": 5,
   "labels": [{"path": "AGENTS.md", "label": "load-bearing",
               "change": "anchor lost: governing AGENTS.md", "reason": "counterfactual"}],
   "disclosure": "Labels are counterfactual under the current ranker at this revision, ..."}}
```

## Non-goals

- No claim of causal relevance, sufficiency, or reading order. A `supporting` file may be the one a
  human needs; the label only reports what the ranker did without it.
- No cross-revision, cross-task, or aggregated necessity statistic, and no learned or cached label.
- No pairwise or subset counterfactual: exactly one path is removed per recompile, so a pair of
  mutually redundant files is labelled `supporting` twice and the packet says so only through the
  disclosure.
- No removal from the import graph, co-change history, or Git object store: the counterfactual is
  over the index's read-side tables, not over the repository.
- No write path, no new verb-level configuration, and no change to the packet contract itself.

## Failure modes

| Mode | Behavior |
|---|---|
| Packet abstains or has no results | packet emitted unchanged, zero reruns, empty labels, exit 0 (NEC-V0-006) |
| Included path count exceeds the budget | remaining paths `unlabelled`/`budget`, exit 0 (NEC-V0-005) |
| A counterfactual recompile refuses | that path is `unlabelled` with reason `error`; other labels stand |
| Invalid limit, unknown flag, missing `--task` | `invalid-arguments` envelope, exit 2, no repository read |
| Repository unreadable or no committed revision | loader error envelope, exit 2, nothing written |
| SIGINT or SIGTERM during the load | loader cancellation envelope, exit 2, empty stdout (NEC-V0-008) |
| Two files redundant with each other | both `supporting`; the disclosure is the only warning (non-goal) |

## Traceability

| Requirement | Implementation | Test |
|---|---|---|
| NEC-V0-001 | `cmd/corvint/necessity.go` `parseNecessityInvocation` | `TestParseNecessityInvocation` |
| NEC-V0-002 | `internal/necessity/necessity.go` `Resolve`, `loadIndex` | `TestRunNecessityIsReadOnlyAndLabelsThePacket` |
| NEC-V0-003 | `without`, `dropSource`, `dropSet`, `dropRecord`, `dropMarkers` | `TestWithoutLeavesTheOriginalIndexUnchanged`, `TestWithoutDropsRecordTablesByPath` |
| NEC-V0-004 | `labelOne`, `difference` | `TestRemovingAnAnchorPathIsLoadBearing`, `TestRemovingARedundantPathIsSupporting` |
| NEC-V0-005 | `label` budget branch | `TestBudgetLeavesRemainingPathsUnlabelled` |
| NEC-V0-006 | `abstains`, `label` abstention branch | `TestAbstainingPacketCarriesNoLabels` |
| NEC-V0-007 | `Disclosure` | `TestLabelledAnswerCarriesTheDisclosure` |
| NEC-V0-008 | `runNecessity` | `TestRunNecessityIsReadOnlyAndLabelsThePacket`, `TestRunNecessityRefusesAnUnknownFlag` |

## Gate and kill

- **Gate:** on the CEM reviewer trial, reviewers shown the labels locate the decisive evidence for a
  change in strictly fewer opened files than reviewers shown the same packet in rank order, over the
  frozen trial set, with the per-invocation cost of the recompiles recorded.
- **Kill:** the labels agree with reviewer judgement of what was decisive no better than rank order
  does, or the recompile cost exceeds the packet compile by more than the trial's stated budget. On
  a kill the verb is withdrawn and this contract is marked rejected; no consumer is affected because
  the block is additive.

## Rollback

The verb is additive and read-only. Rollback is removing the `necessity` dispatch from
`cmd/corvint/main.go` and the entry from `cmd/corvint/help.go`; the packet contract, the index,
and every other verb are untouched, and no stored state exists to migrate. Deleting
`internal/necessity`, `cmd/corvint/necessity.go`, and this document completes the removal.
