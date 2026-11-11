# Decision 0595: `corvint flows` and `corvint how` run in `corvint-docs`

Owner decision, 2026-11-09 (decision 0595); base `e8bd14d9d`, branch `claude/flows-to-docs`.
Requirements `CMS-V0-912..916` in `docs/specs/core-bundled-apps-v0.md`.

What moved:

- The `--root` preamble, the `how` alias and the `runFlows` wrapper moved from `cmd/corvint` to
  `cmd/corvint-docs/flows.go`; the `flows` and `how` help topics are registered in
  `cmd/corvint-docs/help.go`, and the handlers in `internal/command/flows` now spell the binary
  `corvint-docs`.
- `cmd/corvint/flows_shim.go` forwards `corvint flows ...` and `corvint how ...` through the
  existing `forwardToDocs`, so a missing `corvint-docs` is the same `not-installed` refusal the
  `docs` shim gives. Help is native and short, and points at `corvint-docs help flows|how`.
- `flows test-evidence` stays native in `corvint` (`cmd/corvint/flows_test_evidence.go`). It was
  the one subcommand supplied from the tests layer through `flows.Env.TestEvidence`; the docs
  layer cannot import that, so the injection point is gone and `corvint-docs flows test-evidence`
  refuses with a pointer to `corvint flows test-evidence`.
- The flow readers did not move. `tools/import-ratchet/areas.json` already keys
  `internal/appflows`, `internal/appmap` and `internal/flowcoverage` as `shared`, and
  `cmd/corvint`, `internal/testplan` and `internal/testsource` import them in production code.

Import baseline: 18 -> 17 edges. Removed `cmd/corvint -> internal/command/flows`; added none.

Capability catalogue: `subcommandTables` in `tools/capabilities-check` tracks no `flows` table
any more. A first attempt kept the forwarded subcommand names in the `corvint flows` row of
`docs/CAPABILITIES.md`; `CAP-V0-961` reads every backticked name in a `Subcommands` sentence as a
claim about `corvint`'s own tree and flags the forwarded ones as stale, so the row became a shim
row like `corvint docs` and the `corvint-docs` tree snapshot carries the 43 `flows`/`how` nodes
(`TestCMSV0912EveryFlowsSubcommandIsInTheCommandTree` keeps the registry and the tree equal).

Finding while regenerating the trees: `internal/clitree` reads any help line that starts with the
binary name as a synopsis. A wrapped prose line in the shim's `how` help began "corvint or on
PATH" and produced the nodes `corvint or` and `corvint or on`; the paragraph was rewrapped. The
builder has no guard for this.

Shared flow packages that still import a docs or tests package (tolerated baseline edges, not
changed here):

| Edge | Layers | Cause |
| --- | --- | --- |
| `internal/appflows -> internal/doccorpus` | shared -> docs | canonical JSON (`Encode`), `Anchor`/`Evidence`/`LineIndex` and the behaviour-adapter and stability types are defined in the corpus package and used across the flow model, export/import, docs and impact |
| `internal/appflows -> internal/jstestprovider` | shared -> tests | `runingest.go` decodes Playwright attempt receipts (`DecodeAttemptReceipt`, `ReceiptTestProjection`) |
| `internal/appflows -> internal/liveverify/affected` | shared -> tests | `impact.go`, `selection.go` and `repositories.go` build the affected graph and plan (`Graph`, `Plan`, `RangePaths`, `DirtyPaths`, `Unknown`) for `flows impact` and E2E selection |
| `internal/appflows -> internal/liveverify/affected/languages` | shared -> tests | `impact.go` selects the language set for that graph (`languages.All`, `SelectAt`) |
| `internal/appflows -> internal/liveverify/affected/typescript` | shared -> tests | `selection.go` uses Playwright discovery units and frontier codes (`PlaywrightDiscoveryUnit`, `VerifyPlaywrightDiscovery`) |
| `internal/appmap -> internal/doccorpus` | shared -> docs | `site_knowledge_flows.go` emits a behaviour-adapter input (`BehaviorAdapterInput`, `Anchor`, `Digest`) |
| `internal/appmap -> internal/jstestprovider` | shared -> tests | `ledger.go` and `verify.go` read receipt states and the qualified-receipt binding |
| `internal/appmap -> internal/testvaliditydoc` | shared -> tests | `verify.go` decodes a test-validity document (`Decode`, `Project`) |
| `internal/flowcoverage -> internal/doccorpus` | shared -> docs | `doccorpus.Anchor` in the coverage types and `doccorpus.Open` for the corpus source |
| `internal/flowcoverage -> internal/flowdocs` | shared -> docs | coverage compile and write-back materialise and compare flow-doc output (`MaterializeCoverage`, `CompareOutput`, `Output`) |
| `internal/flowcoverage -> internal/jstestprovider` | shared -> tests | `runs.go` decodes Playwright receipts and their states |

Cutting them means moving the shared value types (anchors, canonical encoding, receipt states)
down into `shared`, and moving the impact/selection and docs-generation halves of `appflows`,
`appmap` and `flowcoverage` up into the layer that owns them. That is larger than a re-key and is
not part of this change.

Evidence: see the acceptance rows of `CMS-V0-912..916`. The parity test
(`TestCMSV0912FlowsAndHowParityWithCorvintDocs`) builds `corvint-docs` and compares corvint's
dispatcher forwarding to it, the binary run directly, and the handlers called in process, for
`flows map`, `flows export`, `how`, `flows appmap how` and four refusals. `cmd/corvint` tests
import `internal/command/flows` in test files only, for flow fixtures and that in-process side.

Order of record: the owner's rule that a change is written down before it is built arrived
while this one was in progress. The code commits preceded the written record; decision 0595, the
`CMS-V0-912..916` requirements and this entry followed in their own commits, and the decision now
carries the moved code, the layer rule, the command surface before and after, the failure modes
and the rollback. Requirement text changed: `CMS-V0-912..916` (new), `CAP-V0-007`, `CAP-V0-015`
and `CAP-V0-960` (the `flows` table is no longer tracked in `corvint`), and `CAP-V0-027` (a guide
example spelled `corvint-docs` is not checked against `corvint`'s catalogue).

Shipped agent guide (`internal/capcatalog/AGENT-GUIDE.md`): `CAP-V0-018` checks every guide
line that starts `corvint ` against `corvint`'s own catalogue, which no longer lists the forwarded
verbs, their options or their schemas. The two `flows ingest` lines are spelled `corvint-docs
flows ...`, as the docs lines already were, and name their output in words. The adoption line
keeps `corvint flows appmap build`, because `corvint` still prints that step's help
(`CAP-V0-030`); the shim's `flows` help carries the build synopsis so the catalogue lists the
operation and its options.

Not changed: `.github/cishards/test-slices.json` still names tests this change moved or renamed;
it is a generated timing snapshot and `.github` changes go in their own change.

Rollback: revert the change; `corvint` dispatches `flows` and `how` natively again and the
baseline edge returns.
