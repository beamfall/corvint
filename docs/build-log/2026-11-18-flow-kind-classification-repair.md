# Version 1 flow kind classification repair

The self-review finding identified an APK-V0-002 regression in the version 1
declaration path introduced for FLP-V1-006/007: the conservative E2E return ran
before non-E2E kind classification. The repair moves the existing classification
loop ahead of that guard. The FULL E2E fallback and its uncertainty remain intact;
the previously recorded FLP integration blockers are unchanged.

This is a control-flow bypass of the shared classifier, not a new syntax analyzer.
The existing closed kind-rule vocabulary remains `unit` and `integration`.
All test paths must agree under those rules; unmatched, empty or conflicting
inputs retain `UNKNOWN`. No rule is inferred from a directory name.

`TestAPKV0002Version1EverySelectionCarriesItsKind` upgrades the fixture declaration
while retaining its rules and checks unit, integration and unmatched selections,
plus the FULL E2E fallback. `TestFLPV1007PendingIntegrationNeverNarrows` now retains
the kind rules and compares the complete selections with the classified version 0
control across absent, accepted and unaccepted link records.

Both tests failed against the pre-repair implementation. After the repair, the
whole `internal/appflows` package passed through `script/heavy.sh` with no `-run`
filter: `go test -count=1 -timeout 30m ./internal/appflows`, 12.630 seconds.
The checkout-built `corvint affected --base
a359919975da48761fd263869cc2f000dae1fb79` selected this package; scope remained
UNKNOWN with six unknowns. No wider coverage is claimed.

Raw evidence is retained outside the repository under
`/home/russell/lanes/flp-evidence/`:

| Artifact | SHA-256 |
| --- | --- |
| `self-review-regression-before.log` | `3796199ad64a22904c15d2ae7c5a17f52c47d9037bcdb8fdd4172f1e8c419ebb` |
| `self-review-regression-after.log` | `570da70b8c58281979da866bf47b90b4af3f5657a18700f7df778dfdb6ad76d4` |
| `self-review-affected-local.json` | `53286de49ed4211571bfb548d61a5f0fa100782665f3918436c0a7bae2a8515e` |

No spec or wire contract changes are required: this restores APK-V0-002's existing
requirement. Rollback is a revert of the repair commit, which restores the
demonstrated classification defect. Independent review remains assigned to the
orchestrator. CEM binding is unavailable under the retirement documented in
`docs/DOGFOOD.md`; no binding or seal is claimed.
