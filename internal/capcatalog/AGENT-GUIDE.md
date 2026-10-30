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
   For changed paths instead of a base, pass them all in one call (repeat
   --path, or list them with --paths-from FILE): one call builds the graph
   once. Repeated calls on an unchanged checkout reuse the verified graph from
   a private cache under the user cache directory, outside
   the repository; the plan is the same with or without it. Pass --cache DIR
   to keep it elsewhere, or --no-cache to build the graph on every call;
   `corvint affected cache` shows where the cache is and how large it is.

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
   Data files no test reads can be declared in .corvint/documentation-paths.json;
   plan.declaration then records its path, sha256 and matched dirty paths, and
   a declared path that the config or a source the Playwright run loads names
   refuses the declaration (playwright:declared-path-contradicted) and keeps
   the full suite; a standalone tool outside that closure is only listed in
   plan.declaration.outOfClosure.

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

8. Discover tests statically from committed files
   corvint docs corpus discover-tests --runner playwright --revision FULL_COMMIT --path e2e/checkout.spec.ts
   Output: corvint-test-discovery/1 with trust generated: each example's title
   path, tags, expectations, navigations, reached modules and hooks, each
   anchored to the committed blob, and unknowns with reasons. --runner rspec
   reads RSpec files. It runs nothing.

9. Turn per-test coverage into tested_by evidence
   Step 1: record the test run and pin the coverage file it wrote:
   corvint flows ingest --format go-test-json --from go-test.json --run-id run-1 --runner-version 1.0.0 --source-commit FULL_COMMIT --source-tree FULL_TREE --source-clean --build-artifact-digest SHA256 --environment-id ci --environment-digest SHA256 --fixture-id none --fixture-digest SHA256 --cleanup done --coverage-artifact lcov=trace.lcov > run.jsonl
   Output: test-run-evidence/0 JSONL on stdout.
   Step 2: join one test's coverage to its run record. trace.lcov must hold
   that test's coverage only, and --test-key names the test's key in run.jsonl
   (PACKAGE > TEST for go test). It may be left out only when run.jsonl holds
   one record:
   corvint flows ingest-coverage --format lcov --from trace.lcov --evidence run.jsonl --revision FULL_COMMIT --test-key "example.com/upload > TestRetry"
   Output: corvint-coverage-ingest/1: observed tested_by relations with executed
   line spans, links the coverage contradicts, and unknowns with reasons.
   Coverage from another commit, a dirty tree or an unbound run is not joined.
   --format also takes v8, istanbul and rspec. It only reads.

10. Build and query a documentation corpus
   corvint docs corpus manifest --revision FULL_COMMIT --scope docs --timestamp 2026-10-23T00:00:00Z > manifest.json
   corvint docs corpus build --manifest manifest.json > corpus.json
   corvint docs corpus search --artifact corpus.json --query "retry policy" --kind symbol --include claims
   corvint docs corpus claims --artifact corpus.json --query "retry policy"
   corvint docs corpus claims --artifact corpus.json --path internal/upload/retry.go
   Output: bounded JSON. search ranks subjects by field-weighted text; --kind
   keeps some subject kinds and --include claims adds claim hits to them, not
   a claims-only filter, naming each hit's kind in result_kinds. With claims a
   source declaration ranks by its symbol name, not its file path.
   claims --query ranks claims alone by text.
   Both name each result's rank, tie group and score in result_ranks; a
   document result also names its matching paragraph in match_claim and
   match_anchor. search lists query words nothing holds in unmatched_terms.
   A generated claim that names code carries a claim_mentions check of whether
   its cited spans hold those names. Generated evidence grants no task or
   test-selection authority. The corpus is also served by the
   corvint-corpus-mcp server, whose docs_search takes kind and include.

11. Regenerate generated claims only where sources changed
   corvint docs corpus delta --previous previous-corpus.json --artifact corpus.json --format jsonl
   Output: a header line, then every worklist entry once: carry-forward, moved,
   changed, removed, unresolved or uncovered. Without --format jsonl it pages
   one receipt. A record whose span hash is not confirmed is never carried.

12. Bind a change to its evidence and verify it
   corvint cem prepare --base FULL_COMMIT_ID --target HEAD
   Commit the candidate, then run the verification action that prepare
   returns. It has this shape, naming the commit that holds the candidate and
   the absolute repository root:
   corvint cem verify --map .corvint/change.cem.json --expected-base FULL_COMMIT_ID --target FULL_TARGET_COMMIT_ID --artifacts ABSOLUTE_REPOSITORY_ROOT
   Output: a cem/1.0 candidate map, then its verification.

13. Review a verified change in a fixed order
   corvint frontier --cem MAP --ocm MAP --expected-base FULL_COMMIT_ID --target HEAD --json
   Output: the deterministic Change Frontier review queue.

14. Identify the installed binary
   corvint --version --json
   Output: corvint-version/1.

15. Adopt change-compiled E2E tests (experimental)
   corvint flows appmap build --manifest appmap.json --revision HEAD
   Output: application-map/0, which checks the committed hand-written
   manifest. The chain then runs in the corvint-test companion: e2e catalog,
   obligations, compile, prove and shadow. The workflows member of
   `corvint help --json` lists each step's flags, inputs and commits in order,
   each step's --help prints them, and docs/E2E-ADOPTION.md is the guide.

Non-capabilities

Each item is a capability Corvint does not have today. Each one names an open
tracking issue or an undelivered requirement, or states that it is excluded by
design and names the requirement that excludes it. A test fails when an item
names an issue that internal/capcatalog/NONCAPABILITY-STATUS.tsv records as
closed, or a requirement it records as delivered.

- Product boundary: no network access, telemetry, account or permanent
  daemon. By design (DCW-V0-001; AGENTS.md product invariant 7).
- Product boundary: no embeddings, hosted service or semantic vector search.
  By design (WQO-V0-033; AGENTS.md product invariant 7).
- Test selection: affected --path can leave tsconfig paths aliases unresolved
  (typescript:path-alias-unresolved), so specs that import a helper through
  the alias are not selected. Tracking #1604.
- Capability documentation: the guide ships inside the binary, not as a
  separate file in the release archive, because a seventh archive member would
  break deployed updaters. By design (CAP-V0-029).
- Capability catalog: inputs and outputs are not separated per command. They
  stay null with a reason, and schemaIds lists every schema a command's help
  names. By design (CAP-V0-016: the registry cannot derive them, and the
  catalog has no hand-written fields).
