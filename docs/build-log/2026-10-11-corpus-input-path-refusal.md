# Corpus local input path refusal names the flag and rule (V1-1226)

Intent: proposed, pending owner acceptance (V1-1226; GitHub #796), `DCP-V1-054` in
`docs/specs/documentation-corpus-v1.md`. Baseline: `00dd220753973cd7eb332b3b7572db4a2ca60265`.

## Contract and implementation

`docs corpus` refused an absolute `--migration` with only `invalid local input path`, and an
automated caller retried the same refused command. `doccorpus.LocalInputPathRule` now names the
broken rule (absolute path, parent traversal, the root itself, non-canonical, empty, over 1024
bytes, non-UTF-8, backslash, NUL, tab, CR or LF); `validPath` delegates to it, so the admitted
set is unchanged. Before any read, the CLI checks each local input flag and refuses the first bad
one as `<flag>: local input paths must be canonical and relative to --root (<rule>)` under the
unchanged `corpus-refused` code and error shape. The reader's own refusal, still reached by the
MCP bridge and companions, keeps the `invalid local input path` prefix and adds the rule.
`corvint docs --help` lists the root-relative flags. `--page`, `--config` and `--receipt-input`
are excluded: the first two already have their own validation and messages, and the last names
a request input, not a file. Empty values keep their existing handling (argument error when
required, omitted when optional); review round 1 asked for that to be stated, not changed.

Non-goal: admitting absolute paths. That would widen the bounded reader's authority boundary.
Rollback: revert this change; no stored state or wire field changes.

## Verification

Focused tests: `TestDocsCorpusLocalInputPathRefusals`, `TestDocsCorpusHelpStatesRootRelativeInputs`,
`TestLocalInputPathRuleNamesTheBrokenRule`, plus the existing corpus CLI, Playwright producer, MCP
bridge, parity and republish packages.
