# V1-1231: corpus Playwright discovery applies grep omission rules (GitHub #800)

## Finding

`corvint docs corpus discover-playwright` refused a complete, unfiltered listing from a config with
one `grepInvert` project and two `grep` projects. `affected` discovery accepted the same listing
after #790. Two entry points share one membership check, `observePlaywrightListMembership`.
`PlaywrightDiscoveryFromList` passed it the per-file listed cases. `PlaywrightListedTests`, the
producer behind corpus discovery, passed nil. So `classifyPlaywrightGrepOmission` never ran there,
and every grep-filtered pair counted as missing.

## Change

- `PlaywrightListedTests` collects the per-file cases in its own execution walker and derives the
  checked units from the executions it returns. Membership is therefore checked against exactly the
  files and cases the record carries. Only a spec with at least one execution supplies a case, so an
  empty spec cannot explain an omission. Codex review round 2 found both gaps in an earlier version
  that ran discovery's separate walker, `collectPlaywrightListUnits`.
- An omission the rules explain, or leave unverified, is accepted. It adds no execution, because the
  corpus record lists only executions the listing shows.
- Corpus discovery always passes the pinned revision's paths. The source-revision membership check
  (`checkPlaywrightRevisionMembership`) refused the same pairs again without classifying them. It now
  skips pairs the working-tree check accepted and tolerates a dynamic grep as that check does. Codex
  review round 1 found this.
- An unexplained omission still refuses, as does a revision-only test file.
- `DCP-V1-055` records the rule (proposed).

## Evidence

`TestPlaywrightListedTestsGrepOmission_TJAA_V0_021` covers the following:

- It runs the #790 three-project repro through `PlaywrightListedTests`.
- The fixture is reshaped to Playwright's real one-spec-per-project form, with IDs and locations.
- It expects the four listed executions.
- It passes the revision's paths, as corpus discovery does.
- It checks that an unexplained omission refuses, and that a revision-only test file refuses.

`TestPlaywrightListedTestsCasesMatchExecutions_TJAA_V0_021` checks two more refusals. In the first, a
nested suite names a file that the execution walker does not attribute. In the second, a spec with no
executions would otherwise explain a `grepInvert` omission.

Without the fix, the test reproduces the issue's refusal text exactly.

## Limits

The corpus record has no `unverifiedUnits` member, so an unverified omission is not surfaced in the
record. It is accepted for the same reason the listing itself is: the listing is caller-declared
evidence, and a test it hides stays undiscovered rather than invented. The listing walker also
applies discovery's matcher-model path refusal, which `PlaywrightListedTests` did not apply before.
`collectPlaywrightListUnits`, which `affected` discovery still uses, attributes nested suite files
and empty specs differently. That walker was not changed here.
