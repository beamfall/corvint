<!-- SPDX-License-Identifier: Apache-2.0 -->
# Experimental authoring step records

These records describe a quiescent local observation. They confer no execution authority,
authentication, confinement guarantee or proof about transient/reverted writes. Intent and
implementation remain proposed/experimental under ASS-V0. The host owns frozen declarations,
input/state files, the Git executable and administrative object storage outside author access.

`schema.json` is Draft 2020-12 with closed objects. The observer additionally rejects duplicate
keys, case-folded field aliases, malformed Unicode, missing fields, negative integers, unsafe
paths, invalid mode/kind pairs and inconsistent digests. Schema validation alone is insufficient.
Scope paths are literal: an exact path or trailing slash prefix including its directory anchor.
No glob, backslash, control, `.git` component, traversal or absolute scope is admitted. Checkout
roots and executable paths are canonical absolute paths with no symlink ancestry. Roots are
non-overlapping and checkout IDs unique. Arrays cannot be null except the host's environment.
Read-only checkouts, scopes and key/class observations are sorted for normalized bindings;
repeat entries refuse. Environment values never enter a record or the observer API.

SHA-256 bindings use the existing Corvint strict JSON canonical encoding: lexicographic object
keys, preserved array order and canonical string/integer encoding, with no surrounding whitespace.
The declaration and host digests cover their normalized entire records. A state or receipt's own
`sha256` member is set to the empty string before hashing. Content/index/admin digests cover the
respective entry arrays or index entry; file digests hash raw bytes and symlinks hash raw link text.
Identity digests cover native device/inode/uid/gid. Modes use Go's portable FileMode integer layout.
A directory has size zero and SHA-256 of empty bytes. ABSENT entries occur only in administrative
inventories and have zero mode/size and empty-byte content/identity hashes. Path `.` binds the worktree root; administrative paths `git` and `common` bind their
respective directory roots. Complete pre-states require all three roots.

A complete state binds HEAD commit/tree, raw index, all worktree entries (including ignored,
untracked and empty directories) and admitted administrative entries. SHA-1 and SHA-256 Git
repositories with index v2 and the optional TREE extension are supported. Config includes,
unknown extensions, sparse/split indexes, shallow/partial/alternate/reftable/submodule storage,
unsafe entries and bounds refuse. Objects, logs, other-worktree metadata and private ledgers
are host-protected, not inventoried. ACLs, xattrs and timestamps are not observed. Only Darwin
and Linux descriptor-relative observation is supported; OS qualification remains separate.
Bounds per pass: eight checkouts, 20,000 entries, depth 64, 32 MiB per regular file, 128 MiB total.
Input/output records are at most 32 MiB. Bounds/unsafe/race failures never yield PASS.

`snapshot --declaration FILE --host FILE` emits a complete state with exit 0, or an incomplete
state/fixed error with exit 2. `verify` additionally requires `--before FILE`: a host-protected
complete before-state. It validates all host inputs against the complete before-state authority roots and current
discoverable roots, then recaptures the actual post-state. Unavailable administrative discovery
does not prevent complete raw worktree findings or admit Git execution. It never accepts author-claimed post
hashes. Guarded scope takes precedence over writes. Existing directory anchors cannot be replaced
or have their modes changed. Every complete worktree component retains its digest/findings even
when administrative observation becomes unsupported: whole `post_state_sha256` is null and exit 2.
An observed violation with complete observations yields FAIL/exit 1. A complete policy match yields
PASS/exit 0 with explicit unknowns. Snapshot digests are evidence bindings, not signatures.

`env-check --declaration FILE --host FILE` uses host-supplied key/classes only. NONE and READ_ONLY
must match a declared key/class. OUTWARD_WRITE and UNKNOWN always fail. Null observation yields
NOT_OBSERVED/exit 2; an explicitly observed empty array can pass. Verification retains this separate
environment verdict; unknown environment observation never proves secret isolation.

`vectors/write-cases.json` is the executable violation matrix consumed by
`internal/stepverify/TestStepWrites`. That native test supplies concrete before/after filesystem
fixtures for every operation, initial dirt, determinism and no content leakage. Additional native
fixtures cover malformed bindings, partial post-state, unsafe entries, read-only roots and traps.
No vector is a host enforcement or external interoperability qualification claim.
