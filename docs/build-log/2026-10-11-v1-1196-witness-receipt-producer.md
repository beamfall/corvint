# V1-1196 witness-playwright receipt producer and provider help (GitHub #794)

Decision: document the qualified receipt chain where a witness-import user looks, and give
`corvint-js-test-provider` a help command. Proposed, pending owner acceptance: `DCP-V1-053` and
`LPCV-V0-071`.

Verified producer chain, from source:

- `corvint-js-test-provider e2e --external-server` (profile /0 to /3) writes
  `jstestprovider.EncodeQualified(receipt)` to stdout, and `--retain` stores the same bytes under
  `.corvint/test-evidence/` (`emit`, `cmd/corvint-js-test-provider/main.go`). Managed-server `e2e`
  prints the indented `{receipt, testProjections, runProjection}` wrapper instead. It decodes, but
  its projection has no `playwright` member, so `qualifiedPlaywrightReceipt` refuses it ("not a
  qualified Playwright receipt").
- `docs corpus discover-playwright --receipt FILE` reads that file unchanged
  (`adoptedPlaywrightIDs` through `testvaliditydoc.Decode`, which requires the canonical bytes).
- `docs corpus witness-playwright --receipt-input ID` takes a request input ID, not a file. The one
  conversion is the caller's: commit the receipt bytes unchanged at a tracked path, because
  `.corvint/test-evidence/` is git-ignored. Then add them to the behavior-adapter request as an
  `inputs[]` entry whose `document` is those bytes, with a full-file `external-provider` anchor
  (`addInput`, `internal/doccorpus/behavior_adapter.go`). The import then decodes the input's
  document as the qualified receipt.

Delivered: the `docs --help` text, which `docs corpus witness-playwright --help` prints, has a
producer paragraph and a four-step example. The provider's `--help`, `-h`, `-help` and `help`
print a subcommand summary and the real flag defaults of `unit`, `e2e` and `negate`, then exit 0.
A nil-by-default `usageSink` makes their flag sets print without parsing. `negate -h` itself still
refuses with `negate-invalid-arguments`, unchanged.

Evidence: `TestDocsHelpNamesPlaywrightReceiptProducer` (cmd/corvint) runs the example's discovery
step on the issue-717 fixture with a provider-encoded receipt (a listed test adopts the receipt
ID). It shows that the witness step's arguments are admitted. `TestProviderHelpPrintsEverySubcommandUsage`
re-executes the test binary through `main`. `TestDocsHelpProducerExampleUsesE2EFlags` checks the
example's producer flags against the `e2e` section of the usage only, and parses them through the
real `e2e` flag set with a stubbed runner, requiring every external-mode admission field. The witness join on canonical receipt bytes
stays covered by `TestPlaywrightWitnessImporter`. A live provider run against a real external
application is `NOT_RUN`.
