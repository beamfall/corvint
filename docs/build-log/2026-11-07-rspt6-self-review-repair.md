# Runner qualification and file dependency self-review repair

Repair scope: V1-5108 (RSP-V0-033/034), V1-5106 (RSP-V0-007/022), and the
V1-5104/V1-5115 selector/script integration (RSP-V0-013/050/055).
Base: `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`.

The review found three gaps: file-class mutation names could certify unrelated
edits; write-only opens lost existence dependencies; and script records could
never exclude because both selection and collection readiness required coverage.

File populations now require core ownership and a closed operator/edit pairing:
one-byte XOR-1 replacements for data, executable and configuration byte operators;
strict prefix truncations; absent-to-present additions; present-to-absent deletions;
and deletion plus an identical-content addition for renames. Unknown classes or
core operators refuse using the existing diagnostic. Language-specific body,
declaration and server classes remain delegated to declared mutation pieces.
The per-class fixture and configuration-FULL control now generate file edits
through FilePopulation; no existing assertions or goldens changed.

Every supported ordinary open records a dependency regardless of access mode,
unless exclusive creation or prior tree-owned creation proves independence.
The existing closed open-flag allowlist remains governing; unknown forms retain
a dependency and incomplete evidence. The new write-only fixture writes and closes
without a subsequent read; deleting its target changes its outcome. Existing
unknown-open and tree-created-file regressions exercise the conservative boundary.

File-only capability is derived from the exact existing three-component
`absent`/`script-kind` marker with empty items. It adds no serialized field.
Selection and collection readiness share prerequisites: reads, listings, lookups,
execs and external. Every other marker retains native prerequisites. Agreeing
file-only unions preserve the marker; mixed capabilities stay incomplete.
The integrated test pins a real script adapter record, supplies synthetic admitted
qualification, excludes an unrelated edit, selects edits/deletion of a dependency,
and retains selection for every missing file component and an unknown marker.
Production qualification/admission remains absent; all selection stays shadow.

Focused whole-package results and exact commands are retained in the lane handback.
The first qualification run exposed the configuration-FULL fixture's mislabeled
language edit; replacing that input with a core configuration mutation preserved
its existing assertion and the package passed. Raw logs remain outside the repo.
Independent review and host-wide gates belong to the orchestrator. No live runner
qualification, statistical production evidence, ticket completion or promotion is
claimed. Rollback is a revert of this repair; record formats and goldens are unchanged.

Observed whole-package results: runnerqualification PASS (0.083 s), proctrace PASS
(3.708 s), runnerselection PASS (0.052 s), registry PASS (0.353 s). The testevidence suite
fails only TestRSPV0007WriteProofSymlink/create and /replace in unchanged DropRead.
A whole-package run with a Go overlay restoring every changed testevidence source
to pre-repair HEAD reproduces both failures (0.158 s). This pre-existing failure
is outside the three authorized fixes and remains for the host; assertions stay
intact. The new capability tests produced no failure. Spec requirements and line
citations pass; the line checker retains its 180 unresolved-basename notices.

The downstream qualification CLI suite also fails its existing replay test while
strictly decoding Request.Budget: time.Duration has no default representation.
Restoring the pre-repair operator implementation with a Go overlay reproduces
that failure. Decoder and wire changes are outside this repair and remain untouched.
The local verification outcome is failed, not a package-wide passing claim.

Retained log SHA-256 digests (files under the external rspt6-evidence directory):

| File | SHA-256 |
|---|---|
| repair-runnerqualification-final.log | `bdbcce8c61a1b3109d136f089bc276af1738ae8383eceed9b0034157897138c5` |
| repair-proctrace.log | `6d82c84e8c89b1110e7d76c8839e7e499e988e718856fb28c7eb692f3a17acb4` |
| repair-runnerselection.log | `bb8e7135d13713349a1a091c1cd8366816366ef70e28efbfc4cf71283d18dcf6` |
| repair-registry.log | `92dec86ca0013444184901343127e5eca97b14f082d1c26282ddb0de5ecb82ad` |
| repair-testevidence-final.log | `da326b601f796f23f33d9c40d44b493d4f3d13e881c7cda5992d0a303ed42ad3` |
| repair-testevidence-baseline.log | `928ac7877acef2553d2a8016b3a1d09681a8b953186a91faed2ed3f9d02ab4ca` |
| repair-qualification-cli.log | `a149eb2c831ce352bb887b57b46d6dd85b2cc7d720fc481ff13bdfbdd38d9862` |
| repair-qualification-cli-baseline-final.log | `62c751eef253bc6aa368a1f54a0c93b2462d333edad23b8a27948e37fb859d75` |
| repair-doc-checks.log | `f06ecff4660fd8f318ad46678647833546a2da2b047936aebf8678911c3dd118` |
