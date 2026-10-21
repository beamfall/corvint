# 2026-10-21 bugs-1021j: Ruby regex mask, unittest/test-unit errors, per-path tree bound

Lane bugs-1021j (charter 1021h, base `8def072e0`). Requirements are accepted under the owner's
standing auto-accept (2026-10-11).

## V1-2158: Ruby literal mask treated a quote in a regex literal as a string

Problem: `maskQuotedLiteralsAndInlineComments` masked `%r` literals but not `/.../`, so in
`x =~ /'/` the quote opened a string that ran to the next quote and hid every constant between
(`Foo::Bar.call` lost its advisory edge).

Decision (`RUBY-AFFECTED-V0-044`): mask a slash regex only in an operand position (line start,
after an operator, comma or opening bracket, or after a short keyword list) and only when it closes
on its own line. Division after an identifier, number, method call or closing bracket is never
masked. Ruby's warning-level heuristic for `ident /re/` with a space is not followed: masking less
only keeps the previous behaviour, while mistaking division for a regex could hide code.

## V1-2156: unittest and test-unit errors were UNKNOWN

Problem: after `TRE-V0-110` Minitest reports a raising test as FAILED/UNKNOWN, but the Python
`unittest` and Ruby `test-unit` reporters still emitted `UNKNOWN`, which voids the whole run.

Decision (`TRE-V0-115`): the same FAILED + failure kind UNKNOWN, message `Error: <Class>` (no
exception text). A `unittest` class/module fixture error (`_ErrorHolder`) names no single test and
stays `UNKNOWN`; an erroring `unittest` subtest follows the test rule.

## V1-2151: query freshness and batch inputs used only the aggregate tree bound

Problem: `query.go` and `batch_input.go` resolved paths with `ResolvePaths`, bounded only by the
32 MiB walk total, so a path under a parent tree above the 4 MiB per-path `MaxTreeBytes` was
answered where the per-path lookup refuses with `git-output-exceeded`.

Decision (`DCP-V1-243`): both use `ResolvePathLookups` through `resolvePathsPerPathBound` and refuse
such a path with the per-path lookup's own refusal, compared byte for byte in a `git mktree`
fixture test.

## Evidence

- `TestRegexLiteralMaskHidesQuotesNotDivision_RUBYAFFECTEDV0044`,
  `TestUnittestAndTestUnitErrorFailsOnlyThatCaseLive` (live Python 3.9.6, Ruby 2.6.10 test-unit) and
  `TestDCPV1243ResolvedInputsKeepPerPathTreeBound` each fail without their fix and pass with it.

## NOT_RUN

- Other Python/Ruby versions for TRE-V0-115.
- Tier-2 affected packages: batch CI.

## Rollback

Revert the lane commit: the reporters, the mask and the two `ResolvePaths` call sites return to
their previous behaviour; no stored data or wire shape changes.

## Ticket dispositions

- V1-1553 (P1): already delivered on main by `547048cc9` and `aacaac24d` (TRE-V0-087, fake adb and
  Appium tests); its live device and Appium witnesses remain NOT_RUN (need an emulator and Appium).
- V1-0908: already delivered on main by `4a1d9bf63` and `73d457f0a`.
