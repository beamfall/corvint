# Canonical repository envelope

Status: experimental native envelope `canonical-repository-bounded/1`. This is an
additive implementation envelope, not stable release promotion. The independently
reviewed `primary-clean-config-bounded/1` portable envelope remains preserved;
independently authored expanded-envelope qualification is still required.

The result has the same 21 fields and assurance axes. The repositoryEnvelope field
names the selected algorithm on every result, including failures before repository
admission. This identifier change is deliberate. Compare complete results against
explicit expectations; never remove or normalize the identifier for parity claims.

## Admission and authority

Ordinary generated SHA-1 and SHA-256 configurations, origin/user/branch entries and
actual reciprocal linked worktrees are admissible. Repository root and every named
ancestor must be ordinary directories without symlinks. Keep the root descriptor
and verify the named identities immediately before repository admission and before
success. Static links and named-root replacement fail closed. Concurrent hostile
same-user ABA replacement is outside this claim.

Validate the primary .git directory or bounded regular gitfile, reciprocal worktree
backpointer and common object directory before admitting immutable objects. Refuse
alternate object stores and symlinked object topology. Effective repository
info/attributes is unsupported; comment-only files are allowed. Root/metadata
failure precedes alternate denial. Object identifiers must be independent full
lowercase base/target values. Authenticate format and commit/tree/blob identity;
never substitute a working-tree read or caller patch. A shallow repository with all
required objects is admissible. Missing/corrupt objects cannot become an empty diff
or trigger lazy fetch.

Keep canonical diff flags, self-hashed inventory/replay, exact committed-sidecar
comparison and the complete two-pass opaque artifact validation. These checks do
not grant native Tasks authority, current applicability or execution authority.
Ordinary config is accepted as configuration, never as evidence authority.

## Process boundary

Use a trusted installed Git executable without a shell. Children receive only the
host PATH, SystemRoot, TMPDIR, TEMP, TMP and USERPROFILE when present, with LANG and
LC_ALL set to C and explicit fixed configuration/attribute/no-replace/no-graft/
no-prompt/no-lazy-fetch controls. Pin fsmonitor and untracked cache off, credential
helper empty and attributes to a null file; use canonical diff options disabling
external diff and textconv. Do not run filters, hooks or network helpers. This does
not promise every arbitrary include syntax is ignored: malformed/unreadable config
may produce typed unsupported results. Qualify actual sentinels and raw bytes.

Bounds: 1,024 Git operations; ten seconds per operation; thirty minutes cumulative
and context bound; 64 KiB stderr; patch 8 MiB; blob 64 MiB and distinct blobs
128 MiB; tree 4 MiB and depth 128. Close every object session on every path. On
Darwin/Linux retire ordinary process-group descendants on cancellation, timeout,
overflow and normal teardown. Escaped sessions and hostile same-UID interference
are outside the containment claim. Windows remains unsupported. Actual Linux
execution is NOT_RUN in this packet; Darwin does not qualify Linux.

## Results and reproduction

REPOSITORY-ENVELOPE.cells.json freezes full expected objects, recipe descriptions,
original fixture input identities and stage-specific refusals. Positive cases are
byte-for-byte original full positive results with the newly selected envelope ID.
Early repository refusals have no fabricated patch, sidecar, drift, artifact checks
or completed integrity axes. Effective attributes maps to repository /
unsupported-repository-envelope. Alternates maps to repository /
unsupported-object-alternates. Invalid metadata/root or object topology maps to
repository / repository-object-unavailable. Missing/corrupt required base commits
map to repository / git-read-failed. A missing target tree produces
repository-object-unavailable; a missing or corrupt changed target blob produces
git-diff-failed after retaining EXACT sidecar proof. Context cancellation maps to
verification-timeout at its observed stage. All are UNSUPPORTED with accept=null.

Materialize the original public fixture packet unchanged. For each ordinary case,
run fresh git init (with --object-format=sha256 for SHA-256), then copy only the
literal objects into the newly created object directory. Never copy/reorder the
fixture config. Retain Git version, exact argv, raw config and hashes. For origin
cases add inert origin/user/branch entries. Linked cases use actual git worktree
add --detach at the full target; relative cases also use --relative-paths. Isolate
malformed linked metadata controls from other worktree creation. Shallow controls
use actual local file-URL clones with depth three (complete) and one (missing).
All remaining case mutations are specified in the cells document. Freeze expected
results before evaluation, record original stdout bytes and exit codes, and compare
all 21 fields without dropping diagnostics or unknown axes. Do not claim separate
fixture roots are a same-directory experiment.

The native tests take CEM_STABLE_REPOSITORY_CASES pointing to the materialized
array of ID, Repository, Map, ExpectedBase, Target, ArtifactRoot, Expected, Exit and
optional Env. Paths are absolute local packet paths; Expected points to the frozen
full JSON object. Run TestStableRepositoryEnvelopeOrdinary, Cancelled,
DescendantCleanup and RootIdentity in internal/cem/verify. The public recipe is
language-neutral; portable implementations must not import native implementation.

Rollback selects the previous explicit envelope or disables expanded admission;
preserve all original maps, outputs and configs. Default producers and other
consumer families are not promoted by this envelope.

The native FreshConfigurations test embeds the ordinary/origin/linked recipe and
uses CEM_STABLE_FIXTURE_ROOT for the unchanged original materialized packet.

## Expanded admission amendment (S0E repair 6)

Status: experimental. This section incorporates the public amendment packet with
manifest SHA-256 21655c5d73a4b9b9af9f91e6dacbf702a8d2f95cdc9b40368a4cc240dedbd535
and its 56 full-result cases (FULL-RESULT-CASES.json, SHA-256
a59f490c33b02be55a576496be8ef670dd8b5e5765182f609c8a562b11e66ad1). Where this
section and an earlier section disagree, this section governs; the earlier text is
retained as history. The 56 complete expectations are bound by digest and indexed
in REPOSITORY-ENVELOPE.cells.json under `amendment`. The publicrepair6 packet adds
the ledger-boundary fixture target c82a33df6ae9eea3b210fa4002b3c5bcf66d4206 for
the 336/337 ledger boundary maps and their partial-drift case, uses bytewise
ascending evidence-ID drift order, and records attempted ledgers through the
refused-before-spawn reservation.

Metadata. The fixed admission order is: root; marker; administrative directory;
back-pointer and reciprocity; optional commondir and common directory; objects;
optional objects/info; optional objects/pack; ambient alternates;
objects/info/alternates; objects/info/http-alternates; common info/attributes;
distinct per-worktree info/attributes. Admission stops at the first refusal. No
Git configuration file is read before admission. A metadata pointer is at most
4,096 bytes of valid UTF-8 with no NUL or CR and exactly one nonempty line with at
most one final LF; nothing else is trimmed. commondir must be relative. Every named
ancestor of every admitted directory is an ordinary directory. Only an actual
not-found result establishes that an optional leaf is absent; any other host error
(for example EACCES, ELOOP, EIO) is repository / repository-object-unavailable. The
presence of objects/info/alternates or objects/info/http-alternates, even empty, is
repository / unsupported-object-alternates. The whole admitted boundary is
re-observed after the artifact checks and before the result is published.

Operations and time. A verification admits at most 1,024 logical Git operations;
operation 1,025 is refused before any spawn with repository /
unsupported-resource-limit, and that bound is evaluated before the outer deadline.
A replay of a logical operation reuses its reservation. One clock governs one
30-minute wall deadline for the whole call, a 10-second deadline per logical
operation capped at the wall deadline, and a single non-renewable 10-second
emergency allowance for retirement that starts at the first caller cancellation or
outer expiry. Earlier wording that described the 30 minutes as cumulative child
time is superseded. Caller cancellation and outer expiry are
verification-timeout at the stage in progress.

Causes. Every cause is latched and the result is arbitrated only after owned
cleanup has completed, in this order: unobserved cleanup; caller cancellation or
outer expiry; per-operation deadline; typed consumer failure; stdout overflow;
stderr overflow; launch failure; unsuccessful exit; success. The outer cause is
sampled once after the group has been signalled, immediately before the commitment
point; an outer expiry first observed after that point does not replace the
committed cause.

Process ownership. Every Stable child is the leader of its own process group and
is retired through one owner: observe the leader's exit without reaping it; send
the group one SIGKILL; poll signal 0 on the group until it stops succeeding; reap
the leader; then make exactly one signal-0 observation. Only ESRCH at that
observation is observed absence. Anything else, including EPERM, is HOLD: the
verification is repository / unsupported-process-containment, exit 2, with no
retry, no later spawn and no downgrade, and no real signal is sent after the reap.
A platform without this owner refuses before the first spawn with the same code.
