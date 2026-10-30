# Omission soundness case (CEM-V1-633, V1-2599)

Can a producer leave a failure out of a cem/1.0 stable map and still get a
clean reader verdict? This case reuses the two-profile recipe in
`../multi-runner/` (`EXPECTED.json` `repository`) and adds one runner pair.

- `artifacts/runner-plan.json` is the multi-runner `go-test` plan 0 with
  `example.com/greet::TestGreetName` added to `expectedTests` and `selectors`.
  The target tree defines that test.
- `artifacts/runner-receipt.json` binds that plan and records
  `TestGreetEmpty` PASSED and `TestGreetName` FAILED. Both documents are
  SYNTHETIC: no runner produced them, and `TestGreetName` really passes.
- `failed-cited.json` is the exact output of the plain producer's single form
  without `runnerAdmission` over these files. The receipt row is labelled
  `opaque/0` (CEM-V1-310). With admission the producer refuses
  `runner-receipt-failed` instead. The native producer test reproduces the map
  and both artifacts byte for byte.
- `cases/failed-labelled.json` is the same map with the receipt row relabelled
  `corvint-test-runner-receipt/0`.

`EXPECTED.json` lists eight reader checks, grouped by the V1-2599 case:

- (a) a cited failed receipt under either label, and the narrowed inventory of
  `../multi-runner/stable.json`, whose `go-test` plan names only
  `TestGreetEmpty`;
- (b) a run left out whole, which is byte-identical to a clean map, and the
  three partial omissions in `../multi-runner/cases/`;
- (c) a selection narrower than `corvint affected`, which selects the whole
  `example.com/greet` package.

Both readers, native `internal/cem/verify` and portable `interop/cem01-go`,
ACCEPT the clean cases. They report reference integrity only, with the receipt
semantics, inventory and adequacy axes `NOT_OBSERVED`. They refuse each partial
omission at stage `references`.
