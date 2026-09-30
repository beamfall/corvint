# Optional operator updater — V1-0505

Owner intent: on 2026-09-29 the owner requested machine-wide Corvint/Tasks use, automatic retained
friction tickets, and latest compatible versions, then asked to implement the filed updater gap.
The optional technical contract is `docs/specs/operator-update-v0.md`; it remains proposed/
experimental rather than a release promotion. Core imports no updater/network code.

## Delivered behavior and evidence

Separate `cmd/corvint-update` supplies explicit network `check`/`apply` and offline `rollback` for
one selected Core/Tasks component. It independently discovers their official release channels,
verifies archive/internal checksums and host/build evidence, bounds requests/extraction/probes,
serializes mutation by destination directory, retains previous bytes and a prepared transaction,
rechecks the destination, and atomically activates. Same-byte current candidates are cleaned up;
foreign byte drift, unsafe paths, incompatible evidence and downgrade refuse. Core currently supplies
no source archive or release manifest: output retains NOT_PROVIDED. Tasks supplied source/manifest
and qualification evidence are preserved for activation; publisher identity remains NOT_VERIFIED.

Original task evidence is retained at `/private/tmp/corvint-updater-evidence` and in the worktree's
private Git Corvint directory. Prechange query and tracked installer impact actually ran at the
captured base; ranking omissions remain visible. Corvint's initial dogfood change ran before code;
expected missing citation/outcome rows were retained, not reported passed. Final affected selection
reports UNKNOWN, including broad dependency selections; the owner standing scoped-work preference
uses new updater tests/vet plus focused documentation checks and actual lifecycle, not the exhaustive
repository gate. Full gate/frozen retrieval evaluations are NOT_RUN: no Core retrieval, learning,
wire protocol or shared implementation changed, and no repository-wide qualification is claimed.

The relevant installer cancellation baseline passed. Updater focused tests and vet passed before
review and after its one repair cycle. Named UPD witnesses cover channel/offline read-only behavior,
external/internal checksum failures, archive links/aliases/bounds, platform/qualification checks,
destination locks, downgrade/equal-build divergence, partial-body cancellation, concurrent byte
drift, repeated no-op cleanup, prepared-receipt interruption and ordinary subprocess cleanup.
Spec/index consistency and required focused documentation gates passed; final bound receipts are
retained by the dogfood workflow rather than inferred from these prose claims.

Actual darwin/arm64 lifecycle used official Core v1.0.0-rc.1 build163 and Tasks
`tasks-dev-20260929.2` build202 in disposable directories. Tasks upgraded from retained build163 to
202, then offline rollback restored SHA-256
`7ffe657f299162361919cf20c3a48527050359eaa99290145c5d9bd736273e25` exactly. Repeated apply/rollback
also passed. Rebuilt repaired code passed another actual upgrade/rollback; Core same-byte no-op
passed with unchanged transaction-directory count. Linux/other native-platform lifecycle NOT_RUN;
these checks do not qualify complete Tasks execution or authenticate the publisher.

## Independent findings and decisions

One native Astra/high read-only reviewer passed the prebuild plan with no HIGH, asking for
canonical-destination serialization, revalidation, bounded discovery honesty and owned process
cleanup. Astra/medium built only the code/tests; root handled specs, integration and evidence.
Final review initially retained MED findings: unchanged applies accumulated downloads and could
report stale CURRENT; partial archive cancellation/internal tamper/interrupted receipt witnesses
were missing; the draft overclaimed Core source retention. One repair cycle fixed all code and
added those witnesses. Re-review scored all four ticket criteria PASS with no HIGH/MED. Two LOW
doc corrections (4096-member bound and exact new witness names) were applied/read back. No nested
delegation or duplicate unchanged review. Billed tokens/cache usage remain NOT_OBSERVED.

## Completion and rollback boundary

Native Tasks receipt audit is structurally CONSISTENT with projection AGREES; actor authentication,
runtime qualification and liveness retain NOT_OBSERVED. Admission first refused CAPACITY_EXHAUSTED,
then RESOURCE_COLLISION with another live task's shared spec-index/CEM scope. Capacity and others'
leases were preserved. Native completion remains open until admissible claim/gate/integration can
succeed; neither installed functionality nor CEM sealing implies ticket completion.

Machine adoption installs only the reviewed separate updater, preserves old Core/Tasks/backup bytes,
and updates the existing daily heartbeat to call it. Installation/heartbeat verification evidence
is retained locally. The source branch/draft PR is reviewable under standing publication authority;
merging still requires the owner. Removing the companion and reverting the heartbeat returns to
the manual update baseline. Destination transaction rollback verifies retained digests and leaves
repo/ticket/index state intact. Ordinary process-interruption recovery is demonstrated; power-loss
durability, malicious same-user writers and publisher compromise are not claimed.
