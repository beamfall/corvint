# CEM stable 1.0

Intent status: owner scope accepted; technical contract proposed
Delivery status: planned

## Agent digest
- Claim: Owner requires portable task-linked execution evidence and all main runners for Corvint1.0; technical contract and stable promotion remain open.
- Status: owner scope accepted; technical contract proposed/planned; no stable delivery or promotion.
- Exists: experimental runner profiles, native Tasks criterion binding and portable candidate proofs; hidden Core status/verify stable read (proposed `CEM-V1-017..019`), hidden Core report/MCP preview (proposed `CEM-V1-034..036`), hidden Core stable bundle export (accepted `CEM-V1-060..064`), explicit Core mutator refusals of `cem/1.0` (proposed `CEM-V1-030..033`), the internal OCM/LRF stable universe admission (accepted `CEM-V1-051..059`, no stable LRF evaluation) and library-level stable witness, collapse, documentation and console projections (accepted `CEM-V1-070..079`); a provider-neutral plain criterion form verified by both consumers without a Tasks store (proposed `CEM-V1-083`); DSSE Ed25519 runner receipt authentication that rises to VERIFIED only under a consumer `--trusted-keys` file (proposed `CEM-V1-084`); qualification remains bounded.
- Blocked on: stable wire/consumer integration, all supported runtime qualification and independent interoperability/outcome evidence.
- Read next: Owner context; Requirements; Acceptance evidence.

## Owner context

- Owner intent: accepted scope, 2026-10-01 conversation; CEM1.0 belongs in the main Corvint1.0 release and supports actual main test runners across supported languages, linked to native Tasks criteria.
- Technical contract: proposed. No version alias, consumer promotion or release qualification is established by this document.
- Native work: V1-0591; runner lanes V1-0592..0594; portable wire V1-0595; qualification V1-0596. Existing experimental CEX work remains V1-0575.
- Original evidence: `cem-criterion-experiments-v0.md`, `tasks-criterion-binding-v0.md`, `test-runner-execution-v0.md`; immutable Git CEM binding remains governed by `cem-0.2-canonical-binding.md` and `cem-0.3-structural-mechanical.md`.

## Intended result

An agent and an independent reviewer can follow a changed Git hunk to its governing criterion, the exact test inventory executed, native attempts and report bytes, and any unresolved qualification. The evidence says what was observed and what remains unknown. A test failure does not alone demonstrate criterion discrimination, and receipt integrity does not establish semantic adequacy.

## Requirements

- `CEM-V1-001`: Preserve canonical Git base/target/hunk binding and closed versioned decoding. Keep earlier0.x documents readable by their existing consumers; unknown1.0 documents remain explicit unsupported results until a consumer implements1.0.
- `CEM-V1-002`: Bind criterion references to canonical native ticket identity, acceptance revision, ordered acceptance digest and immutable claim-era record. Current applicability is distinct from historical validity.
- `CEM-V1-003`: Reference bounded runner receipts by digest, with exact runner/profile, admitted source inventory, primary and auxiliary executable pins, native report hashes, expected inventory and observed attempts. Declare dependency closure and execution authority limits.
- `CEM-V1-004`: Preserve FAILED, SKIPPED, FLAKY, interruption, timeout, build, collection and infrastructure outcomes. Unknown states, report contradiction, incomplete inventory or changed inputs produce uncertainty. Aggregate reports must not invent individual retries.
- `CEM-V1-005`: Distinguish selected tests, executed tests, coverage and criterion discrimination. A mutant is killed only with an admitted criterion oracle and a classified, reproducible test failure; arbitrary nonzero exit or infrastructure failure cannot satisfy it.
- `CEM-V1-006`: Integrate all main runners supported by Corvint's languages, including concrete browser, JVM, Apple and Android profiles. An import parser, static selector or compiler-only domain probe cannot satisfy execution qualification.
- `CEM-V1-007`: Migrate Core producers/validators/reviewer reports, OCM/frontier/obligation consumers, companion consumers, portable Go consumer and public conformance packet as one evidenced compatibility contract. Inventory every unchanged or unsupported consumer explicitly.
- `CEM-V1-008`: Retain independent external consumer conformance and matched external outcome evaluation separately from Corvint-authored fixtures and self-use. No self-authored consumer establishes independent adoption.
- `CEM-V1-009`: Promote only with the native release's compatibility, focused documentation, integration, interoperability and required terminal release gates. Runtime/platform/toolchain gaps remain blockers in native Tasks and release readiness.
- `CEM-V1-010`: Rollback preserves original0.x producer defaults and emitted historical packets, disables new optional profiles, and avoids rewriting native task history. Release promotion is an explicit operation.

## Non-goals and failure modes

No daemon, network account, broad language rewrite or new specification language is required. Tests remain trusted local executable code; the optional executor does not provide an operating-system sandbox. A declared executable directory does not attest every dependency in that directory. Missing browsers, simulator/device/app identities, unavailable frameworks, unsupported custom harnesses, stale claims and absent independent consumers are retained failure modes, not passes.

## Acceptance evidence

Stable acceptance remains OPEN until the native tickets retain complete qualification, integration and completion evidence. The current implementation has 55 concrete experimental runner profiles: 21 dynamic, 15 native, 16 platform, two SQL and one Appium Android. Six additional listed IDs explicitly refuse execution; the registry listing is not a support claim. Native Tasks capture/verification and the first portable candidate have passed bounded independent review. A same-target disposable proof also passed genuine focused native Tasks qualification, killed all three CEX controls, observed three generic Go passes and verified candidate references with both consumers. Neither this count nor candidate conformance establishes all-supported execution, a stable CEM1.0 wire or release readiness.

| Requirements | Implemented evidence | Remaining acceptance |
| --- | --- | --- |
| CEM-V1-001, 011..013 | `internal/cem/wire/candidate.go`, `internal/cem/verify/candidate.go`; `TestCandidatePortablePacket`; independent `interop/cem01-go` `TestCandidateNormativePacket`; 31 public cases in `protocol/cem-1.0/manifest.json` | Experimental reference profile only; stable profile and 0.3 vocabulary remain unimplemented |
| CEM-V1-002 | Native `internal/tasks/criterionbinding`, CLI read verbs and `internal/criterionexperiment/tasks_unix.go`; TCB-V0 traceability below its owning spec | A coherent same-target native Tasks/CEX/generic Go/candidate join is retained in `/private/tmp/cem10-build/coherent-join/summary.json`; immutable execution-at-commit attestation and integrated candidate production remain open |
| CEM-V1-003..004, 006 | `internal/testrunner`, registry and `cmd/corvint-test-runner`; TRE-V0 traceability and adapter provenance | Full negative/retry/lifecycle/platform matrices and missing application/device/domain tuples remain open |
| CEM-V1-005 | Existing experimental criterion lifecycle retains three killed controls under its own reviewed oracle contract | Cross-language criterion discrimination is not established by runner failures or candidate links |
| CEM-V1-007, 014..016 | Additive native and portable candidate verification plus the optional `internal/cemcandidate` producer and `cmd/corvint-cem-candidate`; byte-compatible runner envelope extraction; actual same-target assembly and independent `/private/tmp/cem10-build/cem-next/review1.json` PASS | Existing producer/report/OCM/frontier/companion consumer migrations and their compatibility inventory remain open |
| CEM-V1-008..010 | Historical defaults and packets retained; optional-profile rollback defined | Independent external consumer, matched external outcomes and terminal release/promotion evidence remain open |
| CEM-V1-051..059 | `lrfrepo.VerifyStableUniverse`, `StableUniverse`, `StableRefusal`, `lrf.ValidateStableRequest`; explicit `cem/1.0` refusals in `VerifyUniverse`, `readOCMBytes`, `PrepareOCM`; tests in the OCM/LRF stable admission traceability table | Stable LRF evaluation needs an accepted stable LRF profile; no CLI or MCP route; OCM writers stay `cem/0.x` only; C06..C09 and C17 consumers remain open |
| CEM-V1-050 | `structuralRuntimeAdmitted` in `internal/cem/verify/stable.go` and `interop/cem01-go/cem.go`; sealed `protocol/cem-1.0/structural-tuples/go1.27.2/`; `TestCEMV1050StructuralTupleAdmission` (native and portable); S0E, linked-control, ledger and cell tests bound through the tuple packet | Linux and Windows hosts, CI-hosted go1.27.2 runs and the full structural corpus remain NOT_RUN |
| CEM-V1-080..082 | `internal/cem/verify/support.go`, `interop/cem01-go/citation.go` and `testdata/citation/`; `TestCEMV1080*`, `TestCEMV1081*`, `TestCEMV1082*` in `internal/cem/verify`, `internal/cem/workflow` and `interop/cem01-go` | Recorded limits: `held` never produced, no `symbol` anchors, no threshold or cut-off, stable-result emission deferred to V1-1265; historical 591-map report in the build log |
| CEM-V1-083 | Plain criterion form in `internal/cem/wire/stable.go` and `interop/cem01-go/cem.go`; public case `protocol/cem-1.0/stable/neutral-criterion/`; `TestCEMV1083*` in `internal/cem/wire`, `internal/cem/verify` and `interop/cem01-go` | No producer emits the plain form (`corvint-cem-candidate assemble-stable` accepts only Tasks-bound `cem-stable-assembly/1`); the public map was hand-assembled; stranger time NOT_OBSERVED |
| CEM-V1-084 | `internal/cem/wire/stable_auth.go`, `internal/cem/verify/stable_auth.go`, `interop/cem01-go/stable_auth.go`; `--trusted-keys` in `internal/cem/cli/stable.go` and both portable commands; public case `protocol/cem-1.0/stable/receipt-authentication/`; `TestCEMV1084*` in `internal/cem/wire`, `internal/cem/verify`, `interop/cem01-go`, `internal/console` and `cmd/corvint` | No producer signs receipts (operator step, FPK-V0-036); `RESULT.schema.json` and the sealed S0E packet still name only `NOT_OBSERVED` for `authentication` and lack the four attestation issue codes (V1-1265); keys are TEST-ONLY; stranger time NOT_OBSERVED |
| CEM-V1-091 | `tools/cem-stable-regen` and the `CEM_STABLE_REGEN_CAPTURE` hook in `internal/cem/verify/stable_repository_test.go`; `TestCheckPassesOnRepositoryPacket`, `TestRegenerateRealPacketCopy`, `TestRegenerateRefusals`, `TestRegenerateSyntheticPacket`; proof run in `docs/build-log/2026-10-11-cem-stable-regen-tooling.md` | No member has been added to the sealed packet yet; V1-1265 consumes the tool for `citation` |

## First portable candidate build contract

The first explicit candidate is `cem/1.0-experimental.1`. Historical ParseMap, Core producer defaults and portable verify/ci continue to refuse it; a separate candidate parser/verifier and portable `verify-candidate` operation provide the earliest interoperability proof. The candidate retains canonical independent base/target patch and exact target-sidecar binding, then adds closed criterion/runner/link reference records. It emits REFERENCE_INTEGRITY_ONLY with native Tasks authority, current applicability, source Git binding, authentication and discrimination NOT_OBSERVED. The existing execution receipt cannot acquire immutable source authority from a CEM target or a supplied file hash.

- `CEM-V1-011`: Reject missing, duplicate, unknown or escaping reference/artifact identities. Retain original bounded artifact bytes and exact hashes, including the reviewed plan, runner receipt and native capture references. Unknown native verification is explicit; CEM seams do not import Tasks or runner implementations.
- `CEM-V1-012`: Publish the candidate schema and algorithms before the standalone consumer. Native and standalone consumers derive the canonical patch from Git and independently supplied base/target, compare exact committed sidecar bytes and immutable evidence spans, and share positive/adversarial vectors without importing each other's implementation. Corvint-authored consumer conformance remains distinct from an independent external consumer.
- `CEM-V1-013`: Never translate candidate spec labels to earlier0.x profiles to bypass consumer migration. The first candidate supports only the portable0.2 hunk vocabulary; missing0.3 structural/coverage/discrimination portability remains a stable1.0 release blocker.
- `CEM-V1-014`: Inventory every existing consumer's profile and migration disposition. An unsupported candidate consumer retains explicit refusal; no candidate reference link satisfies a criterion, native gate, mutation kill, local completion or stable release requirement.

Gate A reviewed the bounded candidate plan with no remaining HIGH under these restrictions. The implemented candidate passed independent repair-cycle-1 review: `/private/tmp/cem10-build/cem1-wire-rereview1.json` binds the 53-file freeze in `cem1-wire-repair1.json`. The 31-case packet includes the original forged-null criterion identity and mixed-case `.GiT` artifact attacks, now refused by both consumers; valid boundary cases remain admitted. Native and standalone consumers derive immutable Git changes, compare original target-sidecar bytes when present, validate evidence drift and hash opaque artifacts before and after verification. No supplied patch or rewritten profile label substitutes for those checks.

The portable module now requires Go 1.24 or later for rooted reads, including builds of its historical commands. This is an accepted, documented source-build floor change; the historical process ABI and frozen predecessor vectors remain intact. The official task-local Go1.24.13 compiler passed all 38 top-level portable historical/candidate tests (147 including subtests) and vet on Darwin ARM64; Linux/Windows AMD64 builds, test compilation and vet also passed. `/private/tmp/cem10-build/portable-qualification/qualification.json` retains verified archive provenance and unchanged 53-file hashes. Native Linux/Windows execution and original Go1.24.0 remain NOT_RUN; this is minimum-minor compatibility evidence, not ongoing security support. All eight candidate assurance limits remain NOT_OBSERVED: native authority, historical validity, current applicability, source Git binding, authentication, dependency closure, criterion discrimination and external interoperability. Raw artifact integrity does not decode or authenticate a native receipt.

## Optional candidate assembly contract

The optional `corvint-cem-candidate` companion is an experimental producer/consumer slice under the same reference-only candidate. Its technical contract remains proposed. It does not admit this profile to the default Core producer, OCM, frontier or native completion.

- `CEM-V1-015`: Candidate assembly MUST require independently supplied immutable base and target, an exactly verified canonical `cem/0.2` source map, closed bounded declaration documents and independently declared raw artifact digests. It MUST preserve original native Tasks and generic runner artifact bytes, distinguish raw plan SHA256 from the native struct-serialization plan identity, check declared capture/base/optional candidate-tree and ordered criterion references for coherence, and refuse divergent or missing references. Only Tasks wire contracts may be imported. Native capture semantic verification, live applicability, authentication and criterion adequacy remain NOT_OBSERVED; a retained verification envelope is not an independent native authority check.
- `CEM-V1-016`: Assembly MUST compare each declared runner source input with its self-hashed immutable target Git blob under a literal declared source prefix, rejecting missing entries, links, unsupported modes and changed bytes. The result is DECLARED_INPUT_BYTES_MATCH_TARGET only; source inventory completeness and execution-at-commit remain NOT_OBSERVED. It MUST refuse unsupported `cem/0.3` source vocabulary and a target-sidecar conflict rather than dropping witnesses, rewriting target commits or translating profiles. Output goes only to a new confined standalone bundle with exact artifact-role closure and must pass the native candidate verifier before success. A failed or incomplete runner observation remains retained and cannot be turned into a criterion pass.

The shared runner envelope types may be exported without changing field order, tags, bytes or native plan identity. Tests must demonstrate byte-for-byte compatibility. Rollback removes the optional companion and restores the original aliases without altering historical wire bytes, retained packets, stores or default commands. The coherent actual Go demonstration is self-use, not independent external adoption.

## Candidate artifact refusal codes

These codes describe bounded reference-integrity checks; they do not establish native receipt authority.

| Code | Emitting check | Source |
| --- | --- | --- |
| `candidate-artifact-unavailable` | Cancellation, unavailable paths, symlinks or unsupported path kinds, path lstat/open/read/close failures prevent reading the declared artifact. | `internal/cem/verify/candidate.go:93@8c882b4b` |
| `candidate-artifact-bound` | An opened artifact cannot be statted, is not a regular file, exceeds the per-artifact byte bound, or makes the total artifact inventory exceed its byte bound. | `internal/cem/verify/candidate.go:112@77e4c1f4` |
| `candidate-artifact-digest` | SHA256 of the artifact bytes differs from the declared digest. | `internal/cem/verify/candidate.go:128@21c0f6f2` |

## Optional stable assembly prototype

Technical contract: proposed experimental R1 producer; no default or release
admission. `corvint-cem-candidate assemble-stable --experimental --request ABS
--out-dir NEW_ABS` is a separate optional operation. Existing candidate assembly,
verification commands and their emitted bytes retain their contracts.

The native Core entrypoint `corvint cem verify-stable` is likewise explicit-only and
experimental: it is hidden from help, is matched only in command position (after
optional leading global options), and carries no default or promotion.

The closed `cem-stable-assembly/1` request independently names immutable base and
target, ticket/attempt identity, explicit criterion links, source prefix, and six
absolute SHA256-pinned inputs: canonical source map, native capture, retained
historical verification, snapshot-head artifact, runner plan and runner receipt.
The source is parsed and canonically verified as its actual `cem/0.2` or `cem/0.3`
profile. A distinct typed `StableMap` is constructed and encoded through
`EncodeStable`; complete coverage/discrimination witnesses survive. No source
profile substitution bypasses a parser, validator or consumer migration.

The producer retains all six raw artifact roles, including the exact claimed-ticket
string and exact snapshot-head receipt bytes. `snapshotHeadReceiptSha256` remains
the native binding's identity; `snapshotHeadArtifactSha256` is independently
computed from supplied original bytes. Full criterion identity includes both
fields, and links use the new complete identity. Their incidental equality in one
historical proof is not an identity rule. The generic input reader continues to
reject `.git` paths; an operator must separately pin and stage a selected original
receipt outside Git metadata before supplying it.

Only pure Tasks wire decoding is used by the assembler. An independently configured
native Tasks executable can verify historical capture semantics in a separate
bounded read, with original stdin/stdout/stderr, command and pre/post executable
pins retained. That read does not authenticate the selected receipt's journal
linkage. Without that separate linkage evidence, receipt authority remains
`NOT_OBSERVED`. The stable assembly result contains the actual 21-field stable
verification envelope; it does not reuse the candidate result or upgrade any of
its thirteen authority/semantic axes.

Each declared runner input must match its immutable target blob. This establishes
`DECLARED_INPUT_BYTES_MATCH_TARGET` only. Raw plan identity, report hashes,
FAILED/SKIPPED/FLAKY/interruption/build/infrastructure outcomes and reported attempts
remain intact; no retry, execution-at-commit, inventory completeness, criterion
adequacy or discrimination claim is inferred. Output must be a new confined bundle,
the target must have no committed CEM sidecar in this producer slice, and native
stable verification must succeed before `stable.json` and a successful result are
published. Changed, absent or unsupported inputs refuse; incomplete intermediate
artifacts are not a successful bundle.

The bounded historical proof retains base
`474f75da42ffab0f6b04425f26c8b655602d2ed8` and target
`a14f9d0123a923fa11ec411ec293a643856603e5`, actual native capture verification and
original generic Go runner evidence. It compares the producer's embedded result
with Core and portable results recursively across all 21 fields. Historical
coverage/discrimination witnesses remain absent; separate 0.3 witness tests are
manufactured, non-executed fixtures. All evidence is experimental self-use.

| Existing requirements | Bounded producer evidence | Remaining qualification |
| --- | --- | --- |
| CEM-V1-001, 011..013 | Typed stable construction, exact six-role graph, full witness retention, native/portable result equality | Complete public corpus, supported repository/runtime envelopes and all consumer migration |
| CEM-V1-002 | Pure native wire binding plus separately pinned historical Tasks verification | Current applicability, receipt journal authority and semantic adequacy |
| CEM-V1-003..005 | Exact actual runner/source/report bytes and native outcome preservation | Inventory completeness, execution-at-commit and criterion discrimination |
| CEM-V1-007, 014 | Optional companion producer, explicit stable verification, the hidden Core status/verify read (`CEM-V1-017..019`) the Core report/MCP preview (`CEM-V1-034..036`) and the Core stable bundle export (`CEM-V1-060..064`) | Other Core workflow, OCM/frontier/obligation, companion and completion consumers |
| CEM-V1-008..010 | Original defaults/packets unchanged and optional operation removable | External adoption/outcomes and release/promotion gates |

The current stable verifier admits the experimental
`canonical-repository-bounded/1` envelope of
`protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.md`, including its expanded admission
amendment: fixed-order metadata admission without a configuration pre-read, at most
1,024 logical Git operations, one 30-minute wall deadline with a 10-second
per-operation deadline and a single 10-second emergency allowance, and owned
process-group retirement whose unobserved cleanup is `repository` /
`unsupported-process-containment`. The independently reviewed
`primary-clean-config-bounded/1` portable envelope remains preserved as history.
Darwin keeps the quiet-first retirement sequence: after an owned successful group
signal, signal 0 is polled while the leader is unreaped until the group is no
longer signalable, then the leader is reaped and post-reap absence is observed.
Linux uses `ReapAfterSuccessfulSignal`: after the owned successful group signal
and observed leader exit, the leader is reaped exactly once and only signal-0
absence polling follows. Linux preserves all `KillGroup` errors, including
`EPERM`, `ESRCH` and early `Stop` failures; no real numeric group signal may occur
after reaping begins. The same original retirement allowance is checked before
and after owner primitives and before accepting post-reap `ESRCH`. Post-reap
polling is bounded by the intersection of that original allowance and the fixed
two-second post-reap cap. Both are checked synchronously before each probe and
immediately before accepting absence; expiration wins simultaneous interval,
exit or reap readiness, including expiration inside an absence-returning probe.
An already expired allowance admits no new signal, probe or reap, even when exit
is observed or its deadline notification has not arrived. A pending
asynchronous reap can be retained only as sticky `HOLD`, never as a later upgrade
or retry. This is linked to `CEM-V1-007` consumer migration because stable Core
verification depends on supported owned-runner containment. At source
`1a355ea551c2745d3fe9f229a36acc348cb205d8`, actual native Linux arm64 with Go 1.27.1
passes the five scoped packages and all 54 applicable public full-result cases,
with exact recursive comparison of all 21 frozen fields. The two historical
Go 1.24.13 cases separately pass the original portable comparator on Darwin arm64;
they are not Linux results. The repair build log retains these distinct runtime
observations and the initial failed qualification harness runs. Linux amd64,
portable standalone Linux, cross-device topology and escaped-session behaviour
remain pending; external consumers, default promotion and release readiness are
not claimed. Legacy Wait/Run is separate from the new Owner qualification. Canonical ordered fixtures do not qualify ordinary
checkouts. Rollback disables the new optional operation and preserves earlier
command defaults, candidate assembly, native task history and historical packet
bytes. Independent producer implementation review and root source admission remain
separate steps.

## Structural runtime tuples

The structural mechanical proof (`cem-0.3-structural-mechanical.md`) depends on the exact Go
release's `go/parser`, `go/scanner`, `go/ast`, `go/token` and `go/format` (`go/printer`) behaviour, so
the stable verifier admits it only on an exact qualified tuple and otherwise refuses with
`unsupported-structural-runtime`.

- `CEM-V1-050`: The native (`internal/cem/verify`) and portable (`interop/cem01-go`) stable
  verifiers MUST admit structural mechanical proof only on an exact listed Go release: `go1.27.1`,
  the tuple the frozen S0E packet records, and `go1.27.2` (decision 0522). Every other release,
  including later patch releases, MUST keep refusing with `unsupported-structural-runtime` and no
  weakened fallback. A tuple admitted after `go1.27.1` MUST carry its own sealed packet under
  `protocol/cem-1.0/structural-tuples/<version>/` that binds the frozen
  `repository-envelope-packet` manifest and result files by SHA-256, records the stdlib source and
  behavioural byte-identity comparison against its predecessor, and admits exactly one
  substitution: `expectedResult.runtime.goVersion` from the predecessor to the new tuple. The
  frozen packet is never rewritten except by an additive member regeneration (`CEM-V1-091`). Tests run the S0E cases, linked control, ledgers and envelope
  cells on an admitted tuple only through that binding and report NOT_RUN otherwise.
  Qualification stays `EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS`; admitting a tuple does not complete
  the full corpus. Rollback removes the tuple from both lists and its packet; the frozen packet and
  `go1.27.1` behaviour are unchanged.

### Additive member regeneration

An additive stable result member (for example `citation`, V1-1265) changes the closed key set of
every frozen S0E expectation, so it cannot land by hand edits to sealed case files.

- `CEM-V1-091`: A new top-level member of `cem-stable-verification/1` MUST be added only by
  `tools/cem-stable-regen` (`make cem-stable-regen-capture`, then `make cem-stable-regen`). The
  values MUST come from the native S0E harness capture (`CEM_STABLE_REGEN_CAPTURE`, one
  `{id, tuple, exit, result}` line per case, comparing only the frozen keys) or from an explicit
  per-case override file for cases the host cannot run; a case with neither, a case with both, a
  captured exit or frozen key that differs (`runtime.goVersion` only through an admitted tuple
  substitution), an unknown capture key or an existing member MUST be refused before any write. The
  tool MUST insert the member after `--after` in both `RESULT.schema.json` copies (`properties` and
  `required`) and in every `expectedResult`, including each cell of the packet's
  `inherited/s0e/REPOSITORY-ENVELOPE.cells.json` and its stable copy. A cell takes the value of the
  regenerated cases whose exit and frozen keys equal its own, and those cases MUST agree; a cell no
  case matches MUST take an explicit `--cell-values` override, and a matched cell MUST NOT have one.
  The tool MUST leave every other byte unchanged, keep the case count,
  bump the closed key count, record each replaced file's SHA-256 (`predecessorSHA256`,
  `predecessorPublicManifestSHA256`, the tuple's `inherited.predecessorManifestSha256`, and a
  `CHANGES.md` section with old and new digests, value provenance, the fragment digest and the
  predecessor digest of every re-bound file outside the packet), and re-bind the packet manifest,
  the stable manifest entries for the schema, cells and prose, `publicAmendmentManifestSHA256`,
  the stable `REPOSITORY-ENVELOPE.cells.json` amendment index, `REPOSITORY-ENVELOPE.md` and every
  `structural-tuples/<version>/` `TUPLE.json` and manifest. `make cem-stable-regen-check` MUST
  verify that whole chain, including that each manifest lists its mandatory files, and exit nonzero
  on any mismatch. Native and portable consumers then raise
  their closed key count in the same change; a native/portable value disagreement is a consumer
  defect, never a reason to edit captured values. `INPUTS.json` repair-era pins stay historical.
  Rollback is a Git revert of the regenerated files and the consumer change.

## Consumer-computed support tiers (V1-1262, decision 0523 P0)

Status: proposed. Decision 0523 P0. The verifiers derive, from the base blob, target blob,
canonical patch and cited spans alone, how strongly each hunk is supported. No map field is added
or changed. Nothing here is a verdict, threshold or published cut-off.

- `CEM-V1-080`: Support tiers are consumer-derived. The native (`internal/cem/verify`, `support.go`)
  and portable (`interop/cem01-go`, `citation.go`) verifiers MUST assign each hunk of a `cem/0.2` or
  `cem/0.3` map exactly one tier, by precedence `held`, `mechanical-proved`, `anchored`, `cited`,
  `unknown`, plus an `annotation` flag. A supported hunk is one with at least one basis;
  `mechanical-proved` is the `mechanical` disposition after re-proof; `unknown` is a non-mechanical
  hunk with no basis. `held` is reserved and never produced until the go-test hunk-revert falsifier
  lands. No producer-written field can raise a tier. The tier MUST NOT change the verdict, the
  codes or drift. Frozen `cem/0.1` never carries a tier.
- `CEM-V1-081`: Anchor kinds and byte-level rules. `id`: an ASCII requirement ID
  (`[A-Z][A-Z0-9-]{2,31}-[0-9]{3}`, token boundaries where the adjacent bytes are not in
  `[A-Za-z0-9_-]`) occurs in the hunk's added bytes and the cited span contains a defining line
  (optional leading spaces or tabs, then ``- `ID`:``). `path`: the span contains the hunk's
  post-image path (the old path for a pure deletion) or its basename when the basename is at least
  6 bytes. `annotation`: the hunk has added lines and, after every boundary-valid requirement ID in
  the added bytes is masked to spaces, every remaining token (maximal `[A-Za-z0-9_]` run) is shorter
  than 3 bytes or is a `lexical-relevance-floor-v0` stop term. A hunk's anchors are the union over
  its distinct cited spans. A `symbol` anchor is a recorded limit (Go-only, deferred) and is never
  produced.
- `CEM-V1-082`: The `citation` block and the shift null MUST be byte-identical across the native and
  portable consumers and carry no verdict. Shape:

```json
"citation": {
  "profile": "cem-citation-support/0",
  "hunks": [ {"hunkId": "...", "tier": "...", "anchors": ["id","path"], "annotation": false} ],
  "hunksByClass": {"held": 0, "mechanical-proved": 0, "anchored": 0, "cited": 0, "unknown": 0},
  "annotationHunks": 0, "distinctEvidence": 0,
  "evidenceFanOut": [ ["<evidenceId>", 3] ], "maxFanOut": 0,
  "samePathCitations": 0, "concentratedHunks": 0,
  "invariant": true, "specificity": 0
}
```

  `hunks` is sorted by `hunkId` bytewise and each `anchors` list is sorted. `distinctEvidence` is the
  count of distinct cited evidence IDs. `evidenceFanOut` lists `[id, distinct citing hunks]` sorted
  by count descending then ID ascending; `maxFanOut` is the largest count or 0. `samePathCitations`
  counts basis entries whose evidence path equals the hunk's post-image path (the same evidence
  under two relations counts twice). `concentratedHunks` counts supported hunks whose every basis
  cites an evidence ID at `maxFanOut`. The shift null sorts supported hunks by `hunkId`, gives hunk
  i the basis set of hunk i-1 (hunk 0 takes the last) and recomputes `anchored`. `invariant` is true
  when all supported hunks' evidence-ID sets are equal (true below two supported hunks).
  `specificity` is `anchored` minus `anchored` after the shift (0 below two supported hunks).

  Placement in this slice: the native 0.x canonical verification object carries key `citation` on
  success and on drift rejection only when `--citation` is given to `verify` or `status`; default
  output stays byte-identical to the frozen 0.x goldens (decision 0357) and to the base verifier,
  which `dogfood-check` requires. The stable (`cem/1.0`) result does
  not carry it yet: `cem-stable-verification/1` is a sealed 21-key shape
  (`protocol/cem-1.0/stable/RESULT.schema.json`, the S0E repository-envelope packet and the
  go1.27.2 structural tuples), so adding the member is a packet change owned by V1-1265. The
  portable consumer computes the block as a library result pinned by `testdata/citation/`, and
  emits nothing on the wire because the frozen `protocol/cem-0.3` vectors pin its 0.x result.

Non-goals: no pass/fail threshold, no published cut-off, no `symbol` kind, no change to map wire
fields, no producer-side tier, no stable-result emission before V1-1265.

Failure modes: the block is derived only from span bytes the canonical verification already read
and bound, so a span that cannot be read fails verification before any tier exists; a consumer never
emits a partial or estimated block, and an absent block is not a verdict either way.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-080 | `TestCEMV1080RubberStampAndCarefulMapsDiffer`, `TestCEMV1080CitationIsCanonicalOnly`, `TestCEMStatusCitationIsOptIn` (native); `TestCEMV1080RubberStampAndCarefulBothAccept` (portable) | A rubber-stamp map (every hunk cites one `AGENTS.md` span) and a careful map of the same 24-hunk patch both ACCEPT and their `citation` integers differ; tiers never change `valid`, `issues` or `drift`; `cem/0.1` carries no block |
| CEM-V1-081 | `TestCEMV1081IDBoundaryAndAnnotation`, `TestCEMV1081IDAnchorRequiresBoundedTokenAndDefinition`, `TestCEMV1081PathAnchorUsesPostImageAndBasenameFloor`, `TestCEMV1081Annotation` (native); `TestCEMV1081IDBoundaryAndAnnotation`, `TestCEMV1081RequirementIDGrammar` (portable) | A hunk adding ``x // `CEM-SM-007` `` against a span that mentions `CEM-SM-0070` and defines nothing is `cited`, not `anchored`; a hunk adding only that comment is `annotation: true`; the post-image path and the 6-byte basename floor decide `path` |
| CEM-V1-082 | `TestCEMV1082DisjointUmbrellaSpansHaveNoSpecificity`, `TestCEMV1082CitationIsDeterministic`, `TestCEMV1082BlockShapeAndCounts`, `TestCEMV1082ShiftNull`, `TestCEMV1082HunkOrderDoesNotChangeTheBlock` (native); `TestCEMV1082DisjointUmbrellaSpansHaveZeroSpecificity`, `TestCEMV1082CitationIsOrderIndependent`, `TestCEMV1082CountingRules`, `TestCEMV1082StableResultKeepsSealedShape` (portable); `interop/cem01-go/testdata/citation/*/citation.json` byte-matched against the native binary | `AGENTS.md` split into 20 disjoint spans, one per hunk, reports `specificity: 0`; native and portable blocks are byte-identical on four fixtures; the stable result keeps its 21 keys |

Rollback: remove `internal/cem/verify/support.go`, `interop/cem01-go/citation.go` with
`testdata/citation/`, and the `citation` key in `successVerification`; remove the `--citation`
flag and the `Citation` read option. No map, packet or protocol vector changes.

## Provider-neutral criterion (V1-1263, decision 0523 P1)

Status: proposed. Decision 0523 P1. A stable criterion no longer requires a Corvint Tasks store.

- `CEM-V1-083`: A `cem/1.0` criterion binding MUST take exactly one of two closed forms. The plain
  form is `{id, criterionIndex, criterionSha256, evidenceId}`: `evidenceId` MUST name an evidence
  record of the same map (else `unresolved-reference`), `criterionSha256` MUST equal that record's
  `spanSha256` (else `reference-coherence`), no two plain criteria may name the same evidence
  (`duplicate-reference`), and `id` is `criterion:sha256:` over the canonical JSON of the other
  three keys. The Tasks form is the existing eleven keys, unchanged in encoding, identity and
  artifact roles. Any of the eight Tasks keys present selects the whole Tasks form; a record is
  then checked for missing keys (`missing-field`), unknown keys (`unknown-field`) and invalid values
  (`invalid-field`) in that order, so plain keys beside a partial Tasks extension are `missing-field`
  and `evidenceId` beside the full Tasks form is `unknown-field`. A plain criterion contributes no
  Tasks artifact role, and a map with only plain criteria MUST verify on a repository with no
  `.taskman/`. The native and portable consumers MUST return the same result for the same inputs.

Non-goals: no new producer verb (a plain map is hand-assembled until a producer lands), no new
result key, no change to the S0E repository-envelope packet, no change to `cem/0.x`. Runner
receipts, criterion links and runner artifacts stay required by the existing coherence rules.

Failure modes: a mixed or partial record is refused at the wire stage before any repository read;
a plain criterion whose span digest or evidence reference is wrong is refused at the references
stage; artifacts are checked only by SHA-256, so an opaque artifact never upgrades authority.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-083 | `TestCEMV1083NeutralCriterionPublicCase`, `TestCEMV1083TasksFormUnchanged` (`internal/cem/verify`); `TestCEMV1083PlainCriterionWire`, `TestCEMV1083PlainCriterionRefusals` (`internal/cem/wire`); `TestCEMV1083NeutralCriterionPublicCase`, `TestCEMV1083PortablePlainDuplicateEvidence`, `TestCEMV1083PortableTasksFormUnchanged` (`interop/cem01-go`) | Public case `protocol/cem-1.0/stable/neutral-criterion/` ACCEPTs with the same 20 non-runtime keys on both consumers on a repository with no `.taskman/`; seven public refusals match on both; every S0E Tasks criterion re-encodes to its original eleven keys; walkthrough in `docs/build-log/2026-10-12-cem-neutral-criterion-walkthrough.md` |

Rollback: revert the plain branch in `internal/cem/wire/stable.go` and `interop/cem01-go/cem.go`
(restore the eleven required keys), the `criterionBinding` `oneOf` in `stable.schema.json`, the
CEM-V1-083 text in `ALGORITHMS.md`, the `neutral-criterion/` case and its manifest entries. Tasks
maps need no migration.

## Runner receipt authentication (V1-1264, decision 0523 P1)

Status: proposed. Decision 0523 P1. A runner receipt can carry a signed attestation, and only the
consumer decides which keys it trusts.

- `CEM-V1-084`: A runner receipt's `authentication` MUST take one of three closed forms: the sealed
  legacy string `"NOT_OBSERVED"` (read as method none and echoed unchanged), `{"method":"none"}`,
  or `{"method":"dsse-ed25519","keyid":K,"envelopeSha256":E}`, where K is the lowercase hex SHA-256
  of the raw 32-byte Ed25519 public key and E is the SHA-256 of the exact envelope bytes. The method
  selects the key set, so a member is refused `missing-field`, then `unknown-field`, then
  `invalid-field`. E MUST resolve to a `receipt-attestation` artifact (`artifact-role-closure`),
  no two receipts may name the same E (`duplicate-reference`), and a `receipt-attestation` artifact
  no receipt names is unused (`artifact-role-closure`). E's bytes MUST be one closed DSSE envelope
  (`payloadType` `application/vnd.in-toto+json`, exactly one signature) over a closed in-toto
  Statement v1 with `predicateType` `https://corvint-context.dev/attestation/cem-receipt/v1`, an
  empty predicate and exactly the subjects `runner-receipt` (sha256), `runner-plan` (sha256) and
  `target-tree` (gitTree), in that order. After both artifact passes a consumer MUST refuse, at
  stage `artifacts` with exit 1 and `authentication` FAILED, `attestation-malformed`,
  `attestation-key-mismatch` (signature keyid differs from K), `attestation-subject-mismatch`
  (receipt, plan or target tree differs, checked with or without trust) and, for a trusted K only,
  `attestation-signature-invalid`. Trust comes only from `--trusted-keys FILE`: an absolute regular
  file of at most 64 KiB holding `{"profile":"corvint-trusted-keys/1","keys":[{"keyid":K,
  "ed25519PublicKey":B64}]}` with 1..64 unique keys, each keyid the digest of its key; any other
  file is `invalid-arguments` before the map is read. On ACCEPT the axis is NOT_OBSERVED without
  that file, UNTRUSTED_KEY when any signed receipt names an untrusted keyid, NOT_OBSERVED when any
  receipt is unsigned, and VERIFIED only when every receipt is signed by a trusted key. A key the
  map or its artifacts carry is never trusted, and a signature never raises `executionAtCommit`.
  The native and portable consumers MUST return the same result for the same inputs.

Non-goals: no signing producer and no key management (signed production is an operator step,
FPK-V0-036); no network, transparency log, certificate or key discovery; no change to
`RESULT.schema.json`, the sealed S0E packet or `cem/0.x`; no change to the candidate profile.

Failure modes: a malformed member is refused at the wire stage and an unresolved or repeated
envelope at the references stage, before any artifact read; an envelope over another receipt,
plan or tree is refused even without a trusted-keys file; a bad signature under an untrusted key
is not checked and leaves UNTRUSTED_KEY, never VERIFIED; a bad trusted-keys file refuses the whole
run rather than silently lowering the axis.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-084 | `TestCEMV1084ReceiptAuthenticationFixtures`, `TestCEMV1084ReceiptAuthenticationPublicCase` (`internal/cem/verify`); `TestCEMV1084AuthenticationForms`, `TestCEMV1084AuthenticationRefusals`, `TestCEMV1084ReceiptAttestationEnvelope`, `TestCEMV1084TrustedKeysFile` (`internal/cem/wire`); `TestCEMV1084PortableReceiptAuthenticationPublicCase`, `TestCEMV1084PortableTrustedKeysArgument` (`interop/cem01-go`); `TestCEMV1084VerifyStableTrustedKeysFlag`, `TestCEMV1084HelpNamesTrustedKeys` (`cmd/corvint`); `TestCEMV1084ConsoleReceiptAuthentication` (`internal/console`) | Public case `protocol/cem-1.0/stable/receipt-authentication/` ACCEPTs VERIFIED with the same non-runtime result on both consumers; five axis cases and thirteen refusals match on the native verifier and both portable commands; a self-carried key and a signature over the base tree are refused; walkthrough in `docs/build-log/2026-10-12-cem-receipt-authentication.md` |

Rollback: revert `stable_auth.go` in `internal/cem/wire`, `internal/cem/verify` and
`interop/cem01-go`, the `--trusted-keys` flag, the `authentication` `oneOf` and the
`receipt-attestation` kind in `stable.schema.json`, the CEM-V1-084 text in `ALGORITHMS.md`, the
`receipt-authentication/` case and its manifest entries. Maps with the legacy string need no
migration; a map with an object form is refused again after rollback.

## Stable verification result codes

The optional `cem verify-stable` operation emits the closed result codes of
`protocol/cem-1.0/stable/OPERATION.json` and `RESULT.schema.json`; those public files stay the
normative mapping. This table names the codes first emitted by the stable verifier, so each has
one owning specification. A code reports a bounded check and never upgrades authority.

| Stage | Codes | Outcome |
| --- | --- | --- |
| references | `invalid-criterion-identity`, `duplicate-reference`, `unresolved-reference`, `reference-coherence`, `artifact-role-closure` | Exit 2; the decoded reference graph is rejected without filesystem or native authority inference |
| authority-arguments | `artifacts-required` | Exit 2; the explicit artifact root is required even for an empty reference set |
| runtime | `unsupported-structural-runtime`, `unsupported-process-containment` | Exit 2; an unqualified runtime or containment tuple is explicit and has no weakened fallback |
| repository | `unsupported-resource-limit`, `unsupported-repository-envelope`, `verification-timeout` | Exit 2; operational limits stay distinct from invalid evidence |
| repository | `unsupported-patch-inventory` | Exit 2 (`UNSUPPORTED`); a create destination lacks an authenticated base-entry proof, so absence is not inferred |
| artifacts | `artifact-unavailable`, `artifact-resource-limit` | Exit 2 (`UNSUPPORTED`); declared artifact bytes could not be obtained within bounds, which asserts no mismatch |
| artifacts | `artifact-changed-during-verification`, `artifact-digest-mismatch` | Exit 1 (`REJECT` issue codes); the bytes were read and changed between the two passes or differ from the declared digest |
| artifacts | `attestation-malformed`, `attestation-key-mismatch`, `attestation-subject-mismatch`, `attestation-signature-invalid` | Exit 1 (`REJECT` issue codes, CEM-V1-084); a declared receipt attestation does not bind the receipt, plan, target tree or trusted key |

## Core stable read (V1-0632 S1)

Status: proposed. This is the first consumer-migration slice of `CEM-V1-007`. It
adds no producer, no default, no help entry and no stable promotion; the read stays
hidden like `cem verify-stable` until the owner decides help visibility.

- `CEM-V1-017`: Core `cem status` and `cem verify` over an exact `cem/1.0` map MUST run the stable verifier over the repository root byte for byte as the caller named it (the `--root` value, even empty, with no tilde expansion, absolutization, cleaning or symlink resolution; the working directory only when `--root` is omitted), judged exactly as `cem verify-stable --repository` judges the same string: a relative or empty root is refused at stage `arguments` with `invalid-arguments` before the map is verified, and a symlinked or lexically unclean absolute root at stage `repository`. A root beginning with `--` (for example `--root=--repo`) is refused as `cem verify-stable` refuses that value named first as `--repository`: its parse stops there, so the result keeps no `expectedBase` or `targetRevision`. It runs with the independently supplied `--expected-base`, `--target` and an explicit absolute `--artifacts` root. The envelope MUST embed the complete, unmodified 21-field result as `stableVerification` and the command MUST exit with the verifier's own code (0 accept, 1 reject, 2 refusal). It MUST NOT synthesize a 0.x `verification`, counts, worklist, policy issues or `ready-for-ci`; status names the outcome `accepted`, `rejected` or `unsupported`. Core MUST NOT open or pre-read the repository before the verifier's own metadata admission.
- `CEM-V1-018`: A stable read MUST refuse `--patch` and policy ceilings with `invalid-arguments`, and every other Core workflow read of a `cem/1.0` map with `unsupported-spec` naming the stable reads, until its own slice lands; `cem report` and the report preview are read since S2 (`CEM-V1-034..036`). Each such refusal is marked as a stable refusal, which MCP names `cem-map-stable-refused` (`MCPV0-037`). A 0.x map MUST refuse `--artifacts` with `invalid-arguments` after its map parses. Every 0.x parser refusal of an exact `cem/1.0` root MUST name the stable read rather than report malformed 0.x input.
- `CEM-V1-019`: 0.x reads MUST keep their arguments, precedence, symlink-resolved root, output bytes and exit codes. The stable route MUST be chosen only after the 0.x parser refuses a bounded JSON object whose `spec` is exactly `cem/1.0`; the experimental candidate and every other spec keep their existing code and message, and malformed JSON keeps its earlier failure.

Failure modes: an omitted `--artifacts` is the verifier's `authority-arguments` /
`artifacts-required` refusal, never a Core default. A runtime or repository the
verifier does not support is its explicit `UNSUPPORTED` result. The only 0.x change
is that `--artifacts` on a 0.x read is now a stage-3 `invalid-arguments` refusal
after the map read instead of an argument-parser refusal; both exit 2 with no
output. Rollback removes the route and the flag; no 0.x byte, map or packet changes.

### Core stable read traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-017 | `TestCoreStableReadMatchesVerifyStable` | Core verify and status equal `cem verify-stable` (all 21 fields, exit); the public sealed positive is ACCEPT/EXACT on Go 1.27.1 |
| CEM-V1-017 | `TestCoreStableReadKeepsVerifierExit` | Missing `--artifacts` exits 2 (`artifacts-required`); an altered artifact exits 1 (`artifact-digest-mismatch`) |
| CEM-V1-017 | `TestCEMStableReadKeepsCallerRoot` | Through the real argv entry, status and verify with `--root R` equal `cem verify-stable --repository R` (result and exit) for the real root, the symlinked alias, `<alias>/../repo`, `<repo>/`, `./repo`, `repo`, `./alias`, `--root ""`, `--root=`, `--root=--repo`, `--root=-x` and `--root=--map`; an omitted `--root` equals `--repository <working directory>`; only the real root and the omitted root pass admission |
| CEM-V1-017 | `TestCEMStableReadDefaultsToSessionRoot` | A workflow session whose entry point never sets a stable repository admits its own unresolved root: a session opened on the symlinked alias equals `cem verify-stable --repository <alias>` (stage `repository`, result and exit) |
| CEM-V1-018 | `TestCoreStableReadExplicitRefusals` | Status/verify `--patch` and policy ceilings, and report-preview `--patch`, refuse before verification (CLI exit 2) |
| CEM-V1-018 | `TestCoreStableReadRefusesArtifactsFor0x` | A parseable `cem/0.3` map refuses `--artifacts` before base/target checks |
| CEM-V1-018 | `TestParseMapStableRefusalNamesStableRead` | The 0.x parser names the stable read for an exact `cem/1.0` root |
| CEM-V1-019 | `TestCoreStableRead0xRefusalsUnchanged` | Candidate, other, padded and numeric specs keep the 0.x refusal through Core |
| CEM-V1-019 | `TestParseMapOtherSpecRefusalUnchanged` | Parser text unchanged for every non-stable spec |
| CEM-V1-019 | `TestDeclaresStable` | Route selection is exact, bounded and object-rooted |
| CEM-V1-019 | `TestCEMLegacyReadKeepsResolvedRoot` | A 0.x status with `--root` set to the alias, `<alias>/../repo`, `<repo>/`, `./alias` or `repo`, or with an empty (`--root ""`, `--root=`) or omitted `--root` in the aliased working directory, still reports the cleaned, symlink-resolved map path |

## Core stable mutator refusals (V1-0632 S3)

Status: proposed. This slice keeps the Core mutators `cem/0.x` producers: the
optional candidate assembler stays the only stable producer until the stable wire
is frozen (`CEM-V1-007`). It adds no producer, default, help entry or promotion.
The requirement IDs start at 030 so that slices proposed in parallel do not collide;
integration may renumber them.

- `CEM-V1-030`: Core `cem prepare` MUST refuse an existing map at the fixed sidecar path whose bytes declare exactly `cem/1.0` with `unsupported-spec`, with or without `--replace`. It MUST leave the map, the patch cache and any backup unchanged, and MUST NOT resume, downgrade or overwrite a stable sidecar. It MUST refuse in the same way, before it takes the sidecar lock or recovers a backup, when a recognized interrupted-publication backup (`<map>.bak-` plus sixteen lowercase hex digits) declares `cem/1.0`, and MUST refuse a recognized regular backup it cannot read with `map-unavailable`, because unread bytes may be stable; it re-checks backups under the lock, and neither recovery nor resume cleanup restores or removes a refused backup. With `--replace`, it MUST refuse an existing regular sidecar it cannot read with `map-unavailable` instead of regenerating it, because unread bytes may be stable.
- `CEM-V1-031`: Core `cem cite`, `cem mark`, `cem cover` and `cem discriminate` MUST refuse a `cem/1.0` input map with `unsupported-spec` naming the command and the stable read, in place or with `--output`, and write nothing. Every Core map writer (`begin`, `cite`, `mark`, `cover`, `discriminate`) MUST refuse an output destination whose existing bytes declare exactly `cem/1.0`, leaving it unchanged. Each writer first runs a non-mutating preflight that refuses a readable `cem/1.0` input or destination before any lock file, directory or backup recovery touches the file system. It then holds the output lock across a strict probe and publication. Only an absent destination, one that is not a regular file (publication replaces the directory entry, never a link target), or one over the map bound (which `wire.DeclaresStable` never selects) proceeds unread. A regular destination whose bytes cannot be read, or that changes while it is read, MUST be refused with `map-unavailable` naming the command and `cem/1.0`, leaving it unchanged.
- `CEM-V1-032`: The daily bind loop MUST read no hunks from a map declaring `cem/1.0`, so such a map never counts as cited and never binds a citation plan. Its cite step MUST refuse such a map as `cem-cite` `NOT_PRODUCED` `cem-map-stable-unsupported` before it reads the plan, writing no plan binding, cite log or map, and never treat it as a map that owes no hunk. `dogfood batch-check` MUST refuse a sealed `cem/1.0` constituent with `constituent-cem-stable-unsupported` before any verifier run, never as `constituent-cem-invalid`.
- `CEM-V1-033`: Selection is by the exact declared spec only (`wire.DeclaresStable`). Every `cem/0.x` input, the experimental candidate and every other spec MUST keep the earlier arguments, precedence, output bytes, refusal codes and messages, including the `prepare` `map-unavailable` refusal and an explicit `--replace` regeneration of a non-stable existing map.

Failure modes: a stable sidecar the 0.x workflow used to report as "not a valid
CEM document; pass --replace" now refuses as `unsupported-spec`, so `--replace` is
no longer suggested for it. An unreadable existing regular destination of a 0.x
writer, which publication used to replace, is now refused, because its spec cannot
be resolved: it may be a valid stable map. A destination over the map bound, or not
a regular file, is still left to publication and its existing refusals. The output
lock is advisory and binds only Core writers. Nothing guards against an
uncooperative process that replaces the destination after the strict probe and
before the publication rename: the probe's identity-checked read refuses a
replacement during the read, but no re-check or exclusive create closes the
window that follows. Rollback removes the checks; no 0.x byte, map or packet
changes.

### Core stable mutator refusal traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-030 | `TestPrepareRefusesStableSidecar` | The public sealed positive map at the sidecar path is refused with and without `--replace`; its bytes are unchanged and no patch cache is published |
| CEM-V1-030 | `TestPrepareReplaceRefusesUnreadableSidecar` | A mode-000 stable sidecar keeps the earlier refusal without `--replace` and is refused, not regenerated, with it; no patch cache is published |
| CEM-V1-030 | `TestPrepareRefusesStableBackupBeforeRecovery` | A stable backup with no sidecar is refused before lock and recovery, leaving it in place with no sidecar or lock file; a non-stable backup is still restored |
| CEM-V1-030 | `TestPrepareRefusesUnreadableBackup` | A mode-000 stable backup is refused with `map-unavailable` beside a resumable 0.x sidecar, before resume cleanup would remove it, and with no sidecar, before lock and recovery; the backup and the sidecar stay unchanged and no lock file is created |
| CEM-V1-031 | `TestMutatorsRefuseStableInput` | cite, mark, cover and discriminate refuse a `cem/1.0` input in place and with `--output`; no output is written |
| CEM-V1-031 | `TestMutatorsRefuseStableDestination` | Each mutator, and `cem begin`, refuses a stable `--output` destination; before this slice, cite overwrote it with `cem/0.2` |
| CEM-V1-031 | `TestMutatorsRefuseUnreadableDestination` | Each mutator, and `cem begin`, refuses a mode-000 stable destination with `map-unavailable`; before the fix, each one replaced it |
| CEM-V1-031 | `TestStrictProbeRefusesDestinationReplacedWhileRead` | `begin` and `mark` refuse a destination replaced by a stable map during the locked probe's read |
| CEM-V1-031 | `TestBeginProbesUnderOutputLock` | `begin` waits for a held output lock, publishes nothing meanwhile, then refuses the stable map the holder wrote |
| CEM-V1-030 | `TestStableRefusalPrecedesLock` | `prepare --replace` refuses a stable sidecar before locking; an occupied lock path, which `Lock` refuses as `publish-failed`, stays unchanged |
| CEM-V1-031 | `TestStableRefusalPrecedesLock` | Each writer refuses a stable input or destination before locking, leaving an occupied lock path unchanged |
| CEM-V1-032 | `TestMapHunksRefusesStableMap` | `mapHunks` and `citedHunks` read no hunks from the stable map, which a loose decode counted as two |
| CEM-V1-032 | `TestCiteStepRefusesStableMap` | The cite step refuses a stable map as `cem-map-stable-unsupported` for empty and non-empty plans, writing no binding or cite log; before the fix, an empty plan returned `PRODUCED` |
| CEM-V1-032 | `TestBatchCheckRefusesStableConstituent` | A sealed `cem/1.0` constituent fails `constituent-cem-stable-unsupported` with its recovery line, and the verifier never runs |
| CEM-V1-033 | `TestPrepareNonStableExistingMapUnchanged` | Candidate, padded and other specs keep the `map-unavailable` refusal and guidance; `--replace` still regenerates them |
| CEM-V1-033 | `TestMutatorsWriteNonStableDestination` | Candidate, truncated and non-JSON destinations are still replaced by the 0.x map |
| CEM-V1-033 | `TestMapHunksRefusesStableMap` | Padded, candidate, 0.2 and numeric specs keep the layout-agnostic decode |

## Core stable report (V1-0632 S2)

Status: proposed. This slice moves `cem report`, the report preview and the MCP
`corvint.cem.report` tool from refusing a `cem/1.0` map to reading it through the
stable verifier. It adds no producer, no default, no help entry and no stable
promotion; the read stays hidden like the S1 read.

- `CEM-V1-034`: Core `cem report --format json` over an exact `cem/1.0` map MUST run the stable verifier exactly as `CEM-V1-017` does, admitting the root exactly as the caller named it (the MCP server's start-time `--root`, not its resolved root), embed its complete, unmodified 21-field result as `stableVerification`, and exit with the verifier's own code. It MUST add a `review` projection with profile `cem-stable-review/0` that is a pure function of that result: its outcome, stage and code, the map and patch digests, the two revisions, a `stableResultSha256` over the key-sorted result, one row per hunk with its evidence drift state (or `NOT_PRODUCED`), coverage and discrimination states copied from the verifier, `testExecution: NOT_RUN`, an explicit `ocm: NOT_PRODUCED`, the non-claims, and a `recordSetSha256` over the projection. It MUST NOT synthesize a 0.x `verification`, counts, policy issues, worklist or `ready-for-ci`.
- `CEM-V1-035`: Core `cem report` (markdown) over a `cem/1.0` map MUST publish the review rendered from that projection for every verifier outcome, with each repository-derived value in a code span, under the same output resolution, map-aliasing and size bounds as the 0.x report. It MUST locate the worktree's Git directories for publication only after the verifier returns and without the 0.x object admission (alternates, attributes), so a result the verifier refused at stage `repository` still publishes; when the Git directories cannot be located (no valid primary or reciprocal worktree marker), no output path can be checked and publication MUST fail closed with `repository-object-unavailable` and nothing written. The report preview MUST return the same Markdown and `recordSetSha256` without writing, and JSON, preview and published Markdown MUST agree on `recordSetSha256`.
- `CEM-V1-036`: A stable report MUST refuse every supplied `--ocm`, `--patch` and policy ceiling, including an empty value, with `invalid-arguments` before the verifier runs and before any file is written; these refusals and any unsupported stable action are marked as stable refusals. A 0.x report MUST refuse `--artifacts` with `invalid-arguments` after its map parses. 0.x report arguments, bytes, goldens and refusals are otherwise unchanged.

Failure modes: OCM binding of a stable result is not defined, so `--ocm` refuses
instead of binding 0.x obligations to a 1.0 result. A runtime the verifier does not
support yields an `UNSUPPORTED` review that still publishes and exits 2. Rollback
removes the stable report route, the `artifacts` MCP argument and the
`cem-map-stable-refused` code; no 0.x byte, map or packet changes.

### Core stable report traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-034 | `TestCoreStableReportJSONEmbedsVerifierResult` | JSON `stableVerification` equals `cem verify-stable` (all 21 fields, exit); the projection digest recomputes; no 0.x keys; ACCEPT and `stable` drift asserted on Go 1.27.1 |
| CEM-V1-034 | `TestCEMReportToolPreviewsStableMap` | The MCP receipt carries the verifier's complete result for the same inputs, with no 0.x fields |
| CEM-V1-034 | `TestCEMStableReportKeepsCallerRoot`, `TestCEMReportToolAdmitsTheCallerNamedRoot` | For each `--root` spelling (real, alias, `<alias>/../repo`, `<repo>/`, `./repo`, `repo`, `./alias`, `--root ""`, `--root=`, omitted, and the option-shaped `--repo`, `-x`, `--map`) the CLI report equals `cem verify-stable --repository <same string>` (complete result, exit); the MCP server accepts only the real root and the alias, each equal to `verify-stable` on that string, and refuses the other spellings at start as `invalid-root` |
| CEM-V1-035 | `TestCoreStableReportPublishesMarkdown` | Published for every outcome; preview equals the published file; `recordSetSha256` agrees across JSON, Markdown and preview |
| CEM-V1-035 | `TestCoreStableReportPublishesRepositoryRefusals` | With `objects/info/alternates` the `UNSUPPORTED` / `repository` / `unsupported-object-alternates` review publishes to the default and a safe external output, equal to `verify-stable`; with no locatable Git directory JSON keeps the result while Markdown fails closed with nothing written |
| CEM-V1-036 | `TestCoreStableReportRefusals` | `--ocm`, `--patch`, ceilings (each also empty) and an output naming the map exit 2 with nothing published; `--ocm=` and `--patch=` are argument-parser refusals; ceiling refusals are stable refusals, `cem/9` and 0.x are not |
| CEM-V1-036 | `TestCoreStableReportRefusesArtifactsFor0x` | A parseable 0.x report refuses `--artifacts` |
| CEM-V1-036 | `TestCEMReportToolNamesStableRefusals` | MCP maps only stable refusals to `cem-map-stable-refused` |

## Portable stable routing (V1-0632 S4)

Status: proposed. Native work: V1-0632, migration rows C04 and C19 of
`protocol/cem-1.0/stable/MIGRATION.json`. The independent portable consumer `interop/cem01-go`
keeps its `cem/0.1` default and its `ci` report schema. It now recognizes an exact `cem/1.0` map
and names the stable command, and an example CI branch runs that command only on explicit opt-in.

- `CEM-V1-PSR-001`: The default `verify` path refuses a strictly parsed JSON object whose exact
  `spec` member is the string `cem/1.0` with the invalid code `unsupported-spec` (exit 1). The
  refusal comes before the patch is read and before any `cem/0.1` structural check, so a stable map
  is never reported as a malformed `cem/0.1` map.
- `CEM-V1-PSR-002`: `ci` reports such a map as verdict `unsupported-profile`, exit 4, with `code`
  `use-verify-stable-canonical`. The 14 fixed `cem-ci-report/0` members, the `profile` value
  `cem/0.1` and the verdict-to-exit map (CEM-PILOT-021/022) are unchanged.
- `CEM-V1-PSR-003`: Only that exact case changes. A duplicate, non-string, differently cased,
  padded or candidate `spec`, malformed JSON, invalid UTF-8, an unpaired surrogate, trailing data
  and every `cem/0.x` map keep their earlier code, bytes and goldens. Frozen protocol packets are
  not edited.
- `CEM-V1-PSR-004`: `examples/cem/verify-portable.sh` runs `verify-stable-canonical` only when
  `CEM_PROFILE=cem/1.0` is set with an absolute existing `CEM_ARTIFACTS` directory. It reads the
  committed map as Git object data only when the head tree's single entry at exactly
  `CEM_MAP_PATH` is a regular blob, and propagates the verifier's stdout and exit unchanged. Bad
  input, a directory selector or tree entry, an unknown profile or a digest mismatch exits 2 before
  the verifier runs, an absent map exits 3 and an absent head commit exits 5. On the qualified
  runtime tuple the sealed S0e change is accepted and a tampered artifact is rejected at the
  artifact stage; on any other runtime the verifier's own runtime refusal passes through. Acceptance is integrity only and never replaces the
  native completion gate. Nothing switches an existing default workflow to this branch.

Failure modes: a host whose Go toolchain differs from the frozen stable runtime tuple gets
`UNSUPPORTED` `unsupported-structural-runtime` (exit 2) from the stable branch, and the example
passes it through. A map path with characters outside the portable path set is refused before Git
is read. The case-insensitive `cem/0.1` header decoding is unchanged, so `{"SPEC":"cem/1.0"}` keeps
its earlier `version-shape` result.

Rollback: revert the peek in `interop/cem01-go/cem.go`, the route code in `ci.go`, and the
`CEM_PROFILE` branch and recipe hint in `examples/cem/**`. No packet, golden, history or native
state changes.

| Requirement | Implementation | Tests |
| --- | --- | --- |
| CEM-V1-PSR-001 | `interop/cem01-go/cem.go` `decodeMap`, `declaresStableSpec` | `TestDefaultPathRefusesStableSpec` |
| CEM-V1-PSR-002 | `interop/cem01-go/ci.go` `ciMapHeader`, `ciStableRouteCode` | `TestCIRoutesStableSpecToStableVerifier` |
| CEM-V1-PSR-003 | `interop/cem01-go/cem.go` `declaresStableSpec` | `TestDefaultPathKeepsOtherProfileRefusals`; the existing interop golden and packet suites |
| CEM-V1-PSR-004 | `examples/cem/verify-portable.sh`, `examples/cem/recipes/ci-verify.sh` | `TestExampleStableBranchVerifies` (admitted structural tuples go1.27.1 and go1.27.2), `TestExampleStableBranchRuntimeRefusal` (other runtimes), `TestExampleStableBranchRefusals`; `script/cem-recipes_test.sh` case `c-stable` |

---
## OCM/LRF stable admission (V1-0632 S5a)

Status: accepted under the owner's standing auto-accept (2026-10-11), after independent review
(Codex round 2 PASS). Native work: V1-0632, migration row C05 of
`protocol/cem-1.0/stable/MIGRATION.json`. This slice adds the owning internal
admission that the stable TCQ/frontier, obligation and dashboard consumers (C06..C09, C17)
build on. It adds no CLI route, producer, default, help entry or promotion, and it does
not relabel the frozen `lrf/0` profile.

- `CEM-V1-051`: `lrfrepo.VerifyStableUniverse` MUST admit a map whose bytes declare exactly
  `cem/1.0` only after the complete stable verifier (`verify.Stable`) accepts those exact original
  bytes. It returns a `StableUniverse` whose `Spec` is `cem/1.0`, whose `MapSHA256` is the
  SHA-256 of the exact original bytes, and whose `Verification` is the complete accepted verifier
  result, unmodified. The verifier MUST report the exact committed sidecar (`EXACT`); an `ABSENT`
  sidecar refuses with `excluded-artifact-mismatch`. The admission MUST re-derive the canonical patch
  at the accepted base and target, and require its digest to equal both the verifier's and the
  map's `patchSha256`. The OCM phase runs inside one retained stable repository boundary. At the
  end, the admission MUST revalidate that boundary and run the verifier again, and MUST require a
  result byte-identical to the first.
- `CEM-V1-052`: The repository root, expected base, target and artifact root are explicit, with no
  default and no Git-only fallback. A missing or non-absolute argument is the verifier's own
  `authority-arguments` refusal (`expected-base-required`, `target-required`,
  `artifacts-required`, `invalid-arguments`). The artifact references are authenticated under the
  named root, and the authority axes (native authority, authentication, runner execution,
  execution at commit, criterion adequacy) stay `NOT_OBSERVED`.
- `CEM-V1-053`: The OCM MUST bind the exact original stable map digest, the accepted target and
  patch, the pinned intent scope and its claims, under the existing OCM checks. The universe retains
  every OCM obligation in OCM order. An unsatisfied (`unknown`) obligation keeps its disposition and
  reason, and is never dropped, closed or counted as linked.
- `CEM-V1-054`: The projection is read-only. It never exposes a legacy `*wire.Map` or any 0.x
  serialization. `OriginalBytes` returns a copy of the authenticated bytes, and `Document`
  re-parses them into a fresh full `wire.StableMap` (criteria, receipts, links and artifacts), so a
  consumer that edits its copy cannot alter the universe or its digest.
- `CEM-V1-055`: `lrf/0` stays frozen. `lrf.Evaluate` MUST keep refusing a `cem/1.0` context with
  `unsupported-lrf-context`. `lrf.ValidateStableRequest` is the owning stable request admission. It
  applies every frozen request check and admits only a `cem/1.0` context with canonical-derived
  patch identity, a target and a complete OCM binding. Any other spec, including a relabeled
  context, and a context without the OCM binding refuse with `unsupported-lrf-context`. It
  evaluates nothing.
- `CEM-V1-056`: A tampered artifact is the verifier's REJECT (`artifact-digest-mismatch`, exit 1),
  and a missing one its UNSUPPORTED refusal (`artifact-unavailable`). A stable map other than the
  committed exact sidecar, a stale target or base, an OCM bound to another map digest or target, and
  an OCM missing an obligation also refuse. So does an artifact or repository-root change made after
  the first verification: the second verification refuses or differs, or the boundary is no longer
  valid. Every refusal returns no universe, so no consumer can
  derive a closed or empty frontier from it. A verifier refusal is returned as `StableRefusal`, which
  retains the full result and exit; `lrfrepo.CodeOf` reports its code.
- `CEM-V1-057`: Profile swaps refuse in both directions. A `cem/0.x` or relabeled map refuses in
  `VerifyStableUniverse` with `unsupported-spec`. A `cem/1.0` map refuses in the legacy
  `VerifyUniverse` with `unsupported-spec`, naming the stable universe verification.
- `CEM-V1-058`: The old OCM/LRF admission is unchanged for `cem/0.x`: `VerifyUniverse`,
  `lrf.Evaluate`, `ocm read`, `ocm prepare`, `ocm link` and `corvint lrf` keep their arguments,
  bytes and codes. Each of them now refuses a `cem/1.0` map explicitly with `unsupported-spec`,
  instead of reducing it or reporting it as malformed 0.x input. `ocm prepare` refuses before it
  writes or replaces anything.
- `CEM-V1-059`: A stable universe verification is a read. It MUST NOT change the repository HEAD,
  index or worktree status, the artifact files or the caller's input bytes.

Failure modes: when the change has structural mechanical hunks, a host whose Go toolchain is
outside the frozen structural runtime tuples gets the verifier's `UNSUPPORTED`
`unsupported-structural-runtime` refusal; the admission's own fixture has none, so this is not
exercised here. A map whose target commits a different sidecar is refused by the verifier's sidecar
binding. A target that commits no sidecar can still be accepted by the verifier (`ABSENT`); the
admission itself then refuses with `excluded-artifact-mismatch`. There is no stable LRF
result: evaluating the projected request needs an accepted stable LRF profile.

Rollback: remove `VerifyStableUniverse`, `StableUniverse`, `StableRefusal` and
`lrf.ValidateStableRequest`, and the explicit `cem/1.0` refusals in `VerifyUniverse`,
`readOCMBytes` and `PrepareOCM`. No `lrf/0` or OCM profile, default, packet or stored evidence
changes.

### OCM/LRF stable admission traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-051 | `TestCEMV1051StableUniverseAccepts` | A real `cem/1.0` change, with its exact sidecar committed at the target and six artifacts, is ACCEPT/EXACT; the universe keeps the exact bytes, digest, patch, target and LRF context |
| CEM-V1-052 | `TestCEMV1052StableUniverseAuthorityArguments` | Six artifact checks; `NOT_OBSERVED` authority axes; a missing artifact root, base, target or root is the verifier's `authority-arguments` refusal (exit 2) |
| CEM-V1-053 | `TestCEMV1053StableUniverseRetainsUnsatisfiedObligations` | `OCM-TEST-001` linked to the supported hunk; `OCM-TEST-002` stays `unknown`/`unassessed` |
| CEM-V1-054 | `TestCEMV1054StableUniverseReadOnlyDocument` | `Document` keeps criteria, receipts, links and artifacts; edits to returned copies never reach the universe |
| CEM-V1-055 | `TestCEMV1055StableLRFRequestAdmission` | `lrf.Evaluate` refuses the stable request; the stable admission refuses a `cem/0.2`, relabeled, target-less or OCM-unbound request |
| CEM-V1-056 | `TestCEMV1056StableUniverseRefusesStaleOrTamperedInput` | Tampered artifact REJECT (exit 1, result retained); missing artifact `artifact-unavailable`; swapped map `excluded-artifact-mismatch`; OCM `cem-map-digest-mismatch`/`target-mismatch`; an absent sidecar with a rebound OCM and a stale caller target `excluded-artifact-mismatch`; stale base; truncated OCM |
| CEM-V1-056 | `TestCEMV1056StableUniverseRevalidatesAfterOCMPhase` | An artifact changed after the OCM phase is the second verification's REJECT; a root replaced by a symlink is `repository-object-unavailable`; restored inputs admit again |
| CEM-V1-057 | `TestCEMV1057StableUniverseProfileSwapsRefuse` | `cem/0.2` and relabeled maps refuse in the stable route; `cem/1.0` refuses in `VerifyUniverse` |
| CEM-V1-058 | `TestCEMV1058LegacyOCMLRFAdmissionUnchanged` | The `cem/0.2` universe and `lrf.Evaluate` still accept; `ReadOCM`, `PrepareOCM` and `LinkOCM` (map unchanged) and `Evaluate` refuse `cem/1.0` with `unsupported-spec`; existing `internal/lrfrepo`, `conformance/ocm-v0`, frontier, TCQ and CLI LRF/OCM suites pass |
| CEM-V1-059 | `TestCEMV1059StableUniverseReadDoesNotMutate` | HEAD, porcelain status, artifact bytes and input bytes are unchanged after an accepted read |

## Core stable bundle export (V1-0632 S5b)

Status: accepted under the owner's standing auto-accept (2026-10-11), after independent review
(Codex round 8 PASS). Native work: V1-0632, migration row C10 of
`protocol/cem-1.0/stable/MIGRATION.json`. `cem export` over an exact `cem/1.0` map now writes a
relocatable stable bundle instead of refusing; the `corvint-receipt-bundle/0` export of a 0.x map
keeps its layout and bytes. The route stays hidden like the S1 read: help is unchanged.

- `CEM-V1-060`: Core `cem export` over a map whose bytes declare exactly `cem/1.0` MUST first run
  the stable verifier over the caller-named root byte for byte, as `CEM-V1-017` does, with
  `--artifacts` as its artifact root. Unless that result is ACCEPT (exit 0) it MUST return the
  verifier's complete result and exit as `stableVerification` with `ok: false` and write nothing.
- `CEM-V1-061`: An accepted export MUST write a new directory outside the worktree (the 0.x output
  rule) with profile `corvint-cem-stable-bundle/1`: `cem.json` holding the exact map bytes,
  `artifacts/<declared path>` holding the exact bytes of every artifact the map declares (all six
  roles, including the snapshot-head receipt), and a canonical one-line `manifest.json` written last
  that records the profile, spec, base, target and each file's path, digest and byte length. Every
  artifact is re-read and must match its declared digest; the map is never rewritten, relabelled
  or reduced.
- `CEM-V1-062`: The bundle MUST verify from any root: `ReadStable` / `VerifyStable`, and `cem
  verify-stable --map <dir>/cem.json --artifacts <dir>/artifacts`, over a moved bundle MUST
  reproduce the source's complete 21-field result and exit. Export MUST verify the written bundle
  the same way and remove it, with `publish-failed`, unless that result equals the source result.
- `CEM-V1-063`: `ReadStable` MUST refuse a bundle with a missing declared file
  (`stable-bundle-entry-missing`), a file or directory nothing declares
  (`stable-bundle-entry-unused`), a symlink or other non-regular entry anywhere, including the
  artifacts directory (`stable-bundle-entry-escaping`), bytes that differ from the declared digest
  or length (`stable-bundle-entry-digest`), or a manifest that is not exact or disagrees with the
  map (`stable-bundle-manifest`). `VerifyStable` MUST also refuse a bundle that records another base
  or target than the caller supplies (`stable-bundle-revision`). `WriteStable` MUST refuse a bundle
  that is not exactly its map's declared closure (base, order, role, path, bytes) before creating
  anything, and MUST write only inside the new bundle directory: it MUST refuse, before writing, a
  created directory whose name no longer holds that same real directory, whose opened root is
  another directory, or that is not empty; its cleanup MUST remove, without recursion and through
  the opened directory, only the files and directories the bundle declares, and then the name only
  while it still holds that directory. The writer's threat model excludes a concurrent process with
  write access to the output parent directory: against such a process it guarantees only that it
  never writes outside the directory it bound and never follows a swapped symlink (see the remaining
  limits below). Readers MUST open entries without following a final symlink
  or blocking on a substituted FIFO, MUST recheck each name, re-read every declared file against
  the digest it was admitted with, then walk the whole tree again comparing names, kinds and
  directory identities, and each file's identity, size, modification time and inode change time
  with the state its re-read opened (a host without a change time refuses), and check that the caller's path still names the opened directory, so a
  bundle changed during the read (including the bundle or an ancestor directory swapped for a
  symlink) is refused, and MUST read directories in
  bounded batches so a tree over the entry bound is refused before it is held in memory. The Go API (`CollectStable`,
  `WriteStable`, `ReadStable`, `VerifyStable`) is the reusable read/write surface for later
  attestation, Git-notes and example-CI consumers. The export route probe that selects this path
  MUST open the map the same nonblocking way.
- `CEM-V1-064`: A stable export MUST refuse `--witness` with `invalid-arguments` before
  verification, and an output inside the worktree with `bundle-output-refused` after verification;
  neither writes anything. A 0.x export MUST refuse `--artifacts` with `invalid-arguments`. 0.x
  export arguments, manifest, receipts and bytes are otherwise unchanged.

Failure modes: a stale base, tampered or omitted artifact, missing `--artifacts` or unsupported
runtime is the verifier's own REJECT or UNSUPPORTED result, carried unchanged with nothing
written. Closure checks over a received bundle run through the Go API or through `cem
verify-stable` (which checks every declared artifact but not unused entries); no CLI import verb is
added. Witness, dogfood and gate receipts are not carried in a stable bundle. The only 0.x change
is that `--artifacts` on a 0.x export is an `invalid-arguments` refusal instead of an
argument-parser refusal; both exit 2 with nothing written.

Remaining writer limits (out of scope under `CEM-V1-063`'s threat model; evidence: Codex review round 6,
recorded in `docs/build-log/2026-10-11-v1-0632-s5-bundle-export.md`): a concurrent process with write access to the output
parent can (1) move an empty directory of its own into the output name between `Mkdir` and the
writer's identity capture, which the writer then adopts and fills, and removes on a later failed
write; and (2) create a file at a declared path inside the bound directory after it is opened, so
the exclusive write fails and cleanup, which removes every declared path, removes that file.

Rollback: remove the stable branch in `receiptbundle.Export`, `stable_bundle.go`, the new
`Options` fields and the `--artifacts` export flag. No 0.x byte, map, packet or stored bundle
changes.

### Core stable bundle export traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-060 | `TestStableExportRelocatesWithSameVerification` | The export envelope's result equals `verify.Stable` over the source (ACCEPT/EXACT on Go 1.27.1 and 1.27.2) |
| CEM-V1-060 | `TestStableExportPublishesNothingForARefusedSource` | Stale base and tampered artifact (REJECT, exit 1), omitted artifact (UNSUPPORTED, exit 2), no artifact root and relative root (exit 2) return the verifier result with no bundle written; each absolute-root result equals `verify.Stable` over the same inputs |
| CEM-V1-061 | `TestStableExportRelocatesWithSameVerification` | Map bytes equal the sealed bytes; all six roles, including the snapshot-head receipt, are exact copies; the manifest digest is reported |
| CEM-V1-062 | `TestStableExportRelocatesWithSameVerification`, `TestCEMExportStableBundleRelocates` | After `os.Rename` to another parent, `VerifyStable` and `cem verify-stable` over the moved bundle equal the source result and exit |
| CEM-V1-062 | `TestStableBundleKeepsNestedPathsAcrossRoots` | Nested declared artifact paths keep their paths and digests across a move |
| CEM-V1-063 | `TestStableBundleRefusesIncompleteOrTamperedEntries` | Tampered snapshot-head or map byte, omitted artifact or map, extra file, extra directory, symlinked artifact or artifacts directory, inexact or extended manifest, and another target each refuse with their code (exit 2) |
| CEM-V1-063 | `TestWriteStableRefusesAnAlteredClosure` | An escaping path, a duplicated path (each with the artifact count kept), an added artifact, a relabelled or omitted role, altered bytes and another base refuse before anything is created; the output parent stays empty |
| CEM-V1-063 | `TestReadStableRefusesAnOversizedTree`, `TestStableEntryOpenDoesNotBlockOnAFIFO` | A tree over 4,096 entries refuses; opening a FIFO entry returns at once and the read refuses it |
| CEM-V1-063 | `TestReadStableRefusesABundleChangedDuringTheRead` | Between an entry's check and its open, swapping it for an in-root symlink to an undeclared sibling with the same bytes, rewriting an already-read `cem.json`, adding a file, or, during the re-reads, moving `artifacts` or the bundle itself aside and replacing it with a symlink to it, refuses as `stable-bundle-entry-escaping` |
| CEM-V1-063 | `TestReadStableRefusesAFileRewrittenAfterItsReread` | A declared file rewritten in place after its own re-read, while another file is re-read, refuses as `stable-bundle-entry-escaping`, both with its modification time moved and with the same length and its original modification time restored |
| CEM-V1-063 | `TestWriteStableRefusesASwappedBundleDirectory` | A created bundle directory replaced before its identity check by a non-empty sibling, or before its open by a symlink to that sibling or by another real directory, refuses as `publish-failed` with no file written or removed; a write failing after `cem.json` removes the bundle it started |
| CEM-V1-060 | `TestCEMExportStableKeepsCallerRoot` | For the alias, `<alias>/../repo`, `<repo>/`, `./repo`, `repo`, `./alias` and `--repo`, `cem export --root` equals `cem verify-stable --repository` on the same string (complete result, exit) and publishes nothing |
| CEM-V1-064 | `TestStableExportArgumentRefusals` | `--witness` and an in-worktree output refuse with nothing written; a 0.x export refuses `--artifacts` |

## Stable projections (V1-0632 S5c)

Status: accepted under the owner's standing auto-accept (2026-10-11) after independent review
(Codex round 3 PASS); delivery stays library-level and unpromoted. Native work: V1-0632, migration rows C13, C14 and C16 of
`protocol/cem-1.0/stable/MIGRATION.json`. Each family gains a read-only, library-level stable
route beside its unchanged 0.x route. Every route admits an exact `cem/1.0` map only through the
complete stable verifier (or, for the console, the strict stable wire reader with the verifier
named as NOT_RUN). No route reduces, re-encodes or relabels a stable map, writes a map, or treats a
reference as task completion or criterion satisfaction. No CLI flag, default or help entry changes.

- `CEM-V1-070`: The witness compiler's CEM source (`internal/witness/stable.go`) MUST select the stable reader by `wire.DeclaresStable` before the 0.x parser, run `verify.Stable` with the index root, the caller's expected base, the index commit and an explicit artifact root, and bind the source (`BOUND`) only on an `ACCEPT` result whose `mapSha256` equals the digest of the exact bytes read and whose base equals the expected base. The source keeps the complete verifier result under profile `corvint-witness-stable-binding/1` with `criterionSatisfaction: NOT_OBSERVED`.
- `CEM-V1-071`: A stable witness source without an artifact root MUST be `UNSUPPORTED` with no verifier result; a verifier refusal at the binding stage MUST be `UNBOUND` and any other refusal `INVALID`. A refused source MUST keep the full result and, whenever the stable reader returns a document (also beside a graph error), the map's hunk and basis counts and MUST apply no citation, so the report keeps its no-bound-evidence-map precondition.
- `CEM-V1-072`: A report built over a stable source MUST take profile `corvint-witness-stable/0`. Stable citations MUST stay non-closing, and a 0.x source MUST encode no stable field.
- `CEM-V1-073`: `witnesscollapse.CompileStable` MUST admit a stable map only through an `ACCEPT` verifier result over the complete input binding (repository, expected base, target, artifact root) bound to the exact map digest, then collapse over the private mechanics view under profile `corvint-correlated-witness-collapse/experimental-stable-v0` with `cem.spec` `cem/1.0`. The report MUST carry the complete verifier result (every original hunk with its caller coverage and discrimination states, every reference, the artifact checks and the `NOT_OBSERVED` axes) and `criterion_satisfaction: NOT_OBSERVED`. `VerifyStable` MUST recompile from the same bytes and inputs and refuse any difference in the key-sorted JSON value of the report with `report-mismatch`, so a serialized and decoded report verifies and an edited one does not.
- `CEM-V1-074`: The 0.x `Compile` MUST keep refusing a stable map. `CompileStable` MUST refuse a non-stable map with `not-stable`, and any verifier refusal or digest mismatch with `stable-verification-refused`, returning a report that holds only the full verifier result and no hunk, group or cause. Evidence or artifact drift after compilation MUST make `VerifyStable` refuse rather than yield an equivalent report. A nil mechanics view fails with `cem-unavailable`.
- `CEM-V1-075`: `doccorpus.StableCEMProjection` MUST project documentation claims through a stable map only after the stable verifier accepts the exact bytes. Its receipt (schema `corvint-corpus-cem-stable/1`) MUST carry the full map digest and spec, the unmodified verifier result, reference counts marked `REFERENCE_INTEGRITY_ONLY`, `native_authority: NOT_OBSERVED`, and each claim's original immutable span at the verified base bound by evidence identity. It MUST offer no writer argv and MUST NOT encode a reduced stable map. The 0.x `CEMProjection` keeps refusing a stable map.
- `CEM-V1-076`: A wrong base, edited map bytes, a tampered artifact, a missing artifact root and a non-stable map (which still goes through the verifier and is marked `cem_spec: UNSUPPORTED`) MUST refuse with `corpus-stable-refused`, an outcome `REFUSED`, no claim row and the full verifier result kept; a claim whose span differs at the verified base MUST refuse instead of citing.
- `CEM-V1-077`: The console chain pane MUST admit `cem/1.0` in its closed profile registry only when the strict stable wire reader accepts the whole sealed document. It MUST draw the verification envelope as an `unverified` gap naming the stable verifier as NOT_RUN, the map digest pin, the sidecar state `NOT_CHECKED`, the integrity axes `NOT_CHECKED` and the native authority, criterion adequacy, criterion discrimination, runner execution and authentication axes `NOT_OBSERVED`.
- `CEM-V1-078`: Each criterion binding, runner receipt (with its execution authority, source binding, dependency closure and authentication states), criterion link and artifact MUST render as a linked reference row stating that a valid reference is not task completion and not criterion satisfaction, followed by a `task completion NOT_OBSERVED` gap. The pane MUST NOT open or execute artifact content. The caller's survived discrimination state and a stale seal stay visible.
- `CEM-V1-079`: The registry stays closed: an unsupported profile, a near-stable spec, an unknown member, a duplicate or case-variant `spec` member that names `cem/1.0` beside any other value, and a stable map the strict reader refuses MUST draw no chain; 0.x maps keep their reading.

Failure modes: a host whose Go toolchain is not an admitted structural tuple gets the verifier's
`UNSUPPORTED` runtime refusal, which every witness, collapse and documentation route keeps as a
refusal. The console never claims verification; its stable rows are display only.

Rollback: remove `internal/witness/stable.go`, `internal/witnesscollapse/stable.go`,
`internal/doccorpus/cem_stable.go` and `internal/console/chain_stable.go` with the stable
branches that call them; restore `cem/1.0` to unsupported in the console registry. No 0.x
profile, default, packet or stored evidence changes.

### Stable projection traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-070 | `TestStableWitnessSourceAdmitsVerifiedMap` | The public sealed positive binds `BOUND` with the complete ACCEPT result, EXACT sidecar, map digest and axes |
| CEM-V1-071 | `TestStableWitnessSourceRefusals` | No artifact root is `UNSUPPORTED`; a wrong base and a tampered artifact refuse with counts and the result kept |
| CEM-V1-072 | `TestStableWitnessSourceAdmitsVerifiedMap`, `TestStableWitnessReportProfile` | Stable report profile, non-closing citations, WITNESS-P3 kept for an unadmitted source, no stable field on 0.x |
| CEM-V1-073 | `TestStableCollapseRetainsFullMap` | Canonical report keeps the survived witness, killed/survived counts, runner receipts and `NOT_OBSERVED`; the in-memory and the serialized-then-decoded report verify; an edited report and a decoded report claiming satisfaction mismatch |
| CEM-V1-074 | `TestStableCollapseRefusals` | 0.x refusal; wrong base, no artifacts, edited map and tampered artifact refuse; unknown witness after admission; drift refuses `VerifyStable` |
| CEM-V1-075 | `TestStableCEMProjectionCitesVerifiedSpan` | The policy span cites its evidence id at the verified base; no writer argv; 0.x projection refuses |
| CEM-V1-076 | `TestStableCEMProjectionRefusals` | Wrong base, no artifacts, edited map, 0.x map, case-variant spec, edited claim span and tampered artifact refuse, each non-span refusal with a `REFUSED` receipt and the full verifier result |
| CEM-V1-077 | `TestConsoleChainStableDisplay` | Envelope, digest pin, sidecar and axis rows render for the sealed public positive |
| CEM-V1-078 | `TestConsoleChainStableDisplay`, `TestConsoleChainStableStale` | Reference rows say not task completion; artifact content not opened; survived witness and stale seal shown |
| CEM-V1-079 | `TestConsoleChainStableRegistry` | `cem/2.0`, `cem/1.0.0`, an unknown member, duplicate and case-variant `spec` members and an invalid stable map refuse; 0.x reads |
