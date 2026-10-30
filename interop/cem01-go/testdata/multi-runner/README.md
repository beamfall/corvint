# Two-profile runner receipt case (CEM-V1-573, CEM-V1-577)

A cem/1.0 stable map whose two criteria are covered by runner receipts of two
different runner profiles: criterion 0 (SPEC.md line 5, the Go sources) by a
`go-test` receipt and criterion 1 (SPEC.md line 6, the JavaScript sources) by a
`node-test` receipt.

- `repository.recipe.json` builds the base and target commits, exactly like the
  public neutral-criterion recipe.
- `artifacts/runner-plan-<i>.json` and `artifacts/runner-receipt-<i>.json` are
  SYNTHETIC corvint-test-runner documents. They pin the target blobs and bind
  each other, so producer admission accepts them, but no runner produced them
  and their `root` is the placeholder `/fixture/repo`.
- `stable.json` is the exact output of the plain producer's `runners` form
  (`cem-stable-plain-assembly/1`) over this recipe and these artifacts; the
  native producer test reproduces it byte for byte.
- `cases/*.json` are tampered variants of `stable.json`. `EXPECTED.json` gives
  the stage and code each reader must refuse them with.

Both readers, native `internal/cem/verify` and portable `interop/cem01-go`,
ACCEPT `stable.json` and refuse every case. A reader binds a receipt to its
plan only by artifact kind, so swapping the two plan digests is not detected
by a reader; producer admission is where a receipt and its plan are bound.
