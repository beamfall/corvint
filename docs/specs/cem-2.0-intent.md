# CEM 2.0 intent

Owner: Russell Lewis
Date: 2026-11-01
Requirement prefix: `CEM2-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: repository-owner split of 2026-10-24 recorded in V1-2601; `AGENTS.md`
invariants 1, 2, 3, 4, 7 and 8; `docs/specs/cem-stable-v1.md` (the accepted cem/1.0 contract,
unchanged here); V1-3759 (no extension point); V1-3760 (index-based format measurements); V1-3771
(integrity does not bind semantics); the four CEM audits of 2026-10-31 (format, trust, agent
production cost, usage), summarized with their verdicts in `docs/build-log/2026-11-01-cem2-w14.md`.

## Agent digest
- Claim: Proposed CEM 2.0 splits a map into a signed header and a Git-derived index body that consumers project back to byte-identical canonical cem/1.0.
- Status: proposed / experimental
- Exists: this intent and the unwired experimental prototype `internal/cem2proto` (label `cem/2.0-proto`), with round-trip and tamper tests over the committed corpus.
- Blocked on: owner acceptance of this intent; the consumer bound, sidecar and base-retention fixes (F1-F3); the cem/1.1 extension point (V1-3759). No cem/2.0 wire, producer, verifier or CLI exists.
- Read next: Requirements; Failure modes; Phases and kill criteria; Acceptance evidence and rollback.

## Source and its loss

V1-2601 cites a revised CEM 2.0 plan of 2026-10-24 (five expert reviewers, all verdicts CONCERNS)
published as a private claude.ai artifact. That artifact is not readable from any lane host and no
copy exists in the repository, the Tasks store or any branch. This spec rebuilds the plan's named
sections from the V1-2601 ticket body, which lists the section names and the six corrections, and
from the four 2026-10-31 audits. Where the original content of a named slot (C1-C3, A1-A5, T1-T4)
is unknown, the slot is filled from the audits and marked reconstructed. The owner may remap any
slot; a reconstruction carries no authority of the lost plan (invariant 8).

## User and measurable job

A reviewer, or the next agent to touch a change, needs to know for each hunk what pinned evidence
the producer cited, which claims nobody could support, and what was actually executed, and needs
to check that offline from Git. Today the cem/1.0 map answers the first two at a cost: 180
committed maps on this base (5.6 MB, median 27.8 KB), of which key names and derived hashes
are the majority of bytes, and closeout time that is dominated by reruns rather than by the format
(usage audit finding 1). The measurable job is: a consumer holding the base and target Git objects
verifies the same claims from a header plus body that is a small fraction of the 1.0 bytes, and
reaches the same 1.0 verdict.

## Counterfactual-receipt thesis (reconstructed)

A receipt is worth carrying only for what a consumer cannot recompute. Hunk identities, ranges,
blob ids, span hashes, evidence ids and dispositions are all functions of Git content plus the
producer's claims, so a 2.0 map carries only the claims (reason, cited spans, relations) and lets
the consumer recompute the rest. A claim then binds to the patch ordinal it describes, so a map
that says something different is a different map, not an equivalent re-encoding. The thesis does
not make citation semantically adequate: whether a cited span supports a claim stays outside the
format (V1-3771).

## Verified current state

- The accepted cem/1.0 contract is `docs/specs/cem-stable-v1.md`; its decoder is closed, yet
  `provenance` and `role` were added under the unchanged `cem/1.0` label (V1-3759).
- The published attestation predicate is `https://corvint-context.dev/attestation/cem/v2`
  (`internal/attest/cem_v2.go`, CEM-V1-065). This intent keeps it.
- The five trust classes are `project-authority`, `repository-content`, `repository-history`,
  `external-provider` and `tool-output` (`internal/contextindex/trust.go`, TCP-V0-023).
- The tamper matrix records ACCEPT for `span-hash-recomputed`, `evidence-ids-swapped` and
  `hunk-reorder`, and hunk order is never checked against patch order (V1-3771).
- Measurements re-taken on this base's 180 committed maps are in the build-log entry above.

## Plan sections (reconstructed slots)

Core:

- C1: index-based, Git-derived body with consumer-computed identities (CEM2-V0-004, -005).
- C2: small header that binds base, patch and body digests and is the signed unit (CEM2-V0-003).
- C3: the extension point and version-label rule, owned by the cem/1.1 work in V1-3759
  (CEM2-V0-017). V1-2604, filed as "CEM 2.0 C3", stays blocked on owner intent; whether its
  original scope matches this slot is unknown.

Agent additions (from the agent-cost audit; each an admission candidate, CEM2-V0-015):

- A1: executed verification bound as runner receipts, with OUTCOME derived from exit codes.
- A2: a `same-change-intent` disposition, visibly separate from BASE evidence.
- A3: closeout inputs that default to derived values, with files as overrides.
- A4: incremental reseal keyed by a range-free hunk content key (with T-side delta, CEM2-V0-018).
- A5: a deterministic "what to doubt" ordering computed from fields already in the map.

Trust (from the trust audit; each an admission candidate):

- T1: a signed producer statement over the header digest under the published cem/v2 predicate.
- T2: a test-selection proof that a third party re-derives offline to identical bytes.
- T3: an offline reproduction capsule (bundle, map, keys, expected verdict digest).
- T4: currentness of a historical verdict against a later revision, read-only.

Standards work: in-toto/SLSA-style predicate content and DSSE signing, both offline. Sigstore
Fulcio/Rekor and a hosted SCITT log conflict with invariant 7 and are out of scope; a local
append-only log in a Git ref is a deferral.

Deferrals: evidence DAG for batch merges, redaction-stable salted commitments (changes ids, so a
separate profile), signed ledger heads, and delta maps beyond the prototype's refusal.

## Requirements

- `CEM2-V0-001`: Until the owner accepts this intent, no producer, verifier, CLI, hook, release
  note or document may emit, accept or advertise a CEM 2.0 capability as delivered. Any prototype
  is labelled experimental, is imported by no non-test package outside itself, and uses the
  non-final label `cem/2.0-proto`.
- `CEM2-V0-002`: Work beyond the prototype is gated on, in order: the consumer bound fix (F1), the
  sidecar mismatch fix (F2) and base retention (F3) from the format audit, then the cem/1.1
  extension point (V1-3759). A 2.0 wire is considered only after a measurement shows that CEM bytes
  or CEM verification time are a material cost for a consuming agent or reviewer.
- `CEM2-V0-003`: A 2.0 map has a header that is a closed canonical JSON object with exactly the
  members `spec`, `base`, `patchSha256`, `bodySha256`, `role`, `parents` and `critical`. `base` is
  a full commit id, both digests are lowercase sha256 hex, `role` is one of `change`, `bind-chore`
  or `seal-chore`, and `parents` and `critical` are arrays of strings. The target commit stays
  external, as for cem/1.0. The header is the unit a signature covers; signing reuses the published
  cem/v2 predicate and mints no new predicate type.
- `CEM2-V0-004`: The body is closed canonical JSON in index form: a `paths` table, `evidence` rows
  of `[pathIndex, start, end]` with optional pinned `blobOid` and `spanSha256`, `hunks` rows of
  `[reasonCode, [evidenceIndex, relationCode]...]` in canonical-patch order, and an optional
  `provenance` copied verbatim. Hunk ids, hunk paths and ranges, blob ids, span hashes, evidence
  ids, dispositions and `excludedPath` are not carried; consumers compute them.
- `CEM2-V0-005`: A consumer holding the base and target objects MUST project a 2.0 map to the
  canonical cem/1.0 bytes of the map it encodes, byte for byte, and the cem/1.0 verifier's verdict
  on that projection is the verdict. CEM 2.0 adds binding, not acceptance semantics.
- `CEM2-V0-006`: The body's hunk count MUST equal the canonical patch's hunk count, and hunk claims
  bind by patch ordinal; reordering body rows yields a map with permuted claims, never an
  equivalent one.
- `CEM2-V0-007`: Projection MUST refuse, before any 1.0 bytes are produced, a `bodySha256` that
  does not match the body, a `patchSha256` that does not match the recomputed patch, a hunk-count
  mismatch, an index or code out of range, a non-canonical or unknown-member header or body, and a
  pinned `blobOid` or `spanSha256` that differs from Git. An unknown `critical` entry yields
  UNSUPPORTED (abstention), never ACCEPT.
- `CEM2-V0-008`: The reason and relation code tables are closed and frozen per spec label; the
  prototype's tables are the cem/1.0 reason and relation sets in the order recorded in
  `internal/cem2proto`. A code outside its table is refused.
- `CEM2-V0-009`: A 2.0 map is verifiable only while its base and target objects are retained. A
  consumer without them abstains (UNSUPPORTED or NOT_OBSERVED with a reason), never ACCEPT. A
  producer emits a derived-mode map only for a base the repository's retention policy keeps (F3).
- `CEM2-V0-010`: A consumer MUST enforce declared bounds no lower than cem/1.0's (hunks, evidence,
  path bytes, map bytes, patch bytes) and refuse an over-bound header or body with a typed refusal
  before it reads Git objects. Tree-entry limits in a consumer MUST NOT be lower than what the
  repository's committed maps need (F1).
- `CEM2-V0-011`: Neither header nor body carries a numeric confidence, score or probability.
  Uncertainty is a disposition, a reason or an abstention (invariant 2).
- `CEM2-V0-012`: The invariant-4 ledgers (`.corvint/self-observations.jsonl`,
  `.corvint/unplanned-reads.jsonl`) and agent tool traces are never 2.0 evidence, basis or
  disposition inputs; a design that would read them needs its own owner-accepted amendment.
- `CEM2-V0-013`: Any trust label attached to 2.0 evidence or claims reuses the five TCP-V0-023
  classes plus `inherited`, which marks a claim carried from a parent map; no other class is
  introduced.
- `CEM2-V0-014`: The append-only record of verdicts and receipts is called the evidence ledger in
  every 2.0 document and wire name, never a witness log.
- `CEM2-V0-015`: The agent and trust additions (A1-A5, T1-T4) are admission candidates. Each needs
  its own accepted requirement, measured value and evaluation gate; this spec admits none.
- `CEM2-V0-016`: The wire is codified last. No `cem/2.0` label, schema, conformance vector set or
  error-code registry entry is frozen until this intent is accepted and CEM2-V0-002's gates pass.
- `CEM2-V0-017`: Extensions follow the cem/1.1 rule of V1-3759: a consumer that does not implement
  an entry of `critical` returns UNSUPPORTED; a body member that is not defined is refused.
- `CEM2-V0-018`: `parents` names parent header digests for a delta map. The prototype refuses a
  non-empty `parents` as unsupported; delta saves about a quarter of bytes (format audit) and is a
  deferral.
- `CEM2-V0-019`: Derived mode makes the forged-evidence-id, span-hash-mismatch, moved-hunk-range
  and forged-hunk-id vectors unrepresentable, and pinned mode refuses them. A recomputed span hash
  after a range edit and swapped evidence indices remain different, admissible claims; whether
  they are semantic tampers stays a V1-3771 decision.
- `CEM2-V0-020`: The prototype encoder abstains with UNSUPPORTED on any map it cannot encode
  losslessly: non-empty criterion, receipt, link or artifact arrays, coverage or discrimination
  witnesses, an explicit `role` of `change`, a spec other than cem/1.0 or another `excludedPath`,
  an evidence id that is not its derived identity or repeats, a disposition that is not its
  reason's, and a reason, relation or basis it cannot index. Hunk order needs the patch, so a map
  not in canonical-patch order surfaces as a projection byte mismatch.
- `CEM2-V0-021`: The prototype's tests cover fixture round-trips, a committed-corpus round-trip
  (a deterministic sample by default, every committed map on request), one test per CEM2-V0-007
  refusal, and the CEM2-V0-019 vectors.
- `CEM2-V0-022`: The V1-3760 measurements (raw and compressed sizes, header size, round-trip
  counts and time) are re-taken by a committed test on the repository's committed maps and recorded
  in the build log, with the corpus size and base named.

## Non-goals and simpler baseline

- No change to cem/1.0 bytes, verdicts, the frozen sections of `cem-stable-v1.md`, the native
  verifier or `interop/cem01-go`.
- No producer path, CLI, hook, MCP surface, release or migration of committed maps.
- No hosted log, network signer, account or service (invariant 7).
- No numeric confidence and no new authority from learned traces (invariants 2, 3, 5).
- Simpler baseline: keep cem/1.0 and compress at rest. Per-file gzip already reaches about a
  quarter of raw size (format audit), and a CEM is about a tenth of its patch's bytes; 2.0 must beat
  this baseline on a measured consumer cost to proceed.

## Trust boundary and failure modes

The body is producer-authored and untrusted; Git objects at the pinned base and target are the
trusted inputs; the header digest is the signed unit. Every failure closes to a refusal or an
abstention, never to ACCEPT.

| Condition | Required observation |
|---|---|
| Body bytes differ from `bodySha256` | refused, `cem2-body-sha256-mismatch` |
| Recomputed patch differs from `patchSha256` | refused, `cem2-patch-sha256-mismatch` |
| Body hunk count differs from patch | refused, `cem2-hunk-count-mismatch` |
| Path or evidence index out of range | refused, `cem2-index-out-of-range` |
| Reason or relation code out of range | refused, `cem2-code-out-of-range` |
| Unknown `critical` entry or non-empty `parents` | UNSUPPORTED, `cem2-unsupported-critical` or `cem2-unsupported` |
| Pinned blob or span hash differs from Git | refused, `cem2-pinned-mismatch` |
| Header or body not canonical, unknown member, wrong type | refused, `cem2-malformed` |
| Base path or span absent at base | refused, `cem2-evidence-unresolved` |
| Body over a cem/1.0 bound (bytes, hunks, evidence, path bytes) | refused before any Git read, `cem2-bound-exceeded` |
| Base or target objects absent | abstain, never ACCEPT |
| 1.0 map the encoder cannot represent | encoder UNSUPPORTED, no 2.0 bytes |

These codes are local to the prototype and are not registry entries (CEM2-V0-016).

## Phases and kill criteria

1. Phase 0 (this change): intent proposed; prototype round-trips the committed corpus. Kill the
   index-body design if any committed map fails to project byte-identically for a reason inherent
   to the format rather than a prototype abstention.
2. Phase 1: F1-F3 fixed; cem/1.1 extension point accepted (V1-3759); V1-3771 decides which tamper
   vectors are semantic. Kill 2.0 if 1.1 plus compression at rest meets the job.
3. Phase 2: a preregistered measurement of consumer cost (context bytes or verification time) on
   real reviews. Kill the wire if 2.0 does not beat the simpler baseline on that measurement.
4. Phase 3: wire codified with conformance vectors run by both the native verifier and an
   independent consumer; producers keep emitting 1.0 until the owner accepts promotion.

North star: any reviewer can check every hunk claim of a change offline from Git plus a small
signed header, and gets the same verdict as the author. Guardrails: invariants 1, 2, 4 and 7; cem/1.0
remains readable for every committed map; no admission candidate rides in without its own gate.

## Acceptance evidence and rollback

- This spec, its README row and the generated requirement rows.
- `internal/cem2proto` tests: round-trip on fixtures and on the committed corpus, one test per
  refusal, the CEM2-V0-019 vectors, and the measurement test, run on WSL with the commands and
  results recorded in the build log.
- The owner's accept, amend or reject decision, recorded later; until then delivery stays
  experimental and CEM2-V0-001 holds.

Rollback: delete `internal/cem2proto`, this spec and its README row, then regenerate the
requirement index. Nothing else imports or reads them, and no committed map changes.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| CEM2-V0-001, -016 | `internal/cem2proto` (label `cem/2.0-proto`, no importers) | `TestCEM2V0001PrototypeIsUnwired` |
| CEM2-V0-003, -004, -008 | `internal/cem2proto/format.go` | `TestCEM2V0003HeaderIsClosed`, fixture round-trips |
| CEM2-V0-005, -006 | `internal/cem2proto/project.go`, `gitsource.go` | `TestCEM2V0005FixtureRoundTrip`, `TestCEM2V0005CorpusRoundTrip`, `TestCEM2V0006HunkReorderPermutesClaims` |
| CEM2-V0-007, -017, -018 | `internal/cem2proto/project.go` | `TestCEM2V0007*` refusal tests |
| CEM2-V0-010 | `internal/cem2proto/format.go` | `TestCEM2V0010BoundsRefusedBeforeGit` |
| CEM2-V0-019 | `internal/cem2proto/format.go` | `TestCEM2V0019*` |
| CEM2-V0-020 | `internal/cem2proto/encode.go` | `TestCEM2V0020EncoderAbstains` |
| CEM2-V0-021, -022 | `internal/cem2proto/*_test.go` | `TestCEM2V0022CorpusMeasurements`, build log |
| CEM2-V0-002, -009, -011 to -015 | intent only | owner decision pending |

## Unresolved decisions

- Whether the original C1-C3, A1-A5 and T1-T4 contents match these reconstructed slots.
- Whether pinned mode is kept at all once base retention (F3) is guaranteed.
- Which of the V1-3771 vectors are semantic tampers, and whether 2.0 should bind evidence order.
- Whether the target commit belongs in the header rather than staying external.
