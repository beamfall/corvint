# V1-1689: the e2e-safe Decode secret screen no longer dominates the 16,384-test ceiling

Ticket: V1-1689. Requirement: `LTA-V0-014` (no wire or match change; one traceability test added).

## Finding

A CPU profile of `TestAFUV1065CeilingMeasurement` at `CORVINT_E2E_SCALE_TESTS=16384` put
`secretscreen.MatchString` at 2.32 s of 5.23 s of samples (44%), screening 8.2 MB over 16,521
`appflows.Decode` calls with distinct content (callers: `IntentSet.add` 1.22 s, `anchorLinks`
0.76 s, `readSelectionDocument` 0.70 s, `InventoryTest.UnmarshalJSON` 0.29 s). Every inventory
element carries a `"test_key":` field, and the prefilter's bare `key` credential stem made every
such text an assignment-branch candidate, so 4 to 24 live alternatives were compiled and run on
each element although none ever matched.

## Decision

The prefilter's `credentialStems` list replaces the bare `key` stem with the compound key names
the assignment and flag branches actually require (`apikey`, `api_key`, `api-key`, `accesskey`,
`access_key`, `access-key`, `accountkey`, `account_key`, `account-key`, `privatekey`,
`private_key`, `private-key`). This is a necessary-condition change only: `Pattern`, its source
digest and `StoredV1Pattern` are unchanged, and `LTA-V0-014` still requires the prefilter to return
exactly `Pattern`'s matches. The differential `TestMain`, the generated-branch test and a 45 s run
of `FuzzLTAV0014PrefilterReturnsPatternMatches` (219,448 execs) found no disagreement.
`TestLTAV0014BareKeyFieldIsNoCandidate` pins that a `test_key` field compiles nothing while every
compound key name still matches. The conservative fork: no caching of screen results by content
digest (a new state surface) and no removal of the double screen of `InventoryTest` (0.29 s),
which is left for a follow-up.

## Measurement

Interleaved runs of the prebuilt before/after test binaries, same host, load 13 to 23:

| run | before | after |
| --- | --- | --- |
| 1 | 7.86 s | 5.33 s |
| 2 | 9.17 s | 6.07 s |
| 3 | 8.37 s | 5.10 s |

About 35% faster; every run reported `narrow-selection-allowed`. The after profile shows
`secretscreen.MatchString` at 0.12 s of 2.40 s of samples (5%), down from 2.32 s.

## Rollback

Revert the commit; the stem list returns to the bare `key` stem and the screen returns to its
earlier cost with identical matches.
