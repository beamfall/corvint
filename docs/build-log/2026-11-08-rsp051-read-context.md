# RSP-V0-051 read context

Decision: **orchestrator-made under delegation** (owner 2026-11-02), recorded for V1-5116.
The release reads existing T1 records and the linecov index extension. It does not create a
persisted inventory or admitted-qualification contract. Missing inventory is UNKNOWN
`inventory-unavailable`; absent qualification explains a runner FULL `unqualified`; no fresh
record is UNKNOWN `no-fresh-record`; stale records include `stale-record` and the freshness
reason; unresolved independently established run context is UNKNOWN `run-context-unresolved`.
Record identities never establish their own expected context. These abstentions constitute the
specified release delivery, not runner qualification.

Implementation decision: one shared projection uses existing record, freshness, index and
selector types. CLI JSON/text, opt-in MCP `test-evidence`, and the additive AFU
`flows test-evidence` join expose it without changing existing record or flow wires. MCP remains
opt-in to preserve default and test-validity tool sets. The AFU join uses the existing HEAD
lookup and refuses a historical mismatch; the standalone query accepts committed revisions.
Files are rooted and bounded; unsupported or malformed artifacts make the answer UNKNOWN.
There is no runner execution, inventory discovery, evidence admission or repository writer.

The source includes per-shape JSON/text goldens, positive and negative reachability and selector
fixtures, every named abstention, bounded/unsafe input cases, command/MCP/AFU integration checks,
and byte snapshots including Git and trace state. Task-test execution is **NOT_RUN**: this lane's final
build-only directive reserves all tests and independent review for integration. CEM binding,
dogfood checks and seals are **NOT_PRODUCED** by the same explicit directive; the orchestrator
repins them. No task-store writes, publication, live qualification or exhaustive gate was run.
The selected self-development route is the evidence read/AFU route; live providers, collection,
learning and qualification are excluded because the task is a read surface with no run context.

Rollback: remove the new command/group/projection and their help/catalog entries; existing
records, indexes, runner selection and default MCP profiles require no migration.

Process friction: `make spec-requirements` silently invokes the existing
`internal/lrfrepo` `TestSpecsStayWithinTheOCMObligationBound` test after generation; the target
returned 0. That one existing test ran unintentionally under the build-only directive.
`spec-requirements-check` also embeds Go tests, so its static script components are checked
separately and its embedded tests are deferred. No task test or live qualification was run.

Build checks: `GOTOOLCHAIN=local go vet` through `script/heavy.sh` returned 0 for
`internal/testevidencedoc`, `cmd/corvint`, `internal/mcp/cli`, `internal/mcp/toolgroup`,
`internal/command/flows`, and the changed embedded `internal/capcatalog` package. CLI discovery
artifacts were regenerated from help. Static spec generation/check scripts and the line-citation
check passed. Help insertions moved ten citations; the command-list anchor in `docs/AUTOMATION.md`
was read and updated because the new command changed its cited span. No existing golden or
core-freeze output was changed; the query goldens are new, unexecuted fixtures.
