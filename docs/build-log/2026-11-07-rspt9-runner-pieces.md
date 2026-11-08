# JavaScript/TypeScript runner pieces and shared E2E inputs

Date: 2026-11-07. Ticket: V1-5111. Build-only lane rspt9.

## Discovery decision — orchestrator-made under delegation

The Round 2 instruction, under the owner's 2026-11-02 delegation, resolves the
previous node-test discovery question: a selection unit may be a whole test
file. Use a documented native per-test list when available; otherwise inventory
the runner's actual test files. File selection executes all dynamic children.
Unknown versions/configurations, unsupported custom globs, malformed/capped or
failed lists stay FULL with inventory-unavailable, config-unsupported or
list-output-malformed. This is recorded in RSP-V0-018; no requirement is added.
The earlier blocker record is historical, not the current disposition.

The implementation admits stable Vitest 2–4, Jest 29–30.5, Playwright 1.50–1.63,
and Node 22.18–22.23. Jest 30.4–30.5 uses collectTests; older admitted versions
use listTests. Node uses its default JS/TS patterns or explicit tracked regular
files, with no unmodeled CLI/environment overrides. Other versions remain FULL.
The core must bind the effective configuration, version and captured tree to the
inventory key, admit the executable, and observe successful uncapped discovery.
No discovery process runs from a read command or from the plugin methods.

Original references (reviewed for the implementation, no live tools run):

- https://v2.vitest.dev/guide/cli#vitest-list
- https://raw.githubusercontent.com/vitest-dev/vitest/v3.2.0/packages/vitest/src/node/cli/cli-api.ts
- https://raw.githubusercontent.com/vitest-dev/vitest/v3.2.0/packages/vitest/src/node/reporters/json.ts
- https://jestjs.io/docs/30.0/cli
- https://jestjs.io/docs/30.4/cli#--collecttests
- https://nodejs.org/download/release/latest-jod/docs/api/test.html#running-tests-from-the-command-line
- https://raw.githubusercontent.com/nodejs/node/v22.22.1/lib/internal/test_runner/utils.js
- Existing Playwright JSON/report adapters in internal/browsercov/playwright.go
  and internal/testrunner/dynamic, including the existing Node event reporter.

## Contracts and limits

RSP-V0-002..005 use the T3 inventory, exclusion planning and reconciliation types
and the T1 native components. A new admitted-invocation factory parallels the
existing script factory; legacy lookup and frozen command/profile wires retain
their behavior. Native name filters that cannot prove exactness run a declared
superset; files are indivisible for Node and older Jest. Missing outcomes remain
visible to the shared FULL retry. The existing dynamic builders/reporters are
reused, including their scratch-file preparation, rather than duplicated.

RSP-V0-018/019: core tracing launches the runner, whose own Playwright webServer
configuration launches its server descendants with NODE_V8_COVERAGE. Core facts,
not config presence, decide whether the server was inside the tree and coverage
built. Reused/outside servers produce external evidence. Shared server coverage
joins every sharing test unless core has verified qualified reset/dump or a fresh
server for exactly one serial test. Workers=1 alone never proves isolation.
Combining browser/server components does not increase collection counts or invent
producers. The existing browser content/source-map adapter and JS body scanner
are reused. Regions certify only conservative whole-line interiors; unsupported
syntax and missing loader accounting stay incomplete/absent. Source text must
match its immutable Git blob. Existing extmutate JavaScript operators remain the
language mutation piece; no second harness or mutation engine was introduced.

RSP-V0-052: e2e-safe accepts an in-process RunnerEvidence binding and consumes the
same Playwright inventory and T1 records through T2. This branch never reads the
legacy provider or creates a second exclusion proof. T2 is still shadow-only:
even hypothetical supplied qualification cannot activate enforcement here.
The existing e2e-safe wire shape, codes and legacy behavior are preserved.
No CLI rollout is claimed. The new factory and core binding are the pieces for
the integration lane, not an enabled production selection switch.

Qualification records and live replay are NOT_RUN in this lane. All runner
qualification pieces remain absent with reason unqualified. Multi-project
Vitest report identities that cannot be matched reconcile as divergence/FULL;
no project identity is inferred from an ambiguous native report.

## Verification and rollback

Fixture tests cover the positive and negative paths for each owned requirement,
native inventory/result translation, source binding, conservative supersets,
shared/isolated/external servers, and the e2e-safe bridge. The missing review
regression reads the real registry read-scope declaration and verifies selection
for a change only to .github/cishards/test-commands.json, with a negative control
removing that entry from an otherwise identical fixture graph.

Tests are authored, not executed: the final build-only instruction delegates
package execution and independent review to integration. Vet type-checks all
six changed packages. Spec generation, the explicitly requested spec/citation
checks and citation repinning are retained separately in the handback; their
Makefile test prerequisites are not runner qualification evidence.
Rollback removes the factory/bridge and new piece files; existing record stores,
legacy runners and selection wires need no migration. Earlier commits e2436b855
and 903d26243 remain in history. The unrelated make/shell driver findings were
not changed.
