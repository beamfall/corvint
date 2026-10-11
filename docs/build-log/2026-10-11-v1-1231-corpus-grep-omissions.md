# V1-1231: corpus Playwright discovery applies grep omission rules (GitHub #800)

## Finding

`corvint docs corpus discover-playwright` refused a complete, unfiltered listing from a config with
one `grepInvert` project and two `grep` projects. `affected` discovery accepted the same listing
after #790. Two entry points share one membership check, `observePlaywrightListMembership`.
`PlaywrightDiscoveryFromList` passed it the per-file listed cases. `PlaywrightListedTests`, the
producer behind corpus discovery, passed nil. So `classifyPlaywrightGrepOmission` never ran there,
and every grep-filtered pair counted as missing.

## Change

- `PlaywrightListedTests` collects units and per-file cases with the same walker discovery uses,
  `collectPlaywrightListUnits`, and passes the cases.
- An omission the rules explain, or leave unverified, is accepted. It adds no execution, because the
  corpus record lists only executions the listing shows.
- An unexplained omission still refuses.
- `DCP-V1-055` records the rule (proposed).

## Evidence

`TestPlaywrightListedTestsGrepOmission_TJAA_V0_021` covers the following:

- It runs the #790 three-project repro through `PlaywrightListedTests`.
- The fixture is reshaped to Playwright's real one-spec-per-project form, with IDs and locations.
- It expects the four listed executions.
- It checks that an unexplained omission refuses.

Without the fix, the test reproduces the issue's refusal text exactly.

## Limits

The corpus record has no `unverifiedUnits` member, so an unverified omission is not surfaced in the
record. It is accepted for the same reason the listing itself is: the listing is caller-declared
evidence, and a test it hides stays undiscovered rather than invented. The listing walker also
applies discovery's matcher-model path refusal, which `PlaywrightListedTests` did not apply before.
