# Design: `user-docs`

## Decisions

- Decided, R42 (panel pick) B6: all items (a) to (k) ship, as pages in the repository. Items (h) to
  (k) are confirmed by the same pick. The content of the compatibility statement (g) is the table
  of change `compatibility-promise` (decided 2026-11-09 under R42, D10 and D10a).
- First run (a). One page per product, plus the Action and the agent packages, under
  `docs/first-run/`. Each page shows the install command for the three targets, the first command,
  a real output captured from the release smoke fixture, and the abstention and refusal cases.
- Configuration reference (b). A table in `docs/CONFIGURATION.md` with one row per user-facing
  `.corvint/` file and environment variable: path or name, product, profile and version, default,
  and the requirement that defines it. A Go test collects the path and variable constants read by
  shipped binaries (from a declared list in code, not a scan of every string) and fails if a
  constant has no row or a row has no constant. Files read only by Corvint's own self-development
  carry an `internal` mark in the declared list and are excluded.
- Upgrade (c). Each removed verb keeps a stub that prints one line naming the replacement (or "no
  replacement") and the `docs/UPGRADING.md` anchor, and exits with one stable code reserved for
  removed verbs. Stubs stay for the 1.x series unless the compatibility statement says otherwise.
  `docs/UPGRADING.md` lists: `cem`, `runs`, `test-plan`, `tests lint`, the analyzer verbs, the
  retired MCP binaries, `.corvint/e2e-flows.json` (folded into `.corvint/flows.json` by the change
  `affected-plan-kinds`) and the CEM files under `.corvint/`.
- Error catalogue (d). `docs/ERRORS.md` lists exit codes and typed reasons with product, meaning and
  user action. A Go test compares the catalogue with the reason constants exported by a declared
  registry per product and fails on any difference.
- Release notes (e). `CHANGELOG.md` has one section per tag; the release workflow of the change
  `release-process` refuses a tag whose version has no section.
- Location (f). Pages stay in the repository under `docs/` as today; no site (R42 (panel pick) B6).
- Compatibility (g). `docs/COMPATIBILITY.md` holds the 1.x compatibility table defined by change
  `compatibility-promise` (CMP-V1-001 to 006): one row per surface with a `stable` or `preview`
  column (decided 2026-11-09 under R42, D10 and D10a). Earlier wording, superseded: the file
  held the owner's statement and said "not yet decided" until the owner wrote it.
- No telemetry (h). One sentence in `docs/INSTALL.md` and each product's first-run page: Corvint
  products send no data off the machine.
- Uninstall (i). The paragraph that starts at `docs/INSTALL.md` line 333 (integ 9ec3a29d0) is
  replaced: remove the binaries, remove the MCP and hook entries added to host configuration, and
  optionally delete `.corvint/` (and `.taskman/` for Tasks) from each repository.
- `SECURITY.md` (j). The root `SECURITY.md` text is copied into each public repository with the
  repository name changed.
- Windows (k). One sentence in `docs/INSTALL.md` and each first-run page.

## Failure modes

- A new reason constant added without a catalogue row: the catalogue test fails.
- A new `.corvint/` path added without a row: the configuration test fails.
- A removed verb without a stub: a command test over the removed-verb list fails.
- A tag without a changelog section: the release workflow refuses.
- A copied `SECURITY.md` that names the wrong repository: a text check per repository.
- Pages that show output no longer produced: first-run outputs are regenerated from the smoke fixture
  and checked for staleness.

## Acceptance evidence

- The configuration, catalogue, removed-verb and first-run staleness checks passing in `doc-gates`.
- `CHANGELOG.md` with an RC section and the release notes on the RC tag.
- `SECURITY.md` present in each listed repository (read with `gh api`).
- The acceptance record listing the owner's disposition of gaps (a) to (g).

## Rollback

- Each page and check is independent; remove a check from `doc-gates` to stop it blocking.
- Removed-verb stubs can be removed; the verbs then fail as unknown commands.
- Restore the previous uninstall paragraph from Git.
