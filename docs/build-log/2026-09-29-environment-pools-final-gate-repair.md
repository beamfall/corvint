# Environment pools: final scoped gate repair

The enrolled #342 Tasks suite at `774a63b8` failed three checks. Pool preview had
unconditionally audited pool state even in a legacy queue without configured pools,
violating its existing generation-zero read contract. Pool audit now applies only
when pools are configured; pool-enabled previews retain their audit. The staging
cleanup test still used the former 2422-byte descriptor maximum; it now checks every
length through the accepted maximum and rejects maximum plus one. The spec index's
S9 delivery metadata was stale and now matches the spec and catalog.

The three failing regressions and the pool-enabled preview regression passed after
repair. This necessary gate repair follows two completed implementation review
cycles; it does not add another broad review cycle. The independent reviewer checked
the changed branches and bounds. Commit-bound scoped tests still run at closeout.
The six native qualification scenarios remain evidence for source `6955463d`,
not relabeled executions of the repaired source. The pool/process implementation
used by those scenarios is unchanged; the preview compatibility change has focused
regression evidence. Full repository validation remains NOT_RUN for this issue slice.
