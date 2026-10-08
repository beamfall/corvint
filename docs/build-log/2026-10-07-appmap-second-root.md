# 2026-10-07: application map — E2E tests in a second, aliased repository (V1-0982)

## Intent

[Issue 669](https://github.com/beamfall/corvint/issues/669) part 2 (native V1-0982): many
applications keep routers in one repository and the E2E suite in another. The map must join them
without a silent partial join, pin every root's revision, and say which root each piece of evidence
comes from. Contract: proposed AMAP-V0-017..019 in `docs/specs/application-map-v0.md`, each pending
owner acceptance.

## Decisions

- **Reuse the MMR-V0 alias model.** `--repo ALIAS=ABSOLUTE_ROOT` uses the MMR-V0-001/002 spelling,
  alias grammar and MCPV0-001 root bounds. The parser and validators moved from
  `cmd/corvint-mcp/roots.go` and `internal/mcp/bridge` to `internal/rootalias` without behaviour
  change, so both surfaces share one definition.
- **Routers and flows stay in `--root`.** Only `tests.*` (and, with `--manifest-repo`, the manifest)
  come from an aliased root. RVN step anchors are router lineage, so run verification is unchanged.
- **Aliased roots are read at `HEAD`, dirty refuses.** A second revision flag was not added; to keep
  `HEAD` honest the build refuses with `appmap-root-dirty` when `git status` shows staged, unstaged
  or untracked changes under the inputs it reads from that root. Changes elsewhere do not refuse.
  Whether an explicit per-root revision is preferable is owner question 15.
- **Byte-compatible single root.** New members (`roots`, `repo`) are `omitempty`; a single-root
  map is byte-identical with or without `--repo`, and to the pre-change binary.
- **Consumers fail closed.** Projections, the planner and the scaffold read only `--root`, so an
  anchor with `repo` reads `UNKNOWN` (never `FRESH` from a same-named path in `--root`). Letting
  them accept `--repo` is a follow-up.

## Evidence

- Maintained tests: `TestAMAPV0017TwoRootJoin`, `TestAMAPV0017RootUnavailable`,
  `TestAMAPV0019DirtySecondRoot`, `TestAMAPV0018SingleRootUnchanged`,
  `TestAMAPV0020ProjectionsReadAliasedAnchorsUnknown`, `TestAMAPV0017RootPathKeepsTrailingSpace`
  (synthetic Git repositories).
- Independent review (Codex) found two issues, both fixed: `find` and planner references dropped
  the root alias (now `repo` on find items, planner methods and spec), and the top-level check
  trimmed a valid trailing space from a root path (now only Git's newline is removed).
- Measured: the committed fixture's map is 20943 bytes, SHA-256 prefix `5ded12df5785afc6`, from
  both the base binary (`0b5096ca`) and this change, with and without an unused `--repo`. The same
  fixture split into two repositories joins all 9 test files with `repo: e2e`, pins the E2E `HEAD`,
  and builds in 0.63 s wall time on the development host; an undeclared alias, a relative root and
  an uncommitted page object refuse with exit 2 and no stdout.

## Rollback

Revert the change: drop `BuildRoots`, the `repo`/`roots` members, the two refusal codes and the
two flags. `internal/rootalias` may stay as the shared MMR-V0 helper. No stored state changes.
