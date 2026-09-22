# Decision 0167 — Companion binaries build from the staged export; the bundle ships as a verified archive

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

Two `docs/agent-memory/bugs.md` entries (2026-09-12) found that `internal/companionrelease.Run` did
not meet `docs/specs/public-release-v0.md` as written. First, every binary was built with the live
checkout as its module root, while the clean-tree check runs `git status --ignored=no`: a
gitignored `.go` file, or a checkout change after that check, could reach a shipped binary that
the bundle's source archive (read from the exported Git tree) would not reproduce. Second,
`PUB-V0-014` requires the assembled bundle archive to be built twice, compared and decode-verified,
but only the per-component source archives went through `buildTarGzTwice`/`verifyTarGz`. The
bundle was retained as a directory, and its README embedded `time.Now()`, so two assemblies could
not have agreed anyway.

The owner call: implement the spec rather than weaken it.

1. `Run` writes each module's exported, digest-verified tree into a fresh scratch directory and
   re-verifies it there: the exact exported file set, each file matching its Git object id. It
   builds every component with that directory as the module root, never the checkout.
   `PUB-V0-013` now states this.
2. `Run` renders the bundle members (binaries, source archives, notices, `MANIFEST.json`,
   `README.md`, `SHA256SUMS`) from inputs that involve no clock or disk read. It builds the bundle
   tar.gz twice through `buildTarGzTwice`, decode-verifies it with `verifyTarGz`, and only then
   runs the installed smoke on the verified members. The README's `Generated:` wall-clock line is
   removed, not replaced: the manifest already records each component's commit.
3. After a passing smoke, the retained output is `<bundle-name>/<bundle-name>.tar.gz` plus
   `<bundle-name>.tar.gz.sha256`, not an expanded directory. `SHA256SUMS` inside the archive still
   covers every shipped member. `PUB-V0-014` now names this layout. The report and
   `cmd/corvint-companion-release` add the archive path and digest.

Consequences: `PUB-V0-013` and `PUB-V0-014` are amended and the spec header records this decision.
The traceability table gains a `PUB-V0-003` row and new tests for `PUB-V0-013`/`014`/`015`. A
consumer that expected the expanded bundle directory must now extract the archive. No such
consumer exists in this repository: `script/corvint-companion-release-gate` only invokes the command.
No real release was created, signed, tagged or published.

Rollback: revert this decision's commit. That restores checkout-rooted builds, the expanded
directory, and the wall-clock README line, and reopens both bugs.md entries.
