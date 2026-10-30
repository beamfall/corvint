# Decision 0314 — Every commit on main carries a new build number

Date: 2026-09-19. Status: accepted (owner instruction, verbatim: "whenever creating a new version
of corvint on main there should aways be a new build number"; the owner chose "every merge to main"
and the `Corvint <VERSION> (build N)` banner).

## Decision

The build number is the first-parent commit count of the built commit (`PUB-V0-021`). Every merge
to main adds at least one first-parent commit, so every build of a new main commit carries a larger
number than every earlier one, and nobody edits a file to bump it.

- **Stamping.** `make build`, the loose archive gate (`buildOnce`) and the archive build
  (`archiveBuildOnce`) pass `-ldflags=-X main.build=N`. The archive build counts from the real Git
  directory it already uses for VCS stamping, so both independent builds stamp the same number and
  stay byte-identical. A plain `go build` reports build `0`.
- **Banner.** `corvint --version` prints `Corvint 0.4.0a4 (build N)`. `VERSION` and the
  `PUB-V0-001` tuple are unchanged; the build number is not a pin. The VS Code probe
  (`VSC-V0-007`) and the three dogfood coordinators accept the suffix and refuse a banner without it.
- **Smoke.** The release smoke requires the exact stamped number, not any number.

## Alternatives weighed

- *Committed `BUILD` integer bumped only with `VERSION`*: rejected by the owner; commits between
  version bumps would share one build number.
- *Fold the build into the prerelease counter (`a4` → `a5` per merge)*: rejected; every merge would
  move the whole `PUB-V0-001` tuple, including the extension's exact pin.
- *A separate `--build` flag*: rejected by the owner in favour of one banner.

## Consequences

The `--version` wire changes. An extension built before this change refuses the new banner as
`CLI_INCOMPATIBLE`, and a new extension refuses an older binary. Both are shipped from the same
tree, so they move together. A shallow clone counts only the commits it has, so a release build
needs full first-parent history. Historical evidence keeps its original `Corvint 0.4.0aN` strings.

`CRB-V0-003` in `docs/specs/corvint-rebrand-v0.md` and the `CORVINT_BIN` override rule in
`docs/DOGFOOD.md` §4 are amended to the new banner; version equality is unchanged.

## Rollback

Revert the change: the banner returns to `Corvint <VERSION>` and the extension and dogfood checks
return to the exact match. No persisted state or format depends on the build number.
