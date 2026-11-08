# reuse: review round 4 (existence readers, checkout bytes, attribute changes, snapshot drift)

Base: `6f12f4704235dc8f9bea19fc12995bfb68c746c3`. Tickets: V1-1097, V1-1099. Requirements amended
in place, with IDs unchanged: AFP-V0-2320, AFP-V0-2321, AFP-V0-2322, AFP-V0-2325 and AFP-V0-2342
in `docs/specs/affected-plan-reuse-v0.md`. An independent review of the round-3 head `2b010e517`
returned three P1 findings and one P2. Each is fixed with a regression test that fails against
the reviewed implementation. Each fix refuses where it cannot prove soundness.

## Findings and decisions

1. **Existence readers were eligible (P1, AFP-V0-2320).** Only the affected graph's
   `UNBOUNDED_READER` set fed eligibility. A package declared in `.corvint/existence-readers.json`
   (AFP-V0-980) stats paths that no digest binds and the confinement does not deny, so deleting
   its stat target left the digest unchanged. Decision: refuse every declared existence reader
   with `EXISTENCE_READER`, checked after `UNBOUNDED_READER` and before `SLICED_UNIT`.
   Rejected alternative: binding each declared target's presence into the digest, since the
   declaration names manifests, not every path a test may stat. Undeclared existence reads stay
   a recorded residual. Evidence: `TestTestReuseExistenceReaderRuns` runs a real confined
   `go test`, records, deletes the stat target and commits. On the reviewed code both packages
   were `REUSE RECORD_DIGEST_EQUAL`. After the fix `a` is `RUN EXISTENCE_READER`, `b` (not
   declared) is still reused as the negative control, and the real rerun of `a` fails.
2. **A clean status did not prove the executed bytes (P1, AFP-V0-2321, AFP-V0-2322).** A root
   `.gitattributes` with `data/a.txt text eol=crlf` rewrites the checkout to CRLF while the blob
   and the status stay unchanged. Decision: after the dirty check, read every regular-file digest
   input and harness file raw from the worktree. Its Git blob hash (SHA-1 or SHA-256, by HEAD's
   object format) must equal the tree blob, and outside Windows its owner executable bit must
   match its mode; otherwise `worktree-bytes`. This covers `.gitattributes` at any depth,
   `info/attributes`, `core.autocrlf`, filters and `core.fileMode=false` without parsing
   attributes. Rejected alternative: parsing attributes, which misses configuration outside the
   tree. Results are cached per snapshot. Evidence:
   - `TestCheckoutAttributeTransformationRuns`: a `-diff` attribute is the negative control. On
     the reviewed code the CRLF checkout was reused and the harness accepted. Now `a` is
     `DIGEST_ERROR worktree-bytes: data/a.txt`, the harness is refused, and the real rerun fails.
   - `TestHiddenModeChangeRuns`: an executable bit hidden by `core.fileMode=false`.
3. **Job predicates ignored checkout attributes (P1, AFP-V0-2342).** An attribute-only commit
   changed a declared input's checkout bytes and every job was `SKIP`. Decision: any changed
   `.gitattributes`, at any depth, runs every job with `CHECKOUT_ATTRIBUTES_CHANGED`, after
   `DECLARATION_CHANGED`. This is a fail-closed superset of the ancestors of the declared inputs;
   attribute edits are rare, so the precision cost is small. `.corvint/ci-job-inputs.json` is
   unchanged: the rule is in `Evaluate`, so no declaration can omit it. Evidence:
   `TestCheckoutAttributesRunEveryJob`. An unrelated path is the negative control. A root
   attribute commit rewrites `interop/m/a.go` to CRLF, so `gofmt -l` reports it, and the job now
   runs. On the reviewed code every job was `SKIP NO_DECLARED_INPUT_CHANGED`.
4. **The snapshot never checked for drift (P2, AFP-V0-2321).** `Observe` read the status once
   and then listed packages, so an edit during `go list` was digested from stale state.
   Decision: `Observe` records HEAD, its tree and digests of the status and index-flag reads,
   and `Snapshot.Recheck` rereads and compares them. `Observe` calls it last. The CLI now
   observes first, then builds the affected graph and the identity, then rechecks, so all three
   describe one checkout. Drift is `snapshot-drift`, exit 3 `test-reuse-incomplete`. Evidence:
   - `TestConcurrentEditDuringListingIsDrift`: a go wrapper edits a scoped fixture during
     `go list -deps`. On the reviewed code the observation succeeded and `a` was reused. Now it
     is refused and the real rerun fails.
   - `TestRecheckRefusesLaterDrift`: an unchanged checkout and an undone edit pass; a pending
     edit, a new commit and an unobserved snapshot are refused.
