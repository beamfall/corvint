# Observed native facts — not approval of gaps

These facts describe frozen S0E runtime inputs. They are separate from the
proposed required rules. Source citations are held in the coordinator's evidence
packet; no native implementation is distributed here.

## U01–U02: markers and topology

Every regular metadata read checks its reported size before opening, requires a
regular non-symlink leaf, opens without following that leaf, checks opened identity
and size, reads at most bound+1 bytes and requires the byte count to equal the
pre-open size, then checks descriptor identity and size again. Bounds are inclusive:
4,096 bytes for `.git`, `gitdir` and `commondir`; 65,536 for attributes. No UTF-8
validation, modification-time comparison or post-read named-leaf identity check is
performed by this metadata helper.

For a regular `.git` marker, remove at most one final LF. Require the exact ASCII
prefix `gitdir: ` and a nonempty remainder with no LF or NUL. Do not trim other
whitespace or CR. An absolute remainder is cleaned directly; a relative remainder
is resolved against repository root and cleaned. The resulting named leaf must
Lstat as a directory; ancestors are not individually checked here.

The mandatory `gitdir` backpointer removes at most one final LF, then resolves
relative text against the administrative directory. It has no independent syntax
or UTF-8 validation. Empty text refuses; embedded LF/CR or invalid UTF-8 can be
filesystem name bytes; NUL fails the host path call. The referenced marker and its
parent must have the same filesystem identities as the requested root's `.git`
marker and root, respectively. This uses identity, not canonical string equality.
Hard-link equality alone is insufficient when the parent is another directory.

A `commondir` Lstat success causes a bounded regular read. All Lstat errors, not
only absence, currently cause fallback to the per-worktree Git directory. After
removing one LF, the text is concatenated after the administrative directory with
a separator and lexically cleaned: even a leading slash in this pointer does not
replace the prefix on the observed Unix implementation. Empty text selects that
same directory. A relative `../..` selects its common directory. The final named
common leaf must be a directory. There is no requirement that metadata lie in a
particular `.git/worktrees/name` shape, inside the worktree root or on one device;
actual reciprocal identity governs. Absolute commondir text is not a separately
qualified portable form.

## U03–U05: links, optional reads, attributes and alternates

The stable repository root and every named ancestor are ordinary directories,
retained by descriptor and device/inode/mode identity. They are checked before
metadata admission and again before successful return. Root replacement and mode
change refuse; directory mtime/size changes are not themselves replacement.

The Git admission code separately checks `.git` as directory or regular gitfile,
final administrative/common directory leaves, common `objects` (required), and
`objects/info` and `objects/pack` (optional) leaves. A symlink or wrong type at a
checked leaf refuses. It does not scan loose-object fanout directories, loose
object leaves, pack leaves, all administrative ancestors or unused links. Git may
follow paths below those unchecked boundaries, while returned commit/tree/blob
bytes are independently rehashed. This is not whole-object-store no-follow proof.
The metadata helper holds a file descriptor only for that read; it does not retain
all administrative descriptors until success.

Order is root/marker/reciprocity/common, then objects/info/pack topology, then
nonempty ambient GIT_ALTERNATE_OBJECT_DIRECTORIES, then existence of
objects/info/alternates, then attributes. Any successfully statted alternates
entry refuses, even empty, comment-only, symlink, directory or unreadable contents.
Its contents are never interpreted. An error from Lstat is currently ignored.
http-alternates is not inspected explicitly. Optional info/pack Lstat errors are
also ignored. These skipped-error facts are not fail-closed qualification.

Attributes are examined in common/info/attributes, then per-worktree
info/attributes when the directory strings differ. A missing/error Lstat is skipped.
A found non-regular leaf (including a symlink or directory) is repository-object-
unavailable. Over-bound, unreadable or changed reads use that same code. Each line
is split on LF and Unicode whitespace is trimmed. A trimmed empty line or one
whose first byte is `#` is inert; every other line refuses. Invalid UTF-8 is not
explicitly rejected; undecodable non-whitespace content is effective. Effective
attributes maps to unsupported-repository-envelope in stable admission.

## U06: configuration and format

There is no direct configuration-file byte limit or independently parsed extension
allowlist. Installed Git interprets repository includes, worktreeConfig and
config.worktree under the pinned command/environment. Malformed/unreadable
configuration surfaces at the first Git operation it breaks. Both independent
commit resolutions and sidecar lookups occur before explicit object-format query.
The later `rev-parse --show-object-format` output is bounded to 64 bytes and must be
exactly sha1 or sha256 after removal of one LF. Other output is unavailable.

The observed trusted executable is Git 2.54.0 (Apple Git-157), Darwin ARM64. No
minimum-version precheck exists. Unsupported command options fail through the
operation's typed exit boundary. The public controls and qualification, rather
than a guessed minimum version, must establish another executable tuple.

Every child gets only present PATH, SystemRoot, TMPDIR, TEMP, TMP, USERPROFILE plus
fixed LANG=C, LC_ALL=C, GIT_CONFIG_NOSYSTEM=1, GIT_CONFIG_GLOBAL=/dev/null,
GIT_CONFIG_SYSTEM=/dev/null, GIT_TERMINAL_PROMPT=0, GIT_OPTIONAL_LOCKS=0,
GIT_NO_LAZY_FETCH=1, GIT_NO_REPLACE_OBJECTS=1, GIT_GRAFT_FILE=/dev/null,
GIT_ASKPASS=, GIT_ATTR_NOSYSTEM=1 and GCM_INTERACTIVE=never. Host `GIT_*` variables
otherwise disappear. The common prefix disables optional locks, pins admitted
Git administrative directory and sets core.fsmonitor=false, core.untrackedCache=
false, core.attributesFile=/dev/null, credential.helper=, core.quotePath=false,
diff.suppressBlankEmpty=false, diff.orderFile=/dev/null, diff.noprefix=false,
diff.mnemonicPrefix=false, diff.renames=false, diff.algorithm=myers,
diff.wsErrorHighlight=none, diff.srcPrefix=a/, diff.dstPrefix=b/, diff.external=,
diff.ignoreSubmodules=none and advice.graftFileDeprecated=false. The CLI path does
not add the stronger host-pinned-binary-only hooksPath or allow-protocol override.
No claim that every configuration include is ignored is justified.

## U07–U08: memory, work and timing

Commit bodies in authenticated commit/tree streaming have no fixed total byte
ceiling: only the typed header (512 bytes including framing), first tree line
(OID width+6 bytes) and running hash are retained. Signed 64-bit canonical decimal
size is required; streaming is bounded by process/context time. A session resolve
optimizes only commits <=4 MiB; larger bodies fall back to a one-shot resolve,
then later tree lookup streams and rehashes the commit. This is not a 1 MiB cap.

A tree lookup's returned tree-record tail is at most 4,194,304 bytes including
batch headers and separators, across all root/intermediate trees in that lookup.
Canonical changed-tree comparison separately bounds each differing level batch
by that amount; it is not a verification-wide aggregate. Root comparison is depth
zero; children through depth128 can be examined, but requesting another level
from depth128 refuses. There is no independent 4,096 tree-entry/path count or
aggregate name-byte ceiling. Map/evidence/hunk/path and operation bounds still
apply. A different TreePaths API has an aggregate4MiB/depth0..127 rule; stable
verification does not call that API and does not inherit its tighter accounting.

A normal blob read allows 67,108,864 bytes and rehashes its typed bytes. Successful
reads charge each distinct OID once to134,217,728 bytes for this repository
verification, across sidecar, simulation, evidence, structural and drift reads.
Repeated OIDs still consume logical operations but not more blob-byte charge. At
an already exact-total boundary, even a new zero-byte OID is refused before read;
a previously charged OID remains readable. Canonical patch-provenance blob batches
have a separate134,217,728-byte stdout limit including framing, deduplicate OIDs
inside that batch and do not charge the later distinct-blob ledger. They do not
individually apply the normal64MiB blob-body cap. This distinction needs explicit
contract review rather than silently asserting one global blob ceiling.

One logical Git transaction reserves one of1,024 operations. The1,024th reservation
is admitted; the next refuses. Count is checked/decremented before wall time.
Thirty minutes is a monotonic deadline from budget construction, not a sum of child
CPU/runtime. Each operation gets min(10seconds, remaining wall time). Stable also
has a30minute outer context created just before root admission. Format output is64
bytes, commit-name resolution256, diff8,388,608, normal blob67,108,864, tree batches
4,194,304, proof-blob batch134,217,728 and stderr65,536 per child. Exact limits pass;
the first byte beyond fails. A streaming stdout consumer uses its own bounds.

One persistent object session may serve multiple requests. Each logical request
reserves once, regardless of reuse; session startup/close does not add a separate
charge. A failed optimization replays its one-shot transaction without a second
reservation. That replay presently receives the original per-operation span,
not the remaining span. Stable uses no request memo or mutable cache, and repeated
lookups remain charged. Prefix commit/tree streaming and per-level batches each
charge once, regardless of records within that batch.

Simultaneously ready exit/overflow/cancellation/timer channels currently have no
fixed cross-branch priority. On the exit branch, stored stream error precedes
stdout overflow, then stderr overflow, then unsuccessful exit. On overflow branch,
stored stream error precedes output-limit error. Cancellation/timeout branches
select their own codes. Stable often overrides the resulting code when its context
is already done. These facts are not a deterministic parity rule.

## U09–U12: result provenance and ownership limitations

Typed object identity/grammar/missing-tree failures become repository-object-
unavailable. Resolve failure becomes git-read-failed; normal blob Git exit failure
becomes repository-object-unavailable. Canonical derivation Git start/exit failure
becomes git-diff-failed and per-operation timeout becomes git-timeout. Output/count
failures become unsupported-resource-limit. The current stage-aware mapping and
completed fields are enumerated in FULL-RESULT-CASES and PROPOSED-RULES.

The native owner waits without reaping the group leader where its host primitive
succeeds, signals its group while that identity is reserved, then reaps. Failure
of that primitive falls back to wait/reap then a numeric group signal. Group signal
errors are ignored. A timeout may also race a concurrently completed waiter.
Session Close has no result channel and ignores the wait status. Stable builds its
return value before deferred session teardown and does not recheck cancellation
between successful artifact checks/root validation and return. Therefore no
unconditional "cleanup proved before ACCEPT" claim follows from current code.
The test-onlyR1 repair fixed its own PID fallback; it did not change these runtime
facts. Required runtime repairs are separate GateA blockers.

When final root identity validation fails after all normal checks, current output
is UNSUPPORTED/repository/repository-object-unavailable, accept=null, assurance=null
with completed patch digest, sidecar, drift, artifact checks and successful
integrity axes retained. Ancestor/root identity includes device/inode and mode;
it does not include mtime. Descriptor success alone never replaces named-root
validation. Hostile same-user concurrent ABA interference remains outside claim.

## U13–U14: runtime and history

The module build floor and structural runtime gate are separate. Go1.24.13 proposed
nonstructural success and structural refusal have separately frozen complete
expected objects. They are NOT_RUN in this amendment. Structural proof remains
restricted to exact actual go1.27.1; no fake version substitution is admitted.
Recursive INPUTS.json pins all four inherited public files. HISTORY-124.json pins
all124 historical public files from the existing portable source binding. Neither
packet may be silently rewritten to achieve expanded admission.
