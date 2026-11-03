# spc-w47 full-precheck repairs (V1-4331, V1-4332)

The owner's third repair request supplies full-precheck failures against
`e65ad331c2ccdec039eba845d2e9827844067bcb`, with frozen lane base
`b486d3978728731c6638b50fa2210dfbb6e365ea`. Generation resolved an inherited attribution BASE
unconditionally, so a lane revision absent from disposable fixture repositories prevented both
superseded-spec projection and the lane spec-fixture lifecycle. The shell caller completeness
test also retained an unmodelled invocation frontier on the batch precheck itself.

Generation now resolves BASE only for over-cap attribution. An unavailable revision leaves the
original file/count/cap refusal intact and explicitly records unavailable BASE and NOT_OBSERVED
added IDs. Capacity still refuses an invalid supplied revision. SRG-V0-745 records the distinction;
the requirement IDs, OCM bound, generation counts and capacity counts remain stable. Shell and Go
capacity fixtures remove inherited BASE and SPEC_REQUIREMENTS_BASE before setting fixture-owned
revisions. The superseded-spec test explicitly supplies an unavailable lane BASE. Generator
regressions exercise both environment variable names for successful generation, capacity refusal
and preservation of the over-cap diagnostic.

The new batch static check invoked a generator through an override-derived absolute root. Under
TSP-V0-960 this is a computed invocation in the caller itself; its otherwise complete TSP-V0-005
foreign-caller declaration cannot remove that frontier. The precheck already changes directory
to its root before static checks, so invoking the generator by literal repository-relative path
models the edge and preserves BATCH_PRECHECK_ROOT fixture behavior. No declaration or test is
weakened. The unchanged repository caller completeness test supplies the regression.

Rollback is the generation fallback, fixture environment isolation, literal invocation and
SRG-V0-745 clarification together; reverting restores the observed precheck failures. No new
capability, requirement ID or product boundary is introduced. The owner's supplied precheck
findings and earlier independent review remain evidence; no fresh independent review is claimed.
Exact focused results, temporary group-23 harness details and integration limits are retained in
`spc-w47.fix3.md`. Owner prohibitions prevent repository commits, publication, native Tasks
completion and post-commit dogfood rebinding; these remain NOT_PRODUCED/NOT_RUN.
