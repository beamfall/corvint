# Retriever Disagreement V0

Owner: Russell Lewis
Frozen: 2026-09-11
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/specs/task-context-packet-v0.md`,
`docs/specs/lexical-relevance-floor-v0.md`, `docs/agent-memory/ideas.md`
(2026-09-01 "abstention needs an answerability signal"), `AGENTS.md` invariants 2, 4, 5 and 7

## Agent digest
- Claim: The agreement of two independent retrieval channels is a cheap local answerability proposal; it changes no threshold, no packet, and no stored state.
- Status: proposed/experimental
- Exists: `internal/disagree`, `cmd/corvint/answerability.go`, and the five `cmd/corvint` requirement tests.
- Blocked on: the frozen designated missing-evidence corpus that would separate answerable from unanswerable better than the relevance floor alone.
- Read next: User and job; Requirements; Promotion and kill criteria.

## User and job

The 2026-09-01 probe over 82 `v2_abstention` no-gold tasks and 106 `v2_code2test` positives found
the lexical floor statistic cannot tell an answerable task from an unanswerable one: the
distributions overlap almost entirely, and the no-gold answers are carried by wrapper vocabulary.
A natural no-gold task is genuinely about the repository, so *more* lexical overlap is the wrong
test. The missing quantity is whether a second, independently-derived view of the repository picks
the same files.

Retriever Disagreement V0 builds that second view out of index structure alone and reports the
agreement of the two channels. Agreement is a **proposal** for a future abstention layer. It
establishes no semantic entailment, no correctness, no gold-file membership, and no obligation
closure, and it never edits the packet it read.

## Channels and truth boundary

- **Channel L (lexical).** The ordinary task-context packet's ranked result paths, read from
  `contextindex.TaskContext`. That is the only exported contextindex entry point yielding a ranked
  path list for a task, so the packet is channel L verbatim and `internal/contextindex` is not
  modified. The relevance-floor candidate set is not used: it is scored over already-verified CEM
  and OCM artifacts, not over a free-text task.
- **Channel S (structural).** Identifier tokens of the task text that are declared `Symbol.Name`s
  in the index, the sources declaring them, and one hop of `Index.Imports` in each direction.
  `Index.Imports` records raw specifiers; a specifier resolves to a source of the same index by
  that source's Go package import path, its own path, or its path without extension, and an
  unresolved specifier is dropped as an edge out of the repository.
- **No co-change expansion.** `internal/contextindex/pack_cochange.go` exports no accessor and this
  contract may not modify that package, so the co-change table is not consulted. The report states
  the omission in `signal.cochange_expansion` rather than leaving it silent.
- **Independence claim.** The channels are independent in *derivation*, not in *evidence*: both read
  one index at one revision. Channel S does no term matching over prose; channel L does no symbol
  resolution or graph expansion. Agreement is therefore evidence about retrieval, never about truth.

## Requirements

- **RDS-V0-001:** `corvint [--root PATH] answerability --task TEXT [--subject PATH] [--limit N]`
  MUST build channel L from `contextindex.TaskContext` over one index loaded exactly as
  `context defs|refs|grep` loads it — the snapshot `corvint index` wrote when one matches, else one
  `contextindex.BuildContext` over the committed tree — and MUST emit each admitted path with its
  rank and the packet relation that admitted it. `--limit` defaults to 10. A missing or blank
  `--task`, an unrecognized flag, or an unresolvable root MUST be refused with `invalid-arguments`
  (a `gokernel.Error`, as `argumentError` renders) and exit status 2.
- **RDS-V0-002:** Channel S MUST be derived only from index structure. Its seeds are the task's
  identifier tokens (split on every non-`[A-Za-z0-9_]` rune) that equal a declared `Symbol.Name`;
  its members are the sources declaring a seed name plus the sources one `Index.Imports` hop away in
  either direction, excluding the declared `--subject` path. No term, prose, path or BM25 matching
  may admit a channel S member, and each member MUST carry its structural reasons in sorted order.
- **RDS-V0-003:** The signal block MUST report the Jaccard overlap of the two channels' top-K paths,
  where K is `min(limit, 10)`, together with `overlap_paths`, `overlap_size` and `union_size`, and an
  overlap-weighted `rank_correlation`: the Spearman coefficient over the shared paths alone, `null`
  when fewer than two paths are shared or either shared-rank vector is constant.
- **RDS-V0-004:** `signal.agreement` MUST be `high` when the top-K Jaccard is at least `0.34`, `low`
  when it is below `0.34` with at least one shared path, and `none` when no path is shared. These
  thresholds are fixed by this contract; the command reads no configured threshold and writes none.
- **RDS-V0-005:** The whole report MUST be deterministic at one revision: channel S is ranked by
  number of distinct structural links descending, then path ascending, and the report is emitted
  through `gokernel.CanonicalJSON`. Repeated invocations over one revision MUST be byte-identical.
- **RDS-V0-006:** `signal.proposed_state` MUST be `answerable` only when `structural_vote` is
  `present` and `agreement` is `high`; `uncertain` when `structural_vote` is `absent` or `agreement`
  is `low`; `unanswerable` when both channels voted and share no top-K path. A task naming no
  declared symbol leaves channel S empty and MUST report `structural_vote: absent`,
  `agreement: none` and `proposed_state: uncertain` — one channel alone never proposes `answerable`
  (`AGENTS.md` invariant 2).
- **RDS-V0-007:** The command MUST be read-only (`AGENTS.md` invariant 4): it writes no repository
  file, no trace, and no self-observation row, and reports `mutates: false`. Every report MUST carry
  the fixed `disclosure` stating that this is a proposal for the abstention layer and that the
  command changes no threshold and no packet. Nothing it emits is an input to ranking, learning,
  evidence, or authority (`AGENTS.md` invariant 5). Clarifying amendment (2026-09-13, bug hunt): the
  index load runs under the process signal context, so SIGINT or SIGTERM before the report compiles
  is that same loader failure (the loader's cancellation envelope, exit 2), never an ignored signal
  followed by an exit-0 report.
- **RDS-V0-008:** The report MUST carry a `coverage` block mirroring the packet's denominators —
  source and symbol counts, the index's `Unparsed` rows with reasons, `ApproximateImports`, the
  number of sources with extracted imports, and the packet's own `coverage` object — so a channel
  built over a partly-understood index is never read as complete.

## Non-goals

- No new threshold, no change to the packet's own answerability field, and no abstention decision:
  this command proposes, the abstention layer (a later `GPK-V0-039` amendment) would decide.
- No embeddings, no model call, no network, no daemon, no stored state (`AGENTS.md` invariant 7).
- No claim that agreement implies correctness, gold-file membership, or semantic relevance.
- No modification of `internal/contextindex`, and no co-change channel while that table is unexported.
- No conformance corpus and no evaluation gate: the separation claim is unmeasured at V0.

## Failure modes

- **Shared-index correlation.** Both channels read one index, so an index that missed a subsystem
  makes both channels wrong together and agreement reads `high` for the wrong reason. `coverage`
  (RDS-V0-008) is the only guard; it does not remove the correlation.
- **Wrapper identifiers.** A task naming a common declared symbol (`Read`, `Error`) seeds channel S
  from vocabulary rather than intent, exactly the failure the 2026-09-01 probe found in the floor.
  V0 does not weight seeds by rarity.
- **Empty structural channel is the common case.** Natural-language tasks often name no declared
  symbol, so `uncertain` will dominate. That is deliberate abstention, not a defect, but it caps how
  much the signal can separate.
- **Small repositories.** With few sources the Jaccard denominator is tiny and the `0.34` boundary is
  reached or missed by one path.
- **Unresolved specifiers.** Non-Go and web specifiers resolve by path spelling alone, so channel S
  under-expands for those languages; `coverage.approximate_imports` names the size of that gap.
- **Cancellation during the load.** SIGINT or SIGTERM before the report compiles: the loader's
  cancellation envelope, exit 2, empty stdout (`RDS-V0-007`).

## Traceability

| Requirement | Delivery | Go test function |
|---|---|---|
| `RDS-V0-001` | experimental | `TestAnswerabilityHighAgreementProposesAnswerable` (channel L rows and the load path) |
| `RDS-V0-002` | experimental | `TestAnswerabilityStructuralChannelIsStructuralOnly`, `TestAnswerabilityAbsentStructuralVoteProposesUncertain`, `TestStructuralChannelLinksDeclaringAndImportingFiles`, `TestStructuralChannelAbstainsWithoutDeclaredIdentifiers`, `TestStructuralChannelLinksDottedPythonImports` |
| `RDS-V0-003` | experimental | `TestAnswerabilityHighAgreementProposesAnswerable`, `TestCorrelation` |
| `RDS-V0-004` | experimental | `TestAnswerabilityLowAgreementProposesUncertain`, `TestCompareAgreementAndProposedState`, `TestCompareJaccardThresholdBoundary` |
| `RDS-V0-005` | experimental | `TestAnswerabilityIsDeterministic`, `TestRankLinksOrdersByLinkCountThenPathAndTruncates` |
| `RDS-V0-006` | experimental | `TestAnswerabilityAbsentStructuralVoteProposesUncertain`, `TestCompareAgreementAndProposedState` |
| `RDS-V0-007` | experimental | `TestAnswerabilityWritesNothingAndDiscloses`, `TestRunAnswerabilityHonorsCancellation` |
| `RDS-V0-008` | experimental | `TestAnswerabilityCoverageNamesUnparsedSources` |

## Promotion and kill criteria

- **Gate.** On the frozen designated missing-evidence cases, disagreement separates answerable from
  unanswerable strictly better than the relevance floor alone: at a threshold chosen once, it
  abstains on strictly more no-gold tasks than the floor's "strongest support ≥ 3" rule while
  dropping no `v2_code2test` positive.
- **Kill.** No separation at any threshold — every threshold that gains no-gold abstentions loses a
  positive — kills this direction and the command is deleted.
- Until that measurement exists the verb stays experimental, is advertised nowhere as delivered, and
  no abstention path consumes it.

## Rollback

The slice is additive: `internal/disagree/`, `cmd/corvint/answerability.go`, its test file, this
spec, and one dispatch line each in `cmd/corvint/main.go` and `cmd/corvint/help.go`. Deleting
those files and the two dispatch lines removes the verb completely. No stored state, no schema, no
packet field, and no threshold changes, so there is nothing to migrate back.
