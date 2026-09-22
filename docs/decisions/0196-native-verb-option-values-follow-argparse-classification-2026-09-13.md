# Decision 0196 — native verbs' option values follow the argparse classification

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

A `cmd/corvint` bug hunt (2026-09-13) found that decision 0173 guarded every parser that existed in
the retired oracle but missed seven Go-native parsers: `parseDocsInvocation`,
`parseDocsMaintainInvocation`, `parseWitnessOptions`, `parseTestValidityOptions`,
`parseFrontierOptions`, `parseWorkOptions` (`propose-wave`), and `localCompletionFlags` (the `dogfood`
local-completion actions). Each still consumed the next token as a value unconditionally, so
`parseWitnessOptions([--base -x])` bound the base revision `-x`, and
`localCompletionFlags([begin --plan -x])` bound the plan `-x`. `GPK-V0-064` says "every `cmd/corvint` command
parser", so these are contract violations, not new scope.

The call:

(a) Amend `GPK-V0-064` to name the seven parsers; no new requirement ID. Each gains the same
one-line guard, `|| argparseOptionLike(next)`, ahead of its value consumption.

(b) The refusal reuses each parser's own existing missing-trailing-value error, unchanged:
`missing docs argument value: --X`, `missing docs maintain argument value: --X`,
`missing value for --X`, `invalid-frontier-input`, `missing work argument value`, and
`local-completion-option-value-required`. The clause's `argument --X: expected one argument`
wording is not imposed on them. These verbs never existed in the oracle, so there is no argparse
message to reproduce, and `frontier` (`CF-V0-024`, code only, no message) and `work` (stdout
`MALFORMED_INPUT`) have wire contracts that forbid that message. This keeps the clause's "no new
message" rule.

(c) Unchanged: inline `--opt=-x`, negative numbers, and every other refusal.

Consequences: `TestNativeVerbOptionValuesFollowArgparseOptionLikeClassification`
(`cmd/corvint/help_test.go`) runs `-x`, `--help`, and `-h` as bare values through all seven parsers,
plus the inline and negative-number forms. All seven subtests failed before the guard.

Rollback: revert the commit. That restores unconditional next-token consumption in the seven
parsers and the pre-amendment `GPK-V0-064` text and traceability row.
