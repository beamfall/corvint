# Shell and declared make runner pieces (V1-5110)

Scope: RSP-V0-001 through RSP-V0-005; RSP-V0-040 step 4. Decision 0592
and decision 0526 govern. Lane base is
`89372490721bb960ff9261a930c94f8514a2056e`, the integrated base named by the
lane charter. The charter's final build-only restriction governs this delivery:
no test execution, qualification, independent review or CEM binding in this lane.

## Implementation decisions and limits

- `ShellSelectionPlugin` binds one Bats or ShellSpec source file to the core's
  checkout/configuration/target key. The caller admits the runner and shell
  executables and binds their configuration. Native Bats TAP and ShellSpec JUnit
  parsers are reused; no shell syntax parser, coverage collector or harness is added.
- Inventory consumes a core-captured FULL run with native counts and process
  status. Bats' public count command does not enumerate stable names, and shell
  code can discover tests dynamically. FULL collection avoids a static discovery
  guess. It costs a full run; collection latency is NOT_OBSERVED. A read command
  never launches this collection. Selected commands execute the entire bound file
  and explicitly report a superset; the shared T3 session reconciles every identity.
- Native identities are namespaced by runner and source file. Missing/null process
  status, malformed/oversized reports, missing terminal results, unreported skip
  reasons, unknown source paths and mismatched configuration cannot narrow.
- `TraceShellCommand` reuses the core script executor and tracer, retaining bounded
  stdout/stderr with real exit status. `ShellTraceRecords` reuses T13's immutable
  trace pinning; every test in the process tree receives every file component.
  No unqualified per-test markers are trusted. Native executed, regions and loaded
  components remain absent with `shell-no-line-coverage`.
- `ReadMakePlugins` reads only the pinned regular blob at
  `.github/cishards/test-commands.json`. Every declared literal `make TARGET`
  becomes a T13 script instance `script:make:TARGET`; identical target declarations
  coalesce. The declaration is unchanged. No Makefile is read or executed during
  inventory. Options, variable assignments, patterns and multiple goals refuse
  rather than invent independent target semantics. Other declarations remain T13's
  responsibility. Make target native components stay absent with `script-kind`.
- Bound factories follow T13's integration pattern. The existing generic registry
  and language-manifest absence declarations, frozen execution wires, errors,
  goldens and test assertions remain unchanged. Registering usable bound instances
  in a future caller requires its source/configuration inputs; the legacy name-only
  registry cannot supply those. There is no production selection enforcement here.
- Every bound runner has qualification absent with reason `unqualified`, even if
  a caller passes qualified=true. No boundary class narrows. Qualification records
  for data-edit, add, delete-rename and exec-target are NOT_RUN pending T6 and
  integration execution. Mutation/qualification infrastructure is owned by T6;
  RSP-V0-001..005 assign no shell mutation implementation in this slice.

## Evidence and checks

`script/heavy.sh GOTOOLCHAIN=local go vet ./internal/testrunner/registry` passed
(rc 0), type-checking old and new tests; semaphore queue 0 seconds, run 20.1 seconds.
`gofmt -l` on the six changed Go files produced no output. No Go tests ran.
`script/gen-spec-requirements.sh --write`, `--check`,
`script/spec-requirements-worktree-check.sh`, `make -s spec-readme-check`,
`script/check-line-citations.sh --repin` and `make -s line-citations-check` passed
(rc 0). Repin found no moved anchors. The spec change is confined to step 4's
RSP-V0-040 traceability row; shared rows and status/digest are unchanged. Generated
spec indexes did not change.

The full `make spec-requirements` and `make spec-requirements-check` targets invoke
Go tests, so their Go portions are NOT_RUN under the build-only instruction; their
static scripts above ran directly. Existing and new package tests, live shell
suites, qualification, repository gates, independent review, CEM/check/seal and
Tasks completion writes are NOT_RUN/NOT_PRODUCED, as explicitly assigned to the
orchestrator or forbidden in this lane. Rollback is removal of the new factories
and trace adapter; no record-store migration or existing wire change is needed.

## Self-use and friction

Pre-change `corvint query --task 'V1-5110 shell and make runner selection pieces'
--limit 5` succeeded: 5 included results, 2 omitted, 4 authoritative and 1 advisory;
31 test-path symbol candidates withheld. It reported an in-memory index build
because no snapshot matched. Raw receipts stay outside the repository in
`/home/agent/lanes/rspt8-evidence/`.

The initial `context` call used the directory `internal/testrunner/registry` as its
subject and refused it as untracked (rc 2). `impact` on that directory refused the
unsupported suffix (rc 2). The unchanged context query with subject
`internal/testrunner/registry/runner_plugin.go` later succeeded (rc 0), but it was
post-edit, read the base commit, omitted 14 of 19 candidates, and downgraded the
modified RSP spec as self-modified authority. This is not pre-change context proof
or acceptance of new intent. The original accepted spec and inlined ticket govern.
The build-only charter excludes selection, runtime-provider and whole-gate routes.

Retained receipt bindings (SHA-256; bytes):

- `query.json`: `91cb345560f174e02c9896df1217bcbbe9c756e60393c2007f9916faf2fc1287`; 16325 bytes.
- `context-file.json`: `415108dcf6fda5b88c80d10f252fea0c24118f66fa5458f903c54aa936d63b33`; 6469 bytes.
