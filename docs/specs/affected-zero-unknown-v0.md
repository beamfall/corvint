# Affected Zero Unknown V0

Owner: Russell Lewis
Date: 2026-11-04
Requirement prefix: `AZU-V0`
Intent status: experimental (owner goal, owner chat 2026-11-04; orchestrator design under the 2026-11-02 delegation)
Delivery status: experimental
Authoritative inputs: `docs/specs/affected-plan-v0.md` (parent contract: wire, authority, AFP-V0-511
frontier bearing, AFP-V0-560 terminal rule, AFP-V0-601/602 data readers),
`docs/specs/affected-plan-selection-v0.md` (AFP-V0-2405 driver-input hint, AFP-V0-2440 fixture-only
claim), `docs/specs/test-selection-precision-v0.md` (TSP-V0-005 shell foreign callers, TSP-V0-942
frontier bearers), `internal/liveverify/affected`, `AGENTS.md` invariants 2, 3 and 8. The parent
spec is at its OCM obligation limit, so these requirements take a new prefix.

## Agent digest
- Claim: every `corvint affected` plan reaches `unknown: []` through sound selections or stated proofs, never by dropping a reached unit.
- Status: experimental (owner goal, owner chat 2026-11-04; orchestrator design under the 2026-11-02 delegation); delivery experimental.
- Exists: `frontierUnknowns`, `bearsOnUnowned`, `participants`, `unownedUnknown` and `driverInput` in `internal/liveverify/affected/select.go`; `FrontierReach` in `unit.go`; `.corvint/shell-foreign-callers.json`, `.corvint/runtime-readers.json` and `.corvint/test-binary-consumers.json`.
- Blocked on: aggregate sibling integration, empty unknown lists on every frozen merge, and historical full-suite/mutation receipts; lane verification retains sandbox CLI failures; the registered-driver assertion conflict is repaired without changing frozen receipts.
- Read next: User and current state, then the requirement you are implementing.

## User and current state

The owner directed the goal ("figure out how to get the unknown to 0 safely", owner chat,
2026-11-04), and the orchestrator made the design choices under the owner's delegation of
2026-11-02. Experimental candidates for driver ownership, inbound closure and frozen replay
exist; AZU-V0 is NOT_QUALIFIED.

The merge-forward verifier repair restores the base's registered driver and historical
namespace in default graph identities. Frozen strict/coverage receipts (AFU-V1-024) include
both that namespace and observed driver units. Explicit optional driver sets retain
AZU-V0-8905 absent-driver parity. The self-review repair reconciles the lane-only
`TestAffectedAuditDriverDoesNotChangeDefaultPlan_AZU_V0_002` with the accepted registered
registry: a changed Makefile selects its driver target with BOUNDED scope. Frozen receipt
assertions remain unchanged. Go raw-string directory joins also retain unknowns for
undeclared callers, and partial replay reports remain unaccepted until every listed merge
qualifies. AZU-V0-004/009 aggregate qualification still lacks historical full-suite/mutation
evidence; CLI sandbox failures remain explicit verification limits.
No observed graph vertex is hidden from a digest.

The user is the engineer, agent or CI shard that runs only the tests a change can affect. A plan
with any `unknown` entry has scope `UNKNOWN`, and every consumer that cannot trust it falls back
to a wide or full run. That fallback is the largest cost in this repository's build loop.

Measured on 2026-11-04 with the `8e2c13aa9` tree binary over eight consecutive `origin/main`
merges (`8e2c13aa9`, `39c031980`, `d436dff76`, `6c33df33b`, `ea770fa17`, `24d8c04f9`,
`f13f52b16`, `9fdc2f02e`): every plan was `UNKNOWN`, with 63 to 99 entries. At `8e2c13aa9` they
were `LANGUAGE_FRONTIER` 81, `UNOWNED_DIRTY_PATH` 5, `UNINDEXED_SOURCE_PATH` 2,
`NO_SELECTABLE_TEST` 2 and `UNSELECTED_BINARY_CONSUMER` 2. Single-file probes on the same tree:

| Dirty set | Frontier entries | Why |
|---|---|---|
| one owned Go file (`internal/gateselect/allowlist.go`) | 1 | `shell:foreign-invocation-unmodelled`, born by seven unbounded-reader shell units the change reaches |
| one owned TypeScript file (`extensions/vscode/src/configuration.ts`) | 69 | TypeScript participates, so every TypeScript entry bears (AFP-V0-511 (1)) |
| one owned shell file (`script/heavy.sh`) | 1 | the same shell entry |
| a merge carrying `Makefile`, `.github/workflows/*.yml` or another unowned path | 60 to 81 | every non-participating plugin's path-open entries bear on the unowned path (AFP-V0-511) |

So the frontier count has three sources, all closable: the unowned-path cascade, whole-plugin
participation, and inbound shell callers. The remaining classes are small and local.

## Requirements

- **AZU-V0-001:** **Driver claimant.** A built-in claimant SHALL own every driver input that
  `driverInput` recognises (make files at any depth, `.github/workflows/**`, `.github/actions/**`),
  so such a path is never `UNOWNED_DIRTY_PATH` and never triggers the AFP-V0-511 cascade. Its units
  are make targets and workflow jobs. Edges are the repository paths a recipe or step names
  literally: a script, a `go test`/`go run` package pattern, or another make target. A dirty driver
  file SHALL select the targets or jobs whose recipe or step text a diff hunk touches. A hunk
  outside every recipe or job (a variable, an `include`, `on:`, top-level `env:` or `defaults:`)
  SHALL select every target or job of that file. A recipe or step that invokes a computed name
  carries a `driver:computed-invocation` frontier entry at that target or job; AZU-V0-003 closes it.
  Selections carry kind `DRIVER_TARGET` and the argv `make <target>`, or `CI_JOB` and the workflow
  and job IDs. A consumer that cannot run a `CI_JOB` reports it as left to CI, never as unknown.
  Readers of a driver file by path literal, declared read scope or unbounded read stay selected as
  before. Supersedes the AFP-V0-2405 hint for claimed paths.
  Go package commands with no package argument SHALL depend on the current package after
  `cd` or `go -C`; an unresolved directory SHALL retain uncertainty or depend on every owner.
  YAML flow mappings outside the supported subset SHALL retain the unparsable-file frontier
  rather than silently discard execution members. Flow steps accepted as scalar text by the
  supported parser SHALL retain their job or action as a computed invocation when execution
  members are not modelled (AZU-V0-9703, AZU-V0-9852). Unparsable driver
  fallback units SHALL be reached for dirty paths not covered by the existing project-owned
  no-unbounded-reader proof (AFP-V0-772), preserving uncertainty even when the hidden execution
  edge cannot participate in traversal. Literal readers remain selected under that proof.
  Driver execution edges SHALL survive Go test-only containment (AFP-V0-036), selecting
  the target/job and its callers when only the invoked package's tests change; ordinary Go
  importers remain contained. Go's `-C=DIR` form SHALL resolve like `-C DIR`. Shell globs in
  Go package operands and unsupported inline interpreter programs SHALL resolve their
  dependencies or retain the invocation site as computed under AZU-V0-003.
- **AZU-V0-002:** **Ownership completeness gate.** `corvint affected --ownership-audit` SHALL list every
