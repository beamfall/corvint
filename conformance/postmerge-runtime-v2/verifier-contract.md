# /2 native verification contract — private Gate A input

Status: PRIVATE EXECUTABLE DESIGN, NOT IMPLEMENTED OR QUALIFIED. Exact owner direction accepted in ../positive-v2-decision/owner-acceptance.json, artifact ea5dac157e5e7ae6f2ad865a0ed639fa90bb6d5697d192b58a8f1ec023e48bfb. No change to /0 or /1. Schemas are proposed successor wires, not serialization of existing Go structs under their old names. `sem.*` schema definitions describe new named producer-owned Go decision types. Their source-field correspondence is explicit in typed-field-map.json.

## Source binding

Base DD8 commit dd8cc0ca918a80faaa041c98a783de11550b2bcc, tree4809abd1294fbd82ce73d8e0e7276757e234f514. Primary mixed tree excluded. #394 prepared source e5d777cbd1202dbffb88ce114956b304035bd5c5; complete UNAPPLIED11file patch da35448e8644051b63a562f05b80b8a083fc73df924d28785449b67ce983f6e9 ontoDD8. No assumption that either is already integrated. Root owns exact future admission/rebase. Producer types9c7735106b862e37024d041e6d601484f65a82c327fb13b2a36d494494703804; report8f4720f621c0067874efb1ec7d319c02888a5471894cf7454a2d656d74e00705; prepared freshness62b55e5b8500d1938f00d70442baf0c414d69661dfc9bc7d8dcb29a7d5066611. Typed wire inventory records75 reachable source types and every field. Its lexical extraction is preparation, not proof of the future generator or an admitted gate.

## Wire identities and bounds

Decision: corvint-new-e2e-decision/2. Attachment: corvint-new-e2e-attachment/2. Consumer join: postmerge-connector-evidence/2. Workflow dispatch: postmerge-replay/2. Existing native assessment/request/provider/control schemas stay exact; no change to their decoder or raw output bytes. Newly proposed Go decision types use separate names and contain only the closed semantic fields in decision.schema.json.

JSON must be UTF8 without BOM, duplicate/case aliases, invalid Unicode, unknown members, fractional decision numbers or trailing values. Preserve original scalar kinds, nullable pointers, absent optional fields, empty versus null arrays/maps and every observed array order, except the specifically named arrays of already verified logical process witnesses in M1 below. Decoders validate presence before typed decoding; caller-written true/seal markers are not trusted. Native raw JSON remains exact and uses each existing strict decoder, including native float duration syntax.

Decision identity bytes are existing wire.CanonicalValue(wire.Parse(decisionJSON)); no trailing LF, keys lexically sorted, arrays in observed producer order except the explicit verified logical-process witness ordering in typed-field-map.json; native schedules are never sorted. No new approximate canonicalizer. Float durations map to measurement state before canonicalization. Arbitrary ProjectIdentity.use JSON becomes exact base64 preimage in this new decision schema, with closed nested approved configuration schema and config identity checked; no semantic loss through float rounding. Any remaining noninteger value in a stable decision is unsupported under this initial profile, rather than silently rounded. Decision digest: lowercase SHA256 of ASCII `corvint-new-e2e-decision/2` + one NUL byte + canonical decision bytes. Attachments use analogous domain `corvint-new-e2e-attachment/2`. Semantic dependency-node digest uses `corvint-new-e2e-semantic-node/2` +NUL+ canonical {kind,locator,association,payload} from the exhaustive semantic-links.json table. Payload is the exact named typed subtree, excluding its containing wrapper; association and canonical literal locator participate. Before/after attestation targets exclude enclosing outputDigest. Nested control evidence excludes sibling receipt_digest. Hash in frozen attestation→provider→hook→control-evidence→decision order; no general resolver or self/containing-node cycle. The complete synthetic finite link example retains all11target canonical preimages and21references without claiming native execution.

Manifest-pinned admitted HTTPS origin plus fixed `/decisions/sha256/<64lowerhex>` identifies the exact decision preimage. Require rawURL==canonicalOrigin+thatPath, no userinfo/query/fragment/encoded-path aliases/redirect/latest. Resolver has only `ResolveDecision(origin,digest)` semantics; local candidate mode reads exclusively pinned bounded nofollow immutable blobs, performs no HTTP/network lookup and does not establish remote availability. Operational resolver is outside this candidate scope.

Bounds: input request4MiB, original raw assessment16MiB, decision16MiB, each raw artifact32MiB except original producer stricter limits, total attachment preimages128MiB,8192 artifact nodes,32768 edges,4096 semantic provider/hook/process nodes,64 nested levels, paths4096 bytes. Host/stdout/timeout policy must also admit these finite limits before execution. Overflow yields BLOCKED; never truncates into accepted proof. All files regular exclusive nofollow0700 directories/0600 bytes with identity/readback and no overwrite. No source or canonical request changes to repair a digest mismatch.

## Exact native entrypoints to add

The entrypoints below are proposed implementable interfaces; existing ones are called by their actual names. Their outputs carry unexported verification state, so callers cannot construct a successful token. Constructors run verification, never accept a caller bool.

```
// internal/testacceptance/decision_v2.go (new; producer owns all types)
type ArtifactReader interface { ReadArtifact(id string, sha256 string, bytes int64) ([]byte, error) }
type DecisionInputsV2 struct {
    Identity DecisionIdentityV2 // exact decision.schema.json identity object
    RequestBytes []byte
    ReportBytes []byte
    NativeGraph NativeProofGraphV2 // exact artifact/edge types from attachment schema;
                                 // roots request/report/body/host/process, no connector root yet
}
// Every field has a new concrete Go type traced to its closed schema definition;
// no json.RawMessage/any field is an authority input.
type VerifyInputsV2 struct {
    Native DecisionInputsV2
    DecisionBytes []byte
    AttachmentBytes []byte
    CanonicalRequestJSONL []byte
}
type VerifiedDecisionV2 struct { /* unexported raw-binding and independently recomputed canonical bytes */ }
func VerifyDecisionV2(ctx context.Context, in VerifyInputsV2, artifacts ArtifactReader,
    processAdmission postmergeproof.ProcessAdmissionV2) (VerifiedDecisionV2, error)
func (v VerifiedDecisionV2) CanonicalBytes() []byte // copy
func (v VerifiedDecisionV2) SHA256() string
func ExportDecisionV2(ctx context.Context, in DecisionInputsV2, artifacts ArtifactReader,
    processAdmission postmergeproof.ProcessAdmissionV2) (DecisionArtifactV2, error) // same private native recomputation,
                                                               // no canonical-request/attachment yet
func SealAttachmentV2(ctx context.Context, native DecisionInputsV2,
    decision VerifiedNativeDecisionV2, canonicalJSONL []byte,
    artifacts ArtifactReader) (AttachmentManifestV2, error)
// VerifiedNativeDecisionV2 is the unexported-state result of the common native verifier;
// export serializes its bytes. Final consumer VerifyDecisionV2 checks the sealed join again.

// internal/postmergeproof owns concrete wire AND verification (no Tasks imports).
// Complete tagged types: process-types.contract.go and closed process schemas.
// Host/procgroup collectors cannot initialize private VerifiedProcessesV2 fields.
// No ProcessVerifier interface/caller map/acceptance callback exists.
func VerifyProcessV2(ctx context.Context, admission ProcessAdmissionV2,
    expected GraphBindingV2, policyBytes, qualificationBytes, proofBytes []byte,
    store ArtifactReaderV2) (VerifiedProcessesV2, error)
// Concrete final verifier branch solely mints private token; zero or wrong binding fails.
// Native producer invokes postmergeproof.VerifyProcessV2 from verified native inputs.

// internal/postmergeconnector/evidence_v2.go (new separate evidence admission)
func BuildEvidenceV2(ctx context.Context, repo string, f Fixture, p Policy,
    input Input, evidence EvidenceInputV2, verifier EvidenceVerifierV2) (EvidencePlanV2, error)
func ValidateEvidencePlanV2(ctx context.Context, repo string, f Fixture, p Policy,
    plan EvidencePlanV2, verifier EvidenceVerifierV2) error
func RecordEvidenceV2(ctx context.Context, repo string, f Fixture, p Policy,
    plan EvidencePlanV2, ledger *State, writer Writer, verifier EvidenceVerifierV2) error
// EvidenceVerifierV2 is the pinned native testacceptance implementation, not arbitrary adapter verdict JSON.
// EvidencePlanV2 owns unexported checked Plan and join. Every entrypoint independently revalidates preimages.
// Old Build, ValidatePlan, Record are unchanged. /2 runtime uses only these new wrappers for test drafts.
```

No interface exposes a source-supplied acceptance callback. Implementing a plugin-injected verifier is unsupported. The concrete workflow constructor pins the native verifier implementation and host profile; it does not accept an arbitrary implementation from fixture/expected JSON. Small postmergeproof package removes producer↔host↔workflow import cycles and owns closed transport-neutral proof types plus concrete bounded verification/role mapping, as specified in process-verification.md. It grants no authority. Host/procgroup own native collection, not token constructors. All preimages/joins/finite role recipes are closed; current sampled-only observations remain unsupported and qualification remains NOT_OBSERVED.

## Verification sequence — no execution during offline recheck

1. Validate independent runtime manifest, exact immutable source/tool/config/input/test/product/draft inventories, policy, no expectations in admitted reader/author mounts, zero credential scope, logical mounts and host qualification identity. Request canonical digest is testacceptance.Digest(decoded request), checked against approved actual request bytes and producer report. Raw byte hash and native request digest remain distinct, both retained. Validate source repositories/inputs with existing testacceptance.Validate at original run start; offline recheck verifies retained Git inventories and trusted-start attachment rather than rerunning a mutable workspace validation or a hook.
2. Strictly decode exact raw report with testacceptance.Decode and require fresh request/assessment pair /1; old /0 assessments remain unsupported for positive /2. Confirm request/product/test/environment/build/executable identities, full request test inventory and complete exact repeat/probe/control run schedule. Require one ordered control per test. Raw report cannot substitute a supplied decision. Final run classifications derive from the prepared #394 freshness.go, not DD8's obsolete boolean join.
3. Every Run.native_receipt decodes from exact base64 bytes, matches receipt_sha256 via testacceptance.Hash, jstestprovider.DecodeFreshness, actual freshRequestBinding and actual row identity. Verify carried duration as well as state/attempt/retry/validity; prepared classifier currently does not compare declared duration, so /2 verifier adds that comparison before projection. Preserve observed schedule/test inventory exactly and check native row/run schedule consistency. Recompute all provider axes with ReceiptTestProjection/FreshnessCurrency; reject stale/unsafe/failed runs even when they share formatting.
4. Every Control uses behaviorfalsify.VerifyReceipt with exact approved plan digest and tool executable hash. Verify complete attempt bijection, raw hook/native bytes and all cross-digests. Reuse prepared sameFreshClosure, responseDefinition, joinedFreshControl logic inside producer package; do not expose or duplicate a weaker adapter join. Reset aggregate classifications to native initial values, rederive carried rows/schedules from actual receipts and initialize the exact producer-owned unknown list; do not feed caller-provided assessment results back into classifyFreshness. Check all each-attempt target assertions, approved response mutation, plan/native/attestation/source/observer/dependency/environment/request/baseline joins. Recompute assessments/aggregate reasons/repeats/order using existing classifyFreshness from this verified evidence. Verify original Body equals existing renderBody of that rederived original report and request byte-for-byte. Keep raw renderer output; no second Markdown implementation.
5. Raw native preimages currently missing from existing report (application attestation stdout, selected logs and host birth mapping) must be retained by new /2 execution collector. Existing schemas are unchanged; their fields link into new attachment graph. Verify actual stdout hash and native decode before semantic link. Verify referenced plan/approval/control artifacts, content inventories and every host/scope/log/process object from complete retained bytes. Hashes without producer-defined preimages fail. Match actual repository Git object content with draft parent/tree/full path/mode/blob inventory. Missing config Use closed schema or unverified alias blocks.
6. Concrete postmergeproof.VerifyProcessV2 independently verifies the closed ProcessPolicyV2/ProcessProofV2 raw preimages and graph binding and derives injective birth→logical mappings; it alone constructs its opaque token. See process-verification.md; no host callback or claimed JSON role map. Verify raw root/parent identities, lifetime, per-run/control generation distinctness, cleanup interval/failures/limits/completeness and independent absence; maintain topology multiplicity. Numeric PID, lstart or claimed role alone is insufficient. Only source-direct process-backed app kind and explicitly qualified OS/profile supported. Require unique known role/parent/run-ordinal mapping; unresolved multiple indistinguishable nodes block. Launcher assigns roles to owned roots, provider observes worker/browser roles from approved launch semantics, process observer verifies command/birth/parent. Arbitrary author-defined roles unsupported. Original second-granularity/sampled process limitations remain exact and qualified only as bounded trusted-local evidence. Cancellation/interruption must retire descendants; preserve unresolved cleanup and refuse positive.
7. Build fresh typed DecisionV2 from verified native types using 31 explicit field transformations plus exact-default inventory. Existing raw structs are not edited/zeroed or reserialized as fake raw evidence. Compare recomputed canonical decision bytes with claimed decision bytes/digest if supplied. Validate full decision schema; all unmapped raw digest dependencies block instead of blanket hash stripping. Self-seal markers only reflect actual preceding native digest verification; complete semantic payload and child links are still present. Native unsupported/authentication/containment limitations remain exact even where bounded trusted-local qualification permits them; unknown required cleanup/proof blocks.
8. Phase1 native decision is independent of connector JSONL. Build/validate candidate immutable Git draft and fixed connector requests through old renderer after decision verified; canonical request digest is SHA256 of exact canonical JSONL bytes (native connector Request format/order preserved). Build attachment afterward, linking decision and request and fresh graph. Attachment cannot enter decision or URL identity, avoiding request↔attachment digest cycle. Offline consumer first regenerates requests from verified decision/draft, then verifies attachment request digest before its BuildEvidenceV2 token may escape or Record may run. No arbitrary connector callbacks or two-phase record gap.
9. Connector verifies canonical URL→exact decision, same product/test/draft identities and actual Git parent/tree/path/mode/blob inventory; invokes native verifier again on this fresh attachment. It uses existing Build/ValidatePlan for fixed rendering/request checks inside new wrappers; it never changes the fixed old Request profile/body. Before both Record calls, reverify joins and current immutable preimage inventory. Append separate immutable qualification record {execution_id, decision_sha256, attachment_sha256, canonical_request_sha256}. That record never enters Writer.Upsert Request bytes. Each fresh run uses new ledger; same run calls Record twice, requiring no duplicate outward event. Frozen connector /0 caller path remains exactly available.
10. Each of two runs independently executes all applicable stages and verifies its own raw graph. Exact canonical decisions, meaningful author/draft content and canonical JSONL must agree. Raw report/Body/artifact/process/telemetry differences are separately enumerated by exact byte hashes and mapping class. Failed native/cleanup result cannot MATCH. Valid unequal decisions or requests MISMATCH, missing unsupported proof BLOCKED/CLI2, invalid evidence rejects positive admission. Qualification is only actual local OR CI tuple from exact admitted campaign. MATCH does not confer authority or close ticket.

## Existing full-runtime boundary

DD8 /1 native.go intentionally stops at unavailable delta. The /2 decision/connector schema and tests alone cannot deliver the historical docs/tests workflow. No proposed schema supplies that stage. The remaining original #395 route must call the actual supported source-neutral delta producer; a caller-created selection object or archived author patch does not qualify it. One Gate A includes locating that supported producer, actual author/check/scope invocation and metrics bindings. If an admitted API is still unavailable, deliver the verified bounded producer/connector slice as partial with the native ticket OPEN and record the specific upstream dependency. Do not install a synthetic stage or claim the full /2 campaign from unit fixtures. Exact stage adapters outside native testacceptance have no float/proof exceptions until their original native verifier and field map are admitted.

## Repair1 M1 — exact verified logical witness ordering

After concrete native verification assigns an injective birth→logical identity, sort only each `sem.procgroup.DescendantObservation.processes` array by `row.pid.node` in ascending ASCII byte order. This applies through Report.Run/Control cleanup descendants and Provider descendantObservation/freshness.serverDescendants, wherever that exact source type occurs. Raw process arrays remain unchanged in fresh attachments. Duplicate logical identity within one array or ambiguous role/birth mapping blocks; do not deduplicate or invent a PID/first-seen tie-breaker. Preserve null/absent/empty array distinctions and every other row field. Top-level verified `processes` registry uses the same unique-node order; derived `birth_distinct_from` relations are unique ASCII-sorted logical IDs. All test/run/worker/start/probe/control/attempt arrays and native registry source order stay exact. Reversed physical PID order with equivalent verified logical topology yields the same decision; changed native schedule or process parent/role/multiplicity does not.

## Repair1 authority and disposition

Original packet manifest61654b4256d33f4ae750c9886a507bd59bbc0a135c72d9972c204fdaf9a4329e and first Gate A report9afd47015efd316cbe24fbd1f4ebd4f8fd7d22ab84c814dd86d44dabd9dff688 remain unchanged. This separate repair1 directory addresses H1/H2/M1 for the same reviewer to recheck changed design only. No new owner acceptance, reviewer, source mutation, native/GEN admission, product check or qualification is implied.
