# Decision 0601 — build Ruby line-level selection and admission

Date: 2026-11-16. Status: accepted. Authority: repository owner, in chat on 2026-11-16. Ticket:
V1-4365 (acceptance criteria 3 and 6).

## Question and answer

The independent review of lane r1docs stopped V1-4365 criteria 3 and 6: the accepted Ruby rule
(`LPCV-V0-7605`) sends an edit in an uncalled method to static fallback, so the unexecuted-line
control can never select zero tests, and no admission path for a Ruby qualification existed. The
owner was asked whether to keep Ruby explicitly unqualified in 1.0 or to build Ruby line-level
selection and admission. The owner answered: "Build Ruby admission".

Nothing beyond that answer is attributed to the owner. The scope below is how the lane charter
carrying the answer applies it.

## Decision

1. `LPCV-V0-7605` is amended. When the Ruby family is admitted by a passing Ruby qualification
   record, a Ruby edit selects the tests whose measured line coverage hit the edited lines, and an
   edit on executable lines with zero measured hits selects no test. Every unqualified state keeps
   the static plan, and the method-level analysis with its uncalled-method fallback stays the rule
   of unqualified working-tree plans (`linecov watch`).
2. Every fail-safe stays: load-time code, edits Ruby does not count as executable outside a method
   body, files absent from the index or recorded at another blob, a stale or mismatched index,
   non-`.rb` inputs, changed test files and helpers, block-structure edits, a changed source that
   does not parse, and any collector doubt (unordered or unrecorded tests) keep static or whole
   selection with the reason reported.
3. A Ruby qualification record, `.github/linecov/ruby-qualification.json` (`LPCV-V0-7608`), binds
   the selector and collector source blobs, the seed, fault count, kills, per-runner selected-set
   sizes, controls and misses. The admission check reads it at the selector's own clean VCS
   revision; a stale binding, a miss, an unqualified runner or a failed control refuses admission,
   and the plan reports `coverage-collector-unqualified`.
4. The `.github` edits that bind, retain and check the Ruby record are in scope. No other `.github`
   edit is.
5. The test assertions that encode the old rule for the qualified path change only as item 1
   requires; the unqualified method-level assertions (`TestUncalledMethodFallback`, the `checkCalc`
   uncalled-line case) remain true of the unqualified analysis.

## Consequences

- Ruby can become QUALIFIED under `LPCV-V0-963` only through a record with zero misses and passing
  controls for both Minitest and RSpec on the pinned public corpus.
- A change to any bound Ruby selector or collector source invalidates the record until a new
  qualification run.
- This decision admits no Go record and changes no other family's state.

## Rollback

Revert this decision's commits. Ruby then returns to the method-level analysis for every plan, the
Ruby record is no longer read, and every Ruby plan is static with `coverage-collector-unqualified`.
