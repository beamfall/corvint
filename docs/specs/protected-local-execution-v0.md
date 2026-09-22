# Protected local execution V0

Owner: Russell Lewis
Date: 2026-09-08
Requirement prefix: `PLE-V0`
Intent status: proposed
Delivery status: experimental

## Agent digest
- Claim: Optional fixed WASI candidate execution; no admitted execution root or FULL qualification.
- Status: proposed/experimental; decision 0009 still admits no execution root.
- Exists: `tools/local-authority`, `internal/localauthority`, experimental Frontier/OCM/TCQ consumers and native Stop adapter work.
- Blocked on: independent review, privileged principal separation campaign, accepted root, exact native Desktop tuple qualification and AHI performance/equal-recall evidence.
- Read next: Requirements; Wire contract; Runtime boundary; Acceptance and rollback.

## Human intent and scope

The owner selected a protected local runner and renewed the goal of FULL. This document freezes
an additive proposal before promotion; implementation or fixture results cannot accept its intent.
The default native Go product has no runtime dependency, account, service or execution authority.
The useful first subject is the pure-Go WP3 commitment codec. Native Darwin programs, arbitrary
repository tests, CEM data probes and caller-reported test verdicts are outside this execution profile.
A signed result is insufficient without separately admitted current protected policy.

## Requirements

- `PLE-V0-001`: The optional `corvint-protected-execution/0` profile MUST preserve all legacy profiles and default to authority unavailable without separately admitted policy. Unprivileged runs MUST emit `authority: NONE`.
- `PLE-V0-002`: A protected immutable release and independently admitted root/policy MUST bind the executable, driver, toolchain and check floor. Caller executable, root, key, signer-input or policy substitution MUST fail.
- `PLE-V0-003`: Ingestion MUST bound and independently bind expected Git base/target/tree, CEM, OCM, selection, source capsule, recipe and artifact; invalid objects, paths, races and replacement configuration MUST fail before authority.
- `PLE-V0-004`: Candidate build and execution MUST remain in a separate keyless principal with complete owned-process lifetime control, interpreter-only WASI, no filesystem preopens/network/native-process imports, bounded phases and cancellation. Prototype same-UID evidence MUST NOT qualify principal separation.
- `PLE-V0-005`: The protected driver MUST own all assertions, rows and completion. Exact expected canonical bytes/refusals plus valid host terminal, zero exit and cleanup are required; forged guest verdicts, incomplete answers and bad mutants MUST fail.
- `PLE-V0-006`: Attestation MUST use the bounded canonical wire contract below, after cleanup, binding all identities and complete result rows. Malformed framing, signatures or semantics MUST fail.
- `PLE-V0-007`: Protected enrollment MUST freeze nonempty ALL_SELECTED checks, nonce, audience, generation, epoch and freshness; policy rollback, replay/transplant, stale or revoked evidence MUST fail.
- `PLE-V0-008`: The additive OCM/TCQ/Frontier profiles MUST quantify every selected edge and recompute CWR against unique exact-source target definitions and base-stable requirements. Unmatched, changed, failed, absent or unsupported driver edges MUST remain OPEN.
- `PLE-V0-009`: The protected Stop consumer MUST derive current policy, enrollment, target and qualified host tuple independently and compute EMPTY/OPEN. The actual process cwd MUST independently map to the exact canonical admitted worktree before and after evaluation; payload cwd and a global enrollment MUST NOT choose repository scope. A caller boolean or prototype receipt MUST NOT grant authority; one protected-policy-permitted remediation is the maximum and recursion releases unresolved.
- `PLE-V0-010`: FULL MUST require the eleven AHI cases and latency/equal-critical-recall evidence for the exact actual native host tuple. Desktop and CLI tuples MUST remain distinct; synthetic observations MUST NOT qualify either.
- `PLE-V0-011`: Installation, upgrade, revocation and removal MUST follow reviewed immutable manifests, explicit operator action, principal/ancestor/ACL audits and complete process cleanup. No ambient claimant privilege, broad sudoers rule, permanent daemon or self-admitted key is permitted.

- `PLE-V0-012`: A temporary native qualification exercise MUST require a distinct root-owned canonical campaign admission lasting at most 900 seconds and binding current execution root, floor, policy, exact repository/enrollment/target, release images and boot/app/engine process lifetimes. It MUST apply the same protected computation/currentness/runtime/cwd checks, require explicit all-native-Stops repository scope and both root/campaign permission for one remediation, and always remain FALLBACK/UNQUALIFIED without a qualified host digest. It MUST expire/refuse on drift, never accept a public grant or repair invalid completed qualification, and never promote itself.

## Wire contract

The proposed profiles are `ocm/0.2-experimental`, `tcq/1-experimental`,
`frontier/2-experimental`, `corvint-protected-execution/0`, and `corvint-authority-event/0`.
All receipt objects are closed-schema WP3 canonical JSON; unknown or duplicate members, numbers,
noncanonical spellings and trailing data fail. Integer values are unsigned minimal decimal strings;
timestamps are Unix seconds. The receipt is at most 128 KiB. All SHA256 values are lowercase hex.
Ed25519 signs `corvint-protected-execution/0` followed by one NUL byte then exact WP3(payload).
The signature is lowercase hex. A JSON serialization used for prototype diagnostic receipts is not
this signing codec and cannot be accepted as an attestation.

Exact keys and types (all unmarked fields are strings):

| Object | Required members |
|---|---|
| Check | id, profile, driverSHA256, driverPath, driverUnit, claimSelector, invocation, subject |
| Binding | base, target, tree, cemSHA256, ocmSHA256, selectionSHA256, sourceSHA256, recipeSHA256, wasmSHA256, workerSHA256 |
| Enrollment | profile, nonce, audience, repositoryId, policySHA256, rootId, epoch, generation, issuedAt, expiresAt, selection, binding: Binding, checks: array of Check |
| Row | check: Check, status |
| Payload | enrollment: Enrollment, completedAt, cleanup, exitCode, rows: array of Row |
| Receipt | payload: Payload, signature |

Enrollment selection is `ALL_SELECTED`; checks are sorted unique nonempty, at most 256, and their
canonical SHA256 is selectionSHA256. Enrollment expires within 24 hours. Authenticated completed payload has
cleanup `COMPLETE`, exitCode `0`, and exactly one closed `PASS` or `FAIL` row for each frozen check in order. Signature verification authenticates the completed observation, including an independently failed assertion; closure still requires every selected row PASS. Missing/duplicate/unknown statuses or uncertain runtime cleanup remain unavailable.
The protected policy snapshot is deliberately not claimant wire input: it provides accepted/current/
nonfixture/nonrevoked state, root public key, audience, current epoch/generation/minimum-generation,
exact admitted checks and the protected nonce-to-exact-terminal-receipt-digest journal.

The initial aggregate driver is `wp3codec-canonical-v0`, profile `tcq/1-experimental`, immutable
source `tools/local-authority/driver.go`, unit `TestCanonicalOutput`, invocation
`wp3codec-wasip1-go1.27-v0`, subject `internal/wp3codec/codec.go`. Driver SHA256 covers the complete
file. Expected vectors cover canonical escaping, raw UTF-8 ordering, whitespace/LF refusal, forbidden
scalar, malformed UTF-8/surrogate, duplicate key and depth. Candidate stdout supplies answers only.
SourceSHA256 for the prototype is SHA256 of compact JSON plus LF with ordered keys Codec, Wrapper,
Module, each holding the exact SHA256 of codec.go, protected embedded wrapper and generated go.mod.
The eventual protected ingestion independently derives this manifest from enrolled Git objects.

Stop input is strict closed JSON at most 4096 bytes:
`{profile,event:"stop",enrollmentHandle:64-lowercase-hex,stopHookActive:boolean}`.
No caller root/target/host/frontier/permission fields exist. Protected Resolve derives all of them.
Native observer projections retain hashed task/turn/run/sourcePath, exact enum/status/timing and
entry kinds plus SHA256(text), never bodies or raw IDs. Only independently witnessed same-engine
supported `hook/started` and `hook/completed` delivery can qualify observation; fixtures stay UNQUALIFIED.

### Refusal and unavailable reasons

The `authority-event` command and the protected execution packages emit the kebab-case codes below
(decision 0100). Each row cites the first emitting site and states only the condition checked
there. Without `--native-output` the command writes the reason in a `corvint-authority-event/0`
result with authority `NONE`; with it, the refusal writes only the fixed unavailable
`systemMessage`.

| Code | First emitting site | At the cited site |
|---|---|---|
| `authority-event-input-unavailable` | `cmd/corvint/authority_event.go:32` | bounded reading of stdin within the 1600 ms context failed |
| `invalid-authority-event` | `cmd/corvint/authority_event.go:36` | `authorityevent.Parse` refused the input; `Handle` also sets it when re-parsing the marshalled event fails |
| `invalid-authority-event-arguments` | `cmd/corvint/authority_event.go:26` | the arguments are not exactly `--input -`, optionally followed by `--native-output` |
| `invalid-protected-execution` | `internal/localauthority/wire.go:13` | the text of `localauthority.ErrInvalid`, returned when a value fails WP3 canonical encoding and by the protected verification and `frontiernext` paths on a closed-schema refusal; `frontier-next` also writes it as `code` when its arguments are not exactly `--enrollment` and 64 lowercase hex digits |
| `protected-authority-unavailable` | `internal/authorityevent/event.go:145` | the initial `Reason` of every `Handle` result, left in place when no resolver is supplied, the context is done, resolution fails, the root is not current, the universe digest is invalid, or the state is neither `EMPTY` nor `OPEN`; `frontierDecision` also returns it for a state other than `EMPTY` or `OPEN` |

## Runtime boundary and frozen limits

Wazero v1.12.0, release commit `2ab480b55fa408d6b35df97fe32a60d08bd6e201`, is pinned in the separate
optional module with its published sums. Exact release `config.go` documents interpreter runtime,
8192 memory pages and WithCloseOnContextDone. No compilation cache or JIT is used. Go 1.27.0 compiles
only the admitted `codec.go` plus protected wrapper in a fresh offline module/cache, with cgo disabled,
no generate/scripts, no repository module settings and no assembly/compiler directives. Only the six
baseline stdlib imports are allowed. The compiler is TCB, never candidate native executable code.

One run at a time; input 64 KiB; source capsule 16 MiB; module 32 MiB; each guest stream 1 MiB;
module sections <=64, custom data <=1 MiB, table section <=64 KiB, linear memory <=512 MiB.
Build outer deadline 60 seconds; execution outer deadline 5 seconds; runtime compile 2 seconds,
instantiate/start 1 second, exported entry call 1.5 seconds, total runtime context 4.5 seconds.
A separate guardian owns the worker process group and monitors parent liveness, deadline and RSS.
Worker liveness pipe loss kills its complete group, including compiler descendants. Parent cancellation
closes its guardian pipe. Every exit kills and confirms disappearance of the owned group before success.
RSS is sampled at 40 ms, 768 MiB runtime / 2 GiB build, best effort with overshoot; monitor failure
fails execution. Same-UID/group disappearance evidence is feasibility, not arbitrary native containment.
The signer never shares the worker principal. Native key isolation and descendant birth-identity
qualification remain gates; no unsigned diagnostic can claim them completed.

WASI imports are frozen in runtime.go from the pinned wrapper's exact measured import table.
Absent/extra/foreign imports fail. The guest has only bounded memory stdin/stdout/stderr, default
fake clocks/randomness, no environment/args, preopens, network, plugins or host-native FD interface.
Private host lifecycle frames never pass through guest descriptors. Missing/invalid terminal,
nonce/module mismatch, trap, overflow, panic or uncertain cleanup fails.

For PLE-V0-009, each fresh target scan binds the repository root, every traversed parent
directory and the regular leaf to opened descriptors. A root-scoped OpenFile flag alone is not
a no-symlink witness: Go 1.27 can resolve the terminal symlink before the native open. Parent and
leaf Lstat identities must match their opened descriptors, and namespace identities, modes,
sizes, modification times and native change times are checked again after the raw read. Both
initial and final executable mode must match the Git tree. Darwin and Linux currently implement
the required native change-time witness; other platforms remain unavailable at this seam.
Leaf handles close after every joined read window on success, cancellation and failure. A scan-local cache retains at most 30 directory handles including the root, with at most one additional acquisition handle and eight leaf handles. Evicted directory edges retain bounded metadata witnesses; reopening uses retained bound directory handles and compares the original witnesses. The complete parent-first final namespace check includes evicted edges and paths deeper than the cache. Every owned cached handle closes on every return. These finite checks
also force a directory component when opening the repository root or a parent: Go 1.27's
final-name OpenRoot can otherwise block on a FIFO substituted between metadata and open.
Bounded subprocess regressions cover both root and child FIFO refusal. The checks
detect the tested observed substitutions and metadata changes; they do not claim an atomic
worktree snapshot or prove the absence of every concurrent restoration race. No latency or FULL
qualification follows from these correctness tests.

### Protected immutable view and final observed currentness

For PLE-V0-003/008/009, the optional consumer MUST use an independently self-hashed capsule published
as a restricted immutable Git view. Pinned Git alone does not verify every live nested-tree or diff
object link: the retained corruption matrix accepted such substitutions. Demanded BlobBytes now
self-hash their Git typed header/body, and `CEM-CB-023`/`CEM-CB-024` in
`docs/specs/cem-0.2-canonical-binding.md` now re-hash every tree consumed by default path lookup
and every old/new blob the canonical diff names. It is not repaired by a receipt or two worktree
scans.

The selected protected view preserves the existing Git canonical diff. Immutable CEM/OCM/universe
operations run in the view directory using its fixed GitDir, configuration and objects. Fresh live
HEAD/index/untracked/ignore checks retain the admitted actual GitDir/worktree, with internally fixed
GIT_OBJECT_DIRECTORY pointing to the audited view and GIT_ALTERNATE_OBJECT_DIRECTORIES empty. Caller
redirections are scrubbed. A missing view object MUST fail even if a correct live/alternate copy exists.
The exact released Git SHA-1/SHA-256 behavior is independently qualified and retained as supporting
component evidence; source regressions exercise the same boundary. Object formats must match. Raw
scan path/mode/OID expectations come only from protected target trees, never live object storage.

After the initial protected-view audit and object-format agreement, the trusted consumer may
share one request-local memo of successful immutable Resolve(full OID), LookupTreeEntry(full OID,
literal path), self-hashed BlobBytes and CanonicalDiff values. It carries no authority premise or
verdict and is not selectable by a caller path, boolean or wire field. Ordinary repositories and
live currentness readers remain uncached. Canonical verification and adapter eligibility retain
separate repository scopes, operation budgets and distinct-blob accounting; only primitive values
are shared. Revision expressions and other Git operations remain uncached. Successful tree-entry
absence retains the existing parser classification, while errors are never cached.

The memo retains at most 256 entries and 16 MiB of key/value payload with FIFO eviction to the
same protected Git path. Oversized values bypass retention without rejecting the request. Fixed
session identity strings and bounded map/FIFO metadata are additional overhead; the payload bound
is not an RSS or total-memory promise. Stored keys/string values own their backing storage and
byte values are copied on insertion and return. Each logical hit still consumes the requesting
reader's Git operation budget, honors cancellation/deadlines, and charges distinct blob bytes in
that reader. Blob validation and pre/post byte-budget precedence remain unchanged. CanonicalDiff
performs its original lazy object-format operation before memo access, preserving that operation's
unmapped errors. No verification result, parsed document, permission or currentness check is cached.

A memo reader must retain its captured root/admin/format identity without object-view redirection.
Release closes and clears the session, preventing both new readers and retained-reader reinsertion;
a new request starts empty after a new audit. Cached success never replaces the complete final
view, policy, runtime, cwd/configuration and raw-currentness checks below. This optimization grants
no native qualification or latency claim.

The consumer checks policy/runtime/actual cwd and fresh live HEAD/index/untracked state before
independent immutable Compute. It performs ONE complete raw tracked-file scan afterward, preserving
all path, byte, mode, metadata, cancellation and resource bounds. Enumeration counts at most 8192 leaf rows and 4 MiB of exact ls-tree row bytes, with the existing 512-byte relative path grammar. Tree validation visits each unique capsule tree once; memoized leaf counts avoid exponential expansion of empty DAGs. There is no additional depth limit. Parent metadata witnesses derive from admitted leaf-path bytes, capped at (4 MiB / 2) + 1 including the root; they do not share the 8192-leaf limit. Stored prefix strings share admitted leaf storage, while the descriptor cap remains independent. It checks fresh HEAD/index/untracked
state both around that scan, then rechecks protected policy/publication/view, actual cwd/Gitdir,
direct configuration identity/bytes/change time, object format and runtime before a verified result.
An unstaged tracked file dirty at entry but repaired before the final scan may accept; dirty final
bytes refuse. This owner-accepted temporal amendment is final observed currentness, not two-pass
parity, an atomic snapshot, or exclusion of every restoration race. Entry staged/untracked drift refuses.

Before the optional view opens, the Darwin consumer obtains its own RLIMIT_NOFILE and live kernel
FD count after the four repository-binding handles exist and requires 48 descriptors of headroom.
View traversal retains only its shallow bound directory stack (ceiling 32) and one data descriptor,
closing each data descriptor after its initial bounded hash/fstat. All view-walk handles close before
the raw scan. Complete final namespace/metadata revalidation includes every observed file/directory;
no partial enumeration or timestamp-only initial file validation is permitted. Native measurements
must distinguish actual baseline, owned directory/data peaks and any unobserved OS total peak.
No component timing or tool-process resource count qualifies the actual native consumer.

## Acceptance and rollback

Focused tests must execute the real package, a deterministic wrong-code mutant, forged verdict
output, malformed/forbidden imports, function/start loops, memory growth/exhaustion, output limits,
and supervisor INT/TERM/loss. A separate native principal campaign must attempt key/filesystem/
network/process access, driver replacement, build exhaustion, guardian failure and lifecycle cleanup.
Successful fixtures only establish their named observations, never root admission or FULL.
All unrun tests and unsupported authority features remain explicit; accepted-root.json and keys are
unset until independent operator review. Install preparation is reviewable without privileged mutation.
Revoke/disable accepted policy before removal; Stop becomes unavailable/FALLBACK. Remove only
manifest-owned files/accounts after cleanup; preserve immutable receipts and report leftovers.

## Traceability and known unknowns

`tools/local-authority/reader_alias_test.go` (`TestReaderAliasesRetainIdentityChecks`) covers
PLE-V0-011 reader alias validation, service singleton restrictions and conflicting identity refusal.
These focused checks do not establish protected native admission or qualification.

`tools/local-authority/*_test.go` owns executable feasibility evidence for PLE-V0-004/005.
`internal/localauthority` tests own bounded wire verification for PLE-V0-006/007.
New Frontier and host consumer tests own fixture-only PLE-V0-008/009 evidence. PLE-V0-002/003/010/011
native acceptance is NOT_PRODUCED. Pre-context timing and billed tokens are NOT_OBSERVED; the retained
query omitted three ranked results. Prechange dogfood refused no-diff/missing scope/citation inputs;
these failures remain in the private evidence log. No prior failure is relabelled as success.

`internal/cem/gitauth/authority_binding_test.go` covers terminal/parent symlinks, detached parent
substitution, executable-mode changes during reading, restored modification time, cancellation
and descriptor cleanup for the currentness portion of PLE-V0-009.

### Selection wrapper and refusal order

The additive OCM wrapper is exactly `{profile,legacyOCMSHA256,mode,edges}` where each edge is
`{obligationId,claimId,checkId,subject,function}`; mode is ALL_SELECTED. Binding OCMSHA256 hashes this wrapper,
legacyOCMSHA256 hashes the unchanged verified original OCM. Preserve every independently verified
legacy obligation. Duplicate/extra IDs are invalid; absent selected edges remain OPEN.
Validation order is canonical closed selection, profile/nonempty bound, CEM/OCM hashes, independent
legacy repository recomputation, protected current policy, exact enrollment, policy root/epoch/
generation/floor/audience/check equality, freshness, cleanup/exit/full closed PASS-or-FAIL rows, signature and exact
protected journal digest. All unknowns and existing CEM LRF witness requirements remain in force.
CWR conservatively requires byte-identical entire base/target intent, exact backtick function name,
a unique top-level function in a <=1 MiB target Go blob and material target-added-line intersection
with the canonical cited hunk. The admitted whole driver file is unchanged base-to-target and its
named unit uniquely resolves. No fixture or driver relation changes frozen tcq/0 or frontier/0.

The protected policy also freezes repositoryId and policySHA256. These are independently derived repository identity and accepted policy-descriptor digest, never caller Git-remote claims. Protected loading recomputes recipe, worker and toolchain identities from the actual immutable release; free-floating supplied hashes do not establish them.

The comparator TestCanonicalOutput is an explicitly invoked protected assertion unit, not a candidate `go test` invocation. Its executed PLE-V0-005 group drives all actual comparisons, preserving a base-stable legacy source claim anchor. Every distinct legacy claim ID requires its own matching selected edge; unsupported or absent claims remain OPEN.

The admitted Check.claimSelector is exactly `test:TestCanonicalOutput/case:ple-v0-canonical-vector`; its claimId edge binding cannot substitute another nearby test group.

Each selected claimId belongs to that obligation's independently verified legacy ClaimIDs. Its path,
exact selector and target blob match admitted DriverPath/ClaimSelector and the independently resolved
target driver blob. The complete file hash equals DriverSHA256 and the file is byte-identical at base.

Protected source ingestion uses `corvint-source-capsule/0`: closed canonical `{profile,objects}`;
each object is `{oid,data}` with base64 raw Git header+NUL+body bytes. Capsule JSON has its own 24 MiB closed canonical decoder; the view manifest has its own 1 MiB decoder. The 128 KiB receipt decoder is unchanged. Canonical zero epoch/generation remain valid. At most 1024 objects and
16 MiB decoded typed bytes are allowed, including the fixed self-hashed empty tree required by canonical diff attribute isolation. One enrolled SHA-1 or SHA-256 width applies to every object. Every Git object hash, declared size and commit/tree
path link is independently verified; no Git refs/configuration, pack helpers or repository command
runs inside the signer. Target commit/tree are independently enrolled. The package directory admits
exactly codec.go and regular *_test.go files (tests excluded); every other entry, including additional
production .go, assembly and syso, is unsupported. Driver whole-file bytes at base and target match the
admitted driver hash. Complete base/target commit/tree closure and every changed blob-bearing entry
are required, including deleted base symlinks; target raw currentness still refuses target symlinks and
submodules. Every CEM evidence base/existing-target path, OCM intentScope at base/target, target claim,
frozen driver base/target, target subject and target CEM sidecar must be present. Unchanged unused blob
bodies and unneeded parent history may be absent; an unforeseen demanded object fails without live
fallback. Exact required-union admission retains the 1024-object/16 MiB bounds. Absent required objects,
aliases, unsupported selected-source modes and changed source/recipe/artifact fail.

`execution-policy.json` is canonical root:wheel0644, independently pinned by root.policySHA256.
It freezes worker path/hash, Go path/hash, complete immutable Go-root file manifest digest, embedded
driver/recipe digest, released runtime module/version/sum and exact execution limits. The authority
recomputes these from the actual release before ingestion and immediately before signing. It must
run as _corvintauthority; the operator root broker only transports bounded opaque data to _corvintcheck
workers and authority phases. The signed protocol-complete terminal (including assertion FAIL) is written durably before an atomic
public directory rename; partial public success cannot become visible. Private nonce pending/built/
terminal records prevent reuse. Public/<enrollment-digest>/ contains enrollment.json, receipt.json,
cem.json, ocm.json, selection.json, target.json, terminal.json and evidence.json, authority-owned0444
in0755 dirs. evidence.json is a corvint-protected-git-reference/0 containing only enrollmentHandle and
manifestSHA256. Source bytes are NEVER copied into this public namespace.

The closed root wire version is corvint-protected-root/1, adding canonical decimal authorityGid and
readerUid. Reader UID is nonroot and distinct from the authority UID; the existing dedicated authority
GID may be reused only after independent operator preflight of the exact local group/member set,
absence of nested groups and other group-granted privileges. The human reader may have up to 16
case-insensitively distinct RecordName values, each 1–256 ASCII bytes matching
`[A-Za-z0-9_][A-Za-z0-9_.@-]*`, with the exact audited canonical name present once. Before
admission every name must resolve through both local and Search Directory Services to the same
exact audited UID and UUID. Invalid, missing, duplicate, excessive or conflicting aliases refuse;
dedicated service accounts retain singleton names. Existing forward/reverse identity uniqueness,
member inventory, local-only and other-privilege audits remain mandatory (PLE-V0-011).
Full local user inventories accept the same bounded ASCII record-name grammar for unrelated
service records (including dotted names); audited canonical names remain strict. Every inventory
ID remains canonical numeric text, duplicate audited UIDs still refuse, and all implicit primary-GID
members remain visible to the exact group-member check.
`TestReaderUserInventoryPreservesIdentityChecks` covers these inventory boundaries.
A closed root-owned reader audit and
INTENT/DIRECTORY_BOUND/MEMBERSHIP_VERIFIED/ACTIVE ledger govern the mutation, with explicit local
Directory Services and no-other-privilege attestations. Full membership is administrative admission
TCB, bound to root epoch/generation: no per-hook directory enumeration or automatic admin-drift
claim. Runtime checks actual UID/effective group credentials and view owner/GID/mode/no ACL. Fresh
app/engine credentials are required after admission. Group/member replacement requires revoke or
reader-policy epoch change and requalification. Immediate source-read withdrawal first removes
reader traversal with evidence-parent mode 0700, before Directory Services drift checks, because group
removal alone does not revoke existing process credentials. Remove only recorded added membership;
retirement archives retained evidence root:wheel before principal deletion.

The evidence parent /Library/CorvintAuthority/evidence is authorityUID:authorityGID, mode 0750. Each completed
<handle>/ directory and descendant directory is 0550; manifest and files are 0440, same owner/GID,
no ACL, symlink or file hardlink. Private key/enrollment/journal0700/0600 remains unchanged. A private
stage creates repo/.git/{HEAD,config,objects/xx/rest,refs/}; Go zlib encodes independently verified raw
objects without executing Git. Descriptor-relative creation sets exact ownership and file mode 0440 after writing regardless of umask; final files and directories are synced child-first before the exclusive descriptor-relative rename and parent sync. A corvint-protected-git-evidence/0 manifest binds root/epoch/generation,
handle, original capsule digest, base/target/tree/format/reader GID, and sorted unique file paths,
lengths and SHA-256 values plus object OID/type/decoded size. It excludes its own digest. The view
is audited completely before atomic rename; the public active pointer is published only afterward.
Exact namespace and every initial file digest are audited; missing/extra/config/alternate/permission
or identity changes refuse. Rollback leaves failed/incomplete views unpublished and protected; it
never exposes source publicly or restores a live-object fallback.
Root admission and minimum-generation floor are separate root-owned artifacts the signer never writes.

Every frozen enrolled mandatory check has at least one selected edge. Dropping an edge for any enrolled check contributes INTENT_TEST even if other edges cover every legacy claim; authentic FAIL remains OPEN/VERIFIED. The actual descriptive group is `PLE-V0-005 canonical vectors`, normalized by the frozen extractor to `case:ple-v0-canonical-vector`.

Native qualification records topology (`app-owned-stdio` or `shared-daemon`), bootSessionUUID,
fresh-audited appInstance PID/start seconds/start microseconds and the shared control socket where
applicable. Runtime checks kernel loaded code identities and a reciprocal live Desktop↔engine Unix
socket connection for shared-daemon topology. The protected qualification separately lists each
admitted surface and its evidence digest in qualifiedSurfaces. Event output for this topology is
`qualified-shared-runtime` with those qualifiedSurfaces and eventSurface `unattributed`; it MUST NOT
assert per-event Desktop origin or inherit a qualification for an unknown surface. Synthetic or absent
connection observations cannot fill this gate. This is an explicit proposed qualification scope,
not a reinterpretation of earlier per-Desktop observations.

The release directory ID commits original binary/toolchain/Git bytes and the unrendered adapter
source. The final separately hashed install manifest includes that ID and every prepared immutable
file, including rendered authority_hook.py. Accepted root pins exact prepared adapter/consumer/Git
images. Initial bootstrap copies reviewed bytes into independently protected root storage and verifies
the protected copy before execution; shell wrappers refuse caller-writable paths and ACLs. Root
imports enrollment files only in root-only staging, transfers ownership by FD and uses exclusive
rename relative to audited directory FDs. Retirement validates every account and group ID before
mutation, revokes first and archives only held top-level directories root0700; it never recursively
chowns authority-controlled paths. Retained receipts and uncertain leftovers stay explicit.

### Actual working-directory isolation

The protected consumer verifies release images and runtime identity before invoking the pinned Git image for cwd discovery. Nested working directories and aliases are allowed only when their physical nearest Git boundary and protected Git discovery agree on the exact canonical admitted root. Held directory identities and repeated discovery reject another repository or linked worktree, Git configuration redirection, deleted cwd, nested repository insertion, and cwd/root/Git directory replacement during evaluation. Native qualification must demonstrate that the host actually launches the consumer in the task worktree; an app-wide cwd produces unavailable authority with no payload fallback. This is repository isolation, not authenticated per-task attribution. `TestActualCwdRepositoryBinding`, `TestActualCwdRepositoryBindingRejectsReplacement`, and `TestActualCwdRepositoryBindingDistinguishesLinkedWorktree` provide focused regression evidence; native host activation remains NOT_PRODUCED.

### Temporary native qualification exercise (PLE-V0-012)

Execution admission precedes native qualification. When HostQualification is absent, the consumer may read only `/Library/CorvintAuthority/native-campaign.json` through the same root-owned, descriptor-audited reader (32 KiB maximum). A present completed HostQualification selects the normal path exclusively; a malformed completed document cannot fall back to a campaign. The campaign does not substitute for a signature, current execution root, monotonic floor, exact enrollment or immutable release.

The closed profile `corvint-native-qualification-campaign/0` requires status `NATIVE_QUALIFICATION_ONLY`, random 32-byte lowercase-hex campaignId, issuedAt/expiresAt (canonical UTC RFC3339 seconds; positive interval <=900 seconds), exact rootId/epoch/generation/repositoryId/repositoryRoot/policySHA256, SHA256 of current minimum-generation bytes, enrollmentHandle/target, consumer/adapter/Git images, allowStopRemediation, fixed scope `ALL_NATIVE_STOPS_IN_EXACT_REPOSITORY`, and candidate runtime pins. Runtime pins contain topology, boot UUID, appInstance AND engineInstance, app/engine image and kernel code-directory hashes, OS build, architecture, and a controlSocket only for shared-daemon topology. They contain no completed evidence digest or qualified surface list. Process-instance pid/started/startedUsec are minimal unsigned decimal JSON strings; pid and started are nonzero (pid >1), and startedUsec is in [0,999999]. The same integer encoding applies to existing completed-host appInstance; numeric JSON was already invalid WP3 wire. Engine restart or PID reuse with different birth time invalidates the campaign across calls, without adding an engineInstance field to normal HostQualification.

Both modes use the same protected resolver, final integrated immutable-object/currentness contract, actual-cwd binding, frontier computation, runtime checks and native OPEN/EMPTY/continuation decision rule. Campaign bytes, expiry and all current bindings are checked again before returning. Campaign results may state VERIFIED only for the protected computation; support remains FALLBACK, qualification UNQUALIFIED, event surface unattributed, and qualifiedHostSHA256/qualifiedSurfaces are absent. Native block/release text explicitly identifies an UNQUALIFIED exercise. OPEN blocks only if root.RemediationAllowed AND campaign.allowStopRemediation are true and stopHookActive is false. No new event, CLI or environment input can select a campaign.

The operator uses a dedicated campaign repository and explicitly acknowledges every native Stop in that repository during the short interval. Other tasks there must be quiesced/excluded or fall within the independently reviewed blast radius; no authenticated per-task scope is claimed. The native campaign must observe the host's real launch cwd rather than trusting event cwd. Installation/activation order is: final reviewed bundle, execution-only root and real signed enrollment, separately admitted candidate campaign, actual per-surface matrix evidence, campaign removal/expiry and verified hook restoration, independent evidence review, then separate completed HostQualification admission. Expiry disables exercise authority but does not itself restore configuration. No consumer or signer writes campaign admission or promotes qualification.

Focused acceptance witnesses are TestCampaignClosedSchemaAndBinding, TestCampaignMutationExpiryAndRemoval, TestCampaignCannotRepairCompletedQualification, TestCampaignEngineLifetimeAndDecimalWire, TestCampaignProcessWireWidthsAndMissingFields, and TestQualificationExerciseUsesNativeDecisionWithoutFull / RejectsAmbiguousOrUnverifiedResolution. Native exercise, activation, cleanup of installed hooks and completed qualification remain NOT_PRODUCED until actually observed. The inert template under `conformance/local-authority-v0/templates` contains unfilled independent inputs and grants no authority.

## Prospective direct native CLI profile (DCLI-V0)

The separately reviewed [direct native CLI profile](direct-native-cli-authority-v0.md) adds closed
root/2, campaign/1 and QLF/1 for one actual Codex CLI process. PLE-V0-009..012 and QLF-V0-001..010
retain their currentness, protected computation, attribution, native qualification and separate
operator admission obligations. The direct process pin replaces only the app/engine topology in
that new version; root/1, campaign/0, authority-event/0 and QLF/0 keep their existing wire meanings.
Legacy authority-event refuses root/2. A completed direct qualification cannot inherit Desktop
support or recover through a candidate if invalid. This prospective source profile does not accept
an execution root or weaken decision 0009.
