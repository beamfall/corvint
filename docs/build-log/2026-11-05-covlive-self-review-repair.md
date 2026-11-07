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

## Terminal measurement addendum

Code revision: `8015a152c09f9d8ce515227d478892c51fcc6aaf`; subsequent edits in this
addendum change documentation only. The affected plan was captured before each test command.
Whole-package `script/heavy.sh go test -v -count=1 -timeout 30m ./internal/nativelinecov`
with installed live tools and the source commit supplied passed in **38.627s** (wrapper 39.2s;
6s load admission, no slot queue). Every existing test and both new regressions ran. .NET
retained its existing explicit NuGet-Migrations host-mutex skip. Native package vet, spec
projection, requirement collisions/capacity, declared read scopes and line-citation checks passed.

The first live attempt failed in **35.473s**: Swift's observer returned 125 before COMPLETE
and before any native event report. Its original trace/stdout/stderr/capture and failed log
are retained. No assertion or observer code was changed. The one whole-package retry passed;
the exact cause of that transient observer failure is NOT_OBSERVED.

Replacement raw evidence lives under `/home/russell/lanes/covlive-evidence/self-review-repair/`.
The terminal `live-retry` JVM and cmocka projects each retain two source-bound records, an
unordered package, seven native invocations, and four mutants: selected sizes 1,1,0,1,
three kills, one zero-hit negative control and zero misses. Independent Git-blob comparison
matched all five indexed source files to the named code revision. All seven JVM traces
contain only direct Java executable images, retain COMPLETE, and use explicit FORK.
Rust/Swift each execute three invocations but still refuse complete launcher coverage records.
LPCV-V0-963 and qualified general native classification remain NOT_RUN/NOT_PRODUCED.

Replaying the seven original default JVM traces through the repaired adapter marks five
invocations incomplete (including PID 1147 in invocation 1). This is replay of original
observations, not seven new executions. It does not repair the old index's acceptance claim.
The replacement receipt inventory covers **149 files / 892144 bytes**, including replacement
live evidence and original-event replays; all raw bytes remain outside the repository.

| Receipt relative to the external repair directory | SHA-256 |
|---|---|
| `live-retry/jvm-3098271079/records.index` | `df3ba43393eaffafb8b254343030c08cc7f3c9b5978c4fd21b17073149e4ce17` |
| `live-retry/cmocka-561054515/records.index` | `d86e4d4ba937eaf2b38043d36a8ca71d761c1de0e6f53e839d08565ed84f8758` |
| `live-retry/jvm-3098271079/jvm-mutants.json` | `6333ba0f415b7767dd9990daf2f22366628ad28a643fde66abc1bd6119f51afa` |
| `live-retry/cmocka-561054515/cmocka-mutants.json` | `24709490b3ca09cdc8a24443aa4c3b4f68d0d245783c696f3a6eeb904a3f5685` |
| `default-jvm-replay/summary.json` | `b5a7641e427c52b8dccc1890600ec3052474efcd743cbcdbf51bac86a18141a4` |
| `test-native-terminal-retry.log` | `d3e2b6e9de19fe5f0be58aa89f7c6e100331ce63725b469b58e4c409c1ddc709` |

The local outcome remains blocked by the pre-existing external qualifications, not by either
P2 repair. The final handback retains the rebind/check disposition and every NOT_PRODUCED
reason; no seal or native task completion is performed.
