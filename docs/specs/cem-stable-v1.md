# CEM stable 1.0

Intent status: owner scope accepted; technical contract proposed
Delivery status: planned

## Agent digest
- Claim: Owner requires portable task-linked execution evidence and all main runners for Corvint1.0; technical contract and stable promotion remain open.
- Status: owner scope accepted; technical contract proposed/planned; no stable delivery or promotion.
- Exists: experimental runner profiles, native Tasks criterion binding and portable candidate proofs; hidden Core status/verify stable read (proposed `CEM-V1-017..019`), hidden Core report/MCP preview (proposed `CEM-V1-034..036`) and explicit Core mutator refusals of `cem/1.0` (proposed `CEM-V1-030..033`), library-level stable witness, collapse, documentation and console projections (proposed `CEM-V1-070..079`); qualification remains bounded.
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
| CEM-V1-007, 014 | Optional companion producer, explicit stable verification, the hidden Core status/verify read (`CEM-V1-017..019`) and the Core report/MCP preview (`CEM-V1-034..036`) | Other Core workflow, OCM/frontier/obligation, companion and completion consumers |
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

## Stable projections (V1-0632 S5c)

Status: proposed. Native work: V1-0632, migration rows C13, C14 and C16 of
`protocol/cem-1.0/stable/MIGRATION.json`. Each family gains a read-only, library-level stable
route beside its unchanged 0.x route. Every route admits an exact `cem/1.0` map only through the
complete stable verifier (or, for the console, the strict stable wire reader with the verifier
named as NOT_RUN). No route reduces, re-encodes or relabels a stable map, writes a map, or treats a
reference as task completion or criterion satisfaction. No CLI flag, default or help entry changes.

- `CEM-V1-070`: The witness compiler's CEM source (`internal/witness/stable.go`) MUST select the stable reader by `wire.DeclaresStable` before the 0.x parser, run `verify.Stable` with the index root, the caller's expected base, the index commit and an explicit artifact root, and bind the source (`BOUND`) only on an `ACCEPT` result whose `mapSha256` equals the digest of the exact bytes read and whose base equals the expected base. The source keeps the complete verifier result under profile `corvint-witness-stable-binding/1` with `criterionSatisfaction: NOT_OBSERVED`.
- `CEM-V1-071`: A stable witness source without an artifact root MUST be `UNSUPPORTED` with no verifier result; a verifier refusal at the binding stage MUST be `UNBOUND` and any other refusal `INVALID`. A refused source MUST keep the full result and the map's hunk and basis counts and MUST apply no citation, so the report keeps its no-bound-evidence-map precondition.
- `CEM-V1-072`: A report built over a stable source MUST take profile `corvint-witness-stable/0`. Stable citations MUST stay non-closing, and a 0.x source MUST encode no stable field.
- `CEM-V1-073`: `witnesscollapse.CompileStable` MUST admit a stable map only through an `ACCEPT` verifier result over the complete input binding (repository, expected base, target, artifact root) bound to the exact map digest, then collapse over the private mechanics view under profile `corvint-correlated-witness-collapse/experimental-stable-v0` with `cem.spec` `cem/1.0`. The report MUST carry the complete verifier result (every original hunk with its caller coverage and discrimination states, every reference, the artifact checks and the `NOT_OBSERVED` axes) and `criterion_satisfaction: NOT_OBSERVED`. `VerifyStable` MUST recompile from the same bytes and inputs and refuse any difference with `report-mismatch`.
- `CEM-V1-074`: The 0.x `Compile` MUST keep refusing a stable map. `CompileStable` MUST refuse a non-stable map with `not-stable`, and any verifier refusal or digest mismatch with `stable-verification-refused`, returning a report that holds only the full verifier result and no hunk, group or cause. Evidence or artifact drift after compilation MUST make `VerifyStable` refuse rather than yield an equivalent report. A nil mechanics view fails with `cem-unavailable`.
- `CEM-V1-075`: `doccorpus.StableCEMProjection` MUST project documentation claims through a stable map only after the stable verifier accepts the exact bytes. Its receipt (schema `corvint-corpus-cem-stable/1`) MUST carry the full map digest and spec, the unmodified verifier result, reference counts marked `REFERENCE_INTEGRITY_ONLY`, `native_authority: NOT_OBSERVED`, and each claim's original immutable span at the verified base bound by evidence identity. It MUST offer no writer argv and MUST NOT encode a reduced stable map. The 0.x `CEMProjection` keeps refusing a stable map.
- `CEM-V1-076`: A wrong base, edited map bytes, a tampered artifact, a missing artifact root and a non-stable map MUST refuse with `corpus-stable-refused`, an outcome `REFUSED`, no claim row and the full verifier result kept; a claim whose span differs at the verified base MUST refuse instead of citing.
- `CEM-V1-077`: The console chain pane MUST admit `cem/1.0` in its closed profile registry only when the strict stable wire reader accepts the whole sealed document. It MUST draw the verification envelope as an `unverified` gap naming the stable verifier as NOT_RUN, the map digest pin, the sidecar state `NOT_CHECKED`, the integrity axes `NOT_CHECKED` and the native authority, criterion adequacy, criterion discrimination, runner execution and authentication axes `NOT_OBSERVED`.
- `CEM-V1-078`: Each criterion binding, runner receipt (with its execution authority, source binding, dependency closure and authentication states), criterion link and artifact MUST render as a linked reference row stating that a valid reference is not task completion and not criterion satisfaction, followed by a `task completion NOT_OBSERVED` gap. The pane MUST NOT open or execute artifact content. The caller's survived discrimination state and a stale seal stay visible.
- `CEM-V1-079`: The registry stays closed: an unsupported profile, a near-stable spec, an unknown member and a stable map the strict reader refuses MUST draw no chain; 0.x maps keep their reading.

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
| CEM-V1-073 | `TestStableCollapseRetainsFullMap` | Canonical report keeps the survived witness, killed/survived counts, runner receipts and `NOT_OBSERVED`; recompilation verifies; an edited report mismatches |
| CEM-V1-074 | `TestStableCollapseRefusals` | 0.x refusal; wrong base, no artifacts, edited map and tampered artifact refuse; unknown witness after admission; drift refuses `VerifyStable` |
| CEM-V1-075 | `TestStableCEMProjectionCitesVerifiedSpan` | The policy span cites its evidence id at the verified base; no writer argv; 0.x projection refuses |
| CEM-V1-076 | `TestStableCEMProjectionRefusals` | Wrong base, no artifacts, edited map, 0.x map, edited claim span and tampered artifact refuse |
| CEM-V1-077 | `TestConsoleChainStableDisplay` | Envelope, digest pin, sidecar and axis rows render for the sealed public positive |
| CEM-V1-078 | `TestConsoleChainStableDisplay`, `TestConsoleChainStableStale` | Reference rows say not task completion; artifact content not opened; survived witness and stale seal shown |
| CEM-V1-079 | `TestConsoleChainStableRegistry` | `cem/2.0`, `cem/1.0.0`, an unknown member and an invalid stable map refuse; 0.x reads |
