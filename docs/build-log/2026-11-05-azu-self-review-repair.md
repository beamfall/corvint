# AZU self-review repair: preserve driver and replay completeness

The supplied review identified four bounded failures in experimental driver
selection and historical safety qualification. These repairs implement the existing
AZU-V0-001/003/009/8903 intent; they add no public command, field, diagnostic or
acceptance rule. Default CLI driver registration remains excluded under the
previous frozen-receipt rollback. Aggregate AZU qualification and ticket completion
remain blocked by the previously recorded external integration and safety receipts.

## Decisions and failure modes

1. A Go change confined to its own test files remains isolated from ordinary Go
   importers (AFP-V0-036). Driver units invoke those tests directly, so their import
   edges have different semantics. Seed only the direct driver callers of contained
   test changes, then use the existing traversal for their dependents. The changed
   Go unit itself remains absent from `ReachedBeyondOwnTests`. The regression uses
   `go test -race ./core/...`, a dependent make target, a calling workflow job, a
   dependent job, and an ordinary Go importer that must remain excluded.
2. Replay suite inventories include the existing optional `goAdditiveExcluded`
   category as well as `selected` and `excluded`. This is a decoder correction,
   not a change to emitted affected plans or safety receipts. An unchanged excluded
   unit can qualify with complete external observations; a differing outcome in
   that unit still fails the same safety checker. Frontier exclusions describe
   already-reached units, rather than another omitted inventory category.
3. Check operand availability before consuming value-taking `env` and wrapper
   options. Missing operands retain the scanner's existing computed-invocation
   uncertainty. Regressions cover `env -u/-C/-S`, `nice -n`, `timeout -k`,
   `xargs -I`, `ionice -c`, `taskset -c`, nested wrappers, and valid operands.
4. Reject duplicate SHAs in the supplied frozen list before merging automatic
   discovery. Append discovered first-parent merges only if absent from that list.
   The regression creates actual merge commits, extends the frozen list through
   its newest merge, checks discovery from the older list, and rejects duplicate
   supplied rows. The contractual eight-SHA prefix check remains unchanged.

The witnesses are `TestDriverCallersOfContainedTests_AZU_V0_001`,
`TestDriverTruncatedWrapperOptions_AZU_V0_001`,
`TestSafetyIncludesAdditiveExclusions_AZU_V0_8903`, and
`TestReplayGrowingFrozenList_AZU_V0_8903`. Existing assertions and frozen outputs
are unchanged. Rollback is a revert of this bounded repair; it restores the four
reported failure modes without changing the earlier default-driver disposition.

## Verification and limits

Pre-test native `affected --base
8e2c13aa926fcaf1731fae71b27c41c41c8f09af --full` retained scope UNKNOWN with
83 unknown entries and selected both changed Go packages. Pre-change capture was
reused as requested; no new starting dogfood capture ran. Applicable self-use routes
are affected selection, existing context receipts and final CEM rebind/check.
Unrelated feature, learning, release and publication routes are excluded.

Raw evidence stays under
`/home/russell/lanes/azu-new-evidence/self-review/`. Independent review belongs to
the orchestrator. Historical external full-suite and mutation receipts remain
NOT_PRODUCED; these regression fixtures do not establish aggregate safety or
zero-unknown acceptance. Repository-wide gates, Darwin qualification and pre-push
Windows vet are NOT_RUN in this scoped, unpublished repair. Tasks writes,
publication and sealing are prohibited. The final checked CEM is retained unsealed.

Both changed packages ran their EXISTING whole-package tests without `-run`,
with the mandated temporary directory, Go 1.27.3 and the existing host semaphore:

- `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/affected-zero-unknown-replay`
  passed (0.134 s). Log SHA-256:
  `de62dddb0bccc629b73d619ada3bd650ab10ef2247dd5239403e6ce98081da5d`.
- `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/liveverify/affected`
  passed (198.181 s). Log SHA-256:
  `76b6685238dfc2c31aa8346d7baa6be100bb14853aa63b2125b96a023ef7cb9e`.

The pre-test affected plan selected 189 units with 83 unknowns, including both
changed packages. The owner-required whole-package checks were admitted with
`HEAVY_WHOLE_REASON` naming that requirement and the retained affected uncertainty.
No semaphore override was used. Line-citation repinning moved one unchanged-content
reference in the falsifiable-packet spec by nine lines; the repinner's second pass
found no further move. `make line-citations-check` passed, log SHA-256
`855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba`.
