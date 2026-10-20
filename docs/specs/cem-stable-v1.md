# CEM stable 1.0

Intent status: owner scope accepted; technical contract proposed
Delivery status: implemented and produced on main (decision 0525, CEM-V1-140..152, V1-1276); not promoted; qualification bounded

## Agent digest
- Claim: Owner requires portable task-linked execution evidence and all main runners for Corvint1.0; technical contract and stable promotion remain open.
- Status: owner scope accepted; technical contract proposed; implemented and produced on main (decision 0525, CEM-V1-140..152, V1-1276); not promoted; qualification bounded.
- Exists: experimental runner profiles, native Tasks criterion binding and portable candidate proofs; hidden Core status/verify stable read (proposed `CEM-V1-017..019`), hidden Core report/MCP preview (proposed `CEM-V1-034..036`), hidden Core stable bundle export (accepted `CEM-V1-060..064`), explicit Core mutator refusals of `cem/1.0` (proposed `CEM-V1-030..033`, narrowed to `cem/0.x` producers), the Core `cem/1.0` producer and dogfood loop cutover (accepted `CEM-V1-140..152`, V1-1276), the internal OCM/LRF stable universe admission (accepted `CEM-V1-051..059`, no stable LRF evaluation) and library-level stable witness, collapse, documentation and console projections (accepted `CEM-V1-070..079`); a provider-neutral plain criterion form verified by both consumers without a Tasks store (proposed `CEM-V1-083`); DSSE Ed25519 runner receipt authentication that rises to VERIFIED only under a consumer `--trusted-keys` file (proposed `CEM-V1-084`); optional stable map `provenance` and `role` members, a closed producer-only member list emitted empty, and the 0.1/0.3 algorithms re-homed as 1.0-normative `ALGORITHMS.md` text (proposed `CEM-V1-093..095`); a deterministic cem/0.x to cem/1.0 map converter with provenance and full-corpus verdict-parity evidence (proposed `CEM-V1-096..105`, satisfied by the frozen V1-1277 evidence; the converter no longer ships); the one-commit migration of every committed map to `cem/1.0` (accepted `CEM-V1-230..232`, V1-1277); the core 0.x retirement: a stable reader refusal of `cem/0.1..0.3` naming the merge-forward fix, a committed-corpus check, a `cem/1.0`-only sealed-constituent admission with a no-coverage rule for a retired seal, and the converter's removal (proposed `CEM-V1-200..204`, V1-1278 S1); local-completion sidecar-only reuse of a committed `cem/1.0` sidecar on a stable verifier `ACCEPT` over its exact bytes with the absolute repository root and full object IDs, with closed refusal reasons and `CEM-V1-150`'s fail-closed default otherwise (proposed `CEM-V1-205..209`, V1-1303); every consumer outside `internal/cem` reads `cem/1.0` only and refuses a retired map with `unsupported-spec`, with the frontier-v0, ocm-v0 and cli-parity-v0 freezes superseded under decision 0525 §6 (proposed `CEM-V1-210..214`, V1-1278 S2a); the 0.x wire, verifier, producers and `cem` CLI routes, the `frontier/0`, `tcq/0`, `lrf/0` and `external-frontier-obligations/0` routes and the attest `cem/0`/`cem/v1` predicates are deleted, leaving one wire (proposed `CEM-V1-215..219`, V1-1278 S2b); the `protocol/cem-0.2`, `protocol/cem-0.3`, `interop/cem-0.1`, `conformance/cem-0.1`/`cem-0.2` packets, `examples/cem`, `tools/cem-interop-runner` and the portable consumer's 0.x routes are deleted, the inherited text moves into the self-contained `protocol/cem-1.0` packet and the 0.x specs are retired, completing decision 0525 §1 step 7 (proposed `CEM-V1-220..225`, V1-1278 S3); one shared `cem/1.0` tamper matrix in the packet that the native and portable consumers both run with the same pinned verdicts (proposed `CEM-V1-226..229`, V1-1340); `corvint-cem-anchor/1` anchor pointers that read `verified` only when the anchored map describes the commit's canonical patch, `unscoped` legacy `/0` pointers and a `--notes-tip` rollback read (accepted `CEM-V1-067`, V1-1260); the portable consumer's process containment qualified on Darwin and Linux amd64/arm64 through a pidfd/epoll keeper and a `/proc` quiet proof, with every other host still refusing `unsupported-process-containment` (accepted `CEM-V1-180..184`, V1-1358); truthful packet status, the citation rules, stop list, result serialization and diff recipe restated in the packet, and native-generated citation vectors both consumers run (accepted `CEM-V1-107..112`, V1-1333); a discrimination witness whose `mutants` exceeds `bounds.maxMutants` refused `invalid-field` by both consumers (accepted `CEM-V1-090`, V1-1259); a normative nested result member order with pinned whole result lines both consumers reproduce (accepted `CEM-V1-113`, V1-1369); the consumer's Git capability contract (`--attr-source`, Git 2.41 or later, else `repository` / `git-diff-failed`), its PATH-only Git resolution and the hosted `macos-15` qualification through the Tasks gate's own Git (accepted `CEM-V1-185..189`, V1-1377); `cem cite` refusing a governing instruction file the map's own patch changes with `cite-self-modified-authority` (accepted `CEM-V1-114`, V1-0414); `cem status` reporting each governing instruction file the patch changes as `selfModifiedAuthority`, advisory and verdict-neutral (accepted `CEM-V1-270`, V1-0414); `UNSUPPORTED` argument refusals with a stderr or envelope absolute-path fix and a pinned reference precedence, so both consumers return the same outcome, stage and code for the same input (accepted `CEM-V1-115..117`, `CEM-V1-176..177`, V1-1291/V1-1306/V1-1289); an opt-in consumer hunk-revert falsifier, `cem verify --hunk-revert`, that reports `held` beside the unchanged result only from its own `go test` re-execution (accepted `CEM-V1-088..089`, V1-1266; its disposable export refuses a repository-local `TMPDIR`, accepted `CEM-V1-240`, V1-1452); an opt-in, append-only verdict ledger under `refs/notes/corvint-verdicts` that `cem anchor --artifacts` extends with the native `verdictSha256`, refusing a differing verdict for the same map and commit with `verdict-ledger-conflict`, and a read-only `cem provenance --verdicts` listing (accepted under the owner's standing auto-accept (2026-10-11) `CEM-V1-271..279`, V1-1292); opt-in runner receipt admission in the plain producer and an end-to-end fixture chain from a native Tasks attempt through the runner, the producer and the native and portable verdict gate (`CEM-V1-280..287`, accepted under the owner's standing auto-accept (2026-10-11), V1-0632); a positive `cem/1.0` read for the dashboard's separate `cem-stable` document and the local-authority capsule's stable closure, each with stale and incomplete binding controls (accepted under the owner's standing auto-accept (2026-10-11), `CEM-V1-290..299`, V1-0632); qualification remains bounded.
- Blocked on: technical contract acceptance and stable promotion, all supported runtime qualification and independent interoperability/outcome evidence.
- Read next: Owner context; Requirements; Acceptance evidence.

## Owner context

- Owner intent: accepted scope, 2026-10-01 conversation; CEM1.0 belongs in the main Corvint1.0 release and supports actual main test runners across supported languages, linked to native Tasks criteria.
- Technical contract: proposed. No version alias, consumer promotion or release qualification is established by this document.
- Native work: V1-0591; runner lanes V1-0592..0594; portable wire V1-0595; qualification V1-0596. Existing experimental CEX work remains V1-0575.
- Original evidence: `cem-criterion-experiments-v0.md`, `tasks-criterion-binding-v0.md`, `test-runner-execution-v0.md`; immutable Git CEM binding is governed by this contract and the inherited text under `protocol/cem-1.0/stable/inherited/`; `cem-0.2-canonical-binding.md` and `cem-0.3-structural-mechanical.md` are retired by decision 0525 (`CEM-V1-225`).

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
| CEM-V1-001, 011..013 | `internal/cem/wire/candidate.go`, `internal/cem/verify/candidate.go`; `TestCandidatePortablePacket`; independent `interop/cem01-go` `TestCandidateNormativePacket`; 31 public cases in `protocol/cem-1.0/manifest.json` | The candidate stays an experimental reference profile with the 0.2 hunk vocabulary only (`CEM-V1-013`); the stable `cem/1.0` profile is implemented and produced on main (decision 0525, `CEM-V1-140..152`, V1-1276), its technical contract proposed and not promoted, qualification bounded |
| CEM-V1-002 | Native `internal/tasks/criterionbinding`, CLI read verbs and `internal/criterionexperiment/tasks_unix.go`; TCB-V0 traceability below its owning spec | A coherent same-target native Tasks/CEX/generic Go/candidate join is retained in `/private/tmp/cem10-build/coherent-join/summary.json`; immutable execution-at-commit attestation and integrated candidate production remain open |
| CEM-V1-003..004, 006 | `internal/testrunner`, registry and `cmd/corvint-test-runner`; TRE-V0 traceability and adapter provenance | Full negative/retry/lifecycle/platform matrices and missing application/device/domain tuples remain open |
| CEM-V1-005 | Existing experimental criterion lifecycle retains three killed controls under its own reviewed oracle contract | Cross-language criterion discrimination is not established by runner failures or candidate links |
| CEM-V1-007, 014..016 | Additive native and portable candidate verification plus the optional `internal/cemcandidate` producer and `cmd/corvint-cem-candidate`; byte-compatible runner envelope extraction; actual same-target assembly and independent `/private/tmp/cem10-build/cem-next/review1.json` PASS | Existing producer/report/OCM/frontier/companion consumer migrations and their compatibility inventory remain open |
| CEM-V1-008..010 | Historical defaults and packets retained; optional-profile rollback defined | Independent external consumer, matched external outcomes and terminal release/promotion evidence remain open |
| CEM-V1-051..059 | `lrfrepo.VerifyStableUniverse`, `StableUniverse`, `StableRefusal`, `lrf.ValidateStableRequest`; explicit `cem/1.0` refusals in `VerifyUniverse`, `readOCMBytes`, `PrepareOCM`; tests in the OCM/LRF stable admission traceability table | Stable LRF evaluation needs an accepted stable LRF profile; no CLI or MCP route; OCM writers stay `cem/0.x` only; C06..C09 and C17 consumers remain open |
| CEM-V1-050 | `structuralRuntimeAdmitted` in `internal/cem/verify/stable.go` and `interop/cem01-go/cem.go`; sealed `protocol/cem-1.0/structural-tuples/go1.27.2/`; `TestCEMV1050StructuralTupleAdmission` (native and portable); S0E, linked-control, ledger and cell tests bound through the tuple packet | Linux and Windows hosts, CI-hosted go1.27.2 runs and the full structural corpus remain NOT_RUN |
| CEM-V1-080..082 | `internal/cem/verify/support.go`, `interop/cem01-go/citation.go` and `testdata/citation/`; `TestCEMV1080*`, `TestCEMV1081*`, `TestCEMV1082*` in `internal/cem/verify`, `internal/cem/workflow` and `interop/cem01-go`, including `TestCEMV1082MeasureVectorsNative` and `TestCEMV1082MeasureVectorsPortable` over `protocol/cem-1.0/stable/citation-vectors/measures.json` (V1-1331, V1-1335, V1-1332 under decision 0530; mutations in `docs/build-log/2026-10-15-v1-1331-1335-1332-citation-measures.md`) | Recorded limits: `held` never produced, no `symbol` anchors, no threshold or cut-off, stable-result emission is CEM-V1-092; historical 591-map report in the build log, computed before the decision 0530 amendments |
| CEM-V1-083 | Plain criterion form in `internal/cem/wire/stable.go` and `interop/cem01-go/cem.go`; public case `protocol/cem-1.0/stable/neutral-criterion/`; `TestCEMV1083*` in `internal/cem/wire`, `internal/cem/verify` and `interop/cem01-go` | The plain form is produced by the experimental `cem-stable-plain-assembly/1` request of `corvint-cem-candidate assemble-stable` (`CEM-V1-240`, V1-1290); no default `corvint cem` verb emits it; stranger time NOT_OBSERVED |
| CEM-V1-240 | `AssemblePlain` in `internal/cemcandidate/plain.go` and the profile dispatch in `cmd/corvint-cem-candidate/stable.go`; `TestCEMV1240*` in `internal/cemcandidate` and `cmd/corvint-cem-candidate`; timed run in `docs/build-log/2026-10-15-cem-plain-criterion-producer.md` | Experimental companion, not promoted; the runner plan and receipt are opaque inputs checked only by SHA-256, so receipt authority and execution at the target stay `NOT_OBSERVED`; one runner receipt per map; stranger time NOT_OBSERVED |
| CEM-V1-084 | `internal/cem/wire/stable_auth.go`, `internal/cem/verify/stable_auth.go`, `interop/cem01-go/stable_auth.go`; `--trusted-keys` in `internal/cem/cli/stable.go` and both portable commands; public case `protocol/cem-1.0/stable/receipt-authentication/`; `TestCEMV1084*` in `internal/cem/wire`, `internal/cem/verify`, `interop/cem01-go`, `internal/console` and `cmd/corvint` | No producer signs receipts (operator step, FPK-V0-036); `RESULT.schema.json` and the sealed S0E packet still name only `NOT_OBSERVED` for `authentication` and lack the four attestation issue codes (V1-1265); keys are TEST-ONLY; stranger time NOT_OBSERVED |
| CEM-V1-091 | `tools/cem-stable-regen` and the `CEM_STABLE_REGEN_CAPTURE` hook in `internal/cem/verify/stable_repository_test.go`; `TestCheckPassesOnRepositoryPacket`, `TestCheckCoversEveryStableManifestEntry`, `TestRegenerateRealPacketCopy`, `TestRegenerateRefusals`, `TestRegenerateSyntheticPacket`, `TestReplaceRealPacketCopy`, `TestReplaceRefusals`; proof runs in `docs/build-log/2026-10-11-cem-stable-regen-tooling.md` and `docs/build-log/2026-10-15-v1-1331-1335-1332-citation-measures.md` | V1-1265 added `citation` (CEM-V1-092) and `verdictSha256` (CEM-V1-066) through the tool; since V1-1287 `--check` verifies every stable manifest entry (see "Stable manifest entry repair"); V1-1331 re-emitted `citation` and `verdictSha256` of 3 cases and 11 envelope cells with `--replace` under decision 0530 |
| CEM-V1-092 | `citation` member of `StableResult` (`internal/cem/verify/stable.go`) and `stableResult` (`interop/cem01-go/cem.go`, `stable_repository.go`), regenerated into the sealed packet and the public `neutral-criterion` and `receipt-authentication` expectations; `TestCEMV1092CitationOnlyOnAccept`, `TestCEMV1092SealedCasesCarryCitationOnlyOnAccept`, `TestCEMV1092RefuseAndRejectClearCitation` (native); `TestCEMV1092PortableCitationOnlyOnAccept`, `TestCEMV1082StableResultCarriesTheBlock`, `TestStableCanonicalFullResults` (portable) | The go1.24.13 cases 21 and 22 were not run on this host: their values come from the override file (case 21 derived natively on go1.27.2 with every other non-runtime key matching); ten envelope cells take `--cell-values` overrides |
| CEM-V1-066 | `verdictSha256` member and `VerdictDigest` of `StableResult` (`internal/cem/verify/stable.go`, `stable_verdict.go`) and `stableResult` (`interop/cem01-go/cem.go`, `verdict.go`), regenerated into the sealed packet and the public `neutral-criterion` and `receipt-authentication` expectations; `TestCEMV1066SealedVerdictDigests`, `TestCEMV1066VerdictDigestCoversOnlyTheVerdict`, `TestCEMV1066EveryOutcomeCarriesVerdictDigest` (native); `TestCEMV1066PortableSealedVerdictDigests`, `TestCEMV1066PortableVerdictDigestCoversOnlyTheVerdict`, `TestCEMV1066PortableEveryOutcomeCarriesVerdictDigest`, `TestStableCanonicalFullResults` (portable) | The go1.24.13 cases 21 and 22 were not run on this host: their values come from the override file, computed from their sealed expectations; ten envelope cells take `--cell-values` overrides; the experimental Tasks verdict gate pins it and requires the portable consumer to recompute it (`docs/specs/tasks-criterion-binding-v0.md` TCB-V0-017; decision 0525 step 8, V1-1283) |
| CEM-V1-093..095 | `internal/cem/wire/stable_provenance.go`, `interop/cem01-go/stable_provenance.go`; `provenance`, `role`, `sha1Oid` in `protocol/cem-1.0/stable/stable.schema.json`; re-homed 0.1/0.3 text in `protocol/cem-1.0/stable/ALGORITHMS.md`; shared vectors `interop/cem01-go/testdata/stable-map-members/cases.json`; `TestCEMV1093*`, `TestCEMV1094*`, `TestCEMV1095*` in `internal/cem/wire` and `interop/cem01-go` | The V1-1275 converter (CEM-V1-096..105) emits `provenance` but never `role`; `cem status --citation` role grouping is `CEM-V1-106` (V1-1281); provenance OIDs are SHA-1 only; the sealed S0E packet copy of `stable.schema.json` is unchanged; stranger time NOT_OBSERVED |
| CEM-V1-096..105 | Removed by V1-1278 (`CEM-V1-204`); satisfied by the frozen V1-1277 evidence; the converter no longer ships. Historically `internal/cem/convert`, `internal/cem/cli/convert.go` (`corvint cem convert`), `tools/cem-convert-parity`; `TestCEMV1096*`..`TestCEMV1104*` in `internal/cem/convert`, `cmd/corvint`, `tools/cem-convert-parity` and `interop/cem01-go`; full-corpus parity `docs/build-log/evidence/v1-1275/` | Committed maps converted by V1-1277 (`CEM-V1-230`); direct verification at a bind commit is `REJECT excluded-artifact-mismatch` until §5 sealing; cem/0.1 out-of-band patch parity limit (kept, decision 0532); the f0fda542 partial drift difference on the resource-limit abort was a native drift charge, fixed by `CEM-V1-240` (V1-1295); removal with 0.x is V1-1278 |
| CEM-V1-230..232 | Migration commit `0b6f1bc245b44a8620c5e64f91e87f6d08da08d0`; since V1-1278 the committed corpus is checked by `TestCEMV1201*` (`internal/cem/wire` and `internal/cem/corpusbinding`, `CEM-V1-201`); historically `script/cem-migrate-maps.sh`, `tools/cem-convert-parity --migrated`, `TestCEMV1100ConvertCorpusIdempotent`, `TestCEMV1230MigratedSourceChecks`; evidence `docs/build-log/evidence/v1-1277/` | Direct verification of a migrated map at its bind commit stays `REJECT excluded-artifact-mismatch`; checks run at T'; the in-tree citation tool has no T' target; a `cem/0.x` map landing later needs a follow-up migration commit; the binding test is NOT_RUN where Git objects are absent outside CI and fails under `CI=true` (V1-1334) |
| CEM-V1-200..204 | `RetiredSpec`/`RetiredSpecFix` and the `ParseStable` refusal in `internal/cem/wire/stable.go`; `internal/cem/wire/corpus_test.go`, `internal/cem/corpusbinding/binding_test.go`; `verifySealedCEM`, `proves` and the retired-seal branch in `internal/dogfoodflow/batch.go` and `change.go`; the `verify-stable`-only dispatch in `internal/cem/cli/stable.go`; `TestCEMV1200*`, `TestCEMV1201*`, `TestCEMV1204*`, `TestBatchCheckCEMV1203*`, `TestBatchCheckDCWV0039RequiresStableVerification` | The `cem/0.x` wire, verifier, producers and their goldens remain until their consumers move (V1-1278 S2/S3); `cli-parity` `cem-begin-map` still pins `cem/0.1` |
| CEM-V1-205..209 | `stableSidecarReuse` in `internal/localcompletion/sidecar_reuse.go`, `checkQualification` in `lifecycle.go` and the `reuseRefused` member of `CheckObservation`; `TestCEMV1205*`..`TestCEMV1208*` in `internal/localcompletion` | One verifier run per reusing check per evaluation; SHA-256 object-format repositories never reuse; `LCP-V0-004` text unchanged; skipped outside darwin and linux |
| CEM-V1-210..214 | Consumer refusals in `internal/lrfrepo`, `internal/frontier*`, `internal/extevidence`, `cmd/corvint/prove.go`, `internal/attest`, `internal/gitnotes`, `internal/console`, `internal/cem/workflow/read.go`, `internal/receiptbundle`, `internal/witness*`, `internal/doccorpus`, `internal/criterionexperiment`, `internal/cemcandidate`, `internal/localcompletion`, `tools/local-authority`, `tools/cem-citation-baseline`; conformance supersessions in `conformance/ocm-v0`, `conformance/frontier-v0` and `conformance/cli-parity-v0`; tests in the consumer cem/1.0 admission traceability table | S2b deletes the 0.x wire, verifier, `patch`, producers and `cem` CLI routes; S3 the protocol and interop 0.x packets |
| CEM-V1-215..219 | Deleted 0.x wire, verifier and producers in `internal/cem/wire`, `internal/cem/verify`, `internal/cem/workflow` and `internal/cem/cli`; retired `frontier/0`, `tcq/0`, `lrf/0` and `external-frontier-obligations/0` routes; `internal/attest/cem.go` retired-predicate refusal; regenerated `cmd/corvint/testdata/core-freeze` and `conformance/cli-parity-v0` declarations; tests listed under "Core 0.x removal traceability"; evidence `docs/build-log/2026-10-12-v1-1278-s2b-core-removal.md` | The `protocol/`, `interop/`, `examples/cem` and `conformance/cem-0.1` 0.x packets and their consumers remain for S3; the lrf-v0 corpus and frontier-v0 tier are retired whole, not ported |
| CEM-V1-220..225 | Deleted `protocol/cem-0.2`, `protocol/cem-0.3`, `interop/cem-0.1`, `conformance/cem-0.1`, `conformance/cem-0.2`, `examples/cem` (except the 0.x-free `experiments/` companion demo), `docs/CEM-CI.md`, `tools/cem-interop-runner` and `script/cem-recipes_test.sh`; `protocol/cem-1.0/stable/inherited/cem-0.3-algorithms.md`; `interop/cem01-go/testdata/inherited-vectors/`; the retired route in `interop/cem01-go/main.go`; `stableStatusArguments` in `internal/dogfoodflow/change.go`; tests listed under "Packet retirement traceability"; evidence `docs/build-log/2026-10-13-v1-1278-s3-retire-packets.md` | The 0.x packets stay readable in Git history only; the portable consumer verifies `cem/1.0` only; the retired specs keep their bodies and requirement IDs as history |
| CEM-V1-170 | `internal/cem/workflow/stable_read.go` `StableCitationClass`, `stableRecordPaths`; `TestCEMV1170StatusCitationClassRecord` in `internal/cem/cli` | Committed record maps, which carry no role, are classified from their hunk paths retroactively; the portable consumer has no status envelope and is unchanged |
| CEM-V1-106 | `internal/cem/workflow/stable_read.go` `StableCitationClass` and the `status` branch of `readStable`; `TestCEMV1106StatusCitationClassGroupsChoreMaps` in `internal/cem/cli` | No producer writes `role` before the V1-1276 cutover, so every committed map reads class `change`; the portable consumer has no status envelope and is unchanged |
| CEM-V1-065 | `CEMStatementV2` and the v2 verifier in `internal/attest/cem_v2.go`; `prove --attest-cem-v2` and `--verify-cem-attestation --artifacts/--target` in `cmd/corvint/prove_attest_cem.go`; stable routing in `internal/gitnotes/anchor.go` (`wire.ParseStable`; the `wire.ParseRouted` 0.x router in `internal/cem/wire/route.go` was deleted by V1-1278 S2b, `CEM-V1-215`); `TestCEMV1065*` in `internal/attest`, `cmd/corvint` and `internal/gitnotes` | The verdict ledger is `CEM-V1-271..279` (V1-1292), kept beside the anchor pointer and outside `cem/v2`; no signing producer or key management; v2 emission needs full-OID `--expected-base` and `--target` and an ACCEPT result; no independent consumer of `cem/v2` |
| CEM-V1-067 | `corvint-cem-anchor/1` pointers, `describes` and `tree` in `internal/gitnotes/anchor.go`, the scope states and `ProvenanceWithNotesTip` in `internal/gitnotes/provenance.go`, `--notes-tip` in `internal/cem/cli/cli.go` and `cmd/corvint/cem_anchor.go`; tests under "Anchor pointer scope traceability" | No migration of existing `/0` notes; rollback detection needs an operator-retained tip; no independent consumer of the pointer profile |
| CEM-V1-115..117, 176..177 | `Refuse(..., true)` for both argument stages and `RelativePathFix` in `internal/cem/verify/stable.go`, the stderr line in `internal/cem/cli/stable.go`, the envelope `fix` in `internal/cem/workflow/stable_read.go`; the per-link uniqueness pass in `internal/cem/wire/stable.go`; `validateStableReferences` and `stableRelativePathFix` in `interop/cem01-go/cem.go`; public case `protocol/cem-1.0/stable/reference-precedence/`; `TestCEMV1115*`, `TestCEMV1117*`, `TestCEMV1177*` in `internal/cem/cli`, `internal/cem/verify` and `interop/cem01-go` | V1-1295 (partial drift on a resource-limit result) is resolved by `CEM-V1-240`; V1-1297 reports a missing pinned object as `repository-object-unavailable` (decision 0535, superseding the decision 0532 hold; see `CEM-V1-103`); technical contract acceptance and promotion |

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
`NOT_OBSERVED`. The stable assembly result contains the actual closed stable
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
with Core and portable results recursively across all closed fields. Historical
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
observations and the initial failed qualification harness runs. On 2026-10-16
(V1-0650) cases 21 and 22 pass again on current source under the official Go 1.24.13
toolchain on Darwin arm64, and the hosted ubuntu-24.04 amd64 `go-interop` job passes the
portable package at package level (`docs/build-log/2026-10-16-s0e-go1.24.13-and-linux-lifecycle.md`).
Per-test Linux amd64 results stay NOT_OBSERVED; cross-device topology and escaped-session
behaviour remain NOT_RUN; all three are excluded from 1.0; external consumers, default promotion and release readiness are
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
  frozen packet is never rewritten except by an additive member regeneration or a recorded value
  re-emission of an existing member (`CEM-V1-091`). Tests run the S0E cases, linked control, ledgers and envelope
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
  An existing member MAY instead receive a recorded value re-emission, only through
  `make cem-stable-regen-replace` (`--replace`), when an accepted amendment changes that member's
  values: the values come from the capture or override files as above (an override object names
  only the re-emitted members, and a member it omits keeps its value; a cell no case matches takes
  a `--cell-values` entry, which may be empty), every other key, exit, case, the schema and the key
  count stay unchanged, and a `--record` file gives the reason and, for every case and cell value
  that changes, the SHA-256 of the compact JSON of the value it replaces. A changed value the record
  does not name, a recorded value that does not change or a predecessor digest that differs MUST be
  refused before any write. The re-emission re-binds the same chain, sets the packet status to
  `MEMBER_VALUE_REEMITTED` and appends a `CHANGES.md` section with the reason, the record digest,
  the value provenance and a predecessor and new digest per value. (Amended by V1-1331 under
  decision 0530, owner approval of 2026-10-15: the frozen packet was additive-only, so no accepted
  correction to an existing member's values could reach it.) A re-emission MAY change only cell
  values: when no case value changes, `FULL-RESULT-CASES.json` and its `predecessorSHA256` stay
  byte-identical and the run is refused only when no case or cell value changes. A cell whose
  matching cases disagree on a re-emitted member is not derivable and MUST take a `--cell-values`
  entry; without one the re-emission is refused. (Amended by V1-1297 under decision 0535, owner
  delegated to the orchestrator 2026-10-16: re-emitting `code` of the missing-object cells needs
  both, because their other keys equal cases with other codes and no case value changes.)
  Rollback is a Git revert of the regenerated files and the consumer change.

## Consumer-computed support tiers (V1-1262, decision 0523 P0)

Status: proposed. Decision 0523 P0. The verifiers derive, from the base blob, target blob,
canonical patch and cited spans alone, how strongly each hunk is supported. No map field is added
or changed. Nothing here is a verdict, threshold or published cut-off.

- `CEM-V1-080`: Support tiers are consumer-derived. The native (`internal/cem/verify`, `support.go`)
  and portable (`interop/cem01-go`, `citation.go`) verifiers MUST assign each hunk of a verified
  `cem/1.0` map exactly one tier, by precedence `held`, `mechanical-proved`, `anchored`, `cited`,
  `unknown`, plus an `annotation` flag. A supported hunk is one with at least one basis;
  `mechanical-proved` is the `mechanical` disposition after re-proof; `unknown` is a non-mechanical
  hunk with no basis. `held` is reserved and never produced until the go-test hunk-revert falsifier
  lands. No producer-written field can raise a tier. The tier MUST NOT change the verdict, the
  codes or drift. A retired `cem/0.1`, `cem/0.2` or `cem/0.3` map is refused `unsupported-spec`
  (`CEM-V1-210`) and never carries a tier. (Amended by V1-1332 under decision 0530: the text named
  `cem/0.2` and `cem/0.3` maps after the `cem/1.0` cutover.)
- `CEM-V1-081`: Anchor kinds and byte-level rules. `id`: an ASCII requirement ID
  (`[A-Z][A-Z0-9-]{2,31}-[0-9]{3}`, token boundaries where the adjacent bytes are not in
  `[A-Za-z0-9_-]`) occurs in the hunk's added bytes and the cited span contains a defining line
  (optional leading spaces or tabs, then ``- `ID`:``). The hunk path is the post-image path of the
  hunk's canonical patch file section, or its pre-image (base) path when the section has none (a
  whole-file deletion); every rule below that names a hunk's path uses it. `path`: the span
  contains the hunk path or its basename when the basename is at least 6 bytes. `annotation`: the
  hunk has added lines and, after every boundary-valid requirement ID in the added bytes is masked
  to spaces, every remaining token (maximal `[A-Za-z0-9_]` run) is shorter than 3 bytes or is, after
  ASCII lowercasing, one of the 39 annotation stop terms listed in
  `protocol/cem-1.0/stable/ALGORITHMS.md`: the `lexical-relevance-floor-v0` stop list without the
  21 terms that are a keyword, reserved word, predeclared identifier or builtin in a common
  programming language (`error`, `false`, `from`, `include`, `into`, `package`, `print`, `return`,
  `self`, `source`, `string`, `test`, `then`, `this`, `true`, `unknown`, `value`, `when`, `where`,
  `while`, `with`). A hunk adding only `return value - 1` is not an annotation. (Amended by V1-1335
  and V1-1332 under decision 0530: annotation used the full `lexical-relevance-floor-v0` list, and the
  hunk path was stated only for the `path` anchor.) A hunk's anchors are the union over
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
  counts basis entries whose evidence path equals the hunk path of `CEM-V1-081`, so a deletion hunk
  compares against its base path (the same evidence under two relations counts twice).
  `concentratedHunks` is 0 when `maxFanOut` is below 2, and otherwise counts supported hunks whose
  every basis cites an evidence ID at `maxFanOut`: concentration needs an evidence item two hunks
  share. (Amended by V1-1332 and V1-1331 under decision 0530: `samePathCitations` named the
  post-image path, which a pure deletion lacks, and `concentratedHunks` equalled the supported-hunk
  count whenever no evidence was shared.) The shift null sorts supported hunks by `hunkId`, gives hunk
  i the basis set of hunk i-1 (hunk 0 takes the last) and recomputes `anchored`. `invariant` is true
  when all supported hunks' evidence-ID sets are equal (true below two supported hunks).
  `specificity` is `anchored` minus `anchored` after the shift (0 below two supported hunks).

  Placement in this slice: the native 0.x canonical verification object carries key `citation` on
  success and on drift rejection only when `--citation` is given to `verify` or `status`; default
  output stays byte-identical to the frozen 0.x goldens (decision 0357) and to the base verifier,
  which `dogfood-check` requires. The stable (`cem/1.0`) result carries it as the member
  `citation` after `artifactChecks` (CEM-V1-092), added to the sealed `cem-stable-verification/1`
  shape (`protocol/cem-1.0/stable/RESULT.schema.json`, the S0E repository-envelope packet and the
  go1.27.2 structural tuples) by `tools/cem-stable-regen` (CEM-V1-091). The portable consumer
  computes the block as a library result pinned by `testdata/citation/`, emits it in its stable
  result, and emits nothing on the 0.x wire because the frozen `protocol/cem-0.3` vectors pin its
  0.x result. Amended by V1-1278 S3 (`CEM-V1-220`, `CEM-V1-222`): those vectors and the portable
  0.x route are deleted, so the portable consumer has no 0.x result.
- `CEM-V1-092`: The `cem-stable-verification/1` member `citation` MUST be the `CEM-V1-082` block of
  the verified map, computed from the canonical patch and the bound span bytes, and MUST be non-null
  only when `outcome` is `ACCEPT`. On every other outcome (`REJECT`, `UNSUPPORTED`, a cancellation
  or cleanup override after a provisional ACCEPT, and an evidence-drift refusal or rejection) it
  MUST be `null`. Both consumers MUST follow this rule: the native verifier clears the member in
  every refusal and rejection path and sets it only at ACCEPT; the portable consumer clears it in
  its single failure path and sets it only at its ACCEPT points. A native/portable disagreement on
  the member is a consumer defect.

Non-goals: no pass/fail threshold, no published cut-off, no `symbol` kind, no change to map wire
fields, no producer-side tier, no stable `citation` on any outcome other than ACCEPT.

Failure modes: the block is derived only from span bytes the canonical verification already read
and bound, so a span that cannot be read fails verification before any tier exists; a consumer never
emits a partial or estimated block, and an absent block is not a verdict either way. A stable
result that carries `citation` on a non-ACCEPT outcome, or `null` on ACCEPT, violates CEM-V1-092
and fails the sealed full-result comparison.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-080 | `TestCEMV1080RubberStampAndCarefulMapsDiffer`, `TestCEMV1080CitationIsCanonicalOnly`, `TestCEMStatusCitationIsOptIn` (native); `TestCEMV1080RubberStampAndCarefulBothAccept` (portable) | A rubber-stamp map (every hunk cites one `AGENTS.md` span) and a careful map of the same 24-hunk patch both ACCEPT and their `citation` integers differ; tiers never change `valid`, `issues` or `drift`; `cem/0.1` carries no block |
| CEM-V1-081 | `TestCEMV1081IDBoundaryAndAnnotation`, `TestCEMV1081IDAnchorRequiresBoundedTokenAndDefinition`, `TestCEMV1081PathAnchorUsesPostImageAndBasenameFloor`, `TestCEMV1081Annotation` (native); `TestCEMV1081IDBoundaryAndAnnotation`, `TestCEMV1081RequirementIDGrammar` (portable) | A hunk adding ``x // `CEM-SM-007` `` against a span that mentions `CEM-SM-0070` and defines nothing is `cited`, not `anchored`; a hunk adding only that comment is `annotation: true`; the post-image path and the 6-byte basename floor decide `path` |
| CEM-V1-082 | `TestCEMV1082DisjointUmbrellaSpansHaveNoSpecificity`, `TestCEMV1082CitationIsDeterministic`, `TestCEMV1082BlockShapeAndCounts`, `TestCEMV1082ShiftNull`, `TestCEMV1082HunkOrderDoesNotChangeTheBlock` (native); `TestCEMV1082DisjointUmbrellaSpansHaveZeroSpecificity`, `TestCEMV1082CitationIsOrderIndependent`, `TestCEMV1082CountingRules`, `TestCEMV1082StableResultCarriesTheBlock` (portable); `interop/cem01-go/testdata/citation/*/citation.json` byte-matched against the native binary | `AGENTS.md` split into 20 disjoint spans, one per hunk, reports `specificity: 0`; native and portable blocks are byte-identical on four fixtures; the accepted stable result carries the same block as `citation` |
| CEM-V1-092 | `TestCEMV1092CitationOnlyOnAccept`, `TestCEMV1092SealedCasesCarryCitationOnlyOnAccept`, `TestCEMV1092RefuseAndRejectClearCitation` (native); `TestCEMV1092PortableCitationOnlyOnAccept`, `TestStableCanonicalFullResults` (portable); `TestStableS0EPublicCases` | The 56 sealed cases carry `citation` non-null on exactly their three ACCEPT cases and null on every refusal, including both drift-stage cases; both public no-Tasks cases and every public mutation of them obey the rule; a portable REJECT, evidence-drift REJECT and argument refusal carry `null` |

Rollback: remove `internal/cem/verify/support.go`, `interop/cem01-go/citation.go` with
`testdata/citation/`, and the `citation` key in `successVerification`; remove the `--citation`
flag and the `Citation` read option. No map, packet or protocol vector changes. CEM-V1-092 rolls
back by a Git revert of its regeneration run (the packet, cells, schema, manifests, tuple and the
two public case expectations) together with the consumer change.

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
| CEM-V1-083 | `TestCEMV1083NeutralCriterionPublicCase`, `TestCEMV1083TasksFormUnchanged` (`internal/cem/verify`); `TestCEMV1083PlainCriterionWire`, `TestCEMV1083PlainCriterionRefusals` (`internal/cem/wire`); `TestCEMV1083NeutralCriterionPublicCase`, `TestCEMV1083PortablePlainDuplicateEvidence`, `TestCEMV1083PortableTasksFormUnchanged` (`interop/cem01-go`) | Public case `protocol/cem-1.0/stable/neutral-criterion/` ACCEPTs with the same non-runtime result on both consumers on a repository with no `.taskman/`; seven public refusals match on both; every S0E Tasks criterion re-encodes to its original eleven keys; walkthrough in `docs/build-log/2026-10-12-cem-neutral-criterion-walkthrough.md` |

Rollback: revert the plain branch in `internal/cem/wire/stable.go` and `interop/cem01-go/cem.go`
(restore the eleven required keys), the `criterionBinding` `oneOf` in `stable.schema.json`, the
CEM-V1-083 text in `ALGORITHMS.md`, the `neutral-criterion/` case and its manifest entries. Tasks
maps need no migration.

## Provider-neutral producer (V1-1290, decision 0523 P1)

Status: proposed technical contract; `CEM-V1-240` accepted under the owner's standing auto-accept
(2026-10-11); V1-1290. An experimental companion request, not promoted. It adds a producer for the
plain criterion form of `CEM-V1-083` and leaves that requirement's text unchanged.

- `CEM-V1-240`: `corvint-cem-candidate assemble-stable --experimental --request R --out-dir D`
  MUST route a request whose `profile` is `cem-stable-plain-assembly/1` to a closed decoder of
  `{profile, repository, expectedBase, target, sourceMap, runnerPlan, runnerReceipt, criteria}`.
  Each pinned input is `{path, sha256}` with an absolute clean path and the raw SHA-256. Each of 1
  to 256 criteria is `{criterionEvidenceId, hunkIds, evidenceIds}`, with 1 to 32 IDs in each list.
  The request names no ticket, attempt, capture, snapshot or source prefix, and the producer reads
  no Corvint Tasks store. Every other profile keeps the Tasks-bound `cem-stable-assembly/1` path.
  The source map MUST be a `cem/1.0` producer map, as written by `corvint cem prepare` and
  `corvint cem cite`. It has no criterion binding, runner receipt, criterion link, artifact or
  provenance, and its change members and `role` are copied unchanged. Request criterion `i` becomes
  the plain criterion of `CEM-V1-083` with `criterionIndex` `i`. Its `evidenceId` is
  `criterionEvidenceId`, which MUST name an evidence record of the source map. Its
  `criterionSha256` is that record's `spanSha256`. It gets one criterion link
  `{criterionId, hunkIds, evidenceIds, runnerReceiptSha256s}` naming the one receipt.
  The map carries one runner receipt record with `profile` `corvint-test-runner-receipt/0`, the
  receipt and plan digests, `executionAuthority` `CALLER_OBSERVED`, and `sourceGitBinding`,
  `dependencyClosure` and `authentication` `NOT_OBSERVED`. It also carries the artifacts
  `runner-plan` and `runner-receipt`, at `artifacts/runner-plan.json` and
  `artifacts/runner-receipt.json`. The bundle D MUST be a new absolute directory outside the
  repository whose parent has no symlink alias. The producer writes the two artifacts into D and
  verifies the encoded map with the native stable verifier against D. It writes `D/stable.json`
  only after an ACCEPT, and only if every pinned input still matches its digest.
  From the recipe repository of `protocol/cem-1.0/stable/neutral-criterion/`, after `cem prepare`,
  one `cem cite` per hunk, and with that case's two runner artifacts, it MUST re-produce the case's
  `stable.json` and artifacts byte for byte.

Non-goals:
- no new or changed `corvint cem` verb, CLI frozen surface or MCP tool
- no change to the wire, the verifier, `CEM-V1-083` or the Tasks-bound request
- no runner execution: the plan and receipt are opaque caller inputs, checked only by SHA-256 as
  the verifier checks them, unless the request opts into the receipt admission of `CEM-V1-280`
- no receipt signing (`FPK-V0-036`)
- no more than one runner receipt per map

The result `cem-stable-plain-assembly-result/1` reports `receiptAuthority` and `executionAtCommit`
as `NOT_OBSERVED`. An ACCEPT says nothing more than `CEM-V1-083` and the stable verifier already
say; decision 0523 §6 lists what it does not establish.

Failure modes:
- request shape, profile or pin form: refused before any read
- changed pinned input: refused, before the build and again before the map is written
- a source that is not a producer map: refused
- an unknown criterion evidence ID: refused
- duplicate criterion evidence: refused by the stable encoder (`duplicate-reference`)
- an unresolved hunk or evidence ID: refused by the stable encoder (`unresolved-reference`)
- a bundle inside the repository or already present: refused
- any verifier REFUSE or REJECT: refused, naming its stage and code

No refusal writes `stable.json`. A refusal after the bundle was created leaves at most the two
runner artifacts in D.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-240 | `TestCEMV1240PlainAssemblyReproducesNeutralCriterionCase`, `TestCEMV1240PlainAssemblyRefusals`, `TestCEMV1240PlainAssemblyRechecksPinsAfterVerify` (`internal/cemcandidate`); `TestCEMV1240PlainProfileSelectsPlainDecoder` (`cmd/corvint-cem-candidate`) | The produced map, plan and receipt equal the public `neutral-criterion` case byte for byte, on a recipe repository with no `.taskman/`. The native verifier ACCEPTs it inside the producer. Each pinned input changed after that ACCEPT is refused with no `stable.json`. The public case's `TestCEMV1083NeutralCriterionPublicCase` ACCEPTs the same bytes on both consumers. The timed producer and both-consumer run is in `docs/build-log/2026-10-15-cem-plain-criterion-producer.md`. |

Rollback: remove `internal/cemcandidate/plain.go`, the profile dispatch in
`cmd/corvint-cem-candidate/stable.go`, and their tests. Plain maps already produced stay valid
`cem/1.0` maps under `CEM-V1-083`.

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

Decision (V1-1291, 2026-10-14): that `invalid-arguments` refusal, like every `arguments` and
`authority-arguments` refusal, is `UNSUPPORTED` with `accept` `null` and exit 2 in both consumers
(CEM-V1-115, CEM-V1-116). A relative `--trusted-keys` path also prints the absolute-path fix on
stderr (CEM-V1-117).

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-084 | `TestCEMV1084ReceiptAuthenticationFixtures`, `TestCEMV1084ReceiptAuthenticationPublicCase` (`internal/cem/verify`); `TestCEMV1084AuthenticationForms`, `TestCEMV1084AuthenticationRefusals`, `TestCEMV1084ReceiptAttestationEnvelope`, `TestCEMV1084TrustedKeysFile` (`internal/cem/wire`); `TestCEMV1084PortableReceiptAuthenticationPublicCase`, `TestCEMV1084PortableTrustedKeysArgument` (`interop/cem01-go`); `TestCEMV1084VerifyStableTrustedKeysFlag`, `TestCEMV1084HelpNamesTrustedKeys` (`cmd/corvint`); `TestCEMV1084ConsoleReceiptAuthentication` (`internal/console`) | Public case `protocol/cem-1.0/stable/receipt-authentication/` ACCEPTs VERIFIED with the same non-runtime result on both consumers; five axis cases and thirteen refusals match on the native verifier and both portable commands; a self-carried key and a signature over the base tree are refused; walkthrough in `docs/build-log/2026-10-12-cem-receipt-authentication.md` |

Rollback: revert `stable_auth.go` in `internal/cem/wire`, `internal/cem/verify` and
`interop/cem01-go`, the `--trusted-keys` flag, the `authentication` `oneOf` and the
`receipt-attestation` kind in `stable.schema.json`, the CEM-V1-084 text in `ALGORITHMS.md`, the
`receipt-authentication/` case and its manifest entries. Maps with the legacy string need no
migration; a map with an object form is refused again after rollback.

## Verdict digest (V1-1265, decision 0523 P1)

Status: proposed. Decision 0523 P1. A stable result separates the verdict from the environment that
produced it, so a later consumer can bind the verdict alone.

- `CEM-V1-066`: Every `cem-stable-verification/1` result MUST carry `verdictSha256` after
  `repositoryEnvelope`: the lowercase hex SHA-256 of the canonical JSON value (sorted keys, minimal
  separators, decimal integers, no terminal LF) of the result object without `runtime`,
  `repositoryEnvelope` and `verdictSha256`. It MUST be computed on every outcome, ACCEPT, REJECT and
  UNSUPPORTED, and is never null. The excluded members are environment: re-verification of the same
  inputs on another admitted Go tuple, or under another repository-envelope label, MUST yield the
  same digest, while a change to any other member MUST change it. The native and portable
  consumers MUST emit the same digest for every public case (S0E cases, envelope cells,
  `neutral-criterion` and `receipt-authentication`); a disagreement is a consumer defect, never a
  reason to edit the sealed value. Later consumers (decision 0525 step 8 and the Tasks gate,
  V1-1283) consume `verdictSha256` rather than hashing the whole result.

Non-goals: no signature or attestation over the digest, no change to the verdict members themselves,
no digest in `cem/0.x` results, and no consumer of the digest in this change.

Failure modes: an emitted digest that differs from the recomputed digest of the emitted result, a
digest that moves with `runtime` or `repositoryEnvelope`, or a non-integer number in the result
(refused rather than rounded) is a consumer defect; a value held in the result struct is never
emitted, because both consumers recompute the digest at encoding.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-066 | `TestCEMV1066SealedVerdictDigests`, `TestCEMV1066VerdictDigestCoversOnlyTheVerdict`, `TestCEMV1066EveryOutcomeCarriesVerdictDigest` (`internal/cem/verify`); `TestCEMV1066PortableSealedVerdictDigests`, `TestCEMV1066PortableVerdictDigestCoversOnlyTheVerdict`, `TestCEMV1066PortableEveryOutcomeCarriesVerdictDigest`, `TestStableCanonicalFullResults` (`interop/cem01-go`) | The native capture of 54 S0E cases equals an independent recomputation from the sealed expectations; every sealed case, cell and public ACCEPT digest recomputes by hand on both consumers; each verdict member moves the digest and the three excluded members do not; the public refusals, axis cases and a REJECT, drift and UNSUPPORTED fixture each carry their own digest |

Rollback: revert the `verdictSha256` regeneration (the packet files, both schema copies, the
cells, the tuple packet and their manifests), the public `EXPECTED.json` members and stable
manifest entries, `stable_verdict.go`, `interop/cem01-go/verdict.go` and the key-count changes in
both consumers. No map changes; the Tasks verdict gate reads the digest (TCB-V0-012, TCB-V0-017).

## Stable map provenance, producer-only members and role (V1-1265, decision 0525)

Status: proposed. Decision 0525 §3 step 3 and §4. A converted map can say where it came from and
why its change exists, and a producer without task or runner evidence has a closed list of members
it emits empty.

- `CEM-V1-093`: A stable map MAY carry `provenance`, a closed object with exactly the required
  members `sourceSpec` (`cem/0.1`, `cem/0.2` or `cem/0.3`), `sourceBlobOid` (40 lowercase hex),
  `sourceSha256` (64 lowercase hex), `bindCommit` (40 lowercase hex) and `converter` (matching
  `^corvint-cem-convert/[0-9]+(\.[0-9]+)*$`). A native map omits it. A non-object value is
  `invalid-field`; inside the object a missing member is `missing-field`, then an unknown member
  `unknown-field`, then an invalid value `invalid-field`, in that member order. Root optional
  members are validated after every required member and before the reference graph, `provenance`
  before `role`. Provenance MUST NOT be read for authority, ranking, any result axis or verdict, and
  the result MUST NOT echo it; it affects only the map bytes and so `mapSha256` and, through it, `verdictSha256`. The native and
  portable consumers MUST refuse and accept the same documents with the same codes.
- `CEM-V1-094`: The producer-only stable map members are exactly this closed list:
  1. `criterionBindings`;
  2. `runnerReceipts` (including a receipt's `authentication`);
  3. `criterionLinks`;
  4. `artifacts` (including `receipt-attestation` records).

  A producer with no task or runner evidence, including the 0.x converter, MUST emit each as an
  empty array, and both consumers MUST ACCEPT such a map with `referenceIntegrity`
  `EMPTY_REFERENCE_SET` and every non-core axis `NOT_OBSERVED`. There is no "intent and scope"
  member and no "structural witness" member: intent stays in project specs and tickets, and the
  structural reasons are recomputed from Git. Hunk `coverage` and `discriminates` are not
  producer-only; they are optional source witnesses carried unchanged when present, omitted when
  absent and never synthesized.
- `CEM-V1-095`: A stable map MAY carry `role`, a closed string enum of `change` (the meaning when
  absent), `bind-chore` and `seal-chore`. Any other value is `invalid-field`. Role MUST NOT change a
  verdict, tier or result axis, and the result MUST NOT echo it.

Re-homing: `protocol/cem-1.0/stable/ALGORITHMS.md` now restates, in cem/1.0 vocabulary, the 0.1
algorithms (strict JSON, paths, canonical identity JSON, evidence and hunk IDs, the patch profile
and parser, base simulation, evidence and disposition checks, text mechanical reasons, limits and
drift) and the 0.3 hunk vocabulary, structural reasons and witnesses, citing CEM-SM-001..010 in
`docs/specs/cem-0.3-structural-mechanical.md` as references. It names the 13 `$defs` the stable
schema restates from `inherited/cem-0.3.schema.json`. The `inherited/` copies are retained unchanged
as historical provenance; `ALGORITHMS.md` is the 1.0-normative text and governs where they differ.
The 0.1/0.2/0.3 requirement IDs keep their definitions in their own specs.

Non-goals: no converter here (V1-1275 adds it as CEM-V1-096..105 below); reader grouping by role is `CEM-V1-106` (`cem status --citation`,
V1-1281); no "intent and scope" or "structural witness" member; no change to `RESULT.schema.json`,
the stable result struct, the sealed S0E packet or any `inherited/` file; no SHA-256 repository
form of the provenance OIDs.

Failure modes: a malformed `provenance` or `role` is refused at the wire stage before any reference
or Git check, so a map cannot reach a verdict through an ill-formed optional member; a provenance
that names a source the repository does not hold is not checked, because provenance is never an
input to authority; an all-empty map is accepted but reports `EMPTY_REFERENCE_SET`, never task or
runner evidence it does not carry.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-093 | `TestCEMV1093StableMemberVectors`, `TestCEMV1093ProvenanceDecoded`, `TestCEMV1093SchemaMembers` (`internal/cem/wire`); `TestCEMV1093PortableStableMemberVectors` (`interop/cem01-go`) | 50 shared vectors in `interop/cem01-go/testdata/stable-map-members/cases.json` (10 accepts, 40 refusals including member and stage ordering) give the same code on both consumers; accepted maps round-trip losslessly through `EncodeStable`; schema members match the Go constants; build log `docs/build-log/2026-10-12-cem-stable-map-provenance.md` |
| CEM-V1-094 | `TestCEMV1094ProducerOnlyEmpty` (`internal/cem/wire`); `TestCEMV1094ProducerOnlyEmptyVerifyAccept` (`internal/cem/wire`, through `internal/cem/verify`); `TestCEMV1094PortableProducerOnlyEmptyAccept` (`interop/cem01-go`) | An all-empty map over the neutral-criterion repository ACCEPTs on both consumers with `EMPTY_REFERENCE_SET`, change integrity VERIFIED and every non-core axis NOT_OBSERVED; adding `provenance` and `role` changes only `mapSha256` and, through it, `verdictSha256` |
| CEM-V1-095 | `TestCEMV1095Role` (`internal/cem/wire`); `TestCEMV1095PortableRoleAndProvenanceDecoded` (`interop/cem01-go`) | Each role decodes and re-encodes unchanged on both consumers and an absent role is effective `change`; six role refusals are `invalid-field` on both consumers |

Rollback: revert `stable_provenance.go` in `internal/cem/wire` and `interop/cem01-go`, the root
key change in both decoders and `EncodeStable`, the `provenance`, `role` and `sha1Oid` definitions
in `stable.schema.json`, the CEM-V1-093..095 and re-homed sections of `ALGORITHMS.md`, the README
bullets and their manifest entries. A map without the optional members needs no migration; a map
carrying either is refused `unknown-field` again after rollback.

## cem/0.x -> cem/1.0 converter (V1-1275, decision 0525 §4)

Status: proposed; satisfied by the frozen V1-1277 evidence and removed by V1-1278 (`CEM-V1-204`),
so the converter no longer ships and the tests named below are retired with it; their runs are
retained in `docs/build-log/evidence/v1-1275/` and `docs/build-log/evidence/v1-1277/`. Decision 0525
§4 "Converter contract" and §5 "Sealed-map rule". The converter
turns one committed cem/0.1, cem/0.2 or cem/0.3 map into the cem/1.0 map CEM-V1-093..095 shape,
as a pure function of the source bytes and the bind commit, so that every committed map can move
to cem/1.0 (V1-1276/V1-1277) without a verdict changing. It lives in `internal/cem/convert` and
behind `corvint cem convert`.

- `CEM-V1-096`: (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) The converter input is the exact bytes of one map. A map that declares
  `cem/1.0` is refused `convert-source-is-stable` before any other check. Otherwise the bytes are
  read by the existing 0.x reader (`wire.ParseMap`), which accepts `cem/0.1`, `cem/0.2` and
  `cem/0.3`; a wire refusal keeps that reader's own code (for example `invalid-json`,
  `unknown-field`, `orphan-evidence`). A source with zero hunks is refused `convert-empty-hunks`,
  because a stable map cannot carry an empty `hunks` array.
- `CEM-V1-097`: (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) The output is exactly the `wire.EncodeStable` bytes of this stable map, which
  `ParseStable` accepts:
  1. `spec` is `cem/1.0`;
  2. `baseRevision`, `patchSha256`, `evidence` and `hunks` are the source values, carried member
     for member. Hunk `disposition`, `reason`, `basis`, ranges and IDs are unchanged, and hunk
     `coverage` and `discriminates` (cem/0.3) are carried unchanged when present and omitted when
     absent, never synthesized;
  3. `excludedPath` is the protocol literal `.corvint/change.cem.json` (a cem/0.1 source has no
     such member; 0.2 and 0.3 sources already carry the same literal);
  4. the CEM-V1-094 producer-only members `criterionBindings`, `runnerReceipts`, `criterionLinks`
     and `artifacts` are empty arrays;
  5. `provenance` is the CEM-V1-098 object;
  6. `role` is omitted (effective `change`); grouping by role is V1-1281.

  No tier, axis, reason or witness is computed: tiers stay consumer-computed. Nothing in the
  output is read from Git.
- `CEM-V1-098`: (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) `provenance.sourceSpec` is the source `spec`; `sourceBlobOid` is the Git SHA-1
  blob object name of the exact source bytes; `sourceSha256` is the lowercase hex SHA-256 of the
  same bytes; `bindCommit` is the commit the source file name pins (`<40 lowercase
  hex>.cem.json`), or the explicit `--bind-commit` value, which takes precedence; `converter` is
  `corvint-cem-convert/1`. A missing or malformed bind commit is refused
  `convert-bind-commit-unavailable`; the converter never guesses one.
- `CEM-V1-099`: (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) When a repository is given, the converter reads one Git object path: the blob the
  bind commit holds at `.corvint/change.cem.json`. The report states the source binding as `EXACT`
  (a regular file whose blob OID equals `sourceBlobOid`), `ABSENT` (the commit resolves but holds
  no regular file there), `UNAVAILABLE` (the commit or its trees cannot be read), `MISMATCH` (a
  different blob) or `NOT_CHECKED` (no repository). Only `MISMATCH` refuses
  (`convert-source-binding-mismatch`); the output bytes are the same for every other state.
- `CEM-V1-100`: (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) Conversion is deterministic and idempotent at the byte level: converting the same
  bytes with the same bind commit twice gives identical output, on any host and with or without a
  repository. Converting the output again is refused `convert-source-is-stable`, never rewritten.
- `CEM-V1-101`: (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) The converter's own refusal codes are exactly `convert-source-is-stable`,
  `convert-empty-hunks`, `convert-bind-commit-unavailable`, `convert-source-binding-mismatch` and
  `convert-encode-refused` (the encoder refused the assembled map; not reachable from an accepted
  source). A refusal emits no map bytes and no provenance.
- `CEM-V1-102`: (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) `corvint cem convert --map MAP (--output MAP | --in-place) [--bind-commit REV]
  [--repository PATH]` takes absolute clean paths, converts one map and prints one closed
  `corvint-cem-convert-report/1` JSON object: `profile`, `status` (`CONVERTED` or `REFUSED`),
  `code`, `converter`, `sourceSpec`, `sourceBinding`, `provenance`, `hunkCount`, `evidenceCount`
  and `outputSha256`. `--output` never replaces an existing file; `--in-place` replaces the source
  through a sibling temporary file and one rename. Exit 0 is converted, 1 is a CEM-V1-096 or
  CEM-V1-101 refusal, 2 is `invalid-arguments` (including a root option before `cem convert`),
  `map-unavailable` (the map is missing, not a regular file or over the map size limit) or
  `publish-failed` (the output could not be written).
- `CEM-V1-103`: (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) A map whose pinned objects are unavailable still converts from its bytes alone
  (binding `UNAVAILABLE`, output identical to a reachable conversion). Verifying it gives an
  explicit `UNSUPPORTED` result at the `repository` stage on both consumers, never a verdict; the
  observed code was `git-read-failed`, the same code the 0.2 verifier gives the source. Amended by
  V1-1297 (decision 0535, owner delegated to the orchestrator 2026-10-16, superseding the decision
  0532 hold): a pinned object that is absent locally MUST be `UNSUPPORTED` `repository`
  `repository-object-unavailable` on both consumers. When Git cannot resolve a pinned full object
  ID, each consumer probes `git cat-file -e <oid>`: only a probe that exits normally with status 1
  is `repository-object-unavailable`; a present but unreadable or non-commit object, a native name
  that is not a full object ID (the portable consumer resolves only wire-validated object IDs), a
  probe that exits with any other status or by a signal (128 for a repository Git cannot open or a
  name it cannot parse), and a probe that itself fails at the owner, deadline, parser or launch stay
  `git-read-failed` or the probe's own code. The portable consumer charges the probe as an
  `object-existence-probe` ledger operation. The sealed S0E cells `missing-base`,
  `shallow-missing` and `missing-promisor` were re-emitted with that code and its verdict digest
  (`CEM-V1-091`); `corrupt-base` keeps `git-read-failed`.
- `CEM-V1-104`: (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) Verdict parity is measured per committed map at the substituted target T', the
  bind commit's tree with only `.corvint/change.cem.json` replaced by the converted bytes, written
  only into a scratch clone. The 0.x native verifier runs on the source at the bind commit; the
  native stable verifier and `interop/cem01-go verify-stable-canonical` run on the converted map
  at T' with the source `baseRevision` as the expected base. Compared: outcome, stage, the reason
  or issue code (0.x repository codes mapped to their stable names), `changeIntegrity`, the
  sorted drift list and the full per-hunk array (disposition, reason, basis, ranges, IDs and
  witnesses). Excluded: `mapSha256`, `verdictSha256` and `citation`, which bind the map bytes and
  so differ by construction; the reference, criterion, runner and artifact axes, which a 0.x
  result does not have (CEM-V1-094 makes them `EMPTY_REFERENCE_SET` and `NOT_OBSERVED`). Each row
  is classed `identical`, `empty-member-explained` (only a references/artifacts stage explains the
  difference), `unavailable-objects`, `pre-existing-refusal` (both consumers repeat the source's
  `UNSUPPORTED` code, stage and hunks), `NOT_OBSERVED` (the bind commit is absent from the clone)
  or `different`. Recorded limits: direct verification at the bind commit itself is `REJECT
  excluded-artifact-mismatch` on both consumers for every map, because the bind commit's sidecar
  holds the 0.x bytes (§5 sealing replaces the sidecar, not the bind commit); and a cem/0.1 source
  carries an out-of-band patch digest that the canonical diff does not reproduce, so its
  conversion verifies as `REJECT` `patch-digest-mismatch`, the same result `verify.Exact` gives the
  source with canonical-diff bytes. cem/1.0 has no out-of-band patch member; the committed corpus
  holds no cem/0.1 map. Decision 0532 (V1-1299) keeps this recorded limit and adds no `CEM-V1-101`
  refusal code: the converter no longer ships (`CEM-V1-204`), so no producer can reach the case.
  Decision 0535 closes V1-1299 as obsolete on that basis.
- `CEM-V1-105`: (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships) The converter exists only to migrate cem/0.x maps. It is removed with the 0.x
  readers by V1-1278; after that removal `corvint cem convert` is an unknown action and no
  cem/1.0 producer depends on it. V1-1278 S1 removes the converter ahead of the 0.x readers
  (`CEM-V1-204`).

Non-goals: no producer-only evidence (criterion bindings, runner receipts, links, artifacts) is
synthesized; no `role` is assigned; no tier, structural reason or witness is recomputed into the
map; no rewrite of committed maps or bind commits (V1-1276/V1-1277 own the corpus migration and
the §5 sealing commit); no cem/1.0 to 0.x converter; no SHA-256 repository form of the
provenance OIDs; no change to either stable consumer.

Failure modes: a 1.0 or malformed source, an empty hunk list or a missing bind commit is refused
with a bounded code and no output; a sidecar that is a different blob at the bind commit refuses
instead of attributing a map to the wrong commit; an unreadable repository never changes the
output and never becomes a verdict; an existing `--output` file is never overwritten; a
consumer-only divergence on a resource-limit abort (the partial drift list) is retained as a
`nativePortableDiffer` row, not hidden by the class. V1-1295 (2026-10-14) kept that partial drift
list in the result: the sealed S0E full-result cases `ledger-337-evidence` and
`partial-drift-one-item-cancel` pin a partial list on `UNSUPPORTED` results (335 items on a
resource limit, one on a timeout), so the row stays a recorded difference
(`docs/build-log/2026-10-14-batch4-cem-parity.md`). V1-1295 (2026-10-15) traced the difference to
the native drift stage re-reading each changed record's base blob; `CEM-V1-260` removes the re-read
and both consumers now write the same 102 rows and `verdictSha256` (decision 0532).

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-096 | `TestCEMV1096ConvertAcceptsEachZeroDialect` (`internal/cem/convert`) | cem/0.1, cem/0.2 and cem/0.3 fixtures convert to their goldens under `internal/cem/convert/testdata/` |
| CEM-V1-097 | `TestCEMV1097ConvertMemberMapping` (`internal/cem/convert`) | Carried members equal the source; producer-only members empty; `role` absent; 0.3 witnesses carried, absent witnesses not synthesized |
| CEM-V1-098 | `TestCEMV1098ConvertProvenanceValues` (`internal/cem/convert`) | Each provenance member equals its derived value in the map and the report; the empty input gives Git's empty-blob OID; a file name pins only 40 lowercase hex |
| CEM-V1-099 | `TestCEMV1099ConvertSourceBinding`, `TestCEMV1099ConvertGitReaderStates` (`internal/cem/convert`) | Five binding states over a fake reader and real repositories; only MISMATCH refuses; bytes equal across states |
| CEM-V1-100 | `TestCEMV1100ConvertCorpusIdempotent`, `TestCEMV1100ConvertF0fda542` (`internal/cem/convert`) | All 603 committed maps converted twice to identical bytes and reconversion refuses `convert-source-is-stable`; the 877-hunk f0fda542 map converts. Since the V1-1277 migration the committed maps are `cem/1.0`, and the same test reconverts each migrated map's source blob to its committed bytes (`CEM-V1-230`) |
| CEM-V1-101 | `TestCEMV1101ConvertRefusalCodes` (`internal/cem/convert`) | Each reachable converter code from its trigger with no output or provenance; malformed JSON keeps the reader's `invalid-json` |
| CEM-V1-102 | `TestCEMV1102CEMConvertCommand` (`cmd/corvint`) | Golden output, exits 0/1/2, no overwrite of an existing output, `--in-place` with `--repository`, unnamed bind refused, argument and missing-map refusals |
| CEM-V1-103 | `TestCEMV1103ConvertUnavailableObjects` (`internal/cem/convert`); `TestCEMV1104PortableConvertedFixtures` (`interop/cem01-go`) | Empty repository: binding UNAVAILABLE, output equals golden, both consumers UNSUPPORTED `git-read-failed` as the 0.2 source (frozen V1-1277 evidence). V1-1297 (decision 0535): `TestResolveMissingPinnedObjectUnavailable` and `TestResolveProbeOnlyExitOneIsAbsent` (`internal/cem/gitauth`), `TestStableResolveMissingPinnedObjectUnavailable` (including a probe exit 128) and `TestStableCanonicalEnvelopeCells` (`interop/cem01-go`) on the re-emitted cells `missing-base`, `shallow-missing` and `missing-promisor`; `TestCEMV1066SealedVerdictDigests` re-derives their verdict digests |
| CEM-V1-104 | `TestCEMV1104ConvertParityCEM03`, `TestCEMV1104ConvertParityCEM01` (`internal/cem/convert`); `TestCEMV1104PortableConvertedFixtures` (`interop/cem01-go`); `TestCEMV1104ConvertParitySample`, `TestCEMV1104ConvertParityClassify` (`tools/cem-convert-parity`) | Full corpus run `docs/build-log/evidence/v1-1275/` (603 rows: 602 `identical`, 1 `pre-existing-refusal`, 0 `different`, 0 `NOT_OBSERVED`); build log `docs/build-log/2026-10-12-cem-convert.md`. Since V1-1277 the sample test runs with `--migrated` over the committed `cem/1.0` maps (`CEM-V1-230`) |
| CEM-V1-105 | `TestCEMV1204StableDispatchNoLongerCapturesConvert` (`internal/cem/cli`) | V1-1278 S1 removed `internal/cem/convert`, `tools/cem-convert-parity`, `script/cem-migrate-maps.sh` and the `cem convert` action (`CEM-V1-204`) |

Rollback: revert `internal/cem/convert`, `internal/cem/cli/convert.go`, the `convert` dispatch in
`internal/cem/cli/stable.go`, the help lines in `cmd/corvint/help.go`,
`tools/cem-convert-parity`, `interop/cem01-go/convert_fixture_test.go` and this section. No
committed map, schema, consumer or sealed packet changes, so nothing needs migration back; a map
already converted stays a valid cem/1.0 map, and its source bytes are recoverable from Git by
`provenance.sourceBlobOid`.

## Status citation class (V1-1281)

Status: accepted under the owner's standing auto-accept (2026-10-11); V1-1281. The dogfood bind loop
marks a self-referential bind or seal map with its role (`DCW-V0-049`), and a reader of the citation
measure needs to keep those maps apart from change maps without reading the role into the result.

- `CEM-V1-106`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1281) `cem status
  --citation` on a `cem/1.0` map whose wire parses MUST add the envelope member `citationClass`
  beside, never inside, `stableVerification`: an object with exactly `class` and `role`, where
  `role` is the map's effective role (`CEM-V1-095`) and `class` is `change` for role `change` and
  `chore` for `bind-chore` or `seal-chore`. Status without `--citation`, a map whose wire does not
  parse, and every other read keep their envelope unchanged. `stableVerification` stays exactly
  what `cem verify-stable` reports for the same arguments; the class is not a result member, so
  "the result MUST NOT echo it" in `CEM-V1-095` still holds, and the role changes no outcome,
  issue, tier or citation block. Amended 2026-10-13 (V1-1344): for role `change`, `class` is
  `record` when `CEM-V1-170` holds, otherwise `change`.
- `CEM-V1-170`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1344) For a map
  whose effective role is `change`, `citationClass.class` MUST be `record` when its hunk paths name
  at least one path under `docs/build-log/` or `.agent-evidence/` (nonempty remainder) and every
  other hunk path is `.corvint/change.cem.json` or `.corvint/changes/NAME.cem.json`, NAME one
  nonempty segment (`DCW-V0-057`); `role` stays `change`. A bind-chore or seal-chore map stays
  `chore` whatever its paths, and a map with no hunks is `change`.

Non-goals: no `role` or class in `RESULT.schema.json`, the stable result struct, the 0.x
verification object or the portable consumer; no class for a `cem/0.x` map, which has no role; no
`record` role value.

Failure modes: a map that fails verification still reports its class when its wire parses, so a
rejected chore map stays out of the change-map measure; a map whose wire does not parse has no
class, and a reader counts it as an error row, never as either class.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-170 | `TestCEMV1170StatusCitationClassRecord` (`internal/cem/cli`); `TestDogfoodDCWV0058RecordChangeBindsChecksAndSeals` (`cmd/corvint`) | Rewritten hunk paths under `docs/build-log/` or `.agent-evidence/`, alone or with a sidecar path, give class `record` with role `change`; a code path or only a sidecar path gives `change`; a bind-chore role gives `chore`; a dogfood-sealed record map reads `record`; build log `docs/build-log/2026-10-13-v1-1344-record-citation-class.md` |
| CEM-V1-106 | `TestCEMV1106StatusCitationClassGroupsChoreMaps` (`internal/cem/cli`) | Status without `--citation` has no `citationClass`; with it, a map with no role is class `change`, and re-encoded roles `change`, `bind-chore` and `seal-chore` give their class; each `stableVerification` equals `cem verify-stable` with no `role` key, and the outcome, issue codes and citation block match across roles; build log `docs/build-log/2026-10-12-dogfood-per-hunk-citations.md` |

Rollback: revert the `--citation` branch of the stable `status` read and `StableCitationClass` in
`internal/cem/workflow/stable_read.go`; the envelope loses `citationClass`, and no map, result or
sealed artifact changes. Roll back `CEM-V1-170` alone by reverting `stableRecordPaths`: record maps
read class `change` again.

## Committed map migration (V1-1277, decision 0525 §3 step 6)

Status: accepted under the owner's standing auto-accept (2026-10-11); V1-1277. Every committed
`.corvint/changes/<bind>.cem.json` map moves to `cem/1.0` in a migration commit, so the sealed
corpus is read by the stable consumers only and the 0.x readers can be removed (V1-1278).

- `CEM-V1-230`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1277) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships, so no follow-up migration commit can be written; a later `cem/0.x` seal follows `CEM-V1-203`, and the committed corpus is checked by `CEM-V1-201`) A migration commit MUST rewrite every committed
  `.corvint/changes/<bind>.cem.json` map that declares `cem/0.x` to the exact bytes `corvint cem
  convert --in-place` writes for it (`CEM-V1-096..102`), keeping the file name, and MUST leave
  every map that already declares `cem/1.0` byte-identical; the commit changes no other path.
  `script/cem-migrate-maps.sh` performs the rewrite. It refuses a repository that is not the top
  level of a Git work tree or that holds no map, stops on the first refusal with the converter's
  report, and prints `converted=N unchanged=N refused=N`. It converts only non-1.0 maps, so it is
  rerun after a pre-cutover branch lands further `cem/0.x` maps; those rewrites form a follow-up
  migration commit (`DCW-V0-050`). The migration check is reproducible from the committed tree:
  for each map that carries converter provenance, `TestCEMV1100ConvertCorpusIdempotent` and
  `cem-convert-parity --migrated` read the source blob named by `provenance.sourceBlobOid`,
  require `bindCommit` to equal the file name and the blob's SHA-1 and SHA-256 to equal the
  recorded digests, reconvert it, and require byte equality with the committed file. The test
  reports `NOT_RUN` (skip) when a Git object it reads is absent from the checkout's object store,
  never a pass.
- `CEM-V1-231`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1277) (V1-1278: satisfied by the frozen V1-1277 evidence; the converter no longer ships; `CEM-V1-201` keeps the native-map byte check) A committed `cem/1.0` map without converter provenance is accepted as
  natively produced only when it is byte-identical to the `.corvint/change.cem.json` blob its bind
  commit tracks, the immutable evidence that it was bound with those bytes. Otherwise
  `cem-convert-parity --migrated` fails the row `native-bytes-not-bound` and
  `TestCEMV1100ConvertCorpusIdempotent` fails, so removing the provenance of a migrated map cannot
  bypass the source-blob equality check. A native map that passes is verified at its bind commit
  on both stable consumers and must refuse reconversion.
- `CEM-V1-232`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1277) The migration is accepted only when, over every committed map, both stable
  consumers re-verify the migrated bytes with no `different` parity row (`CEM-V1-104`, at T'),
  and the per-map citation rows measured at T' after the migration equal the rows measured before
  it in every member except `spec`. The one admitted exception is bounded: a row that is invalid
  on both sides and has no citation block may differ in its `issues` member only when the
  stable issue code is the `CEM-V1-104` mapping of the 0.x code (`refusalCode`,
  `internal/cem/verify/stable.go`), with the message following the code. Any other difference
  stops the migration.

Non-goals: no re-binding of any bind commit or sidecar; no `role` on migrated maps; no change to
either consumer, the converter or the schemas; no change to `tools/cem-citation-baseline` in this
lane (its `cem/1.0` `--artifacts` root is `CEM-V1-146`/`CEM-V1-147`, V1-1281).

Failure modes: a refused conversion stops the run and leaves no migration commit; a migrated map
verified directly at its bind commit is `REJECT excluded-artifact-mismatch` (`CEM-V1-104`), so
every check runs at T', and a citation measurement that verifies at the bind commit reads all
migrated maps as invalid; a Git object missing from a shallow clone is `NOT_RUN`, never a pass;
a rebase that rewrites a migration commit invalidates the `DCW-V0-050` seal baseline, which then
fails closed.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-230 | `TestCEMV1100ConvertCorpusIdempotent` (`internal/cem/convert`); `TestCEMV1104ConvertParitySample`, `TestCEMV1230MigratedSourceChecks` (`tools/cem-convert-parity`) | Migration commit `0b6f1bc245b44a8620c5e64f91e87f6d08da08d0`: `converted=614 unchanged=1 refused=0`; a rerun over the migrated tree prints `converted=0 unchanged=615 refused=0`; an empty inventory exits 1 and a non-repository exits 2; the corpus test reconverts 614 migrated maps from their source blobs to the committed bytes, and a one-byte edit of a committed map fails it; `--migrated` run `docs/build-log/evidence/v1-1277/` (`committed`: 614 `migrated`, 1 `stable-native`; `committedEqualsConverted` 614) |
| CEM-V1-231 | `TestCEMV1230MigratedSourceChecks` (`tools/cem-convert-parity`); `TestCEMV1100ConvertCorpusIdempotent` (`internal/cem/convert`) | A stable map without provenance whose bind commit tracks other bytes fails `native-bytes-not-bound`; 860f59ea equals its bind commit's sidecar blob `495159656e689a037e73448d49f72d97a0a76e94` |
| CEM-V1-232 | Full-corpus runs (`tools/cem-convert-parity --migrated`; citation rows at T') | 615 parity rows: 613 `identical`, 1 `native-stable`, 1 `pre-existing-refusal` (f0fda542, as in V1-1275), 0 `different`, 0 `NOT_OBSERVED`. Citation rows: the pre-migration rows equal the V1-1281 reference (`docs/build-log/evidence/v1-1281/`) in every member for all 615 maps; the T' rows equal them except `spec` for 614 maps, and f0fda542 differs only by the admitted `git-budget-exceeded` to `unsupported-resource-limit` mapping. The T' rows were measured with the `--targets` tool of commit `ffc6c194b`, reverted in favour of V1-1281's tool; that tool verifies at the bind commit and reads the 614 migrated maps as `excluded-artifact-mismatch` (`citation-post-1281tool.jsonl`); build log `docs/build-log/2026-10-12-v1-1277-cem-migration.md` |

Rollback: revert the V1-1277 merge commit. The maps return to their `cem/0.2` bytes, which the
0.x readers still verify until V1-1278 removes them; after V1-1278 S1 such a revert also needs the
`CEM-V1-201` corpus check reverted; the `DCW-V0-050` migration-commit list goes with it.

## Core cem/0.x retirement (V1-1278 S1, decision 0525 §3 step 7 and §6)

Status: proposed; V1-1278 stage S1. Decision 0525 removes `cem/0.x` once the migration (step 6,
V1-1277) has passed. Every committed map is `cem/1.0` since migration commit
`0b6f1bc245b44a8620c5e64f91e87f6d08da08d0`, so this stage retires the converter, makes the
daily loop's sealed-change admission `cem/1.0`-only and pins the committed corpus with a check
that needs no converter. Decision 0357's `cem/0.2` freeze and its 0.1 N-1 reader are superseded
by decision 0525 §6. The `cem/0.x` wire (`wire.ParseMap`), the 0.x verifier, the
`--spec cem/0.2` and `cem begin` 0.x producers and the 0.x goldens stay in this stage, because
the consumers listed in `docs/build-log/2026-10-12-v1-1278-s1-remove-0x-core.md` still read or
write them; V1-1278 S2/S3 remove them after those consumers move.

- `CEM-V1-200`: `wire.ParseStable`, and so every stable read (`cem verify-stable`, a stable
  `cem status`, the corpus check), MUST refuse bytes whose `spec` is `cem/0.1`, `cem/0.2` or
  `cem/0.3` with `unsupported-spec` and the message `<spec> is retired; cem/1.0 is the only
  stable wire; fix: <RetiredSpecFix>`, where the fix line says that committed 0.x maps were
  migrated in place and names the forward recovery: merge `origin/main` into the branch, remove
  the branch's own 0.x seal from `.corvint/changes/` (`CEM-V1-203`), then rerun `dogfood-change`,
  `dogfood-check` and `dogfood-seal` with `BASE=<the origin/main commit merged>`. The fix
  MUST NOT name a rebase or a force-push. Any other non-`cem/1.0` spec keeps the existing
  "stable profile required" refusal.
- `CEM-V1-201`: Every committed `.corvint/changes/<bind>.cem.json` MUST be accepted by
  `wire.ParseStable`, and a map that carries `provenance` MUST name its own file as
  `bindCommit`. A migrated map (one with `provenance`) MUST name a retired `sourceSpec`
  (`CEM-V1-200`) and a source blob that the repository holds under `sourceBlobOid` with the
  recorded `sourceSha256`, and MUST be byte-identical to the blob one of the `DCW-V0-050`
  migration commits (`cemMigrationCommits`) wrote at its path, so an edit after migration fails.
  A native map (no `provenance`) MUST be byte-identical to the
  `.corvint/change.cem.json` blob its bind commit tracks (`CEM-V1-231`). A Git object the
  checkout lacks makes the binding test report `NOT_RUN` (skip), never a pass. This replaces the
  converter reconversion of `CEM-V1-230`; byte equality between each source and its migrated map
  is the frozen V1-1277 evidence.
- `CEM-V1-202`: `dogfood batch-check` and `dogfood change`'s sealed-CEM admission (`DCW-V0-039`
  (b), `DCW-V0-042`) MUST verify a sealed constituent only as a `cem/1.0` map: the verifier's
  `cem status` always runs with `--artifacts <absolute repository root>`, and only `ok`,
  `accepted` and a stable `ACCEPT` with exactly that base and bind proves it (`CEM-V1-148`). The
  0.x `ready-for-ci` proof form is retired: a 0.x status line proves no map. A sealed map that is
  unreadable, not a stable map, or declares a spec that is neither `cem/1.0` nor retired
  (`CEM-V1-200`) fails `constituent-cem-invalid`.
- `CEM-V1-203`: A sealed `cem/0.1`, `cem/0.2` or `cem/0.3` map that the range adds and whose
  blob still equals its bind commit's `.corvint/change.cem.json` (a pre-cutover branch's own
  seal) is a retired seal. It is not verified and grants no coverage. `batch-check` prints
  `NOTE sealed-cem-retired bind=<bind> spec=<spec> NOT_REVERIFIED coverage=none` with a
  `fix:` line, does not count it as a constituent, and does not count its range or its seal
  commit as bound, so those commits fail `unbound-commits` unless a verified `cem/1.0`
  constituent's own range covers them. The recovery is to merge `origin/main`, remove the
  retired seal file with a forward commit (its bind commit and sidecar stay in history), and rebind
  the branch as one `cem/1.0` change whose `BASE` precedes the retired bind, for example the
  `origin/main` commit merged. The removal is required because `CEM-V1-201` refuses any `cem/0.x`
  map in the committed corpus; since the seal is added and removed inside `BASE..HEAD`, the net
  diff that both `dogfood change` and `batch-check` read holds no sealed-CEM change for it, and
  the new map's range covers the retired bind and seal commits. `dogfood change` admits a retired seal with
  `NOTE sealed-cem-retired ... NOT_REVERIFIED bound-by-this-change`, because that change binds
  and cites the whole range itself. No rebase, history rewrite or allow-list is needed. A seal
  modified after its bind commit keeps `sealed-cem-drift` and `DCW-V0-050` keeps its migration
  results.
- `CEM-V1-204`: The converter (`internal/cem/convert`), `corvint cem convert`
  (`internal/cem/cli/convert.go` and its dispatch), `tools/cem-convert-parity`,
  `script/cem-migrate-maps.sh` and their tests are removed. The stable dispatch handles only
  `verify-stable`, so `corvint cem convert ...` falls through to the generic `cem` parser and
  exits 2 with `invalid-arguments` naming the invalid choice; the help names no `convert`
  action.

Non-goals: no removal of `wire.ParseMap`, the 0.x verifier, the `--spec cem/0.2` or `cem begin`
0.x producers, the MCP, frontier, conformance or attest/anchor 0.x paths (decision 0525 §2 keeps
attest and anchor with the wire owner), the 0.x goldens or the frozen `cli-parity` case
`cem-begin-map` (V1-1278 S2/S3); no rewrite of any committed map or bind commit; no new
migration commit.

Failure modes: a pre-cutover branch that sealed a `cem/0.x` map and merges without rebinding
fails `unbound-commits` with the retired-seal NOTE and its fix, never passes on an unverified
map; a migrated corpus map edited after its migration commit fails `CEM-V1-201` by byte comparison with the blob that commit wrote, and a native map by byte comparison with its bind commit's sidecar; a
shallow checkout without a source blob or bind commit reports `NOT_RUN`; a verifier build that
prints a 0.x `ready-for-ci` line for a stable map fails `constituent-unverified`.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-200 | `TestCEMV1200RetiredSpecRefusedWithMergeFix` (`internal/cem/wire`) | Each retired spec gives `unsupported-spec` with "is retired" and the exact fix, and no "rebase", "force-push" or `--force`; `cem/0.4` keeps the plain refusal; a late 0.x seal in a copied corpus is caught by the corpus scan |
| CEM-V1-201 | `TestCEMV1201CommittedCorpusIsStable` (`internal/cem/wire`), `TestCEMV1201CommittedCorpusBinding` (`internal/cem/corpusbinding`, undeclared so full CI runs it unconfined with Git; it fails instead of skipping under `CI=true`, V1-1334) | At `945867cf` (V1-1334): 726 committed maps parse stable; 615 migrated maps match their source blob digests and retired source specs and equal the blobs their migration commits wrote (614 at `0b6f1bc2`, 1 at `3b86121f`), and 111 native maps equal their bind commits' sidecars; the native count grows with each seal; a late 0.x seal and its removal are checked together |
| CEM-V1-202 | `TestBatchCheckDCWV0039RequiresStableVerification`, `TestBatchCheckDCWV0039PassesSealedConstituents`, `TestBatchCheckAdmitsStableConstituent` (`internal/dogfoodflow`) | A stable ACCEPT admits with `--artifacts`; empty, malformed, incomplete, REJECT, other-target, count-less and 0.x `ready-for-ci` output fail `constituent-unverified` |
| CEM-V1-203 | `TestBatchCheckCEMV1203RetiredSealHasNoCoverage` (`internal/dogfoodflow`) | For each retired spec: batch-check exits 1 with the NOTE, its fix and `unbound-commits count=3` and runs no verifier; `dogfood change` admits it with the bound-by-this-change NOTE; after a `cem/1.0` rebind whose `BASE` precedes the retired bind the batch passes with one constituent; a `cem/0.4` seal fails `constituent-cem-invalid` |
| CEM-V1-204 | `TestCEMV1204StableDispatchNoLongerCapturesConvert` (`internal/cem/cli`) | `cem convert` is not captured by the stable dispatch and is refused by the generic parser with exit 2 |

Rollback: revert the V1-1278 S1 change. The converter, `cem convert`, the parity tool, the
migration script and the 0.x `ready-for-ci` proof form return; `ParseStable` refuses a 0.x map
with the plain "stable profile required" message again; and a retired seal again verifies
through the 0.x reader. No committed map, schema, packet or bind commit changes.

## Local-completion stable sidecar reuse (V1-1303, decision 0525)

Status: proposed; V1-1303. `CEM-V1-150` left local-completion sidecar-only reuse
(`AllowCemSidecarOnlyReuse`) with no `cem/1.0` grant path, so a check observed before the CEM
commit always ran again. This section supersedes `CEM-V1-150` for the grant path only: a
passing observation made on an earlier target may stand for the current target when the only
difference is a committed `cem/1.0` sidecar that the stable verifier accepts in process. Every
other case keeps `CEM-V1-150`'s fail-closed default.

- `CEM-V1-205`: A check whose plan sets `AllowCemSidecarOnlyReuse` and whose latest passing,
  clean observation was made on another target MUST qualify on the current target only when
  (a) the observation's content digest of the tree excluding `.corvint/change.cem.json` equals
  the current one; (b) the plan's base and the current target are both full 40-hex object IDs;
  (c) `verify.Stable` returns exit 0, outcome `ACCEPT` and `accept: true` for the sidecar with
  `Repository` and `ArtifactRoot` both the absolute repository root, `ExpectedBase` the plan's
  base and `Target` the current target; and (d) the result reports `sidecar: EXACT`,
  `expectedBase` equal to the plan's base and `targetRevision` equal to the current target.
- `CEM-V1-206`: The verifier MUST receive the exact bytes read from
  `<root>/.corvint/change.cem.json` (bounded by `wire.MaxMapBytes`), never a re-encoding, and
  the result's `mapSha256` MUST equal the SHA-256 of those bytes. A working-tree sidecar that
  differs from the sidecar committed at the target is refused by the verifier or is not `EXACT`,
  and grants nothing; a
  committed sidecar is identified by its raw digest, so a committed whitespace-only re-encoding
  that still verifies is those bytes and may grant.
- `CEM-V1-207`: A refused reuse MUST leave the check unqualified (`selected-check-unverified`,
  with the existing `dogfood verify` rerun action) and record one closed reason in the check
  observation's additive `reuseRefused` member: `sidecar-content-changed` (a),
  `sidecar-ids-not-full` (b), `sidecar-unavailable` (the sidecar cannot be read),
  `sidecar-rejected` (verifier `REJECT`, for example a stale map, an edited digest or a wrong
  base), `sidecar-unverifiable` (verifier `UNSUPPORTED` or refusal, for example a missing
  artifact under the root) or `sidecar-binding-mismatch` (an accepted result that does not name
  these exact bytes, this base and this target as `EXACT`). `reuseRefused` is omitted when the
  plan does not allow reuse, when the check qualifies, or when no observation applies.
- `CEM-V1-208`: Without `AllowCemSidecarOnlyReuse`, a check observed on another target MUST stay
  `selected-check-unverified` and no verifier runs (`CEM-V1-150`). No reuse path may report an
  internal error or grant silently: every verifier outcome other than the `CEM-V1-205` grant maps
  to a `CEM-V1-207` reason.
- `CEM-V1-209`: Each of `CEM-V1-205..208` MUST be traced to a named test in
  `internal/localcompletion` that drives a real `cem prepare` stable map through the in-process
  verifier.

Non-goals: no change to the stable verifier, the wire, `cem prepare`, the dogfood bind loop or
the plan format; no reuse for a repository whose object IDs are not 40-hex (SHA-256 object
format); no caching of verifier results across evaluations; no change to `LCP-V0-004` text.

Failure modes: a sidecar edited after the observation's target, a map from an earlier target, a
wrong base or a missing artifact refuses with its closed reason and the check reruns; an
abbreviated base refuses `sidecar-ids-not-full` before any verifier run; a source change after
the observation refuses `sidecar-content-changed`; a host where structural hunks are outside the
frozen runtime tuples gets `UNSUPPORTED` and refuses `sidecar-unverifiable`. Each reusing check
costs one in-process verifier run per evaluation.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-205 | `TestCEMV1205StableSidecarAcceptGrantsReuse`, `TestCEMV1205ArtifactRootIsRepositoryRoot` (`internal/localcompletion`) | A committed `cem prepare` stable map grants reuse with no rerun; a map with runner artifacts under the root grants, and the same map with one artifact removed refuses `sidecar-unverifiable` |
| CEM-V1-206 | `TestCEMV1206EditedSidecarRefusesReuse` (`internal/localcompletion`) | A committed `patchSha256` edit refuses `sidecar-rejected` (`patch-digest-mismatch`); an uncommitted re-spacing of the committed sidecar refuses `sidecar-rejected` (`excluded-artifact-mismatch`) |
| CEM-V1-207 | `TestCEMV1207StaleSidecarRefusesReuse`, `TestCEMV1207WrongBaseSidecarRefusesReuse`, `TestCEMV1207UnboundSidecarRefusesReuse` (`internal/localcompletion`) | A map from the earlier target refuses `sidecar-rejected` (`patch-digest-mismatch`); a later source change refuses `sidecar-content-changed`; a wrong base refuses `sidecar-rejected` (`base-revision-mismatch`); no sidecar refuses `sidecar-unavailable`, an uncommitted sidecar `sidecar-binding-mismatch` and an abbreviated base `sidecar-ids-not-full`; each refusal names the `dogfood verify` rerun |
| CEM-V1-208 | `TestCEMV1208SidecarReuseFailsClosedByDefault`, `TestCEMV1150StableSidecarGrantsNoReuse` (`internal/localcompletion`) | Without the plan flag a verifying committed sidecar leaves the check `selected-check-unverified` with no `reuseRefused` |
| CEM-V1-209 | This table | Every test above passes on darwin; the suite skips on hosts other than darwin and linux |

Rollback: revert the V1-1303 change. Sidecar-only reuse again grants nothing (`CEM-V1-150`),
the `reuseRefused` member disappears and every check observed before the CEM commit reruns. No
wire, schema, packet, plan or committed map changes.

## `cem/v2` statement (V1-1265, decision 0523 P1)

Status: proposed. Decision 0523 P1. A `cem/v1` statement signs only the map's bytes, its base and
its patch, so a stable map's artifacts and the commit it was accepted at are not signed. `cem/v2`
signs all three for a `cem/1.0` stable map. `cem/0` and `cem/v1` are unchanged.

- `CEM-V1-065`: `internal/attest.CEMStatementV2(name, map, target)` MUST accept only bytes that
  declare `cem/1.0` and parse as a stable map, and otherwise MUST return a canonical in-toto
  Statement v1. Its `predicateType` is `https://corvint-context.dev/attestation/cem/v2`. Its
  `subject` list is the map first (`{"digest":{"sha256":HEX},"name":NAME}` over the exact map
  bytes), then one `{"digest":{"sha256":S},"name":P}` for each declared artifact, in map order,
  where P and S are the artifact's declared `path` and `sha256`; subject names are unique. Its
  `predicate` is exactly `{"baseRevision":B,"patchSha256":H,"target":{"gitCommit":C,"gitTree":T}}`,
  with B and H the map's `change.baseRevision` and `change.patchSha256`, and C and T the full
  lowercase commit and tree OIDs of the accepted target. A verifier MUST refuse a predicate with an
  unknown, repeated or case-variant member, a missing target, or a malformed digest.
  `corvint prove --cem MAP --expected-base OID --target OID [--artifacts ABS] --attest-cem-v2`
  MUST refuse a `cem/0.x` map with `attest-cem-v2-requires-stable-map`. It runs the stable verifier
  over the map, and MUST refuse any result that is not ACCEPT with a target revision, with
  `attest-cem-v2-not-accepted`. Otherwise it prints the unchanged proof statement or envelope,
  then the `cem/v2` statement or its DSSE envelope over the accepted target. `--attest-cem`,
  `--attest-cem-v1` and `--attest-cem-v2` are mutually exclusive, and `--artifacts` requires
  `--attest-cem-v2` (`invalid-arguments`). Amended by V1-1278 S2b (`CEM-V1-218`): `--attest-cem`
  and `--attest-cem-v1` are retired and refuse as unrecognized arguments (`invalid-arguments`), so
  `--attest-cem-v2` is the only CEM predicate flag. `prove --verify-cem-attestation ENVELOPE
  --attest-public-key PEM --cem MAP [--artifacts ABS] [--target REV]` MUST refuse:
  - map bytes that differ from the signed map subject, with `attest-cem-mismatch`;
  - an artifact file whose digest is not its signed subject, or an artifact set that differs from
    the signed artifact subjects, with `attest-cem-subject-mismatch`;
  - a `--target` whose commit or tree is not the signed target, with `attest-cem-target-mismatch`.

  The same two codes refuse `--artifacts` or `--target` given against a `cem/0` or `cem/v1`
  envelope, which signs neither. Amended by V1-1278 S2b (`CEM-V1-218`): a signed `cem/0` or
  `cem/v1` envelope is no longer read as a claim; it refuses with `attest-verification-failed`,
  whether or not a map is supplied, and the library error names `cem/v2` as the only CEM predicate. A check that is not supplied is reported NOT_RUN in
  `status`, `artifactsStatus` or `targetStatus`, never VERIFIED. `cem anchor` and `cem provenance`
  MUST route a committed map that declares `cem/1.0` through the stable parser. The pointer records
  `map_spec` `cem/1.0` and the map's sha256, the provenance read reports it verified, and the read
  mutates nothing. A `cem/0.x` map's pointer bytes are unchanged (superseded by `CEM-V1-210`: a
  committed `cem/0.x` map refuses with `unsupported-spec` and writes no pointer).

Non-goals: `cem/v2` carries no verdict ledger and no stored verification result (the opt-in ledger beside the anchor pointer is `CEM-V1-271..279`, V1-1292); `cem/v1` is not widened to stable
maps; no signing producer, key management, transparency log or network step; no change to the
stable map schema, `RESULT.schema.json` or the verifier; the target tree is not re-derived from the
artifacts.

Failure modes: a map re-issued over another receipt attestation has new bytes, so the old envelope
refuses it with `attest-cem-mismatch`; the old map with a swapped artifact file is refused with
`attest-cem-subject-mismatch`; a missing or unreadable artifact file is refused with the same
code, never skipped; a rejected, unsupported or drifted stable verification emits no statement;
a `--target` that does not resolve to a commit exits 2 with `unsupported-prove-revision`; a stable
map that fails to parse is not anchored.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-065 | `TestCEMV1065StatementBytesAndSubjectOrder`, `TestCEMV1065VerifiesMapArtifactsAndTarget`, `TestCEMV1065RefusesSwappedReceiptArtifact`, `TestCEMV1065RefusesTargetThatIsNotTheSubject`, `TestCEMV1065PredicateIsClosed`, `TestCEMV1065V1UnchangedForStableMaps` (`internal/attest`) | Statement bytes and subject order over the public `receipt-authentication/` map; the receipt swap is refused under the old envelope both ways; a closed predicate; `cem/v1` still refuses a stable map |
| CEM-V1-065 | `TestCEMV1065ProveEmitsAndVerifiesTheV2Statement`, `TestCEMV1065RefusesAReceiptSwapUnderTheOldEnvelope`, `TestCEMV1065RefusesATargetThatIsNotTheSubject`, `TestCEMV1065RefusesALegacyMap`, `TestCEMV1065RefusesConflictingFlags` (`cmd/corvint`) | `prove --attest-cem-v2` over the materialized public case emits four subjects and the accepted target, verifies VERIFIED on all three axes without mutation, and refuses the swap, a foreign target, a `cem/0.x` map and every flag conflict |
| CEM-V1-065 | `TestCEMV1065AnchorStableMap`, `TestCEMV1210AnchorRefusesARetiredMap` (`internal/gitnotes`) | A committed stable map anchors with `cem/1.0` and its sha256, and provenance reads it back verified without mutation; a committed retired `cem/0.x` map refuses `unsupported-spec` with the fix line and writes no note (`CEM-V1-210`); walkthrough in `docs/build-log/2026-10-12-cem-v2-statement.md` |

Rollback: revert `internal/attest/cem_v2.go`, the `V2` field and the v2 parser entry in
`internal/attest`, the stable anchor read in `internal/gitnotes/anchor.go` (the original
`internal/cem/wire/route.go` router was deleted by V1-1278 S2b), the
`--attest-cem-v2` and `--artifacts` branches in `cmd/corvint/prove.go` and
`cmd/corvint/prove_attest_cem.go`, the help lines and this section. No stored state was written in
the new shape: `cem/0` and `cem/v1` bytes are unchanged. A stable-map anchor note already written
stays readable as a plain note, and after rollback `cem provenance` reports its map as
unsupported again.

## Stable verification result codes

The optional `cem verify-stable` operation emits the closed result codes of
`protocol/cem-1.0/stable/OPERATION.json` and `RESULT.schema.json`; those public files stay the
normative mapping. This table names the codes first emitted by the stable verifier, so each has
one owning specification. A code reports a bounded check and never upgrades authority.

| Stage | Codes | Outcome |
| --- | --- | --- |
| references | `invalid-criterion-identity`, `duplicate-reference`, `unresolved-reference`, `reference-coherence`, `artifact-role-closure` | Exit 2; the decoded reference graph is rejected without filesystem or native authority inference |
| authority-arguments | `artifacts-required` | Exit 2 (`UNSUPPORTED`, CEM-V1-116); the explicit artifact root is required even for an empty reference set |
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

- `CEM-V1-017`: Core `cem status` and `cem verify` over an exact `cem/1.0` map MUST run the stable verifier over the repository root byte for byte as the caller named it (the `--root` value, even empty, with no tilde expansion, absolutization, cleaning or symlink resolution; the working directory only when `--root` is omitted), judged exactly as `cem verify-stable --repository` judges the same string: a relative or empty root is refused at stage `arguments` with `invalid-arguments` before the map is verified, and a symlinked or lexically unclean absolute root at stage `repository`. A root beginning with `--` (for example `--root=--repo`) is refused as `cem verify-stable` refuses that value named first as `--repository`: its parse stops there, so the result keeps no `expectedBase` or `targetRevision`. It runs with the independently supplied `--expected-base`, `--target` and an explicit absolute `--artifacts` root. The envelope MUST embed the complete, unmodified closed result as `stableVerification` and the command MUST exit with the verifier's own code (0 accept, 1 reject, 2 refusal). It MUST NOT synthesize a 0.x `verification`, counts, worklist, policy issues or `ready-for-ci`; status names the outcome `accepted`, `rejected` or `unsupported`. Amended by V1-1276: the one exception is the `cem status` policy ceilings of `CEM-V1-145`, which after an `ACCEPT` add the counts and `policyIssues` and exit 1 when a ceiling fails, with `stableVerification` still unmodified. Core MUST NOT open or pre-read the repository before the verifier's own metadata admission.
- `CEM-V1-018`: A stable read MUST refuse `--patch` with `invalid-arguments`, and `cem verify` MUST refuse policy ceilings with `invalid-arguments` (`cem status` admits them since V1-1276, `CEM-V1-145`), and every other Core workflow read of a `cem/1.0` map with `unsupported-spec` naming the stable reads, until its own slice lands; `cem report` and the report preview are read since S2 (`CEM-V1-034..036`). Each such refusal is marked as a stable refusal, which MCP names `cem-map-stable-refused` (`MCPV0-037`). A 0.x map MUST refuse `--artifacts` with `invalid-arguments` after its map parses. Every 0.x parser refusal of an exact `cem/1.0` root MUST name the stable read rather than report malformed 0.x input. Amended by V1-1278 S2b (`CEM-V1-215`): no 0.x map is read and no 0.x parser remains, so the 0.x `--artifacts` refusal and the parser's naming of the stable read are retired; every root that is not exactly `cem/1.0` refuses with `unsupported-spec`, a retired `cem/0.x` root with the `wire.RetiredSpecFix` line, with or without `--artifacts`.
- `CEM-V1-019`: 0.x reads MUST keep their arguments, precedence, symlink-resolved root, output bytes and exit codes. The stable route MUST be chosen only after the 0.x parser refuses a bounded JSON object whose `spec` is exactly `cem/1.0`; the experimental candidate and every other spec keep their existing code and message, and malformed JSON keeps its earlier failure. Amended by V1-1278 S2b (`CEM-V1-215`): the 0.x reads are retired. `cem status`, `cem verify` and `cem report` take the stable route for an exact `cem/1.0` root and refuse every other spec value, the experimental candidate included, with `unsupported-spec` before any Git read.

Failure modes: an omitted `--artifacts` is the verifier's `authority-arguments` /
`artifacts-required` refusal, never a Core default. A runtime or repository the
verifier does not support is its explicit `UNSUPPORTED` result. The only 0.x change
is that `--artifacts` on a 0.x read is now a stage-3 `invalid-arguments` refusal
after the map read instead of an argument-parser refusal; both exit 2 with no
output. Rollback removes the route and the flag; no 0.x byte, map or packet changes.

### Core stable read traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-017 | `TestCoreStableReadMatchesVerifyStable` | Core verify and status equal `cem verify-stable` (all fields, exit); the public sealed positive is ACCEPT/EXACT on Go 1.27.1 |
| CEM-V1-017 | `TestCoreStableReadKeepsVerifierExit` | Missing `--artifacts` exits 2 (`artifacts-required`); an altered artifact exits 1 (`artifact-digest-mismatch`) |
| CEM-V1-017 | `TestCEMStableReadKeepsCallerRoot` | Through the real argv entry, status and verify with `--root R` equal `cem verify-stable --repository R` (result and exit) for the real root, the symlinked alias, `<alias>/../repo`, `<repo>/`, `./repo`, `repo`, `./alias`, `--root ""`, `--root=`, `--root=--repo`, `--root=-x` and `--root=--map`; an omitted `--root` equals `--repository <working directory>`; only the real root and the omitted root pass admission |
| CEM-V1-017 | `TestCEMStableReadDefaultsToSessionRoot` | A workflow session whose entry point never sets a stable repository admits its own unresolved root: a session opened on the symlinked alias equals `cem verify-stable --repository <alias>` (stage `repository`, result and exit) |
| CEM-V1-018 | `TestCoreStableReadExplicitRefusals` | Status/verify `--patch`, verify policy ceilings and report-preview `--patch` refuse before verification (CLI exit 2); status ceilings are `CEM-V1-145` |
| CEM-V1-018, CEM-V1-019 | `TestCEMV1215ReadsRefuseEveryNonStableSpec` | Since V1-1278 S2b a retired 0.x map refuses `unsupported-spec` with the fix line, with or without `--artifacts`, and candidate, other, padded and numeric specs refuse `unsupported-spec`; the 0.x `--artifacts` and parser-text tests were deleted with the 0.x parser |
| CEM-V1-019 | `TestDeclaresStable` | Route selection is exact, bounded and object-rooted |

## Core stable mutator refusals (V1-0632 S3)

Status: proposed. This slice keeps the Core mutators `cem/0.x` producers: the
optional candidate assembler stays the only stable producer until the stable wire
is frozen (`CEM-V1-007`). It adds no producer, default, help entry or promotion.
The requirement IDs start at 030 so that slices proposed in parallel do not collide;
integration may renumber them.

Amended by V1-1276 (`CEM-V1-140..148`): `CEM-V1-030` and `CEM-V1-031` now bind the `cem/0.x` producers (`cem prepare --spec cem/0.2`, `cem begin` without `--spec cem/1.0`, and a mutation of a `cem/0.x` input); a `cem/1.0` producer map is written and mutated by `CEM-V1-141..144`, and `CEM-V1-032` is superseded by `CEM-V1-147` and `CEM-V1-148`.

- `CEM-V1-030`: A `cem/0.x` Core `cem prepare` MUST refuse an existing map at the fixed sidecar path whose bytes declare exactly `cem/1.0` with `unsupported-spec`, with or without `--replace`. It MUST leave the map, the patch cache and any backup unchanged, and MUST NOT resume, downgrade or overwrite a stable sidecar. It MUST refuse in the same way, before it takes the sidecar lock or recovers a backup, when a recognized interrupted-publication backup (`<map>.bak-` plus sixteen lowercase hex digits) declares `cem/1.0`, and MUST refuse a recognized regular backup it cannot read with `map-unavailable`, because unread bytes may be stable; it re-checks backups under the lock, and neither recovery nor resume cleanup restores or removes a refused backup. With `--replace`, it MUST refuse an existing regular sidecar it cannot read with `map-unavailable` instead of regenerating it, because unread bytes may be stable.
- `CEM-V1-031`: Core `cem cite`, `cem mark`, `cem cover` and `cem discriminate` MUST refuse a `cem/1.0` input map that is not a producer map (`CEM-V1-143`) with `unsupported-spec` naming the command and the stable read, in place or with `--output`, and write nothing. Every Core map writer (`begin`, `cite`, `mark`, `cover`, `discriminate`) writing `cem/0.x` MUST refuse an output destination whose existing bytes declare exactly `cem/1.0`, leaving it unchanged; a writer writing `cem/1.0` refuses such a destination unless it is a producer map. Each writer first runs a non-mutating preflight that refuses a readable `cem/1.0` input or destination before any lock file, directory or backup recovery touches the file system. It then holds the output lock across a strict probe and publication. Only an absent destination, one that is not a regular file (publication replaces the directory entry, never a link target), or one over the map bound (which `wire.DeclaresStable` never selects) proceeds unread. A regular destination whose bytes cannot be read, or that changes while it is read, MUST be refused with `map-unavailable` naming the command and `cem/1.0`, leaving it unchanged.
- `CEM-V1-032`: (superseded by `CEM-V1-147` and `CEM-V1-148`, V1-1276) The daily bind loop MUST read no hunks from a map declaring `cem/1.0`, so such a map never counts as cited and never binds a citation plan. Its cite step MUST refuse such a map as `cem-cite` `NOT_PRODUCED` `cem-map-stable-unsupported` before it reads the plan, writing no plan binding, cite log or map, and never treat it as a map that owes no hunk. `dogfood batch-check` MUST refuse a sealed `cem/1.0` constituent with `constituent-cem-stable-unsupported` before any verifier run, never as `constituent-cem-invalid`.
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
| CEM-V1-032 | Superseded (V1-1276) | Its tests became `TestMapHunksReadsStableMap`, `TestCiteStepAdmitsStableMap` and `TestBatchCheckAdmitsStableConstituent` (`CEM-V1-147`, `CEM-V1-148`) |
| CEM-V1-033 | `TestPrepareNonStableExistingMapUnchanged` | Candidate, padded and other specs keep the `map-unavailable` refusal and guidance; `--replace` still regenerates them |
| CEM-V1-033 | `TestMutatorsWriteNonStableDestination` | Candidate, truncated and non-JSON destinations are still replaced by the 0.x map |
| CEM-V1-033 | `TestMapHunksReadsStableMap` | A map that does not decode still has no hunks; every other spec keeps the layout-agnostic decode |

## Core stable report (V1-0632 S2)

Status: proposed. This slice moves `cem report`, the report preview and the MCP
`corvint.cem.report` tool from refusing a `cem/1.0` map to reading it through the
stable verifier. It adds no producer, no default, no help entry and no stable
promotion; the read stays hidden like the S1 read.

- `CEM-V1-034`: Core `cem report --format json` over an exact `cem/1.0` map MUST run the stable verifier exactly as `CEM-V1-017` does, admitting the root exactly as the caller named it (the MCP server's start-time `--root`, not its resolved root), embed its complete, unmodified closed result as `stableVerification`, and exit with the verifier's own code. It MUST add a `review` projection with profile `cem-stable-review/0` that is a pure function of that result: its outcome, stage and code, the map and patch digests, the two revisions, a `stableResultSha256` over the key-sorted result, one row per hunk with its evidence drift state (or `NOT_PRODUCED`), coverage and discrimination states copied from the verifier, `testExecution: NOT_RUN`, an explicit `ocm: NOT_PRODUCED`, the non-claims, and a `recordSetSha256` over the projection. It MUST NOT synthesize a 0.x `verification`, counts, policy issues, worklist or `ready-for-ci`.
- `CEM-V1-035`: Core `cem report` (markdown) over a `cem/1.0` map MUST publish the review rendered from that projection for every verifier outcome, with each repository-derived value in a code span, under the same output resolution, map-aliasing and size bounds as the 0.x report. It MUST locate the worktree's Git directories for publication only after the verifier returns and without the 0.x object admission (alternates, attributes), so a result the verifier refused at stage `repository` still publishes; when the Git directories cannot be located (no valid primary or reciprocal worktree marker), no output path can be checked and publication MUST fail closed with `repository-object-unavailable` and nothing written. The report preview MUST return the same Markdown and `recordSetSha256` without writing, and JSON, preview and published Markdown MUST agree on `recordSetSha256`.
- `CEM-V1-036`: (amended by V1-1278 S1 for V1-1305 under the owner's standing auto-accept of lane requirements (2026-10-11)) A stable report MUST refuse every supplied `--patch` and policy ceiling, and an empty `--ocm`, including an empty value, with `invalid-arguments` before the verifier runs and before any file is written; these refusals and any unsupported stable action are marked as stable refusals. A non-empty `--ocm` on a stable report MUST produce the OCM review over the `cem/1.0` map through the installed stable OCM reader (`lrfrepo.VerifyStableUniverse`), bound to the OCM bytes Core read; an OCM the stable admission refuses is reported invalid with its closed code and exit 1, and a caller without a stable reader (library or MCP) reports the OCM `NOT_PRODUCED` with reason `ocm-verifier-unavailable`, never ok. A 0.x report MUST refuse `--artifacts` with `invalid-arguments` after its map parses. 0.x report arguments, bytes, goldens and refusals are otherwise unchanged. Amended by V1-1278 S2b (`CEM-V1-215`): the 0.x report and its review projection are retired, so a 0.x report map refuses with `unsupported-spec` and the `wire.RetiredSpecFix` line, with or without `--artifacts`.

Failure modes: an empty `--ocm` refuses; a non-empty `--ocm` (since V1-1305) is read within
the session root, an absent file is `map-unavailable`, a path outside the root is
`invalid-arguments` (`path-traversal`), and an OCM the stable admission refuses is reported
invalid, never bound to the 0.x obligations of another map. A runtime the verifier does not
support yields an `UNSUPPORTED` review that still publishes and exits 2. Rollback
removes the stable report route, the `artifacts` MCP argument and the
`cem-map-stable-refused` code; no 0.x byte, map or packet changes.

### Core stable report traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-034 | `TestCoreStableReportJSONEmbedsVerifierResult` | JSON `stableVerification` equals `cem verify-stable` (all fields, exit); the projection digest recomputes; no 0.x keys; ACCEPT and `stable` drift asserted on Go 1.27.1 |
| CEM-V1-034 | `TestCEMReportToolPreviewsStableMap` | The MCP receipt carries the verifier's complete result for the same inputs, with no 0.x fields |
| CEM-V1-034 | `TestCEMStableReportKeepsCallerRoot`, `TestCEMReportToolAdmitsTheCallerNamedRoot` | For each `--root` spelling (real, alias, `<alias>/../repo`, `<repo>/`, `./repo`, `repo`, `./alias`, `--root ""`, `--root=`, omitted, and the option-shaped `--repo`, `-x`, `--map`) the CLI report equals `cem verify-stable --repository <same string>` (complete result, exit); the MCP server accepts only the real root and the alias, each equal to `verify-stable` on that string, and refuses the other spellings at start as `invalid-root` |
| CEM-V1-035 | `TestCoreStableReportPublishesMarkdown` | Published for every outcome; preview equals the published file; `recordSetSha256` agrees across JSON, Markdown and preview |
| CEM-V1-035 | `TestCoreStableReportPublishesRepositoryRefusals` | With `objects/info/alternates` the `UNSUPPORTED` / `repository` / `unsupported-object-alternates` review publishes to the default and a safe external output, equal to `verify-stable`; with no locatable Git directory JSON keeps the result while Markdown fails closed with nothing written |
| CEM-V1-036 | `TestCoreStableReportRefusals` | An empty `--ocm`, an absent or out-of-root OCM, `--patch`, ceilings (each also empty) and an output naming the map exit 2 with nothing published; `--ocm=` and `--patch=` are argument-parser refusals; ceiling refusals are stable refusals, `cem/9` and 0.x are not |
| CEM-V1-036 | `TestV11305StableReportOCM` (`cmd/corvint`), `TestV11305StableReportOCMWithoutVerifier` (`internal/cem/cli`) | A legacy-bound OCM is invalid `cem-map-digest-mismatch`; a rebound OCM produces coverage, worklist rows and the hunk join; without a reader the OCM is `NOT_PRODUCED` `ocm-verifier-unavailable` |
| CEM-V1-036 | `TestCEMV1215ReadsRefuseEveryNonStableSpec` | Since V1-1278 S2b a 0.x report map refuses `unsupported-spec` with the fix line, with or without `--artifacts` |
| CEM-V1-036 | `TestCEMReportToolNamesStableRefusals` | MCP maps only stable refusals to `cem-map-stable-refused` |

## Portable stable routing (V1-0632 S4)

Status: proposed; retired by V1-1278 S3 (`CEM-V1-222`, `CEM-V1-223`). The portable consumer's
`cem/0.1` default, its `ci` report and `examples/cem` are deleted, so `CEM-V1-PSR-001..004` no
longer bind anything; their tests are kept at commit `5a056d0a`. The text below is history.
Native work: V1-0632, migration rows C04 and C19 of
`protocol/cem-1.0/stable/MIGRATION.json`. The independent portable consumer `interop/cem01-go`
keeps its `cem/0.1` default and its `ci` report schema. It now recognizes an exact `cem/1.0` map
and names the stable command, and an example CI branch runs that command only on explicit opt-in.

- `CEM-V1-PSR-001`: (retired by V1-1278 S3) The default `verify` path refuses a strictly parsed JSON object whose exact
  `spec` member is the string `cem/1.0` with the invalid code `unsupported-spec` (exit 1). The
  refusal comes before the patch is read and before any `cem/0.1` structural check, so a stable map
  is never reported as a malformed `cem/0.1` map.
- `CEM-V1-PSR-002`: (retired by V1-1278 S3) `ci` reports such a map as verdict `unsupported-profile`, exit 4, with `code`
  `use-verify-stable-canonical`. The 14 fixed `cem-ci-report/0` members, the `profile` value
  `cem/0.1` and the verdict-to-exit map (CEM-PILOT-021/022) are unchanged.
- `CEM-V1-PSR-003`: (retired by V1-1278 S3) Only that exact case changes. A duplicate, non-string, differently cased,
  padded or candidate `spec`, malformed JSON, invalid UTF-8, an unpaired surrogate, trailing data
  and every `cem/0.x` map keep their earlier code, bytes and goldens. Frozen protocol packets are
  not edited.
- `CEM-V1-PSR-004`: (retired by V1-1278 S3) `examples/cem/verify-portable.sh` runs `verify-stable-canonical` only when
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
| CEM-V1-PSR-001 | Retired by V1-1278 S3 (`CEM-V1-222`); `decodeMap` and `declaresStableSpec` are deleted | The test is kept at commit `5a056d0a` |
| CEM-V1-PSR-002 | Retired by V1-1278 S3 (`CEM-V1-222`); `interop/cem01-go/ci.go` is deleted | The test is kept at commit `5a056d0a` |
| CEM-V1-PSR-003 | Retired by V1-1278 S3 (`CEM-V1-222`); the 0.x default path is deleted | The tests are kept at commit `5a056d0a` |
| CEM-V1-PSR-004 | Retired by V1-1278 S3 (`CEM-V1-223`); the example recipe tree and its script test are deleted | The tests are kept at commit `5a056d0a` |

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
  bytes and codes. `VerifyUniverse`, `lrf.Evaluate` and `corvint lrf` refuse a `cem/1.0` map
  explicitly with `unsupported-spec`, instead of reducing it or reporting it as malformed 0.x
  input. Amended by V1-1276: `ocm read`, `ocm prepare` and `ocm link` admit a `cem/1.0` map only
  through `CEM-V1-149`; `ocm prepare` still refuses before it writes or replaces anything.
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
`lrf.ValidateStableRequest`, and the explicit `cem/1.0` refusals in `VerifyUniverse`
(the `readOCMBytes` and `PrepareOCM` refusals became the `CEM-V1-149` admission in V1-1276). No `lrf/0` or OCM profile, default, packet or stored evidence
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
| CEM-V1-058 | `TestCEMV1058LegacyOCMLRFAdmissionUnchanged` | The `cem/0.2` universe and `lrf.Evaluate` still accept; `Evaluate` refuses `cem/1.0` with `unsupported-spec`; a bound `cem/1.0` map whose artifacts are outside the repository root fails closed (`ReadOCM` state `invalid` with cause `artifact-unavailable`, `PrepareOCM` `artifact-unavailable` and `LinkOCM` `invalid-cem`, map unchanged, `CEM-V1-149`); existing `internal/lrfrepo`, `conformance/ocm-v0`, frontier, TCQ and CLI LRF/OCM suites pass |
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
  reproduce the source's complete closed result and exit. Export MUST verify the written bundle
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
- `CEM-V1-079`: The registry stays closed: an unsupported profile, a near-stable spec, an unknown member, a duplicate or case-variant `spec` member that names `cem/1.0` beside any other value, and a stable map the strict reader refuses MUST draw no chain; 0.x maps keep their reading (superseded by `CEM-V1-210`: a retired 0.x map is refused with `unsupported-spec` and the recovery line).

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
| CEM-V1-079 | `TestConsoleChainStableRegistry` | `cem/2.0`, `cem/1.0.0`, an unknown member, duplicate and case-variant `spec` members and an invalid stable map refuse; since `CEM-V1-210` each retired 0.x map refuses with `unsupported-spec` and the recovery line |

## Core cem/1.0 producer and dogfood loop (V1-1276, decision 0525 step 5)

Status: accepted under the owner's standing auto-accept (2026-10-11), after independent review
(Codex round 6 PASS); V1-1276. Decision 0525 makes `cem/1.0` the only map the RC3 loop produces;
this slice switches the Core producers and the daily bind loop to it while the `cem/0.x` readers
stay until V1-1278. It amends `CEM-V1-018`, `CEM-V1-030..033` and `CEM-V1-058` in place and the
`DCW-V0` text that named the refusals. A producer map is a `cem/1.0` map with no criteria,
receipts, links, artifacts or provenance; it may carry a `role` (`CEM-V1-095`).

- `CEM-V1-140`: `corvint cem prepare` MUST write `cem/1.0` by default. `--spec cem/0.2` keeps the
  `cem/0.2` producer as an explicit, fail-closed escape until V1-1278 removes it with the 0.x
  readers. `corvint cem begin` keeps `cem/0.1` by default, because the frozen cli-parity case
  `cem-begin-map` pins it, and writes `cem/1.0` with `--spec cem/1.0`. Any other `--spec` value,
  including `cem/0.1`, `cem/0.3` or the experimental candidate for prepare, MUST be refused with
  `invalid-arguments` before anything is written. The library zero value keeps the legacy
  producer, so MCP, frontier, conformance and every other library caller is unchanged.
- `CEM-V1-141`: A `cem/1.0` prepare MUST write the canonical stable encoding of a producer map
  whose hunks are the canonical diff of base..target, and MUST resume a same-base, same-patch
  stable map byte for byte. An existing `cem/0.x` sidecar MUST be refused with `map-unavailable`
  "records a different spec; pass --replace to regenerate", and a stable map of another base or
  patch with the existing "different base or patch" refusal. With `--replace` it MUST regenerate
  a `cem/0.x` map, a stable producer map, or bytes that declare `cem/1.0` but do not parse, and
  MUST refuse, unchanged, a valid stable map that is not a producer map (`unsupported-spec`) and a
  regular sidecar it cannot read (`map-unavailable`). Without `--replace`, sidecar bytes that are
  neither a valid `cem/0.x` nor a `cem/1.0` map keep the `map-unavailable` "is not a valid CEM
  document; pass --replace to regenerate" refusal. Before the sidecar lock and again under it,
  before backup recovery or resume cleanup restores or removes a recognized interrupted-publication
  backup, a `cem/1.0` prepare MUST refuse, unchanged, a backup that is a valid stable map but not a
  producer map (`unsupported-spec`) or that it cannot read (`map-unavailable`); a stable producer
  or `cem/0.x` backup is restored or removed as before. A `cem/0.x` prepare keeps `CEM-V1-030`.
- `CEM-V1-142`: `cem begin --spec cem/1.0` MUST write the canonical stable producer map of the
  supplied patch at its `--output`, may rewrite a stable producer map there, and keeps the
  `CEM-V1-031` destination guards; a `cem/0.x` begin never replaces a stable map.
- `CEM-V1-143`: `cem cite`, `cem mark`, `cem cover` and `cem discriminate` over a stable producer
  map MUST write a canonical `cem/1.0` producer map, in place or at `--output`, keeping its `role`;
  they MUST NOT upgrade it to `cem/0.3`. A stable map that is not a producer map stays refused
  with `unsupported-spec` naming the command, as `CEM-V1-031` requires.
- `CEM-V1-144`: A structural `cem mark` on a stable producer map writes the claimed reason; the
  stable verifier, not the producer, proves it, so an import reorder verifies `ACCEPT` and a
  wrong-class claim on the same hunk is `REJECT` `unproven-mechanical` from `cem status`.
- `CEM-V1-145`: `cem status` over a `cem/1.0` map MUST admit `--max-unknown` and
  `--max-mechanical`, applied to the verifier's hunk dispositions after an `ACCEPT`: within the
  ceilings the envelope reports `ok: true`, `state: accepted`, the counts and empty
  `policyIssues`; an exceeded ceiling reports `ok: false` with its policy issue and exits 1. The
  embedded `stableVerification` stays exactly the `cem verify-stable` result. `cem verify` keeps
  refusing ceilings (`CEM-V1-018`).
- `CEM-V1-146`: A stable prepare's `nextActions` MUST be the single `cem status` argv with the
  literal base and target, `--artifacts` set to the absolute repository root, and zero
  ceilings. A producer map names no artifact, so that root is never opened.
- `CEM-V1-147`: The daily bind loop MUST cite, count, check and seal a `cem/1.0` map like a
  `cem/0.x` map: `mapHunks` and `citedHunks` read its hunks, the cite step binds the plan to them,
  and the loop's and `dogfood check`'s `cem status` add `--artifacts <absolute repository root>`.
  The first stable pass over a sidecar left by a `cem/0.x` run treats the different-spec refusal
  as an outdated map and regenerates it with `--replace`. `cem-map-stable-unsupported` is no
  longer reported; every other `NOT_PRODUCED` reason stays visible.
- `CEM-V1-148`: `dogfood batch-check` MUST admit a sealed `cem/1.0` constituent when the verifier,
  run with `--artifacts <absolute repository root>`, reports `ok`, `accepted` and a stable
  `ACCEPT` with exactly its base and bind. The map's declared spec selects the proof form: a 0.x
  `ready-for-ci` line never proves a stable map, and a stable line never proves a 0.x map.
  `constituent-cem-stable-unsupported` is retired.
- `CEM-V1-149`: `ocm prepare`, `ocm read` (status) and `ocm link` MUST admit a `cem/1.0` map only
  when the stable verifier, with the repository root as artifact root, returns `ACCEPT` for the
  expected base and target, and the canonical diff of that base and target has the map's patch
  digest. Every other result fails closed with the verifier's code (prepare, read cause) or the
  existing `invalid-cem` mapping (link); nothing is written. The sidecar need not be committed,
  because the first bind pass runs before the CEM commit.
- `CEM-V1-150`: Local-completion sidecar-only reuse (`AllowCemSidecarOnlyReuse`) MUST keep
  binding only a canonical `cem/0.x` sidecar. A `cem/1.0` sidecar grants no reuse: the check stays
  `selected-check-unverified` until it runs again on the current target. This fails closed and
  costs a rerun; stable reuse needs its own requirement (superseded for the grant path by
  `CEM-V1-205`, V1-1303).
- `CEM-V1-151`: `dogfood finish` MUST read a `cem/1.0` change map: its `cem status` (and the
  status it names in `nextActions`) adds `--artifacts <absolute repository root>` and keeps the
  ceilings (`CEM-V1-145`), and its `cem report` adds the same root and passes no ceilings, which a
  stable report refuses and the preceding status already enforced. A `cem/0.x` map's arguments
  are unchanged.
- `CEM-V1-152`: The aggregate verifier worker's closed argv grammar (`ALO-V0-018`) MUST admit the
  strict check's `cem status` read of a `cem/1.0` change map with a trailing
  `--artifacts <absolute path>` only when that path names the worker's own repository root; any
  other artifact root, a relative one, or `--artifacts` on another read is refused
  `aggregate-worker-invalid` before a verifier runs.

Non-goals: no removal of the `cem/0.x` readers, converter or `--spec cem/0.2` (V1-1278); no
change to `cem begin`'s default, the frozen cli-parity cases, MCP, frontier or conformance
producers; no stable LRF evaluation; no local-completion reuse of a `cem/1.0` CEM (`CEM-V1-150`; V1-1303
adds the grant path, `CEM-V1-205`).

Failure modes: `internal/localcompletion` reads only canonical `cem/0.x` CEMs, so it reports no
reuse for a `cem/1.0` change and fails closed (`CEM-V1-150`; since V1-1303 the stable grant path is
`CEM-V1-205..208`). A structural hunk on a host outside the frozen
structural runtime tuples is the verifier's `UNSUPPORTED`. The change alters the `cem status`
output the loop records, so its own dogfood check declares `DOGFOOD_OUTPUT_CHANGE=DCW-V0-047`
(V1-1279). A stable map with artifacts outside the repository root is not admitted by OCM.

Acceptance evidence:

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-140 | `TestCEMV1140ProducerSpecSelection` (`internal/cem/workflow`) | Library defaults stay `cem/0.2` and `cem/0.1`; invalid prepare and begin specs refuse `invalid-arguments` and write nothing |
| CEM-V1-141 | `TestCEMV1141StablePrepareWritesResumesAndReplaces`, `TestCEMV1141StablePrepareGuardsBackups` (`internal/cem/workflow`) | Writes and verifies `ACCEPT`, resumes byte-identical, refuses a 0.x sidecar as different spec and invalid bytes as not a valid CEM document, regenerates the 0.x sidecar with `--replace`; a bound stable map is refused by stable and 0.x prepare with `--replace`; a producer backup is removed on resume or restored, a bound or unreadable backup is refused and kept |
| CEM-V1-142 | `TestCEMV1142StableBeginWritesProducerMap` (`internal/cem/workflow`) | Two stable begins write the same verifying producer map; a 0.x begin is refused on it |
| CEM-V1-143 | `TestCEMV1143StableProducerMutationsStayStable` (`internal/cem/workflow`); `TestMutatorsRefuseStableInput`, `TestMutatorsRefuseStableDestination` | cite, cover and discriminate keep `cem/1.0` and role `bind-chore` and the result verifies `ACCEPT`; every mutator refuses the bound public map |
| CEM-V1-144 | `TestCEMV1144StableStructuralMark` (`internal/cem/workflow`) | `import-reorder` stays `cem/1.0` and verifies `ACCEPT` within ceilings; `rename` on the same hunk is `REJECT` `unproven-mechanical`, `ok: false` |
| CEM-V1-145 | `TestCEMV1145StatusCeilings` (`internal/cem/cli`) | Within ceilings exit 0, `accepted`, counts and empty policy issues; `--max-mechanical 0` exits 1 with one issue; verify ceilings exit 2 |
| CEM-V1-146 | `TestCEMV1141StablePrepareWritesResumesAndReplaces`; `TestStableStatusArgumentsAddArtifactRoot` (`internal/dogfoodflow`) | `nextActions` names the absolute root; status argv adds it only for a stable map |
| CEM-V1-147 | `TestMapHunksReadsStableMap`, `TestCiteStepAdmitsStableMap`, `TestStableStatusArgumentsAddArtifactRoot`, `TestChangeDCWV0049MarksPreparedStableChoreMap` (`internal/dogfoodflow`) | Hunks and cited count match the parsed map; missing plans keep their reasons; the chore map is marked and never refused; live round trip in `docs/build-log/2026-10-12-v1-1276-dogfood-producer-cem-1-0.md` |
| CEM-V1-148 | `TestBatchCheckAdmitsStableConstituent` (`internal/dogfoodflow`) | Stable ACCEPT admits with `--artifacts`; a 0.x line, a REJECT and another bind fail `constituent-unverified`; a stable line never proves a 0.x map |
| CEM-V1-149 | `TestCEMV1149OCMPrepareAdmitsStableProducerMap`, `TestCEMV1149OCMLinkAdmitsStableProducerMap`, `TestCEMV1149OCMPrepareRefusesUnverifiedStableMap`, `TestCEMV1058LegacyOCMLRFAdmissionUnchanged` (`internal/lrfrepo`) | Prepare binds target and patch digest, status is not invalid, resume works; link records a cited stable hunk and the linked map reads valid; a wrong base and a tampered map write nothing; artifacts outside the root fail closed |
| CEM-V1-150 | `TestCEMV1150StableSidecarGrantsNoReuse` (`internal/localcompletion`) | A committed default (`cem/1.0`) sidecar leaves the check `selected-check-unverified`; sidecar-only reuse bound only a retired `cem/0.x` map, so under `CEM-V1-210` no sidecar grants reuse and its 0.x reuse test is retired; since V1-1303 the test runs without the plan flag and the stable grant path is `CEM-V1-205` |
| CEM-V1-151 | `TestDogfoodFinishRunsFromBinaryInForeignRepository`, `TestTraceGitignoreFinishThenChangeCheckSeal` (`cmd/corvint`) | The default prepare writes `cem/1.0`; finish reports, is reviewed and is satisfied, and the next change checks and seals |
| CEM-V1-152 | `TestAggregateVerifierStableArtifactRoot` (`internal/dogfoodflow`); `TestAggregateFinishNativeForeignRepository`, `TestTransportAdaptedRecoveryNative` (`cmd/corvint`) | The worker root, or another spelling of it, is admitted; another, relative or NUL-bearing root, a misspelled flag, an extra flag and `--artifacts` on an OCM read are refused; the aggregate finish over a default `cem/1.0` map passes its final check |

Rollback: revert the V1-1276 change. `cem prepare` again writes `cem/0.2` and refuses a stable
sidecar, the loop and batch-check again refuse `cem/1.0`, and OCM again refuses it. A `cem/1.0`
map already sealed stays valid for `cem status`/`cem verify-stable`, but the reverted
`batch-check` refuses it, so a batch over it needs this change or a rebind under `--spec
cem/0.2`. No wire, schema, packet or frozen case changes.
## Closure-chain stable consumers (V1-0632 C06..C08)

Status: accepted under the owner's standing auto-accept (2026-10-11) after independent review
(Codex round 2 PASS); delivery stays library-level plus two explicit CLI flags, unpromoted. Native work: V1-0632, migration rows C06 (TCQ/frontier), C07 (obligation
sidecar) and C08 (criterion experiment companion) of `protocol/cem-1.0/stable/MIGRATION.json`,
built on the C05 admission (`CEM-V1-051..059`). Each family gains a stable route beside its
unchanged 0.x route. Every stable route admits a `cem/1.0` map only through the shipped stable
verifier's `ACCEPT` over the exact bytes, with an explicit repository, expected base, target and
artifact root, and with the exact committed sidecar (`EXACT`). Every emitted document carries a
distinct stable profile; no 0.x profile, identity domain, default or emitted byte changes, and no
0.x document is relabeled. `MIGRATION.json` is digest-pinned by
`protocol/cem-1.0/stable/manifest.json` and is not edited here; its rows stay
`OPEN_NOT_IMPLEMENTED_BY_THIS_PROPOSAL` until that packet is regenerated.

- `CEM-V1-120`: `tcq.EvaluateStable` MUST emit a distinct profile `tcq-stable/0`, whose identity
  has the prefix `tcq-stable:sha256:` and is hashed in the domain `corvint-tcq-stable/0`. `tcq/0`
  stays frozen: it keeps admitting `cem/0.2` only, with its identity domain unchanged.
- `CEM-V1-121`: The stable TCQ stage 3 MUST admit only a bounded object declaring exactly
  `cem/1.0`. Its upstream stage MUST be a `StableUpstreamVerifier`, whose refusal passes through
  unchanged, with no result. `tcq.Evaluate` and the `frontier/0` upstream MUST keep refusing a
  `cem/1.0` map, and the stable route MUST refuse `cem/0.2`.
- `CEM-V1-122`: The stable result MUST carry:
  - the stable profile;
  - `cem/1.0`;
  - the accepted map and patch digests;
  - an identity that differs from the `tcq/0` identity of the same computation.

  Claims, units, issues and relations MUST come from the unchanged `tcq/0` machinery. A
  supplied cached result MUST verify byte for byte, and an edited cached result refuses with a
  typed TCQ code.
- `CEM-V1-123`: The verifier binding MUST name the SHA-256 of the exact input map bytes, and the
  OCM MUST bind that map and the accepted patch digest; anything else refuses. The
  `frontierrepo` upstream seam MUST return its memoized accepted binding only for the exact
  verified CEM and OCM bytes, by digest, and the accepted revisions. Any other input runs a real
  stable verification.
- `CEM-V1-124`: The 0.x TCQ and frontier routes are unchanged. `parseArtifacts` keeps the
  `tcq/0` stage-3 check through `parseArtifactsWith`, and the existing `internal/tcq`, `internal/frontier` and
  `internal/frontierrepo` suites pass unchanged.
- `CEM-V1-125`: `frontierrepo.RecomputeStable` MUST:
  1. admit the map and OCM only through `lrfrepo.VerifyStableUniverse`;
  2. open the target under the stable repository boundary;
  3. recompute TCQ under `tcq-stable/0` against the same accepted map;
  4. revalidate the boundary.

  It returns the stable universe, its obligations carried field for field, the stable TCQ result
  and its per-edge claims. A declared criterion link or a caller discrimination witness MUST NOT
  match a TCQ claim or grant adequacy.
- `CEM-V1-126`: No stable frontier document is produced. `frontier/0` binds an `lrf/0` digest, and
  no stable LRF or frontier profile is accepted, so the fail-closed choice is
  `StableFrontierState` `NOT_PRODUCED`. That state is never `EMPTY` or closed. Unknown obligations
  cross with their reasons, and nothing closes them.
- `CEM-V1-127`: Each of the following MUST refuse, keeping the stable verifier's full result and
  returning no upstream:
  - a wrong base;
  - edited map bytes;
  - a tampered artifact;
  - a missing artifact root.
- `CEM-V1-130`: `extevidence.StableObligations` MUST emit `external-frontier-obligations-stable/0`,
  with an identity prefix of `obligations-stable:sha256:`. Its `binding` names:
  - `cem/1.0`;
  - the exact map SHA-256, the accepted patch digest, base and target, and the excluded path;
  - the verifier outcome;
  - the impact receipt digest and tool.

  `external-frontier-obligations/0` stays frozen and admits `cem/0.2` only.
- `CEM-V1-131`: Hunk associations MUST be computed exactly as in the frozen sidecar, with the same
  join, bound, freshness and state rules. Every unknown association, including
  `no-external-section`, is kept.
- `CEM-V1-132`: A verifier refusal MUST return a `StableObligationsRefusal` that holds the
  complete result. It unwraps to a kernel error carrying the verifier code and produces no
  document. A sidecar other than `EXACT`, or an accepted result that does not bind the exact map
  digest, base and patch, MUST refuse with `invalid-obligations-input`.
- `CEM-V1-133`: The map's declared criterion links MUST be listed in a separate `criteria` member,
  each giving its criterion, ticket, hunks, evidence and runner receipts with satisfaction
  `NOT_OBSERVED`. They are never associations and close no obligation.
- `CEM-V1-134`: The frozen sidecar MUST keep refusing `cem/1.0`, and the stable sidecar MUST refuse
  a `cem/0.x` map before verification, both with `invalid-obligations-input`.
- `CEM-V1-135`: `corvint obligations` MUST select the stable sidecar only when all four of these
  flags are given:
  - `--repository`
  - `--expected-base`
  - `--target`
  - `--stable-artifact-root`

  A partial set is an argument error, with exit 2. A stable refusal is an error envelope with
  exit 2 and no stdout. Without the flags, the verb is the unchanged frozen sidecar, which reads
  no repository.
- `CEM-V1-136`: The criterion experiment companion MUST add the explicit input mode
  `cem-criterion-experiments-stable/0`. A request with that schema yields a plan, receipt and
  summary with the same schema, and a plan never changes mode between phases. A
  `cem-criterion-experiments/0` request keeps its exact frozen behavior and output bytes.
- `CEM-V1-137`: Stable admission MUST:
  - require bytes declaring `cem/1.0`;
  - run `verify.Stable` with the repository, the request base and target, and the explicit
    artifact root;
  - require `ACCEPT`, an `EXACT` sidecar, and the exact map digest, base, target and patch;
  - require every criterion hunk to be a hunk of the accepted change.

  It MUST NOT read the map's criterion links. Criterion authority stays the native Tasks capture
  and verifier, as in the frozen mode.
- `CEM-V1-138`: Plan, run and verify MUST each re-admit the plan's map through the stable verifier
  with the invocation's artifact root, which is never plan data. The frozen phases MUST refuse a
  stable plan. A missing artifact root or a tampered artifact MUST refuse verification with
  `StableRefusal`. That refusal keeps the complete result and names the verifier code, else its
  first issue code, else `stable-verification-failed`.
- `CEM-V1-139`: The two modes never cross:
  - the frozen mode refuses `cem/1.0` and any artifact root;
  - the stable mode refuses `cem/0.2`;
  - a stable request without an artifact root refuses.

  `corvint-cem-experiments --stable-artifact-root DIR` selects the stable phases and prints no
  payload on refusal.

Failure modes:
- A host outside the frozen structural runtime tuples gets the verifier's `UNSUPPORTED` refusal,
  which every route keeps as a refusal.
- A target that commits no sidecar can be `ACCEPT`/`ABSENT` at the verifier, but every stable
  route here refuses it, which is fail-closed.
- The stable TCQ route does not apply the frontier adapter's Python grammar-failure reason rewrite
  (TCQ-V0-047). Such a claim stays abstained with the generic `unsupported-anchor-profile` reason.
- No route promotes, sets a default, or has an MCP surface.

Rollback:
- Remove `internal/tcq/stable.go`, `internal/frontierrepo/stable.go`,
  `internal/extevidence/obligations_stable.go`, `internal/criterionexperiment/stable.go` and the
  test-only `internal/cem/stablefixture`.
- Restore `parseArtifacts`, the companion's single-schema checks and the
  `--stable-artifact-root` flag, and remove the four `obligations` stable flags with their help
  text.

No 0.x profile, default, packet or stored evidence changes.

### Closure-chain stable consumer traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-120 | `TestCEMV1122StableEnvelope` | `tcq-stable/0` profile and `tcq-stable:sha256:` identity in the stable domain |
| CEM-V1-121 | `TestCEMV1121StableAdmissionRefusals`, `TestCEMV1121LegacyRoutesRefuseStable` | `cem/0.2` never enters the stable route; verifier refusal passes through; `tcq/0` and `frontier/0` refuse `cem/1.0` |
| CEM-V1-122 | `TestCEMV1122StableEnvelope`, `TestCEMV1125StableUpstreamAccepts` | Map and patch digests bound; identity differs from `tcq/0`; cached result verifies and an edited one refuses |
| CEM-V1-123 | `TestCEMV1121StableAdmissionRefusals`, `TestCEMV1123StableTCQMemoNeverBypasses` | Binding or OCM digest not the exact map refuses; memo answers only the exact verified bytes |
| CEM-V1-124 | `TestCEMV1121LegacyRoutesRefuseStable` | Frozen routes unchanged; existing `internal/tcq`, `internal/frontier`, `internal/frontierrepo` suites pass |
| CEM-V1-125 | `TestCEMV1125StableUpstreamAccepts` | Verified change yields `tcq-stable/0` bound to map and patch; caller claim stays unmatched |
| CEM-V1-126 | `TestCEMV1125StableUpstreamAccepts`, `TestCEMV1126StableUpstreamKeepsUnknownObligation` | `NOT_PRODUCED` frontier; unknown obligation keeps its reason |
| CEM-V1-127 | `TestCEMV1127StableUpstreamRefusals` | Wrong base, edited bytes, tampered artifact and missing root refuse with the result kept |
| CEM-V1-130 | `TestCEMV1130StableObligationsAccepts` | Stable schema, identity prefix and exact binding; no frozen schema bytes |
| CEM-V1-131 | `TestCEMV1130StableObligationsAccepts`, `TestCEMV1131StableObligationsKeepUnknowns` | Same association kinds as the frozen join; `no-external-section` kept |
| CEM-V1-132 | `TestCEMV1132StableObligationsRefusals` | Wrong base, edited bytes, missing root and tampered artifact refuse with the result kept and a kernel code |
| CEM-V1-133 | `TestCEMV1130StableObligationsAccepts` | Criterion link listed apart with `NOT_OBSERVED`; no association carries a criterion |
| CEM-V1-134 | `TestCEMV1134ObligationRoutesStayDisjoint` | Frozen refuses `cem/1.0`; stable refuses `cem/0.2`; both `invalid-obligations-input` |
| CEM-V1-135 | `TestCEMV1135ObligationsStableMode` | Accept over the four flags; wrong base exit 2 with no stdout; partial flags exit 2; frozen form refuses `cem/1.0` |
| CEM-V1-136 | `TestCEMV1136StableExperimentRunsAndVerifies` | The planning core (`planWithCaptures`, after stable admission, over verified native captures) writes a stable plan and marker; receipt and summary carry the stable schema; `VERIFIED_SATISFIED` over the stable map digest. The live `capture` call is not exercised |
| CEM-V1-137 | `TestCEMV1137StableExperimentAdmission` | Accepts the verified map; wrong base, edited bytes, missing root, tampered artifact and unknown hunk refuse |
| CEM-V1-138 | `TestCEMV1136StableExperimentRunsAndVerifies` | Frozen run and verify refuse the stable plan; missing root and tampered artifact refuse stable verification |
| CEM-V1-139 | `TestCEMV1137StableExperimentAdmission`, `TestCEMV1139StableFlagRefusesWithoutPayload` | Frozen with root, frozen over `cem/1.0`, stable over `cem/0.2` and stable without root refuse; CLI flag prints no payload on refusal |

## Stable projection routes (V1-0632 C13..C17)

Status: accepted under the owner's standing auto-accept (2026-10-11) after independent review
(Codex round 2 PASS); delivery is explicit flags and one explicit corpus operation, unpromoted.
Native work: V1-0632, migration rows C13 (documentation corpus), C14
(witness and collapse), C15 (capsule closure), C16 (console chain pane) and C17 (dashboard) of
`protocol/cem-1.0/stable/MIGRATION.json`, built on the library projections of S5c
(`CEM-V1-070..079`). Each route admits a `cem/1.0` map only on the shipped stable verifier's
`ACCEPT` over the exact bytes, against an explicit expected base, target and artifact root. A wrong
base, edited map bytes, a tampered artifact or a missing artifact root refuses with no projection.
Every 0.x route, profile and default is unchanged and stays until V1-1278; no 0.x document is
relabeled. `MIGRATION.json` is digest-pinned and is not edited here.

- `CEM-V1-160`: `corvint witness --stable-artifact-root DIR` MUST pass the absolute artifact root to
  the witness compiler, which verifies the head tree's `cem/1.0` map with the repository, `--base` and
  the checked-out HEAD as target. The command MUST print a report only when the source is `BOUND`
  under `corvint-witness-stable/0`, carrying the complete verifier result.
- `CEM-V1-161`: The witness route MUST fail closed with exit 2, no stdout and the code
  `stable-witness-refused` when:
  - a `cem/1.0` source is anything but `BOUND` (a wrong base, edited map bytes, a tampered artifact,
    or a missing or absent artifact root);
  - `--stable-artifact-root` is given over a source that is not `cem/1.0`, including an absent map.

  Without the flag a 0.x or absent map keeps the `corvint-witness/0` report unchanged.
- `CEM-V1-162`: `corvint docs corpus cem-stable` MUST require `--artifact`, `--cem`,
  `--expected-base`, `--target` and `--stable-artifact-root`, with an optional `--id`. It verifies
  with `--root` as the repository and prints `corvint-corpus-cem-stable/1` only when the verifier
  accepts the exact map bytes. The artifact root is resolved as an absolute directory and is never a
  `--root`-relative local input.
- `CEM-V1-163`: A `cem-stable` refusal MUST exit 2 with no stdout and the code
  `corpus-stable-refused`. A partial binding MUST be an argument error. The 0.x `cem` operation MUST
  keep refusing a `cem/1.0` map.
- `CEM-V1-164`: `corvint-console -stable-artifact-root DIR` MUST select the chain pane's verified
  mode. The pane runs the stable verifier over the specs repository, with the request's `base` as
  expected base, the sealed map's bind commit as target and the operator's artifact root. It MUST
  draw a `cem/1.0` chain only on an `ACCEPT` whose map digest is the SHA-256 of the sealed bytes. The
  accepted chain shows:
  - the verifier outcome on its envelope;
  - the verifier's sidecar state;
  - the verifier's change- and reference-integrity axes.

  The NOT_OBSERVED authority, criterion, runner and authentication axes stay gaps.
- `CEM-V1-165`: In verified mode, the pane MUST replace the chain with a refusal, drawing no chain
  row, when any of these holds:
  - the `base` is missing or malformed: the refusal is an argument refusal, raised before the
    verifier runs, that asks for `base=<full commit>`;
  - the verifier returns anything but `ACCEPT`, including for a wrong base, a missing artifact
    root, a tampered artifact or edited sealed bytes: the refusal names the verifier outcome, stage
    and code;
  - the verifier does not finish within the console's `-timeout`: the refusal names the timeout.

  An accepted chain's hunk links MUST carry the accepted `base`, so following them stays in
  verified mode. Its reference rows MUST show the verifier's `referenceIntegrity` value in place of
  the display-only "not checked" text, and keep the not-completion qualification.
- `CEM-V1-166`: Without `-stable-artifact-root`, the display-only NOT_RUN pane of `CEM-V1-077` MUST be
  unchanged. Verified mode MUST NOT change how a 0.x sealed map is drawn.
- `CEM-V1-167`: The protected local-authority consumer holds a Git object capsule and no
  repository, so it cannot run the stable verifier. It MUST keep refusing an enrollment whose
  `cem.json` declares `cem/1.0`, with `capsule CEM base`, before requesting any object.
  Amended by V1-0632 (`CEM-V1-295..299`): the capsule now admits an exact `cem/1.0` map whose base
  is the enrolled base, derives its object demand from it and leaves verification to the stable
  verifier over the published evidence view; `capsule CEM base` remains the stale-base refusal.
- `CEM-V1-168`: The dashboard's hash-pinned adapter registry declares no `cem/1.0` profile. So
  `corvint-dashboard-snapshot --cem-ocm-profile` naming `cem/1.0` MUST be
  `DASHBOARD_INVALID_ARGUMENT`, with no stdout, before any path opens. The `cem-ocm-bundle-v0`
  registration MUST keep only its 0.x profiles.
  Amended by V1-1278 (decision 0525 §1, `CEM-V1-217`): the `cem-ocm-bundle-v0` registration and
  its five bundle flags are retired. Every bundle flag and profile, `cem/1.0` and 0.x alike, is
  `DASHBOARD_INVALID_ARGUMENT` with no stdout before any path opens, and the hash-pinned registry
  has no row that names a `cem/` profile.

Failure modes:
- A host outside the frozen structural runtime tuples gets the verifier's `UNSUPPORTED` refusal.
  Every route keeps it as a refusal.
- The console's verified mode needs the operator to add `base=<full commit>` to the chain URL. The
  change listing does not carry it, so a click without it refuses.
- One console artifact root serves every sealed change. A change whose artifacts are elsewhere
  refuses.
- Superseded by V1-0632 (`CEM-V1-290..299`): the dashboard has a separate `cem-stable` read and
  the capsule a stable closure; the bundle registry stays retired. The original text follows.
  The capsule verifier and the dashboard bundle stay without a stable path (`CEM-V1-167`,
  `CEM-V1-168`). Each needs its own accepted contract:
  - the capsule needs an enrollment format carrying the stable artifact roles and a capsule-native
    stable verifier;
  - the dashboard needs a new registry version or registration, verification over the C05 stable
    universe, snapshot fields for the axes and bindings, and new conformance vectors.
- `witnesscollapse.CompileStable` has no command. The collapse contract lists a CLI as a non-goal,
  so it stays library-level. No witness, corpus-CEM or console MCP tool exists, so none is added.

Rollback:
- Remove the `--stable-artifact-root` witness flag and `admitStableWitness`.
- Remove the `cem-stable` corpus operation and its three flags.
- Remove `Options.StableArtifacts`, `VerifyStableChain` and the console flag.
- Remove the five tests named below.

No 0.x profile, default, packet or stored evidence changes.

### Stable projection route traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-160 | `TestCEMV1160WitnessStableRoute` | Accepted map prints `corvint-witness-stable/0` with `BOUND`, `ACCEPT` and the exact map digest |
| CEM-V1-161 | `TestCEMV1160WitnessStableRoute` | Wrong base, missing and absent root, tampered artifact, edited map and flag without a stable map exit 2 with no stdout; 0.x report unchanged without the flag |
| CEM-V1-162 | `TestCEMV1162CorpusStableOperation` | Accepted map prints `corvint-corpus-cem-stable/1` with `PROJECTED`, `ACCEPT`, the map digest and claim rows |
| CEM-V1-163 | `TestCEMV1162CorpusStableOperation` | Wrong base, edited map, missing root and tampered artifact exit 2 with no stdout; partial binding refused; 0.x `cem` refuses `cem/1.0` |
| CEM-V1-164 | `TestCEMV1164ConsoleVerifiedStableChain` | Verified chain shows the `ACCEPT` envelope, `changeIntegrity VERIFIED`, `sidecar EXACT` and keeps NOT_OBSERVED axes |
| CEM-V1-165 | `TestCEMV1164ConsoleVerifiedStableChain` | Missing or malformed base, wrong base, missing root, tampered artifact, edited sealed bytes and a verifier timeout draw only a refusal; a followed hunk link stays verified; reference rows show the verifier value |
| CEM-V1-166 | `TestCEMV1164ConsoleVerifiedStableChain` | Without the root the NOT_RUN envelope renders unchanged |
| CEM-V1-167 | `TestCEMV1167CapsuleRefusesStableEnrollment` | As amended by `CEM-V1-295`: a matching-base `cem/1.0` enrollment reaches the object demand, which refuses an empty capsule; a 0.x map refuses `unsupported-spec` with the fix line |
| CEM-V1-168 | `TestCEMV1168DashboardRefusesStableBundle` | Every retired bundle flag and every profile, `cem/1.0` and 0.x, refuses with `DASHBOARD_INVALID_ARGUMENT` and no stdout; no registry row is `cem-ocm-bundle-v0` or names a `cem/` profile |

## Consumers read cem/1.0 only (V1-1278 S2a, decision 0525 §1 and §6, 2026-10-12)

Status: proposed; V1-1278 stage S2a. S1 (`CEM-V1-200..204`) retired the converter and made the
sealed-change admission `cem/1.0`-only, but left the `cem/0.x` wire in place because the consumers
listed in `docs/build-log/2026-10-12-v1-1278-s1-remove-0x-core.md` still read or wrote it. This
stage moves every one of those consumers to `cem/1.0`, so that no code outside `internal/cem`
calls `wire.Spec01..03`, `wire.ParseMap`, `wire.Canonical`, `verify.Exact` or `verify.Canonical`,
and S2b can delete them. The `internal/cem` 0.x code, the `cem begin`/`--spec cem/0.2` producers
and the 0.x `cem` CLI routes stay in this stage.

- `CEM-V1-210`: Every consumer outside `internal/cem` (LRF, OCM, the stable universe, frontier
  and its `frontierrepo`/`frontiernextrepo` adapters, external frontier obligations, `prove`,
  attest predicates, anchor, the console chain pane, the MCP report preview, the receipt bundle,
  witness and witness collapse, the documentation corpus, the criterion experiment, the CEM
  candidate assembler, local completion, `tools/local-authority` and
  `tools/cem-citation-baseline`) MUST admit a map only as `cem/1.0`. A map whose `spec` is
  `cem/0.1`, `cem/0.2` or `cem/0.3` MUST refuse with `unsupported-spec` and, where the consumer's
  error shape carries text, the `CEM-V1-200` recovery line (`wire.RetiredSpecFix`). It MUST
  never be an internal error, a silent fallback to another reader, or a pass. Three routes keep
  the code an accepted requirement already assigns: `prove --attest-cem-v2` refuses with
  `attest-cem-v2-requires-stable-map` (`CEM-V1-065`) and `docs corpus cem-stable` with
  `corpus-stable-refused` (`CEM-V1-076`), each naming the recovery line in its message, and the
  MCP report preview maps `unsupported-spec` into its closed, text-free failure set as
  `cem-map-invalid`. A signed `cem/0` or `cem/v1` attestation still verifies as a claim without
  map bytes (`NOT_RUN`), and refuses every supplied map with `unsupported-spec`. (Superseded since
  V1-1278 S2b by `CEM-V1-218`, under which a `cem/0` or `cem/v1` envelope is a retired predicate and
  never verifies.) A frozen 0.x
  profile whose result was defined over a 0.x map (`lrf/0`, `frontier/0`, `tcq/0`,
  `external-frontier-obligations/0`, the attest v0/v1 predicates) now admits no map: a 0.x map
  refuses as above, and a `cem/1.0` map refuses with `unsupported-spec` or takes that profile's
  existing stable route. This supersedes the "0.x maps keep their reading" clause of
  `CEM-V1-079`, and every other clause in this file or a consumer spec that keeps a 0.x reading
  for a consumer.
- `CEM-V1-211`: The frontier, LRF/OCM, dashboard, console and MCP projections MUST read the
  stable map through the existing stable routes (`wire.ParseStable`, `verify.VerifyStable` and the
  stable read in `internal/cem/workflow`) and MUST NOT project a 0.x map. The MCP report preview
  admits a map only when the stable reader does; a retired map refuses before any blob is read.
- `CEM-V1-212`: A frozen conformance suite whose expectations depend on a 0.x map changes only
  through its own contract, and the superseded freeze is recorded under decision 0525 §6:
  `conformance/ocm-v0` regenerates its structural vectors from the real `cem/1.0` producer (only
  `mapSha256` changes); `conformance/frontier-v0` keeps its hand-authored expectations and skips
  every case that needs a 0.x map to verify through the declared capability
  `verified-retired-cem`, while `TestRetiredCEMUniverseRefusesThroughTheEntryPoint` drives the
  refusal for real; `conformance/cli-parity-v0` retires `lrf-evaluate-supported` with a pinned
  `retired` declaration under decision 0525 (`retiredCaseDecisions`), so replay prints `RETIRED
  ... decision=0525` and `SUMMARY parity=103 retired=30`; the `internal/lrf` conformance corpus
  keeps its frozen bytes, runs its validator cases unchanged and skips, as superseded, the four
  cases that read a 0.x map through the retired repository route (`ocm.no-links`,
  `context.01-explicit`, `context.01-default`, `failure.structural`; recorded in
  `docs/specs/lexical-relevance-floor-v0.md`), adding no 0.x consumer path; the core freeze
  retires its open `frontier --json` golden, because no admitted map produces `frontier/0`
  (`CEM-V1-210`), and keeps its 0.x `cem status`/`cem verify` goldens until the 0.x route goes. No
  suite regenerates an expectation from candidate output outside its own regeneration contract.
  Amended by V1-1278 S2b (`CEM-V1-217`, `CEM-V1-219`): with the `frontier/0` engine and the
  `lrf/0` repository route gone, the frontier-v0 fixture tier and the lrf-v0 corpus are retired
  whole rather than skipped, cli-parity retires `cem-begin-map` too (`SUMMARY parity=102
  retired=31`), and the core freeze's `cem status`/`cem verify` goldens are regenerated from a
  `cem/1.0` map.
- `CEM-V1-213`: The daily loop's admission (`CEM-V1-202`) stays `cem/1.0`-only on every route a
  consumer reaches: the bind loop gives a chore role only to a map that declares `cem/1.0`
  (`markChoreRole` refuses a 0.x map and leaves it byte-identical), the verifier's `cem status`
  gets `--artifacts` only for a stable map, and a sealed 0.x constituent is a retired seal
  (`CEM-V1-203`), never a verified one.
- `CEM-V1-214`: A test that guards behavior `cem/1.0` keeps MUST be ported to a `cem/1.0` map
  built by the stable producer (`workflow.PrepareOptions{Spec: wire.StableSpec}` plus `Cite`) or
  by `internal/cem/stablefixture`, and keep its assertion. A test that only guarded 0.x behavior
  MUST be deleted, and a test that fed a 0.x map to a consumer becomes a refusal test under the
  admission rule `CEM-V1-210`. A test MUST NOT relabel a 0.x map as `cem/1.0` to reach a stable
  route.

Non-goals: no removal of `wire.Spec01..03`, `wire.ParseMap`, `verify.Exact`, `verify.Canonical`,
`patch`, the 0.x producers in `internal/cem/workflow` or the 0.x `cem` CLI routes and their
goldens (S2b); no change to `protocol/`, `interop/` or the frozen `cem-0.1` packet (S3); no
rewrite of a committed map; no new stable LRF or frontier profile.

Failure modes: a consumer that still decodes a 0.x map fails the acceptance search recorded in
`docs/build-log/2026-10-12-v1-1278-s2a-consumers-cem-1-0.md`; a frozen profile that cannot be
produced from a `cem/1.0` map refuses rather than emitting a partial document; a frontier-v0
runner that verifies a 0.x universe again fails the retired-universe entry-point test (itself
retired with the fixture tier by `CEM-V1-217`; the manifest retirement record now guards it).

Rollback: revert the V1-1278 S2a commits. The 0.x wire and verifier are still in `internal/cem`,
so the reverted consumers compile and read 0.x maps again; no committed map, seal or task record
changes.

### Consumer cem/1.0 admission traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-210 | `TestCEMV1210LRFRefusesEveryMapBeforeAnyInput`, `TestCEMV1210OCMRefusesRetiredMap`, `TestCEMV1210FrontierRefusesARetiredMap`, `TestCEMV1210ProveRefusesEveryMapOutsideTheStableRoute`, `TestCEMV1210FrozenObligationsAdmitNoMap`, `TestCEMV1210AnchorRefusesARetiredMap`, `TestCEMV1218RetiredPredicatesRefuse`, `TestCEMV1210FixtureRecomputeRefusesEveryMap`, `TestAssembleRefusesRetiredAndStableSources`, `TestExportRefusesRetiredMaps`, `TestReadCEMSourceRefusesRetiredMap`, `TestConsoleChainStableRegistry`, `TestStableCollapseRefusals`, `TestStableCEMProjectionRefusals`, `TestBaselineDCWV0049SplitsRowsByCitationClass`, `TestCEMV1150StableSidecarGrantsNoReuse`, `TestCEMV1137StableExperimentAdmission`, `TestCEMV1065RefusesALegacyMap`, `TestCEMReportToolNamesStableRefusals` | Each retired spec refuses with `unsupported-spec` (and the recovery line where the error carries text); no consumer reads a 0.x map |
| CEM-V1-211 | `TestCEMReportToolPreviewsCLIReportWithoutPublishing`, `TestCEMReportToolNamesStableRefusals`, `TestContextAndCEMReportAreBoundReadOnlyAndFramed`, `TestCEMV1058LegacyOCMLRFAdmissionUnchanged`, `TestConsoleChainPanelsAgreeOnObligationHunkEdges` | The MCP preview, OCM/LRF universe and console read the stable map; a retired map refuses |
| CEM-V1-212 | `TestFrozenVectorsMatchTheRealProducer`, `TestManifestPinsRetirementAndArtifacts`, `TestCEMV1210FrontierRefusesARetiredMap`, `TestDecision0308DeclarationsAreClosed`, `TestGPKV0005ManifestWorkerDefault`, `TestCoreVerbsEmitTheFrozenProfiles` | ocm-v0 vectors refrozen from the `cem/1.0` producer; cli-parity retires its cases under decision 0525; the core freeze retires its open-frontier golden. Since V1-1278 S2b (`CEM-V1-217`, `CEM-V1-219`) the frontier-v0 fixture tier and the lrf-v0 corpus are retired whole, so their skip and corpus tests are deleted and the retirement manifest is pinned instead |
| CEM-V1-213 | `TestMarkChoreRoleDCWV0049WritesStableRoleOnly`, `TestStableStatusArgumentsAddArtifactRoot`, `TestBatchCheckCEMV1203RetiredSealHasNoCoverage` | A 0.x map gets no role and no `--artifacts`; a retired seal is not reverified |
| CEM-V1-214 | `TestProveCEMAcceptedMapProvesSupportedHunksAndWritesNothing`, `TestAnchorWritesAVerifiedPointerAndReadsItBack`, `TestHistoricalVerifyArtifactsAndSurvivors`, `TestFrozenVectorsMatchTheRealProducer` | Ported tests build their map with the stable producer or `stablefixture` and keep their assertions |

## Core 0.x removal (V1-1278 S2b, decision 0525 §1 and §6, 2026-10-12)

Status: proposed; V1-1278 stage S2b. S2a (`CEM-V1-210..214`) moved every consumer outside
`internal/cem` to `cem/1.0`. This stage deletes what that left: the `cem/0.1`..`cem/0.3` wire
(`wire.Spec01..03`, `wire.ParseMap`, `wire.ParseMapSpec`, `wire.Canonical`), the 0.x verifier
(`verify.Exact`, `verify.Canonical`), the 0.x producers in `internal/cem/workflow`, the 0.x `cem`
CLI routes and their goldens, the frozen 0.x projection routes and the attest `cem/0`/`cem/v1`
predicates. The mechanics `cem/1.0` inherits (`wire.Map`, structural and mechanical reasons, the
coverage and discrimination witnesses, `internal/cem/patch` and the canonical change derivation in
`internal/cem/verify/verify.go`) stay, because the stable reader, producer and verifier use them.
Owner standing auto-accept of lane requirements (2026-10-11) applies; the requirements stay
proposed until recorded under that acceptance.

- `CEM-V1-215`: The product MUST ship one CEM wire, `cem/1.0`. No Go package outside the frozen
  S3 packets (`protocol/`, `interop/`, `examples/cem`, `conformance/cem-0.1`) may parse,
  canonicalize, verify or produce a `cem/0.x` map. Every Core read (`cem status`, `cem verify`,
  `cem report`, `cem mark`, the daily loop) MUST refuse a root whose `spec` is not exactly
  `cem/1.0` with `unsupported-spec` before any Git read, and a retired `cem/0.x` root MUST carry
  the `wire.RetiredSpecFix` recovery line (`CEM-V1-200`). The inherited mechanics keep their
  `cem/0.3` meaning without a spec argument: structural reasons are always admitted.
  Amended by V1-1278 S3 (`CEM-V1-220`): the S3 packets are deleted, so no Go package in the
  repository parses a `cem/0.x` map; the portable consumer refuses one (`CEM-V1-222`).
- `CEM-V1-216`: `cem begin` and `cem prepare` MUST accept only `--spec cem/1.0` (the default). A
  retired spec label (`cem/0.1`, `cem/0.2`, `cem/0.3`) MUST refuse with `unsupported-spec` and
  the recovery line; any other value, the experimental candidate included, MUST refuse with
  `invalid-arguments`. A refused begin MUST leave the existing map byte-identical.
- `CEM-V1-217`: The frozen 0.x projection routes MUST be retired, each with its stable successor
  named in its own contract: `frontier/0` by `frontier-stable-view/0` (`CEM-V1-125`, V1-1304);
  `tcq/0` by `tcq-stable/0` (`CEM-V1-120..124`); `external-frontier-obligations/0` by
  `external-frontier-obligations-stable/0` (`CEM-V1-130`); and `lrf/0` evaluation, which has no
  stable successor, by the stable universe admission only (`CEM-V1-051..059`), so `lrf evaluate`
  refuses every map. A retired route MUST refuse with `unsupported-spec` in its existing closed
  error shape; it MUST NOT emit a partial document. The frozen suites that only exercised a
  retired route (the frontier-v0 fixture tier and the lrf-v0 corpus) are retired whole with a
  pinned retirement record rather than skipped case by case.
- `CEM-V1-218`: Only the `cem/v2` attestation predicate (`CEMStatementV2`, `CEM-V1-065`)
  remains. A `cem/0` or `cem/v1` envelope MUST fail verification with an error naming
  `cem/v2` as the only CEM predicate (CLI `attest-verification-failed`), whether or not a map is
  supplied. The `prove --attest-cem` and `--attest-cem-v1` flags are removed and MUST be
  refused as unrecognized arguments (`invalid-arguments`). This supersedes the `NOT_RUN`
  clause of `CEM-V1-210` and the v0/v1 clauses of `FPK-V0-030..035` and `FPK-V0-050`.
- `CEM-V1-219`: The core compatibility freeze and cli-parity-v0 MUST change only under decision
  0525 §6, which supersedes decision 0357 for these cases: the core freeze's `cem status` and
  `cem verify` goldens are regenerated from a `cem/1.0` map read with `--artifacts`
  (`stableVerification.spec` = `cem/1.0`), and cli-parity retires `cem-begin-map` with a pinned
  `retired` declaration (`SUMMARY parity=102 retired=31`). No other frozen expectation changes.

Non-goals: no change to `protocol/cem-0.2`, `protocol/cem-0.3`, `interop/cem-0.1`,
`interop/cem01-go`, `conformance/cem-0.1`, `examples/cem`, `tools/cem-interop-runner` or the 0.x
mode of `tools/cem-citation-baseline` (S3); no rewrite of a committed map, seal or task record;
no new stable LRF evaluation; no new attestation predicate. Amended by V1-1278 S3
(`CEM-V1-220..224`): those S3 paths are now deleted or `cem/1.0`-only.

Failure modes: a reintroduced 0.x parser or verifier call fails the acceptance search recorded in
`docs/build-log/2026-10-12-v1-1278-s2b-core-removal.md`; a 0.x map handed to any Core read or
producer refuses with `unsupported-spec` and the recovery line instead of an internal error; a
retired attestation envelope fails closed; a retired frozen suite that is edited without its
retirement record fails its manifest test.

Rollback: revert the V1-1278 S2b commit. The S2a consumers already read `cem/1.0` only, so the
revert restores the 0.x code without changing any committed map, seal, golden outside this change
or task record.

### Core 0.x removal traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-215 | `TestCEMV1200RetiredSpecRefusedWithMergeFix`, `TestCEMV1215ReadsRefuseEveryNonStableSpec`, `TestCoreStableReadExplicitRefusals`, `TestMarkRefusesARetiredMapBeforeMutation`, `TestInheritedStructuralReasons`, `TestInheritedExcludedPath`, `TestInheritedCoverageWitness`, `TestInheritedDiscriminationWitness`, `TestMapArmsRefuseTheRetiredWire`, `TestSidecarBaseRevisionReadsAnyLayout` | Core reads refuse every non-stable root before a Git read; the inherited mechanics keep their behavior without a spec; the trial tool's map arms refuse; the dogfood check reads a stable sidecar's base revision (V1-1326) |
| CEM-V1-216 | `TestCEMV1140ProducerSpecSelection`, `TestCEMV1142StableBeginWritesProducerMap`, `TestCEMV1216CommandSpecRefusals` | Retired labels refuse with the fix line, other values with `invalid-arguments`; a refused begin leaves the map unchanged; the command parser leaves the judgement to the producer |
| CEM-V1-217 | `TestCEMV1210FrontierRefusesARetiredMap`, `TestCEMV1210FrozenObligationsAdmitNoMap`, `TestObligationsCommandRefusesRetiredMap`, `TestUpstreamProfileRefusals`, `TestStableSectionsMatchOracle`, `TestCacheVerificationAcceptsComputedBytes`, `TestCEMV1210LRFRefusesEveryMapBeforeAnyInput`, `TestManifestPinsRetirementAndArtifacts`, `TestArtifactDigestDriftFails`, `TestCEMV1168DashboardRefusesStableBundle` | Retired routes refuse in their closed shapes; the TCQ oracle is ported to `tcq-stable/0`; the frontier-v0 tier is retired by its manifest record |
| CEM-V1-218 | `TestCEMV1218RetiredPredicatesRefuse`, `TestCEMV1218RetiredAttestationFlagsAreUnrecognized`, `TestProveCEMAttestationVerifyRefusesAnEnvelopeThatDoesNotVerify`, `TestProveCEMAttestationVerifyNamesTheRetiredPredicate`, `TestCEMV1065RefusesALegacyMap` | v0/v1 envelopes fail closed and the CLI names cem/v2; the removed flags are unrecognized; v2 still refuses a 0.x map |
| CEM-V1-219 | `TestCoreVerbsEmitTheFrozenProfiles`, `TestCEMEndToEndAndGoldenEnvelope`, `TestDecision0308DeclarationsAreClosed`, `TestGPKV0005ManifestWorkerDefault` | Core-freeze goldens regenerated from `cem/1.0`; cli-parity replays `parity=102 retired=31` |

## Packet retirement (V1-1278 S3, decision 0525 §1 step 7, 2026-10-13)

Status: proposed; V1-1278 stage S3, the last step of decision 0525 §1. S2b (`CEM-V1-215..219`)
left the product with one wire, `cem/1.0`, but kept the frozen `cem/0.x` packets, examples,
portable routes and specs. This stage retires them. Git history keeps every deleted byte.
Owner standing auto-accept of lane requirements (2026-10-11) applies; the requirements stay
proposed until recorded under that acceptance.

- `CEM-V1-220`: The `cem/0.x` protocol and conformance packets MUST be deleted:
  `protocol/cem-0.2`, `protocol/cem-0.3`, `interop/cem-0.1`, `conformance/cem-0.1`,
  `conformance/cem-0.2` and `docs/CEM-CI.md`. No tracked file outside history, frozen evidence,
  generated indexes and retired spec bodies may depend on those paths, and the acceptance search
  in the build-log entry classifies every remaining `cem/0.x` mention.
- `CEM-V1-221`: `protocol/cem-1.0` MUST be self-contained. The 0.x normative text that `cem/1.0`
  inherits is kept under `protocol/cem-1.0/stable/inherited/` (`cem-0.1-algorithms.md`,
  `cem-0.3-algorithms.md`, `cem-0.3.schema.json`) and listed in the packet `manifest.json` with
  its SHA-256 digest and byte count; no other packet file changes. The patch, simulation and
  repository vectors the inherited mechanics are tested against move unchanged to
  `interop/cem01-go/testdata/inherited-vectors/`, and the EOL policy exemption moves with them.
- `CEM-V1-222`: `interop/cem01-go` MUST verify `cem/1.0` only. The `cem/0.1` default `verify`
  route, `verify-03` and `ci` are retired: an invocation that names a map whose strictly parsed
  `spec` is a `cem/0.x` string MUST exit 1 with `unsupported-spec` and the fix line naming
  `verify-stable-canonical`, and every other invocation outside the `cem/1.0` commands MUST exit 2
  with `invocation`. The `ci` report, the in-toto envelope and the 0.3 packet tests are deleted.
  This retires `CEM-V1-PSR-001..003`, `FPK-V0-035` and `CEM-PILOT-020..023`.
- `CEM-V1-223`: `examples/cem` except the 0.x-free `examples/cem/experiments/` companion demo, its recipe script test and gate member (`cem-recipes-test`), and
  `tools/cem-interop-runner` MUST be deleted, with their Makefile, gate-ledger, CI shard and
  unbounded-reader entries. This retires `CEM-V1-PSR-004` and `CEM-PILOT-024..027`.
  `tools/cem-citation-baseline` has had no 0.x mode since S2a and is unchanged.
- `CEM-V1-224`: The Core remnants MUST name no retired wire: the dogfood loop passes
  `--artifacts` to every `cem status` it runs (`stableStatusArguments`), the experimental
  `frontiernextrepo` fixture recompute refuses every map and returns an empty universe, and the
  patch grammar errors name "the CEM patch grammar" instead of `cem/0.1`. Their codes are
  unchanged.
- `CEM-V1-225`: The `cem/0.2`, `cem/0.3`, 0.1 Go portability probe and 0.1 external interop specs
  MUST be marked retired by decision 0525 and superseded by this contract, keeping their bodies
  and the requirement IDs `REQUIREMENTS.tsv` traces. Live specs and guides whose 0.x sentences
  describe current behavior (the LRF, change-frontier profile, witness, pilot kit, dogfood, install,
  automation and architecture documents, `README.md` and `ROADMAP.md`) MUST be amended, and
  `LICENSING.md` MUST list exactly the surviving Apache-2.0 paths without widening the boundary.

Non-goals: no change to a `cem/1.0` packet file other than the inherited text and its manifest
entry; no rewrite of a committed map, seal, task record, decision or build-log entry; no edit of
frozen benchmark or performance outputs; no removal of the `docs/cem-0.1.schema.json` and
`docs/cem-0.2.schema.json` files that `protocol/cem-1.0/ALGORITHMS.md` cites; no new 1.0
behavior.

Failure modes: a link or citation into a deleted packet fails `line-citations-check` and is
rewritten as prose pinned to commit `5a056d0a`; a traceability row naming a deleted test fails
`traceability-tests-check` and is rewritten as a retirement note; a 0.x map given to the
portable consumer is refused rather than verified; a manifest entry that drifts from the inherited
file fails the packet digest check.

Rollback: revert the V1-1278 S3 commit. The packets, examples, runner and portable 0.x routes
return unchanged; no committed map, seal, golden or task record depends on them.

### Packet retirement traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-220 | `TestRetiredRouteRefusesEveryZeroXMap` | The acceptance searches in `docs/build-log/2026-10-13-v1-1278-s3-retire-packets.md` |
| CEM-V1-221 | `TestGoldenHunkIdentities`, `TestFrozenInvalidPatches`, `TestFrozenValidPatchesSimulate`, `TestNormativePatchFixtures` | The inherited vectors read from their new path; `script/check-eol-policy_test.sh`; the manifest digest equals the file's SHA-256 |
| CEM-V1-222 | `TestRetiredRouteRefusesEveryZeroXMap`, `TestStableArtifactBoundary` | Every 0.x map refuses on `verify`, `verify-03` and `ci`; the stable route is unchanged |
| CEM-V1-223 | None; a deletion has no behavior to test | `make test-commands-check` reads the literal `GATE_STEPS` list without the removed member; the acceptance search finds no `examples/cem` path other than the kept `experiments/` demo, and no runner path, outside history |
| CEM-V1-224 | `TestStableStatusArgumentsAddArtifactRoot`, `TestCEMV1210FixtureRecomputeRefusesEveryMap`, `TestFrozenInvalidPatches` | Codes unchanged; no 0.x name in the messages |
| CEM-V1-225 | `TestIndexCoversSpecsAndHeaders` | The retired specs, `INDEX.json` supersession and `README.md` rows agree; `license-headers-check` passes |

## Shared stable tamper matrix (V1-1340, 2026-10-13)

Status: proposed; V1-1340. No native test on the stable path covered the classic tamper classes:
the native tamper suites went through the retired `cem/0.1` path, and the portable consumer had its
own cases. Running one shared matrix showed five tampers on which the consumers disagreed, and the
matrix now pins the inherited normative disposition for each. Owner standing auto-accept of lane
requirements (2026-10-11) applies; the requirements stay proposed until recorded under that
acceptance.

- `CEM-V1-226`: The packet MUST carry one shared tamper matrix at
  `protocol/cem-1.0/stable/tamper-matrix/`: a repository recipe, one clean map (`stable.json`),
  tampered copies of it under `cases/`, and `EXPECTED.json` (profile `cem-stable-tamper-matrix/1`),
  every file listed in the stable `manifest.json` with its SHA-256 digest and byte count. It MUST
  cover at least these tamper classes: `PatchDigestMismatch`, `FabricatedEvidenceID`, `UncitedHunk`,
  `InvalidSpan`, `SpanOutOfRange`, `EvidenceIDSwap` (between hunks), `EvidenceDrop`, `HunkReorder` and
  `SpanRangeEdit`, plus the clean `control`.
- `CEM-V1-227`: The native verifier (`internal/cem/verify`, `Stable`) and the portable consumer
  (`interop/cem01-go`, `verify-stable-canonical`) MUST each run every case of the matrix from the
  packet files and return the pinned verdict. A case that only one consumer runs, or a disagreement
  between them, is a failure.
- `CEM-V1-228`: Each `EXPECTED.json` case MUST carry `name`, `tamper`, `description`, `map`,
  `exit`, `outcome`, `stage`, `code`, `issueCodes` and `citation`. A REJECT pins the exact stage and
  closed code (`code` for the wire and references stages, `issueCodes` for verification) and a null
  `citation`. An ACCEPT pins the whole `cem-citation-support/0` block. The pinned codes follow the
  inherited normative text: a duplicate evidence ID, a basis naming no evidence record, a basis
  pair repeated within one hunk, and an evidence record no basis cites are map-local wire checks
  refused `invalid-field`. An empty or
  reversed span is admitted by the wire and refused at verification as `evidence-unavailable`, as
  is a span past the end of its blob. A map whose hunk count differs from the canonical patch is
  the verification issue `patch-hunk-count`, the stable closed name for the inherited `uncited-hunk`.
- `CEM-V1-229`: Swapping evidence IDs between hunks, when every record stays a valid digest-bound
  base span, MUST ACCEPT. A support tier never changes the verdict (`CEM-V1-080`). The matrix pins the
  degraded tier instead: both hunks go from `anchored` to `cited`, `hunksByClass.anchored` from 2
  to 0, and `specificity` from 2 to -2.

Non-goals: no change to an existing packet file other than the stable `manifest.json` entries; no
new closed code; no tier threshold or verdict that depends on a tier; no change to native verifier
semantics.

Failure modes: a case file that drifts from its manifest entry fails both consumers' manifest
check. A recipe that no longer builds the pinned commits fails before any case runs. A consumer
whose stage or code differs from the pinned value fails that case by name. A matrix that drops a
required tamper class fails the coverage check in both tests.

Rollback: revert the V1-1340 commit. The matrix directory and its manifest entries go away, and
the portable consumer returns to `invalid-field` for a hunk-count mismatch, to `unresolved-reference`
or verification `invalid-field` for an unknown basis, uncited evidence or a repeated basis pair,
and to a wire
`invalid-field` for an empty or reversed span. No committed map, seal or golden depends on those
codes.

### Shared tamper matrix traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-226 | `TestCEMV1226TamperMatrixNative`, `TestCEMV1226TamperMatrixPortable` | Both check every matrix file against the stable manifest and the required tamper classes |
| CEM-V1-227 | `TestCEMV1226TamperMatrixNative`, `TestCEMV1226TamperMatrixPortable` | Each runs every case listed in `EXPECTED.json` as a named subtest (the case count is that file's, not restated here) |
| CEM-V1-228 | `TestCEMV1226TamperMatrixNative`, `TestCEMV1226TamperMatrixPortable` | Exit, outcome, stage, code, issue codes and the full citation block are compared per case |
| CEM-V1-229 | `TestCEMV1226TamperMatrixNative`, `TestCEMV1226TamperMatrixPortable` | Subtest `evidence-ids-swapped` pins ACCEPT with anchored 0, cited 2 and specificity -2 |

## Anchor pointer scope and notes-ref rollback (V1-1260, decision 0523 P1)

Status: accepted under the owner's standing auto-accept (2026-10-11); V1-1260. Decision 0523
proposed this as `CEM-V1-087`; the V1-1260 ticket and lane charter number it `CEM-V1-067`, and
`CEM-V1-087` stays unassigned. The 1.0 scope is `PRS-V1-018` (1.0 scope per the V1-0020
dependency). Before this change a `corvint-cem-anchor/0` pointer bound any commit: `cem anchor
--commit REV` attached HEAD's committed map to REV, and `cem provenance` checked only the map
path, blob and digest, so a sibling commit with the same parent read `verified`. A notes ref
reset to its parent read as a missing anchor. The `wire.ParseMap` part of the ticket was already
fixed by V1-1278 (`CEM-V1-065`, `CEM-V1-210..214`): anchoring and provenance parse the map with
`wire.ParseStable`. Decision 0355 is not rewritten; the build log carries the superseding note
for it and for the `/0` pointer text of `falsifiable-packet-v0.md` FPK-V0-037/038.

- `CEM-V1-067`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1260) `cem
  anchor` MUST write the pointer profile `corvint-cem-anchor/1`, which adds the members `base`
  and `patch_sha256` (the map's `change.baseRevision` and `change.patchSha256`) and
  `target_tree` (`REV^{tree}`) to the `/0` members. Anchor MUST refuse with
  `anchor-scope-mismatch`, writing no note, when the canonical `base..REV` patch (the
  `gitauth.CanonicalDiff` recipe every CEM consumer uses, which excludes the map sidecar) does
  not hash to the map's `patchSha256`. `cem provenance` MUST report the anchor row `verified`
  only when the note's map blob, path and digest check as before, the map parses as `cem/1.0`
  with `baseRevision` and `patchSha256` equal to the pointer's, `REV^{tree}` equals
  `target_tree`, and the canonical `base..REV` patch hashes to `patch_sha256`; otherwise it
  reports `scope-mismatch`, or `scope-unavailable` when the patch cannot be derived (for example,
  an absent base object). A `/0` pointer MUST stay readable and is reported `unscoped`, never
  `verified`; a `/0` pointer naming any scope member (even empty or null), a `/1` pointer
  missing one, or a note with bytes after its one pointer object, is `malformed`. `cem
  provenance --commit REV --notes-tip OID` MUST add a `notes_tip` member
  (`ref`, `pinned`, `live`, `state`) and report `state` `rollback`, with an anchor row in state
  `rollback`, when OID is not an ancestor of the live `refs/notes/corvint` tip or the ref is
  absent; otherwise `state` is `current`. Without `--notes-tip` the result has no `notes_tip`
  member. OID MUST be a full lowercase hexadecimal object ID, because a revision name would
  resolve to the live tip it is meant to check; anything else is refused `invalid-arguments`,
  and an OID that names no commit is refused `repository-object-unavailable`. When
  `refs/notes/corvint` moves between the tip comparison and the end of the read, provenance
  MUST refuse with `provenance-notes-moved` rather than report rows from a different tip.
  Provenance stays read-only: it writes no ref, note, object, index, config or worktree file.

Wire and schema: the pointer profile changes from `/0` to `/1` for new notes and the
`cem provenance` result gains the optional `notes_tip` member and the anchor states `unscoped`,
`scope-mismatch`, `scope-unavailable` and `rollback`; the stable map, `cem-stable-verification/1`
and every other provenance row are unchanged. Readers of `/0` notes keep working, so the change
is additive for readers. It is not additive for a consumer that treated a `/0` `verified` row as
trusted: that row is now `unscoped`.

Non-goals: no default-on or automatic anchoring; anchoring stays an explicit operator step. No
read command writes anything. `verified` does not imply that the map's citations or criteria
verify, who wrote the change, or (without `--notes-tip`) that the notes ref was not rolled back.
No automatic migration of existing `/0` notes.

Failure modes: an existing `/0` note on REV blocks re-anchoring with `anchor-note-conflict`; the
operator removes it deliberately (`git notes --ref=refs/notes/corvint remove REV`) and anchors
again. A map whose base object is absent from the repository is refused by anchor and reads
`scope-unavailable` in provenance. A `--notes-tip` the operator did not retain from a trusted
source proves nothing about rollback; the pin is only as good as where it came from.

Rollback: revert the V1-1260 commit. New anchors are written as `/0` again, provenance reports
`/1` notes `malformed`, and `--notes-tip` becomes an unrecognized argument. No committed map,
seal or golden depends on the pointer profile.

### Anchor pointer scope traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-067 | `TestCEMV1067SiblingCommitIsScopeMismatch`, `TestCEMV1067VerifiedImpliesTheAnchoredPatchIsBaseToREV`, `TestCEMV1067ResetNotesRefIsRollback`, `TestCEMV1067LegacyPointerIsReadableAndUnscoped` (`internal/gitnotes`); `TestCEMAnchorAndProvenanceInteropThroughTheCLI` (`cmd/corvint`); `TestAnchorActionsParseLikeTheirSiblings` (`internal/cem/cli`) | A sibling commit is refused by anchor and every forged `/1` note on it reads `scope-mismatch`; each `verified` row's `patch_sha256` equals the recomputed canonical `base..REV` digest and `target_tree` equals `REV^{tree}`; a reset or deleted notes ref reads `rollback` under `--notes-tip`; a `/0` note reads `unscoped`; no ref moves on any read |

## Portable Linux process containment (V1-1358, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1358.
The portable consumer's owner/keeper process containment was qualified only on Darwin, so
`verify-stable-canonical` refused every Linux run with `runtime` /
`unsupported-process-containment`, and the experimental Tasks verdict gate (TCB-V0-016/017) could
grant only on Darwin. This section qualifies an explicitly bounded Linux route in
`interop/cem01-go` and supersedes, for that consumer on Linux amd64 and arm64 only, the earlier
statement that portable standalone Linux remains pending. The Linux route was written from this
specification and `protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.md`; it copies no code from the
AGPL native reaper, and the module stays Apache-2.0 under `LICENSING.md`.

- `CEM-V1-180`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1358) The
  portable consumer's process containment MUST be qualified exactly on Darwin and on Linux
  amd64 and arm64, chosen by build constraint. Every other operating system and architecture
  MUST keep refusing with `runtime` / `unsupported-process-containment` before admission or any
  spawn. Both qualified routes share one owner/keeper implementation: one Git child in the
  keeper's private process group, retirement proven by absence, and no keeper or Git process
  outliving its owner.
- `CEM-V1-181`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1358) On Linux
  the consumer MUST probe its primitives before admission and refuse with
  `unsupported-process-containment` unless all of these hold: the owner is not PID 1,
  `pidfd_open(2)` succeeds on the owner itself (Linux 5.3 or later), `epoll_create1(2)` succeeds,
  and `/proc/self/stat` is readable and agrees with the owner's parent and process group.
- `CEM-V1-182`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1358) The Linux
  keeper MUST watch the owner PID that the owner names in the keeper's argument vector at spawn,
  never whichever process is its parent when it runs. It MUST refuse a named PID that is not a
  canonical decimal greater than 1 or is not its parent, then open a pidfd on that PID and
  re-check after opening that its parent is still that PID. It MUST wait for control input and for its Git child (through a pidfd on
  the unreaped child) in one close-on-exec epoll set together with the owner's pidfd. Owner exit,
  a failed open or any epoll failure MUST end in the keeper killing its own group and exiting, the
  same fail-closed path as the Darwin kqueue watch.
- `CEM-V1-183`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1358) Linux
  answers signal 0 successfully for a group that holds only zombies, so on Linux step 1 of the
  two-step retirement MUST NOT end on signal 0. After the owned group SIGKILL, with the keeper
  still unreaped, the group is quiet only when `/proc` shows the keeper as a zombie (`Z` or `X`)
  whose parent is the owner and whose process group is its own PID, one complete `/proc` listing
  shows every process in that group as a zombie with at most one thread and includes the keeper,
  and the keeper still qualifies after the listing. The observation sends no signal. Each `stat`
  read is bounded at 4,096 bytes, each `status` read at 16,384 bytes and the listing at 2^22
  entries. Any read failure, parse failure or exceeded bound counts as not quiet, which ends in
  `HOLD` at the retirement bound. Step 2 is unchanged on both systems: reap the keeper once, then
  only signal-0 `ESRCH` within the two-second post-reap cap proves absence.
- `CEM-V1-184`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1358) On a
  qualified Linux host `verify-stable-canonical` MUST return the same outcome and `verdictSha256`
  as the native verifier for the same sealed map, and the shared tamper matrix and every
  applicable public full-result case MUST pass there as on Darwin. The qualified hosts for the
  portable leg of the Tasks verdict gate are Darwin and Linux amd64/arm64 (TCB-V0-030).

Non-goals: no change to the native verifier, the packet under `protocol/cem-1.0`, any closed code,
result field or retirement bound; no Linux architecture other than amd64 and arm64; no cgroup,
namespace or `PR_SET_CHILD_SUBREAPER` containment; no claim about escaped sessions or
cross-device topology.

Failure modes: a kernel older than 5.3, a missing or unreadable `/proc`, a seccomp policy that
denies `pidfd_open`, or an owner running as PID 1 refuses before admission. A member that
survives the group SIGKILL, a zombie leader with a live thread, or an unreadable member keeps the
group not quiet and ends in `HOLD` (`unsupported-process-containment`), never a silent pass. A
process created in the group after the listing cannot exist, because every member has a pending
fatal signal and Linux refuses `fork` to such a task. On Linux the PID 1 refusal also keeps the
keeper's owner watch meaningful.

Rollback: revert the V1-1358 commit. `stable_process_unix.go` returns to
`stable_process_darwin.go`, the Linux files are deleted, Linux refuses
`unsupported-process-containment` again, and `TestTCBV0016GateVerdictRunsBothConsumers` returns
to skipping `NOT_RUN` off Darwin. No packet, committed map, seal or result byte depends on the
Linux route.

### Portable Linux process containment traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-180 | `TestStableLifecycleLingeringDescendantRetired`, `TestStableLifecycleHostileDescendantsRetired`, `TestStableLifecycleCancelRetiresDescendants` (Darwin and Linux); build constraints of `stable_process_{unix,darwin,linux,other}.go` | `go vet` of the module for darwin/arm64, linux/amd64, linux/arm64, linux/386, linux/riscv64, windows/amd64 and freebsd/amd64 (build log) |
| CEM-V1-181 | `TestStableLinuxOwnerSupported` | Linux arm64 container run as uid 1000 (build log) |
| CEM-V1-182 | `TestStableLifecycleKeeperOwnerExitMidFrame`, `TestStableLifecycleKeeperOwnerExitWhileHolding`, `TestStableLifecycleKeeperOwnerExitWhileRunning`, `TestStableLifecycleKeeperRefusesForeignOwner` | Pass on Darwin and on Linux arm64 with `-race` |
| CEM-V1-183 | `TestStableLinuxZombieOnlyGroupRetires`, `TestStableLinuxLiveMemberBehindZombieLeaderHolds`, `TestStableLinuxQuietRequiresOwnZombieLeader`, `TestStableLinuxProcStatParsesHostileCommandName`, `TestStableLifecycleRetireTwoStep`, `TestStableLifecycleRetireBoundHold`, `TestStableLifecycleRetirePostReapBoundIsInsideOuterBound` | Signal 0 still succeeds on the zombie-only group while the `/proc` proof retires it |
| CEM-V1-184 | `TestTCBV0016GateVerdictRunsBothConsumers`; `TestStableCanonicalFullResults`, `TestStableCanonicalLedgers`, `TestCEMV1226TamperMatrixPortable` | Native and portable `verdictSha256` both `14ac383d61741ab12c6a422d086701302b230295ff97babcb443aead1cb40476` on linux/arm64 and darwin/arm64 (build log); linux/amd64 is the CI `go-product-shard` run |

## Runner receipt admission and the Tasks chain (V1-0632, 2026-10-20)

Status: accepted under the owner's standing auto-accept (2026-10-11); V1-0632. The plain producer
(`CEM-V1-240`) treats its runner plan and receipt as opaque files, so it would bind a failed,
skipped, stale or mismatched receipt as readily as a passing one. Nothing showed the whole chain
from a native Tasks attempt through the runner, the producer and both verifiers. This section adds
one opt-in producer check and one end-to-end fixture chain. It changes no wire member, verifier
rule, map byte or `corvint cem` verb.

- `CEM-V1-280`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) A
  `cem-stable-plain-assembly/1` request MAY carry one more optional member, `runnerAdmission`.
  Its only accepted value is `corvint-test-runner/0`; any other value, including the empty string
  and `null`, is refused before any read. Only an absent member keeps the runner files opaque,
  exactly as `CEM-V1-240` states. When it is present
  the producer applies `CEM-V1-281..285` in that order, after the source-map check and before it
  creates the bundle, and a refusal writes no bundle at all. An admitted receipt yields the same
  map, artifacts and result as the opaque path; the map does not record that admission ran, and
  the receipt record keeps `executionAuthority` `CALLER_OBSERVED`.
- `CEM-V1-281`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The plan
  and receipt MUST decode as closed `corvint-test-runner-plan/0` and
  `corvint-test-runner-receipt/0` documents, and every plan input path MUST be a literal
  repository path (checked just before the `CEM-V1-283` read); otherwise the request is refused
  with `runner-document-invalid`.
- `CEM-V1-282`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The
  receipt MUST bind the plan: its `planSha256` is the plan's identity, its execution
  `inputSha256` is the identity of the plan's input files, and its execution runner is the plan's
  runner. Otherwise the request is refused with `runner-receipt-tampered`.
- `CEM-V1-283`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The plan
  MUST pin 1 to 4096 input files, and each MUST be a blob of the request's `target` whose SHA-256
  equals the pin, read by one bounded `git cat-file --batch`. An empty or oversized set, an absent
  path, a non-blob or a different digest is refused with `runner-receipt-stale`.
- `CEM-V1-284`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) Every
  observed test MUST be `PASSED` or `SKIPPED`, the observation MUST be complete, and the receipt
  MUST carry no execution error; otherwise the request is refused with `runner-receipt-failed`.
- `CEM-V1-285`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) At least
  one observed test MUST be `PASSED`; a receipt with only skipped tests or none is refused with
  `runner-receipt-skipped`.
- `CEM-V1-286`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) One
  fixture test MUST drive, on a private disposable native Tasks store and one fixture checkout:
  a native ticket claim; real `corvint-test-runner` plans and receipts built from repository
  sources; the plain producer with admission; a bind and seal; and the store's `cem-verdict` gate
  running the built `corvint cem gate-verdict` with the portable consumer built from
  `interop/cem01-go`. The gate MUST pass with outcome `GRANT`, native `ACCEPT` with sidecar
  `EXACT`, portable `ACCEPT`, equal native and portable `verdictSha256`, and every governed path
  satisfied. Before that, the producer MUST refuse the runner's real failed, skipped, stale and
  tampered receipts with `runner-receipt-failed`, `runner-receipt-skipped`,
  `runner-receipt-stale` and `runner-receipt-tampered`, writing no bundle, and MUST accept the
  failed receipt when admission is absent.
- `CEM-V1-287`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) In the
  same chain, a runner artifact edited after production MUST be refused three ways. Tasks refuses
  the dirty worktree with `DIRTY_WORKTREE`. The gate's native and portable recomputations both
  `REJECT` at stage `artifacts`, the native issue being `artifact-digest-mismatch`, with equal
  verdict digests, so the gate answers `CEM_VERDICT_UNVERIFIABLE`. A committed edit fails the
  Tasks gate with `CEM_VERDICT_PRODUCER_MISMATCH` / `sealed-tree-mismatch`.

Non-goals:
- no runner execution by the producer; admission reads the two documents and the target's blobs
- no receipt signing or authentication (`FPK-V0-036`, V1-1264); a caller can still forge a
  self-consistent passing receipt
- no verifier-side admission: a portable consumer still checks the runner files by SHA-256 only
- no new public case, capsule or dashboard (V1-0632 gap 1)

Failure modes: each refusal above names its code and leaves no bundle; a Git read failure is
refused without a code; an unknown `runnerAdmission` value is refused before any read.

### Runner receipt admission and Tasks chain traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-280 | `TestCEMV1280RunnerAdmissionIsOptIn` and `TestCEMV1280PresentAdmissionMemberIsNeverOpaque` (`internal/cemcandidate`); the opaque control in `TestCEMV1286TasksRunnerProducerVerifyChain` (`internal/cemchain`) | `docs/build-log/2026-10-20-V1-0632-cem10-tasks-chain.md` |
| CEM-V1-281 | `TestCEMV1281To1285RunnerAdmissionRefusals` (opaque public-case files, unknown receipt member, input path escapes) | Fails with the admission call disabled (build log) |
| CEM-V1-282 | `TestCEMV1281To1285RunnerAdmissionRefusals` (another plan, other inputs, another runner); `TestCEMV1286TasksRunnerProducerVerifyChain` | Fails with the admission call disabled (build log) |
| CEM-V1-283 | `TestCEMV1281To1285RunnerAdmissionRefusals` (earlier source, absent path, no inputs); `TestCEMV1286TasksRunnerProducerVerifyChain` | Fails with the admission call disabled (build log) |
| CEM-V1-284 | `TestCEMV1281To1285RunnerAdmissionRefusals` (failed test, incomplete observation, execution error); `TestCEMV1286TasksRunnerProducerVerifyChain` | Fails with the admission call disabled (build log) |
| CEM-V1-285 | `TestCEMV1281To1285RunnerAdmissionRefusals` (only skipped, no tests); `TestCEMV1286TasksRunnerProducerVerifyChain` | Fails with the admission call disabled (build log) |
| CEM-V1-286 | `TestCEMV1286TasksRunnerProducerVerifyChain` (`internal/cemchain`) | Darwin arm64 run (build log); Linux is CI |
| CEM-V1-287 | `TestCEMV1286TasksRunnerProducerVerifyChain` (`internal/cemchain`) | Darwin arm64 run (build log) |

Rollback: remove `internal/cemcandidate/plain_runner.go`, the `runnerAdmission` member and its
call in `internal/cemcandidate/plain.go`, `internal/cemchain/`, and their tests. Requests without
the member are unaffected; no committed map depends on admission.

## Packet truthfulness and citation vectors (V1-1333, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1333,
recorded by V1-1370. The Apache-2.0 packet still described stable `cem/1.0` as an unimplemented
proposal, carried the citation rules, the stop list, the result serialization and the diff recipe
only by reference to this AGPL spec, Go code or a retired 0.3 packet, and had no vector that pinned
an anchored hunk on a deletion, a nonzero specificity together with same-path citations, or the
exact citation bytes from the native verifier. Owner standing auto-accept of lane requirements
(2026-10-11) applies and V1-1370 recorded each requirement under it. The
scope is the 1.0 packet scope of `PRS-V1-018`.

- `CEM-V1-107`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1333) Every status statement in `protocol/cem-1.0/README.md` and
  `protocol/cem-1.0/stable/` (the README files, `ALGORITHMS.md`, the `stable.schema.json`
  description, the `status` members of `OPERATION.json` and `manifest.json`) MUST say that the stable `cem/1.0` wire is implemented and
  produced on main, that its technical contract is proposed and not promoted, and that its
  qualification is bounded. None may call the technical contract accepted. `R1-EXPECTED.json` MUST
  be described as a declarative expectation set that no test executes; its bytes stay unchanged.
- `CEM-V1-108`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1333) `protocol/cem-1.0/stable/ALGORITHMS.md` MUST state, as new Apache-2.0 prose, every
  rule a consumer needs to reproduce the `cem-citation-support/0` block byte for byte: hunk order,
  hunk path (base path for a deletion), the added-bytes join that keeps each line's LF, the
  requirement ID scan, span-relative defining lines, the `id` and `path` anchors, tiers, annotation
  with the full stop list, fan-out, `samePathCitations` over basis entries, concentrated hunks and
  the shift null behind `invariant` and `specificity`.
- `CEM-V1-109`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1333) The same file MUST state the member order of the citation block and of the
  top-level result, the compact encoding with its escapes, and the canonical form behind
  `verdictSha256` (sorted names, separators, string escapes, numbers, and the three removed
  members). The member order inside the nested result objects is fixed by `CEM-V1-113`. (Amended in
  place by V1-1369 while still proposed: the earlier clause said that the nested order was not fixed
  and that results compare by digest, citation bytes or parsed value.)
- `CEM-V1-110`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1333) The same file MUST carry the canonical diff recipe in full: Git options, `-c`
  settings, environment, the attribute source, the path exclusion and the provenance proof the
  output must pass. It MUST name Git's own diff output as the one part defined by reference.
- `CEM-V1-111`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1333) The packet MUST carry `protocol/cem-1.0/stable/citation-vectors/`: a repository
  recipe, a clean map, refused copies and `EXPECTED.json` (profile `cem-stable-citation-vectors/1`),
  every file listed in the stable `manifest.json`. The expected values MUST come from the native
  verifier. The clean case MUST have anchored hunks, nonzero `specificity` and nonzero
  `samePathCitations`, including a cited deletion hunk anchored through its base path. The refused
  cases MUST include a same-path deletion drift, a stale-span drift, a base mismatch and wire classes
  not in the tamper matrix. The native verifier and the portable consumer MUST each run every case
  and match exit, outcome, stage, codes, drift, `verdictSha256`, top-level member order and the
  citation block bytes.
- `CEM-V1-112`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1333) For each listed misreading (an LF-stripped join, citation or top-level member order,
  digest separators or escapes, an absolute instead of span-relative defining line, a stop-list
  omission or case-sensitive stop terms, a specificity miscount, a deletion hunk without its path, a
  same-path count over distinct evidence, a deleted cited file not treated as drift), a one-line
  mutation of the native verifier MUST fail at least one citation vector. The packet README MUST
  list each misreading with the cases that fail it.

Non-goals: no change to verifier or producer behaviour; no change to `R1-EXPECTED.json`, the
inherited copies or any frozen digest; no restatement of Git's diff algorithm; no technical
acceptance or promotion of this contract. (V1-1369 removed the non-goal "no fixed member order
for nested result objects"; `CEM-V1-113` now fixes that order.)

Failure modes: a vector file that drifts from its manifest entry fails both consumers' manifest
check. A recipe that no longer builds the pinned commits fails before any case runs. A consumer
whose citation bytes, digest or member order differ fails that case by name. A packet edit that
again calls the contract accepted, or a rule restated wrongly, is caught only by review and by the
vectors that exercise the rule.

Rollback: revert the V1-1333 commit. The vectors and their manifest entries go away and the packet
prose returns to its previous text. No committed map, seal or golden depends on them.

### Packet truthfulness traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-107 | none (prose) | Readback of the packet status statements; build log `2026-10-14-v1-1333-cem-packet-truthful.md` |
| CEM-V1-108 | `TestCEMV1111CitationVectorsNative`, `TestCEMV1111CitationVectorsPortable` | The clean case pins the citation block bytes the restated rules produce |
| CEM-V1-109 | `TestCEMV1111CitationVectorsNative`, `TestCEMV1111CitationVectorsPortable` | Each case pins `verdictSha256` and the top-level member order; the nested order is traced under `CEM-V1-113` |
| CEM-V1-110 | `TestCEMV1111CitationVectorsNative`, `TestCEMV1111CitationVectorsPortable` | Both consumers derive the same canonical patch from the recipe repository |
| CEM-V1-111 | `TestCEMV1111CitationVectorsNative`, `TestCEMV1111CitationVectorsPortable` | All 8 cases run as named subtests; both also check the clean case's anchored, specificity and same-path counts and its path-anchored whole-file deletion hunk |
| CEM-V1-112 | `TestCEMV1111CitationVectorsNative` | Scratch mutation run of 13 mutants, all killed; commands and table in the build log |

## Discrimination witness mutant bound (V1-1259, decision 0523 P2, 2026-10-14)

Status: accepted under the owner's standing auto-accept (2026-10-11); V1-1259. A hunk
`discriminates` witness claiming 1,000,000 mutants under `bounds.maxMutants: 1` was admitted by
both consumers: the count rule checked only `killed + survived <= mutants`. Decision 0523 P2
proposed this ID; it is the native obligation the inherited `ALGORITHMS.md` text waits for before
a consumer may require `mutants <= maxMutants`.

- `CEM-V1-090`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1259) A
  `cem/1.0` hunk `discriminates` witness whose `mutants` exceeds its `bounds.maxMutants` MUST be
  refused at the wire stage with `invalid-field` by the native reader (`internal/cem/wire`,
  `validateDiscrimination`) and the portable consumer (`interop/cem01-go`,
  `stableDiscrimination`); `mutants` equal to `maxMutants` is admitted. A witness whose
  `survived` differs from the number of `survivors` entries, or whose survivor `line` lies
  outside the hunk's `newRange`, stays refused `invalid-field` by both, as the inherited count
  rules already required. Survivors need not be distinct. The shared tamper matrix carries the
  cases `discrimination-mutants-over-bound` and `discrimination-survivors-inconsistent` (wire
  `invalid-field`) and the boundary control `discrimination-at-bound` (ACCEPT), each pinned for
  both consumers under `CEM-V1-227`.

Non-goals: no new closed code; no check of `maxHunks` or `wallTimeSeconds` against anything; no
requirement that `killed + survived = mutants`; no duplicate-survivor rule; no change to the
witness encoding, the producer (`internal/cemdiscriminate` already stops at `maxMutants`) or
`discriminationValidation`, which stays `WIRE_VALIDATED_ONLY`.

Failure modes: a map from an external producer that over-reports mutants now fails the wire stage
instead of verifying. No committed `.corvint/changes` map carries `discriminates`, so no committed
map changes verdict. A consumer that skipped the bound fails the matrix case by name.

Rollback: revert the V1-1259 change commit. Both consumers return to admitting any `mutants`
count, and the three matrix cases and their manifest entries go away.

### Discrimination mutant bound traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-090 | `TestInheritedDiscriminationWitness` (`internal/cem/wire`); `TestCEMV1226TamperMatrixNative`, `TestCEMV1226TamperMatrixPortable` | Subtests `discrimination-mutants-over-bound` and `discrimination-survivors-inconsistent` pin wire `invalid-field` and `discrimination-at-bound` pins ACCEPT on both consumers; without the bound check both consumers fail `discrimination-mutants-over-bound`; the matrix runs the 23 cases of `EXPECTED.json` (the CEM-V1-227 row no longer restates a count) |

## Consumer hunk-revert falsifier (V1-1266, decision 0523 P2)

Status: accepted; `CEM-V1-088..089` under the owner's standing auto-accept (2026-10-11), V1-1266;
`CEM-V1-250` under the same standing auto-accept, V1-1452. Decision 0523 P2 makes `held` the one support tier that a consumer
observes rather than reads: the consumer reverse-applies one hunk and runs the tests the map names
for it. The verifier result and its `verdictSha256` (`CEM-V1-066`) stay exactly as they are; the
falsifier reports beside them, opt-in, under the fixed `go test` runner profile.

- `CEM-V1-088`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1266) `held`
  MUST come only from the consumer's own re-execution. `cem verify --hunk-revert` adds the envelope
  member `hunkRevert` (profile `cem-hunk-revert/0`, runner `go-test`) beside the unchanged
  `stableVerification` result, and only after an `ACCEPT`. A candidate is a `supported` hunk of
  non-test Go source whose basis cites a `_test.go` span as `test-claim`; its named tests are the
  top-level `func TestX(t *testing.T)` declarations whose headers lie inside the cited span of the
  parsed base file (`TCQ-V0-061`), run in
  the cited file's package directory. The consumer exports the target tree from Git objects into a
  disposable directory under the system temporary directory, reverse-applies exactly that one hunk
  there, and runs only the named tests with `go test -json -count=1 -run '^(names)$' .` under the
  native go-test pins (`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`) plus `GOWORK=off` and
  `GOENV=off`. A row is `held` only when a named test passed at the target and, with the hunk
  reverted, the consumer decoded a test-level `fail` event for it in a package whose own result is
  `fail`; a test-level `fail` in a package that passed is printed output, not a failure, and is
  `not-run: infrastructure`. A row is `survived` when the eligible tests all pass reverted.
  A producer-written witness, including a `discriminates` member, never counts. The caller's
  worktree, index, refs and objects are only read (`git ls-tree`, `git cat-file --batch`, no
  optional locks), and the copy is removed on every exit path. The copy's reverted file is replaced
  only through real directories, never through a symlink, and a Go cache variable that points into
  the caller's repository is moved into the disposable workspace. The report's `hunksByClass` moves
  each held hunk from `anchored` or `cited` to `held`; the in-result `citation` block keeps
  `held` 0. Without the flag the envelope and every output byte are unchanged; `status` and
  `report` refuse the flag `invalid-arguments`.
- `CEM-V1-089`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1266) A row that
  is not `held` or `survived` MUST be `not-run` with one closed reason, never `held`:
  `revert-breaks-build` (the reverted package does not compile, or the package fails with no
  eligible test result), `fails-without-revert` (no named test passes at the target, including a
  package that does not build there), `skipped`, `test-not-observed`, `timeout` (a per-run bound,
  default 300 seconds, or the test binary's own timeout panic), `total-limit` (the whole falsifier
  bound, default 1200 seconds), `infrastructure` (no `go` binary, an export or decode failure, an
  output bound), `no-named-test` (the cited span declares no top-level test), `not-a-modification`
  (an added or deleted file) and `revert-mismatch` (the target bytes at the hunk's new range are
  not its new side). The report records both bounds and its elapsed seconds.
- `CEM-V1-250`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1452) The
  disposable export of `--hunk-revert`, and of `cem verify --guarding-tests` (`TCQ-V0-062`), MUST
  be created by one shared helper under the system temporary directory, and that helper MUST
  refuse a temporary directory that is, or lies below, the repository's worktree or Git common
  directory, decided by file identity of the directory and each ancestor so that a symbolic link,
  a case alias on a case-insensitive volume or any other spelling counts: nothing is exported or run, and every row that would
  have run is `not-run` with reason `infrastructure`. A repository below the temporary directory,
  the ordinary layout, is unaffected.

Non-goals: no change to `StableResult`, the citation block, `verdictSha256`, the wire, the public
cases or the portable consumer, which does not re-execute; no runner other than `go test`; no
promotion of `held` into the verifier result; no mutation testing (`TCQ-V0-056`); no change to
`cem status`, `report` or any dogfood command.

Failure modes: a hunk whose revert leaves the named test passing is `survived`, so a weak test is
visible rather than credited. A test that fails at the target, a broken build, a skip, a timeout or
a host without Go is `not-run` with its reason. A tree with a symlink, a submodule or a path
escaping the copy is refused as `infrastructure` rather than copied partially. A run whose output
exceeds 32 MiB is `infrastructure`.

Recorded limits: only `go test` re-executes; every other runner is a recorded rc.3 limit, as is the
portable consumer (`interop/cem01-go`), which reports no falsifier. The run environment passes the
host `HOME`, `PATH`, `GOROOT`, `GOPATH`, `GOMODCACHE` and `GOCACHE` through so cached modules and
builds are reused, which the native executor's minimal environment does not; dependencies absent
from the module cache fail offline as `fails-without-revert` or `revert-breaks-build`. Tests are
named only from the cited span; a subtest is never named on its own. The flag executes the target's
test code with the caller's privileges and no filesystem confinement, exactly as `go test` would:
Corvint itself only reads the caller's repository, but code under test can write anywhere the caller
can, and code under test that deliberately fails when its source is reverted makes a row `held`.
Run the flag only on a change whose tests you would run. `CEM-V1-250` keeps Corvint's own export
out of the repository; it does not confine what the tests themselves write.

Rollback: revert the V1-1266 code commit. The flag, the `hunkRevert` member and the falsifier code
go away; no committed map, seal, golden or public case depends on them. `CEM-V1-250` rolls back with
the V1-1452 code commit: both flags again create the export wherever `TMPDIR` points.

### Hunk-revert falsifier traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-088 | `TestCEMV1088HeldOnlyFromConsumerReexecution`, `TestCEMV1088ForgedWitnessNeverCounts`, `TestCEMV1088FlagIsVerifyOnly` (`internal/cem/workflow`); `TestCEMV1088VerifyHunkRevertIsOptIn` (`internal/cem/cli`); `TestCEMV1088JudgeRevertsOnlyInsideTheCopy`, `TestCEMV1088ReverseHunkAppliesExactlyOneHunk`, `TestCEMV1088GoTestProfileMatchesNative`, `TestCEMV1088ExportTreeReadsOnlyObjects`, `TestCEMV1088CopyWritesNeverFollowSymlinks`, `TestCEMV1088CountMovesOnlyAcceptedTiers` (`internal/cem/verify`) | Held only from a consumer-decoded failure; a forged `discriminates` witness on a weak test stays `survived`; the caller's tree including `.git` hashes identically before and after; the stable result is byte-identical with and without the flag |
| CEM-V1-089 | `TestCEMV1088HeldOnlyFromConsumerReexecution` (`internal/cem/workflow`); `TestCEMV1089ClassifyRevert`, `TestCEMV1089DecodeGoTestJSON`, `TestCEMV1088JudgeRevertsOnlyInsideTheCopy` (`internal/cem/verify`) | Real `go test` runs: a revert that breaks compilation and a named test that fails without the revert are both `not-run`; a printed test failure in a passing package (the recorded go1.27.2 stream) is never `held`; timeout, total limit, mismatch, skip and infrastructure reasons pinned |
| CEM-V1-250 | `TestCEMV1250HunkRevertRefusesRepositoryTMPDIR`, `TestCEMV1250GuardingTestsRefusesRepositoryTMPDIR` (`internal/cem/workflow`); `TestCEMV1250WorkspaceRefusesEverySpellingOfTheRepository` (`internal/cem/verify`: worktree, subdirectory, Git directory and, where the volume is case-insensitive, a case alias are refused; a directory outside is used) | With `TMPDIR` set to an existing directory inside the worktree, every row of both flags is `not-run: infrastructure`, no plan row prints, and nothing is written there; without the refusal the same runs export into it and judge the rows |

## Normative nested result member order (V1-1369, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1369.
The packet left the member order inside the nested objects of `drift`, `hunks` and
`references` unfixed, because the two reference consumers wrote it differently. A consumer could
therefore not reproduce a whole result line from the packet alone. Measured at
`b8a2e0336cb88269378f87432104bfc33fc4e89b`, both consumers are deterministic, the native order
follows from Go's sorted map keys and fixed struct order, and the digest was already independent of
nested order. Aligning the portable consumer to the native order needs no change to `internal/cem`
output bytes, so the native order becomes the contract. The scope is the 1.0 packet scope of the
product requirement `PRS-V1-018`.

- `CEM-V1-113`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1369) Every
  `verify-stable` result line MUST be the bytes `protocol/cem-1.0/stable/ALGORITHMS.md` defines,
  including the member order inside every nested object. A `drift` entry has `evidenceId`, `path`,
  `baseBlobOid`, `status`, `targetBlobOid`, `targetSpan`, and a `targetSpan` object has `end`, then
  `start`. Within every object inside a `hunks` entry, at any depth, members are sorted by name.
  `references` has `artifacts`, `criterionBindings`, `criterionLinks`, `runnerReceipts`, and each
  of their entries uses the order and omission rule the packet states. `axes` is sorted by name. The
  packet MUST state this order as Apache-2.0 prose. It MUST pin the whole result lines of every
  `citation-vectors` case and of the `neutral-criterion` ACCEPT as `results/*.json` files listed in
  the stable manifest. The native verifier and the portable consumer MUST each produce those bytes
  exactly. The only exception is `runtime.goVersion`, which names the running toolchain.

Non-goals: no change to the native verifier's output bytes, the verdict digest, the top-level
member order or the citation block; no change to map encoding; no witness-bearing (`coverage` or
`discriminates`) result line is pinned (the public cases have none, so the sorted-hunk rule covers
those members only by statement); no technical acceptance or promotion of this contract.

Failure modes: a consumer that writes any nested member in another order fails the pinned line for
that case by name, in both consumers' tests. A pinned file that drifts from its manifest entry fails
both manifest checks. A toolchain other than go1.27.2 changes only `runtime.goVersion`, which the
tests substitute before comparing.

Rollback: revert the V1-1369 commit. The portable consumer then writes its previous nested order,
the pinned lines and their manifest entries are removed, and `CEM-V1-109` returns to saying that
nested order is not fixed. No committed map, seal or digest depends on nested order.

### Nested result order traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-113 | `TestCEMV1111CitationVectorsNative`, `TestCEMV1111CitationVectorsPortable`, `TestCEMV1083NeutralCriterionPublicCase` (both consumers) | Nine pinned whole result lines under `citation-vectors/results/` and `neutral-criterion/results/`; the build log records the order measured at base and the mutation check |

## Portable consumer Git capability and the Tasks gate's PATH (V1-1377, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1377.
On the hosted GitHub `macos-15` runner the experimental Tasks verdict gate refused every fixture with
`CEM_VERDICT_VERIFIER_DISAGREEMENT` because `verify-stable-canonical`, run under the gate's fixed
`PATH=/usr/bin:/bin`, answered `UNSUPPORTED` while the native verifier accepted. The retained
diagnostic output of release-gates dispatch 38650914390 names the cause: that PATH resolves Apple
Git 2.39.5, which rejects the `--attr-source` option the stable repository diff passes
(`interop/cem01-go/stable_repository.go` `stableOpDiff`; the native `internal/cem/gitauth` diff
passes the same option), so the consumer's canonical diff child exits non-zero and it refuses at stage `repository`
with the packet's existing `git-diff-failed` code. Process containment (CEM-V1-180..184) is not involved. This
section records the consumer's Git capability contract and the gate-side fix
(`docs/specs/tasks-criterion-binding-v0.md` TCB-V0-040..044). The consumer's code, the packet and
every closed code and result field are unchanged.

- `CEM-V1-185`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) The stable
  repository diff requires a Git that accepts `--attr-source` (Git 2.41 or later), because the
  envelope pins attributes to the empty tree through that option. On a Git that rejects it the
  consumer MUST answer `UNSUPPORTED` at stage `repository` with `git-diff-failed`, the code its
  repository envelope already assigns to a canonical diff child that exits non-zero, with
  `accept` `null` and no fabricated patch, verdict or citation. No new code is introduced for
  this and the consumer MUST NOT probe `git --version` in its place: the diff's own failure is
  the proof.
- `CEM-V1-186`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) The
  consumer resolves `git` only through the `PATH` its caller gives it, which `stableGitEnv`
  passes to the Git child with the pinned `GIT_*` isolation, and MUST NOT search any other
  location. A caller that needs the consumer and its own verifier to judge the same bytes with
  one Git MUST put that Git's directory first in the consumer's `PATH`; the Tasks gate does so
  with the directory of the Git it resolves itself (TCB-V0-043).
- `CEM-V1-187`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) An
  `UNSUPPORTED` result's only diagnostic is its `stage` and `code`, both members of the closed
  `stage` and `code` enumerations of `protocol/cem-1.0/stable/RESULT.schema.json` and never a
  host path, version string or free text. A gate that pins them (TCB-V0-040) therefore records
  the cause of an unsupported route without recording the host.
- `CEM-V1-188`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) The hosted
  GitHub `macos-15` runner is a qualified host for the consumer under a caller-supplied `PATH`
  whose first Git accepts `--attr-source`: there `verify-stable-canonical` MUST return the same
  outcome and `verdictSha256` as the native verifier for the same sealed map, as on Darwin
  (local) and Linux amd64/arm64 (CEM-V1-184). Under `/usr/bin:/bin` alone that runner answers
  `repository` / `git-diff-failed`, which is the correct answer, not a defect.
- `CEM-V1-189`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1377) A test in
  the Tasks gate (TCB-V0-042) MUST run the real consumer under `PATH=/usr/bin:/bin` and accept
  only the answer that path's Git explains, so a host whose fixed-path Git loses `--attr-source`
  again, or gains it, is observed rather than assumed.

Non-goals: no consumer change, no `git --version` floor enforced by the consumer, no packet
change, no new stage or code, no containment change, no Windows statement.

Failure modes: a caller whose own Git is older than 2.41 sees `repository` / `git-diff-failed`
from the consumer and, in the Tasks gate, the native leg's own `git-diff-failed` (`stable-not-accepted`) or
`portable-route-unsupported`; neither is a grant. A caller that puts an untrusted directory first
in the consumer's `PATH` runs that directory's Git; the gate only ever puts first the directory of
the Git it already trusted for its native reads.

Rollback: revert the V1-1377 commits. The consumer is unchanged by them; the Tasks gate returns to
the fixed PATH and the hosted `macos-15` gate tests fail again with `verifier-disagreement`.

### Portable consumer Git capability traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-185 | `TestTCBV0042PortableConsumerUnderBareHostPath` (real consumer, `PATH=/usr/bin:/bin`); `TestStableCanonicalFullResults` (unchanged) | Hosted `macos-15` run 38650914390: Apple Git 2.39.5 rejects `--attr-source`, consumer `repository` / `git-diff-failed`, native `ACCEPT`; darwin/arm64 Apple Git 2.54.0 accepts and the consumer `ACCEPT`s |
| CEM-V1-186 | `interop/cem01-go/stable_repository.go` `stableGitEnv`; `TestTCBV0043PortableConsumerResolvesTheGateGit` | The consumer's recorded `PATH` and `command -v git` are the gate's Git directory first, then `/usr/bin:/bin` |
| CEM-V1-187 | `TestTCBV0040PortableUnsupportedRefusesNamingTheRoute`, `TestTCBV0016GateVerdictRunsBothConsumers` | The pinned `stage` and `code` are closed tokens; any other shape is unreadable |
| CEM-V1-188 | `TestTCBV0016GateVerdictRunsBothConsumers`, `TestTCBV0017PortableConsumerMustRecomputeTheSameVerdict` on hosted `macos-15` | Release-gates dispatch 38654510601 at 52ad86a36: macos-15 full-gate `success` (`ok internal/taskman`, `ok cmd/corvint`); dispatch 38650914390 at 22fca01e6 retained the bare-PATH answer `repository` / `git-diff-failed` under Apple Git 2.39.5 |
| CEM-V1-189 | `TestTCBV0042PortableConsumerUnderBareHostPath` | Passes on darwin/arm64 (`GRANT`) and on hosted `macos-15` (`portable-route-unsupported`) with the probe output logged |

## Stable consumer argument and reference parity (V1-1291, V1-1306, V1-1289, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-1291,
V1-1306, V1-1289. Measured at `ee27168739ec1b65dd5a653d720cf811338c37ae`, the native verifier and
the portable consumer gave different results for the same input in two places. An `arguments` or
`authority-arguments` refusal was `REJECT` (`accept` `false`) natively and `UNSUPPORTED` (`accept`
`null`) in the portable consumer. Four reference-graph mutations got a different `references` code
from each consumer. `OPERATION.json` fixes the stage and exit of both argument stages but not their
outcome, and `ALGORITHMS.md` states the graph precedence only coarsely. This section fixes both
answers so the two consumers agree. The scope is the 1.0 packet scope of the product requirement
`PRS-V1-018` (1.0 scope per V1-0020 dependency).

- `CEM-V1-115`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1291) A
  `verify-stable` refusal at stage `arguments` MUST be `UNSUPPORTED` with `accept` `null`,
  `assurance` `null` and exit 2 in every stable entry point: the native `cem verify-stable`, the
  Core stable read and export, the internal stable universe admission and both portable commands.
  A bad invocation proves nothing about the change, so it is never a rejection of the evidence.
- `CEM-V1-116`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1291) A
  `verify-stable` refusal at stage `authority-arguments` (`expected-base-required`,
  `target-required`, `artifacts-required`, `invalid-arguments`) MUST likewise be `UNSUPPORTED` with
  `accept` `null`, `assurance` `null` and exit 2 in both consumers. Its stage, code and the members
  already decoded (hunks, references) are unchanged.
- `CEM-V1-117`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1306) When an
  `invalid-arguments` refusal follows a relative path option (`--root`, `--repository`, `--map`,
  `--artifacts` or `--trusted-keys`), the consumer MUST keep refusing it (CEM-V1-017: a relative path
  is refused, never resolved) and MUST name the fix beside the result, never inside it
  (CEM-V1-187): one stderr line `<command>: invalid-arguments: relative path option <options>;
  fix: pass each as an absolute path such as "$PWD/<path>"; a relative path is refused, never
  resolved` for the native and portable commands, and the same text in a `fix` member of the Core
  stable read envelope. The absolute form of the same invocation MUST pass both argument stages and
  print no fix line. A repeated option is still refused, and the fix names a relative value in any
  of its occurrences. The closed result bytes are unchanged.
- `CEM-V1-176`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1289) Both
  consumers MUST apply the reference precedence the CEM-V1-176 paragraph of
  `protocol/cem-1.0/stable/ALGORITHMS.md` states. A hunk basis that names no evidence record is
  `wire` / `invalid-field`. A repeated hunk ID is `duplicate-reference`, after runner receipt
  uniqueness and before any link is resolved. A repeated ID inside one link array is
  `duplicate-reference` before any ID of that link is resolved. A declared criterion or runner receipt that no link names is
  `reference-coherence`, after link resolution and basis membership and before artifact role
  closure. A repeated artifact path or digest is `duplicate-reference`, before that record's role
  check. The other cases are `REJECT` at stage `references` with exit 2.
- `CEM-V1-177`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1289) The packet
  MUST carry one public case per CEM-V1-176 rule under
  `protocol/cem-1.0/stable/reference-precedence/`, added by the manifest extension rule (new
  directory and new `manifest.json` entries; no existing vector byte or digest changes), with the
  whole result line each consumer must write. Both consumers MUST run every case and match its line
  byte for byte, `runtime.goVersion` excepted.

Non-goals: no new stage or code; no change to an `ACCEPT` or a Git-reading result; no change to any
sealed S0E cell, full-result case or committed map result; the fix line is not a machine contract
beyond its prefix and is never part of `verdictSha256`; a relative path is still never resolved.
V1-1295 (partial drift on an `UNSUPPORTED` resource-limit result) and V1-1297 (a separate code for
a missing pinned object) are not decided here; the build log
`docs/build-log/2026-10-14-batch4-cem-parity.md` records why both stayed open. Decision 0532 later
resolves V1-1295 with `CEM-V1-260` and kept `git-read-failed` for V1-1297; decision 0535 then
amends `CEM-V1-103` to `repository-object-unavailable`.

Failure modes: a caller that branched on `accept` `false` for a bad invocation now sees `null`;
every in-repository caller (receipt bundle export, stable universe, Tasks gate verdict) already
treated exit 2 as not accepted. A consumer that checks a link's IDs one at a time, or the role
before the artifact repeat, fails the pinned `reference-precedence` line for that case by name in
both consumers' tests. A stderr reader that expected silence on a refusal sees one line.

Rollback: revert the V1-1291 commit. The native verifier then returns `REJECT` for argument
refusals again, the reference order and the fix line are removed from both consumers, and the
`reference-precedence/` directory and its manifest entries are deleted. No committed map, seal or
digest depends on these answers.

### Stable consumer argument and reference parity traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-115 | `TestCEMV1115ArgumentRefusalsNative` (`internal/cem/cli`); `TestCEMV1115ArgumentRefusalsPortable` (`interop/cem01-go`); `TestStableExportPublishesNothingForARefusedSource` (`internal/receiptbundle`) | The same argv gives `UNSUPPORTED` `arguments` / `invalid-arguments`, `accept` `null`, exit 2 on the native command and both portable commands; at base the native command answered `REJECT` |
| CEM-V1-116 | `TestCEMV1115ArgumentRefusalsNative`, `TestCEMV1115ArgumentRefusalsPortable`; `TestCEMV1052StableUniverseAuthorityArguments` (`internal/lrfrepo`) | `artifacts-required` and a relative `--artifacts` are `UNSUPPORTED` `authority-arguments` in both consumers |
| CEM-V1-117 | `TestCEMV1115ArgumentRefusalsNative`, `TestCEMV1117CoreReadNamesRelativePathFix` (`internal/cem/cli`); `TestCEMV1115ArgumentRefusalsPortable`; `TestCEMStableReadKeepsCallerRoot`, `TestCEMV1084VerifyStableTrustedKeysFlag` (`cmd/corvint`) | Relative `--root`, `--repository`, `--artifacts` and `--trusted-keys` print the fix line (envelope `fix` in the Core read), also when the relative value is the second of a repeated option; each absolute form passes both argument stages with empty stderr |
| CEM-V1-176 | `TestCEMV1177ReferencePrecedenceNative` (`internal/cem/verify`); `TestCEMV1177ReferencePrecedencePortable` (`interop/cem01-go`) | At base five of the six cases differed: portable `REJECT` `verification` (issue `patch-hunk-count`, exit 1) against native `references` / `duplicate-reference` for a repeated hunk ID, native `unresolved-reference` against portable `duplicate-reference` for a repeated unknown link ID, portable `unresolved-reference` against native `reference-coherence` for an unused criterion or runner receipt, and portable `artifact-role-closure` against native `duplicate-reference` for a repeated artifact |
| CEM-V1-177 | `TestCEMV1177ReferencePrecedenceNative`, `TestCEMV1177ReferencePrecedencePortable`; `TestStableS0EPublicCases`, `TestStableCanonicalFullResults` (unchanged) | Six cases and pinned lines under `protocol/cem-1.0/stable/reference-precedence/`, listed in the stable manifest; existing vectors byte-identical |

## Self-modified governing authority is never cited (V1-0414, 2026-10-14)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-0414.
Ticket V1-0414 requires that a governing file changed in the diff under review is not cited as
authority for hunks in the same change. At `495eda08cf3dc4d8122b169a4cc16d9c4e599b19` the packet
downgraded such a file (TCP-V0-057) and `cem status` reported a citation of it
(`selfModifiedAuthority`, CEM-CB-026), but `cem cite` still wrote the citation whenever the cited
span was stable: an agent could edit `AGENTS.md` and cite its unchanged lines as the
`specification` basis of that same edit. The threat and its two external references are recorded
in `task-context-packet-v0.md` ("Authority screen for reserved rows"). The scope is the 1.0 scope
per the V1-0020 dependency.

- `CEM-V1-114`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0414) `cem cite`
  MUST refuse, with the producer-only closed code `cite-self-modified-authority`, any evidence path
  that is a governing instruction file under `workflow.GoverningInstructionPath` (the CEM-CB-026
  rule, equal to TCP-V0-008's `documentKind`) and is the path of a hunk of the same map, whatever
  the span, the relation or the hunk selected. The refusal precedes the span-stability check and
  writes no map bytes. A stable span of a changed non-governing file, and any span of a governing
  file the patch does not change, are cited as before. The verifier, `cem status` and the wire are
  unchanged: a map written by another route that carries such a citation still verifies and is
  reported by `selfModifiedAuthority`.

Non-goals: no change to verification, the verdict digest or any committed map; no refusal in
`verify-stable` or `cem status`; no screen of a governing file's old path in a rename (the hunk
path is the new path, as in CEM-CB-026), of governing names `documentKind` does not classify, or of
a non-governing spec the change edits.

Failure modes: a change that edits a governing file and cites its unchanged lines now fails at
`cem cite`; its author cites another file (the spec or decision the edit implements) or keeps the
hunk `unknown` with a recorded reason. The dogfood plan runner reports the code as the `cem-cite`
reason with its own fix hint, and the self-observation ledger admits the code (both added by V1-1385
after this section was written).

Rollback: revert the V1-0414 change commit. `cem cite` then writes such citations again and
`cem status` keeps reporting them.

### Self-modified governing authority traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-114 | `TestCiteRefusesSelfModifiedGoverningAuthority`; `TestStatusReportsSelfModifiedAuthority` (hand-written citation still reported) | Disabling the refusal makes the first test fail on the `AGENTS.md` hunk; build log `2026-10-14-v1-0414-closeout-evidence.md` |

## Self-modified governing authority in `cem status` (V1-0414, 2026-10-16)

Status: accepted under the owner's standing auto-accept of lane requirements (2026-10-11); V1-0414.
Ticket V1-0414 requires that a governing file changed in the diff under review is reported as
self-modified authority in `cem status`. The report shipped with PR #467 and was specified only as
`CEM-CB-026`, an experimental requirement of `cem-0.2-canonical-binding.md`, which decision 0525
retired (`CEM-V1-225`). The `cem/1.0` status read (`internal/cem/workflow/stable_read.go`) still
emits the member, so this section restates the contract for `cem/1.0` without changing it. The
threat and its two external references are recorded in `task-context-packet-v0.md` ("Authority
screen for reserved rows") and `docs/SECURITY.md`.

- `CEM-V1-270`: (accepted under the owner's standing auto-accept (2026-10-11), 2026-10-16; V1-0414)
  For a `cem/1.0` map, `cem status` MUST report every governing instruction file the map's own
  patch changes, under `workflow.GoverningInstructionPath` (the rule of TCP-V0-008's
  `documentKind`: basename `AGENTS.md`, `CLAUDE.md`, `GEMINI.md` or `copilot-instructions.md`,
  case-insensitive, or a `.github/instructions/*.instructions.md` file). The envelope then carries
  `selfModifiedAuthority`, an array sorted by path of `{path, hunks, citedBy, reason}`: `hunks`
  lists the IDs of the hunks changing that path, and `citedBy` lists, in map order, every
  `{hunk, evidenceId, relation}` basis whose evidence row cites that path (empty when none does).
  The member is absent when the patch changes no governing file, so such an envelope is
  byte-identical to one written before the report. The report never changes `state`, `ok`, the
  verifier result, the map or any other command's output, and status refuses nothing: refusal of a
  new citation is `CEM-V1-114`'s, at `cem cite`. It supersedes `CEM-CB-026` for `cem/1.0`, which
  keeps its ID for traceability only.

Non-goals: no new member, code or state; no screen of a renamed governing file's old path (the hunk
path is the new path), of names `documentKind` does not classify, or of non-governing specs the
change edits; no hidden-Unicode screen of map evidence (the packet screens reserved rows,
TCP-V0-055).

Failure modes: a consumer that ignores the member treats a self-modifying change as ordinary; the
member is advisory beside an unchanged verdict, so CI that must block such a change reads it
explicitly.

Rollback: delete this section and its traceability row; the code and the wire are unchanged by it.

### Self-modified authority status traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-270 | `TestStatusReportsSelfModifiedAuthority`, `TestStatusOmitsSelfModifiedAuthorityWithoutAGoverningChange`, `TestSelfModifiedAuthorityListsAnUncitedGoverningChange` (`internal/cem/workflow`); `TestCEMGoverningInstructionRuleMatchesThePacket` (`internal/contextindex`) | build log `2026-10-16-v1-0414-governing-authority-acceptance.md` |

## Stable manifest entry repair (V1-1287, 2026-10-14)

The packet seal (fc5588948) wrote `protocol/cem-1.0/stable/manifest.json` before the final bytes of
two files, so two entries never matched their files: `REPOSITORY-ENVELOPE.cells.json` recorded
sha256 `c3eed8b8371322ffb6d03a7bd3065fbf956c1c6d312c92826699a406c396d257` / 302453 bytes and
`REPOSITORY-ENVELOPE.md` recorded `9c35b7a5e681c80fb148cd85d972f3200d67c838782242aa41116590a4f3dbab`
/ 9705 bytes. The first regeneration under `CEM-V1-091` (af8f466dd, the V1-1265 `citation` member)
re-bound both entries to the bytes it wrote (`ecbc86f8aab33a9548ae9cf9cb4f6e02d3198ff13d9d01a92814a78ccb1609c5`
/ 316109 and `89385b7ae4a910a4f2588264547f75353b0e30f144b7ae0851e118678b111a25` / 10003), and later
regenerations keep them current. Until then `tools/cem-stable-regen --check` deliberately checked
only the `RESULT.schema.json` entry of this manifest; it now checks every entry, as it already did
for the packet and tuple manifests, and passes on the repository packet.
`TestCheckCoversEveryStableManifestEntry` appends one byte to two stable files no other check reads
(`PACKING.md`, `tamper-matrix/README.md`) and requires `--check` to name each drifted entry; it fails
against the schema-only check. The stable manifest also lists the
`repository-envelope-packet/` files, so a regeneration now re-binds each listed packet file it
rewrites and the packet `manifest.json` there too (`TestRegenerateRealPacketCopy` checks the
result); without that a regenerated copy fails its own widened check. No sealed byte changed in
this repair. Rollback: restore the schema-only filter in `check()` and drop the packet-entry
re-binding from the stable manifest step; the manifest needs no change.

## Drift Git accounting parity (V1-1295, decision 0532, 2026-10-15)

Status: `CEM-V1-260` accepted under the owner's standing auto-accept (2026-10-11); V1-1295.

Intent: two stable consumers that read the same map at the same commits under the same 1,024
logical Git operation limit write the same result, including a result the limit cuts short in the
drift stage. V1-1295 measured the f0fda542 map ending `UNSUPPORTED` `repository`
`unsupported-resource-limit` with 68 drift rows on the native verifier and 102 on the portable
consumer, so their `verdictSha256` differed. V1-1295 (2026-10-14) had already kept the partial drift
list in an `UNSUPPORTED` result, because the sealed S0E full-result cases pin one; what remained
unpinned was the drift stage's own per-record charge. The native verifier read each changed
record's base blob a second time (three operations per record); the portable consumer searched the
span bytes the evidence stage had already authenticated (two). With the native re-read removed both
consumers write 102 rows and verdict
`92747a603414f0d4a08dd4ce10df19afeec2d00bbec18e7a2d6eda16a4db2d10`, byte-identical results.

- `CEM-V1-260`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1295) Drift
  (`protocol/cem-1.0/stable/ALGORITHMS.md` item 12) MUST charge, for each evidence record in
  evidence-ID order, one target path lookup, and one target blob read only when the target entry is
  a 100644 or 100755 blob whose OID differs from the record's `blobOid`. The span bytes it searches
  MUST be the bytes the evidence stage authenticated; drift MUST NOT read a base blob. When the
  operation limit refuses an operation during drift, the result is `UNSUPPORTED` `repository`
  `unsupported-resource-limit` and keeps exactly the rows completed before the refused operation, so
  every conforming consumer writes the same `drift` list and `verdictSha256`. The packet MUST carry
  the public case `protocol/cem-1.0/stable/drift-resource-limit/`, added by the manifest extension
  rule (a new directory and new `manifest.json` entries; no existing vector byte or digest changes
  other than `ALGORITHMS.md`, whose item 12 states the charge), with the whole result line. The case
  declares a lowered operation limit (`operations`, 113) that each consumer applies only through its
  in-package test seam (`stableSeam.operations` natively, `stableHooks.callLimit` in
  `interop/cem01-go`); production always uses 1,024. Both consumers MUST run it and match the line
  byte for byte, `runtime.goVersion` excepted.

Non-goals: no change to the limit, to any other stage's charge, to the drift statuses or to an
`ACCEPT`, `REJECT` or completed drift result; no change to any sealed S0E cell or full-result case
(their partial lists hold only `stable` rows, which read no blob, so both charges agree on them); no
dropping of the partial drift list from an `UNSUPPORTED` result.

Failure modes: a consumer that re-reads a base blob or skips a record's target lookup stops at a
different row on this case and fails it. Every record of the case cites a changed blob, so a
consumer that also read the target blob of an unchanged record (identical OID) is not caught by
it. A record whose authenticated span length is not its declared span length fails closed
`evidence-unavailable` rather than searching other bytes. A case that spent the full 1,024
operations was too slow to run (14 minutes natively; the portable consumer passed the 30-minute
verification deadline at load 65 to 76), hence the lowered seam limit; a charge difference that
shows only past operation 113 is caught by the f0fda542 re-run above, not by the case. The case is
skipped only where stable process containment is not qualified.

Acceptance evidence: on the case (limit 113) the fixed native verifier and the portable consumer
write byte-identical lines, 13 drift rows and verdict
`a5611d4da50659d8f87988a89f7515413b6ed12df21abc801760b866f17d33d6`; restoring the base re-read
makes the native verifier write 8 rows and fail its case test. At the full 1,024 limit on a 340-record
form of the same map the BASE native verifier (253d6341f) wrote 103 rows and the fixed one 154.
Build log `docs/build-log/2026-10-15-cem-stable-consumer-residuals.md`.

Rollback: revert the change commit. The native verifier then re-reads base blobs in drift again, the
case and its manifest entries go, and V1-1295 reopens as a recorded difference.

### Drift Git accounting traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-260 | `TestCEMV1260DriftResourceLimitNative` (`internal/cem/verify`); `TestCEMV1260DriftResourceLimitPortable` (`interop/cem01-go`); `checkDrift`, `driftItem` in `internal/cem/verify/verify.go` | f0fda542: native 68 to 102 rows, byte-identical to portable; the public case: native and portable 13 rows, byte-identical; base re-read mutation 8 rows; build log `2026-10-15-cem-stable-consumer-residuals.md` |

## Protocol pointer gate (V1-1296, decision 0532, 2026-10-15)

Status: `CEM-V1-261` accepted under the owner's standing auto-accept (2026-10-11); V1-1296.

Intent: a digest-pinned file pointer in the `protocol/cem-1.0` packet either resolves or is a
recorded, explained exception. V1-1296 found `stable/MIGRATION.json` naming a missing
`../migration-plan.json`; the scan also found the same member in the packet's inherited S0E copy and
`repository-envelope-packet/INPUTS.json` naming a missing `../REVIEW-of-repair5.json`. All three sit
in sealed packet bytes, pinned by the stable manifest and the packet's input list, and are marked
informative there. Removing them rewrites sealed bytes, so decision 0532 records them and leaves
their removal to the owner. Decision 0535 (owner delegated to the orchestrator, 2026-10-16)
declines the removal: sealed evidence stays immutable, and the three pointers stay recorded
exceptions that `--check` reports.

- `CEM-V1-261`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1296)
  `tools/cem-stable-regen --check` MUST refuse a dangling protocol pointer. A pointer is a string
  member `K` of a JSON object under `protocol/cem-1.0` whose object also has a `K` + `SHA256` or
  `K` + `Sha256` string, or a `path` member starting `../` beside a `sha256` string; a document
  whose top-level `spec` starts `cem/` is wire data and is not scanned, and a file that is not JSON
  carries no pointer. Each pointer MUST resolve, relative to its file's directory and inside the
  repository with no symbolic link on the way, to a regular file whose SHA-256 is the pinned one. The only exceptions are the recorded
  dangling pointers, each named by file and JSON pointer with its exact text; a recorded pointer
  that resolves, changes text or disappears from a file that still exists is itself refused, so the
  record cannot outlive its reason.

Non-goals: no change to any sealed packet byte, manifest or digest; no pointer kinds outside the
two shapes above; no scan outside `protocol/cem-1.0`.

Failure modes: a pointer written in another shape is not checked; the recorded exceptions stay
dangling by decision (decision 0535 declined their removal).

Acceptance evidence: `--check` passes on the repository with the three recorded pointers and
reports exactly three problems without the record. Removing the gate call from `check` makes
`TestCEMV1261CheckRunsPointerGate` fail, and reading the target without the symbolic-link check
makes `TestCEMV1261PointerSymlinkRefused` fail. Build log
`docs/build-log/2026-10-15-cem-stable-consumer-residuals.md`.

Rollback: revert the change commit; `--check` then ignores protocol pointers.

### Protocol pointer gate traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-261 | `TestCEMV1261PointerProblems`, `TestCEMV1261RepositoryPointersRecorded`, `TestCEMV1261PointerSymlinkRefused`, `TestCEMV1261CheckRunsPointerGate` (`tools/cem-stable-regen`); `pointerProblems` in `tools/cem-stable-regen/pointers.go` | Three recorded sealed pointers; build log `2026-10-15-cem-stable-consumer-residuals.md` |

## Verdict ledger (V1-1292, decision 0523 K6)

Status: `CEM-V1-271..279` accepted under the owner's standing auto-accept (2026-10-11); V1-1292.

Intent: an operator who anchors a stable map can also keep, beside the pointer, the verification
verdict the native consumer reached for that map on that commit, so a later disagreement (another
artifact set, another verifier release) is detected instead of silently replacing the earlier
result. Before this change the `CEM-V1-065` row and the `cem/v2` non-goals said "no verdict
ledger"; the anchor pointer (`CEM-V1-067`) records which map describes a commit, never what a
verifier concluded. The precedent the ledger stands on is decision 0523 K6: native and portable
consumers agree on `verdictSha256` (`CEM-V1-066`) for every public stable case before any record is
written. That precondition already holds through the `CEM-V1-066` tests; this change does not
re-qualify it.

- `CEM-V1-271`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) The
  verdict ledger MUST be kept as Git notes under `refs/notes/corvint-verdicts`, a sibling of the
  anchor ref `refs/notes/corvint` in the Corvint notes namespace (a ref below
  `refs/notes/corvint/` cannot coexist with `refs/notes/corvint`, and the anchor note stays one
  pointer object). Each target commit has at most one ledger note: a sequence of LF-terminated
  lines, each exactly one canonical `corvint-cem-verdict/1` record, at most 256 records. The
  recorded digest is the `CEM-V1-066` `verdictSha256`. The `refs/notes/corvint` pointer note and
  its `CEM-V1-067` reads are unchanged.
- `CEM-V1-272`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) A record
  MUST be the closed object with exactly these members in this order and no others: `schema`
  (`corvint-cem-verdict/1`), `mapSha256` (the anchored map's SHA-256), `target` (`gitCommit`, the
  annotated commit, and `gitTree`, its tree, both full lowercase object IDs), `outcome` (`ACCEPT`
  or `REJECT`), `verdictSha256` (64 lowercase hexadecimal), `consumer` (`native`), `runtime`
  (`goVersion`, the verifier's Go release, printable ASCII of 1..256 bytes), `recordedAt` (RFC
  3339 UTC with second precision) and `previousRecordSha256` (null for the first line of the note,
  otherwise the SHA-256 of the previous line's exact bytes). Its line MUST be the compact JSON
  encoding of those members (Go `encoding/json`, no insignificant whitespace); a line whose bytes
  differ from the re-encoding of its decoded record does not verify.
- `CEM-V1-273`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) `cem
  anchor --map MAP [--commit REV] --artifacts DIR` MUST, after every `CEM-V1-067` check, run the
  native stable verifier over the exact committed map bytes with the map's `baseRevision` as the
  expected base, REV as the target and DIR as the artifact root, as `cem verify` does, and append
  one record per (`mapSha256`, target commit). Its result gains a `verdict` member (`ref`,
  `written`, `record`, `state`). Without `--artifacts`, anchor runs no verifier, writes no ledger
  note and its result has no `verdict` member.
- `CEM-V1-274`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) Only an
  `ACCEPT` or `REJECT` result MUST be recorded. Recording again a `verdictSha256` equal to the
  one already recorded for the same key MUST write nothing (`written` false, the existing record
  returned). A record for a new key MUST be appended after the existing lines, whose bytes are
  kept, with `previousRecordSha256` naming the last of them.
- `CEM-V1-275`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) Anchor
  with `--artifacts` MUST refuse, writing neither the pointer note nor the ledger note, with:
  `verdict-ledger-conflict` when the target's ledger already records a different
  `verdictSha256` for the map (a recorded verdict is never replaced);
  `verdict-ledger-malformed` when the target's existing ledger note does not read `verified`;
  `verdict-ledger-unsupported-result` when the verifier reaches no verdict (`UNSUPPORTED`, for
  example a relative or missing artifact root); and `verdict-ledger-full` when the note already
  holds 256 records. Every refusal of the pointer or of the verdict MUST be decided before any
  note is written. Anchor MUST build each new notes commit on the ref tip it validated and publish
  the pointer ref and the ledger ref in one atomic `git update-ref --stdin` transaction whose old
  values are those tips (the zero object ID for an absent ref), so a lock, a failure or a lost race
  leaves both refs unchanged. A lost race MUST be refused after re-reading the tips, with
  `verdict-ledger-conflict` when the ledger ref moved and `anchor-note-conflict` when only the
  pointer ref moved; nothing is overwritten.
- `CEM-V1-276`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) A ledger
  read MUST report exactly one state: `absent` (no note), `verified`, `malformed` (empty note, no
  final LF, more than 256 lines, a line that is not one canonical well-formed record),
  `chain-broken` (a `previousRecordSha256` link that is not null on the first line or not the
  previous line's digest), `scope-mismatch` (a record naming another commit or tree than the
  annotated commit) or `conflict` (a repeated `mapSha256`), in that precedence.
- `CEM-V1-277`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) `cem
  provenance --commit REV --verdicts` MUST add a `verdicts` member (`ref`, `trust`
  `repository-history`, `authority` `git-history`, `state`, `records`) listing REV's records
  only when the state is `verified`, and an empty list otherwise. Without `--verdicts` the result
  has no `verdicts` member. The listing MUST write no ref, note, object, index, config or
  worktree file.
- `CEM-V1-278`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) The
  ledger MUST carry no authority: a record changes no anchor or provenance row state, no `cem
  verify`, `cem status` or export result, tier, ranking or learning input. Only an explicit `cem
  anchor --artifacts` writes it; no read command writes it (product invariant 4).
- `CEM-V1-279`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1292) The CLI
  surface MUST be the two flags, with no new `cem` action: `cem anchor --map MAP [--commit REV]
  [--artifacts DIR]` and `cem provenance --commit REV [--notes-tip OID] [--verdicts]`, both shown
  in `corvint cem --help`.

Wire and schema: new note ref and record profile `corvint-cem-verdict/1`; the anchor result gains
the optional `verdict` member and the provenance result the optional `verdicts` member. The stable
map, `cem-stable-verification/1`, the anchor pointer profile and every provenance row are
unchanged, so the change is additive.

Non-goals: no portable writer and no portable reader of the ledger (the portable consumer agrees on
`verdictSha256` by `CEM-V1-066`; only the native consumer writes); no retry of a lost race; no
`--notes-tip` rollback check for the ledger ref; no signing; no default-on recording; no automatic removal or migration of a conflicting record.

Failure modes: a differing verdict for an anchored map blocks recording with
`verdict-ledger-conflict`; the operator finds the cause (artifacts, verifier release) and removes
the note deliberately (`git notes --ref=refs/notes/corvint-verdicts remove REV`) before recording
again. An edited, reordered or truncated ledger note reads `malformed`, `chain-broken`,
`scope-mismatch` or `conflict`, lists no records, and is never extended. A deleted or reset ledger
ref reads `absent`; the chain detects edits inside one note, not removal of a whole note. Two
concurrent anchors of the same commit cannot both publish: the second compare-and-swap fails, and
that anchor is refused `verdict-ledger-conflict` with nothing written; rerunning it reads the
winner's ledger. A crash between building a notes commit and publishing it leaves at most a
transient `refs/notes/corvint-staging/` ref and unreachable objects, never a moved public ref.

Acceptance evidence: the tests below, each shown failing under a mutation of its guard; build log
`docs/build-log/2026-10-20-V1-1292-cem-verdict-ledger.md`.

Rollback: revert the V1-1292 commit. `--artifacts` and `--verdicts` become unrecognized
arguments; existing `refs/notes/corvint-verdicts` notes stay in the repository, unread, and can be
removed with `git update-ref -d refs/notes/corvint-verdicts`. No committed map, seal or golden
depends on the ledger.

### Verdict ledger traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-271 | `TestCEMV1273AnchorAppendsOneVerdictRecordAndRepeatsIdempotently` (`internal/gitnotes`); `VerdictRef`, `readLedger` in `internal/gitnotes/verdict.go` | The one-line note is written under `refs/notes/corvint-verdicts` and the anchor receipt stays `ok` (its pointer reads back `verified`) |
| CEM-V1-272 | `TestCEMV1276TamperedLedgerIsNeitherListedNorExtended` (`internal/gitnotes`); `VerdictRecord`, `wellFormedRecord` in `internal/gitnotes/verdict.go` | An unknown member, non-canonical spacing, a missing final LF and an unknown outcome each read `malformed` |
| CEM-V1-273 | `TestCEMV1273AnchorAppendsOneVerdictRecordAndRepeatsIdempotently` (`internal/gitnotes`); `TestCEMAnchorAndProvenanceInteropThroughTheCLI` (`cmd/corvint`); `AnchorWithVerdict` in `internal/gitnotes/anchor.go` | The record's `verdictSha256` equals the digest of an independent `verify.Stable` run over the same inputs |
| CEM-V1-274 | `TestCEMV1274AppendLinksToThePreviousRecord`, `TestCEMV1274UnsupportedResultWritesNothing`, `TestCEMV1273AnchorAppendsOneVerdictRecordAndRepeatsIdempotently` (`internal/gitnotes`); `planAppend` | A repeat leaves every ref, note and object unchanged and returns the first `recordedAt`; a second key keeps the first line's bytes and links to its digest; a relative artifact root writes no note |
| CEM-V1-275 | `TestCEMV1275DifferingVerdictIsRefused`, `TestCEMV1275FullLedgerIsRefused`, `TestCEMV1275LostRaceIsRefusedWithoutOverwrite`, `TestCEMV1275LockedLedgerRefLeavesThePointerUnwritten`, `TestCEMV1274UnsupportedResultWritesNothing`, `TestCEMV1276TamperedLedgerIsNeitherListedNorExtended` (`internal/gitnotes`); `planAppend` | `verdict-ledger-conflict` creates no `refs/notes/corvint` pointer; each refusal leaves every ref, note and object unchanged; a 256-record ledger plans no body; a writer interleaved between validation and publish wins and the other is refused with the winner's ledger intact; a locked ledger ref leaves the pointer ref unwritten |
| CEM-V1-276 | `TestCEMV1276TamperedLedgerIsNeitherListedNorExtended` (`internal/gitnotes`); `readLedger` | Eight tamper cases read `malformed`, `chain-broken`, `scope-mismatch` or `conflict` |
| CEM-V1-277 | `TestCEMV1277VerdictListingIsReadOnly` (`internal/gitnotes`); `TestCEMAnchorAndProvenanceInteropThroughTheCLI` (`cmd/corvint`); `ProvenanceWithVerdicts` in `internal/gitnotes/provenance.go` | Refs, notes and objects are unchanged across the listing; no `verdicts` member without the flag |
| CEM-V1-278 | `TestCEMV1277VerdictListingIsReadOnly` (`internal/gitnotes`); `readLedger` is called only by `AnchorWithVerdict`, `appendLedger` and `verdictsMember` in `internal/gitnotes` | No verifier, status, export, tier or ranking path reads `refs/notes/corvint-verdicts`; the listing reports `mutates` false and changes nothing |
| CEM-V1-279 | `TestCEMAnchorAndProvenanceInteropThroughTheCLI` (`cmd/corvint`); `TestAnchorActionsParseLikeTheirSiblings` (`internal/cem/cli`) | The help lists both flags and the action list is unchanged |
## Capsule and dashboard stable positive cases (V1-0632, 2026-10-20)

Status: `CEM-V1-290..299` accepted under the owner's standing auto-accept (2026-10-11); V1-0632
audit gap 1.

Intent: the dashboard (C17) and the local-authority capsule (C15) each read an actual `cem/1.0`
change map, not only refuse one. Each keeps the exact raw stable graph, leaves artifacts opaque and
retains the verifier's whole result and its unknown authority axes. A stale or incomplete binding
refuses without losing evidence. This amends `CEM-V1-167` and the failure mode that left both
families without a stable path. The frozen `corvint-dashboard-snapshot/0` schema, its hash-pinned
registry and the retired bundle flags (`CEM-V1-168`, `CEM-V1-217`) are unchanged.

- `CEM-V1-290`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632)
  `corvint-dashboard-snapshot cem-stable --root R --cem C --ocm O --expected-base B --target T
  --artifact-root A` MUST admit the map through `lrfrepo.VerifyStableUniverse` and, on success,
  write one `corvint-dashboard-cem-stable/0` document to stdout with exit 0. It carries the input
  paths and SHA-256 digests, spec, map digest, base and target revisions, object format, patch and
  OCM digests, intent scope, obligations, the verifier's whole `verify.StableResult` as
  `verification`, the exact original map bytes as standard base64 in `mapBase64`, whose SHA-256
  MUST equal `mapSha256` (final LF and formatting included), and `taskCompletion` and
  `criterionSatisfaction` as `NOT_OBSERVED`. A hunk's coverage witness, present or absent, MUST
  survive in `mapBase64` unchanged.
- `CEM-V1-291`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The
  `cem-stable` subcommand MUST NOT change the snapshot command: its schema, adapter registry and
  hash, error profile, and the `DASHBOARD_INVALID_ARGUMENT` refusal of every bundle flag and profile
  stay as `CEM-V1-168` and `CEM-V1-217` state.
- `CEM-V1-292`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) Each of
  the six flags MUST appear exactly once; `--root` and `--artifact-root` MUST be absolute and
  clean, and `--cem` and `--ocm` relative and clean. Any other form MUST be
  `DASHBOARD_INVALID_ARGUMENT` with no stdout before any path opens. A map or OCM that is missing,
  not a regular file or larger than the stable map bound MUST be `DASHBOARD_REPOSITORY_UNAVAILABLE`
  with no stdout. The read MUST NOT follow a symlink in any path component under `--root`: each
  directory opens through its parent's descriptor and must be the directory `Lstat` saw, and the
  file opens non-blocking and its descriptor must be the regular file `Lstat` saw, so a symlinked
  parent or final component, a FIFO, or a component swapped after `Lstat` refuses without
  blocking.
- `CEM-V1-293`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) A refused
  admission MUST write no stdout, exit 2, and write one `corvint-dashboard-cem-stable-refusal/0`
  document to stderr with the refusal code, the input paths and digests, and `verification` set
  to the verifier's whole result when the verifier ran, else `null`. A stale base, a stale target,
  an edited map, a tampered or missing artifact, an OCM bound to another map and a retired `cem/0.x`
  map each refuse.
- `CEM-V1-294`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The
  subcommand MUST NOT write any repository, map, OCM or artifact byte, and MUST NOT derive task
  completion, criterion satisfaction or any authority axis that the verifier leaves unknown.
- `CEM-V1-295`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) Amending
  `CEM-V1-167`, the local-authority capsule MUST parse the enrolled `cem.json` with the stable
  decoder. A retired `cem/0.x` map keeps `unsupported-spec` with the fix line (`CEM-V1-210`). A
  `cem/1.0` map whose base is not the enrolled base MUST refuse `capsule CEM base`. A matching map
  proceeds to the object demand.
- `CEM-V1-296`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The
  capsule's stable closure MUST account one artifact for each of `tasks-capture`,
  `tasks-verification`, `tasks-claimed-ticket`, `tasks-snapshot-head-receipt`, `runner-plan` and
  `runner-receipt`, in that order, and the SHA-256 of the exact enrolled map bytes. A missing role
  MUST refuse `capsule stable role missing`. The capsule MUST NOT read artifact bytes.
- `CEM-V1-297`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The object
  demand MUST require the base and target trees. It requires each evidence path at the base with
  the map's blob OID (`capsule CEM blob binding`), the OCM intent path at base and target, each OCM
  claim path at the target, each check's driver at base and target and its subject at the target.
  It requires the committed `.corvint/change.cem.json` at the target, whose blob MUST equal the
  enrolled `cem.json` bytes (`capsule CEM sidecar binding`). An absent path refuses
  `required capsule path missing`, and an absent blob `changed blob missing`.
- `CEM-V1-298`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) The
  capsule closure confers no authority. Acceptance comes only from the stable verifier run over the
  published minimal evidence view, with the artifact root supplied separately. Over that view, the
  verifier MUST return the same map digest, all six artifact checks, an `EXACT` sidecar, every
  result key, and the unknown axes as `NOT_OBSERVED`. A tampered artifact the closure still admits
  MUST be rejected `artifact-digest-mismatch` by that verifier.
- `CEM-V1-299`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0632) Every
  capsule refusal in `CEM-V1-295..297` MUST happen before evidence publication and leave the
  enrolled bytes unchanged.

Non-goals:
- no new snapshot schema version or registry row, and no bundle route;
- no capsule-native stable verifier;
- no enrollment format that carries artifact bytes;
- no native Tasks authority, authentication or runner execution claim;
- no C15 stale-live-gate control: the capsule holds no live Tasks gate to go stale.

Failure modes:
- When the committed sidecar is not exact, `VerifyStableUniverse` returns a plain
  `excluded-artifact-mismatch` without a result. The dashboard refusal then keeps the input digests
  and code, with `verification: null`. The same holds for an OCM failure and a retired map.
- The protected enrollment command still accepts only a codec-package target tree
  (`onlyCodecPackage`), and it needs the root-owned authority store, so a full protected enrollment
  of a stable change was not run.

Acceptance evidence: the tests below pass, and each fails without its fix. With the dispatch
removed, the dashboard tests see `DASHBOARD_INVALID_ARGUMENT`. With the base capsule closure and a
stub, the capsule tests see `capsule CEM base`. With the first reader, a symlinked parent is
admitted (exit 0), a swapped-in FIFO blocks the open, and the emitted map is compacted without its
final LF. Without the check demand the driver control admits; without the subject demand the
subject control admits; without the OID comparison the mismatch reaches `capsule CEM sidecar
binding`. Build log
`docs/build-log/2026-10-20-V1-0632-capsule-dashboard-positive.md`.

Rollback: revert the change commit. The dashboard then has no `cem-stable` subcommand, and the
capsule refuses every `cem/1.0` enrollment with `capsule CEM base` again.

### Capsule and dashboard stable positive traceability

| Requirement | Test | Evidence |
| --- | --- | --- |
| CEM-V1-290 | `TestCEMV1290DashboardCEMStableReadsRealUniverse` (`cmd/corvint-dashboard-snapshot`) | Strict key set; decoded `mapBase64` equals the raw on-disk stable map, final LF included, and its SHA-256 is `mapSha256`, with and without a witness; `verification` equals an independent `verify.Stable` marshal with 23 keys, 6 artifact checks, `EXACT` sidecar |
| CEM-V1-291 | `TestCEMV1168DashboardRefusesStableBundle`, `TestCEMV1290DashboardCEMStableReadsRealUniverse` | Bundle flags still refuse; snapshot package suite and `conformance/dashboard-snapshot-v0` pass unchanged |
| CEM-V1-292 | `TestCEMV1293DashboardCEMStableRefusesStaleOrIncompleteBindings`, `TestCEMV1292DashboardCEMStableInputStaysUnderRoot`, `TestCEMV1292DashboardCEMStableRefusesSwappedFIFO` | Four bad-argument forms are `DASHBOARD_INVALID_ARGUMENT` with no stdout; a symlinked parent (map or OCM), a symlinked final component and a FIFO are `DASHBOARD_REPOSITORY_UNAVAILABLE` within 10 s; a FIFO swapped in after `Lstat` refuses without blocking |
| CEM-V1-293 | `TestCEMV1293DashboardCEMStableRefusesStaleOrIncompleteBindings` | Seven controls refuse with code, input digests, and the verifier result when it ran |
| CEM-V1-294 | `TestCEMV1290DashboardCEMStableReadsRealUniverse`, `TestCEMV1293DashboardCEMStableRefusesStaleOrIncompleteBindings` | `NOT_OBSERVED` completion and axes; inputs byte-identical after every refusal |
| CEM-V1-295 | `TestCEMV1167CapsuleRefusesStableEnrollment`, `TestCEMV1297CapsuleRefusesStableClosureControls` (`tools/local-authority`) | 0.x `unsupported-spec` with fix line; stale base `capsule CEM base` |
| CEM-V1-296 | `TestCEMV1295CapsuleAdmitsStableClosure`, `TestCEMV1297CapsuleRefusesStableClosureControls` | Six roles in order, map digest of exact bytes; producer map `capsule stable role missing` |
| CEM-V1-297 | `TestCEMV1295CapsuleAdmitsStableClosure`, `TestCEMV1297CapsuleRefusesStableClosureControls` | Minimal capsule admits with and without two checks; other sidecar, missing sidecar and missing evidence blob refuse; an evidence OID naming another carried blob is `capsule CEM blob binding`; a missing driver blob and a missing subject blob of an unchanged, uncited path are each `changed blob missing`; an absent driver path is `required capsule path missing` |
| CEM-V1-298 | `TestCEMV1295CapsuleAdmitsStableClosure` | Published view `ACCEPT` with the same keys and axes as the direct run; tampered artifact `REJECT` `artifact-digest-mismatch` |
| CEM-V1-299 | `TestCEMV1297CapsuleRefusesStableClosureControls` | Every refusal from the closure step, which precedes publication; enrolled map unchanged |
