# Decision 0074 — Stable analyzer identity for experimental packs

Date: 2026-09-05. Status: accepted (delegated call; bounded opt-in implementation).
Authority: Russell Lewis's wave-1 instruction. Owner-authored coordinating decision.

## Measurement and disposition

L2's registered A/B executables have different digests and identical analyzers.
After A primes an opt-in pack, B reports a fresh pack, returns byte-identical
context packets, and leaves snapshot files unchanged. C changes the analyzer
schema and misses. A changed token extraction is covered by the schema audit,
which fails when its reviewed input digest is stale. The falsifier passes.
Original registration, result and audit witnesses are retained in
`benchmarks/results/nextgen-L2-engine-2026-09-05/`.

IDX-SNAP-V0-017 keys experimental packs by a reviewed schema constant plus the
Go toolchain version. The source digest is a maintenance guard, not a runtime
key. Review any production analyzer change and update the schema/audit together;
the final merged analyzer receives a new schema identity. This is a correctness
and invalidation result, not a latency measurement against section 3. It does
not claim that every snapshot format survives executable rebuilds: default gob
and sectioned snapshots retain executable identity, and B's default gob misses.

Independent review found that the new pack stem escaped legacy gob retention.
The repair independently caps packs at eight across schema identities, reserving
the current pack even under an older timestamp. Twelve actual committed trees,
current-pack reload, old-clock and non-pack sentinel witnesses pass under verified
nice 15. The original A/B/C binaries remain frozen; the repair has its own evidence.

## Scope and rollback

Keep the pack format opt-in under IDX-SNAP-V0-015's existing experimental status.
The engine-key contract and its bounds are accepted only within that scope.
No default latency, pack-format promotion or global-superiority claim follows.
Unset `CORVINT_SNAPSHOT_FORMAT=pack` to return to the existing snapshot path;
derived packs can be rebuilt without affecting repository evidence or authority.
