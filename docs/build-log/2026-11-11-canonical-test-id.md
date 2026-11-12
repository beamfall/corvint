# 2026-11-11: canonical test ID (V1-5398)

Change `canonical-test-id`, tasks 1 to 7. Requirements CTI-V1-001 to CTI-V1-012 in
`docs/specs/canonical-test-id-v1.md`. First change of the R38 testing order.

## What changed

- `internal/testid` (layer 0, standard library only; one prefix line in
  `tools/import-ratchet/areas.json`) formats, parses, escapes and matches the ID; maps each
  runner's machine report to IDs; disambiguates duplicates; resolves a requested ID with its
  widening reason; and reads legacy unit keys.
- `corvint-test-run/0` outcomes carry an added `testId` (omitempty, settled D25), stamped by
  `Runner.Execute` from each outcome's unit. `Outcome.ReadTestID` reads a receipt with or without
  it.
- `affected-summary/1` (`corvint affected --summary`, and the `corvint.affected` MCP tool, which
  returns it) gains `tiers.testIds`; `corvint watch` invocations gain `TestIDs`.
- The spec records the per-runner surfaces table, the traceability table, and the acceptance-log
  row `canonical-test-id` (mode `auto`).

## Differences from the proposal, followed by intent

1. Integ `9ec3a29d0` is not reachable in this clone. The legacy receipt fixture
   (`internal/sessiontest/testdata/receipt-before-testid.json`) is this tree's encoding without
   the field. At `9ec3a29d0`, the proposal says `Outcome` held only `outcome`, `slice` and `unit`,
   and with the omitempty field absent the bytes are the same.
2. `corvint test` units are packages and files, not tests. So `testId` is the package-level ID
   (Go) or the file-level ID (the other runners), and a selected Go slice keeps its package ID
   beside its `slice`. Test-level IDs appear in report mapping and in watch.
3. Go units are keyed by import path. The package directory comes from the root `go.mod`, the same
   way in the receipt and in the plan verdict (`sessiontest.GoUnitDir`). A nested-module unit
   (`go-module:<dir>`) is a runner instance, not one package, and carries no ID.
4. "The plan from `corvint affected`": the default `affected-plan/1` and `--full`
   `affected-plan/0` documents are frozen Core output. CCF-V1-006 admits an optional member only
   with a regenerated Core golden, and the lane charter forbids editing frozen goldens. The plan's
   IDs therefore surface in `affected-summary/1`, which is also the MCP result, for the tier-0 and
   tier-1 units it names. Extending the full plan is left to an orchestrator decision.
5. Ruby units are `ruby:<kind>:<stem>` with `.rb` dropped. They read as `rspec:<stem>.rb` or
   `minitest:<stem>.rb`, and `rails-test` is Minitest. A `python:<module>` unit names a module,
   not a file, so pytest has no plan or receipt ID until a pytest provider names files.
6. RSpec: neither the RSpec JSON report (Corvint's formatter adds only inclusion backtraces) nor
   the Corvint line-coverage hook (which records `example.id`) carries the example group chain.
   Each RSpec example therefore maps to its file-level ID with `test-id-unmapped`, never a split
   of `full_description`. `RSpecExample` implements the rule for a source that has the chain;
   adding the chain to the formatter belongs to the `ruby-providers` change.
7. pytest has no built-in JSON report. The mapping reads pytest-json-report `tests[].nodeid` and
   takes the pytest rootdir relative to the repository root.
8. Watch IDs are relative to the watch root. That root is the repository root for a module at the
   root, the only layout `corvint watch` plans.
9. Report fixtures: `go-test.jsonl` is a live `go test -json -count=1` recording made in this
   lane. `vitest-mixed.json` and `playwright-mixed.json` are copies of the recorded runs in
   `internal/jstestprovider/testdata`. `jest.json`, `playwright-projects.json`, `pytest.json`,
   `rspec.json` and `minitest.jsonl` are written by hand in each tool's documented shape, because
   Jest, pytest and Ruby are not installed on this host.
10. Acceptance: the change implements accepted plan requirements (decision 0596, G14). Under
    ACR-V1-006 to ACR-V1-008, the spec marks each requirement with the standing auto-accept
    marker and logs the change as `auto`, never `owner`.

## Assertions extended

The added `testId` field changes the value of whole-struct `Outcome` comparisons. Six such
comparisons in five tests now also name the expected `testId`; no existing field's expectation
changed:
`TestSTRV08704RunnerReportsProviderWithGo` and `TestSTRV08705RunnerProviderUnavailable`
(`internal/sessiontest/provider_unix_test.go`), `TestSTRV0010CLIRunWritesReceipt`, and
`TestSTRV08704CLIMixedGoAndPlaywrightRun` and `TestSTRV08705CLIUnavailableProviderIsNotRun`
(`cmd/corvint`). No frozen golden was edited.

## Rollback

Revert the change's commits. Readers fall back to the unchanged `unit` key, and no stored record
needs rewriting.
