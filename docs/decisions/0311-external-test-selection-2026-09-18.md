# Decision 0311 — External verification relations may narrow test selection only when every obligation qualifies

Date: 2026-09-18. Status: accepted (delegated call on the owner's own feature request
Beamfall/corvint#5; owner review of the intent is welcome and changes only the digest line).

## Decision

`corvint affected --provider FILE` adds `advice.test_selection`, specified in
`docs/specs/external-test-selection-v0.md` (`ETS-V0`). Without `--provider` the receipt is
unchanged, and with it every other member is byte-identical.

- **Surface.** The advice lives in the affected plan, which already owns "what to run", and not in
  `impact`. `--selection-profile strict|coverage` defaults to `strict` (`verifies`, `asserts`);
  `coverage` also admits `covers`.
- **Narrowing.** `narrow-selection-allowed` requires every changed path, every entity it maps to,
  and every one-hop downstream entity to have a qualifying relation and no blocking failure. A
  qualifying relation is declared or observed, repository-bound (EEP-V1), fresh for its test side,
  verified at that revision, and not a dirty root path. V0 records carry no repository identity
  and never qualify.
- **Fail closed.** An unavailable record blocks. An unbounded plan scope or an empty change is
  `unknown`. Identity, binding, freshness, verification, and namespaced-type failures touching an
  obligation block it. Weak evidence (context types, inferred, learned, `covers` under strict) is
  listed as a coded candidate and neither qualifies nor blocks.
- **Mandatory checks** are echoed unchanged and never narrowed. The state never derives from the
  count of selected tests, and `confidence` is `unscored` because no record carries one.

## Alternatives weighed

- *Widen-only, as the ideas backlog proposed*: safe, but it cannot answer the request, which is
  to run fewer tests when the evidence proves that is enough. Narrowing is kept behind the full
  obligation check, and the conformance evaluation fails on any unsafe narrowing.
- *Admit V0 records*: a V0 record has no repository identity, so a test path cannot be bound to
  the commit it was verified at.
- *Transitive obligations*: unbounded and unmeasured. One hop is the V0 bound, and deeper
  obligations are a follow-up.
- *Inspect bound checkout worktrees*: a second dirty-state read per checkout. Rows disclose
  `checkout-worktree-not-inspected` instead.

## Rollback

Remove `internal/extevidence/selection.go` and its tests and fixtures, the three `affected`
options and their help text, the spec, its index rows, and this record, and revert the AFP-V0-009
wording. A run without `--provider` is unaffected in both directions.
