# SPDX-License-Identifier: Apache-2.0
# AFP-V0-531 (V1-2689): classifies a failed go-product-shard from its go test -json
# stream (read with -R -s; non-JSON lines are ignored). Each package with a
# package-level fail event (or a build-fail event) gets one typed reason:
# test-failures (its failed tests are named), build-failed, timeout, signal, panic
# or package-fail-no-test. Packages that started but never finished are listed as
# unfinished, and the FAIL/panic/exit lines of the whole output are kept, bounded.
def failLine: test("^\\s*(--- FAIL|FAIL\\b|panic: |fatal error: |exit status |signal: )");
split("\n") | map(fromjson? // empty) as $ev
| ($ev | map(select(.Action == "fail" and ((.Test // "") == "") and .Package != null) | .Package) | unique) as $failedPkgs
| ($ev | map(select(.Action == "build-fail" and .ImportPath != null) | .ImportPath) | unique) as $buildFailed
| ($ev | map(select(.Action == "fail" and ((.Test // "") != ""))) | map({Package, Test}) | unique) as $failedTests
| ($ev | map(select(.Package != null) | .Package) | unique) as $pkgs
| ($ev | map(select((.Action == "pass" or .Action == "fail" or .Action == "skip") and ((.Test // "") == "") and .Package != null) | .Package) | unique) as $finished
| ($pkgs - $finished) as $unfinished
| (reduce ($ev[] | select((.Action == "output" or .Action == "build-output") and .Output != null)) as $e ({};
    .[$e.Package // $e.ImportPath // "(build)"] += $e.Output)) as $out
| (($failedPkgs + $buildFailed) | unique | map(. as $p | ($out[$p] // "") as $o
    | ($failedTests | map(select(.Package == $p) | .Test)) as $t
    | {package: $p, tests: $t[:50], testCount: ($t | length), reason: (
        if ($t | length) > 0 then "test-failures"
        elif any($buildFailed[]; . == $p) or ($o | test("\\[(build|setup) failed\\]")) then "build-failed"
        elif ($o | test("panic: test timed out")) then "timeout"
        elif ($o | test("(^|\\n)signal: ")) then "signal"
        elif ($o | test("(^|\\n)(panic: |fatal error: )")) then "panic"
        else "package-fail-no-test" end)})) as $reasons
| ([$out | to_entries[] | .value | split("\n")[] | select(failLine)]) as $fails
| {reasons: $reasons, unfinished: $unfinished[:20], unfinishedCount: ($unfinished | length),
   failLines: $fails[:200], failLineCount: ($fails | length)}
