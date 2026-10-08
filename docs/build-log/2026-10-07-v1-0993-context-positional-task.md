# 2026-10-07: `corvint context` accepts one positional task (V1-0993)

## Intent

Ticket V1-0993: `corvint context <text>` was refused with the argparse-style
`unrecognized arguments: <text>`, so agents had to retry with `--task`. They kept making this
mistake. The coordinator chose to accept exactly one positional as the task when `--task` is
absent. The requirement is TCP-V0-062 in `docs/specs/task-context-packet-v0.md`, proposed (V1-0993),
experimental.

## Decisions

- **Shape.** Exactly one positional is the task when `--task` is absent. `context TEXT` and
  `context --task TEXT` produce byte-identical stdout, stderr and exit status, because the
  positional fills the same `options.task` before the shared validation runs.
- **Refusals.** A positional given with `--task`, or two or more positionals, is refused with exit 2
  and `invalid-arguments`. The message names `--task` and ends with the example
  `corvint context --task "fix the parser"`. When neither form is given, the existing
  `the following arguments are required: --task` stays unchanged, and `batch_test.go` still pins it.
- **Precedence.** `context grep|defs|refs` is still the lookup surface, because its parser runs
  first. A positional counts as a task for the `--expand` exclusion. Under `--corpus`, the corpus
  search query comes from the parsed task, so a positional searches the same way `--task` does.
- **Compatibility.** CCF-V1-006 classifies this change as additive. `context` is not one of the
  `cli-parity-v0` byte-pinned outputs (`query`, `impact`, `init`, `adopt`). The change only accepts
  input that used to be refused. No packet member, CCF-V1-002 identifier or registered enumeration
  value changes. No goldens or decision files changed.
- **Help.** `cmd/corvint/help.go` is unchanged. The parallel V1-0945 lane owns it, and the
  `context` usage line there still shows only the `--task` form. Mentioning the positional form in
  help is left as follow-up work.

## Evidence

`TestTaskContextPositionalTaskMatchesTheFlagBytes` and
`TestTaskContextRefusesAmbiguousPositionalTasks` in `cmd/corvint/taskcontext_positional_test.go`.

## Rollback

See the TCP-V0-062 paragraph under Rollback in the spec. The change persists no state.
