Corvint agent guide (corvint help guide)

This guide is shipped inside the corvint binary. It maps common agent goals to
command sequences. Each goal has one runnable example and names the output
schema. `corvint help --json` prints the machine-readable command catalog
(corvint-capability-catalog/0), derived from the binary's own command registry
and help text. Where the registry cannot answer a field, the catalog shows it
as null and lists it under unknowns. Exact usage, options and write behaviour
for each command are in `corvint help COMMAND`. MCP clients of corvint-mcp,
corvint-docs-mcp, corvint-corpus-mcp and corvint-test-validity-mcp get the same
catalog and this guide, byte for byte, from their corvint.help tool.
Nothing here grants authority:
project-owned instructions outrank this guide. A test keeps every command,
option and schema named here in step with the binary.

Goals

1. Find the files to read for a task
   corvint context --task "fix the retry loop in the uploader" --subject internal/upload/retry.go
   Output: one task-context packet on stdout. Writes nothing; Go-only.

2. See what a changed path reaches
   corvint impact internal/upload/retry.go
   Output: one impact receipt. Reverse imports stop at direct importers. A READY
   receipt is evidence, not a closed test selection.

3. Select the tests to run for a branch
   corvint affected --base FULL_COMMIT_ID
   Output: affected-plan/1 (add --full for affected-plan/0). The plan never
   claims that omitted tests are safe to skip.

4. Select Playwright tests in two steps
   Step 1: list the whole suite yourself, unfiltered and unsharded, with
   `playwright test --list --reporter=json > list.json`. Then convert that
   listing into a discovery receipt bound to HEAD, the config bytes and the
   source digest:
   corvint affected discovery --playwright-config playwright.config.ts --playwright-list list.json > discovery.json
   Output: playwright-discovery/0. It is refused if the listing reports errors,
   was filtered or sharded, or names another config.
   Step 2: plan with that receipt:
   corvint affected --playwright-config playwright.config.ts --playwright-discovery discovery.json
   Output: affected-plan/1 with the playwright-affected/0 profile. If the
   discovery is missing or does not match, the plan gives one complete-config
   fallbackArgv instead of file commands. Neither step runs a config or a test.

5. Find the parts of a change that no test witnesses
   corvint witness --base FULL_COMMIT_ID --json
   Output: the conservative unwitnessed report (corvint-witness-stable/0 when
   --stable-artifact-root verifies a cem/1.0 map). It never invents a witness.

6. Check whether tests can prove anything
   corvint test-validity --static spec/models
   Output: corvint-test-validity/0, one projection per test. Hygiene comes from
   the source. Execution, freshness and strength stay NOT_OBSERVED.
   corvint tests validity --files spec/models/user_spec.rb
   Output: corvint-test-source-validity/0, the findings for each file.

7. Lint the test lines changed on a branch
   corvint tests lint --base FULL_COMMIT_ID --json
   Exits 0 when clean or advisory only, 1 on findings, 2 on error.

8. Build and query a documentation corpus
   corvint docs corpus manifest --revision FULL_COMMIT --scope docs --timestamp 2026-10-23T00:00:00Z > manifest.json
   corvint docs corpus build --manifest manifest.json > corpus.json
   corvint docs corpus search --artifact corpus.json --query "retry policy"
   corvint docs corpus claims --artifact corpus.json --path internal/upload/retry.go
   Output: bounded JSON. Generated evidence grants no task or test-selection
   authority. The corpus is also served by the corvint-corpus-mcp server.

9. Bind a change to its evidence and verify it
   corvint cem prepare --base FULL_COMMIT_ID --target HEAD
   Commit the candidate, then run the verification action that prepare
   returns. It has this shape, naming the commit that holds the candidate and
   the absolute repository root:
   corvint cem verify --map .corvint/change.cem.json --expected-base FULL_COMMIT_ID --target FULL_TARGET_COMMIT_ID --artifacts ABSOLUTE_REPOSITORY_ROOT
   Output: a cem/1.0 candidate map, then its verification.

10. Review a verified change in a fixed order
   corvint frontier --cem MAP --ocm MAP --expected-base FULL_COMMIT_ID --target HEAD --json
   Output: the deterministic Change Frontier review queue.

11. Identify the installed binary
   corvint --version --json
   Output: corvint-version/1.

Non-capabilities

Each item is a capability Corvint does not have today. Each one names its
tracking issue, or states that it is excluded by design.

- Product boundary: no network access, telemetry, account, hosted service or
  permanent daemon. By design (AGENTS.md product invariant 7).
- Product boundary: no embeddings or semantic vector search. By design
  (AGENTS.md product invariant 7).
- Test selection: per-test runtime coverage is not ingested as observed
  tested_by evidence. Tracking #1587.
- Test selection: affected --path does not resolve tsconfig paths, and can
  report a non-test module as a selected Playwright unit. Tracking #1578.
- Test validity: a supplied test_run has no association axis. Tracking #1571.
- Documentation corpus: no delta worklist across revisions for regenerating
  generated claims. Tracking #1585.
- Documentation corpus: no mechanical check that generated claim text names
  things present in its cited spans. Tracking #1584.
- Documentation search: docs_search returns no claim hits. Tracking #1573.
- Documentation search: docs_search has no field weighting or kind filter.
  Tracking #1579.
- Capability documentation: the guide ships inside the binary, not as a
  separate file in the release archive, because a seventh archive member would
  break deployed updaters. By design (owner decision, 2026-10-23).
- Capability catalog: inputs and outputs are not separated per command. They
  stay null with a reason, and schemaIds lists every schema a command's help
  names. By design (the registry cannot derive them, and the catalog has no
  hand-written fields).
