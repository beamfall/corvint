# CEM stable 1.0

Intent status: owner scope accepted; technical contract proposed
Delivery status: planned

## Agent digest
- Claim: Owner requires portable task-linked execution evidence and all main runners for Corvint1.0; technical contract and stable promotion remain open.
- Status: owner scope accepted; technical contract proposed/planned; no stable delivery or promotion.
- Exists: experimental runner profiles, native Tasks criterion binding and portable candidate proofs; qualification remains bounded.
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
| CEM-V1-050 | `structuralRuntimeAdmitted` in `internal/cem/verify/stable.go` and `interop/cem01-go/cem.go`; sealed `protocol/cem-1.0/structural-tuples/go1.27.2/`; `TestCEMV1050StructuralTupleAdmission` (native and portable); S0E, linked-control, ledger and cell tests bound through the tuple packet | Linux and Windows hosts, CI-hosted go1.27.2 runs and the full structural corpus remain NOT_RUN |

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
| CEM-V1-007, 014 | Optional companion producer and explicit stable verification | Core workflow/report, OCM/frontier/obligation, companion and completion consumers |
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
  frozen packet is never rewritten. Tests run the S0E cases, linked control, ledgers and envelope
  cells on an admitted tuple only through that binding and report NOT_RUN otherwise.
  Qualification stays `EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS`; admitting a tuple does not complete
  the full corpus. Rollback removes the tuple from both lists and its packet; the frozen packet and
  `go1.27.1` behaviour are unchanged.

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
| CEM-V1-PSR-004 | `examples/cem/verify-portable.sh`, `examples/cem/recipes/ci-verify.sh` | `TestExampleStableBranchVerifies` (go1.27.1 only), `TestExampleStableBranchRuntimeRefusal` (other runtimes), `TestExampleStableBranchRefusals`; `script/cem-recipes_test.sh` case `c-stable` |
