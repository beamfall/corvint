# Proposed required rules for expanded admission

Status: DRAFT, requiring independent critique and a new implementing Gate A.
Statements here are proposed requirements, not claims that the frozen native or
portable binary implements them. OBSERVED-NATIVE.md records differences. The
coordinator must freeze the admitted algorithm revision and exact result
expectations before build; old packets remain bound to their original revision.
The 21-key result schema and authority axes remain unchanged.

## 1. Boundary, metadata grammar and topology (U01–U05)

Use an absolute, lexically clean repository path. The root and all named ancestors
must be ordinary directories. Retain descriptor-backed device/inode/type/mode
identity through the verification, and validate the named chain before metadata
admission and before success. A changed directory mtime or entry count alone is
not a changed identity. Hostile same-user ABA replacement is not claimed safe.

Metadata payload bounds are inclusive: 4,096 bytes for the regular `.git` marker,
per-worktree `gitdir`, and optional `commondir`; 65,536 bytes for each attributes
file. Reject any found nonregular leaf, symlink leaf, unreadable file, over-bound
file, changed identity/size or short/growing read as
repository/repository-object-unavailable. Open without following the leaf and
compare both descriptor and named identity after the read. Only an actual
not-found result may mean an optional entry is absent; permission, I/O and loop
errors refuse. These last two requirements repair observed omissions.

Proposed interoperable metadata text grammar is deliberately narrower than
arbitrary native filesystem byte names: valid UTF-8; no NUL or CR; a single
nonempty line, optionally followed by one LF; no further LF. Do not trim spaces
or tabs from a path. `.git` requires exact prefix `gitdir: ` and a nonempty path.
Backpointer requires one nonempty path. `commondir`, if present, requires one
nonempty RELATIVE path. Leading slash is unsupported for commondir. An absolute
or relative `.git` target and backpointer are permitted. Resolve relative marker
targets against repository root; relative backpointers and commondir against the
per-worktree administrative directory. Lexically remove dot segments before
no-follow directory admission. Invalid grammar has
repository/repository-object-unavailable. CRLF, multiple final LFs, empty
commondir and non-UTF-8 names are proposed negative cells, not current-native
qualification claims.

A primary `.git` directory is its common directory. A regular marker selects a
per-worktree directory whose mandatory backpointer must identify the exact root
marker AND whose referenced parent identifies the exact requested root directory.
Compare filesystem identity, not merely string spelling. When commondir is absent,
use the per-worktree directory as common; a present relative pointer selects a
separately admitted common directory. No `.git/worktrees/<name>` spelling, root
containment or common-device restriction is added: external primary/common
locations and cross-device directories are permitted when every required chain
and reciprocal identity is admitted. A different parent with a hard-linked marker
is not reciprocal. Relative-path worktrees follow these same rules.

The mandatory no-follow set is: root ancestry; `.git` marker; administrative and
common directory ancestry; all actually inspected metadata path components;
common `objects`; and present direct `objects/info` and `objects/pack` directories.
Retain/revalidate administrative and common named identities through the operation.
This is a repair to the current leaf-only administrative checks. It is NOT a full
scan of every unused object directory or object file. Loose-object fanout/leaf and
pack file storage remain Git-owned; every returned commit/tree/blob is rehashed
and tree links are authenticated before use. No claim that Git never follows a
loose or packed storage symlink may be made. A future requirement to reject every
such link needs its own bounded object-store audit contract and Gate A.

Admission order is fixed: root; marker; administrative directory; mandatory
backpointer and reciprocity; optional commondir and common directory; objects;
optional objects/info; optional objects/pack; nonempty ambient alternate variable;
local objects/info/alternates; local objects/info/http-alternates; common
info/attributes; distinct per-worktree info/attributes. Stop at first refusal.
Any existing alternates or http-alternates entry refuses as
repository/unsupported-object-alternates regardless of empty/comment bytes,
regularity or readability. Failure to establish existence/absence refuses as
repository/repository-object-unavailable. Metadata/topology failure precedes
ambient alternates. No alternate store is read. Explicit http-alternates refusal
is a proposed addition, not an already measured native fact.

Attributes grammar is LF-separated lines. Trim these Unicode White_Space scalars
at both ends: U+0009–000D, U+0020, U+0085, U+00A0, U+1680, U+2000–200A, U+2028,
U+2029, U+202F, U+205F, U+3000. Empty lines or lines beginning with ASCII `#` after
that trim are inert; all others refuse as
repository/unsupported-repository-envelope. Invalid UTF-8 is effective content,
not a whitespace escape. Missing is allowed; unreadable or wrong-kind files are
repository-object-unavailable. No repository attribute content is interpreted.

## 2. Git/configuration authority (U06)

The trusted executable and actual version/OS/architecture are pinned in the
qualification manifest. Current observed tuple is Git 2.54.0 Apple Git-157 on
Darwin ARM64. This draft does not invent a lower version floor. Every additional
tuple must demonstrate the fixed flags, SHA-1/SHA-256 behavior, no-lazy-fetch and
hostile config sentinel controls; an unsupported flag remains an operational
failure, not a reason to retry with weaker flags.

Use the exact inherited WP1 canonical diff arguments, replacing only admitted
administrative directory, independently supplied full base/target OIDs and
format-correct empty-tree OID. SHA-1 empty tree is
4b825dc642cb6eb9a060e54bf8d69288fbee4904; SHA-256 is
6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321.
No caller patch or map path chooses another exclusion.

The process environment and config pins are exactly enumerated in
OBSERVED-NATIVE.md U06. Do not silently import extra host-specific
configuration pins as current CLI facts. Qualify that the selected operation set
and published controls prevent helper execution on the actual Git tuple; refuse
an unqualified tuple instead of retrying weaker arguments. Never execute
shells, external diff/textconv/filter/hooks/credential/network helpers as part of
verification. Ordinary inert remote/user/branch settings are admissible.

Git remains the parser of repository config/includes and supported extensions.
There is no independent config byte ceiling or mirrored Git extension grammar;
fixed operation deadlines/output limits apply. Worktree config may affect only
what the fixed flags permit. Includes are not assumed ignored. Unreadable or
malformed config is classified at the first attempted operation: resolution,
format or object read -> git-read-failed for child start/exit failure; canonical
transaction -> git-diff-failed. Do not pre-read config and invent a different
precedence. Format discovery must return sha1 or sha256 plus optional final LF
within 64 bytes; otherwise repository-object-unavailable. Authenticate all object
names and widths; a configuration claim never authenticates object contents.

## 3. Bytes, trees and logical operations (U07–U08)

All KiB/MiB below mean powers of 1,024. A bound admits equality and refuses the
first excess byte. Wire/map/patch/evidence limits remain inherited. Explicit
implementation ceilings must be named as such and return operational unsupported;
they cannot silently become protocol invalidity or equivalent interoperability.

* Normal blob body: 64MiB; successful distinct OIDs charge once to 128MiB across
  sidecar/simulation/evidence/structural/drift work. A previously charged OID may be
  read at the exact aggregate cap. Current behavior refuses a new OID at an
  already reached cap before reading, including a zero-size object; retain that
  boundary unless explicitly changed and retested.
* Canonical patch provenance: one separately bounded 128MiB batch INCLUDING
  headers and separators, with OIDs deduplicated for that batch. These temporary
  proof bytes do not also charge the normal-blob ledger. There is no asserted
  per-blob 64MiB ceiling inside that batch. This is an explicit separate budget,
  not a claim that all memory or all reads total 128MiB.
* Tree lookup: one 4MiB trailing batch INCLUDING headers/separators for root and
  every intermediate tree. Changed-tree walk: 4MiB per differing level batch,
  including base/target root batch, not one global aggregate. Root is depth 0;
  children at depth 128 may be inspected; requesting depth 129 refuses. No separate
  tree-entry count or aggregate name-byte cap is silently inherited from the old
  portable implementation. Old 4096-entry and aggregate-tree ceilings must stay
  explicitly narrower until changed by the reviewed stable-only implementation.
* Commit bodies: stream/self-hash; no new fixed 1MiB cap. Header <=512 bytes;
  canonical nonnegative signed 64-bit decimal length; commit first line exactly
  `tree `, full lowercase OID, LF (OID width+6 bytes). Retain only that first line
  and running identity state. Remaining commit bytes are opaque to this minimal
  proof. Commit data must finish inside process/context bounds.

Tree-record parsing requires complete framing; expected object kind/name/width;
exact self-hash; unique names; and an unsigned octal mode in 32-bit range. The
regular-file mode normalization and canonical patch rules remain inherited.
Do not infer a full source-inventory audit: unchanged subtree bodies and unused
storage leaves need not be enumerated. Selected paths still obey inherited wire
path rules. A malformed accessed tree or mismatched identity is unavailable.

The logical ledger is independent of process reuse. Charge once before each
commit-resolution transaction, format query, authenticated path-lookup batch,
canonical diff child transaction, root-pair authentication transaction,
changed-level batch, changed-blob proof batch, and normal blob-body request.
Within a batch, records do not add charges. Repeated calls still charge, including
repeated path/sidecar lookups and cached/session-served answers. Starting or
closing a session/keeper does not add a Git operation. Replaying an optimization
fallback uses the SAME reservation and remaining deadline. The native replay
reset is a required repair. The 1,024th reservation is allowed; 1,025th is
unsupported-resource-limit. An independently authored implementation must not
charge extra merely for choosing one-shot children, nor erase logical charges by
combining or memoizing operations. LOGICAL-LEDGERS.json freezes public operation-kind, phase, input-key and ordinal
traces for the literal positive packets and exact count boundaries before build.
A complete positive has 18 reservations without target sidecar and 21 with it; the
changed .corvint level still requires authentication even though its sidecar leaf
is excluded. Unchanged refs subtree is not read in the changed-tree walk.
The 337-evidence limit input would require 1026 logical calls to complete, but its
attempted trace ends at the 1025th reservation: call 1024 is admitted and call 1025
refuses before spawn, preserving 335 completed drift items and no later row. Cache
hits retain identical logical ordinals; process start/close/replay adds none.

One outer monotonic 30-minute operational deadline starts before repository admission.
Normal cleanup is within that deadline. Caller cancellation or outer expiry invokes
one separate emergency-retirement allowance of at most 10 seconds; it admits no new
Git work, replay or verification. It cannot be renewed by later events. Thus normal
verification plus emergency retirement is bounded by 30 minutes + 10 seconds, subject
to the stated trusted terminating-host-primitive assumption. Each logical transaction gets min(10 seconds, outer remaining span),
including any optimization and replay. It is a wall deadline, not summed runtime.
Output ceilings: format 64B; resolved name 256B; patch 8MiB; normal blob 64MiB; tree
batch 4MiB; proof-blob batch 128MiB; stderr 64KiB per child. Streaming commit consumers
own their declared framing/state bounds. Store overflow/consumer errors explicitly
while draining/discarding further output; an I/O copy error cannot mask them as a
child exit status.

After retaining ownership and collecting all final observed flags, proposed fixed
priority is: (1) cleanup/ownership proof failure; (2) outer cancellation/deadline;
(3) logical per-operation deadline; (4) independently observed typed stdout-parser/identity failure;
(5) stdout overflow, then stderr overflow; (6) launch failure; (7) unsuccessful Git
exit; (8) successful result. No branch race establishes precedence. A parser EOF/short-read caused solely by local clipping, forced pipe close or
termination is transport truncation, not independently malformed input; retain its
cause provenance and select the initiating overflow/timeout/cancellation. Complete
invalid syntax or a completed wrong hash observed before that truncation remains
an independent parser cause. Do not synthesize a parser error from discarded bytes.
A failure already returned by an earlier verifier stage is not retrospectively replaced by
an event that arrives later. When outer cancellation or expiry arrives during ordinary abnormal teardown after a
per-operation failure, that failure stands if the operation had already returned it to a
verifier stage, and otherwise arbitration at the active checkpoint applies rank 2 and
yields verification-timeout (this qualifies section 5), while failed or unobserved
teardown remains rank 1 in both branches. Tests must synchronize simultaneous causes before
final arbitration and pin complete results. The current native arbitration does
not satisfy this rule and cannot be accepted by normalizing outcomes.

## 4. Error taxonomy and progress (U09–U10)

Operational refusals have exit 2, outcome UNSUPPORTED, accept=null, assurance=null,
a scalar code and empty issueCodes. Preserve every completed evidence field and
axis; do not clear completed patch/sidecar/drift/artifact progress. Before a stage
completes, do not invent progress. Ordinary semantic rejects remain exit 1,
REJECT/accept=false/code=null with issueCodes; inherited wire/argument errors keep
their separately specified exit 2 REJECT behavior.

| Origin | Public operational code and stage |
| --- | --- |
| Required metadata/root/tree/hash/framing/identity unavailable | repository / repository-object-unavailable |
| Normal blob child nonzero (including missing or corrupt compressed storage), complete missing/type/hash/framing failure | repository / repository-object-unavailable |
| Normal blob launch failure; resolve, format or noncanonical path-lookup launch/child nonzero | repository / git-read-failed |
| Any canonical transaction suboperation launch/child nonzero | repository / git-diff-failed |
| Per-logical-operation deadline | repository / git-timeout |
| Count or stdout/stderr ceiling | repository / unsupported-resource-limit |
| Alternates entry or nonempty ambient alternate variable | repository / unsupported-object-alternates |
| Effective attributes | repository / unsupported-repository-envelope |
| Proven cleanup/owner failure | repository / unsupported-process-containment |
| Outer cancellation/deadline | active checkpoint / verification-timeout |

OPERATION-ERRORS.json is the exhaustive operation-kind/origin table for Git work;
its normal-blob child-exit exception is intentional. A corrupt compressed normal
evidence/sidecar blob that makes Git exit is repository-object-unavailable. A
corrupt compressed object encountered by noncanonical path lookup is git-read-failed
when observed as child nonzero, or repository-object-unavailable when complete
invalid framing/identity is independently observed. Canonical suboperations use
git-diff-failed for child launch/nonzero, including their proof batches. Precedence
uses cause provenance rather than a guessed underlying object defect. A missing target tree is unavailable; a missing changed blob during Git
diff is git-diff-failed. A missing evidence/sidecar blob read is unavailable even
when patch or sidecar progress already exists. Unsupported mode in parsed canonical
patch is an inherited semantic verification issue; do not reclassify every
unsupported-tree-mode as an operational repository failure. Base-side wrong
sidecar kind is excluded-path-not-file; target-side wrong kind/bytes is
excluded-artifact-mismatch, with existing sidecar progress retained.

Provisional target sidecar state: when the initial target-entry lookup finds a
regular file at the excluded target sidecar path, sidecar becomes MISMATCH at once.
Only a completed repeated target-sidecar lookup and equal-bytes body comparison
replaces it with EXACT. A failure or cancellation before that comparison completes
retains sidecar MISMATCH with patchSha256 null (cancel-during-target-sidecar-body,
target-sidecar-blob-*, repeated-target-sidecar-lookup-cancel). Cancellation during
the initial target-entry lookup itself leaves sidecar NOT_CHECKED. This sentence
states the rule already frozen by those expected results; it adds no outcome.

Cancellation checkpoints are explicit in FULL-RESULT-CASES: expected-base/before
first content request -> binding; target resolution -> repository; base sidecar
-> binding; initial target-entry lookup -> repository; repeated target-sidecar
lookup/body -> binding; canonical transaction -> repository; simulation/evidence/
mechanical proof -> verification; drift -> drift; artifact passes -> artifacts.
Metadata itself is synchronous: cancellation is sampled after its admission and
before expected-base work, preserving the original binding-stage pre-cancel cell.
Add cancellation checks before stage work and before committing its progress.
After final artifact/root checks, a final cancellation check uses verification
stage and retains completed fields. This terminal check is a required repair.
Malformed earlier input still takes its existing precedence over cancellation.
Host kernel/FS calls that never return are outside the trusted terminating-local-
filesystem assumption; no system-wide filesystem sandbox is claimed.

Artifact aggregate accounting, two complete passes and raw comparison remain
unchanged. If any artifact pass fails, artifactChecks is not partially published;
referenceIntegrity remains NOT_CHECKED for operational failure, or FAILED for the
existing digest/change semantic reject. Empty artifact sets do not waive the final
cancellation or repository-root/owner checks.

## 5. Process ownership and terminal admission (U11–U12)

No ACCEPT is committed until every owned Git session/operation has completed
proven teardown. Cleanup status must reach the verifier; a void close or ignored
wait/signal error cannot establish success. Group ownership must remain valid for
every permitted signal. Never signal a stored numeric PID/PGID after its leader
has been reaped, and never fall back to a post-reap sweep. If ownership cannot be
established, refuse unsupported-process-containment; do not guess from process
names, paths or scans. Signal 0 observations may establish absence, but do not
create kill authority. The native current owner/close paths require repair here.

A private same-binary keeper is an admissible PROPOSED portable approach, pending
its own independent Gate A. It must stay owned and unreaped until ordinary group
retirement is complete, including when Git exits normally but descendants retain
pipes. Control/status descriptors must not leak into Git. The controller accepts
only its fixed internal operation grammar; no public arbitrary-command API,
shell, network helper, daemon or installation is introduced. The portable author
must publish exact bounded transport framing in that implementing Gate A; this
public behavioral amendment does not invent its private serialization.

Required timing: abnormal cancellation/overflow/deadline has zero TERM grace:
retire the proven owned group immediately. Ordinary abnormal teardown is bounded
by 10 seconds within remaining outer time. If caller cancellation or outer expiry
occurs, one separately bounded 10-second emergency retirement interval begins,
anchored to that first terminal event. It cannot be restarted by signal errors,
secondary expiry or another cancellation. The keeper/leader must remain owned and
unreaped until the signal decision is complete. A normal persistent session first
closes stdin, allows at most min(10 seconds, remaining outer time) to finish, then
retires its still-owned group and reaps before success. Keeper implementation may
retire a normal completed operation immediately. Caller cancellation and outer expiry are distinct retained causes. Both prevent
new work. After proven emergency teardown both produce verification-timeout at
the active checkpoint (verification at final publication); failed or unobserved
teardown instead produces repository/unsupported-process-containment, which has
priority. Expiry during normal session close uses the same emergency interval.
An emergency interval that reaches its limit without observed completion enters
HOLD: retain the owner handle and cleanup responsibility, emit containment refusal,
and report cleanup NOT_OBSERVED. No renewed timer or post-reap signal is allowed.
Child result and owner teardown result are distinct. An unproved teardown overrides provisional success using
repository/unsupported-process-containment. If a trusted host primitive cannot
complete, return failure with cleanup NOT_OBSERVED and retain an owned-process
hold; do not report the task fully cleaned up. Escaped sessions, hostile same-UID
interference and unsupported OSes are outside the ordinary-group claim.

After all verification and artifact checks, revalidate the entire retained named
root/administrative boundary. Root replacement uses repository-object-unavailable
and retains the full already-completed result as frozen in the root-change case.
Complete process teardown, then perform the final cancellation/root validity
checkpoint before publishing ACCEPT. Fixing metadata or owner behavior must not
widen native Tasks/execution authority; all 13 unknown axes stay NOT_OBSERVED.

## 6. Runtime expectations and immutable routing (U13–U14)

Use actual runtime.Version-equivalent values. The Go 1.24.13 nonstructural and
structural cases are separate full expected objects and new map inputs where
needed; Go 1.27.1 cases are separate. No runtime/envelope-field replacement during
comparison is allowed. The source module can keep a Go 1.24 build floor while
structural proof remains restricted to exact go1.27.1. Actual compilation/tests
must qualify both tuples; this amendment only freezes pre-evaluation expectations.
The prior 0.3 Go 1.24 control remains bound to its original source and artifacts.

INPUTS.json pins the complete recursive public stable input set and all four
inherited files. Its repair4Routing section binds each routed role to exactly one
in-packet path and hash. Routing order, highest first:
(1) this amendment (PROPOSED-RULES.md, OPERATION-ERRORS.json, LOGICAL-LEDGERS.json,
FULL-RESULT-CASES.json) for explicitly amended envelope behavior; (2) the original
stable schemas RESULT.schema.json and stable.schema.json, which are in the packet
and byte-identical in both predecessor sets; (3) the original operation and
algorithm contracts OPERATION.json and ALGORITHMS.md, with the repository envelope
REPOSITORY-ENVELOPE.md and REPOSITORY-ENVELOPE.cells.json that they delegate to; (4) corrected inherited
algorithms inherited/ALGORITHMS.md; (5) canonical 0.1 algorithms
inherited/cem-0.1-algorithms.md. Rank 3 is inherited/s0e/OPERATION.json,
inherited/s0e/ALGORITHMS.md (root decision OQ-01 A), inherited/s0e/REPOSITORY-ENVELOPE.md
and inherited/s0e/REPOSITORY-ENVELOPE.cells.json (root decision OQ-07 SUPPLIED); inherited/stable-r1/ holds the
stable R1 versions as historical input only. The literal fixture pack is
inherited/fixtures/FIXTURES.pack.json; both positive maps are extracted from it under
inherited/fixtures/maps/ by the rule in INPUTS.json, after whole-pack validation
under inherited/s0e/PACKING.md. inherited/s0e/MIGRATION.json and
inherited/s0e/PACKING.md are informative and carry no rank. Rank 1 overrides every
rank-3 envelope statement it differs from. Both stable R1 envelope clauses remain in
force (root decision OQ-08 RETAINED): a result's repositoryEnvelope is always an exact
envelope ID, never the bare label native, and its resource and process constraints
are published with it, here by OPERATION-ERRORS.json and LOGICAL-LEDGERS.json.
Cases stay code-only; each implementing Gate A must carry a numeric timing-boundary
ledger for the outer, per-operation and emergency spans (root decision OQ-05 B). A dependency's old
prototype-status prose does not override stable admission. HISTORY-124.json binds
historical public bytes. Preserve every prior raw map/result/config; publish a new
manifest and explicit expected set for new runtime/contract behavior. Native and
portable same-directory qualification uses one pinned absolute repository and
artifact directory sequentially, with input/config hashes checked before/after.
Separate roots and projected outputs are never same-directory full-result proof.


## Repair1 binding and status

Algorithm amendment revision: cem-s0e-expanded-admission-r1. The result envelope
label remains canonical-repository-bounded/1; exact packet manifest hash and this
algorithm revision distinguish the proposed repair from historical measurements.
This does not relabel old results or claim their implementation meets the new rules.
All new full 21 expectations are derived before runtime evaluation. Public repair1
requires independent re-review and implementing Gate A; no native/portable runtime
qualification is claimed. Complete operation error table and reference logical
ledgers are normative for this revision. Historical 124 pins and observed native
facts remain byte-identical to the predecessor packet.


## Repair2 binding and status

Repair2 changes routing, recipe and prose text only. Every expectedExit and
expectedResult in FULL-RESULT-CASES.json is byte-identical to repair1, and the
algorithm revision label is unchanged. The linked-worktree test topology T-LINKED
(FULL-RESULT-CASES.json /admissionTopologies) is a NEW normative test choice
pending root acceptance (OQ-06). The packet stays blocked until root decides OQ-01
and supplies the reserved inherited bytes (OQ-02, OQ-03). The outer-expiry
interaction during ordinary abnormal teardown (OQ-04) and numeric deadline values
in cases (OQ-05) remain open; this revision adds no semantics for either.


## Repair3 binding and status

Repair3 applies the root decisions on OQ-01 to OQ-06. It adds the inherited bytes,
pins both positive maps, and adds the outer-expiry commitment sentence in section 3
with two synchronized full-result cases (OQ-04 C). Every pre-existing expectedExit
and expectedResult is byte-identical to repair2, and the algorithm revision label is
unchanged. T-LINKED is accepted (OQ-06 A). The public author re-verified the root
additivity claim: confirmed for ALGORITHMS.md, not for OPERATION.json (OQ-08). The
packet stays blocked on OQ-07 until the referenced repository-envelope files are
supplied or retired.


## Repair4 binding and status

Repair4 applies root decisions OQ-07 SUPPLIED and OQ-08 RETAINED. It adds the four
pinned S0E envelope files under inherited/s0e/, routes the two envelope files at
rank 3 and the other two as informative, and restates the retained stable R1
envelope clauses in section 6. A check of all 29 envelope cells against the rank 1
files found no conflict that changes an expected result (INPUTS.json
/repair4Routing/envelopeConflictCheck). The fixture pack passes PACKING.md
whole-pack validation, and the map extraction rule now requires it. Every
expectedExit and expectedResult is byte-identical to repair3, and the algorithm
revision label is unchanged. All open questions are decided.


## Repair6 rules

Repair6 is a rank 1 amendment for the ledger-boundary repair only. It overrides
repair5's inconsistent boundary prose and LOGICAL-LEDGERS rows where they differ;
all other routing and inherited text remains in force.

Drift processing order is the inherited rule: public inherited/ALGORITHMS.md says,
"Retain all evidence drift records in ascending evidence ID order." Repair6 reads
that as bytewise ascending order on the complete evidence ID string. Drift lookups
are processed in that order independent of map input order, and retained drift
records are emitted in that same order. The boundary ledgers and the partial drift
cases are bound to this order.

LOGICAL-LEDGERS records the attempted trace, not a theoretical required trace after
refusal. For boundary-337, the refused 1025th reservation is the final row and is
marked `refused-before-spawn`; there are no rows after it. That refused reservation
is for sorted drift item 336. Sorted drift item 337 is unattempted and has no ledger
row. The 335 completed drift records are retained; artifacts do not start.

The fixed non-evidence prefix for the 12-hunk two-file sidecarless ledger target is
15 logical calls:

| Count | Operation | Object | Why required |
| --- | --- | --- | --- |
| 1 | resolve-commit | expected base `bff36ef7f94a44f0b43dd71ba7125e0185f01fe1` | independent full base authentication; section 3 charges each commit-resolution transaction |
| 2 | resolve-commit | target `c82a33df6ae9eea3b210fa4002b3c5bcf66d4206` | independent full target authentication; section 3 charges each commit-resolution transaction |
| 3 | authenticated-path-lookup | base `.corvint/change.cem.json` | base sidecar state must be checked; section 3 charges each authenticated path-lookup batch |
| 4 | authenticated-path-lookup | target `.corvint/change.cem.json` | initial target sidecar state must be checked; section 4 sidecar progress rule |
| 5 | format-query | repository object format | canonical diff must use the repository-format empty tree; section 3 charges format query |
| 6 | canonical-diff-child | base to target canonical patch | inherited canonical authority derives the whole-repository patch; section 3 charges canonical diff child transaction |
| 7 | canonical-root-pair | base/target root pair | canonical patch proof authenticates root pair; section 3 charges root-pair authentication transaction |
| 8 | canonical-proof-blobs | changed blobs in the canonical patch | canonical patch provenance proof; section 3 charges changed-blob proof batch |
| 9 | authenticated-path-lookup | base `app.txt` | hunk base simulation needs immutable base file bytes; section 3 charges authenticated path lookup |
| 10 | normal-blob | base `app.txt` blob | hunk base simulation reads the normal blob body; section 3 charges normal blob-body request |
| 11 | authenticated-path-lookup | base `sort.go` | hunk base simulation needs immutable base file bytes; section 3 charges authenticated path lookup |
| 12 | normal-blob | base `sort.go` blob | hunk base simulation reads the normal blob body; section 3 charges normal blob-body request |
| 13 | authenticated-path-lookup | base `sort.go` for mechanical proof | formatter/import proof reuses the immutable base file image but repeated calls still charge |
| 14 | normal-blob | base `sort.go` blob for mechanical proof | repeated blob-body request still charges under section 3 |
| 15 | resolve-commit | drift target `c82a33df6ae9eea3b210fa4002b3c5bcf66d4206` | drift stage resolves the target before item lookups; section 3 charges commit resolution |

No sixteenth fixed-prefix call is required for this sidecarless target. The rejected
candidate is a canonical changed-level batch for `.corvint` or a per-file target
inventory lookup. Section 3 names changed-level batches as chargeable when they
are performed, but the same paragraph freezes a complete sidecarless positive at
18 reservations and says the changed `.corvint` level is required for the target
sidecar case. Because this target has no target sidecar, repair6 keeps the prefix
at 15 and keeps the 336-evidence map accepted. The arithmetic is:
`15 + 2 * 336 + 336 = 1023` accepted reservations; boundary-337 attempts
`15 + 2 * 337 + 335 + 1 = 1025` reservations, with the last refused before spawn.
