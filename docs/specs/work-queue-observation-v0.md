# Work Queue Observation V0

**Owner:** Russell Lewis  
**Date:** 2026-08-23  
**Intent status:** accepted (decision 0046, 2026-09-04)  
**Delivery status:** experimental (core library, `corvint work observe` and `propose-wave`, the Corvint self-dogfood adapter, and `conformance/work-queue-v0` landed 2026-09-04; repository adoption through `corvint work init` and `corvint work adapter` landed 2026-09-19 under decision 0321; Beamfall adapter, independent recorder, and the 500 shadow cycles NOT_RUN)  
**Rollout class:** shadow-only  
**Wire profiles:** `work-queue-policy/0`, `work-queue-snapshot/0`,
`work-queue-detail/0`, `work-queue-checkpoint/0`, `work-queue-observation/0`,
`work-capacity-envelope/0`, `work-wave-proposal/0`, `work-command-result/0`

## Agent digest
- Claim: Read-only queue snapshots produce deterministic shadow proposals, derived path clashes, and the largest collision-free wave, authorizing nothing.
- Status: accepted (decision 0046, 2026-09-04)/experimental (core library, `corvint work observe` and `propose-wave`, the Corvint self-dogfood adapter, and `conformance/work-queue-v0` landed 2026-09-04; repository adoption through `corvint work init` and `corvint work adapter` landed 2026-09-19 under decision 0321; Beamfall adapter, independent recorder, and the 500 shadow cycles NOT_RUN)
- Exists: a closed read-only observation and deterministic non-operative wave-proposal contract.
- Blocked on: the Beamfall adapter and the independent recorder; the Corvint-side algorithm, its conformance, and the Corvint self-dogfood adapter are unblocked.
- Read next: Decision; Authority and trust boundary; §5.6 Corvint-derived collision closure and maximal wave; §5.7 Repository adoption; Traceability and owner inputs.

## 1. Decision

Corvint may gain a generic read-only work-queue observer and deterministic shadow wave proposer. One
repository-tracked policy selects one repository-owned adapter. That adapter exports an atomic,
bounded queue snapshot, optional digest-bound details, and a final checkpoint. Corvint validates the
closed wires, binds their identities, and deterministically projects a possible wave.

V0 is non-operative. `corvint work observe` and `corvint work propose-wave` never claim, lease,
heartbeat, dispatch, review, repair, finish, merge, edit, add, or close work. The strongest freshness
label is `VALIDATED_AT`; a selected item is only `ELIGIBLE_AT` the observed checkpoint. A proposal is
disposable analysis, never permission to act.

**Amendment 2026-09-04 (decision 0046).** Corvint also derives collision closure itself. Every ticket
may declare the repository paths its work intends to touch; Corvint expands them at the snapshot's
repository source through its own dependency index, and two tickets clash when their expansions
share a path. Derived groups only add exclusions to adapter-exported groups. The proposal selects the
largest collision-free set of eligible tickets (§5.6), so an orchestrator can run the most tickets at
once. The proposal still authorizes nothing; starting the work remains the orchestrator's act.

## 2. User and measurable job

The user is an agent-fleet operator whose repository already owns a work protocol. V0 must answer,
within a declared scope and byte budget:

1. which facts the repository adapter asserted at one checkpoint;
2. which ready tickets were compatible with supplied capabilities, capacity, leases, and complete
   repository-produced collision closure;
3. why every ready ticket was selected, excluded, or abstained; and
4. whether Corvint validated identical repository and queue identities before and after capture.

The baseline is the repository's supported status and fanout commands. V0 is useful only if it
preserves that authority, produces byte-replayable capsules, and materially reduces agent-facing
bytes. It never promises that the result remains current after return.

## 3. Authority and trust boundary

The authoritative inputs are `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`, `docs/PRODUCT.md`,
`docs/TECHNICAL-BRAIN.md`, `docs/specs/genesis-backfill.md`, this contract after owner acceptance,
and the adopting repository's accepted policy and adapter output.

```text
tracked repository policy at target tree
  -> tracked fixed-operation repository adapter
  -> atomic snapshot + fixed detail set + final checkpoint
  -> Corvint validation and deterministic projection
  -> VALIDATED_AT observation + ELIGIBLE_AT shadow proposal
  -> repository orchestrator independently compares and decides whether to act
```

The adapter is trusted repository code, not hostile plugin code. Queue bodies are hostile data. The
adapter is authoritative only for meanings assigned by the tracked policy. Corvint is authoritative
only for validated bytes and its deterministic projection. Corvint does not authenticate whether the
adapter told the truth, and a Corvint proposal never gains repository authority.

For Beamfall, only supported `script/roadmap.sh` and `script/bf` code may interpret or mutate the
canonical roadmap and lifecycle. Corvint never parses roadmap shards, derived SQLite, locks,
worktrees, process listings, or coordinator internals.

## 4. Closed wire registry

### 4.1 Canonical primitives

All objects are closed: only keys listed in this section are legal, all listed keys are required,
and `null` is legal only where stated. JSON bodies contain no insignificant whitespace and are
followed by exactly one LF on transport. The LF is excluded from content identities and included
only in fields explicitly named `rawSha256`. Objects use lexicographically sorted UTF-8 key bytes.
Semantic arrays retain declared order; all other arrays sort by canonical element bytes and contain
no duplicates. `argv`, operation arrays, adapter receipts, tickets, proposal entries, and interpreter
chains are semantic. No Unicode normalization occurs.

- `Digest` is 64 lowercase hexadecimal SHA-256 characters. `OID` is 40 lowercase hexadecimal
  characters for object format `sha1` and 64 for `sha256`.
- A content ID is `<kind>:sha256:<Digest>` using the exact kind in §4.3.
- `Identifier` is 1..128 UTF-8 bytes and obeys the identifier-string restrictions below.
- A repository authority ID is `repo:<authority-token>`, and its queue ID is
  `queue:<authority-token>:<queue-token>`, where each token matches
  `[A-Za-z0-9][A-Za-z0-9._-]*`. Every repository-qualified stable ID is
  `<kind>:<same-authority-token>:<same-queue-token>:<local-token>`; required kinds are `ticket`,
  `lease`, `holder`, `collision`, `route`, `capability`, and `capacity`. Three kinds are shorter,
  as the §4.2 profiles show: `checkpoint` and `scope` are `<kind>:<authority-token>:<queue-token>`,
  and `access` is `access:<authority-token>:<context-token>`. The shared repository and queue tokens
  are the mechanical foreign-authority check.
  Each complete ID is at most 128 ASCII bytes.
- `Path` is a repository-relative POSIX path of 1..512 UTF-8 bytes with `/` separators, no leading
  `/`, no empty, `.`, or `..` segment, and no backslash, NUL, or control byte. A trailing `/` denotes a
  directory prefix.
- `Count` is a JSON string containing `0` or a non-zero decimal integer without leading zero,
  maximum `2147483647`. `Rank` has the same syntax and maximum, and is unique in one snapshot.
- Identifier strings and labels reject NUL, BOM, C0/C1 controls, bidi-format controls, and invalid
  Unicode. Prose additionally permits TAB, LF, and CR; canonical JSON uses only `\t`, `\n`, and
  `\r` for them, `\"` and `\\` where required, never optional `\/` or optional `\u` escapes.

Exact reusable records are:

| Record | Exact keys and value types |
|---|---|
| `RepositorySource` | `commit:OID`, `id:repository-source ID`, `materializationSha256:Digest`, `objectFormat:"sha1"|"sha256"`, `statusSha256:Digest`, `tree:OID` |
| `Checkpoint` | `id:checkpoint:*`, `version:identifier` |
| `Scope` | `complete:boolean`, `id:scope:*`, `ticketCount:Count` |
| `CapacityClass` | `availableUnits:Count`, `id:capacity:*` |
| `CapacityUse` | `classId:capacity:*`, `units:positive Count` |
| `RouteAlternative` | `id:route:*`, `requires:[capability:*]` |
| `ExecutableIdentity` | `fileSha256:Digest`, `mode:four lowercase octal permission digits`, `pathSha256:Digest` where the digest covers exact native absolute path bytes |
| `SelectionFacts` | `approvals:"CLEAR"|"BLOCKED"|"UNKNOWN"`, `dependencies:"SATISFIED"|"BLOCKED"|"UNKNOWN"`, `holds:"CLEAR"|"HELD"|"UNKNOWN"`, `lease:"ABSENT"|"PRESENT"|"UNKNOWN"` |
| `TicketSummary` | `atomicRepositoryAuthorityIds:[repo:*]`, `authority:"COMPLETE"|"UNKNOWN"`, `capacityUses:[CapacityUse]`, `collisionGroupIds:[collision:*]`, `declaredVersion:Identifier`, `dependencyTicketIds:[ticket:*]`, `detailPayloadSha256:Digest|null`, `lifecycle:lifecycle enum`, `queueAuthorityId:queue:*`, `rank:Rank`, `repositoryAuthorityId:repo:*`, `routeAlternatives:[RouteAlternative]`, `selectionFacts:SelectionFacts`, `ticketContentSha256:Digest`, `ticketId:ticket:*`, `ticketVersionId:ticket-version ID`, `touchPaths:[Path]` |
| `LeaseSummary` | `blocksSelection:boolean`, `capacityUses:[CapacityUse]`, `collisionGroupIds:[collision:*]`, `holderId:holder:*`, `leaseId:lease:*`, `leaseVersionId:lease-version ID`, `lifecycle:lifecycle enum`, `queueAuthorityId:queue:*`, `repositoryAuthorityId:repo:*`, `ticketId:ticket:*`, `ticketVersionId:ticket-version ID` |
| `DetailPayload` | `acceptanceCriteria:[prose]`, `body:prose|null`, `displayKey:label|null`, `evidenceHandles:[label]`, `owner:label|null`, `title:prose|null` |
| `Detail` | `detailId:work-queue-detail-record ID`, `payload:DetailPayload`, `payloadSha256:Digest`, `repositoryAuthorityId:repo:*`, `ticketId:ticket:*`, `ticketVersionId:ticket-version ID` |
| `AdapterReceipt` | `adapterBlobOid:OID`, `adapterFileSha256:Digest`, `adapterMode:"100755"`, `argv:[Identifier]`, `containmentClass:containment enum`, `executableQualification:"EXACT"|"UNQUALIFIED"`, `exitCode:Count|null`, `id:adapter-execution ID`, `interpreterChain:[ExecutableIdentity]`, `operation:"snapshot"|"details"|"verify"`, `policyId:work-queue-policy ID`, `signal:Identifier|null`, `state:"PASSED"|"FAILED"|"INCOMPLETE"`, `stderrBytes:Count`, `stderrRawSha256:Digest`, `stdoutBytes:Count`, `stdoutRawSha256:Digest` |
| `CollisionGroup` | `id:collision:*`, `memberTicketIds:[ticket:*]`, `path:Path|null`, `source:"ADAPTER"|"CORVINT_INDEX"` |
| `ProposalEntry` | `collisionGroupIds:[collision:*]`, `reason:proposal-reason`, `routeAlternativeId:route:*|null`, `state:"SELECTED"|"EXCLUDED"|"ABSTAINED"`, `ticketId:ticket:*`, `ticketVersionId:ticket-version ID` |

Lifecycle enum is exactly `READY|BLOCKED|HELD|ACTIVE|REVIEW|REPAIR|DONE|RETIRED|UNKNOWN`.
Containment enum is exactly `LINUX_CGROUP_V2|LINUX_PROCESS_GROUP_UNQUALIFIED|DARWIN_PROCESS_GROUP_UNQUALIFIED|UNSUPPORTED`. Windows is unsupported in V0.

### 4.2 Top-level profiles

`work-queue-policy/0` is exactly:

```json
{"accessContextId":"access:corvint:opaque","adapterPath":"script/corvint-work-queue","adapterProfile":"repository-work-queue-adapter/0","detailLimit":"512","id":"work-queue-policy:sha256:64-lowercase-hex","mappingVersion":"opaque-version","operations":{"details":["details"],"snapshot":["snapshot"],"verify":["verify"]},"profile":"work-queue-policy/0","queueAuthorityId":"queue:corvint:opaque","repositoryAuthorityId":"repo:corvint","scopeId":"scope:corvint:opaque"}
```

`operations` contains exactly `details,snapshot,verify`; `detailLimit` is `Count` at most 512; each
operation value is 1..8 identifier strings,
1..128 bytes each. Corvint executes `adapterPath` followed by the corresponding literal array. No
runtime substitution is allowed.

`work-queue-snapshot/0` is exactly:

```json
{"accessContextId":"access:corvint:opaque","capacityClasses":[],"checkpoint":{"id":"checkpoint:corvint:opaque","version":"opaque-version"},"detailRequestTicketVersionIds":[],"id":"work-queue-snapshot:sha256:64-lowercase-hex","leases":[],"policyId":"work-queue-policy:sha256:64-lowercase-hex","profile":"work-queue-snapshot/0","queueAuthorityId":"queue:corvint:opaque","repositoryAuthorityId":"repo:corvint","repositorySource":{"commit":"40-or-64-hex","id":"repository-source:sha256:64-lowercase-hex","materializationSha256":"64-lowercase-hex","objectFormat":"sha1","statusSha256":"64-lowercase-hex","tree":"40-or-64-hex"},"scope":{"complete":true,"id":"scope:corvint:opaque","ticketCount":"0"},"tickets":[]}
```

`tickets` is semantic ascending `(Rank, raw 32-byte ticketVersionId digest)` order. `leases`,
capacity classes, route alternatives, capabilities, capacity uses, collision groups, and detail
requests are canonical-byte sorted. Detail requests are exactly the first at most policy
`detailLimit` `READY` ticket versions in ticket order whose detail digest is non-null. All
eligibility-critical facts are in `TicketSummary`; details never affect selection.

`work-queue-detail/0` contains exactly
`details:[Detail], id:work-queue-details ID, profile:"work-queue-detail/0",
snapshotId:work-queue-snapshot ID`. `details` is canonical-byte sorted and exactly covers the
snapshot detail-request set. `payloadSha256` uses §4.3 and equals the summary declaration.

`work-queue-checkpoint/0` contains exactly
`checkpoint:Checkpoint, id:work-queue-checkpoint ID, policyId:work-queue-policy ID,
profile:"work-queue-checkpoint/0", repositorySource:RepositorySource,
snapshotId:work-queue-snapshot ID`.

`work-queue-observation/0` is exactly:

```json
{"adapterReceipts":[],"containmentClass":"DARWIN_PROCESS_GROUP_UNQUALIFIED","detailIds":[],"endCheckpoint":{"id":"checkpoint:corvint:opaque","version":"opaque-version"},"id":"work-queue-observation:sha256:64-lowercase-hex","mutationState":"UNCHANGED_OBSERVED","networkState":"HOST_UNOBSERVED","policyId":"work-queue-policy:sha256:64-lowercase-hex","profile":"work-queue-observation/0","queueSourceId":"queue-source:sha256:64-lowercase-hex","snapshotId":"work-queue-snapshot:sha256:64-lowercase-hex","startCheckpoint":{"id":"checkpoint:corvint:opaque","version":"opaque-version"},"state":"VALIDATED_AT","unknowns":[]}
```

Observation state is `VALIDATED_AT|PARTIAL|CONFLICTED|STALE|UNKNOWN`. Mutation state is
`ENFORCED_READ_ONLY|UNCHANGED_OBSERVED|CHANGED|UNKNOWN`. Network state is
`DENIED|HOST_UNOBSERVED|UNKNOWN`. Adapter receipts retain semantic operation order
`snapshot,details,verify`; detail IDs are canonical-byte sorted.

`work-capacity-envelope/0` contains exactly
`available:[CapacityClass], capabilities:[capability:*], id:work-capacity-envelope ID,
profile:"work-capacity-envelope/0", repositoryAuthorityId:repo:*`. Arrays are canonical-byte sorted.
The envelope is caller-asserted compatibility input, not authenticated capability evidence.

`work-wave-proposal/0` contains exactly
`capacityEnvelopeId:work-capacity-envelope ID, collisionClosure:[CollisionGroup],
entries:[ProposalEntry], id:work-wave-proposal ID,
mutationAuthority:false, observationId:work-queue-observation ID,
profile:"work-wave-proposal/0", queueSourceId:queue-source ID,
state:"ELIGIBLE_AT"|"EMPTY"|"STALE", unknowns:[Code], waveLimit:Count,
waveOptimality:"MAXIMUM"|"GREEDY"|"NONE"`. Entries retain ticket order; `collisionClosure` is
canonical-byte sorted and complete for the READY tickets (§5.6).

`work-command-result/0` contains exactly
`errorCode:command-error|null, id:work-command-result ID,
observation:work-queue-observation|null, profile:"work-command-result/0",
proposal:work-wave-proposal|null, state:"OK"|"ERROR"`. `OK` has exactly one non-null payload;
`ERROR` has both payloads null. This represents failures before source identity exists.

### 4.3 Exact identity preimages

Every content identity is
`SHA-256(UTF8(kind) || 0x00 || UTF8(profile) || 0x00 || canonicalBody)` and uses the canonical body
below. A calculated ID is always excluded from its own preimage.

| ID/field | kind | profile | canonical body |
|---|---|---|---|
| policy `id` | `work-queue-policy` | `work-queue-policy/0` | complete policy without `id` |
| repository source `id` | `repository-source` | `repository-source/0` | complete `RepositorySource` without `id` |
| snapshot `id` | `work-queue-snapshot` | `work-queue-snapshot/0` | complete snapshot without `id` |
| queue source ID | `queue-source` | `queue-source/0` | exact object `{accessContextId,adapterProfile,checkpoint,complete,policyId,repositorySourceId,scopeId,snapshotId}` where `complete` is the snapshot scope boolean |
| ticket version ID | `ticket-version` | `ticket-version/0` | exact object `{declaredVersion,queueAuthorityId,repositoryAuthorityId,ticketContentSha256,ticketId}` |
| lease version ID | `lease-version` | `lease-version/0` | complete `LeaseSummary` without `leaseVersionId` |
| detail `payloadSha256` | `work-queue-detail-payload` | `work-queue-detail-payload/0` | exact `DetailPayload` |
| detail `detailId` | `work-queue-detail-record` | `work-queue-detail-record/0` | complete `Detail` without `detailId` |
| detail document `id` | `work-queue-details` | `work-queue-detail/0` | complete detail document without `id` |
| checkpoint document `id` | `work-queue-checkpoint` | `work-queue-checkpoint/0` | complete checkpoint document without `id` |
| adapter receipt `id` | `adapter-execution` | `adapter-execution/0` | complete receipt without `id` |
| observation `id` | `work-queue-observation` | `work-queue-observation/0` | complete observation without `id` |
| capacity envelope `id` | `work-capacity-envelope` | `work-capacity-envelope/0` | complete envelope without `id` |
| proposal `id` | `work-wave-proposal` | `work-wave-proposal/0` | complete proposal without `id` |
| command result `id` | `work-command-result` | `work-command-result/0` | complete result without `id` |

Stable repository, queue, ticket, lease, and holder IDs never include commit, checkpoint, version,
rank, path, title, URL, hostname, or clock. Version IDs bind mutable content separately.

### 4.4 Bounds

Decoded depth is at most 24; one identifier/label at most 32 KiB encoded UTF-8; one prose field at
most 256 KiB; one array at most 10,000 elements; aggregate decoded nodes at most 250,000. Snapshot
input is at most 16 MiB and 10,000 tickets. Detail output is at most 32 MiB, 512 details, and 1 MiB
per detail. A ticket declares at most 256 `touchPaths`; one derived collision closure is at most 4,096 paths
(WQO-V0-042). Policy and checkpoint are at most 64 KiB each. Observation and capacity envelope are at
most 1 MiB each. Proposal is at most 16 MiB and its enclosing command result at most 17 MiB. An
observe command result is at most 2 MiB. Adapter stderr is at most 1 MiB per operation.
Aggregate adapter stdout is at most 49 MiB. Exceeding a bound stops the operation and emits
`work-command-result/0 ERROR/INPUT_LIMIT`; no truncated valid profile exists.

One invocation, from source acquisition through every adapter operation and the final freshness
check, is also stopped after 10 minutes of wall time. That bound is a hang detector, not a
performance budget (decision 0082): the former 30 seconds per operation failed gate tests on a
loaded host while nothing hung. Its expiry keeps the closed `ERROR/INPUT_LIMIT` result, and stderr
states that the hang detector stopped the invocation and that no input bound was exceeded, so the
code is never read as a measured input size.

Each source-qualification Git command has its own 10-minute hang detector. After its leader exits
or cancellation starts, a one-minute pipe-drain detector bounds descendants that retain command
pipes. These finite time guards and the output limits also bound resource retention when hostile
repository state makes Git or a descendant hang; they are not source-quality evidence or
performance budgets. Either expiry is inability and emits `ERROR/INPUT_LIMIT`, never
`SOURCE_UNQUALIFIED`, because Corvint did not establish the missing source fact. A shorter parent
deadline still wins.

## Requirements

### 5.1 Policy, source, and adapter

- `WQO-V0-001`: Every V0 document and nested record MUST implement the exact closed registry in §4.
  Unknown profiles, keys, enums, malformed framing, duplicate keys, invalid UTF-8/scalars, optional
  encodings, non-integer JSON numbers, and out-of-bound values fail before identity use.
  Clarification (2026-09-12, no new rule): writers and identity preimages use the same §4.1 escape
  profile, so prose containing U+2028 or U+2029 is written raw and a written document re-parses.

- `WQO-V0-002`: Every identity MUST use §4.3 exactly. Implementations MUST independently rederive
  all IDs before use. `statusSha256` hashes the complete bounded raw NUL-delimited porcelain-v2 status
  stdout; for a qualified clean source it is SHA-256 of the empty byte string.

- `WQO-V0-003`: The only authority root is target-tree regular `100644` blob
  `.corvint/work-queue-policy.json`. Corvint reads it from Git, not caller bytes. Its canonical LF-
  terminated bytes and ID MUST verify. Missing policy produces `SOURCE_UNQUALIFIED`; caller-selected
  adapter paths, IDs, scopes, mappings, operation arguments, or manifests are forbidden.

- `WQO-V0-004`: The adapter path MUST be a normalized repository-relative regular `100755` target-
  tree blob with no symlink traversal. Corvint executes only fixed policy operations with direct argv,
  closed stdin, cwd at an isolated target materialization, and VPO-V0-022's sanitized
  `verification-env/posix-local/0`. Adapter and script interpreter chains are qualified as in
  VPO-V0-024; unqualified chains add `EXECUTABLE_IDENTITY_UNQUALIFIED` and cannot support an exact-
  executed-bytes claim.

- `WQO-V0-005`: Repository source qualification follows VPO-V0-007..010: HEAD equals target commit,
  tree and index equal target tree, complete porcelain-v2 status including untracked paths is empty,
  and every tracked regular file/symlink is mode-and-byte verified. Sparse checkout, submodules,
  special files, unmerged/intent-to-add, skip-worktree, assume-unchanged, split/alternate index,
  replacement/graft objects, escaping symlinks, or incomplete Git facts are `SOURCE_UNQUALIFIED`.
  Ignored private queue state is not repository source; only the adapter interprets it and snapshot
  and checkpoint identities bind its complete effect.

  The Corvint-local repair interprets `materializationSha256` as the bare digest payload of
  VPO-V0-008's domain-separated `verification-materialization/0` identity over ordered
  `{blobOid,mode,path,rawSha256}` rows. The shared observer/producer source helper supplies
  the same commitment. Private standalone Git objects, HEAD, index, and fresh configuration
  are generated execution metadata outside that target-entry commitment; they never point
  at the caller's mutable Git metadata. Only the pinned target object closure is exported;
  missing parent history cannot trigger caller or ambient fallback. The observer owns one
  private target, HOME, and TMPDIR for the fixed operations and final freshness check.
  Same-run temporary compilation can be reused within that tuple; every operation launches
  a fresh process. The script's standalone temporary build also owns interruption cleanup.
  Every capture builds the producer from that run's source bytes. Compiled package objects may
  come from one per-user Go build cache under `/tmp`, outside the repository, its Git metadata,
  and every run tuple; Go keys it by compile-input content and toolchain, so it cannot substitute
  other source. The script uses it only while it is a non-symlink `0700` directory owned by the
  running user, and otherwise compiles into run-private scratch. The build uses `-trimpath`, so
  the private target's directory is not part of a package's cache key and a fresh target reuses
  unchanged package objects; trimming changes no observation field (decision 0130). §4.4's hang
  detector still includes this build.

- `WQO-V0-006`: Adapter receipts bind policy, adapter blob/mode, interpreter qualification,
  operation argv, containment, exit/signal, and all stdout/stderr bytes. `stdoutRawSha256` and
  `stderrRawSha256` hash every raw byte observed, including the profile's terminal LF; counts cover
  the same bytes. Corvint uses no shell, closes inherited descriptors, applies §4.4, does not reuse
  sessions, and never constructs argv from queue data.

- `WQO-V0-007`: Adapter contract exposes only `snapshot`, `details`, and `verify`. It MUST NOT expose
  arbitrary pass-through, claim, release, heartbeat, dispatch, review, verdict, finish, merge, close,
  edit, add, amend, dependency-write, approval, or repair operations.

### 5.2 Snapshot closure and identity

- `WQO-V0-008`: A snapshot binds exactly one policy, repository authority, queue authority,
  repository source, checkpoint, access context, and declared scope. `scope.complete:false` is a valid
  partial diagnostic snapshot but can never yield `VALIDATED_AT`. Missing owner-assigned authority IDs
  are `SOURCE_UNQUALIFIED`, not locally composable success.

- `WQO-V0-009`: Queue source identity uses §4.3. Policy, adapter profile/mapping, access context,
  repository source, checkpoint, scope ID, `scope.complete`, or snapshot change creates another source.
  Observation time never changes immutable identity.

- `WQO-V0-010`: Stable ticket identity is authority-qualified `ticket:*`; mutable content uses
  `ticketVersionId`. Display keys, titles, URLs, rank, paths, and commits never establish identity.

- `WQO-V0-011`: Stable lease identity is `lease:*`; version ID binds the complete summary.
  `blocksSelection`, lifecycle, holder, ticket version, capacity, and collision groups are repository
  facts. Corvint never uses wall time to expire, reclaim, or supersede a lease.

- `WQO-V0-012`: Every dependency outcome, hold, approval, lease absence, route, capacity use, and
  collision group required for selection is complete in summaries. Every typed reference carries the
  snapshot `repo:*`; foreign authority is `MULTI_REPO_UNSUPPORTED`. Missing references, conflicting
  stable IDs/versions, contradictory facts, duplicate ranks, or incomplete collision closure make the
  observation `CONFLICTED` or `UNKNOWN` per §5.3 and force an empty proposal.

Cross-field validation is exact:

| Invariant | Failure |
|---|---|
| snapshot `policyId`, repository, queue, access and scope equal the tracked policy; details `snapshotId` equals snapshot; checkpoint `policyId`/`snapshotId` equal policy/snapshot and its source equals the reacquired source; observation policy/snapshot/source/checkpoints equal those referenced artifacts | `CONFLICTED`, except an advanced reacquired source/checkpoint is `STALE` |
| `scope.ticketCount == len(tickets)` and every stable/version ID is unique | `CONFLICTED` |
| every dependency/ticket/lease/capacity reference resolves in the snapshot; route, capability and collision IDs are authority-qualified; capabilities are matched against the later envelope and collisions join by exact ID equality | missing resolvable reference is `UNKNOWN_REFERENCE`; malformed or foreign authority is `MULTI_REPO_UNSUPPORTED` |
| every ticket's `atomicRepositoryAuthorityIds` contains the snapshot repository exactly once and no other authority | missing is `CONFLICTED`; foreign is `MULTI_REPO_UNSUPPORTED` |
| every ticket/lease version ID, detail digest/ID, snapshot ID and source ID rederives exactly | `CONFLICTED` |
| details cover exactly `detailRequestTicketVersionIds` and match their ticket/version/digest | `PARTIAL/DETAIL_MISSING` for missing; extra or conflicting is `CONFLICTED` |
| snapshot checkpoint equals observation start; verify policy/snapshot/repository fields equal capture; verify checkpoint equals observation end | mismatch is `STALE/CHECKPOINT_CHANGED` unless bytes contradict the same stable version, which is `CONFLICTED` |
| every capacity class ID is unique; every capacity use references one declared class; arithmetic fits `Count` | `CONFLICTED` |

### 5.3 Observation, failure, and nonmutation

- `WQO-V0-013`: `work observe` executes `snapshot`, `details`, then `verify`; validates every wire
  and ID; and emits one observation binding receipts, details, start checkpoint, final checkpoint,
  and source. Successful observation has exactly three `PASSED` receipts in that semantic order.
  Observation containment is the weakest receipt class in order `UNSUPPORTED < process-group
  unqualified < LINUX_CGROUP_V2`; mixed Darwin/Linux classes are `UNSUPPORTED`. It contains no raw
  body, credential, absolute root, environment, or output body; receipt argv is only the tracked
  policy-fixed non-secret argv.

- `WQO-V0-014`: State precedence is exact: identity contradiction is `CONFLICTED`; else detected
  mutation or adapter failure is `UNKNOWN`; else pre/post repository or checkpoint difference is
  `STALE`; else positively incomplete scope/details is `PARTIAL`; else inability to establish
  authority, parse, bounds, access, adapter behavior, or closure is `UNKNOWN`; only complete equal
  facts are `VALIDATED_AT`. `VALIDATED_AT` is historical immediately after return, never continuing
  currency.

- `WQO-V0-015`: Unknown codes are exactly `ACCESS_INCOMPLETE`, `ADAPTER_INVALID`,
  `CHECKPOINT_CHANGED`, `COLLISION_CLOSURE_INCOMPLETE`, `CONTAINMENT_UNQUALIFIED`, `DETAIL_MISSING`,
  `EXECUTABLE_IDENTITY_UNQUALIFIED`, `HOSTILE_INPUT`, `INPUT_LIMIT`, `MULTI_REPO_UNSUPPORTED`,
  `MUTATION_DETECTED`, `MUTATION_ENFORCEMENT_UNQUALIFIED`, `NETWORK_UNOBSERVED`, `PROCESS_RESIDUE`,
  `REPOSITORY_DIRTY`, `ROUTE_UNKNOWN`, `SOURCE_UNQUALIFIED`, and `UNKNOWN_REFERENCE`. They are unique
  and canonical-byte sorted. Command errors are exactly `ADAPTER_FAILED`, `CANCELLED`, `HOSTILE_INPUT`,
  `INPUT_LIMIT`, `MALFORMED_INPUT`, `SOURCE_UNQUALIFIED`, and `UNSUPPORTED_PLATFORM`.
  Source Git command-deadline and pipe-drain expiry map to `INPUT_LIMIT`; neither may assert
  `SOURCE_UNQUALIFIED` without the Git facts needed to establish a source refusal.

- `WQO-V0-016`: Derived state is disposable. Cache keys bind policy, source, scope, and access
  context. ACL non-interference forbids returning any path, identifier, digest, count, aggregate, or
  fact unavailable to the caller. Revocation, scope contraction, or context change purges or
  invalidates affected state; contexts are never unioned.

- `WQO-V0-017`: Corvint MUST NOT write repository, index, worktree, queue, checkpoint, lease/coordinator
  store, `.corvint` trace, or user configuration. `ENFORCED_READ_ONLY` requires OS enforcement.
  Otherwise Corvint reports only `UNCHANGED_OBSERVED` after complete pre/post manifests and adds
  `MUTATION_ENFORCEMENT_UNQUALIFIED`; this does not prove no transient/restored write. Detected change
  is `CHANGED`, adds `MUTATION_DETECTED`, makes state `UNKNOWN`, and discards selections.

  The closed V0 policy declares no external store roots. The default local detect-only
  runner therefore retains `mutationState:UNKNOWN` even after equal bounded manifests of
  the caller tree (including ignored data and `.corvint`), resolved Git/common metadata, and
  private target. It does not infer complete store scope from those roots or scan a user's
  whole HOME. Incomplete coverage contributes inability under WQO-V0-014; positively
  observed mutation contributes its higher-priority mutation/failure predicate. A closing
  source-qualification failure cannot erase an already observed change. Equal pre/post
  bytes do not detect transient or restored writes. This clarification adds no store-root
  configuration, OS enforcement, or stronger containment qualification.

  WQO-V0-046 is the one exception, and it adds no store root either: when the
  queue's store is provably the qualified committed tree itself, complete monitored
  manifests are complete store scope. Corvint's own `decision-0046-v0` self-dogfood
  mapping is not that exception and keeps the paragraph above.

### 5.4 Deterministic shadow proposal

- `WQO-V0-018`: `work propose-wave` consumes one `VALIDATED_AT` observation, exact snapshot, valid
  capacity envelope, and wave limit 1..128. It never refreshes or repairs. Any other state, foreign
  authority, or mismatch emits an empty proposal. Clarification (2026-09-13, bug hunt): an absent
  observation state is missing evidence and therefore emits an empty proposal; it MUST NOT default
  to `VALIDATED_AT`.

- `WQO-V0-019`: A ticket is eligible only when lifecycle is `READY`, authority `COMPLETE`, selection
  facts are `CLEAR,SATISFIED,CLEAR,ABSENT`, one route is a subset of caller capabilities, capacity fits
  both repository and caller pools, and collision groups intersect neither a blocking lease nor an
  earlier selection. Details, prose, age, history, priority, and model output never affect eligibility.
  Snapshot `availableUnits` is repository-authoritative free capacity already net of every active
  lease at its checkpoint; lease `capacityUses` is explanatory closure and MUST NOT be subtracted
  again. The effective starting units for each class are `min(snapshot available, caller available)`.
  The caller envelope MUST contain each snapshot class exactly once and no unknown class; missing,
  duplicate, conflicting, zero-use, or overflowing capacity data is `CONFLICTED` and yields no wave.

- `WQO-V0-020`: Corvint walks the WQO-V0-044 selection order once. For each selected ticket it chooses the canonical-
  byte-first satisfying route, subtracts every capacity use from both pools, and records collision
  groups. It stops selecting at the limit but classifies every remaining ready ticket. Equal inputs
  and execution profile produce byte-identical proposal bytes. Capacity uses for one ticket are first
  validated as a set, then subtracted together in canonical class-ID order; partial subtraction is
  forbidden.

- `WQO-V0-021`: Every `READY` ticket appears exactly once using the first applicable row and exactly
  one reason.

| Priority | Condition | State | Reason |
|---:|---|---|---|
| 1 | authority or selection fact unknown | `ABSTAINED` | `UNKNOWN_AUTHORITY` |
| 2 | known hold, approval, dependency, or lease block contradicts READY | `ABSTAINED` | `CONTRADICTORY_READY` |
| 3 | no route alternative is satisfied | `EXCLUDED` | `ROUTE_UNAVAILABLE` |
| 4 | capacity cannot fit | `EXCLUDED` | `CAPACITY_EXHAUSTED` |
| 5 | collision with blocking lease | `EXCLUDED` | `ACTIVE_COLLISION` |
| 6 | collision with earlier selection | `EXCLUDED` | `SELECTED_COLLISION` |
| 7 | wave limit reached | `EXCLUDED` | `WAVE_LIMIT` |
| 8 | otherwise | `SELECTED` | `ELIGIBLE_AT_CHECKPOINT` |

Contradictory READY input makes the global observation `CONFLICTED`; row 2 exists for standalone
algorithm conformance and never permits selection. Proposal reasons are exactly table literals.
Selected entries carry the chosen route ID. Every excluded or abstained entry has
`routeAlternativeId:null`. Entries excluded under rows 5 and 6 list the blocking collision group IDs
in `collisionGroupIds`; every other entry has `collisionGroupIds:[]`. Proposal `unknowns` copies observation unknowns and, only after
WQO-V0-025 drift, adds `CHECKPOINT_CHANGED`. Proposal state is `STALE` only after that drift,
otherwise `ELIGIBLE_AT` when at least one entry is selected and `EMPTY` when none is selected.

- `WQO-V0-022`: Capabilities are opaque repository-defined exact facts. Runtime, model, reasoning,
  skills, isolation, review separation, permissions, and concurrency must be encoded by the adapter.
  Corvint chooses only explicitly emitted alternatives and never downgrades, invents, or authenticates
  caller capabilities.

- `WQO-V0-023`: Non-READY lifecycle classes are never entries or selections. Continuation/adoption
  needs another profile. Corvint never reclaims leases, bypasses holds, satisfies dependencies, adopts
  review, or reinterprets approval.

- `WQO-V0-024`: V0 supports exactly one repository authority. Every typed fact uses it. Any foreign
  or atomic cross-repository fact makes state `UNKNOWN/MULTI_REPO_UNSUPPORTED` and proposal empty;
  Corvint never splits it.

- `WQO-V0-025`: Immediately before return, Corvint reruns fixed `verify` and repository acquisition.
  Mismatch produces `STALE` without selections; prior analysis is diagnostic only. Consumers freshly
  compare the complete queue source ID. Only repository atomic claim can decide action-time eligibility.

### 5.5 Hostile data and claim language

- `WQO-V0-026`: Queue IDs, titles, bodies, criteria, comments, route labels, holder names, reasons,
  URLs, and diagnostics are untrusted and cannot alter policy, schema, rank, identity, argv,
  environment, paths, framing, authority, lifecycle, or tool selection.

- `WQO-V0-027`: Corvint never executes, interpolates, templates, evaluates, or shells queue data.
  Machine output stays canonical JSON. Human/agent rendering labels fields `UNTRUSTED_QUEUE_DATA` and
  renders prose as escaped JSON string content; it never decodes control escapes to a terminal,
  Markdown directive, prompt role, link control, path, or command. Disallowed code points fail
  `HOSTILE_INPUT`; allowed confusables remain distinct inert scalars.

- `WQO-V0-028`: Presentation may say only `VALIDATED_AT(checkpoint)`,
  `ELIGIBLE_AT(checkpoint)`, `EXCLUDED_AT(checkpoint)`, `ABSTAINED_AT(checkpoint)`, `STALE`, or
  `UNKNOWN`, with mutation, containment, and network qualification shown separately. `CURRENT`,
  `CLAIMABLE`, `SAFE`, `APPROVED`, `MERGEABLE`, `DONE`, and generic green are forbidden.

### 5.6 Corvint-derived collision closure and maximal wave

Added by decision 0046 (2026-09-04) to answer two operator questions directly: are two pieces of
work likely to clash on this repository, and what is the most work that can start at once.

- `WQO-V0-041`: Each ticket's `touchPaths` declares the repository paths its work intends to change,
  as `Path` values sorted by canonical bytes without duplicates; an empty array declares nothing and
  derives no group. A trailing `/` matches every tracked path under that prefix at the snapshot
  repository source. Declared paths are repository facts exported by the adapter; Corvint never infers
  them from prose, titles, details, or history.

- `WQO-V0-042`: Corvint expands each ticket's declared paths at `repositorySource.commit` into a
  collision closure: the declared paths, every tracked path matched by a declared prefix, and every
  tracked path that the Corvint dependency index at that commit records as directly importing or
  directly imported by a declared file. Paths that are untracked, unparsed, or of an unindexed
  language contribute only themselves. Expansion reads immutable commit content only, never the
  working tree, and writes no index snapshot or trace, so it stays a read command under invariant 4.
  A Go import path resolves to the package directory under the tracked `go.mod` whose module path is
  its longest segment-boundary prefix; a nested module excludes its directory from an enclosing
  module. Every tracked `.go` file in that package is a direct neighbour. Standard-library, external,
  ambiguous, and otherwise unresolvable imports add no path rather than inventing a neighbour.
  A closure larger than 4,096 paths, or an index that cannot be built for that commit, adds
  `COLLISION_CLOSURE_INCOMPLETE`, makes the observation `UNKNOWN`, and empties the proposal.
  Completeness is relative to this direct indexed-path expansion, not proof of transitive or
  semantic independence. Decision 0156 retains this closure and its wire output unchanged.

- `WQO-V0-043`: Two READY tickets clash when their closures share one path. For every path shared by
  at least two READY tickets Corvint emits one `CollisionGroup` with `source:"CORVINT_INDEX"`, `path` set
  to that path, `memberTicketIds` listing every READY ticket whose closure contains it, and `id` equal
  to `collision:<authority-token>:<queue-token>:corvint-` followed by the first 32 lowercase hexadecimal
  characters of SHA-256 over the exact path bytes. Adapter-exported groups appear with
  `source:"ADAPTER"` and `path:null`. Derived group IDs are joined to each member's
  `collisionGroupIds` for selection; they only add exclusions, never remove an adapter group, and never
  intersect a lease, because leases carry adapter groups only. The complete set is the proposal's
  `collisionClosure`; two tickets clash exactly when one group lists both.

- `WQO-V0-044`: Selection maximizes the number of selected tickets. Let C be the READY tickets that
  pass rows 1..5 of WQO-V0-021. When C has at most 64 members Corvint computes an exact maximum
  collision-free subset M of C by deterministic branch and bound (branch on the candidate with the
  most clashes inside the remaining candidate set, ties by ascending rank, include-branch first; a
  branch whose greedy clique-cover bound cannot strictly beat the best set is pruned; the first maximum
  reached in that order wins) and reports `waveOptimality:"MAXIMUM"`. The search visits at most
  1,000,000 nodes. When C has more than 64 members, or the node budget is exhausted, Corvint computes M
  by walking C in ascending (clash count within C, rank) order and reports `"GREEDY"`. Selection order is the members
  of M in ascending rank followed by the remaining members of C in ascending rank; WQO-V0-020 walks
  that order once, applying capacity, collision with earlier selections, and the wave limit. An empty
  C reports `"NONE"`. Equal inputs produce byte-identical proposals under either mode.

- `WQO-V0-045`: The wave is analysis, never dispatch. Corvint reports which tickets can start together
  and why every other READY ticket cannot; starting them, claiming leases, or ordering agents is the
  repository orchestrator's act outside every V0 command, and a proposal naming N tickets is not
  permission to start any of them. The public queue analysis commands are `work observe` and
  `work propose-wave`. Decision 0156 retires the unaccepted `lane-plan` command and its help
  topic: both invocation and help MUST refuse with `invalid-arguments`, exit 2, no stdout and
  no stdin read; root help and the public command inventory MUST omit it. It is not an alias
  for a WQO command.

### 5.7 Repository adoption (decision 0321)

These requirements let a repository other than Corvint reach a `VALIDATED_AT` observation
without a new policy vocabulary, store root, or adapter protocol. They change no wire profile.

- `WQO-V0-046`: The store scope is complete exactly when the policy `mappingVersion` is
  `repository-worklist-v0` and Corvint, independently of the adapter, recomputes that closed
  mapping from the qualified committed tree and obtains byte-identical canonical snapshot,
  details, and checkpoint documents. The committed worklist is then the queue's whole store, so
  complete monitored pre/post manifests of WQO-V0-017 are complete store scope and equal
  manifests report `UNCHANGED_OBSERVED`. The final check repeats the comparison for the second
  checkpoint against the closing source. Any other mapping version, an unreadable or invalid
  worklist, or any differing byte leaves the store unqualified: `mutationState:UNKNOWN`,
  `SOURCE_UNQUALIFIED`, empty proposal. `CONTAINMENT_UNQUALIFIED`,
  `EXECUTABLE_IDENTITY_UNQUALIFIED`, `MUTATION_ENFORCEMENT_UNQUALIFIED`, and
  `NETWORK_UNOBSERVED` stay present; this requirement qualifies store scope only.

- `WQO-V0-047`: `corvint work init --repository NAME` writes exactly three files for the
  operator to review and commit: `.corvint/work-queue-policy.json` (a `work-queue-policy/0`
  with `access:NAME:local`, `repo:NAME`, `queue:NAME:worklist`, `scope:NAME:worklist`,
  adapter profile `repository-work-queue-adapter/0`, mapping `repository-worklist-v0`, detail
  limit 512), `.corvint/worklist.json` (an empty `corvint-worklist/0`), and the executable
  `.corvint/work-queue-adapter`, which runs `corvint work adapter`. NAME must yield a policy
  the WQO-V0-001 parser accepts. If any of the three paths exists, init writes nothing and
  exits 2. Initialization is rooted in the repository, rejects a symlinked `.corvint`, and
  rolls back files it created if any later write fails. Init accepts only the repository root (not
  a plain directory or repository subdirectory), and the generated adapter uses `/bin/sh`. Init neither stages nor commits;
  until the three files are committed, observe
  returns `ERROR/SOURCE_UNQUALIFIED`. Because the observer runs the adapter under the
  fixed VPO-V0-022 `PATH`, `corvint` must be installed in `/opt/homebrew/bin` or
  `/usr/local/bin`; otherwise observation fails `ADAPTER_FAILED`.

- `WQO-V0-048`: `corvint work adapter snapshot|details|verify` is the
  `repository-work-queue-adapter/0` producer shared with the self-dogfood adapter. It reads only
  the qualified committed policy and worklist and writes one canonical document. Every
  `corvint-worklist/0` item becomes a READY ticket `ticket:NAME:worklist:ID`, ranked in file
  order, with one `capacity:NAME:worklist:agent` unit, route `route:NAME:worklist:agent`
  requiring `capability:NAME:worklist:agent`, and the item's `touchPaths`. Bounded verification
  work (a suite batch, a failure-classification repair, a test-validity receipt, a cleanup and
  retry) is an ordinary item whose `touchPaths` name what it changes; two items that share a
  path clash under WQO-V0-043 and the proposal exposes the excluded one. The adapter holds no
  dispatch, lease, merge, or execution authority, and neither do `observe` or `propose-wave`
  (WQO-V0-045).

Explicit unknowns for an adopted repository:

| Condition | Result |
|---|---|
| no committed policy, adapter, or worklist | `ERROR/SOURCE_UNQUALIFIED`; no observation |
| dirty, mixed, or partially committed worktree | `ERROR/SOURCE_UNQUALIFIED`; no observation |
| unknown mapping version, or adapter bytes that differ from the recomputed mapping | `UNKNOWN`, `SOURCE_UNQUALIFIED`; empty proposal |
| `corvint` absent from the fixed `PATH`, adapter crash, timeout, or oversized output | `ERROR/ADAPTER_FAILED`, `INPUT_LIMIT`, or the WQO-V0-032 code |
| worklist or source changes between the two checkpoints (queue-source drift) | `STALE`; empty proposal |
| executable identity, containment, OS mutation enforcement, network | always-present unknowns; never qualified by adoption |

### 6. Beamfall ownership mapping

| Concern | Beamfall authority | Corvint V0 authority |
|---|---|---|
| Ticket facts and readiness | roadmap protocol | digest-bound projection only |
| Claims, leases, heartbeat, reclaim, release | Beamfall lease protocol | none |
| Routing, capacity, reservation collision | Beamfall planner semantics | exact opaque facts, plus additive Corvint path-closure groups (§5.6) |
| Review, repair, gates, merge, closeout | Beamfall workflow | none |
| Proposal ordering | repository rank plus WQO algorithm | exact bytes only |

- `WQO-V0-029`: Beamfall MUST add one supported atomic read-only adapter surface equivalent to
  `script/bf corvint-work-queue snapshot|details|verify`. It may internally use Beamfall implementation;
  Corvint cannot. It captures roadmap and leases at one checkpoint, exports complete conflict groups
  equivalent to Beamfall path-overlap, exclusivity, unknown-path, per-repository capacity, and active-
  reservation rules, and performs no roadmap, SQLite, status-file, claim, lease, lock-file, worktree,
  or coordinator write. Existing `status`/`fanout-plan` is only a comparison oracle while it refreshes
  derived state.

- `WQO-V0-030`: Corvint MUST NOT call Beamfall mutation verbs or `fanout-plan --claim`. Adapter or
  proposal success never authorizes them.

### 7. Failure and containment policy

- `WQO-V0-031`: `PARTIAL`, `CONFLICTED`, `STALE`, or `UNKNOWN` always produces an empty proposal.
  Missing requested details make the observation `PARTIAL/DETAIL_MISSING` even though details are not
  selection-critical. Scope, leases, collision closure, routes, and capacity can never be partial.

- `WQO-V0-032`: Timeout, cancellation, non-zero exit, malformed bytes, digest mismatch, overflow,
  residue, mutation, or final-check failure emits a closed error/observation and empty proposal. Linux
  cgroup v2 may claim escape-resistant cleanup only when an independent supervisor survives
  coordinator interruption. Unix process groups target only members; they always
  add `CONTAINMENT_UNQUALIFIED` and cannot claim absence of `setsid` escape or crash residue.
  `UNSUPPORTED` fails closed. Containment is cleanup, not a security sandbox.

  Command output mapping is exact: failure before a complete observation exists emits
  `work-command-result/0 ERROR` with its command error and no payload; a structurally complete
  non-`VALIDATED_AT` observation emits `OK` with that observation; successful observe emits `OK` with
  the observation; proposal over a valid but non-validated observation emits `OK` with an empty
  proposal; malformed command input emits `ERROR/MALFORMED_INPUT`. No invocation emits both payloads.
  `work --help` and `help work` are help requests, not command input: they print usage text on stdout
  and exit 0 before any work command is parsed (amendment 2026-09-12).

- `WQO-V0-033`: Corvint core makes no network request, model call, database, daemon, embedding, or
  hosted-service call. Adapter network is `DENIED` only under enforced denial; otherwise
  `HOST_UNOBSERVED` with `NETWORK_UNOBSERVED`. V0 never claims hermeticity. ACL mismatch invalidates
  state per WQO-V0-016.

### 8. Deterministic acceptance

- `WQO-V0-034`: Independent Go conformance validates every schema, identity, decision row, bound,
  error, and fixture below without importing adapter code.

  | Area | Required fixtures |
  |---|---|
  | Wires/identities | minimum/maximum, unknown/duplicate keys, framing, all preimages, self-reference trap, old/new profile |
  | Git source | HEAD/tree/index drift, dirty/untracked, skip/assume flags, alternate index, replace refs, submodule, symlink swap |
  | Snapshot | equal start/end, queue/repository/policy/access drift, partial scope, conflicting version/rank, oracle mismatch |
  | Details | deterministic request, missing/extra/wrong digest, 512/513, detail changes never alter selection |
  | Leases/collisions | active/apparently expired lease, every Beamfall collision class, out-of-scope conflict, incomplete closure |
  | Route/capacity | alternatives, absent capability, no downgrade, multi-resource subtraction, both exhaustion sources, limit |
  | Multi-repo | every foreign typed reference and atomic cross-repo fact empties wave |
  | Hostile data | prompt/role, shell, ANSI/OSC, Markdown, traversal, NUL/control/bidi, confusable, oversized prose |
  | Process/network | timeout, cancel, overflow, child/`setsid`, coordinator crash, blocked stdin, non-reuse, denied/unobserved |
  | Determinism | 100 fresh processes, shuffled non-semantic arrays, hostile locale/time/env, parallel schedules |
  | Nonmutation/ACL | enforced/detect-only, transient-write qualification, all manifests, high/low context non-interference |
  | Derived collisions/wave | exact path, prefix match, import neighbour both directions, unindexed path, closure overflow, 64 exact versus 65 greedy, equal-size tie-break, clash symmetry, adapter group preserved |

### 9. Shadow rollout and promotion

- `WQO-V0-035`: Rollout is frozen conformance, Corvint self-dogfood, Beamfall shadow, 500 consecutive
  Beamfall cycles, then explicit owner acceptance of assisted display. Shadow never changes decisions.

- `WQO-V0-036`: Before counting, freeze an independent recorder profile/version, repository-oracle
  commands, policy/adapter tuple, replay artifacts, byte-baseline encoding, denominators, and evaluator
  digest. Corvint cannot grade itself. Each cycle records oracle and Corvint at the same checkpoint and
  proves identities, asserted readiness, leases, dependencies/holds/approvals, route/capacity,
  collision closure, deterministic bytes, and no stale result presented as validated.

- `WQO-V0-037`: Of 500 consecutive cycles, at least 100 contain active leases, 100 at least two
  route/capacity classes, and 100 a collision/exclusion/abstention; sets may overlap. Unsafe selection,
  authority mismatch, stale-as-validated, nondeterminism, detected mutation, omitted constraint, or
  unclassified state resets the count after repair. Unqualified containment/mutation is reported and
  never masquerades as a strong claim.

- `WQO-V0-038`: Passing permits only owner-reviewed assisted read-only display. Where the frozen
  Beamfall oracle reports compatible ready work, at least 95% of cycles yield a non-empty proposal.
  Median canonical proposal transport bytes are at most 25% of frozen canonical broad-status/fanout
  baseline bytes at the same checkpoint. Safety overrides value. Operative phases require a new
  accepted spec, repository compare-and-act, replay protection, independent review, and approval.

### 10. Rollback and compatibility

- `WQO-V0-039`: Rollback disables commands and deletes disposable state without canonical repair.
  Historical observations remain `VALIDATED_AT` records and are never displayed as current.

- `WQO-V0-040`: Any policy, adapter/profile, mapping, schema, lifecycle, route/capacity/collision
  vocabulary, access context, scope, repository source, or command change creates another identity and
  invalidates proposals. Unknown future fields/states fail closed. Promotion never transfers tuples.

The experiment is killed if Beamfall cannot export the atomic bounded read-only view, if Corvint must
parse private state, if unsafe selection or forbidden operation occurs, or value gates fail.

## 11. Non-goals

- roadmap, ticket, claim, lease, worker, review, gate, merge, or closeout authority;
- parsing repository roadmap files, databases, locks, worktrees, or coordinator internals;
- ticket rewriting, reprioritization, dependency repair, or acceptance;
- cross-repository scheduling or shared mutable truth;
- model ranking, route downgrade, or self-selection; and
- daemon, hosted control plane, telemetry, UI, database, embeddings, or network fetch.

`corvint-worklist/0` (WQO-V0-048) is Corvint's own committed worklist format, not a parser for a
repository's existing roadmap files; a repository with another queue writes its own adapter.

## 12. Traceability and owner inputs

| Contract area | Requirements | Evidence |
|---|---|---|
| Closed wires/policy | WQO-V0-001..007 | schema/identity goldens, adapter transcripts |
| Snapshot closure | WQO-V0-008..012 | Git, queue, ticket, lease, collision fixtures |
| Observation | WQO-V0-013..017 | state, ACL, mutation evidence |
| Proposal | WQO-V0-018..025 | decision and deterministic goldens |
| Hostile data | WQO-V0-026..028 | injection and presentation corpus |
| Beamfall | WQO-V0-029..030 | read-only adapter and forbidden-verb spy |
| Containment | WQO-V0-031..033 | OS-qualified process/network evidence |
| Promotion | WQO-V0-034..040 | independent conformance, 500 cycles, rollback |
| Derived collisions/wave | WQO-V0-041..045 | closure goldens, optimality fixtures, clash report |
| Direct closure boundary | WQO-V0-042 | `internal/workqueue/collision.go`, `TestIndexCollisionSourceDirectNeighboursOnly`, `TestIndexCollisionSourceResolvesGoModuleImports`, `TestDeriveWorkCollisionsResolvesGoModuleImports` |
| Retired competing command | WQO-V0-045 | `cmd/corvint/main.go`, `cmd/corvint/help.go`, `TestLanePlanRetired` |
| Store scope by mapping reproduction | WQO-V0-046 | `cmd/corvint/work.go` `workMappingReproduced`, `TestWorkMappingReproduced`, `TestWorkAdoptedRepositoryWorklist` |
| Repository adoption | WQO-V0-047..048 | `cmd/corvint/work_adopt.go`, `internal/worklistadapter`, `TestWorkAdoptedRepositoryWorklist`, `TestWorkMappingReproduced` |

Decision 0046 accepted this contract and assigned the Corvint self-dogfood authority IDs
(`repo:corvint`, `queue:corvint:worklist`, `scope:corvint:worklist`, `access:corvint:local`); the core
library, both commands, the self-dogfood adapter, and `conformance/work-queue-v0` landed 2026-09-04
as experimental. Decision 0048 defers the Beamfall shadow: the adapter (`WQO-V0-029`) and the
independent recorder (`WQO-V0-036`) are Beamfall-owned, and until they exist the evidence is Corvint
self-use only and `WQO-V0-035..037` stay `NOT_RUN`. Acceptance grants no delivery claim.

The Corvint producer's exact requested-detail coverage is exercised by
`TestDocumentsRequestedDetails` and `TestDocumentsDetailFailures` in
`cmd/corvint-work-queue/main_test.go` (WQO-V0-008..012 and the §5.2 cross-field table).
`TestDocumentsRejectTrailingWorklistData` holds the producer's `docs/worklist.json` input to
exactly one JSON document: a second document or trailing bytes is malformed framing (WQO-V0-001)
and fails as `invalid worklist`, never a snapshot of the first value.
These serialize the actual producer output through native parsers and validators, retaining all
summaries across empty, zero, partial, equal, and above-count limits, with missing, rehashed
payload-tampering, and surplus-detail controls. At the A19 producer repair, the native direct
coverage function returned `UNKNOWN/DETAIL_MISSING` for missing details, contrary to the table's
`PARTIAL/DETAIL_MISSING`; that historical failure remains in `docs/BUILD-LOG.md`. The bounded
2026-09-06 protocol repair now verifies `PARTIAL/DETAIL_MISSING` with
`TestMissingDetailSuppressesWave` and the producer's missing-detail control. This is local
protocol evidence, not full observation or promotion qualification.

The same repair adds requirement-literal assertions in `internal/workqueue/`:
`TestProtocolRequiredShapeRepair`, `TestProtocolGrammarRepair`, and `TestProtocolPositiveRepair`
cover exact Policy/Details/Checkpoint keys, types, nullable fields, canonical ordering, and identities;
`TestProtocolHostileRepair` and `TestProtocolHostileBoundariesRepair` distinguish forbidden
identifier/label/prose controls from size and shape failures. Criteria and evidence arrays follow
§4.1's existing canonical-element ordering, while operation argument order remains semantic.
`TestDetailCoverageRepair` and `TestDetailRequestMustResolveExactlyOnce` bind each requested
version to its stable ticket and declared digest; foreign-only detail repository fields retain
`MULTI_REPO_UNSUPPORTED`, while independent tuple or tracked-policy contradictions remain
`CONFLICTED`. `TestForeignAtomicAndMissingOwnAuthority` exercises the combined foreign/missing-own
case rather than relabelling a contradictory snapshot as foreign-only.

`ValidateDetailRequests` supplies the policy-aware §4.2 check during capture; standalone
`ValidateSnapshot` and `ProposeWave` retain their policy-free contracts. `TestDetailRequestDerivation`
and `TestWorkDetailRequestDerivation` cover WQO-V0-001/012/031 with semantic selection before
canonical sorting, READY/non-null eligibility, zero and bounded limits, and exact request coverage.
Coherent extra, omitted, or wrong-prefix requests are `CONFLICTED`, including with drift or incomplete
closure/manifests; a correct request vector with missing returned details remains
`PARTIAL/DETAIL_MISSING`. Both produce empty proposals. `TestDetailRequestUnusablePolicy` and
`TestWorkDetailRequestUnusablePolicy` check unusable Count limits as `UNKNOWN/ADAPTER_INVALID`;
the standalone helper also rejects nil inputs without a panic. These are process-free protocol
witnesses, not production store-coverage or external outcome qualification.

`TestResolveStatePrecedence` asserts all 32 combinations of the five predicates in WQO-V0-014.
`TestWorkCaptureStateRepair` in `cmd/corvint/work_capture_repair_test.go` verifies process-free
capture assembly, receipt failures, diagnostic retention, source drift and identity contradictions,
missing details, and distinct manifest completeness versus observed change. Synthetic complete
manifests in those tests do not qualify production store coverage; that evidence belongs to the
separate execution/source slice.

`TestCapacityRepair` and `TestCapacityRepairPoolControls` cover same-authority semantic capacity
conflicts, foreign/nonvalidated abstention, legal zero availability, per-pool minima, atomic
subtraction, and already-net repository capacity. The Gate A compatibility interpretation retains
WQO-V0-018/024 empty proposals for valid nonvalidated or foreign inputs. For otherwise matching
validated inputs, invalid same-authority capacity is classified internally as `CodeConflicted`
and returns no proposal; the existing closed command mapping emits `ERROR/MALFORMED_INPUT` with
both payloads null. `TestWorkCommandErrorRepair` verifies that mapping and exact hostile-input
errors. `TestProposalMissingObservationStateIsEmpty` proves that a missing observation state cannot
select work. No replacement observation or new wire state is emitted. These witnesses close only the
named protocol defects; runner qualification, independent conformance, Beamfall integration,
recorder, 500-cycle evidence, and external promotion remain separately gated.

The bounded Corvint-local source/execution repair supplies focused production witnesses without
closing the whole WQO-V0-034 matrix. `internal/worksource/source_test.go` tests full source
qualification, immutable materialization preimages, hidden/stat-cache edits, unsupported index
and Git states, source/layout drift, fixed Git environment, output bounds and no caller writes.
`cmd/corvint/work_materialization_test.go` exercises private target/Git isolation, exact child
environment, cleanup, and actual script/producer parity for a multi-commit caller. The script
build disables automatic VCS stamping because the private target exports no parent history,
and trims build paths so the per-user cache hits across private targets; source equality still
binds the committed producer inputs.

`TestWorkLimitedBufferHashesAllObservedBytes`, `TestWorkRunnerTranscripts`,
`TestWorkRunnerInterpreterQualification`, and `TestWorkRunnerExecutableSwap` in
`cmd/corvint/work_runner_test.go` bind raw receipt bytes, fixed operations, terminal failures,
and actual unqualified executable/interpreter evidence. `cmd/corvint/work_process_unix_test.go`
exercises group descendants, bounded escaped-pipe capture, and coordinator interruption with
a separately registered bounded test guardian. This is process-group evidence, not stronger
containment qualification. `internal/contextindex/git_execution_unix_test.go` verifies the
request-scoped absolute Git/closed environment, immutable-only blob reads, ordinary compiler
parity, pipe ownership and cancellation without ambient fallback.

`TestObserveWorkUsesOnlyTargetMaterialization`, `TestObserveWorkMutationDiagnostics`, and
`TestObserveWorkFreshnessRetainsMutation` in `cmd/corvint/work_observe_test.go` reach production
snapshot/details/collision/verify paths. `cmd/corvint/work_mutation_test.go` retains positive
byte/name/type/mode/link changes through partial scope acquisition; complete external-store
coverage stays unknown. Source/runner failures, independent review findings and corrected
witnesses are retained in the repair evidence rather than relabelled as original passes.

The 2026-09-06 independent verification slice adds the following bounded evidence in
`conformance/work-queue-v0`. A row supports its named assertions, not automatic closure of the full
requirement or a promotion gate:

| Requirements | Independent witness | Scope and outcome |
| --- | --- | --- |
| WQO-V0-001 | `TestIndependentClosedInputRegistry` | Five publicly parsed input profiles and their reusable records: missing, null, wrong-type, duplicate and unknown fields, plus future profiles, reject before identity use. Original parser failures are retained; the reviewed protocol repair passes. Output profiles remain a separate CLI boundary. |
| WQO-V0-002 | `TestIndependentIdentityPreimages`, `TestIndependentIdentitySelfInclusionRejected`, `TestIndependentIdentityRawStatusBinding` | Fifteen frozen literal complete preimages and independent SHA-256 values; self-ID exclusion; canonical LF/raw-status byte binding. Raw receipt capture is separately runner-owned. |
| WQO-V0-019/020/023 | `TestProposalAtomicMultiresourceWitnesses`, `TestProposalEveryNonReadyLifecycleWitness`, `TestProposalLeaseCapacityAlreadyNetWitness` | Failed second-resource fits preserve the first resource, successful fits consume both, all eight non-READY states stay absent, net lease capacity is not subtracted again, and input facts remain unchanged. |
| WQO-V0-027 | `TestIndependentProseControlRegistry` | Forbidden scalar registry rejects with `HOSTILE_INPUT`; permitted TAB/LF/CR and inert shell/prompt/Markdown prose round-trip canonically. This is parser evidence, not execution isolation. |
| WQO-V0-016/040 | `TestProposalLibraryContextIsolationWitness`, `TestProposalContextContractionBindingWitness` | Explicitly trusted library inputs: high/low projection order, private sentinel exclusion, access/scope identity changes and old-checkpoint rejection. Fresh-command ACL authentication is not established by these tests. |
| WQO-V0-040 | `TestIndependentWireTupleInvalidation`, `TestIndependentFutureVocabularyRefusal` | Twelve changes to specified policy/source/checkpoint/lifecycle/route/capacity/collision/path dimensions invalidate old wire bindings; rehashed future adapter profiles and lifecycle values reject. Installed-Corvint-binary identity and source acquisition remain outside this wire-only witness. |
| WQO-V0-044 | `TestProposalFirstEqualMaximumWitness`, `TestProposalLiteralMillionNodeBudgetWitness` | Triangle-with-tail graph checks the exact first maximum and collision explanations. The original bounded 80-graph search reached only 3,195 nodes and retains its `NOT_PRODUCED` outcome. A later frozen graph of nine disjoint C7 cycles plus one isolated vertex exercises public `ProposeWave` with exactly 64 candidates and the unchanged 1,000,000-node budget: `GREEDY`, the exact 28 selections and every collision explanation pass, with byte-identical output after closure-order reversal. This is a production library witness, not CLI qualification or an exported node counter. |

The complete independent conformance package passed after protocol commit `2f0d86a` was integrated,
evidenced by the table above's checked-in tests. The `verificationSchemaRepaired1` run that was cited
as separately preserving exact pre/post source identities under
`/private/tmp/corvint-obligation-repairs-20260906/verification-task-evidence/` is `NOT_PRODUCED`: that
host scratch path was never checked in and has since been purged (verified 2026-09-12: the
directory and its 38 named per-run subdirectories, including this one, hold zero regular files). No
pre/post source-identity evidence from that run can be produced or re-checked, and none is
reconstructed here; WQO-V0-001, WQO-V0-002, WQO-V0-016, WQO-V0-019, WQO-V0-020, WQO-V0-023,
WQO-V0-027, WQO-V0-040, and WQO-V0-044 rest only on the table's named tests, not on this withdrawn
run. That withdrawn execution would not have established production eligible replay, independent
Beamfall rollout cycles, installed-Corvint-binary
wire invalidation, Linux enforcement, or actual Windows execution. Those qualifications remain
separate from local fixture results.

The R7-INT-001 final-check repair is covered by `cmd/corvint/work_final_check_test.go`.
`TestWorkFinalCheckCommandFailures` reaches the real second adapter verification and asserts
canonical `ERROR` envelopes, exit 2 and two null payloads for nonzero, malformed, hostile and
oversized responses. `TestWorkFinalCheckInterruptedAdapter` exercises actual bounded final-only
cancellation/deadline and checks both registered process-group PIDs disappear.
`TestWorkFinalCheckClosingContext` preserves known closing cancellation/deadline as terminal errors;
`TestWorkFinalCheckSourceBoundary` covers ordinary source rejection and qualified source advance.
`TestWorkFinalCheckCaptureBinding` checks final policy/snapshot contradictions, coherent checkpoint
and source drift, prior and final mutation, ordinary closing qualification failure, and equal control.
Complete final observations bind proposals before independent drift suppression; observation state
still uses the single §014 reducer. An unavailable closing source supplies no comparison witness,
and a canonical observation ID may remain equal when its evidence is already identical. These are
bounded Corvint-local witnesses for §021/025/032, not complete mutation-scope or external qualification.


The final bounded local evidence follow-up adds these independently reviewed witnesses:

| Requirements | Witness | Exact scope |
| --- | --- | --- |
| WQO-V0-007/030 | `TestProducerOperationBoundary` in `cmd/corvint-work-queue/operation_boundary_test.go` | Forbidden verbs and invalid argv fail before scratch/source acquisition; supported operations reach the discriminating acquisition failure. The direct test launches no adapter or Git child and does not qualify an external adapter. |
| WQO-V0-026 | `TestLocalProseSelectionInvariance` in `conformance/work-queue-v0/local_evidence_test.go` | Independently rebound benign/adversarial title/body fixtures retain authoritative facts, stable selected tickets, routes and exclusion/abstention reasons. Parsed complete wires, fifty frozen identity preimages and a stale-content negative support a nonempty pure `ProposeWave` witness. Its validated inputs are explicitly synthetic. |
| WQO-V0-028 | `TestLocalHistoricalSerialization` in the same file | Exact independent checkpoint A/B wire oracles and identities; serializing B leaves historical A objects, bytes and bindings unchanged. This tests the existing machine serializer, not a new renderer or actual eligible production. |
| WQO-V0-033 | The descriptive network-qualification case inside `TestObserveWorkUsesOnlyTargetMaterialization` | Actual capture retains `HOST_UNOBSERVED` and `NETWORK_UNOBSERVED`, alongside unknown mutation scope. It establishes the emitted qualifier, not absence of network/model/service activity. |

These focused Go tests and all five new native claim extraction/re-extraction checks passed on
exact retained source. The independent pure fixtures use hypothetical complete mutation evidence
for their synthetic validated state; the actual default observer over the `decision-0046-v0`
self-dogfood mapping remains `UNKNOWN` with incomplete mutation scope (the WQO-V0-046 adoption path
is witnessed separately by `TestWorkAdoptedRepositoryWorklist`). No synthetic state or structural link is promoted into production eligibility.

The native paired-prose CLI harness in `conformance/work-queue-v0/cmd/cli-prose` separately checks
the frozen identity registry and reconstructs capture inputs through the Go fixture producer. The
producer's snapshot, detail and checkpoint bytes are pinned against the unchanged independent
fixture wires. The native scope finalization control retains ten provisional passing cells while
recording the injected late tuple failure. Actual paired preparation and four production commands
remain pending until the final clean combined build is frozen. Expected production qualification
remains `UNKNOWN/EMPTY`. The corrected clean committed policy negatives and built-adapter operation
controls in `conformance/work-queue-v0/cmd/cli-replay` also require actual runtime receipts; source
review and fixture selftests do not
satisfy that execution requirement. Finite prose safety, historical eligible production, core
service-activity evidence, claim extraction and external/owner gates remain separate limits.
