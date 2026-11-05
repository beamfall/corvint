# Native coverage self-review repair

Owner request: repair only the two P2 findings in lane covlive, mandated base
`39c03198086681fb24f509fbd00b34696d3268c4`. Governing requirements are
LPCV-V0-8205, 8207, 9401 and the macro/static fallback in LPCV-V0-953.

The supplied review found that the JVM adapter reset process instrumentation on each EXEC.
PID 1147 in the original `final-live/jvm-2116917343/jvm-1.trace` executed an uninstrumented
jspawnhelper and then Java; the final Java image incorrectly upgraded the whole process.
Those original JVM completeness/index and mutation-acceptance claims are withdrawn. The
original bytes remain outside the repository for reproduction.

Decision: retain every image's instrumentation/export result in the adapter and emit the
conjunction through the unchanged process capture fields. No later image may erase a gap
or an earlier export. Repeated Java images with only PID-scoped sessions remain incomplete
because those sessions cannot independently attest each image. `TestLiveImageHistory`
executes the same Python reducer used by the live adapter and feeds its capture through
process coverage admission. It covers launcher-to-Java, Java-to-launcher, ambiguous repeated
Java images, no EXEC, and a single-image positive control. For the positive live project,
explicitly request the JDK FORK mechanism so the child is directly Java; this is a narrower
witness, not qualification of default POSIX_SPAWN or permission to ignore its launcher.

The second finding showed that body containment did not establish edit eligibility: a C
#define inside one function can change another function without intersecting its coverage.
Decision: require a separate source-provider classification of the complete hunk list,
bound to both exact blob OIDs, before candidate line intersection. Missing/stale classification
falls back. Classification must rule out macros, declarations and structural edits across both
revisions. No general native classifier is claimed or synthesized from coverage boundaries.
`TestNativeEditClassification` retains a cross-function macro/consumer fixture, structural
fallback, unavailable classification, stale base/target/hunks and a classified constant
control. Existing selection assertions remain intact; their known fixture edits now carry
explicit classifications. Live mutants validate literal return replacements and bind the
actual before/after bytes. All native packages remain unordered for production selection.

Initial focused whole-package verification through the configured `script/heavy.sh` passed
(0.378s). The preceding compile failure in the new test's profile byte literals is retained
as `self-review-repair/test-native-initial.log`, alongside the passing repair log. Terminal
live receipts, commands, digests and final CEM/check results will be retained in the external
handback; any later measurement addendum binds its code revision explicitly.

No frozen capture/index format, field, error/exit code, committed stderr, golden or existing
test assertion changes. No new requirement IDs, production exclusion or ticket-completion
claim. Existing external qualification blockers remain PARTIAL. Independent review is reserved
to the orchestrator. Rollback removes these repair inputs and adapter changes together; retain
the review's withdrawn evidence status and keep native exclusion disabled.
