<!-- SPDX-License-Identifier: Apache-2.0 -->
# Driver computed-invocation sites narrowed for CI steps (V1-5000)

The driver scanner marked many CI jobs as `driver:computed-invocation` sites. Every such site, and
everything that depends on it, is selected for any dirty path. This lane adds AFP-V0-9620 to
AFP-V0-9627 to `docs/specs/affected-zero-unknown-v0.md`. It records an internal reason for each
computed site, then narrows only the forms that provably cannot change what a later command runs
or reads. When it is unsure, the site stays computed. There is no wire, witness or receipt change.

## Computed-site reasons before and after

These are taken from the repository at base `b9b2b8785`, using the opt-in census
`TestDriverComputedReasonCounts` with `CORVINT_DRIVER_REASONS_ROOT`. A site can carry several
reasons. The `sole` column counts sites with only that reason. Total computed sites fell for CI
(35 to 18) and stayed the same for make (136 to 136) and actions (0 to 0).

| reason | make before | make after | ci before | ci after | sole before | sole after |
|---|---|---|---|---|---|---|
| `go:list-unproven-output` | 0 | 0 | 2 | 2 | 0 | 0 |
| `make:evaluated-uncertified` | 69 | 69 | 0 | 0 | 69 | 69 |
| `make:recipe-ambiguous` | 6 | 6 | 0 | 0 | 1 | 1 |
| `shell:assignment` | 65 | 65 | 32 | 14 | 67 | 64 |
| `shell:builtin-export` | 0 | 0 | 4 | 4 | 0 | 0 |
| `shell:builtin-read` | 0 | 0 | 2 | 0 | 1 | 0 |
| `shell:builtin-set` | 1 | 0 | 19 | 2 | 2 | 0 |
| `shell:computed-command-word` | 0 | 0 | 7 | 7 | 0 | 2 |
| `shell:interpreter-inline` | 3 | 3 | 2 | 2 | 1 | 1 |
| `shell:local-command-word` | – | 0 | – | 8 | – | 2 |
| `shell:github-env-file` | – | 0 | – | 1 | – | 0 |
| `workflow:github-env-file` | – | 0 | – | 1 | – | 0 |

The make site that lost `builtin-set` is still computed for other reasons.

`shell:local-command-word` is the new check that keeps a narrowed local from hiding a command.
For example, `docs-plan` assigns `tool="$PWD"` and later runs `$tool/...`, so that step stays
computed.

## Selection on a one-file change

A synthetic clone of `b9b2b8785` gets one commit that adds a single
`.corvint/changes/*.cem.json` file. The test runs `corvint --root <clone> affected --base
b9b2b878569b1bd3b52d30992d7cda1216f2d72a --format json`, with binaries built before and after.

| | before | after |
|---|---|---|
| summary | BOUNDED, dirty 1, selected 211, excluded 678, unknown 0 | BOUNDED, dirty 1, selected 206, excluded 678, unknown 0 |
| `FRONTIER_SITE` CI / make | 10 / 103 | 5 / 103 |
| driver `DEPENDENCY_PATH` CI / make | 30 / 42 | 30 / 42 |

Five CI jobs are dropped:
- `cache-prune.yml#cache-prune`;
- `ci-control-plane.yml#merge-group`;
- `ci-route-health.yml#routes`;
- `ci.yml#artifact-integrity-record`;
- `ci.yml#static-reuse`.

Their only computed forms were `set -euo pipefail`, `read -r` and lowercase locals holding
temporary paths or API strings. Each was checked by hand. The other witnesses are unchanged. The
30 CI `DEPENDENCY_PATH` jobs reach computed make targets, so make is the next lever.

## Dynamic arguments

The remaining CI sites use dynamic arguments, which are fail-closed and not narrowed here:
- a dynamic `go` package pattern becomes `...`;
- a dynamic `make -C` directory is unbounded, and a dynamic make target is computed;
- `cd $x` leaves the working directory unknown;
- a dynamic file operand is never recorded and leaves its command uncertified (AFP-V0-9051).

An uncertified CI read carries no frontier claim. That is existing policy, the same as for an
environment variable, and this lane does not change it.

## Forms left computed

These forms are left computed deliberately:
- **Make recipe assignments.** GNU make exports command-line variables to recipe shells.
- **Composite-action steps.** The caller's environment is unknown.
- **Names a tool may read.** This covers uppercase, special and env-declared names.
- **Prefix assignments on a command.**
- **Multi-line `$(...)` values with `\` continuations.** The scanner misreads these as prefix
  assignments, which fails closed.
- **Other `set` and `read` forms.** This covers `set -a`, `set --` and `set -f`, and `read -a`,
  `read` without `-r` and `IFS=,` reads.
- **Writes to `GITHUB_ENV` and `GITHUB_PATH`.**
- **Assigning and arithmetic expansions.**
- **`printf -v`, `mapfile`, `let`, `hash`, `alias` and related builtins.**
- **Assignments inside `zsh`, `ksh` or other non-bash `-c` scripts.**

One assumption remains: the runner exports no other lowercase variable that a tool interprets.
Shell functions and aliases that shadow builtins are not modelled. The step would have to define
them itself, and `alias` already stays computed.

## Verification

- `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run
  'Driver|Workflow|Make|Shell|Computed|Frontier|Action|AZU|AFP' ./internal/liveverify/affected/`
  passes.
- `gofmt -l`, `go vet` for the package and `make windows-vet` are clean.
- `make gate` was not run, under the lane charter.

Rollback is to revert the lane commit. That restores every CI assignment, `set` and `read` as a
computed site, which only widens selection.
