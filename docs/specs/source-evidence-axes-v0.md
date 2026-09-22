# Source Evidence Axes V0

Owner: Russell Lewis
Date: 2026-09-08
Requirement prefix: `SEA-V0`
Intent status: proposed
Delivery status: not-started
Authoritative inputs: `../../AGENTS.md` invariants 1–4 and 8,
`../SPEC-DRIVEN-DEVELOPMENT.md`, `local-admin-console-v0.md` (`LAC-V0-006..011`),
`local-observability-dashboard-v0.md` (Independent truth axes), and
`go-production-kernel-migration-v0.md` (`GPK-V0-027..030`, `GPK-V0-039..040`).

## Agent digest
- Claim: Corvint could emit source-owned, independently scoped evidence axes beside query and impact values without translating legacy authority labels in the console.
- Status: proposed/not-started
- Exists: the console admits six closed vocabularies; query and impact retain their existing authority/confidence vocabulary and do not emit this profile.
- Blocked on: owner decisions D1–D4 below, independent contract review, implementation, and all executable acceptance evidence.
- Read next: Verified current state; Proposed wire and value scope; Owner decisions and promotion.

## Human intent and measurable job

The accepted console intent is one inspector for user-facing Corvint and task-management verbs.
`LAC-V0-007` requires source-owned axes for each rendered value. The console's S3 account explicitly
defers query and impact until Corvint states those axes. The current task authorizes preparing this
proposal; it does not accept its proposed transport, classifications, or support expansion.

Proposed job: from one query or impact response, inspect a result's exact citation, its relevance
claim, and the packet's omissions while distinguishing each claim's support. A source read, a
ranking score, and a proposed verification command must never appear to share one verdict.
Qualification measures correct attribution and preserved uncertainty; operator-time savings and
console U4 remain separate work.

## Verified current state

Sources were inspected at commit `2f843df329afb52ebef9516721ead77bec2cf491`, whose Git tree is
`51a686ea0d3546555bd93fe86df14f036b5b302e`:

| Source | What it establishes |
|---|---|
| `local-admin-console-v0.md`, Requirements and Delivered S3 behaviour | Consumer admission only; foreign labels are not translated; query/impact are intentionally absent from the surface. |
| `../../internal/console/axes.go`, `Axes.Set`, `UnstatedAxes` | Closed-set admission; an absent or unrecognized axis stays `NOT_STATED`. This is consumer behavior, not a source value. |
| `../../internal/contextindex/impact.go`, `evidence` | Existing evidence contains `path`, `line`, `blob_hash`, `reason`, `confidence`, and `authority`. |
| `../../internal/contextindex/receipt.go`, `receipt` and `compileReceipt` | Packet revision, freshness, bounded ranking coverage, exclusions, and uncertainty have existing meanings and budget behavior. |
| `../../cmd/corvint/main.go`, `emitContextPayload` | Query/path-impact success emits the canonical `{ok,mutates,tool,context}` envelope. |
| `go-production-kernel-migration-v0.md`, User and measurable job | Migration permission does not authorize new wire semantics; existing parity and resource gates remain binding. |

The pre-change Corvint query returned one dashboard-spec lead with four ranked results omitted.
That discovery packet is not evidence that this proposal is complete or accepted. Stored query
stdout remains unsupported as a dashboard usage source under the dashboard's Verified current
state and gap; adding these axes would not create a storage verifier or usage denominator.

## Source ownership

| Responsibility | Owning source / proposed implementation boundary | Consumer boundary |
|---|---|---|
| Contract acceptance and promotion | Repository owner; this proposal and the accepted GPK profile must record the decision. | Neither code, tests, generated docs, nor the console can accept it. |
| Capture, invocation identity, option admission, final envelope | Corvint `cmd/corvint`, using its existing captured repository/index observation. | Console names the invocation and reads the returned provenance. |
| Query/path-impact value, claim scope, denominator, witness and six axes | Corvint `internal/contextindex`, at the originating calculation or declaration admission. | No mapping from `authority`, `confidence`, `state`, or `freshness.state`. |
| Range/untracked-impact extension | A later accepted source profile; current owners are `internal/contextindex/range_impact.go` and `internal/worktreeimpact`. | Unsupported by this proposed first slice. |
| Axis vocabulary | The six sets below, reproduced from the LOD axis contract referenced by accepted `LAC-V0-007`. | `internal/console/axes.go` admits exact values and displays missing axes as `NOT_STATED`. |

## Proposed wire and value scope

The following is one concrete recommendation for D1–D4, not callable syntax or an accepted schema.
Opt in with `--evidence-axes=source-evidence-axes/0` on `query` or tracked-path `impact`. The first
slice admits only the existing Darwin/Linux profiles over a clean, stable committed observation.
Range, untracked-path, mixed-worktree, stale-index, batch, harness, context and feature profiles are
out of scope. The source must identify the actual selected existing query/impact profile; this
option cannot expand that profile's language, ranking, trace, platform, or path support.

Without the option, stdout, stderr, exit status, budgets and execution path are unchanged. With it,
a successful existing command is wrapped in an object containing exactly:

```text
schema = "source-evidence-axes/0"
payload = the unchanged existing success-envelope JSON value
payloadSha256 = SHA-256 of the original canonical payload bytes including trailing LF
producer = {toolVersion, executableSha256, profile}
observation = {objectFormat, headCommit, treeRevision, indexRevision, statusSha256}
values = [value annotation, ...]
```

Digests use `sha256:` followed by 64 lowercase hex digits. Git identities remain Git object IDs,
with 40 or 64 lowercase hex digits according to `objectFormat`; tree and commit identities are
separate. `profile` identifies the selected existing source profile and its owning requirement,
for example `GPK-V0-028`. All provenance is captured during the same invocation; no second query
supplies missing identity. The observation is a Git/index validation boundary, not a timestamp,
execution attestation, or perpetual freshness promise.

The wrapper uses the existing `contextindex.CanonicalJSON` encoding and one trailing LF. It does
not adopt the dashboard's distinct no-JSON-number dialect. Each annotation contains exactly:

```text
pointer       = canonical JSON Pointer into payload; one existing non-null scalar
claim         = CITATION_IDENTITY | DECLARATION | ADVISORY_RELATION | PACKET_BOOKKEEPING
scope         = {subjectPointers, denominatorPointers, exclusionPointers}
witnesses     = [{kind, pointers}, ...]
axes          = {validity, epistemicClass, authorityClass, completeness, currency, deliveryStage}
reasons       = {validity, epistemicClass, authorityClass, completeness, currency, deliveryStage}
```

Pointers start at the `payload` value, not at the wrapper, use `~0`/`~1` escapes, and use canonical
decimal array indices without leading zeroes. `values` sorts by pointer bytes with no duplicate
pointer. Each scope pointer array and witness pointer array sorts by bytes without duplicates.
Witness rows sort by `(kind, pointers)` without duplicates. Scope and witness pointers may target
scalar or structured payload values. Every pointer must resolve; aliases, recursive inheritance,
wildcards, duplicate JSON object keys, unknown fields, and unknown enum values are rejected.

`witnesses.kind` is `GIT_SELECTOR`, `SOURCE_DECLARATION`, or `SOURCE_CALCULATION`. These identify
the source procedure supporting the annotation; they are not independent execution receipts.
`GIT_SELECTOR` names the exact path/blob/line/revision payload fields whose object identity and
selector Corvint checked. `SOURCE_DECLARATION` names the field being copied and its attribution.
`SOURCE_CALCULATION` names the existing count, cohort or result inputs used by the computation.
An unexposed input is not invented as a pointer: the annotation is omitted until an accepted
profile can expose the required witness. Each of the six reason strings is nonempty, at most
1,024 UTF-8 bytes, and explains only its own axis using this claim and its referenced evidence.

Axes apply only to the exact scalar named by `pointer` under `claim` and `scope`. A result, packet,
or citation ancestor cannot donate axes to descendants. A citation identity proves where bytes
were read; it does not prove the assertion within those bytes or the relevance of a result.
Transport/provenance metadata identifies the envelope and is not a second semantic assertion
about payload content. Every payload value presented with six real axes must have its own admitted
annotation. Null, absent, structured, unsupported, or unannotated payload values retain their
original diagnostic and `NOT_STATED` presentation; the consumer cannot manufacture an annotation.

### Vocabulary and initial assignment rules

| Axis | Exact closed values |
|---|---|
| `validity` | `VALID`, `INVALID`, `NOT_PRESENT`, `INACCESSIBLE`, `UNSUPPORTED`, `DISABLED`, `EXPIRED` |
| `epistemicClass` | `OBSERVED`, `DECLARED`, `ADVISORY`, `NOT_OBSERVED` |
| `authorityClass` | `REPOSITORY_ACCEPTED`, `OWNING_VERIFIER`, `PROVIDER_QUALIFIED`, `ADAPTER_QUALIFIED`, `CALLER_REPORTED`, `ADVISORY`, `NONE` |
| `completeness` | `COMPLETE`, `PARTIAL`, `UNKNOWN` |
| `currency` | `VALIDATED_AT`, `HISTORICAL`, `STALE`, `MIXED`, `UNKNOWN` |
| `deliveryStage` | `ACCEPTED`, `VALIDATED`, `IMPLEMENTED`, `EXPERIMENTAL`, `NOT_STARTED`, `FAILED`, `UNSUPPORTED` |

These sets are independent. Only authority has the specified strongest-to-weakest ordering above;
this proposal introduces no total order or generic minimum operation for the other axes.

The proposed initial source assignment is deliberately narrower than the vocabulary:

| Claim / example scalar | Source admission needed | Proposed axis assignment |
|---|---|---|
| `CITATION_IDENTITY`: an evidence `path`, `blob_hash`, or `line` | Resolve path to the stated regular blob in the captured tree and validate the claimed selector in that blob. Merely copying index fields is insufficient. | `VALID`, `OBSERVED`, `OWNING_VERIFIER`, `COMPLETE` for that single identity check, `VALIDATED_AT`, `EXPERIMENTAL`. |
| `DECLARATION`: legacy `authority` or `confidence` text | Identify and reproduce that source field exactly; claim is only that Corvint declared the label. | `VALID`, `DECLARED`, `ADVISORY`, `UNKNOWN`, `VALIDATED_AT`, `EXPERIMENTAL`. |
| `ADVISORY_RELATION`: result `score`, match `reason`, or suggested `verification` command | Retain the originating calculation/declaration and its exact result scope; no test execution is inferred. | `VALID`, `ADVISORY`, `ADVISORY`, `UNKNOWN`, `VALIDATED_AT`, `EXPERIMENTAL`. |
| `PACKET_BOOKKEEPING`: a coverage count | Verify the existing admitted-results denominator and every narrowing stage under `GPK-V0-040`; scope names the relevant coverage and exclusion fields. | `VALID`, `OBSERVED`, `OWNING_VERIFIER`, completeness as below, `VALIDATED_AT`, `EXPERIMENTAL`. |

For packet bookkeeping, `COMPLETE` is allowed only over the exact source-defined admitted-results
cohort when nothing from that cohort was omitted or suppressed and the calculation has no unresolved
inputs. Known omissions or suppression give `PARTIAL`; unclosed input accounting gives `UNKNOWN`.
This does not claim complete repository search, full impact closure, or complete test selection.
An actually computed zero may remain in the unchanged payload while its annotated completeness is
`PARTIAL` or `UNKNOWN`; consumers must not present it as a complete measured zero.

`VALID` checks the named claim and wire witness, never whole-program correctness. `VALIDATED_AT`
means checked at the recorded immutable observation only; a later page is historical under
`LAC-V0-011` without the console rewriting source axes. The proposed first profile's fixed
`EXPERIMENTAL` is its own declared delivery stage, not the cited spec's intent status, the CLI's
release status, or a promotion result. D4 must approve this explicit scope before implementation.

No initial rule emits `REPOSITORY_ACCEPTED`, `PROVIDER_QUALIFIED`, or `ADAPTER_QUALIFIED`. An
`accepted-contract` or `project-instructions` string alone cannot grant any of them. A future
accepted-source rule needs its own exact acceptance witness and owner-approved semantics.

## Requirements

- `SEA-V0-001`: The source MUST assign all six axes with a claim, exact scalar scope, witnesses,
  and separate reasons; the console MUST only admit source values under `LAC-V0-007`.
- `SEA-V0-002`: The source MUST preserve the six independent vocabularies and initial assignment
  rules above; legacy labels and success states MUST NOT confer a stronger claim or authority.
- `SEA-V0-003`: The opted-in wrapper MUST bind the unchanged payload and captured producer/Git/index
  identities; all pointers MUST resolve within that payload without inheritance or another read.
- `SEA-V0-004`: Completeness MUST use the named source denominator and retain omissions,
  suppression, exclusions, uncertainty, abstention and unparsed/extraction gaps; no axis closes
  repository-wide impact or establishes a test pass.
- `SEA-V0-005`: Source refusal, unsupported profiles, missing witnesses, malformed annotations,
  and unknown versions MUST follow the degradation rules below without default axes or measured zero.
- `SEA-V0-006`: The default command path MUST preserve its canonical wire, parity oracle,
  existing support, resource budgets and read-only/local-only behavior; this profile MUST be opt-in.
- `SEA-V0-007`: Qualification MUST freeze deterministic source/consumer fixtures, legacy byte
  comparisons, bounds and cost evidence before any delivery or support claim changes.
- `SEA-V0-008`: Rollout and rollback MUST retain existing receipts and failed evidence; publication,
  console enablement, new profile support and stronger classification require their owning approval.

## Trust boundary, limits and degradation

The producer owns claims; a checksum detects byte mismatch but is not trust or execution attestation.
Repository text and task input remain untrusted. This profile adds no network, durable store,
background process, source discovery, index writer or trace mutation. Existing self-observation
exceptions remain exactly as defined by product invariant 4; the wrapper adds no new event.

Proposed fixed added limits are 4,096 annotations, 4,096 UTF-8 bytes per pointer, 32 pointers per
scope array or witness row, eight witnesses per annotation, and 2,000,000 bytes for the complete
canonical wrapper including LF. Existing command/input/packet limits apply first. The original
`--budget-bytes` continues to describe the embedded context packet; the explicit wrapper limit
includes payload and all metadata. These are candidate caps requiring D1 and the measured gate.
No truncation of annotations, hidden omission, or silent increase of a requested packet budget is
permitted. An exceeded wrapper cap refuses the opt-in profile before publishing stdout.

| Condition | Proposed behavior |
|---|---|
| Existing command refuses or fails | Preserve its exact stdout/stderr/exit result, including codes and warning text; no wrapper or axes. Existing validation precedence wins. |
| Existing success uses an out-of-scope profile | Exit 2, no stdout; ordinary typed CLI error with code `unsupported-source-evidence-axes-profile`. Do not retry another source profile. |
| Necessary witness absent for one scalar | Omit that annotation, preserve the unchanged value and its source diagnostics; display `NOT_STATED`. No inferred `VALID`, `NONE`, or `NOT_OBSERVED`. |
| New producer identity, pointer, schema, digest or bound check fails | Exit 2, no stdout; typed code `unsupported-source-evidence-axes-identity`, `unsupported-source-evidence-axes-shape`, or `unsupported-source-evidence-axes-limit`, naming the failed check. |
| Consumer receives missing/unknown schema or malformed annotation set | Admit no axes from that set; show the unsupported/invalid profile and preserve access to source diagnostics. No legacy-label fallback. |
| Consumer sees later repository state | Keep source axis bytes and observation visible; mark the page historical. A refresh invokes the source again and replaces the observation as one unit. |

## Deterministic acceptance and traceability

All rows below are proposed future checks, **NOT_RUN**. No test name here asserts an existing test,
an implementation, a passed gate, or owner acceptance.

| Requirements | Future evidence / falsifier | Proposed implementation owner |
|---|---|---|
| SEA-V0-001..002 | Exact six-axis output for each claim; mutate each axis and legacy label independently. A citation to a proposed spec must not gain acceptance authority; an unrun verification proposal must remain advisory. | `internal/contextindex`; consumer admission in `internal/console` |
| SEA-V0-003 | Frozen Git fixture and executable identity produce identical bytes on repeated runs. Change payload, selector, tree, digest, or pointer; require rejection. No commit/tree substitution. | `internal/contextindex`, `cmd/corvint` |
| SEA-V0-004 | Omission, limit/budget narrowing, suppression, exclusion, unparsed input, empty/out-of-scope result and missing denominator vectors preserve payload uncertainty and forbid false completeness. | `internal/contextindex` |
| SEA-V0-005 | Every unsupported profile and original failure keeps its declared result; unknown version, foreign vocabulary, absent witness, duplicate key/pointer, invalid escape and dangling pointer never gain axes. | `cmd/corvint`, `internal/console` |
| SEA-V0-006 | Option-absent bytes/status match frozen legacy query and path-impact corpus; original parity oracle stays immutable. Filesystem before/after comparison shows no new writes. | `cmd/corvint`; existing conformance owners |
| SEA-V0-006..007 | At-cap and cap-plus-one vectors; paused/failed output path publishes no partial success before validation. Paired latency/allocation measurements preserve the existing profile's gate and default cost. | `cmd/corvint`; existing benchmark owners |
| SEA-V0-007..008 | Independent source/consumer contract review, downstream admission fixture, rollback to legacy response leaves axes unstated, and final clean-target dogfood evidence. | Source and console owners; integration coordinator |

The existing `TestConsoleAxisPassthrough` and `TestConsoleEvidencePane` locate admission witnesses;
they have not been run for this proposal and cannot qualify the new source profile. Documentation
checks validate document shape only. The implementation plan must name exact new fixtures and
required existing gates before build, coordinate canonical gates, and retain every failure and
`NOT_RUN`/`NOT_PRODUCED` result.

## Non-goals and simpler baseline

No console mapping patch, ranking/confidence change, taskman axes, query journal, usage statistics,
generic authority registry, source acceptance parser, dashboard storage adapter, test execution,
OCM repair, Portal instrumentation, or U4 measurement is included. No promotion of the LOD proposal
as a whole follows from using the axes already referenced by accepted LAC intent.

The baseline remains the existing CLI receipt, whose source labels and uncertainty are inspectable,
and the console's current dashboard-snapshot evidence pane. Keep that baseline if the wrapper cannot
make value-level support clearer within the existing performance gate.

## Rollout, rollback and compatibility

First obtain owner decisions and independent contract review; update the affected owning specs in
the same accepted contract change. Only then plan and implement the opt-in producer profile, qualify
its source and consumer fixtures, and separately enable the console invocation. No existing receipt,
schema version, parser, frozen oracle or expectation is rewritten to make this proposal pass.

Rollback disables the opt-in invocation and returns to the existing CLI/pane baseline. Retain
captured experimental wrappers as historical artifacts with their schema and stage; never relabel
them as legacy output or remove failed measurements. Unknown future schemas fail closed for axes.
Default-wire additions, stronger assignment rules, changed scopes or expanded profile support
require a new accepted revision and compatibility review, not an unannounced producer update.

## Owner decisions and promotion

All four decisions are **UNRESOLVED**. The recommendations above make review concrete; none is an
owner call, and none blocks recording this proposed document.

| Decision | Recommendation requiring approval | Competing interpretation / consequence |
|---|---|---|
| D1: transport and bounds | Explicit opt-in versioned wrapper, unchanged embedded legacy payload, separate bounded wrapper overhead. | Add fields to the default receipt: affects every consumer, packet fixed point and parity profile. Requires a broader accepted wire change. |
| D2: first supported scope | Clean/stable query and tracked-path impact only, scalar annotations with no inheritance; range, untracked and other surfaces remain unsupported. | Include mutable/range profiles now: requires distinct observation/cohort witnesses and a larger qualification matrix. |
| D3: authority meaning | Attest citation identity and bookkeeping; keep declarations and relevance advisory. No initial accepted-source authority inference. | Emit repository-accepted semantic values now: requires an exact acceptance-admission contract; source labels alone cannot supply it. |
| D4: delivery meaning | `deliveryStage: EXPERIMENTAL` describes this annotation producer profile; cited capability status remains separate payload evidence. | Make delivery refer to the cited capability: requires authoritative per-capability stage witnesses and different per-value scope. |

Promotion requires the accepted choices, completed source implementation, deterministic fixtures,
independent review, applicable parity/resource gates and scoped dogfood evidence. A passing code
gate alone does not accept the wire or prove console usefulness. Reject or narrow this proposal if
it collapses claim scopes, invites acceptance inference, loses uncertainty, or cannot preserve the
default profile's behavior and measured cost.
