# CEM stable 1.0

## Agent digest

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
