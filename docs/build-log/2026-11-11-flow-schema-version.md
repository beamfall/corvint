# V1-5410: flows schema version, one reader, standalone smoke (FSV-V1-001..007)

Delivery of the OpenSpec change `docs/specs/openspec/changes/flow-schema-version/` on branch
`claude/fsv`, base `af393f1b1e20f59d536663276f30469bea688f6a`. It serves PROD-INT-016 and the
PROD-DOCS-011 flow reports (owner answer G5). The canonical spec is
`docs/specs/flow-schema-version-v1.md`. Decision 0596 accepted the 1.0 plan; under G14 a change
that implements an accepted requirement is accepted automatically, recorded as the
`flow-schema-version` row (kind `change`, mode `auto`) of `docs/specs/openspec/acceptance-log.tsv`.

## What was built

- Version and supported list (task 2.1; FSV-V1-001, FSV-V1-002). `internal/appflows/flowschema.go`
  reads N from `profile` `corvint-flows/N` before any other member, so a newer file whose new
  members this build does not know is refused as unsupported, not as invalid. `flowsSchemas` is the
  closed, ascending list (today: 0); each entry names the member type the reader decodes for that
  N. N must be digits without a leading zero; anything else, or a missing prefix, is
  `FLOWS_DECLARATION_INVALID`. A well-formed N outside the list is `FLOWS_SCHEMA_UNSUPPORTED`; the
  typed `*FlowsRefusal` carries the found N as written (so an N too long for an int is still named)
  and the supported list. Version 0's member list and a golden file using every member are under
  `internal/appflows/testdata/flows-schema/`; the member-list test derives the reader's members by
  reflection and fails, naming added and removed members and saying to increment N, on a fixture
  type that adds one.
- One reader (task 2.2; FSV-V1-003). `ReadFlowsDeclaration` stays the one reader. The source scan
  lives in its own package, `internal/appflows/flowsreaderscan`, not in `internal/appflows`: it
  walks every non-test Go file of `cmd/` and `internal/`, and the test-read-scope check
  (AFP-V0-3010) pointed out that the whole-tree scope would select every appflows test for each
  edit under those trees. Only the scan now carries that scope. It flags a string literal ending in
  `.corvint/flows.json`, or the literals `.corvint` and `flows.json` in one file; help text and
  test files do not count. `internal/pathbrief`, which only checks that HEAD tracks the file, was
  the one other package naming the path; it now uses `appflows.FlowsDeclarationPath` (same import
  layer, no cycle). The scan fails, naming `internal/pathbrief`, when that change is reverted.
- Planner (task 2.3; FSV-V1-004). `JoinFlowPlan` maps the unsupported refusal to a `plan.e2e` with
  scope `FULL`, no listed selection and the unknown `FLOWS_SCHEMA_UNSUPPORTED`, whose detail is
  `.corvint/flows.json version N; supported 0` and so shows in the text summary
  (`e2e-unknown ...`). Unit and integration selections are those of a plan without the declaration.
  `corvint test` (`testE2EUnits`) reads that member as INCOMPLETE for every E2E test, never as no
  tests. The affected help and the APK-V0 flows-declaration definition name the reason.
- Reports (task 2.4; FSV-V1-005). `flows gaps`, `flows status` and `flows impact` in
  `internal/command/flows`, and the native `corvint flows test-evidence`, call
  `appflows.ReportRefusal` before anything else. A refusal exits 2 with code
  `flows-schema-unsupported` or `flows-declaration-invalid`, a message beginning with the reason,
  and empty stdout. The flows handlers are shared by `corvint-docs flows` and the `corvint flows`
  shim, so the documentation product refuses the same way; `internal/flowdocs` and
  `cmd/corvint-docs` were not edited, since neither reads the file.
- Writer (task 2.5; FSV-V1-006). `appflows.EncodeFlowsDeclaration` writes `FlowsWriteVersion`,
  the highest supported version, and returns the bytes only after this build's reader accepts them.
- Smoke (task 2.6, open; FSV-V1-007). `go run ./tools/flows-smoke --archive
  corvint_linux_amd64.tar.gz` (or `--binary FILE`) extracts only the `<root>/corvint` member, runs
  it with `PATH` set to that directory plus git's directory, and fails if any other `corvint` or
  `corvint-*` executable is on that `PATH`. It builds the fixture twice with fixed identities and
  dates, so the commits are the same on every host: once with profile `corvint-flows/0` and once
  with one above the highest supported version. The checks take an injectable runner. Tests feed
  them a stand-in candidate and prove the smoke fails when the unsupported-version fixture plans
  fewer than all E2E tests (read as version 0, a partial list under `FULL`, no member), when the
  reason is missing and when a report prints partial output.

## Differences from the proposal's tree (9ec3a29d0)

- `corvint flows` is no longer a `corvint` verb. Under decision 0595 (CMS-V0-913) it is a shim onto
  `corvint-docs`; only `flows test-evidence` stays native. The refusal therefore sits in the shared
  handlers (`internal/command/flows`) and in the native verb.
- No command writes `.corvint/flows.json` in this tree, so FSV-V1-006 is met by the one encoder
  that later writers must use, and by its round-trip test.
- `flows gaps`, `status` and `impact` take `--flows DIR` and did not read the declaration before
  this change. They now read it only to refuse; they do not take the intent directory from it
  (non-goal: no change to how flows select tests).
- "Select every E2E test" cannot list files when the file is refused, since the inventory (the
  flows directory or the Playwright discovery receipt) is named only inside it. The plan states it
  as scope `FULL` with no listed selection, and `corvint test` treats that as INCOMPLETE. The design
  also asks for the reason "on each such selection"; with nothing listed, the reason appears in
  `plan.e2e.unknown` and the summary only.

## What remains

- Task 2.6: the release-workflow job (a separate `.github` change) and the smoke log on the RC SHA.
  The real run on this tree (`go build ./cmd/corvint`, then
  `go run ./tools/flows-smoke --binary`) passes both affected checks and fails the four report
  checks: with `corvint` alone, `corvint flows status` and `gaps` refuse `not-installed`, because
  they forward to `corvint-docs`. The same two reports run through a built `corvint-docs` on the
  smoke's fixtures print a report for version 0 and refuse `FLOWS_SCHEMA_UNSUPPORTED` with empty
  stdout for version 1. Making those reports native in `corvint` is the PROD-DOCS-011 (G5) work,
  outside this change. Until it lands the smoke cannot pass.
- Task 3.1: `FLOWS_SCHEMA_UNSUPPORTED` in the user-docs error catalogue (change `user-docs`), which
  has not landed. FSV-V1-004's catalogue part is not yet built.
- Task 3.2: the ticket completion, which the orchestrator does.
- `openspec validate`: NOT_RUN, since the OpenSpec CLI is not installed on this host. The change is
  not archived.

## Rollback

Revert the branch's commits. The exact-profile reader returns, and every profile other than
`corvint-flows/0` reads as `FLOWS_DECLARATION_INVALID` again. `FLOWS_SCHEMA_UNSUPPORTED` stays
reserved.
