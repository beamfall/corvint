<!-- SPDX-License-Identifier: Apache-2.0 -->
# heavy.sh's host-only ENVFILE source is bounded (V1-3360)

Base `8a1935c528bc314057d15e7db7c92f7c36384e01`. Requirements AFP-V0-3340 to AFP-V0-3342
(`docs/specs/affected-plan-selection-v0.md`, section "Host-only shell environment source").

## Problem

The remote half of `script/heavy.sh` sourced `"${ENVFILE/#\~/$HOME}"`, the `HEAVY_REMOTE_ENV`
value carried in the generated metadata. AFP-V0-2953 correctly keeps an environment-named source
dynamic, so heavy.sh was an unbounded reader (`shell:dynamic-source`) and every unit and make
target whose chain reaches it was selected for any edit. This corrects the 2026-10-30 V1-3160
entry (`2026-10-30-v1-3160-frontier-scoping.md`), whose "Measured outcome" read as if heavy.sh
were already bounded: TSP-V0-004 bounded only its fresh `mktemp -d` metadata source
(`$w/meta.env`); the ENVFILE source stayed dynamic until this change.

## Decision

- Runtime refusal (AFP-V0-3340). Before sourcing, the remote half resolves the file's real path
  with `command -p perl` (`Cwd::realpath`) and refuses with its infra error (97) when the file is
  not a regular file, holds CR/LF in its path, lies inside any Git checkout (a `.git` file, link or
  directory in any ancestor), or lies at or below the caller checkout (new `CHECKOUT` metadata
  field), the remote clone or the remote worktree area (`$ROOT/wt`). The validator is the source
  operand itself, so a refusal prints nothing and `.` fails on the empty operand; no separate guard
  can be skipped or its result reassigned. `HEAVY_REMOTE_ENV` is documented as host-only
  configuration.
- Structural proof (AFP-V0-3341). The shell analyzer bounds a source only when its sole operand is
  byte-for-byte the checked validator (`hostEnvironmentOperand` in
  `internal/liveverify/affected/shell/host_source.go`), with no prefix assignment, redirection or
  extra operand, outside a substitution or command string, in a file that defines no `command`,
  `.` or `source` function, runs no `alias`, `hash`, `enable` or `eval`, and holds no other
  source anywhere except a TSP-V0-004 fresh-directory one (text position is not execution order
  for function bodies). The rule is keyed on the validator, not a file name: the same form in
  another script is admitted, and any changed guard keeps `shell:dynamic-source`.
- Records (AFP-V0-3342). `.corvint/unbounded-readers.json` drops the ten shell tests that were
  on the floor only through heavy.sh; `script/dogfood-cache_test.sh` keeps its own
  `shell:runs-toolchain` floor and `script/lane-scripts_test.sh` keeps its recorded reason.

Rejected: a file-name exemption for heavy.sh, and a declaration file trusting the source; neither
proves what the operand can name.

## Measurement

Comment-only probe: append `// V1-3360 measurement probe` to `internal/tasks/journal/anchor.go`,
run `corvint affected --base HEAD --format json` with a binary built outside the checkout, then
`git checkout` the file. Baseline: `origin/main` binary in a clean worktree at the base; candidate:
this change's binary on its own commit.

| Metric | Before | After |
|---|---:|---|
| `driver:make` units whose witness cause is `shell:script/heavy.sh` | 31 | 0 |
| any unit whose witness cause is `shell:script/heavy.sh` | 41 | 0 |
| `driver:make` units selected | 145 | 135 |
| units selected (scope stays `UNKNOWN`) | 290 | 270 |

Of the 41 former heavy.sh-witnessed units, 20 left the plan (10 make targets, among them
`analyzer-schema-test`, `codex-review-test`, `dogfood-cache-test`, `shell-test`, and 10 shell
tests). The other 21 stay selected under independent witnesses: 19 `FRONTIER_SITE`, one
`DEPENDENCY_PATH` and one through the `tools/test-commands-run` unbounded reader. The ticket's
earlier figure of 45 make units was measured on an older base. The plan remains `UNKNOWN`
(65 language-frontier unknowns), so this is not complete coverage.

## Evidence

- `TestHostEnvironmentSource_AFPV03341` (17 cases: bounded guarded, `source` and in-function forms;
  dynamic for unguarded, other, prior and later sources, removed realpath/checkout/root checks,
  unquoted, suffixed, `command` shadow, alias, eval, prefix assignment, redirection).
- `TestRepositoryScriptsAreBoundedSources_V14283`: heavy.sh is bounded, and a copy with
  `realpath(shift @ARGV)` mutated is `shell:dynamic-source` again.
- `script/lane-scripts_test.sh --heavy-host-env-only`: 16 cases; caller-checkout, clone and
  worktree-area files, each direct and through a symlink, return remote 97 with no side effect;
  an external file reached through a symlink, sharing the checkout path prefix, still loads.
  A mutation that disables the checks fails the first refusal case.
- `make unbounded-readers-check` exit 0 (its pre-existing `internal/tasks/cli` notice is
  unchanged from the base); `go vet`, `gofmt -l`, shellcheck (no new findings).
- Real offloads to the WSL host during this work sourced its existing `HEAVY_REMOTE_ENV` through
  the new guard.

## Residual risks

- The operand is byte-exact; any edit to the validator in heavy.sh (even whitespace) returns
  heavy.sh to `shell:dynamic-source` until `hostEnvironmentOperand` is updated with it. That is the
  fail-closed direction.
- A host whose `HEAVY_REMOTE_ENV` lives inside a Git checkout (for example a home directory under
  dotfile version control) is now refused with exit 97 and the run falls back to local.
- The remote needs `perl` on the default `command -p` PATH; heavy.sh's remote half already runs
  perl for its slot semaphore.
- The proof assumes the trusted host tool model: it does not defend against concurrent hostile
  replacement of the file between the check and the source, and does not bound what the host
  configuration itself executes once loaded.

## Rollback

Revert this commit: the analyzer rule, the checked operand, the `CHECKOUT` metadata field and the
restored unbounded-reader records go back together.
