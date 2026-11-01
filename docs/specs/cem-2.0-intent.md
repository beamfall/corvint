# CEM 2.0 intent

Owner: Russell Lewis
Date: 2026-11-01
Requirement prefix: `CEM2-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: the CEM 2.0 counterfactual-receipt plan of 2026-10-24, recorded in full in
V1-2600 to V1-2609 (umbrella V1-2609); `AGENTS.md` invariants 1, 2, 3, 4, 7 and 8;
`docs/specs/cem-stable-v1.md` (the accepted cem/1.0 contract, unchanged here); V1-3759 (no extension
point); V1-3760 (index-based format measurements); V1-3771 (integrity does not bind semantics);
V1-3978 (prototype bounds); V1-3979 (this revision); V1-3980 (content-keyed claims and incremental
closeout); the four CEM audits of 2026-10-31 and the CEM 2.0 audit of 2026-11-01, summarized in
`docs/build-log/2026-11-01-cem2-w14.md`.

## Agent digest
- Claim: CEM 2.0 is the V1-2609 counterfactual-receipt plan; cheaper closeout and a kill-gated value replay come first, a compact 1.0 codec is optional.
- Status: proposed / experimental
- Exists: this intent and the unwired experimental codec prototype `internal/cem2proto` (label `cem/2.0-proto`), with round-trip, refusal, bound and fuzz tests.
- Blocked on: owner acceptance; V1-3980 (content-keyed claims, incremental closeout); the V1-2600 value replay verdict. No cem/2.0 wire, producer, verifier or CLI exists.
- Read next: Plan order; Requirements; Failure modes; Phases and kill criteria.

## Source and its loss

The plan was revised on 2026-10-24 after five expert reviews (all verdicts CONCERNS) and published
as a private claude.ai artifact. That artifact is not readable from the lane hosts, but its content
is not lost: the owner's Tasks store records it in full as V1-2600 to V1-2609 (umbrella V1-2609),
and this revision restates it from those tickets. The first draft of this intent, written on a host
whose Tasks store lacked those tickets, wrongly said no copy existed and rebuilt the plan from the
V1-2601 section names and the 2026-10-31 audits; that reconstruction dropped the user job, north
star, guardrails, stop-loss and the V1-2600 kill experiment, and is superseded here. Where this
intent still departs from V1-2600..V1-2609, the departure is named and carries no authority until
the owner accepts it (invariant 8).

## User and measurable job

User job (V1-2609): before merging an agent's change, show which changed behaviour no executed check
would notice, and what the agent never looked at.

North star: escaped-defect capture against a coverage-diff baseline on held-out external
repositories.

Guardrails:

- flagged units per change and the false-alarm rate, both reported with their denominators;
- cost no greater than 1.0 times today's closeout (target about five times cheaper);
- an incremental refresh equals a full rebuild, byte for byte;
- zero invented certainty (invariant 2).

Stop-loss: stop the plan if Entire or Agent Trace add verification fields that do the job, if Qodo
or Copilot ship per-PR mutation, if cem/1.0 gets no adopters, or if the V1-2600 replay does not
beat LLM reviewers.

The measured cost today is the workflow, not the format: lane reruns (V1-1629: 855
`dogfood-change` runs, 27.9 h, where the protocol needs about two per lane), seals of 8 to 25
minutes per lane (V1-3667), and a 5m36s reseal for a four-hunk change (V1-3475). The 180 committed
cem/1.0 maps on base e9a5d6c8 are about 1.02 MB packed in Git. No measurement shows that CEM bytes
or verification time are a material cost for a consuming agent or reviewer.

## Counterfactual-receipt thesis

A receipt is worth carrying for what a reviewer cannot otherwise see: which changed behaviour no
executed check discriminates, and which inputs the agent never read. The plan's core components
(V1-2602 to V1-2604) produce those facts on the existing 1.0 model; the trust components (V1-2607)
say who produced them and whether anyone reproduced them. Whether a cited span supports a claim
stays outside the format (V1-3771).

## Verified current state

- The accepted cem/1.0 contract is `docs/specs/cem-stable-v1.md`; its decoder is closed, yet
  `provenance` and `role` were added under the unchanged `cem/1.0` label (V1-3759). Additive
  profiles exist (`docs/specs/cem-additive-profiles-v0.md`).
- The published attestation predicate is `https://corvint-context.dev/attestation/cem/v2`
  (`internal/attest/cem_v2.go`, CEM-V1-065). It signs the exact cem/1.0 map bytes together with the
  target commit and tree; this intent keeps it.
- `interop/cem01-go` compares the target's 1.0 sidecar bytes, so replacing the sidecar would break it.
- The five trust classes are `project-authority`, `repository-content`, `repository-history`,
  `external-provider` and `tool-output` (`internal/contextindex/trust.go`, TCP-V0-023).
- The tamper matrix records ACCEPT for `span-hash-recomputed`, `evidence-ids-swapped` and
  `hunk-reorder`, and hunk order is never checked against patch order (V1-3771).
- The codec prototype refuses criteria, receipts, links, artifacts and witnesses, so it is a
  restricted citation codec, not a lossless 1.0 encoding.
- Codec measurements on this base's 180 committed maps are in `docs/build-log/2026-11-01-cem2-w14.md`.

## Plan order

1. Content-keyed claims and incremental closeout first (V1-3980): the measured cost. Claims persist
   keyed by path, edit content and unique BASE anchors under the V1-2604 identity ladder (blob OID,
   then patch-id, then symbol, then inherited-unverified) and compile to exact 1.0 hunk ids at bind.
   One incremental refresh replaces reruns; checks are cached by their complete inputs; evidence
   objects are detached and immutable; preparation runs in parallel with serialized publication.
2. The V1-2600 value replay, the kill gate before any 2.0 wire work: 30 to 50 changes,
   `prove --mutate` and `cem verify --hunk-revert`, thresholds X and Y preregistered by the owner,
   verdict CONTINUE or KILL.
3. Only on CONTINUE, the V1-2609 phases on the 1.0 wire: P1 C1; P2 C2, A1 and A3; P3 C3 and T1 to
   T3; P4 wire, second consumer, A4 and A5 (V1-2608: wire and standards last).
4. The compact codec, at any time, only as an optional transport experiment over the unchanged 1.0
   logical model (CEM2-V0-002 to -005). It is not a delivery dependency of any step above.

## Plan components (V1-2602 to V1-2608)

- C1 (V1-2602): typed discrimination labels per changed unit (`discriminated`,
  `type-discriminated`, `weak-test`, `likely-equivalent`, `uncovered`), each reported with its
  denominator.
- C2 (V1-2603): a residual-risk report; binding of context pack to patch to receipt; ingestion of
  Agent Trace, Entire and git-ai records as attributed observations.
- C3 (V1-2604): layered identity, the expected inventory from `affected`, per-stage budgets, and
  incremental equals full rebuild.
- A1 to A3 (V1-2605): `cem preview` on the uncommitted tree; blast-radius pre-commitment; a
  stop-condition API.
- A4 and A5 (V1-2606): the reviewer's falsification duty; a spend ledger.
- T1 to T4 (V1-2607): a provenance axis (self-reported, signed, reproduced,
  independently-reproduced); reproduction verdicts; evidence-ledger tree heads;
  `corvint-trusted-keys/2`; the name "Witness" reserved for independent cosigners.
- V1-2608: wire and standards last.

Deferred by the plan: behaviour flows, the learning stage, non-test witness classes beyond types and
fuzz, hermetic execution beyond Go, and an external transparency log.

Boundaries this revision adds to the audit's addition candidates:

- A1: an exit code alone is not an executed-verification receipt.
- A2: same-change intent stays visibly separate from BASE evidence.
- A3 and A4: derivation is deterministic and carry-forward is conservative; A3 and A4 together are
  the first workflow proposal (V1-3980).
- A5: a "what to doubt" ordering is explanatory only and never changes a verdict.
- T1: a producer statement names its signer's scope and its subjects.
- T2: a test-selection proof shows what was selected, not that the selection is adequate.
- T3: keys supplied with a capsule cannot authorize themselves.
- T4: currentness against a later revision is separate from the historical verdict.

Trust sentence: Git supplies content-addressed facts; independently pinned policy supplies
authority; authenticated observers supply attributed observations.

## Requirements

Each of CEM2-V0-001 to -022 opens with its 2026-11-01 audit disposition, KEEP or CHANGE; a CHANGE
requirement is stated in its revised form. CEM2-V0-023 to -025 are new.

- `CEM2-V0-001`: KEEP. Until the owner accepts this intent, no producer, verifier, CLI, hook,
  release note or document may emit, accept or advertise a CEM 2.0 capability as delivered. Any
  prototype is labelled experimental, is imported by no non-test package outside itself, and uses
  the non-final label `cem/2.0-proto`.
- `CEM2-V0-002`: CHANGE. Work is prioritized by measured workflow cost: V1-3980 first, then the
  V1-2600 replay as the kill gate before any 2.0 wire work. A consumer that needs maps beyond the
  4096-entry tree envelope uses the qualified route (`verify-stable-canonical`) rather than a blanket
  bound change. cem/1.1 is not a prerequisite. A codec wire is considered only after a measurement
  shows that CEM bytes or verification time are a material consumer cost.
- `CEM2-V0-003`: CHANGE. Attestation stays the published cem/v2 predicate over the projected
  canonical cem/1.0 bytes plus the exact target commit and tree. A codec header is a transport
  envelope, not the signed subject, and mints no predicate type. The prototype header is a closed
  canonical JSON object with exactly `spec`, `base`, `patchSha256`, `bodySha256`, `role`, `parents`
  and `critical`; final header members are deferred to the wire phase (CEM2-V0-016).
- `CEM2-V0-004`: CHANGE. The codec is either a restricted citation transport that declares what it
  cannot carry (CEM2-V0-020), or lossless for every cem/1.0 member; it is never presented as a
  complete 1.0 encoding while it is restricted. The prototype body is closed canonical JSON in index
  form: a `paths` table, `evidence` rows of `[pathIndex, start, end]` with optional pinned `blobOid`
  and `spanSha256`, `hunks` rows of `[reasonCode, [evidenceIndex, relationCode]...]` in
  canonical-patch order, and an optional verbatim `provenance`. Spans are byte offsets into the base
  blob. Hunk ids, hunk paths and ranges, blob ids, span hashes, evidence ids, dispositions and
  `excludedPath` are not carried; consumers compute them.
- `CEM2-V0-005`: CHANGE. Projection is a compatibility oracle: from the same independently supplied
  base, target, artifacts, trust inputs and profile, a consumer MUST rebuild the canonical cem/1.0
  bytes of the map the codec encodes, byte for byte, and the cem/1.0 verifier's verdict on that
  projection is the verdict. The committed 1.0 sidecar stays authoritative; a codec copy, if ever
  stored, has its own specified placement and never replaces the sidecar. The codec adds transport,
  not acceptance semantics.
- `CEM2-V0-006`: KEEP. The body's hunk count MUST equal the canonical patch's hunk count, and hunk
  claims bind by patch ordinal; reordering body rows yields a map with permuted claims, never an
  equivalent one. Ordinal binding gives no per-claim content binding across a rebase with an equal
  hunk count; that binding is V1-3980's.
- `CEM2-V0-007`: CHANGE. Projection MUST refuse, before any 1.0 bytes are produced, a `bodySha256`
  that does not match the body, a `patchSha256` that does not match the recomputed patch, a
  hunk-count mismatch, an index or code out of range, a non-canonical or unknown-member header or
  body, an invalid span (CEM2-V0-023), a reference over its limit (CEM2-V0-024), and a pinned
  `blobOid` or `spanSha256` that differs from Git. Authentication and target failures are explicit
  refusals, never a silent fallback. An unknown `critical` entry yields UNSUPPORTED (abstention),
  never ACCEPT.
- `CEM2-V0-008`: KEEP. The reason and relation code tables are closed and frozen per spec label; the
  prototype's tables are the cem/1.0 reason and relation sets in the order recorded in
  `internal/cem2proto`. A code outside its table is refused.
- `CEM2-V0-009`: CHANGE. A codec map is verifiable only while the complete object closure it needs
  is retained: the base and target commits, their trees and every cited blob. A consumer without
  that closure abstains (UNSUPPORTED or NOT_OBSERVED with a reason), never ACCEPT. A producer emits a
  derived-mode map only for a base the repository's retention policy keeps.
- `CEM2-V0-010`: CHANGE. A consumer MUST bound compact input, expanded output, bases per hunk (32,
  as cem/1.0), aggregate basis references, Git reads and runtime, at limits no lower than cem/1.0's,
  and refuse an over-bound input with a typed refusal. Input bounds are checked before any Git read.
  The prototype bounds a header at 16384 bytes, checked before parsing, and at 64 entries in each of
  `parents` and `critical`; Git blob and runtime bounds are those of `internal/gitauth` and
  `internal/gitrun`.
- `CEM2-V0-011`: KEEP. Neither header nor body carries a numeric confidence, score or probability.
  Uncertainty is a disposition, a reason or an abstention (invariant 2).
- `CEM2-V0-012`: KEEP. The invariant-4 ledgers (`.corvint/self-observations.jsonl`,
  `.corvint/unplanned-reads.jsonl`) and agent tool traces are never 2.0 evidence, basis or
  disposition inputs; a design that would read them needs its own owner-accepted amendment.
- `CEM2-V0-013`: CHANGE. Any trust label attached to 2.0 evidence or claims uses only the five
  TCP-V0-023 classes. `inherited` is not a trust class: it is orthogonal provenance that names the
  verified parent claim a claim was carried from, and a claim whose parent is not verified is
  `inherited-unverified` on the V1-2604 identity ladder.
- `CEM2-V0-014`: KEEP. The append-only record of verdicts and receipts is called the evidence ledger
  in every 2.0 document and wire name, never a witness log.
- `CEM2-V0-015`: CHANGE. The agent and trust additions (A1 to A5, T1 to T4) are admission
  candidates within the boundaries in Plan components. Each needs its own accepted requirement,
  measured value and evaluation gate; this spec admits none. A3 with A4 is the first workflow
  proposal, delivered as V1-3980.
- `CEM2-V0-016`: KEEP. The wire is codified last. No `cem/2.0` label, schema, conformance vector set
  or error-code registry entry is frozen until this intent is accepted and the V1-2600 replay returns
  CONTINUE.
- `CEM2-V0-017`: CHANGE. A 2.0 wire defines its own extension registry, the binding of each
  extension's payload, and the behaviour for an unsupported entry (UNSUPPORTED, never ACCEPT); a body
  member that is not defined is refused. Any dependency on a cem/1.1 label needs a stated reason,
  since additive profiles already extend 1.0.
- `CEM2-V0-018`: CHANGE. Delta maps are unsupported: the prototype refuses a non-empty `parents`,
  no byte saving is credited to delta, and `parents` reserves no parent semantics.
- `CEM2-V0-019`: CHANGE. Neither mode carries hunk ids, hunk ranges or evidence ids, and derived mode
  carries no span hash, so the forged-evidence-id, moved-hunk-range and forged-hunk-id vectors (and,
  in derived mode, span-hash-mismatch) are redundant assertions the codec makes unrepresentable; a
  false claim stays representable. The checked encoder, which projects against Git, MUST refuse a
  1.0 input whose hunk ids, paths or ranges differ from the rebuilt ones
  (`cem2-hunk-identity-mismatch`) and any other projection difference (`cem2-projection-mismatch`);
  pinned mode also refuses a stale or forged blob id or span hash (`cem2-pinned-mismatch`). Signing
  binds the projected cem/1.0 bytes, so an unauthorized alteration of them is a signature failure,
  which is distinct from a false claim; equivalent transport encodings of one map (derived or
  pinned) project to the same bytes and are not authenticated byte-for-byte. A recomputed span hash after a range edit and swapped evidence
  indices remain admissible claims; whether they are semantic tampers stays a V1-3771 decision.
- `CEM2-V0-020`: KEEP. The prototype encoder abstains with UNSUPPORTED on any map it cannot encode
  losslessly: non-empty criterion, receipt, link or artifact arrays, coverage or discrimination
  witnesses, an explicit `role` of `change`, a spec other than cem/1.0 or another `excludedPath`, an
  evidence id that is not its derived identity or repeats, a disposition that is not its reason's,
  and a reason, relation or basis it cannot index. Hunk order needs the patch, so a map not in
  canonical-patch order surfaces as a projection byte mismatch.
- `CEM2-V0-021`: CHANGE. Before any codec promotion, its tests cover every legal cem/1.0 feature it
  claims, hostile and fuzzed inputs (CEM2-V0-025), a pinned committed-corpus projection, and verdict
  parity between the native verifier and an independent consumer. The prototype covers fixture
  round-trips, a committed-corpus round-trip (a deterministic sample by default, every committed map
  on request), one test per CEM2-V0-007 refusal, the CEM2-V0-019 vectors and the CEM2-V0-023 to -025
  cases.
- `CEM2-V0-022`: CHANGE. Size figures are unsigned representation bytes and are labelled so. A
  promotion measurement covers the complete signed transport, the encode, project and verify stages,
  cold and warm latency distributions, Git storage, review usage and closeout effort, and keeps every
  map the codec cannot represent in the denominator. The prototype's committed test re-takes the
  V1-3760 representation measurements on the committed maps and records them in the build log, with
  the corpus size and base named.
- `CEM2-V0-023`: An evidence span MUST satisfy `0 <= start < end`, with both ends wire integers; a
  span that does not is refused as malformed before any Git read, and projection also refuses a
  negative start supplied in-process. The end is then checked against the base blob's length.
- `CEM2-V0-024`: A hunk row MUST carry at most 32 bases, and a body MUST carry at most
  `wire.MaxMapBytes` divided by the shortest canonical basis object (119 bytes) basis references in
  total; both are refused before any Git read. A projection whose canonical 1.0 bytes exceed
  `wire.MaxMapBytes` is refused as over-bound, `cem2-bound-exceeded`.
- `CEM2-V0-025`: Projection never panics on any header or body: every hostile or fuzzed input yields
  a typed refusal or a projection that parses as canonical cem/1.0.

## Non-goals and simpler baseline

- No change to cem/1.0 bytes, verdicts, the frozen sections of `cem-stable-v1.md`, the native
  verifier, the cem/v2 predicate or `interop/cem01-go`.
- No producer path, CLI, hook, MCP surface, release or migration of committed maps.
- No hosted log, network signer, account or service (invariant 7).
- No numeric confidence and no new authority from learned traces (invariants 2, 3, 5).
- No pinned codec mode by default; pinned mode survives only as a justified diagnostic or import
  option.
- Reconstructed codec slots are not a delivery dependency of any plan component.
- Simpler baseline: keep cem/1.0 and compress at rest. Per-file gzip reaches 23.2% of raw 1.0
  representation bytes on the committed corpus; the codec must beat this baseline on a measured
  consumer cost to proceed. The patch-to-CEM byte ratio is NOT_OBSERVED on this base.

## Trust boundary and failure modes

The body is producer-authored and untrusted; Git objects at the independently supplied base and
target are the trusted content inputs; independently pinned policy is the authority; the cem/v2
attestation over the projected 1.0 bytes and exact target is the signed statement. Every failure
closes to a refusal or an abstention, never to ACCEPT.

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
| Span not `0 <= start < end` | refused before any Git read, `cem2-malformed` |
| Base path absent, or span end past the base blob | refused, `cem2-evidence-unresolved` |
| Body over a cem/1.0 bound (bytes, hunks, evidence, path bytes) | refused before any Git read, `cem2-bound-exceeded` |
| Over 32 bases in a hunk, or over the aggregate reference bound | refused before any Git read, `cem2-bound-exceeded` |
| Projected 1.0 bytes over `wire.MaxMapBytes` | refused, `cem2-bound-exceeded` |
| Header over 16384 bytes, or over 64 `parents` or `critical` entries | refused before any Git read, `cem2-bound-exceeded` |
| 1.0 input whose hunk id, path or range differs from the patch (checked encoder) | refused, `cem2-hunk-identity-mismatch` |
| 1.0 input that does not project back byte-identically (checked encoder) | refused, `cem2-projection-mismatch` |
| Base, target or a cited blob absent | abstain, never ACCEPT |
| 1.0 map the encoder cannot represent | encoder UNSUPPORTED, no 2.0 bytes |

These codes are local to the prototype and are not registry entries (CEM2-V0-016).

## Phases and kill criteria

1. Phase 0 (V1-3980): content-keyed claims and incremental closeout. Accept on a differential test
   showing incremental equals full rebuild, a replay of 20 fix sequences with zero incorrect
   carry-forwards, at least 50% fewer manual row edits, at least 30% lower p95 closeout time, and
   tombstones for deleted rows. Kill the approach if incremental and full rebuild ever differ.
2. Phase 1 (V1-2600): the value replay. KILL stops all 2.0 wire work; the stop-loss conditions apply
   at every later phase.
3. Phases P1 to P4 of V1-2609, each gated by its own component tickets.
4. Codec experiment (optional, any time): the prototype round-trips the committed corpus. Kill the
   codec if any committed map fails to project byte-identically for a reason inherent to the format,
   or if it does not beat compression at rest on a preregistered consumer cost.

## Measurements

All sizes are unsigned representation bytes, measured on the 180 committed maps at base e9a5d6c8
(`TestCEM2V0022CorpusMeasurements`, build log):

| Form | Raw | gzip per file | gzip of whole corpus |
|---|---|---|---|
| cem/1.0 | 5,567,310 | 1,290,574 (23.2%) | 1,234,628 |
| derived header and body | 340,815 (6.1%) | 148,286 (2.7%) | 92,308 |
| pinned header and body | 920,075 (16.5%) | 408,453 (7.3%) | 339,956 |

The mean header is 278.0 bytes, and 180 of 180 maps round-trip in 17.9 s (mean 99 ms per map,
repository open plus projection only). These figures exclude any signature or attestation. The
format audit's delta saving and the claim that key names are the majority of 1.0 bytes were not
reproduced on this base: NOT_OBSERVED.

## Acceptance evidence and rollback

- This spec, its README row and the generated requirement rows.
- `internal/cem2proto` tests: round-trip on fixtures and on the committed corpus, one test per
  refusal, the CEM2-V0-019 vectors, the CEM2-V0-023 to -025 bound, hostile and fuzz tests, and the
  measurement test, run on WSL with the commands and results recorded in the build log.
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
| CEM2-V0-007, -017, -018 | `internal/cem2proto/project.go` | `TestCEM2V0007BodySha256Mismatch`, `TestCEM2V0007PatchSha256Mismatch`, `TestCEM2V0007HunkCountMismatch`, `TestCEM2V0007IndexOutOfRange`, `TestCEM2V0007CodeOutOfRange`, `TestCEM2V0007MalformedBody`, `TestCEM2V0007UnknownCriticalIsUnsupported`, `TestCEM2V0007ParentsAreUnsupported`, `TestCEM2V0007PinnedMismatch`, `TestCEM2V0007EvidenceUnresolved` |
| CEM2-V0-010 | `internal/cem2proto/format.go` | `TestCEM2V0010BoundsRefusedBeforeGit`, `TestCEM2V0010HeaderBoundsRefusedBeforeGit` |
| CEM2-V0-019 | `internal/cem2proto/format.go`, `encode.go` | `TestCEM2V0019DerivedVectorsAreUnrepresentable`, `TestCEM2V0019CheckedEncodeRefusesForgedInput`, `TestCEM2V0019AdmissibleClaimEdits` |
| CEM2-V0-020 | `internal/cem2proto/encode.go` | `TestCEM2V0020EncoderAbstains` |
| CEM2-V0-021, -022 | `internal/cem2proto/*_test.go` | `TestCEM2V0022CorpusMeasurements`, build log |
| CEM2-V0-023 | `internal/cem2proto/format.go`, `project.go` | `TestCEM2V0023SpanRefusedBeforeGit`, `TestCEM2V0023ResolveRefusesNegativeStart` |
| CEM2-V0-024 | `internal/cem2proto/format.go`, `project.go` | `TestCEM2V0024BasesPerHunkBound`, `TestCEM2V0024ReferenceTotalBound`, `TestCEM2V0024MinProjectedBasisBytes`, `TestCEM2V0024ExpandedOutputBound` |
| CEM2-V0-025 | `internal/cem2proto/project.go` | `TestCEM2V0025HostileBodiesNeverPanic`, `FuzzCEM2V0025Project` |
| CEM2-V0-002, -009, -011 to -015 | intent only | owner decision pending |

## Unresolved decisions

- Reconstructed codec slots: proposed DROP as a delivery dependency.
- Pinned mode: proposed DROP from the default; keep only as a justified diagnostic or import option.
- Tamper framing: proposed CHANGE to binding the exact authored bytes and distinguishing an
  unauthorized alteration from a false claim (CEM2-V0-019).
- Target binding: proposed CHANGE to a mandatory signed exact-target binding in the detached cem/v2
  attestation, not a header member (CEM2-V0-003).
- The V1-2600 thresholds X and Y, which the owner preregisters before the replay runs.
- Which of the V1-3771 vectors are semantic tampers, and whether 2.0 should bind evidence order.
