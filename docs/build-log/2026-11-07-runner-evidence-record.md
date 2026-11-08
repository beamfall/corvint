# Runner evidence record and immutable index extension

Ticket: V1-5103. Accepted intent: decision 0592 and
`docs/specs/runner-selection-plugins-v0.md`. Lane base:
`f42df32d12bae40c3bfcc6bb8a48442966e4a71d`. The requested pre-change dogfood and
focused affected-plan base is `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`.

## Delivered boundary

- RSP-V0-005, RSP-V0-006: exported native and core-trace inputs, record components,
  identities and producer bindings; bounded JSON round trips for present, absent
  and incomplete states. Malformed states, members, paths, ranges, identities and
  capped records refuse with named reasons. These are record APIs, not admission.
- RSP-V0-007, RSP-V0-049: lexical/resolved names, case-fold matching, repository IDs,
  immutable blob and link identities, proven-new-content read suppression, and
  outside-path audit counts. Undeclared outside reads and unrepresentable
  cross-repository links stay incomplete. Sibling pin drift invalidates freshness.
- RSP-V0-008: the existing gob index accepts an additive evidence field. Existing
  collectors and index format names remain unchanged. Go/JS legacy records expose
  executed ranges and covered-but-unexecuted ranges, with all other components
  absent and no invented runner, target or provenance identities. Mixed indexes
  retain tests present only in legacy coverage. Sharded merge preserves evidence
  and refuses duplicate target keys. Extension writes enforce the inherited size
  ceiling before publishing bytes.
- RSP-V0-009, RSP-V0-054: alone/qualified-marker attribution, shared-tree union,
  modeled build activity only in loaded, and unmodeled build trace on every test
  of that build. Trace collection itself remains outside this slice.
- RSP-V0-010, RSP-V0-053: blob, executable, runner and execution freshness, with
  repository/test/OS/architecture keys. Missing snapshot entries cannot establish
  freshness; target reuse is never inferred. Diff-level decisions remain T2.
- RSP-V0-055, RSP-V0-058: distinct collection counts, conservative component and
  range unions, nondeterminism, and HMAC-SHA256 environment digests. Replaying a
  collection does not satisfy the two-collection prerequisite. Digest framing is
  deterministic; rotation invalidates even an empty environment's digest.

## Implementation decisions and compatibility

The requested package name already held retained-output helpers and a read-scope
entry. The new APIs coexist there; no existing helper or test assertion changed.
The existing read scope already covers the new package-local goldens.

A union with conflicting blob identities refuses rather than guessing which
commit is newer. A consumer must recollect those blobs. Collection readiness is a
prerequisite only: it never authorizes exclusion or producer admission. A record
with disagreeing observations retains their union and remains nondeterministic;
without the qualified single-collection exception it cannot claim two agreeing
collections. Outside-path counts are saturated audit upper bounds over unions,
never selection inputs. These conservative choices preserve the accepted
fail-closed rule without adding selector or collector behavior.

No command, stderr, exit code, existing golden, core-freeze output, collector or
selection behavior changes. Rollback removes the new APIs and additive index
field; records are derived state and can be rebuilt.

## Evidence and limits

Whole-package tests for `internal/testevidence` and `internal/linecov`, including
all existing tests, pass through `script/heavy.sh`; their positive and negative
RSP-named cases are listed in the spec traceability rows. Both package vets and `make windows-vet` pass.
The 1,000-record fixture encoded to **824,352 bytes**, below **67,108,864 bytes**;
its single encode/decode observation was **17.390711 ms**. This is not a reference
host p95 measurement or a selector latency claim.

`make spec-requirements` regenerated no artifacts. The focused spec and citation
checks pass; repin found no unchanged citation content that moved. Formatting and
staged whitespace checks pass. The final affected receipt selects both touched
packages and retains **80 unknowns** (77 language frontiers and three unowned
new JSON paths). Its earlier read-scope parse failure was an editing mistake,
repaired before the final receipt and checks.

Pre-change context, query and impact calls returned `ok: true`. Raw receipts and
logs stay outside the repository in `/home/russell/lanes/rspt1-evidence/`.
Repository-wide gates, reference-host p95 selection qualification, live tracing,
runner qualification and independent review are NOT_RUN in this lane: the charter
reserves host verification/review, and tracing and selection belong to later
slices. No Tasks completion, publication or seal is authorized by this charter.

Retained local evidence digests (SHA-256):

- `context.json`: `afe468a486df7f8ad9f394369cb7bc3383b5e44fb57cd453d248845a35668d30`
- `query.json`: `9a10d1c21e38bc74efd986fca9508c4e8bb6e28b89b744e21b963b70b384d8da`
- `impact.json`: `dea64301f265db90ccf09cd3a9092ddb36cd7748befc2832d8950e401c378342`
- `affected-final.json`: `f09cb85906b7256d4a395dc5924fc183544128e74b7fd1962733b95e65381778`
- `test-testevidence-final.log`: `9b02e77386ea06946ebca91502912a4f57b230f95adedf30dd9d578bdf6e8bec`
- `spec-citations-check.log`: `5b0a28d4aad222dd7e0f813b4c6782f3e2a2fa87636987afebddbc35a6dcbb45`
- `test-linecov-final.log`: `15f545e1a5695d03b5f58a869a8057265e7249dfe51cee488a1ed0d43a37d6a8`
- `windows-vet.log`: `019f2f328eb02643776ecc5d43141ce57c095be16eb580e3ba26d9a3a9517e1e`
- `vet-testevidence-final.log`: `6479ff6ec027ca736c39cdccbe62b6cf7fed9cd548a715e87db81962bc8bd3e0`
- `vet-linecov.log`: `0c16b0a182dd70639703bf97d508653ecd0891cf795ab60c76f8297eee25d4ca`
