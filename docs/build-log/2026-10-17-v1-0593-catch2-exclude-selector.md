# V1-0593 review finding: Catch2 `exclude:` selector (2026-10-17)

Finding (Codex review of V1-0593, verified by the orchestrator): after TRE-V0-046 the
`catch2` plan joins exact selectors into one comma-joined quoted OR argument. The
refusal list did not cover Catch2's `exclude:` prefix, which Catch2 3.16.0 interprets
even inside quotes. `["pass","exclude:fail"]` became `"pass","exclude:fail"`, meaning
`pass OR NOT fail`, and ran unrelated tests. Before TRE-V0-046 the arguments were ANDed,
so the selection narrowed.

Decision: `Build` refuses any Catch2 selector whose case-insensitive prefix is
`exclude:` with the existing "ambiguous Catch2 selector" error. TRE-V0-046 already
promised unchanged refusal of exclusion selectors; its text now names `exclude:`.
No other in-quote keyword was confirmed (the Catch2 source was not found under
/private/tmp/cem10-build/native-2026-10-17), so none was added; this is NOT_OBSERVED.

Evidence: `TestCatch2RefusesExcludePrefix` fails before the fix and passes after.
