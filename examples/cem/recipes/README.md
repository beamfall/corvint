# Understand, review and CI-verification recipes

Three ordinary Bash scripts that compose existing Corvint commands for one committed change,
`BASE..HEAD`. They add no workflow language, scheduler, service or new Corvint verb. The
requirements are `CEM-PILOT-024`..`027` in
[`docs/specs/cem-pilot-kit.md`](../../../docs/specs/cem-pilot-kit.md). Like everything under
`examples/**`, these files are Apache-2.0 ([`LICENSING.md`](../../../LICENSING.md)).

| Recipe | Script | Status |
| --- | --- | --- |
| 1. Understand a proposed change | `understand-change.sh` | experimental: composes experimental `impact --base`, `affected --base` and `context` |
| 2. Prepare a change-evidence review | `review-change.sh` | experimental: CEM pilot workflow (`cem/0.2`) plus advisory `review --base` |
| 3. Verify portable proof in CI | `ci-verify.sh` | experimental: wraps the digest-pinned `verify-portable.sh` (`cem/0.1`, [runbook](../README.md)) |

No recipe is a supported or qualified release path yet. A recipe that completes proves only
that the composed commands ran and returned their receipts; it does not prove the change is
correct, reviewed or safe to skip tests for.

## Common behaviour

- Prerequisites: Bash 3.2 or later, Git, and a `corvint` executable (tested with the installed
  release, Corvint 0.7.0 build 46, and a build of this checkout). Recipe 3 runs no `corvint`, but
  producing its map does.
- Every command runs as a child in its own process group, bounded by `RECIPE_TIMEOUT` seconds per
  step (default 600). A step that overruns gets `TERM`, then `KILL` after a 2-second grace, and
  the recipe exits 2. `EXIT`, `HUP`, `INT` or `TERM` of the recipe kills the running step first.
- Each step writes `NAME.out` and `NAME.stderr` into `RECIPE_OUT` (default: a new temporary
  directory). `RECIPE_OUT` must be new or empty (exit 2 otherwise), so receipts from different
  runs never mix; keep it outside the repository. Refusals stay there: nothing is deleted,
  retried or replaced to turn a refusal into success.
- Standard output is one `step=NAME exit=N` line per step and one final `outcome=...` line.

## 1. Understand a proposed change

Inputs: `CORVINT_BASE` (full commit ID; `HEAD` is the change), `CORVINT_TASK` (one sentence),
optional `CORVINT_SUBJECT` (repository-relative path), `CORVINT_ROOT` (default `$PWD`),
`CORVINT_BIN`. The worktree must be clean. Read-only: no repository, index or trace writes.

```sh
CORVINT_BASE=$(git rev-parse origin/main) CORVINT_TASK='Raise the session token lifetime' \
CORVINT_SUBJECT=auth/auth.go RECIPE_OUT=/tmp/understand examples/cem/recipes/understand-change.sh
```

It runs `corvint impact --base`, `corvint affected --base` and `corvint context --task`. Expected
output ends `outcome=complete impact-state=READY out=DIR`, exit 0. Read `impact.out`
(`context.results`, `coverage.uncertainty`), `affected.out` (`plan.selected`, `plan.unknown`,
`advice`) and `context.out`. Completion boundary: three receipts exist and impact is `READY`.
Mandatory checks named in `affected` advice stay mandatory; unknowns stay open.

| Case | Outcome | Recovery |
| --- | --- | --- |
| Base not in the repository (missing) | exit 3, `refused=impact`, `unsupported-impact-range` kept | fetch the base, or pass the full ID of a local commit |
| Uncommitted edits (stale range) | exit 3, `refused=impact`, `unsupported-impact-worktree` kept | commit or stash, rerun |
| No Go path in the range (unsupported) | exit 3, `impact-state=OUT_OF_SCOPE`, all receipts kept | review those paths with the repository's own docs and tests |
| `CORVINT_TASK` unset | exit 2, `outcome=operational` | set it |

## 2. Prepare a change-evidence review

Inputs: `CORVINT_BASE`, optional `CORVINT_ROOT`, `CORVINT_BIN`, `CEM_MAX_UNKNOWN` and
`CEM_MAX_MECHANICAL` (default 0 each). This is the public-command form of the Corvint dogfood
loop; Corvint's own changes use `make dogfood-change` and `make dogfood-check` as
[`docs/DOGFOOD.md`](../../../docs/DOGFOOD.md) §4 and §6 describe.

```sh
CORVINT_BASE=$(git rev-parse origin/main) RECIPE_OUT=/tmp/review examples/cem/recipes/review-change.sh
```

Steps: `cem prepare --base BASE --target HEAD` (writes or resumes `.corvint/change.cem.json`;
the only writer), strict `cem status` with the caps, a check that the map is committed at `HEAD`,
`cem report`, and advisory `review --base`. Expected output ends
`outcome=complete report=PATH out=DIR`, exit 0; `PATH` is the rendered review, kept private under
the Git directory. Completion boundary: every hunk is within the caps, the map is committed, and
the report exists. Whether the cited evidence supports each hunk is still the reviewer's call.

| Case | Outcome | Recovery |
| --- | --- | --- |
| First run, hunks unknown (missing) | exit 3, `refused=status`; `status.out` lists each hunk | `corvint cem cite --map .corvint/change.cem.json --hunk N ...` or `cem mark`, then rerun |
| Map cited but not committed | exit 3, `map-uncommitted=.corvint/change.cem.json` | commit the map, rerun |
| New commit after the map (stale) | exit 3, `refused=prepare`, `different base or patch` kept; map unchanged | a person decides to run `cem prepare --replace` and cite again |
| Map with an unknown `spec` (unsupported) | exit 3, `refused=prepare`, `not a valid CEM document` kept | restore the map from Git, or replace it deliberately |

## 3. Verify portable proof in CI

Inputs: those of [`verify-portable.sh`](../verify-portable.sh): `CEM_BASE_SHA`, `CEM_HEAD_SHA`,
`CEM_VERIFIER`, `CEM_VERIFIER_SHA256`, optional `CEM_REPOSITORY`, `CEM_HEAD_REPOSITORY`,
`CEM_MAP_PATH`. Prerequisites: a `cem/0.1` map committed at the head, and the `interop/cem01-go`
executable pinned by module version and SHA-256 as [`../README.md`](../README.md) §1 describes.
Verification uses no hosted Corvint service, no LLM and no network.

Produce the map from the exact [`docs/CEM-CI.md`](../../../docs/CEM-CI.md) patch profile:

```sh
git -c core.quotePath=false -c diff.algorithm=myers -c diff.context=3 \
  diff --binary --full-index --no-color --no-ext-diff --no-textconv --no-renames \
  --no-indent-heuristic --diff-algorithm=myers --unified=3 \
  --src-prefix=a/ --dst-prefix=b/ --ignore-submodules=none \
  "$BASE" "$CHANGE" -- . ':(exclude).corvint/change.cem.json' > /tmp/change.patch
corvint cem begin --patch /tmp/change.patch --output .corvint/change.cem.json --base "$BASE"
corvint cem cite --map .corvint/change.cem.json --hunk 1 --evidence-path docs/decision.md \
  --lines 3:3 --relation decision
git add .corvint/change.cem.json && git commit -m 'change evidence'
```

Then verify:

```sh
CEM_BASE_SHA=$BASE CEM_HEAD_SHA=$(git rev-parse HEAD) CEM_VERIFIER=/path/to/cem01-go \
CEM_VERIFIER_SHA256=<pinned sha256> RECIPE_OUT=/tmp/ci examples/cem/recipes/ci-verify.sh
```

Expected output ends `outcome=accepted exit=0 recovery=none out=DIR`; `DIR/verify.out` is the
`cem-ci-report/0` line. The exit status is the verifier's (0..5). Completion boundary: the map is
structurally valid for the exact patch; the report's `limits` say semantic support is not proven.
On GitHub Actions use [`../github-actions-portable.yml`](../github-actions-portable.yml), which
adds the network-denied sandbox.

| Case | Outcome | Recovery |
| --- | --- | --- |
| No map at the head (missing) | exit 3, `missing-evidence`, report `code` `map-absent` | commit a `cem/0.1` map, rerun |
| Code committed after the map (stale) | exit 1, `rejected` | re-derive the patch, rebuild and cite the map, commit, rerun |
| A `cem/0.2` map (unsupported) | exit 4, `unsupported-profile` | use `../verify-pr.sh` with a reviewed `corvint`, or produce `cem/0.1` |
| Executable differs from the pin | exit 2, `not executed` on stderr, no report | fetch the pinned version again; never update the pin to match an unreviewed build |

## Fixtures and tests

`script/cem-recipes_test.sh` (`make cem-recipes-test`) builds fixture repositories and runs every
row above, the accepted and complete paths, a step killed at `RECIPE_TIMEOUT=1`, a `TERM`-ignoring
step that must end within the 2-second grace, and a recipe interrupted with `TERM` while its step
ignores `TERM`, asserting that no step process survives. `CORVINT_BIN=$(command -v corvint)`
runs it against an installed release; without it the test builds `corvint` from the checkout.
The verifier is always built from the checkout's `interop/cem01-go`; the pinned public-proxy fetch
is not exercised.
