# Decision 0595: `corvint flows` and `corvint how` move to the docs app

- Date: 2026-11-09
- Status: accepted (owner, 2026-11-09).
- Authority:
  - Owner chat, 2026-11-09, on where the `flows` and `how` commands belong under the layer
    direction of decision 0594 item 8: "move them to docs as that's where they make the most
    sense."
  - Owner constraint, same day: the testing side must still be able to read flows, because flows
    are linked to tests (code -> flow -> E2E test selection). The flow data model and the
    read-only flow readers that test selection needs stay importable by both the tests and docs
    layers; only the commands and docs-only generation move. The tests layer must not import the
    docs layer or run `corvint-docs` to read a flow.
  - Owner instruction, same day: the move's requirements are numbered and each is traced to a
    maintained test, not asserted.
- Amends: decision 0594 item 3, which listed `flows` and `how` among the verbs `corvint` keeps
  natively beside the forwarded `docs`. `docs/specs/core-bundled-apps-v0.md` gains
  `CMS-V0-912..916`; `docs/specs/capability-catalog-v0.md` (`CAP-V0-007`, `CAP-V0-015`,
  `CAP-V0-027`, `CAP-V0-960`), `docs/specs/import-ratchet-v0.md` and the traceability note of
  `docs/specs/application-flow-understanding-v1.md` follow.
- Keeps: every `flows` and `how` argument vector, flag, output schema, exit code and refusal;
  `internal/appflows`, `internal/appmap` and `internal/flowcoverage` in the `shared` layer; the
  MCP flows tools, which call `internal/appflows` in process and never ran the CLI.

## Context

After decision 0594 `corvint docs` was already an exec-only shim onto `corvint-docs`
(`CMS-V0-003`), but `corvint flows` and `corvint how` still ran inside `corvint`. Their handlers
live in `internal/command/flows`, which the import manifest keys `docs`, so `cmd/corvint` (keyed
`tests`) carried the tolerated wrong-way edge `cmd/corvint -> internal/command/flows`.

The flow readers are a different matter. `tools/import-ratchet/areas.json` keys
`internal/appflows`, `internal/appmap` and `internal/flowcoverage` as `shared` (layer 0), so both
`tests` and `docs` (layer 1) may import them. Test selection does: `cmd/corvint`,
`internal/testplan` and `internal/testsource` read flows through those packages in process. No
package had to move to honour the owner constraint.

One subcommand crosses the layers the other way. `flows test-evidence` joins flow links to test
evidence, and its implementation was injected from the tests layer through `flows.Env`. The docs
layer may not import the tests layer, and `corvint-docs` cannot receive that injection.

## Decision

1. `corvint-docs flows ...` and `corvint-docs how ...` run the existing handlers
   (`internal/command/flows`) with the same output, exit codes and refusals; help spells the
   binary `corvint-docs` (`CMS-V0-912`).
2. `corvint flows ...` and `corvint how ...` are exec-only shims onto `corvint-docs`, with the
   `corvint docs` shim's behaviour: unchanged argument vector, the child's bytes and exit code, no
   authority passed, `not-installed` (the named `NOT_INSTALLED` gap, exit 2) when no
   `corvint-docs` is beside `corvint` or on PATH, native help that names `corvint-docs`, and
   `--require-clean` refused by the shim (`CMS-V0-913`).
3. `flows test-evidence` stays native in `corvint`; `corvint-docs flows test-evidence` refuses
   with exit 2 and names `corvint flows test-evidence` (`CMS-V0-914`). This is the smallest sound
   option: forwarding it would need either a docs -> tests import or a second process hop back
   into `corvint`, and the subcommand is the tests layer's own read of flow links
   (`RSP-V0-051`), so it belongs where it already was.
4. The tests layer reads flows only through the `shared` packages, in process, with no
   `corvint-docs` installed (`CMS-V0-915`). The three packages stay `shared`.
5. No cross-layer edge is added. `tools/import-ratchet/baseline.json` drops
   `cmd/corvint -> internal/command/flows` (18 -> 17 edges) and no baselined edge may again start
   at `cmd/corvint` or end at the flows command package or `cmd/corvint-docs` (`CMS-V0-916`).
6. The capability catalogue follows the `docs` precedent: the `corvint flows` row is a shim row,
   the `corvint-docs` command tree catalogues the forwarded subcommands, and no `flows` table is
   tracked in `corvint` any more. The `corvint` tree still supplies what `corvint` answers itself
   (`flows test-evidence`, the `flows appmap build` help).

## What moves and what stays

No package changes directory or layer. The command wiring moves between two `main` packages:

| Code | Before | After |
| --- | --- | --- |
| `--root` preamble, `how` alias, `runFlows` wrapper | `cmd/corvint/flows.go` | `cmd/corvint-docs/flows.go` |
| `flows` and `how` help topics and tree nodes | `cmd/corvint/help.go` | `cmd/corvint-docs/help.go` |
| Subcommand handlers | `internal/command/flows` (docs) | unchanged; help spells `corvint-docs` |
| `flows test-evidence` implementation | `cmd/corvint`, injected through `flows.Env.TestEvidence` | `cmd/corvint/flows_test_evidence.go`, called directly; `Env.TestEvidence` removed |
| `corvint flows` / `corvint how` dispatch | native call into `internal/command/flows` | `cmd/corvint/flows_shim.go`, onto the existing `forwardToDocs` |
| Flow model and readers | `internal/appflows`, `internal/appmap`, `internal/flowcoverage` (shared) | unchanged |
| MCP flows tools | `internal/appflows` in process | unchanged |

## Layer rule

`tools/import-ratchet/areas.json` is unchanged. It keys `internal/appflows`, `internal/appmap` and
`internal/flowcoverage` as `shared` (layer 0); `cmd/corvint`, `internal/testplan` and
`internal/testsource` as `tests` (layer 1); `cmd/corvint-docs` and `internal/command` as `docs`
(layer 1). A package may import its own area and lower layers, so `tests` and `docs` both import
the shared flow readers and neither imports the other. `tools/import-ratchet/baseline.json` lists
the tolerated exceptions and only shrinks (`IRT-V0-004`).

## Command surface before and after

| Invocation | Before | After |
| --- | --- | --- |
| `corvint flows SUB ...` (not `test-evidence`), `corvint how ...` | runs in `corvint` | forwarded to `corvint-docs` with the same argument vector; same stdout, stderr and exit code |
| `corvint flows test-evidence ...` | runs in `corvint` | unchanged |
| `corvint flows --help`, `corvint how --help`, `corvint help flows`, `corvint help how` | full group help | short native help that names `corvint-docs help flows` and `corvint-docs help how`; `test-evidence` and `flows appmap build` help stay in `corvint` |
| `corvint flows ... --require-clean` | reached the flows handlers, none of which takes the flag | refused by the shim before forwarding, exit 2, as for `corvint docs` |
| `corvint-docs flows ...`, `corvint-docs how ...`, `corvint-docs help flows`, `corvint-docs help how` | argument error | the group, with synopses spelled `corvint-docs` |
| `corvint-docs flows test-evidence` | argument error | refusal, exit 2, naming `corvint flows test-evidence` |
| `corvint help --json` | lists the `flows` subcommands | lists only what `corvint` answers itself; `corvint-docs help --json` lists the group |

## Failure modes

- `corvint-docs` is neither beside `corvint` nor on PATH: `corvint flows ...` and `corvint how ...`
  refuse with `not-installed` (`NOT_INSTALLED`), exit 2; nothing runs and nothing is written.
  `flows test-evidence` and help still answer.
- An installed `corvint-docs` older than this decision: it answers `flows` and `how` with its own
  argument error, exit 2 (inferred from its dispatcher; not exercised by a test). The two binaries
  are released together.
- `corvint-docs flows test-evidence`: refusal with the pointer, exit 2 (`CMS-V0-914`).
- A subcommand registered in `internal/command/flows` but absent from the `corvint-docs` help
  tree: `TestCMSV0912EveryFlowsSubcommandIsInTheCommandTree` fails.
- A flow package re-keyed out of `shared`, a tests-layer production import of the flows command
  package, or a re-baselined `cmd/corvint` edge: `TestCMSV0915FlowReaderIsSharedAndFlowCommandsAreDocs`
  and `TestImportRatchet` fail.

## Limits recorded with the decision

- `cmd/corvint`'s tests import `internal/command/flows` in test files only, to build flow fixtures
  and as the in-process side of the parity test. The ratchet reads production imports only
  (`IRT-V0-003`), so this is not a baseline edge; it goes when those fixtures move.
- Eleven tolerated baseline edges still lead out of the shared flow packages into the docs or
  tests layers (`internal/appflows`, `internal/appmap` and `internal/flowcoverage` importing
  `internal/doccorpus`, `internal/flowdocs`, `internal/jstestprovider`,
  `internal/liveverify/affected` and `internal/testvaliditydoc`). They predate this decision and
  are unchanged by it; the build-log entry lists each with its cause.
- A user with `corvint` alone loses `flows` (other than `test-evidence` and help) and `how` until
  `corvint-docs` is installed beside it; the refusal names the gap.

## Rollback

Revert the change: `corvint` dispatches `flows` and `how` natively again and
`cmd/corvint -> internal/command/flows` returns to the baseline. No wire format, flag or stored
state changed, so nothing persisted depends on either side.
