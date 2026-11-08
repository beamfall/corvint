<!-- SPDX-License-Identifier: Apache-2.0 -->
# Driver computed-invocation sites narrowed for CI steps (V1-5000)

The driver scanner marked many CI jobs as `driver:computed-invocation` sites. Every such site, and
everything that depends on it, is selected for any dirty path. This lane adds AFP-V0-9620 to
AFP-V0-9628 to `docs/specs/affected-zero-unknown-v0.md`. It records an internal reason for each
computed site, then narrows only the forms that provably cannot change what a later command runs
or reads. When it is unsure, the site stays computed. There is no wire, witness or receipt change.

## Computed-site reasons before and after

These are taken from the repository at base `b9b2b8785`, using the opt-in census
`TestDriverComputedReasonCounts` with `CORVINT_DRIVER_REASONS_ROOT`. A site can carry several
reasons. The `sole` column counts sites with only that reason. "Head 1" is the first lane head
(`623a28375`); "repaired" is after review round 1. Total CI computed sites went 35 (base), 18
(head 1), 34 (repaired). Make stayed at 136 and actions at 0 throughout.

| reason | make base | make repaired | ci base | ci head 1 | ci repaired | sole base | sole repaired |
|---|---|---|---|---|---|---|---|
| `go:list-unproven-output` | 0 | 0 | 2 | 2 | 2 | 0 | 0 |
| `make:evaluated-uncertified` | 69 | 69 | 0 | 0 | 0 | 69 | 69 |
| `make:recipe-ambiguous` | 6 | 6 | 0 | 0 | 0 | 1 | 1 |
| `shell:arithmetic-context` | – | 0 | – | – | 7 | – | 0 |
| `shell:arithmetic-substitution` | – | 1 | – | – | 0 | – | 0 |
| `shell:assignment` | 65 | 65 | 32 | 14 | 32 | 67 | 59 |
| `shell:builtin-export` | 0 | 0 | 4 | 4 | 4 | 0 | 0 |
| `shell:builtin-read` | 0 | 0 | 2 | 0 | 2 | 1 | 0 |
| `shell:builtin-set` | 1 | 1 | 19 | 2 | 16 | 2 | 0 |
| `shell:builtin-shopt` | 0 | 0 | 0 | 0 | 1 | 0 | 0 |
| `shell:computed-command-word` | 0 | 0 | 7 | 7 | 7 | 0 | 0 |
| `shell:function-definition` | – | 0 | – | – | 5 | – | 0 |
| `shell:github-env-file` | – | 0 | – | 1 | 1 | – | 0 |
| `shell:github-output` | – | 0 | – | – | 7 | – | 0 |
| `shell:interpreter-inline` | 3 | 3 | 2 | 2 | 2 | 1 | 1 |
| `shell:local-command-word` | – | – | – | 8 | – | – | – |
| `shell:local-referenced` | – | 0 | – | – | 23 | – | 0 |
| `shell:loop-variable` | – | 0 | – | – | 10 | – | 0 |
| `workflow:github-env-file` | – | 0 | – | 1 | 1 | – | 0 |

Every repaired site that carries a new reason (`local-referenced`, `github-output`,
`function-definition`, `loop-variable` and so on) was already computed at base. Revocation marks
each narrowed reason too, so `shell:assignment` and `shell:builtin-set` return almost to their
base counts. The one CI site left uncomputed is `ci-control-plane.yml#merge-group`. Its steps
use only `set -euo pipefail` and uppercase env-block names.

## Selection on a one-file change

A synthetic clone of `b9b2b8785` gets one commit that adds a single
`.corvint/changes/*.cem.json` file. The test runs `corvint --root <clone> affected --base
b9b2b878569b1bd3b52d30992d7cda1216f2d72a --format json`, with binaries built at base, at head 1
and after the repair.

| | base | head 1 | repaired |
|---|---|---|---|
| selected (BOUNDED, dirty 1, unknown 0) | 211 | 206 | 210 |
| `FRONTIER_SITE` CI / make | 10 / 103 | 5 / 103 | 9 / 103 |
| driver `DEPENDENCY_PATH` CI / make | 30 / 42 | 30 / 42 | 30 / 42 |

Head 1 dropped five CI jobs:
- `cache-prune.yml#cache-prune`;
- `ci-control-plane.yml#merge-group`;
- `ci-route-health.yml#routes`;
- `ci.yml#local-authority-deps`;
- `ci.yml#static-reuse`.

It also moved `ci.yml#artifact-integrity-record` from `FRONTIER_SITE` to `DEPENDENCY_PATH`. The
first version of this entry named `artifact-integrity-record` in place of
`local-authority-deps`, which was wrong. The repair keeps only `merge-group` dropped. Each of the
other jobs expands a narrowed local again, writes `GITHUB_OUTPUT` or uses an arithmetic test, and
the repaired scanner cannot prove such a value bounded. The 30 CI `DEPENDENCY_PATH` jobs reach
computed make targets, so make is still the next lever.

## Review round 1 repair

The Codex sol review of head 1 returned CONCERNS, with 7 P1 and 2 P2 findings. Each was checked
against source and confirmed. The repair follows the review's governing rule: when the scanner
cannot prove a value bounded, the site stays computed. It does not model the shell more
precisely.

| finding | counterexample | disposition |
|---|---|---|
| 1 `read` value used later | `read -r dir < chosen-dir; cat "$dir/payload"` | Any other whole-word occurrence of a narrowed local revokes the script (`shell:local-referenced`). |
| 2 `+=` fragment as path | `d=foo; d+=bar; cat "$d/payload"` | Value path recording removed; the reference rule covers it. |
| 3 substitution in `${...}` | `read -r d < chosen-dir; : "${unset_name:-$("$d/run")}"` | Computed `shell:brace-substitution`; `$((...))` gets `shell:arithmetic-substitution`. |
| 4 function shadows builtin | `function read { "$2/run"; }; read -r tools` | Any function definition revokes (`shell:function-definition`); `alias` stays computed. |
| 5 source before `set -x` | `. ./ci/setup.sh; set -x; :` | `source` or `.` revokes (`shell:sourced`); `eval` stays computed; startup env names disable narrowing. |
| 6 inline shell options | `bash -ac 'read -r npm_config_userconfig < chosen-config; npm ci'` | Inline scripts are opaque unless exactly `sh -c` or `bash -c`; a step `shell` with extra options leaves the environment unknown; `set -a` stays computed. |
| 7 `GITHUB_OUTPUT` carry | `read -r dir < chosen-dir; echo "dir=$dir" >> "$GITHUB_OUTPUT"`, then `${{ steps.choose.outputs.dir }}/run` | A narrowed script naming `GITHUB_OUTPUT` revokes (`shell:github-output`); the AFP-V0-9626 claim is corrected. |
| 8 runner-exported names (P2) | `npm_config_*` | `npm_`, `yarn_`, `pnpm_` and `proxy` names abstain; the residual is documented. |
| 9 rollback wording (P2) | – | The rollback now says a revert restores prior behaviour and removes the new guards. |

Each counterexample is a test in `TestDriverReviewCounterexamples_AFP_V0_9628` and
`TestDriverWorkflowCounterexamples_AFP_V0_9628`. `TestDriverReviewCounterexamplesSelected_AFP_V0_9628`
runs them end to end through `affected.Select` and checks that each job is a `FRONTIER_SITE`
while a plain `set -euo pipefail` job is left out. As a mutation check, `settleNarrowing` was
disabled. All three tests then failed on each counterexample that revocation guards (findings 1,
2, 4, 5 and 7). The other cases are guarded by separate checks and pass either way:
- brace substitution;
- inline and step shell options;
- tool-configuration names;
- `alias`, `eval` and `set -a`.

A residual from before this lane remains. A non-literal `GITHUB_OUTPUT` write with no narrowed
local, such as `echo "x=$(cmd)" >> "$GITHUB_OUTPUT"`, read later as
`${{ steps.x.outputs.y }}/run`, is admitted by the root-like `$X/` exemption in
`driverComputedWord`. That is the base scanner's consumer-side behaviour, and this lane leaves
it unchanged.

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

One assumption remains. The runner, or an earlier `uses:` action, is assumed to export no other
lowercase variable that a tool interprets. `npm_`, `yarn_`, `pnpm_` and `proxy` names abstain.

## Verification

- `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run
  'Driver|AFP|AZU|Shell' ./internal/liveverify/affected/` passes.
- `gofmt -l`, `go vet` for the package and `make windows-vet` are clean.
- `make gate` was not run, under the lane charter.

Rollback is to revert the lane commits. That restores the scanner's prior behaviour. Every CI
assignment, `set`, `read` and glob `shopt` becomes computed again. The guards these commits added
are removed as well, so a revert can narrow those forms back to their earlier treatment:
- `printf -v` and the other variable-setting builtins;
- substitution inside `${...}` and `$((...))`;
- uppercase or env-declared loop variables;
- environment files named through an expression.
