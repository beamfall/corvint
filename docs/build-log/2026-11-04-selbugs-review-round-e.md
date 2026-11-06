# Selection correctness: review round e

Owner intent: correct the P1 and P2 findings from review round e of the selbugs lane. The
orchestrator chose the P1 remedy. Lane base `6f12f4704235dc8f9bea19fc12995bfb68c746c3` stays as
it was, and no requirement ID is new. This record describes local evidence only; the next
independent review and integration belong to the orchestrator.

## Decision: the darwin split is retired (ASD-V0-2403 deferred)

The reviewer showed that a platform check can sit anywhere: a common test can run
`exec.Command("uname", "-s")`, call an imported helper that checks the platform, or probe the
host some other way. No static scan of a package's own files proves that the Linux half of a
split runs every common test the way Darwin would, so the keyword rule of round d cannot make the
split sound. The orchestrator decided to retire the split rather than keep patching it.
`script/heavy.sh` now never splits a command between hosts, and every darwin-only command stays
wholly local (ASD-V0-934), as at the lane base. The split was also worth nothing here: measured
over the 42 packages of this repository that hold darwin-only files, it split 0 (build-log entry
`2026-11-04-selbugs-review-round-d`). ASD-V0-2403 is amended in place to status deferred, records
that count and the soundness reason, and pins only that darwin-only commands stay local. A
redesign belongs to a post-1.0 ticket the orchestrator files.

The split code path is removed, not left behind a flag: `platform_neutral`, `runtime_gated`,
`go_imports`, `darwin_split`, `contains_word`, the `darwin_tail` global and `darwin_tail_run` are
gone, the offload branch calls `darwin_local` directly again, and the offload return no longer
runs a local tail. The darwin handling of `script/heavy.sh` now matches the lane base exactly
(`git diff 6f12f47 -- script/heavy.sh` holds no darwin hunk).

## Tests

Offload group 9 keeps the round c and round d refusal fixtures as "stays local" assertions:
each injects a common test that fails only on Darwin (the go stub fails `TestHidden` in a local
run) and expects the command to stay local, with no SSH call, and return that failure (rc 6).
Kept: a test-only darwin package runs the unchanged command once here (`local test -count=1
./mtest`, no `-run`) with the darwin-local notice; its failure is the command's (rc 4); `./mac`,
`-run` and a mix stay local; an other test file constrained `//go:build !linux`, `// +build
!linux` or by a `_linux` name, and an assembly file; a blank, named, dot, cgo or plain import in
the darwin test file; a second package argument whose `//go:build !linux` test fails; and the
seven runtime-gated cases. New: a common test that runs `uname -s` and fails on Darwin, whose
source names no platform at all.

Deleted, because they only exercised splitting: the split-success assertions, the failing local
half, the helper, type, next-line and same-line declaration cases, the parser-uncertainty and
`init` cases, and the controls that an unconstrained common test or a shared import still split.

Before and after, group 9 ran from copies whose only change narrows `selected()` to group 9.
Against the reviewed `script/heavy.sh` (71a5ea4c5) three assertions fail: the test-only package
offloads, its failure is lost, and the `uname -s` case splits and passes (rc 0). Against the
fixed wrapper the group passes, and all 13 offload groups pass in the tree. ShellCheck warnings
are unchanged (3 in heavy.sh, 31 in the test file).

## P2: planned packages a manifest omits (AFP-V0-2403)

`checkAgreement` compared only one direction: a Go package the manifest runs must be planned. A
NARROWED or NOTHING manifest that dropped a planned package passed whenever the change recorded no
failure. It now also records `testOmitted`, each root Go package (`go:` unit) the plan selects
that the manifest does not run, and `make selection-baseline-check` fails on any entry, whatever
the failures. The separate failing-package check (`testMisses`) is unchanged, FULL stays exempt,
and the verdict line ends `test-extra=N test-omitted=N test-misses=N`.

A dependent nested module's synthetic unit (AFP-V0-582) also carries a `go:` ID (the nested
module path) but is no root package; the manifest lists it under `excluded` (STR-V0-069). It is
skipped only when the manifest excludes it and its directory below the root module path holds a
regular `go.mod` at the replayed head; any other excluded entry, a path outside the root module,
or one escaping the clone is still compared, so the check fails closed.

Tests: `TestAFPV02403ManifestOmitsPlannedPackage` (a NARROWED manifest that omits a planned
package on a change without failures fails; a nested-module manifest unit, a non-Go plan unit and
a duplicate plan unit are handled), `TestAFPV02403NestedModuleUnitIsNotOmitted`, and the amended
`TestAFPV02403NothingManifestMissesFailingPackages`, whose NOTHING-without-failures case now
fails the gate. The first two failed before their fixes; the nested-module test failed with the
skip removed.

The real-range replay (`make selection-baseline-check`) over the 58 recorded changes, 53 with a
NARROWED manifest and 5 FULL, reports `narrow=33/43 merged (rate 0.767) bounded=0 misses=0 over
26 failing packages of 15 failed CI runs; median cost 0.927; whole=6648 sliced=2
whole-source-and-tests=26 test-extra=0 test-omitted=0 test-misses=0` and `OK floor narrow>=30
misses<=0`. The new check does not trip on any recorded range. How many planned units the
nested-module skip excused on those ranges is not recorded by the tool and stays unobserved.

## Rollback

Revert the selection-baseline commit to drop `testOmitted`; the ASD-V0-2403 retirement needs no
rollback, since it restores the lane base's darwin behavior.
