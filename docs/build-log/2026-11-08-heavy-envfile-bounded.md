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

## Fix round 1 (independent review, five P2 findings)

- Guard rewrites the analyzer could not see: `hostEnvironmentSource` now also refuses, anywhere in
  the file, a `trap` other than a listing, a reset or an EXIT/INT/TERM/HUP/QUIT handler whose text
  lexes to definition- and assignment-free commands that each run a file-defined function, an inert
  builtin or a literally named external command (a DEBUG trap could redefine `command` before the
  source line); any mention of `command_not_found_handle`; and any `PERL`-prefixed variable name
  longer than `PERL` (`PERL5LIB`, `PERL5OPT`, `PERLLIB`, other `PERL5*`), plus `LD_`/`DYLD_` loader
  names, which are the same class (perl loading chosen code). Mentions are matched in the raw text
  so assignments built in strings or prose also fail closed. heavy.sh's own EXIT, INT/TERM/HUP and
  reset traps pass the narrower rule, so the real script stays bounded.
- Hard links: the validator now refuses a file whose link count is above 1, since a path check
  cannot see a second name inside a checkout. heavy.sh and `hostEnvironmentOperand` carry the same
  new line byte for byte.
- `script/lane-scripts_test.sh --heavy-host-env-only` adds two refusals with their expected reason:
  a file in an unrelated `git init` directory outside every excluded root (the `.git` walk on its
  own) and a hard link outside every checkout to a checkout file (the link-count rule on its own).
- `.corvint/unbounded-readers.json`: the `script/lane-scripts_test.sh` reason no longer claims a
  dynamic HEAVY_REMOTE_ENV source; only the sourced temporary copy remains.
- AFP-V0-553 no longer claims the two heavy.sh tests stay selected for an unrelated edit; AFP-V0-3340
  to 3342 state the new refusals and the hard-link rule.

Evidence (fix round): `TestHostEnvironmentSource_AFPV03341` 32 subtests pass (15 new: inert traps
admitted; DEBUG, RETURN, signal-rewrite, EXIT-source, eval, dynamic-command and assignment traps,
four PERL variable forms, both `command_not_found_handle` forms and a removed link check refused);
`TestRepositoryScriptsAreBoundedSources_V14283` passes on the real heavy.sh; the host-environment
lane test reports 18 cases.

Residual: a variable name assembled at run time (for example `declare "$name=..."` or a nameref) is
not matched by the text scan; heavy.sh has no such construct, and the proof remains a static source
path proof under the trusted host model stated above.

## Fix round 2

Two confirmed P2 review findings could admit a bounded host source while executing code chosen
elsewhere: arbitrary literal external commands in trap strings, and quoted/escaped names evading
the raw hijack scan. The trap rule now admits only `git`, `rm`, `/bin/rm`, `rmdir` and `/bin/rmdir`
as external verbs. Git accepts only `-C <dir>` global options and the `update-ref` subcommand;
command runners are refused without unwrapping. Independent review caught a further option-injection
path through expanded `-C` directory operands; these now accept only literal paths or heavy.sh's
exact single-word quoted form, with regressions for variable, array and glob expansion. Both the not-found handler and environment-hijack
checks match raw text and a second form with backslash-newline continuations removed first, then
all single quotes, double quotes and backslashes deleted.

The real heavy.sh EXIT trap also contained `chmod`. Its run-directory cleanup now lives in the
file-defined `heavy_cleanup_run_tmp` function, whose body the analyzer scans, preserving cleanup
and its redirected output without widening the external trap allowlist. Regression cases cover
the reported command runners, Git configuration options/subcommand, split/escaped/quoted and
continued PERL names, the split not-found handler, and both actual heavy.sh trap forms.

The focused regression initially rejected the real trap after the Git directory restriction:
`unquote` had erased its inner double quotes. Literal single-quoted handlers now retain their
inner quoting; other encodings with expansion/escape syntax fail closed. Independent delta review
found no remaining findings after this correction. The existing static-proof and trusted-host
limits remain; runtime-computed variable names are still outside the text match.

Verification (final code):

- `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'Source|Dynamic|Bounded|Unbounded|Fresh' ./internal/liveverify/affected/shell/`: PASS on the WSL remote.
- `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'TestRepositoryScriptsAreBoundedSources_V14283|UnboundedReaders|DynamicSource' ./internal/liveverify/affected/`: PASS; heavy.sh selected Darwin-local execution for Darwin-specific files. The real heavy.sh remains bounded.
- `script/lane-scripts_test.sh --heavy-host-env-only`: PASS, 18 cases.
- `make unbounded-readers-check`: PASS; retains the existing `internal/tasks/cli` extra-record notice.
- `make spec-requirements`: PASS, no generated changes; `make spec-requirements-check`: PASS.
- `GOTOOLCHAIN=local go vet ./internal/liveverify/affected/shell/`: PASS.
- `gofmt -l internal/liveverify/affected/shell/host_source.go internal/liveverify/affected/shell/host_source_test.go`: PASS, no output.

The sandbox denied creating the worktree Git `index.lock`, so staging in the real index and the
requested commit remain blocked. Spec generation/checking used a temporary index/object directory
under `/tmp`, with the changed spec staged there; no repository Git state was overwritten. Core
context/affected ran with the installed build 133's `self-binary-stale` diagnostic retained;
`corvint-update check --component core --allow-network` failed process-quiescence verification
(`operation not permitted`), so freshness is UNKNOWN. Tasks commands were explicitly excluded.
