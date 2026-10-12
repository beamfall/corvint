# V1-0040 slice 2: STR-V0-013 driver parity and local balanced placement

Date: 2026-10-11. Branch `claude/v1-0040-slice2`, stacked on slice 1 at
`30a918949d50ff00046da8e45aecf4029cb0dbd8`. `tools/str-parity` was built from
`8811e43dc8327237c1bf72fb6cd34d8c8ea77146` and the corvint binary from
`1a248337c7c58d4bd817902884934cf127d4d268`; the later commits change no `cmd/corvint` or
`internal/` source. Scope:
`docs/specs/session-test-runner-v0.md` STR-V0-013 (parity report) and STR-V0-008 (balanced
placement, local profile only). No call site changes: `script/gate-affected.sh`,
`tools/corvint-pr-tests`, the Makefile and `.github/workflows` are untouched, and the existing
drivers stay the rollback. Balanced placement stays experimental, and nothing here claims its
20% promotion bar.

## Parity method

`tools/str-parity` replays first-parent pairs offline in a shared clone. For each merge it checks
out the merge, uses its first parent as the base, and runs three planners in the same controlled
environment (`PATH=<fake go>:<go dir>:/usr/bin:/bin`, private `HOME` and `TMPDIR`, `LANG=C`,
`LC_ALL=C`, `TZ=UTC`):

- `corvint test --base <first parent> --dry-run`, local and `--profile ci --confine`;
- the local driver: `script/gate-affected.sh <base>` with `GO_TEST_COMMAND` from
  `make -s -n go-test` and `CORVINT_PLANNER` set to the same corvint binary;
- the CI driver: `tools/corvint-pr-tests --mode run` with the `tools/gate-affected-select`
  binary built from the same source.

A fake `go` first on `PATH` records each driver's `go test` argv and environment and runs nothing;
other `go` commands pass through. Corvint's package set is read from its `go test` arguments, and
units that name other packages are an error. The tool compares the Go package set, the argv flags, the
environment overlay (each variable whose value differs from the base environment) and the
fallback verdict. It categorises every difference or marks it `UNEXPLAINED`, and checks that the
checkout is clean after each pair. Scratch paths in the report are spelled `$WORK`.
`tools/str-parity/main_test.go` pins the argv, overlay and category reading, including that an
unrecognised package, environment or `INCOMPLETE` difference stays `UNEXPLAINED`.

Command (the selector and `corvint-pr-tests` binaries were built from `30a918949`; no later
commit changes anything in their dependency closure):

```sh
str-parity -repo <checkout> -end 645304a2fb71065f9061d8d8d866600d703e84c6 -pairs 50 \
  -corvint <corvint> -selector <gate-affected-select> -pr-tests <corvint-pr-tests> \
  -script <checkout>/script/gate-affected.sh -source 8811e43dc8327237c1bf72fb6cd34d8c8ea77146 -work <new dir>
```

## Inputs

- (a) The 50 first-parent pairs of the AFP-V0-011 measurement: `NOT_OBSERVED`. That measurement
  ended at `09e4bc2f` (affected-plan-v0.md AFP-V0-011; decision 0131). The commit is in neither
  `origin/main` nor `archive/private-main-be0d614`, and its commit list was never retained.
  Substitute, labelled as such: the 50 first-parent pairs ending at
  `645304a2fb71065f9061d8d8d866600d703e84c6`, the spec's verified state. All 50 are two-parent
  merges; the oldest is `99f6a03f25e0c7680d7d17d0d5dbb4bc87a19d99`.
- (b) The retained AFP-V0-029 shadow rows: `NOT_OBSERVED`. AFP-V0-029 is not implemented, and no
  forward-qualification rows exist in the repository.

So this report covers the substitute pairs only. It does not discharge STR-V0-013 for either call
site: the frozen pairs and the shadow rows must still be replayed once they exist (slice 3).

## Result

The run exited 0 in 1,086 seconds. The `corvint-str-parity/0` report is 225,542 bytes, with
sha256 `f509361d2b80d0c39e069729b484dc50f7c9d9f442324c3ae266799a9f724f86`. Runs with the
`97accef7b` and `bf3d88362` builds gave the same plans in every pair; the only change since is
the 9 executed-set differences the round-3 fix now counts for narrowed `INCOMPLETE` pairs. No
corvint invocation's units disagreed with its arguments. It is not committed:
the command above regenerates it, and the table below keeps every per-pair verdict, count and
category.

- No difference is `UNEXPLAINED`. There are 277 differences in six categories.
- Package sets are identical in all 46 pairs where corvint narrows: corvint local, the local
  driver and the CI selector's planned set name the same packages. Every package-set difference
  is one of the two widenings below.
- Corvint local's environment overlay equals the local driver's in all 50 pairs:
  `GOCACHE=/tmp/corvint-go-build-cache` and `GOTOOLCHAIN=local`.
- Fallback verdicts are identical in 47 pairs. The 3 that differ are STR-V0-003 widenings, where
  corvint is `FULL` and both drivers narrow.

| Profile | Field | Category | Count |
|---|---|---|---|
| local | argv | `local-json-flag` | 50 |
| ci | argv | `ci-exec-confine` | 50 |
| ci | env | `ci-closed-environment` | 50 |
| ci | admission | `ci-admission-no-qualification` | 49 |
| ci | executed | `ci-admission-no-qualification` | 46 |
| local, ci | error | `str-v0-004-undeclared-language` | 10 + 10 |
| local, ci | verdict | `str-v0-003-unknown-widening` | 3 + 3 |
| local, ci | packages | `str-v0-003-unknown-widening` | 3 + 3 |

Per pair, oldest first. The packages column gives counts in this order: corvint local, local
driver, CI planned, CI executed, universe. Corvint ci's package count equals corvint local's in every pair.
`INCOMPLETE` marks a corvint plan that would not pass.

| # | Merge | corvint local | Local driver | corvint ci | CI driver planned/executed | Packages corvint/local/CI planned/CI executed/universe | Categories |
|---|---|---|---|---|---|---|---|
| 1 | `99f6a03f25` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 136/136/136/319/319 | admission x2, env, exec, json |
| 2 | `1120e565a0` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 198/198/198/319/319 | admission x2, env, exec, json |
| 3 | `684cca5f7c` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 253/253/253/320/320 | admission x2, env, exec, json |
| 4 | `3503617bc7` | NARROWED INCOMPLETE | NARROWED | NARROWED INCOMPLETE | NARROWED/FULL | 246/246/246/320/320 | admission x2, env, exec, json, undeclared x2 |
| 5 | `136d905e4a` | NARROWED INCOMPLETE | NARROWED | NARROWED INCOMPLETE | NARROWED/FULL | 234/234/234/320/320 | admission x2, env, exec, json, undeclared x2 |
| 6 | `016bbd1748` | NARROWED INCOMPLETE | NARROWED | NARROWED INCOMPLETE | NARROWED/FULL | 130/130/130/320/320 | admission x2, env, exec, json, undeclared x2 |
| 7 | `8a0959922a` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 176/176/176/320/320 | admission x2, env, exec, json |
| 8 | `e5298bc573` | NARROWED INCOMPLETE | NARROWED | NARROWED INCOMPLETE | NARROWED/FULL | 200/200/200/320/320 | admission x2, env, exec, json, undeclared x2 |
| 9 | `41ed082121` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 171/171/171/320/320 | admission x2, env, exec, json |
| 10 | `cd5298d37d` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 99/99/99/320/320 | admission x2, env, exec, json |
| 11 | `1786d9dac5` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 185/185/185/320/320 | admission x2, env, exec, json |
| 12 | `1b35c6a83b` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 167/167/167/320/320 | admission x2, env, exec, json |
| 13 | `3776e1c38b` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 188/188/188/320/320 | admission x2, env, exec, json |
| 14 | `bdf171e565` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 177/177/177/320/320 | admission x2, env, exec, json |
| 15 | `e0ef9aa891` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 142/142/142/320/320 | admission x2, env, exec, json |
| 16 | `284c43129d` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 234/234/234/320/320 | admission x2, env, exec, json |
| 17 | `fb1986bc1a` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 124/124/124/320/320 | admission x2, env, exec, json |
| 18 | `b59a8d4a02` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 133/133/133/320/320 | admission x2, env, exec, json |
| 19 | `76c4df1dac` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 158/158/158/320/320 | admission x2, env, exec, json |
| 20 | `b7596eea9d` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 162/162/162/320/320 | admission x2, env, exec, json |
| 21 | `556fef970f` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 218/218/218/320/320 | admission x2, env, exec, json |
| 22 | `64b5d1745b` | NARROWED INCOMPLETE | NARROWED | NARROWED INCOMPLETE | NARROWED/FULL | 211/211/211/320/320 | admission x2, env, exec, json, undeclared x2 |
| 23 | `8b6af3eeab` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 159/159/159/320/320 | admission x2, env, exec, json |
| 24 | `a3607ec5b5` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 194/194/194/320/320 | admission x2, env, exec, json |
| 25 | `bf04e8a086` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 164/164/164/320/320 | admission x2, env, exec, json |
| 26 | `a4d284b3fb` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 193/193/193/320/320 | admission x2, env, exec, json |
| 27 | `3beedd981f` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 128/128/128/320/320 | admission x2, env, exec, json |
| 28 | `408c83bcb7` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 183/183/183/320/320 | admission x2, env, exec, json |
| 29 | `b524b5ed40` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 151/151/151/320/320 | admission x2, env, exec, json |
| 30 | `fac080be5e` | FULL | NARROWED | FULL | NARROWED/FULL | 321/164/164/321/321 | admission, env, exec, json, widening x4 |
| 31 | `bf2899b701` | NARROWED INCOMPLETE | NARROWED | NARROWED INCOMPLETE | NARROWED/FULL | 164/164/164/321/321 | admission x2, env, exec, json, undeclared x2 |
| 32 | `5e664ce9cf` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 162/162/162/321/321 | admission x2, env, exec, json |
| 33 | `26abaafcfa` | FULL | NARROWED | FULL | NARROWED/FULL | 321/155/155/321/321 | admission, env, exec, json, widening x4 |
| 34 | `36f0089a60` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 171/171/171/321/321 | admission x2, env, exec, json |
| 35 | `2145af619f` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 200/200/200/321/321 | admission x2, env, exec, json |
| 36 | `29e1f65348` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 139/139/139/321/321 | admission x2, env, exec, json |
| 37 | `71b2eb8fc3` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 173/173/173/321/321 | admission x2, env, exec, json |
| 38 | `32bfc85a44` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 231/231/231/321/321 | admission x2, env, exec, json |
| 39 | `363c8b9c75` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 138/138/138/321/321 | admission x2, env, exec, json |
| 40 | `686fb1ac9c` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 159/159/159/321/321 | admission x2, env, exec, json |
| 41 | `97c3a4bd03` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 148/148/148/321/321 | admission x2, env, exec, json |
| 42 | `55aa6bb9f1` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 204/204/204/321/321 | admission x2, env, exec, json |
| 43 | `00dd220753` | NARROWED INCOMPLETE | NARROWED | NARROWED INCOMPLETE | NARROWED/FULL | 295/295/295/322/322 | admission x2, env, exec, json, undeclared x2 |
| 44 | `8b65914249` | NARROWED INCOMPLETE | NARROWED | NARROWED INCOMPLETE | NARROWED/FULL | 256/256/256/323/323 | admission x2, env, exec, json, undeclared x2 |
| 45 | `10efc02d16` | NARROWED INCOMPLETE | NARROWED | NARROWED INCOMPLETE | NARROWED/FULL | 282/282/282/323/323 | admission x2, env, exec, json, undeclared x2 |
| 46 | `8f032d969f` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 226/226/226/324/324 | admission x2, env, exec, json |
| 47 | `3b123d3def` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 154/154/154/324/324 | admission x2, env, exec, json |
| 48 | `a0e56f3c14` | NARROWED | NARROWED | NARROWED | NARROWED/FULL | 220/220/220/324/324 | admission x2, env, exec, json |
| 49 | `01f95dbe46` | FULL | NARROWED | FULL | NARROWED/FULL | 324/162/162/324/324 | admission, env, exec, json, widening x4 |
| 50 | `645304a2fb` | FULL INCOMPLETE | FULL | FULL INCOMPLETE | FULL/FULL | 324/324/324/324/324 | env, exec, json, undeclared x2 |

## Explanations

- `local-json-flag` (local argv, 50): corvint adds `-json` to the repository's `go-test` flags
  so it can attribute outcomes per unit (STR-V0-005/006). The owner accepted this in slice 1. The
  rule requires the argv to be identical once `-json` is removed.
- `ci-exec-confine` (ci argv, 50): corvint's ci profile runs
  `go test -exec <confine> -json -p 1 -count=1 -race -timeout 50m` on the AFP-V0-028 confined
  branch. `tools/corvint-pr-tests` runs the same flags with no `-exec`, because its confinement is
  the workflow's. The rule requires exactly one `-exec`, right after `test`, naming the wrapper
  the tool passed, and an identical argv once it is removed. Slice 3 reconciles this at the call
  site.
- `ci-closed-environment` (ci env, 50): corvint sets exactly `CGO_ENABLED=1`, `GOFLAGS=`,
  `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local` and `GOWORK=off`, plus
  `CORVINT_CONFINE_ROOT=<checkout>`, over the inherited environment. The CI driver instead builds
  a closed environment: those six, the fixed entries `CC=/usr/bin/cc`, `GIT_CONFIG_*`,
  `GIT_GRAFT_FILE=/dev/null`, `GIT_NO_REPLACE_OBJECTS=1`, `GIT_TERMINAL_PROMPT=0`, `GOENV=off`,
  `GOMAXPROCS=2`, `LANG=C`, `LC_ALL=C` and `TZ=UTC`, and one path each for `GOCACHE`,
  `GOMODCACHE`, `HOME`, `PATH`, `TEMP`, `TMP` and `TMPDIR`. The rule requires corvint's exact
  overlay and exactly that driver environment, with only the path values free, so a dropped,
  changed or added driver variable stays `UNEXPLAINED`.
  Whether `corvint test --profile ci` should build the driver's closed environment itself or
  inherit it from the workflow is a slice-3 call-site decision.
- `ci-admission-no-qualification` (49 admission, 46 executed): with no trusted qualification pin
  (AFP-V0-013/029 forward rows do not exist offline), `tools/corvint-pr-tests` executes `FULL`
  over the universe even when its selector narrows. Admission is the call site's job (STR-V0-012),
  so the comparison uses the selector's planned verdict and set. Both the admission and the
  executed-set difference are explained only when the driver executes exactly the universe, and
  the executed one also only when corvint runs nothing outside it.
  It is also compared when corvint is `INCOMPLETE` (the next category), because that plan still
  names its packages.
- `str-v0-004-undeclared-language` (10 pairs, both profiles): the change touches TypeScript in
  all 10, and also Python or Ruby in one each. The repository Makefile declares no
  `typescript-test`, `python-test` or `ruby-test` target, so corvint reports `INCOMPLETE`, while
  the Go-only drivers run their Go set and could pass. This is STR-V0-004 by design: no adapter
  and no declared full command means the run is not a pass. It is still the largest practical gap.
  If `make gate-affected` switched today, 10 of these 50 pairs (20%) would end `INCOMPLETE`
  instead of a Go-only pass. In 9 of them the Go package sets are identical.
- `str-v0-003-unknown-widening` (3 pairs: `fac080be5e`, `26abaafcfa`, `01f95dbe46`): each plan
  has a `plan.unknown` entry outside the AFP-V0-028 allowlist (`UNINDEXED_SOURCE_PATH` for
  `.github/cishards/cmd/main.go`, `.github/cishards/slices_test.go` and
  `interop/cem01-go/cem.go`). Corvint widens to `FULL` (STR-V0-003); the selector's attribution
  narrows. The rule requires corvint to run exactly the universe and omit nothing a driver runs.
  This is the intended difference: the runner does not trust an unindexed source path, even where
  the selector's attribution narrows.
- Not observed in these pairs: `ci-nothing-runs-full` (no merge planned `NOTHING`). No plan
  declared a non-Go command, because the Makefile declares none. A declared command has no
  validated explanation yet, so one would be `UNEXPLAINED` until slice 3 checks it against the
  Makefile's declarations.

No difference needed a runner fix. `internal/sessiontest/declared.go` typed the go-test recipe in
all 50 pairs: the corvint local argv equals `make -s -n go-test` apart from `-json`. Parity
exposed no typing issue.

## Balanced placement (STR-V0-008)

- For a `NARROWED` run, `--placement balanced` calls the same `cishards.PlanFrom` as universe
  placement, over the deduplicated run set instead of the universe. Its execution units are the
  units universe placement would produce: the same split rules, the same catch-all slice, sibling
  slices on different shards. A run set of at least N units uses all N shards. With an unusable
  cost table, `PlanFrom` places the run set round-robin in lexical order.
- `FULL` keeps the complete-universe partition in either mode. The manifest's `placement` records
  the requested mode, so a balanced and a universe manifest have different digests even when
  their units are the same.
- Every shard recomputes the manifest. A shard whose recomputed digest differs from `--manifest`
  exits 3 before running anything. Because membership depends on the plan, a sharded balanced run
  without `--manifest` is refused (exit 2) instead of letting each shard place on its own.
- `--profile ci --placement balanced` is accepted only with `--dry-run`, which computes it. CI
  executes universe placement until an accepted AFP-V0-028 amendment says otherwise. Retaining
  the computed placement beside the executing one is the CI call site's job (slice 3).
- Evidence: `internal/sessiontest/balanced_test.go` covers 1 to 16 shards over four run sets
  (one package, the split package alone, every third package plus the split package, and the
  whole universe narrowed); every shard is used; `FULL` and digest binding. The CLI tests
  `TestSTRV0008CLIUsageRefusals` and `TestSTRV0008CLIBalancedShards` cover two balanced shards
  that pass and then verify, a universe shard refused against a balanced manifest, and a ci dry
  run. A mutation that disables the balanced branch fails the placement tests.
- Not measured: the local 20% p90 bar over executed runs (`NOT_RUN`). Balanced placement is not
  promoted.

## Remaining for slice 3

- Replay the real inputs once they exist: the AFP-V0-011 measurement's pairs ending at
  `09e4bc2f` (lost with the private lineage; re-deriving them needs owner direction) and the
  AFP-V0-029 shadow rows. Until then STR-V0-013 stays partial, and neither call site may switch.
- Before `make gate-affected` calls `corvint test`: declare `typescript-test` (and `python-test`
  and `ruby-test` where those languages change), or accept that such changes end `INCOMPLETE`.
- The CI call site (AFP-V0-016 consent, a new `CORVINT_PR_FORWARD_EPOCH`): reconcile the confined
  `-exec` and the closed environment, keep admission (qualification pin, queue backstop, trip) in
  the driver, and retain the computed balanced placement beside the executing universe placement.
- Balanced placement: measure the local 20% p90 bar over executed runs before any promotion.

## Review and dependencies

- Codex read-only review, round 1: changes requested (4 findings: CI packages used the planned
  set instead of the executed one; widening could hide a package corvint omitted; the closed
  environment check accepted omissions; any `-exec` wrapper was accepted). Fixed in `97accef7b`.
- Round 2: changes requested (4 findings: admission did not require the driver to execute the
  universe; the driver environment was only checked for containment; corvint's package set came
  from units, not argv; decision 0524 is absent from this branch). The three tool findings were
  fixed in `bf3d88362`, and the run was repeated with that build. It gave the same plans and
  categories in all 50 pairs.
- Round 3 (the last allowed): changes requested (2 findings: an `INCOMPLETE` corvint plan skipped
  the executed-set comparison, so a package the CI driver dropped could go unseen; every declared
  non-Go command was explained without validation). Both were fixed in `8811e43dc` with regression
  tests, and the run above is from that build. These fixes had no further review round.
- The spec's accepted intent cites decision 0524
  (`docs/decisions/0524-str-v0-001-018-session-test-runner-accepted-2026-10-11.md`). It is
  retained on `claude/v1-0040-session-test-runner-spec` (beamfall/corvint#815) and not in this
  stack's base, so this branch must land after that one or together with it.
