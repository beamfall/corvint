# V1-0595 U2: stable repository envelope qualified over real checkouts

Date: 2026-10-20. Ticket: V1-0595 (unit U2). Base: origin/main 7d78a56b0.
Requirements: `CEM-V1-360..367` in `docs/specs/cem-stable-v1.md`.

## Change

This change adds tests only. No product code, packet byte or wire byte changed.

- `internal/cem/verify/stable_envelope_cells_unix_test.go` adds two native tests:
  - `TestStableCanonicalEnvelopeCellsNative` runs the 29 public cells of
    `REPOSITORY-ENVELOPE.cells.json` through `verify.Stable`. Each cell is built with real
    `git init` and `git init --object-format=sha256`, with `git worktree add` (absolute and
    `--relative-paths`) where the cell needs it. Every cell is compared on all 23 keys.
  - `TestStableEnvelopeAdversarialShapes` runs 11 shapes that the cells do not name.
- `interop/cem01-go/stable_envelope_adversarial_test.go` is the portable twin. It pins the same 11
  shapes and codes through the `verify-stable-canonical` CLI.

## Observed (Darwin arm64, Go 1.27.3, Apple Git 2.54.0)

- Natively, 27 of the 29 cells are exactly equal. These include ordinary, origin, linked and
  relative checkouts in both object formats. In the hostile cell, no sentinel helper ran.
- `missing-target-blob` and `corrupt-target-blob` diverge, and only in `code` and `verdictSha256`.
  The native verifier answers `repository-object-unavailable`, while the packet and the portable
  consumer answer `git-diff-failed`. This is retained as `CEM-V1-367`.
- The 11 adversarial shapes get identical answers natively and portably:
  - An outside `gitdir` refuses with `repository-object-unavailable`. The three variants are no
    back-pointer, another repository's real worktree admin, and a relative escape.
  - A symlinked `commondir` refuses with `repository-object-unavailable`. The three variants are a
    self-loop, a link to the real common directory, and a `commondir` file that is itself a link.
  - Three configurations refuse with `git-read-failed`: an unknown extension at version 1, a SHA-1
    store claiming sha256, and a SHA-256 store with the extension dropped.
  - Two configurations accept and equal a base cell's full result:
    - `[Extensions]` placed first with mixed-case `[CoRe]` equals `ordinary-sha256`;
    - an unknown extension at version 0 equals `ordinary-sha1`, because Git ignores it.
- The portable `TestStableCanonicalEnvelopeCells` still passes all 29 cells.

## Decision: retain the blob divergence, do not resolve it in this lane

Two accepted contracts conflict on a changed blob that is missing or corrupt:

- `protocol/cem-1.0/stable/repository-envelope-packet/PROPOSED-RULES.md` section 4 says the code
  follows cause provenance, so a blob missing during Git diff is `git-diff-failed`.
- `internal/cem/gitauth` (V1-0349, CEM-CB-019, CCF-V1-004) walks the change set after a failed
  diff and reports `repository-object-unavailable`. This keeps partial clones fail-closed without
  a fetch.

I tried a stable-only switch to cause provenance, using a diff-cause flag threaded through gitauth
and the stable verifier. It made `cmd/corvint`
`TestMapCoreVerbsRefuseAPromisorObjectWithoutFetching` fail: `cem status` and `cem verify` then
answered `git-diff-failed` for a `blob:none` clone. I reverted the change.

Choosing which contract yields belongs to the owner. Until that decision, the divergence is pinned
explicitly so that neither side can drift silently. Both answers are UNSUPPORTED at stage
`repository` with exit 2, so the divergence fails closed.

## Limits

- Darwin only was observed. The native test is built for darwin and linux, and Linux is NOT_RUN
  locally.
- The sealed `protocol/cem-1.0/stable/REPOSITORY-ENVELOPE.md` still names
  `CEM_STABLE_REPOSITORY_CASES` and `TestStableRepositoryEnvelopeOrdinary`, neither of which
  exists. It was left unchanged because it is sealed in `manifest.json`.
