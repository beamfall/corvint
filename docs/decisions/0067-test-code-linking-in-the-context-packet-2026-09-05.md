# Decision 0067 — The context packet links tests and code as a bidirectional relation

Date: 2026-09-05. Status: proposed (measured 2026-09-05; trace2code recall@20 loss disclosed, promotion under decision 0070). Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that open calls be resolved through the expert panel, and this record applies `docs/reviews/b1-retrieval-breakthroughs.md` section B2 (the test↔code direction the panel ranked second) to the `context` verb's retrieval shape only, after decision 0066 landed the lexical order it builds on.

## Context

The `context` packet without a subject admits files by mention, definition and lexical score,
and none of those relations names a test's source or a source's tests: the `pair` slot that
does exists only in the change shape, where `--subject` names the anchor. The Agent Retrieval
Bench `code2test` subset asks for exactly that counterpart and the arm's recall there sat at
0.464 (hit@k 0.547) on the decision 0066 tree; the memo's section B2 traces 3 of blind-v3's
critical selectors and 8 of 50 held-out gold files to the same missing relation.

Section B2 names four deterministic, language-neutral signals the snapshot already answers:
the mirrored path/stem convention `pair` reads, an import edge from a test path to the source,
a whole-word mention in a test of a name the source declares weighted by rarity, and test-name
tokens (`Test<Name>`, `test_<name>`, `describe('<name>')`) that camel-split into a declared
name. Its proposed index-time relation table is deferred: this record derives the relation at
query time from `Tracked`, `Imports`, `Symbols` and the identifier vocabulary, so the snapshot
format is unchanged while the retrieval gain is measured.

A second, smaller defect showed in the same packets: a task that backticks an English word
(`all`) spent three `definition` rows on its definers, because TCP-V0-010 makes every backticked
identifier eligible.

## Decision

1. `TCP-V0-015`: in the retrieval shape the packet reserves one slot for a `test` row bound to
   an admitted code file or test by the four signals, ordered by the anchor's packet position,
   then summed rarity-weighted signal weight, then fired signals (the first measured order,
   signals before position, let a two-signal stem match from a lexical anchor outrank the
   changed file's own one-signal test on 3 of 3 probed misses); the evidence reason names the signals that
   fired and the relation is counted under TCP-V0-011's `withheld`. The relation is
   `not-applicable` with a subject, where `pair` already owns the counterpart.
2. The `definition` slot refuses a backticked identifier on TCP-V0-014's stop list, which gains
   `all`. A backticked camelCase, snake_case or digit-bearing identifier is unaffected.
3. The signals stay query-time derivations of existing tables; a snapshot relation table is a
   separate decision once the measured gain justifies the format change.

## Evidence and falsifier

Fixtures in `internal/contextindex/taskcontext_testlink_test.go` bind one pairing per signal
in both directions, pin the one-slot reservation with its `withheld` count, and the
definition-slot rule. The Agent Retrieval Bench `context`-arm runs on `code2test`,
`edit2ripple` and `trace2code` against the decision 0066 tree are the promotion gate written
into `TCP-V0-015`: this record moves to accepted when code2test hit@k and recall@20 rise and
edit2ripple recall@20 stays at or above the grep arm's, and is withdrawn otherwise. Measured
2026-09-05 against the decision 0066 tree rerun on the same machine: code2test hit@k 0.566 →
0.604 and recall@20 0.485 → 0.512 (grep 0.274 / 0.259); edit2ripple 0.741 → 0.759 /
0.608 → 0.629 (grep 0.690 / 0.543); trace2code 0.901 → 0.871 / 0.833 → 0.804 (grep 0.891 /
0.827) with recall@5 0.421 → 0.431 and mrr 0.282 → 0.356. The stated falsifier passed;
trace2code lost three golds at rank 20, which the lane's report had hidden behind the b=0.75
baseline. Implementation note (2026-09-05, landing delegate, not owner text): under decision
0070 this record stays proposed until the paired interval on both repository folds resolves
the trace2code recall@20 loss as noise or the lane recovers it.

## Consequences

- A retrieval-shape packet may carry one `test` row before its lexical fill, displacing the
  last lexical row at the limit; `unexamined` gains the `test` relation in fixed order.
- Per query the packet reads the reverse-import rules and up to ten candidate texts per anchor
  beyond the existing posting walks; no persisted state changes.

## Rollback

Revert `testRows` and its helpers, the `test` entry in `contextRelationOrder`, the
`definitionEligible` stop-list clause and `all` in `taskStopWords`; no wire, snapshot or oracle
migration is required.
