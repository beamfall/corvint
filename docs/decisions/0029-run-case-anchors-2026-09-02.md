# Decision 0029 — `t.Run("...")` literals are Go test-claim anchors; untouched obligations stay `unassessed`

Date: 2026-09-02. Status: accepted. Authority: repository owner, verbatim instruction "go ahead
with the two fixes" (2026-09-02), on the dogfood binding of decision 0028, where the two
requirements that change altered could be linked only after their tests were rewritten as
table cases because `t.Run` string literals were not claim anchors, and where `ocm mark` had no
reason meaning "unchanged by this change".

## What is decided

1. A Go case anchor is a table entry's `name`/`testName`/`test_name` field or the first
   argument of an identifier's `.Run(` call (`t.Run("...")`), with the same literal, parent-body,
   and uniqueness rules as before (TCQ-V0-018 amended). Go (`goTableCase`, `goTableCaseTail`)
   and the Python oracle (`context_corvint_claims.py`, `context_corvint_test_claim.py`) change
   identically; any `.Run(` receiver is accepted so the two implementations stay a regexp apart
   from a token rule, and the selector's uniqueness rule still rejects an ambiguous anchor.
2. No new `ocm mark` reason. The four reasons are a wire contract (OCM-V0 "exactly", CF-V0-011's
   one-to-one map to frontier codes, the oracle, and the frontier consumers), and `unassessed`
   already says what an untouched requirement is under a scoped change: not assessed by it.
   `docs/DOGFOOD.md` says so, so the next binding does not go looking for a fifth reason.

## What is not decided

Whether a scoped change should be able to declare its obligation scope up front (the requirements
it touches) so the OCM aggregate reports "n of m in scope linked" rather than "2 of 19 linked";
that is a contract change for the OCM aggregate and waits.
