# Hidden Go packages, uncovered units and runnable consumers

Date: 2026-11-04
Owner direction: azu-small charter, V1-4688 / AZU-V0-005, V1-4689 / AZU-V0-006,
and V1-4690 / AZU-V0-007. Experimental local implementation; overall zero-unknown
acceptance, independent review and task completion remain with the orchestrator.

## Decisions and implementation

- AZU-V0-005 observes dot-directory packages through the bounded source walk on both disk
  and immutable filesystems. VCS metadata stays excluded; testdata stays fixture data.
  Explicit underscore packages also retain parse-failure uncertainty. Committed read scopes
  admit the same directories. Newly owning a hidden Go file must keep its testless readers'
  callers: ownership is not an execution edge from a shell reader to its tests.
- AZU-V0-006 inspects modelled/declared callers and selected readers before classifying a
  testless changed unit. Proven coverage gaps become optional `plan.uncovered` entries;
  unowned declared callers and undeclared inbound frontiers retain uncertainty. Foreign source
  embedding keeps AFP-V0-811's execution-coverage uncertainty. Full, compact and by-path
  plans retain gaps; text renders them. The frozen application-flow-impact/1 projection keeps
  coverage gaps in its legacy graph.unknown/UNKNOWN representation, preserving its golden.
- AZU-V0-007 keeps legacy consumer arrays and admits an extended `{commands, run}` object.
  Valid root argv selects a DECLARED_CONSUMER once per consumer, with its command-build witness;
  test-only and reader-only command reach do not trigger it. Invalid argv fails through the
  existing declaration frontier. The run map is pinned in graph digest and cache projections.
  Empty optional members preserve existing wires. No committed golden or core-freeze output changed.
- `tools/web-flows/package.json` has only `test: node --test test/*.test.mjs`; neither e2e
  script is in that script. Both require built core/observer binaries through environment
  variables. Their declared root argv is therefore `["bash", "script/web-flows-gate"]`, the
  existing supported wrapper that builds and supplies those binaries and runs both scripts.
  No new npm script, runner or background job was introduced by this lane.

## Merge replay

Merge: 8e2c13aa926fcaf1731fae71b27c41c41c8f09af; base: its first parent.
The before binary was built from the initial worktree at 83d703f98, whose Go sources equal
that merge. A private replay checkout at the merge supplies the same tracked inputs to both
binaries; it is used for read-only planning, never for dogfood binding. All raw receipts,
binaries and logs live outside this repository in `/home/russell/lanes/azu-small-evidence/`.

| Input / binary | Dirty paths | Selected units | Frontier | Unowned | Unindexed | No test | Binary consumer | Total unknown |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Frozen merge / before | 104 | 339 | 81 | 5 | 2 | 2 | 2 | 92 |
| Frozen merge / after | 104 | 342 | 81 | 5 | 0 | 2 | 2 | 90 |
| Merge paths + runnable declaration / after | 104 | 344 | 81 | 5 | 0 | 2 | 0 | 88 |

The first two rows use `affected --full --no-cache --base 8e2c13aa9^1` in the replay
checkout. The third overlays only this lane's consumer declaration, then uses the original
104 merge paths with `affected --full --no-cache --worktree --paths-from merge-paths.txt`.
It is explicitly a worktree-derived configuration experiment, not an immutable merge receipt.
All three scopes are UNKNOWN. A set comparison verifies that every before selection is retained
in both final after plans. Three newly indexed Go packages enter the selection, then two runnable
consumers enter it. The merge's two NO_SELECTABLE_TEST units have unresolved inbound callers;
AZU-V0-006 deliberately leaves both unknown. This lane does not claim zero unknowns.

Commands to build each binary: `script/heavy.sh go build -o <evidence>/corvint-before ./cmd/corvint`
and the corresponding `corvint-after` command. The after receipts below bind the worktree-built
binary and inputs; wall-clock replay performance was NOT_OBSERVED as a standalone measurement.

## Acceptance evidence and retained failures

- `TestAZUV0005HiddenPackages`: disk/immutable parity, hidden/underscore ownership, importer
  selection, testdata and VCS metadata exclusions. `TestAZUV0005KeepsTestlessReadersCallers`
  retains shell-reader dependents after hidden Go ownership.
- `TestAZUV0006CoverageGap`: isolated uncovered units, modelled and declared caller tests,
  unowned declared callers and undeclared inbound frontiers. The cross-language conformance
  fixture now asserts the exact uncovered record for closed graphs and retains unknown assertions
  for unresolved frontiers. Existing embedding uncertainty assertions remain intact.
- `TestAZUV0007RunnableConsumer` and `TestAZUV0007InvalidRun`: dependency/command/mixed changes,
  test-only/unrelated exclusions, exact argv, digest sensitivity and fail-closed invalid argv.
  Existing array-only consumer tests still assert UNSELECTED_BINARY_CONSUMER.
- `TestAZUV0006And007CompactPlan` checks compact and text preservation; existing by-path
  assertions compare all fields with deep equality after the optional argv addition.
- Whole `internal/liveverify/affected/golang`: PASS (12.998s); whole
  `internal/liveverify/affected`: PASS (132.485s); whole `internal/appflows`: PASS (13.598s).
  Commands: `script/heavy.sh HEAVY_WHOLE_REASON=<reason> go test -count=1 -timeout 30m ./PACKAGE`,
  one package per command, after `corvint affected --base 8e2c13aa926fcaf1731fae71b27c41c41c8f09af`.
- Whole `cmd/corvint` was run and failed: the required TMPDIR is below the parent Git repository
  `/home/russell/lanes`, so tests expecting a non-repository scratch location observe that parent;
  source qualification also refuses its group-writable, non-sticky ancestor. The frozen flow
  golden mismatch found in that run was fixed through the legacy projection above, without
  changing the golden. The terminal whole-package rerun also failed (445.174s) on the external scratch/source
  qualification failures; its exact failures and repros are retained in the handback. Focused CLI checks for compact/text output, by-path
  projection, the frozen flow golden and both core-freeze groups PASS (1.437s).
- A transient replay incorrectly dropped two shell tests (`agent-preflight_test.sh` and
  `check-windows-main_test.sh`) after ownership changed. The selection-set check caught it;
  testless hidden-source readers now retain their callers, and both final replay comparisons
  have zero missing before selections. Earlier receipts are retained as failed evaluations.
- Initial whole affected runs failed the unchanged 100ms/5-probe live-budget gate under host
  contention: one best p95 was 114.123034ms at 5.15 probes. The final whole run passed unchanged
  thresholds. No timing assertion was weakened. The coverage proof avoids allocating and sorting
  a second witness traversal after this measured failure.

## Process and limits

Used: native context, query and impact with the original task, affected package selection,
merge replay, CEM/OCM dogfood binding and reviewer reports. Query did not abstain and query/impact
reported no critical missing inputs. The automatic prechange collector refused
`prechange-after-change`: the supplied branch starts at 83d703f98, after the requested base,
and the named zero-unknown spec is absent at that base. Manual original receipts remain retained;
automatic prechange query/impact receipts are NOT_PRODUCED. The start dogfood-change pass ran
and retained its expected missing binding-input reasons.

NOT_RUN: repository-wide make gate (owner's focused-check policy), independent review (orchestrator
owns it), full eight-merge safety qualification (other lanes / AZU-V0-009), real-browser web-flows
qualification (no browser behavior changed; the existing wrapper starts background processes,
which this lane was directed not to run), Windows vet (no push), push/PR/task writes/seal (forbidden
by charter). External interoperability and external outcome validation are NOT_PRODUCED by this
local planner change. Overall outcome and exact remaining host checks are in the handback.

Line citations were repinned from staged source and checked; spec requirements were regenerated.
The original test function name remains stable for the parent's traceability row. An initial
traceability check caught its temporary rename; restoring the name fixes the row without weakening
its assertions. A commit attempted to create `/home/russell/projects/corvint/.git/packed-refs.lock`
and printed `Read-only file system`; the task commit itself succeeded. No Git configuration or
permission override was used.
The seal identity is Russell Lewis <me@russe11.com>. The checked CEM is intentionally left unsealed.

## Retained SHA-256 digests

- `before-merge.json` (431140 bytes): `aa8799a6bf02c040f30f144fc3f35a305d8ca2d97c429a7d71503ad65fc5233c`.
- `after-merge-final.json` (432335 bytes): `e971662e6b31bdc73edfc7928681616ed04bd79ddcdd0cbd3f7ca9016abf08bf`.
- `after-configured-final.json` (430193 bytes): `1dd49f18dd4d3d58a41c306cbf6474e156d42e75281a69810e2518faf5261153`.
- `corvint-before` (57885290 bytes): `77f243de966ff2958b9633b5c7a7cae71dbeff0c1cf64f2c94476e9ff1d85e59`.
- `corvint-after` (57919439 bytes): `8cfc278440ebdc3f7a6a797f46b18d54d33a6f96149f3450d1d9d7530d4aed04`.
- `context.json` (7559 bytes): `5ac505814c3089bfa7600b4733ad7042be86c0839916f75dbd2c56910d376702`.
- `query.json` (19192 bytes): `7402920b21888467dbeca461396bd0a4c20dc619fb735644672fe1bc131d30f9`.
- `impact.json` (17540 bytes): `959ff8e06bce137e11266da303fa5edec7930fc4896f9e1c5f977f37e05f6bcc`.
- `test-golang-terminal.log` (744 bytes): `fd1e22795510c13b7c6aeee1b1e8f3cdbe583b1fd5212ea74e23d128c851eefd`.
- `test-affected-terminal.log` (799 bytes): `58769e907858730c8d5f2ae998fd7e3e0d4cf821f8ff024a6bf94bf80f085731`.
- `test-appflows-whole.log` (593 bytes): `0cd2943eafa77ca4fdf38aac9c8dde270f44135015a0cdb5f9413f8a80fe0057`.
- `test-cmd-whole.log` (70077 bytes): `179dc57e8f17762d0476dc758798baeeecf7613ea737c28601b1555647b4afb6`.
- `after-merge.json` (432302 bytes): `e167c4b3a15b3381c442ac228f0e03f3ab121a7cfcac8c922e3d8583abf405d4`.
- `test-cmd-final.log` (68495 bytes): `1b063ef817cd67222c35da67c9944a9ee17dd12de2d588ed3dbca1568a0c23ec`.
- `test-cmd-focused-final.log` (898 bytes): `966baa988d313ffd494cd658dd4cb020712f7ae99f5da4c49449d81a5eaf99eb`.
- `spec-requirements-final.log` (1763 bytes): `7e5d1c3dad6ccae7c74c1913208bedc628d2a882d0ed3eaae7db065d6b88aa2f`.
