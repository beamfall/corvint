# 2026-11-03 lane selbugs: selection correctness and over-selection

Lane selbugs (WSL), branch `claude/selbugs`, base `6f12f4704235dc8f9bea19fc12995bfb68c746c3`
(origin/main). Requirement IDs AFP-V0-2400 to AFP-V0-2406 (in `affected-plan-selection-v0.md`,
the parent being at its obligation bound), CIN-V0-2320 and ASD-V0-2400 to ASD-V0-2403 (in the new
companion `corvint-self-development-selection-v0.md`, the parent's Requirements section holding
254 of 256 obligations). The native tickets were not readable from this host (no journal; not in
the committed `.taskman` snapshot), so the lane worked from the lane brief and V1-3480's text in a
retained audit file.

## Dispositions

| Ticket | Requirement | Result |
|---|---|---|
| V1-3489 | AFP-V0-2400, AFP-V0-2401, CIN-V0-2320 | script self-tests selectable by their inputs and runnable by selection; CI job in its own commit |
| V1-4363 | AFP-V0-2402, AFP-V0-2403 | `corvint test` runs exactly the plan's Go selection; selection-baseline pins agreement |
| V1-3480 | AFP-V0-2404, AFP-V0-2405 | LICENSING.md, protocol/ and 12 declarations mapped to their readers; Makefile keeps its entry with the reason named |
| V1-3968 | ASD-V0-2400 | heavy.sh admits a brand-new package |
| V1-4358 | ASD-V0-2401 | heavy.sh suggests a manifest path `corvint test` accepts |
| V1-3804 | ASD-V0-2402 | batch-precheck narrows with CI's allowlist |
| V1-4281 | AFP-V0-2406 | frontierSites counts only involved plugins |
| V1-4245 | ASD-V0-2403 | test-only darwin packages offload whole, their darwin tests run locally |

## Decisions

- V1-3489: the omission had two layers. No PR job ran any `make shell-test` step, and the plan
  missed reads it cannot see (`no-python-runtime-dependency_test.sh` prints the bare word
  `Makefile` and lists `script/*.sh` and `integrations/` with `find`). The first is fixed by a
  selection tool and a CI job, the second by an additive declaration
  (`.corvint/shell-test-inputs.json`), which can only add selections. `UNOWNED_DIRTY_PATH`
  runs every shell step, because a shell test may read an unowned path by an unresolved word.
  The CI job is its own commit, effective only after the owner's review.
- Direct evidence for V1-3489: `make no-python-runtime-dependency-test` is red on main at the base.
  `script/batch-precheck.sh:402` and `script/batch-precheck_test.sh:1019`, from ASD-V0-912, name
  `python`, and no PR gate ran the check. This lane removes the first line, rewrites the second
  as a split literal, and the check passes.
- V1-4363: the two paths are reconciled toward the plan. The extra packages came from the
  attribution's rule (a) to (c) lines: a lone-component or directory-relative literal that
  AFP-V0-021/072 narrow, and namer edges from any path token that names a package directory. Each
  carried an `attributed:` reason, but no witness chain. The CI deciding selector keeps them, so
  CI's executed set is unchanged.
- V1-3480: AFP-V0-602 (runtime-readers) is already the declared input mapping. `Makefile` cannot
  be declared (driver input), and neither can the three self-classifying declarations, so they
  keep `UNOWNED_DIRTY_PATH`; AFP-V0-2405 names why on the Makefile's entry.
- V1-4245: only packages whose darwin-only files are all test files split. A darwin-only source
  file can be reached by every test, so its package stays local. Of the 42 packages with
  darwin-only files, 8 are test-only (`internal/tasks/store`, `internal/testrunner`,
  `internal/testrunner/platform`, `internal/testacceptance`, `internal/jstestprovider`,
  `internal/analyzercap`, `cmd/corvint-analyzer-go`, `cmd/corvint-analyzer-html-css`).

## Measurements (same build before/after: `6f12f4704` and the lane head; ten merged PRs #79 to #88)

- `corvint test --profile ci --dry-run` Go units over the nine NARROWED ranges: 1418 before, 891
  after, equal to `corvint affected`'s 891 (extras per range before: 0, 18, 25, 36, 52, 28, 143,
  124, 101; after: 0). The FULL range (#85, `UNINDEXED_SOURCE_PATH`) is unchanged.
- batch-precheck first-precheck narrowing over the same ten plans: 2 before; 9 after, with #85
  still running everything for `UNINDEXED_SOURCE_PATH`.
- `frontierSites` reasons per plan: 27 over 8 plugins before; 3 (`go`, `shell`) after for a Go and
  shell change, 25 for #85, which involves the other plugins.
- `make selection-baseline-check` replay (43 merged, 15 failed-CI changes; run in two halves of the
  change set to stay inside the foreground limit): narrow 33/43 before and after, misses 0;
  `corvint test` whole packages over merged changes 8188 before, 6648 after; test-extra 0 and
  test-misses 0 over 26 failing packages.
- Plans of one dirty path (`--path`, unknown entries before -> after, selections unchanged):
  `LICENSING.md` 79 -> 2; `protocol/cem-1.0/README.md` 61 -> 1; each of the 12 declared
  `.corvint/*.json` 61 or 62 -> 1 or 2; `Makefile` 62 -> 62, now with the AFP-V0-2405 hint, and
  `no-python-runtime-dependency_test.sh` selected (`PATH_GLOB_READER glob /Makefile`).
- V1-3489 fault injection: `python3 -c 1` appended to `script/batch-regen.sh`. Base planner:
  `shell-test-select` NARROWED 14 of 45 steps, no-python not among them. Lane head: `make
  shell-test-affected` NARROWED 15 of 45, runs `no-python-runtime-dependency-test`, which fails
  naming `script/batch-regen.sh:188:python3 -c 1`.

## Failures seen at the base (not caused here)

- `make dogfood-cache-test` and `make spec-requirements-test` fail on a clean base checkout.
- `make ci-least-privilege-check` fails at base on `go-product-shard`'s `actions: read`.
- `TestSelectionOnTheLiveDirtyWorktree` fails at base whenever `.gitignore` is dirty
  (`internal/dogfoodflow` witness via `.gitignore` is unresolvable).
- `make repin-test` failed only in a scratch worktree whose HEAD was an unpublished commit.
