# Post-merge Runtime V2

Owner: Russell Lewis
Date: 2026-10-03
Intent status: accepted
Delivery status: partial
Intent basis: accepted /2 direction by direct human receipt; the executable elaboration below passed independent Gate A.
Only the PMR-V2-006 process verifier slice is implemented; no /2 workflow runtime, host qualification
or qualified positive replay is claimed.

## Agent digest
- Claim: Accepted /2 contract compares stable producer decisions and canonical requests across fresh verified runs; only its process verifier slice exists.
- Status: accepted; partial; PMR-V2-006 process verifier slice only, qualification NOT_OBSERVED.
- Exists: This spec, `conformance/postmerge-runtime-v2/` and the `internal/postmergeproof` verifier; no /2 workflow. /0 is unchanged; inherited /1 now observes provider-free native delta, then blocks at follow-up.
- Blocked on: #394 producer decision (needs a qualified PMR-V2-006 process proof), provider-backed delta, Linux procfs tuple qualification and real follow-up/author/scope/validation stages.
- Read next: Requirements; Closed executable profiles; Remaining work and promotion boundary.

## Authority

The owner approved “Approve the proposed /2 contract” in question `call_1iMk9n0ElLmb6WSgCliyzQn7`.
The receipt SHA-256 is `3cfe04dbfad15a37fc564c28ef18616028baa44a7befadd26353fd025cd1b3cb`;
accepted direction SHA-256 is `ea5dac157e5e7ae6f2ad865a0ed639fa90bb6d5697d192b58a8f1ec023e48bfb`.
These receipts accept the separate contract, never unperformed execution or qualification.
The frozen executable packet SHA-256 is `00f4536dedff1331f667eb62738af600ff61d74778caa0dabbf42b5f314dd637`.
Independent repair-1 Gate A PASS SHA-256 is `88f5e73beec92f170c13b3bc3508ceed9f899a6a43e2b5bc5695492813b734da`:
H1, H2 and M1 closed; no remaining actionable HIGH/MED/LOW in that bounded design review.
Project authority, the existing /1 runtime and native producer contracts govern all inherited obligations.
Owner decision 2026-10-04, given through the orchestrator session's question to the owner: the
process verifier's host tuple identity, SHA-256 over the domain `postmerge-host-tuple/2`, NUL and the
wire-canonical `HostTupleV2` JSON, is ratified as chosen. The frozen conformance data is unchanged.

This one owning spec registers the producer decision, process proof and connector admission helpers
because they share the deterministic historical replay outcome and its promotion gate. The provisional
separate helper spec names in private planning are not new authority or required files.
The byte-identical conformance artifacts below freeze the reviewed helper profiles together.

## User job and verified current state

An adopter replays a historical merge with zero credentials, sees basis-labelled expectation mismatches,
and qualifies deterministic canonical recording before enabling outward writes. The complete workflow
must run twice with independently fresh verified physical proof. Stable decisions and requests must match;
raw report/Body/process/telemetry differences remain explicit.

At immutable base `dd8cc0ca918a80faaa041c98a783de11550b2bcc`,
`internal/postmergeworkflow/native.go` explicitly refuses unavailable actual delta with BLOCKED/CLI2.
Actual author/scope/check/metrics stages and positive full-workflow qualification remain NOT_PRODUCED.
The #394 complete prepared integration patch SHA-256
`da35448e8644051b63a562f05b80b8a083fc73df924d28785449b67ce983f6e9`
is a dependency proposal, not an applied or qualified producer. Its immutable source is
`e5d777cbd1202dbffb88ce114956b304035bd5c5`. Revalidate the source map at the actual admitted base.
Synthetic examples demonstrate schema/link topology only; they cannot mint verified process tokens,
qualify a host, satisfy historical replay, or close native ticket V1-0542.

Current state 2026-10-04 (delta slice; not acceptance evidence for any PMR-V2 requirement):
#389 is closed and `corvint delta` (`internal/delta`) is on main. The inherited /1 delta stage now
runs actual `delta.Compile` without providers, documentation baseline or work keys, retains its exact
canonical record, then blocks at follow-up (PMR-V1-002). Every /1 report is still BLOCKED/CLI2.
Every added line of patch `da35448e` is already present on main, and it contains no /2 decision
emitter. `internal/testacceptance` has no `ExportDecisionV2`/`VerifyDecisionV2`. The PMR-V2-006
verifier described next mints no token without an admitted qualification, so the required decision
`processes` member still has no verified source and the PMR-V2-002 emitter stays NOT_PRODUCED until a
qualified process proof exists.

Since 2026-10-04 the PMR-V2-006 verifier slice (#395) exists in `internal/postmergeproof`.
`VerifyProcessV2` checks the parent admission, an admitted qualification report, the closed policy and
the raw proof against retained preimages and an independently constructed graph, and only then mints
the opaque `VerifiedProcessesV2` token, which no other package can construct. `ValidFor` refuses a
zero or differently bound token. The `procfs` subpackage reads bracketed Linux birth captures and
final `/proc` sweeps on amd64 and arm64; other hosts report NOT_OBSERVED with
`process-observation-unsupported`. No real qualification campaign has been admitted, so every
production call stays BLOCKED with `process-qualification-unavailable`. No collector, producer,
workflow or connector consumes the token yet; PMR-V2-009 and PMR-V2-010 are not wired. Retained
limitations: a double-forked descendant reparented to PID 1 outside the captured tree escapes the
final sweep, and no token field carries that gap; each proof must use one native start tool, but
neither policy nor qualification pins it; the host supervisor's state is
`outside-workload/exit:unknown` because nothing observes its liveness.

Verifier refusals keep their class and never collapse zero, absent and unknown. BLOCKED:
`process-qualification-unavailable`, `process-host-unsupported`, `process-token-invalid`,
`process-artifact-unavailable`, `process-bound-exceeded`, `process-graph-invalid`,
`process-native-source-unsupported`, `process-legacy-sample-unsupported`, `process-role-ambiguous`,
`process-role-unsupported`, `process-parent-unverified`, `process-native-reference-missing`,
`process-native-start-unavailable`, `process-cleanup-unknown`, `process-sweep-incomplete`,
`process-sweep-scope-ambiguous` and `process-verification-cancelled`. REJECTED:
`process-admission-mismatch`, `process-policy-digest-mismatch`, `process-qualification-invalid`,
`process-wire-invalid`, `process-graph-substituted`, `process-artifact-digest-mismatch`,
`process-capture-malformed`, `process-stat-malformed`, `process-birth-changed`,
`process-bracket-changed`, `process-namespace-mismatch`, `process-executable-mismatch`,
`process-invocation-mismatch`, `process-retirement-invalid`, `process-trusted-start-mismatch`,
`process-native-reference-invalid`, `process-cleanup-join-invalid`, `process-cleanup-survivors`,
`process-sweep-invalid` and `process-sweep-survivor`.

## Requirements

- `PMR-V2-001`: Closed dispatch and authority. Explicit `/2` dispatch admits only independently pinned closed manifest/policy/schema versions. Pin product/base/merge, author/test Git commits and trees, complete content inventory, executable hashes/builds/runtime/config/host policy, selection and approved control definitions. Expectations remain parent-only; they cannot choose a command, output, verdict, applicability or evidence source. `/0` and `/1` continue under their exact existing decoders and comparisons.
- `PMR-V2-002`: Producer decision, not adapter reconstruction. The #394 native producer emits a proposed closed decision alongside its unchanged raw report. Decision fields are defined in the inventory below. A consumer derives no acceptance from caller-written decision JSON: a trusted native verifier recomputes it from that run's actual verified report, requests and complete referenced preimages. No second Markdown renderer, regex stripping or replacement report is introduced. Unknown fields, schema drift, unsupported mappings and absent proof refuse positive admission.
- `PMR-V2-003`: Stable canonical identity. Canonical UTF-8 JSON uses an agreed bounded schema, lexically ordered object keys, preserved array order, exact scalar types and null/presence distinctions. Decision digest is SHA-256 over a fixed domain tag plus the exact canonical bytes. Do not sort observed schedules to manufacture equality. The domain and every digest preimage are producer-owned and versioned; a profile/schema change changes the identity. No floating elapsed value participates unless it affects an acceptance rule; measurement presence and all derived acceptance outcomes do participate.
- `PMR-V2-004`: Verified fresh proof attachments. Each execution retains a separate immutable attachment manifest addressing its exact raw report/Body, request, native receipts, provider/control artifacts, scope/host/process observations and retained logs. Its own SHA-256 binds all byte lengths/digests and dependency edges. Bind execution identity, decision digest, product/test/draft identities and canonical request digest. Verify every reference from actual retained bytes with the producer's supported verifier before recording; a digest alone is not proof. Each execution needs its own complete graph. No first-run proof reuse, missing preimage invention or successful comparison from two unverified attachments.
- `PMR-V2-005`: Immutable Evidence URL and connector join. The writer-visible admitted-origin HTTPS Evidence URL addresses the canonical decision digest, not the varying full attachment. The #392 consumer verifies that URL→exact decision preimage, actual draft parent/tree/path/mode/blob identity in its verification repository, and the fresh attachment→same decision/request/product/test/draft joins before Build/ValidatePlan/Record. A manifest-pinned local content resolver can verify these immutable URL identities during credential-free recording without publishing remotely. Operational hosted resolution needs its own actual contract/qualification. Neither shape/origin validation alone nor a mutable alias suffices. The fixed generated draft body remains fixed, with the new URL meaning declared by a separate connector profile. Existing connector behavior is unchanged.
- `PMR-V2-006`: Process and time semantics. Fresh numeric PID/parent PID/start-clock values and physical mount/inode identifiers stay exact in raw attachments. A qualified host verifier binds actual birth identity, ownership, parent edges, role/run ordinal and observed lifetime to logical process nodes. Stable decision references preserve verified role/topology, exit, cancellation, timeout, survivor/absence, observer interval policy and limitations. Unverified aliases or topology differences cannot be collapsed. Raw elapsed values may differ only when no accepted rule consumes them; measurement absence, timeout thresholds/violations, historical metrics durations and selection-affecting timestamps remain exact decision facts. Zero, absent and unknown never become equivalent.
- `PMR-V2-007`: Compare complete fresh executions. Run the complete applicable workflow twice from independently fresh admitted environments with the same pinned logical inputs. Verify each attachment independently. Compare canonical decisions and canonical writer request JSONL exactly; compare unchanged exact stage output classes as required by the inherited contract. Report raw report/Body/proof/telemetry differences explicitly beside the equality verdict. An attachment difference alone is permitted only when all mapped acceptance facts are equal and independently verified. It is never a mismatch waiver for changed author content, source IDs, outcomes, reasons, controls, order, cleanup or unknowns.
- `PMR-V2-008`: Recording and idempotence. Each execution uses its own ledger and calls Record twice with its same verified plan/attachment join; no duplicate outward-request events may appear. The two executions' canonical outward-request JSONL must match byte-for-byte. Fresh attachment manifests are a distinct immutable qualification artifact stream; they cannot be injected into canonical outward JSONL or its fixed body to reintroduce changing per-run bytes. The complete delivery packet includes both streams and proves their request-digest links. Any writer that cannot support this separation blocks `/2` positive drafts.
- `PMR-V2-009`: Failure, rejection and comparison. Missing/unsupported/unverifiable evidence, unknown cleanup or required unavailable stages yield BLOCKED and CLI2. Known unsafe scope, surviving descendants, invalid proof, unauthorized identity/content change or failed native acceptance rejects positive draft admission. Two valid but unequal decisions/requests yield MISMATCH and preserve both exact graphs; differing generated/human-verified expectations remain separately labelled. Only complete applicable stage execution plus verified equal decisions/requests and required unchanged exact outputs can qualify positive deterministic replay. MATCH grants no truth authority and generated labels remain generated. Failed execution cannot be laundered into a decision-equivalent successful run.
- `PMR-V2-010`: Original outcome and bounded qualification. Acceptance evidence includes a historical fixture whose independently pinned inputs trigger actual docs and tests work, complete fresh validation/verified draft requests and triageable findings, comparison against frozen basis-labelled expectations, and idempotent equal canonical recordings under recording mode with no outward writes or credentials. Establish actual local **or** CI execution for the exact admitted tuple. Requiring both, operational reuse, remote hosting or broader portability needs separately governing accepted scope. Retain all bounds/cancellation/error controls and failed attempts; complete native integration/closeout separately. Rollback disables `/2` dispatch and leaves `/0`/`/1` and their archives intact.

## Closed executable profiles

The reviewed conformance data, not arbitrary caller JSON or a new adapter renderer, defines the
closed source inventory, exact field transformations, hash preimages and proof boundary.
All 75 reachable native source types and 31 explicit transforms remain covered. A new native field
is exact by default; unmapped fields or unsupported branches block admission rather than disappear.

- [Producer verifier contract](../../conformance/postmerge-runtime-v2/verifier-contract.md): concrete native verification sequence, export and connector joins.
- [Semantic links](../../conformance/postmerge-runtime-v2/semantic-links.md): twelve exhaustive native link branches and four finite typed target kinds.
- [Process verification](../../conformance/postmerge-runtime-v2/process-verification.md): concrete postmergeproof verifier ownership, role derivation, birth/topology and cleanup checks.
- The matching JSON schemas, inventories, tagged Go contract text and synthetic examples in `conformance/postmerge-runtime-v2/` are frozen byte-identical data. The `.go.txt` signatures are not compiled source.

Decision construction precedes canonical connector requests; attachment sealing follows both. Native
receipt preimages are verified before typed semantic substitution. No cycle, generic JSON stripping,
alternate locator alias, first-run artifact reuse, caller PASS bit or injectable verifier bypass is admitted.
Only the concrete `internal/postmergeproof` verifier can mint a nonzero opaque process token.
Host/procgroup collect bytes and observations. Core must not import `internal/tasks` outside its wire package.
Only unique qualified logical process witness lists may be sorted by node ASCII bytes; native schedules
and raw physical witness arrays retain their exact observed order.

## Non-goals and simpler baseline

The existing /0 and /1 paths remain the compatibility baseline. There is no silent wire redefinition,
network resolver, credentialed or remote writer, daemon, hosted service, permanent database, default
Core expansion, broad OS portability or schema-only claim of positive replay. This contract does not
authorize merging, releases, live browser campaigns or paid service changes.

## Trust, limits and failures

The initial process-backed profile is a trusted-local Linux procfs tuple on amd64 or arm64 only after
actual exact-tuple qualification. Legacy PID samples, Darwin/Windows, unqualified roles and incomplete
raw native observations refuse positive admission. The observation target interval is 20ms; process
and capture bounds are 4096 and 16384. Paired stat observations and an independent final bounded
absence sweep are mandatory. Transient processes between samples and continuous lifetime remain
explicit limitations; observed owned-tree evidence never becomes a full-host or hostile-host claim.
Policy implementation/executable/source identities, all proof bytes and qualification cases are pinned.

Missing/unsupported evidence or a required unavailable stage is BLOCKED/CLI2. Known invalid or unsafe
proof rejects positive admission; valid unequal decisions/requests are MISMATCH. Errors, uncertainty,
cancellation, timeouts, surviving descendants and unknown cleanup remain acceptance-sensitive facts.
The fixed canonical outward JSONL excludes fresh attachment records; a separate immutable qualification
stream binds each run to the decision and its exact request digest. The local resolver verifies pinned
immutable decision bytes without contacting the HTTPS origin.

## Acceptance and test matrix

Source checks must cover strict missing/null/duplicate/alias/enum/bounds handling; native preimage and
Body recomputation; all twelve link branches; semantic kind/locator/context/ordinal association;
zero/wrong-bound process tokens; paired procfs parsing, ambiguous roles, parent/birth changes, final
absence and interruption of grandchildren; actual Git draft blob/mode/parent/tree tampering; immutable
URL preimages; per-call connector revalidation, Record twice and two independent ledgers. Existing
/0 and /1 tests remain passing. Fixtures assert independent recomputation and tamper refusal rather
than only matching constructed implementation output.

Complete promotion additionally needs a real historical fixture that triggers actual docs and tests,
real intake/delta/author/scope/check/metrics/applicable-corpus stages, triageable findings, frozen
generated/human-verified expectations, two fresh complete attachments and equal decisions/requests.
Qualify actual local OR CI execution for the exact tuple; neither both nor remote hosting is required.
Private schema/hash readback and Gate A PASS are design evidence only. The process verifier source
checks named in Traceability now exist; every other product test, independent source review, process
qualification, full historical workflow and native completion remain NOT_RUN/NOT_PRODUCED.

## Rollout, rollback and drift

Register this seed in a separate repository with its own Git objects. It is an ordinary source change:
retain actual pre-change context, existing authority at the immutable seed base, actual metadata/docs
checks, source CEM and inspected OCM reports, clean postcommit dogfood-change/dogfood-check and a pure
rename-only dogfood seal against that SAME base. Older governing evidence may justify inclusion;
new acceptance/schema evidence cannot become invented historical CEM authority. Unknown outside an
actually OCM-validated base-absent intent hunk blocks closure; newly added schema/data files get no
blanket bootstrap exception. Retain raw unknowns and every NOT_PRODUCED reason.
Only that verified seed seal commit/tree may become the second own-object implementation repository's
immutable base. A plain content commit or unbound base shift is insufficient. Preserve seed enrollment
state and create the implementation's own PMR-V2 enrollment; do not reset/rebegin/cancel or reuse the
sealed /1 enrollment. CODE source editing follows an actual baseline in INIT; source checks, metadata,
Git mutations and final CEM/check/seal run only in their separately admitted serialized phases.
The source-direct producer dependency and actual base must be rehashed before construction and gates.
Rollback disables explicit /2 dispatch and preserves /0, /1 and their retained evidence. Changed source,
schema, verifier, observer, policy or host tuple invalidates affected proof/qualification. Update this
spec and exact conformance data with any changed behavior or wire contract; new authority still needs
owner acceptance. Checks retain their actual head/content binding, including clean postcommit CEM
binding/check/seal. No passed design check is a source-execution authorization.

## Traceability

| Requirements | Planned implementation | Required evidence / current status |
|---|---|---|
| PMR-V2-001, 007, 009, 010 | `internal/postmergeworkflow/v2*.go`, workflow CLI | strict routing, refusal and complete historical replay; NOT_RUN |
| PMR-V2-002, 003, 004 | `internal/testacceptance/*v2*.go`, provider collector | native rederivation, all source/link tamper cases, fresh graph coverage; NOT_RUN |
| PMR-V2-005, 008 | `internal/postmergeconnector/*v2*.go` | immutable resolver and actual Git joins, both-call revalidation, idempotent JSONL; NOT_RUN |
| PMR-V2-006 | `internal/postmergeproof` (verifier slice delivered), `internal/postmergehost`, `internal/procgroup` | token boundary: TestVerifyProcessMintsBoundToken, TestZeroProcessTokenIsInvalid, TestVerifyProcessAdmissionRefusals, TestQualificationRefusals; raw birth/role/cleanup controls: TestRawProcessProofDerivesLogicalGraph, TestRawProcessRefusals, TestDistinctProcessStates, TestRoleWitnessOrderInvariant, TestProcStatParser, TestPolicyRefusals, TestWireTableMatchesFrozenSchemas; procfs: TestUnsupportedHostIsNotObserved, TestCaptureOwnBirth, TestSweepListsOwnProcess, TestLinuxProcfsOwnedExecution, TestLinuxProcfsSweepFindsSurvivor (Linux arm64 container only; amd64 NOT_RUN); collector integration and actual tuple qualification NOT_PRODUCED |
| PMR-V2-007, 010 (prerequisite only) | inherited /1 `internal/postmergeworkflow/native.go` delta stage | provider-free actual delta observed and repeated byte-identically (`TestNativeDeltaRepeatsExactly`); provider-backed delta and later stages NOT_PRODUCED |

## Remaining work and promotion boundary

Root must admit the seed and implementation resource scopes, actual base, own-object repositories,
closed environment/cache/process effects and shared GEN lane before execution. The first producer/
connector/refusal slice may remain PARTIAL. Provider-backed native delta and all later full-workflow stages remain
required upstream work; frozen patches cannot stand in for author execution. A native ticket stays
OPEN until full acceptance, integration and successful native completion. Kill promotion if complete
native proof, unique process mapping or deterministic canonical recording cannot be established within
the admitted profile; retain bounded refusal and original failures instead of weakening acceptance.
