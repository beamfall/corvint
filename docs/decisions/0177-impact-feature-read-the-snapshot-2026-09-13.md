# Decision 0177 — standalone `impact` and `feature` read the snapshot

Date: 2026-09-13. Status: accepted, delegated coordinator call. Authority: repository owner
delegation to make owner calls and record them (orchestration thread, 2026-09-12).

`docs/specs/index-snapshot-v0.md` proposed `IDX-SNAP-V0-019` on 2026-09-13 (landed in commit
d8c5ca87): standalone path `impact` (no `--base`, no `--working-tree-untracked`) and `feature`
read the committed tree's snapshot on the terms `IDX-SNAP-V0-008` gives the query verbs, instead
of unconditionally building. The clause was left `proposed` pending an owner call on acceptance.

The call: accept `IDX-SNAP-V0-019`. `TestImpactAndFeatureReadTheSnapshotWithoutChangingAByte`
(`cmd/corvint/index_snapshot_test.go`) proves the byte-identical answer between a cold build and
a warm snapshot hit, and `TestPackQueryAndEventVerbsRereadARefusedBodyWithoutChangingAByte`
(`cmd/corvint/pack_snapshot_test.go`) covers the refused-pack-body reread path the same read
shares with `query`, `user-prompt`, and `file-change`. Both were re-run on this branch before
accepting (`go test -count=1 -timeout 30m ./cmd/corvint -run
'TestImpactAndFeatureReadTheSnapshotWithoutChangingAByte|TestPackQueryAndEventVerbsReread'`,
PASS, 2026-09-13) and pass. The measured win is a 7-11x faster standalone path `impact`, with no
change to stdout, stderr, exit code, or written state on any hit, miss, or refusal path.

Consequences: `docs/specs/index-snapshot-v0.md` marks `IDX-SNAP-V0-019` accepted (its heading,
requirement bullet, non-goals cross-reference, and traceability row all now cite decision 0177
instead of "proposed"); `docs/specs/REQUIREMENTS.tsv` regenerated to match.

Rollback: revert commit d8c5ca87's `impact`/`feature` dispatch in `cmd/corvint/main.go`, which
restores the unconditional build; the range (`--base`) and working-tree
(`--working-tree-untracked`) profiles are unaffected, since they already build unconditionally.
