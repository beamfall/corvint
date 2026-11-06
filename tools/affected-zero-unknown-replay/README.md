# Zero-unknown replay

`script/affected-zero-unknown-replay.sh --help` documents the flags. Build the candidate and
baseline binaries through `script/heavy.sh`, then pass their absolute paths with `--corvint` and
`--baseline`. `--out` must name a private directory outside the repository. The report retains
binary/list digests, exact historical revisions, plans and before/after deltas. It adds later
first-parent merges through `--through` (default `origin/main`); it never changes a historical
snapshot's declarations. Exit 1 and `accepted: false` mean qualification is incomplete or unsafe.

`--safety DIR` reads `DIR/<full-merge-sha>.json`. Each document uses this shape:

```json
{
  "merge": "FULL_MERGE_SHA",
  "parent": "FULL_FIRST_PARENT_SHA",
  "parentSuite": {
    "revision": "FULL_FIRST_PARENT_SHA",
    "inventory": ["go:example.test/pkg"],
    "complete": true,
    "results": [{"unit":"go:example.test/pkg","test":"TestExample","outcome":"pass"}]
  },
  "mergeSuite": {
    "revision": "FULL_MERGE_SHA",
    "inventory": ["go:example.test/pkg"],
    "complete": true,
    "results": [{"unit":"go:example.test/pkg","test":"TestExample","outcome":"pass"}]
  },
  "mutationComplete": true,
  "mutants": [{"id":"changed-line-mutant-1","side":"merge","path":"pkg/pkg.go","line":12,
    "killedBy":[{"unit":"go:example.test/pkg","test":"TestExample"}]}]
}
```

Inventories are sorted unit IDs and must match all eligible units (selected plus excluded) at
that exact revision. Every inventory unit needs results. Outcomes are `pass`, `fail` or `skip`;
unit/test pairs are unique. A parent test absent from the merge outcomes keeps safety
`NOT_PRODUCED`, including a missing test within an otherwise observed unit. Mutant IDs are unique; `side` is `parent` or `merge`; its line must
belong to a zero-context diff hunk on that side. Killers must occur in the merge suite inventory
and results. Every differing/new outcome and every killer must be selected. The tool's mutation
fixture deliberately removes such a selection and requires `UNSAFE_DROP` from this same checker.

The external runner owns the completeness statements. This comparator checks bindings and
selection inclusion; it cannot authenticate full-suite execution or establish mutation adequacy.
Keep the original runner artifacts with these receipts. Missing receipts remain `NOT_PRODUCED`.
The example is documentation, never qualification evidence. No historical receipt is supplied by
this repository.
