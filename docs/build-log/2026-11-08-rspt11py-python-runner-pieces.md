# Python runner selection pieces (V1-5113)

- Lane: rspt11py; integration base: `89372490721bb960ff9261a930c94f8514a2056e`.
- Governing contract: decision 0592 and RSP-V0-001 through RSP-V0-005 in
  `docs/specs/runner-selection-plugins-v0.md`. This is a build-only delivery;
  behavior tests, independent review and retained qualification are NOT_RUN.
- Scope: compiled-in pytest and unittest pieces, including the coverage.py
  native evidence conversion absorbed from V1-4373. No public command, wire,
  error, golden, legacy execution profile or existing assertion changes.

## Implementation and limits

`registry.PythonPlugin` constructs either runner with inventory and evidence
present and qualification absent, reason `unqualified`. The existing core
inventory canonicalizer, exclusion planner, reconciliation and single FULL
retry remain responsible for the run set. The plugin never grants qualification.

The core-invoked embedded Python helper uses pytest collect-only hooks and
unittest discovery, reports source paths and native identities, and captures
terminal outcomes including skips with reasons. Its stdout encodes the existing
in-process RunnerInventory / RunnerResult Go shapes, not a new public protocol.
Commands carry literal selections; absent and empty selections remain distinct.
The core owns interpreter resolution, process containment, exit status and
capture bounds. The helper is not a new bundled executable. It imports Python
runner packages only in the explicitly invoked process.

Coverage collection uses coverage.py in-memory data and exports a JSON artifact
in the caller's run directory. The evidence factory accepts captured bytes,
never paths to read. Format 2/3 line records are checked against core-provided
Git blobs (including their object hashes). Definition spans and enclosing
relationships come from the existing Python grammar and syntax validator.
Excluded lines, malformed or oversized reports, unpinned sources and unsupported
syntax retain explicit absence or incompleteness. Loaded-module evidence is
absent (`python-loaded-unobserved`), because coverage files cannot prove it.
Every test of a shared process receives its entire coverage union. The core
supplies trace components through the existing T1 assembler; unavailable or
incomplete tracing is not upgraded. There is no record-store writer.

## Integration decision and remaining blocker

Use an explicit factory, as the already delivered script plugin does. Automatic
activation through SelectionPlugin and the language manifest remains PARTIAL:
`TestRSPV0001EveryExistingRunnerAbsent` asserts every runner piece is absent and
matches the legacy registry. The lane forbids weakening or replacing existing
assertions. Changing activation without that coordinated contract/test update
would knowingly break the existing suite. The factory is available for the
integration harness without modifying those assertions. No frozen output is
changed to work around this blocker.

RSP-V0-050 describes declared script commands, not unittest discovery; that row
is unchanged. Shared RSP-V0-001 through RSP-V0-005 traceability rows and the digest
are unchanged per the integration charter. Requirement/test mappings are in the
external lane handback for the orchestrator to integrate. Existing Python
mutation operators remain in the language mutation piece; no qualification
harness or mutation engine is duplicated.

## Verification and rollback

Focused registry vet (including test type checking) and gofmt passed.
`make -s spec-requirements-check line-citations-check` passed; this expressly
permitted target internally ran specindex and lrfrepo tests through heavy.sh.
Registry behavior tests and live interpreter invocations remain NOT_RUN;
fixture assertions are not passing-test evidence. The host owns package tests, independent review,
CEM binding/checking, qualification and ticket completion. The final build-only
instruction supersedes the earlier test and bind/seal instructions.

Rollback removes the Python factory, embedded adapter and fixture tests. Existing
registry behavior, evidence formats and persisted records need no migration.
