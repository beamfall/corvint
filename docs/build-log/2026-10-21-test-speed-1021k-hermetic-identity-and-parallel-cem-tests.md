# Lane test-speed-1021k: hermetic Git identity in contextindex and parallel CEM tests

Lane `test-speed-1021k`, base `58d4ed2d309d5dc1afc21c127647d2e22b1bed3f`. Tickets V1-2287, V1-2220
and the V1-0643 remainder. These are test-only changes. No requirement was added or changed, so the
owner's standing auto-accept (2026-10-11) is not exercised.

## V1-2287: contextindex failed with an exported Git identity

**Problem.** Fixture repositories pin identity with `git config` and dates with `t.Setenv`. An
exported `GIT_AUTHOR_*` or `GIT_COMMITTER_*` value overrides both. Lane and CI shells export them.

**Fix.** A new `internal/contextindex/testmain_test.go` unsets the six identity and date variables
in `TestMain`. `TestGitIdentityEnvironmentIsScrubbed` keeps that guarantee. No other file changed,
which keeps the overlap with open PR #1461 (contextindex test parallelization) to one new file.

**Evidence.** With the lane identity exported:

- Before the fix, `-run TestContextRecency` FAILED in two places:
  - `TestContextRecencyReportsCodeOwnersBlameDisagreement/TCP-V0-037` reported blame author
    `me@russe11.com`.
  - `TestContextRecencyDefaultBytes/TCP-V0-035` reported changed golden packet bytes.
- After the fix:
  - The focused run passes, both with and without the variables.
  - The full package passes with the variables exported (217.8 s, load 26→15).

## V1-2220: slow CEM tests under load

**Profile.** `go test -json` was run per package through `heavy.sh`. Slot wait was 0 s on every
run, so all time below is run time, not slot wait.

| Package | Before | After | Change |
|---|---|---|---|
| `internal/cem/gitauth` | 228.5 s (load 13→69) | 68.3 s (load 37→45); 53.8 s rerun (load 14→19) | 84 of 99 top-level tests now call `t.Parallel()` |
| `internal/cem/verify` | 92.0 s (load 42→24) | 37.8 s (load 16→31); 29.5 s and 33.4 s reruns | 71 of 76 top-level tests, plus the subtests of `TestStableS0EPublicCases` and `runS0ECasesWhere`, now run in parallel |
| `internal/cem/workflow` | 38.4 s (load 30→42) | unchanged | 77 of 87 tests were already parallel |

In `internal/cem/verify`, the long pole was `TestStableS0EPublicCases` at 43.5 s, which ran its
ledger-336 and ledger-337 cases serially at about 14 s each. Those cases are now independent
subtests.

**Kept sequential (decision).** Tests that call `t.Setenv`, `os.Setenv` or `Chdir` stay
sequential. So do tests that reach a test-file helper that does. `t.Setenv` panics under
`t.Parallel`, and process environment is shared.

- That leaves 15 gitauth tests sequential, totalling 35.5 s, with the longest at 8.3 s
  (`TestBoundedBlobRejectsHeaderWithoutFallback`).
- It leaves 5 verify tests sequential.

**Assertions.** No assertion, fixture or timeout was changed. Each test only gained `t.Parallel()`.

**Not changed.** `internal/gitnotes` is excluded because open PR #1459 edits it.

## V1-0643 remainder: SKIPPED

`internal/companionrelease` and `conformance/release-artifact-v0` cannot leave `UNBOUNDED_READER`
with hermetic fixtures without dropping what their tests prove:

- **companionrelease.** Seven tests reach the real checkout through `filepath.Join("..", "..")`.
  They `go build` the real `./cmd/*` binaries, or export HEAD with `git archive`. Examples:
  - `TestCorvintBundledBinariesBuildTwiceIdentically`
  - `TestJSProviderUsageAdmitsTheBuiltProvider`
  - `TestRubyProviderUsageAdmitsTheBuiltProvider`
  - `TestCurrentTasksSourceArchiveBuildsOffline`
- **release-artifact-v0.**
  - The non-test CLI calls `os.Getwd`.
  - `testCorvintHostArchivePartialProof`, cited by PUB-V0-021, builds Corvint from the clean
    checkout.

A `.corvint/test-read-scopes.json` declaration cannot cover either package, because entries can
never name `.git`.

Moving only the two archive-gate tests in release-artifact-v0 onto the existing fixture would
leave both packages on the floor. Residual: split the real-checkout proofs into a dedicated
package, or record them as accepted floor members.

## NOT_RUN

Tier-2 units of `corvint affected` (59 units) are NOT_RUN locally and left for batch CI. Tier 0 is
the three changed packages, and all three ran in full. Tier 1 was empty. Only `_test.go` files
changed, so no package imports the changed code.

## Rollback

Revert the three test commits. No production code, spec or fixture changed.
