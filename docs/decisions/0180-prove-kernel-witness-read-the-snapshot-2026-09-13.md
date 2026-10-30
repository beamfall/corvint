# Decision 0180 — `prove` impact and change modes, `kernel` and `witness` read the snapshot

Date: 2026-09-13. Status: accepted, delegated coordinator call. Authority: repository owner
delegation to make owner calls and record them (orchestration thread, 2026-09-12).

`docs/specs/index-snapshot-v0.md` proposes `IDX-SNAP-V0-020` in the same change: `prove`'s impact
and change modes, `kernel` (render and verify) and `witness` read the committed tree's snapshot on
the terms `IDX-SNAP-V0-008` gives the query verbs, and build on any miss exactly as before. It
continues the AT-04 work decision 0177 started for standalone path `impact` and `feature`.

The call: accept `IDX-SNAP-V0-020`. Each candidate was settled before switching. The index these
verbs read is a function of the committed tree plus the worktree status, which a hit rehydrates
(`IDX-SNAP-V0-002`, `IDX-SNAP-V0-004`), and a hit reports the live `HEAD`, so the answer is the
build's. Change mode and `witness` reach `RangeImpact`, which parses `Source.Data`; a deferred pack
body leaves that nil, so those verbs and `kernel` take only the whole-file read, while impact mode
shares standalone `impact`'s deferred read and refusal reread. A stale, absent, or corrupt snapshot
is a miss that calls `contextindex.Build` unchanged. `prove --checkpoint` stays excluded
(`FPK-V0-024`). No candidate was rejected.

Evidence: `TestProveKernelAndWitnessReadTheSnapshotWithoutChangingAByte`
(`cmd/corvint/index_snapshot_test.go`) compares stdout, stderr and exit across an absent, a stale
and a current snapshot for all six invocations, and
`TestPackQueryAndEventVerbsRereadARefusedBodyWithoutChangingAByte` now also covers a refused body
under `prove PATH`; both pass (`go test -count=1 -timeout 30m ./cmd/corvint`, 2026-09-13). On this
repository the same four verbs gave byte-identical output across the pre-change binary and the
changed binary over absent, stale and current snapshots, with the medians recorded in the clause
(`kernel` 1.02 to 0.15 s, `prove PATH` 1.27 to 0.39 s; `prove --base` and `witness --base` keep
about 1 s of range and affected-graph work that no snapshot removes).

Consequences: `docs/specs/index-snapshot-v0.md` adds `IDX-SNAP-V0-020` and cross-references it in
the non-goals, failure modes, evidence and traceability; `FPK-V0-008` and `AGW-V0-001` wording now
names the snapshot read; `docs/specs/REQUIREMENTS.tsv` is regenerated.

Rollback: restore `contextindex.Build` in `provePacket`, `runKernel` and `runWitness`
(`cmd/corvint/prove.go`, `kernel.go`, `witness.go`) and delete `snapshotOrBuild`; no stored state
changes, so no migration.
