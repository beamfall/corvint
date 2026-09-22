# Decision 0109 — the 0.4.0a4 alpha's publication set, receipt, promotion and readiness bar

Date: 2026-09-12. Status: accepted. Authority: repository owner, verbatim instruction "I want you to
make the calls for me" (delegated owner call).

Review `docs/reviews/r8-release.md` section 3(c) left D3 (publication destination and receipt,
`ARTIFACT-RDY-V0-004`) and D4 (promotion, `ARTIFACT-RDY-V0-005`) as proposals, and they were written
for a `v0.4.0` tag. `script/release-checklist` exits zero only when all six rows `PASS`, and
native-performance is unconditionally `NOT_RUN` (`GOC-V0-005`). Read literally, that bar can never be
met for this alpha. `docs/plans/public-release-v0-gap-2026-09-12.md` records the gap table this
decision closes against.

The owner call, for the `0.4.0a4` alpha prerelease only:

1. **Publication set (D3, adapted).** The destination is a GitHub Release marked *prerelease* on
   `Beamfall/corvint` at tag `v0.4.0a4`. It carries the `darwin_amd64`, `darwin_arm64`, `linux_amd64`
   and `linux_arm64` `corvint` archives exactly as `script/go-archive-gate` retained them at the
   frozen candidate, plus that gate's archive-level `SHA256SUMS` unchanged. The `windows_amd64` zip
   is not attached (`PUB-V0-007`). `SHA256SUMS` still names it, because rewriting the file would
   break its binding to the gate witness, and the release notes say it is absent. The notes also
   say only the gate host's platform was smoke-qualified. The optional companion bundle is
   attached only if `script/corvint-companion-release-gate` has retained one at the same candidate.
   No PyPI or other registry upload happens.
2. **Receipt.** After the owner performs the outward actions, the receipt is a new decision record.
   It names the release URL, tag, commit, tree, and every attached file's SHA-256, copied from the
   witnesses. It states publisher identity `NOT_VERIFIED` (decision 0108). The checklist's
   publication row stays `NOT_RUN` until a receipt reader is specified and implemented. That reader
   is a separate slice. `ARTIFACT-RDY-V0-001`'s current-revision witness rule conflicts with a
   receipt that is committed after the tag, and that slice must resolve the conflict.
3. **No promotion (D4).** Every command stays at its current label. The promotion row stays
   `NOT_RUN` by design, and the release notes say so. This is the only option consistent with
   `internal/betarung/admissions.json`.
4. **Readiness bar.** `script/release-checklist` exiting 1 is expected for this alpha while exactly
   native-performance, publication and promotion are `NOT_RUN`. Any `FAIL` row, or a `NOT_RUN`
   native-runtime, go-archive or tag row, blocks publication. The alpha is ready to publish when
   all of the following hold:
   - native-runtime `PASS`, go-archive `PASS` and tag `PASS`, at the frozen candidate (`PUB-V0-008`);
   - `PUB-V0-006` is met;
   - if the companion bundle ships, it was retained by the gate with corvint-taskman's `LICENSE` and
     `PROVENANCE.md` present (decision 0107), and the `PUB-V0-004` browser qualification names the
     tested browser and macOS versions;
   - if the bundle does not ship, the release notes drop it from Scope and say so.

Nothing here tags, pushes, changes visibility, signs, uploads or promotes. Those remain the owner's
outward actions, listed in the gap plan's release checklist.

Rollback: delete the GitHub Release (the tag stays), or supersede this record before publication.
