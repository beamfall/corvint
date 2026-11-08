# JavaScript runner evidence integration repair

Base: `fce5b85b53306df6a0b276245ac04ae3c6097add` (the integration head supplied
for this repair). The earlier `9761ea1f459c0d3b3c963953e24a76eaeda16370` is the
original lane base; intervening integration changes are outside this repair.
Authority: owner repair charter, decision 0592, `RSP-V0-002`, `RSP-V0-003`,
`RSP-V0-013`, `RSP-V0-019`. Status: implemented; test execution NOT_RUN.

## Verified defects and decisions

1. `JoinServer` returned an untouched, complete browser record for every other
   sharer of an isolated reset/dump. That record had no server dependencies and
   could exclude the test on a server change. Unjoined complete records now have
   executed incomplete with `browser-server-dump-pending`. Existing uncertainty
   remains uncertainty. The single-dump entry point delegates to an in-process
   `JoinServerDumps` collection join, which records attribution locally and marks
   pending sharers after joining available dumps. Every dump must name the same
   complete sharer set. Callers replay a growing collection against original
   browser records, rather than inferring attribution from overlapping paths or
   clearing an incomplete reason on persisted records. This bounded batch helper
   is needed to complete all sharers without adding progress fields to the frozen
   evidence record. It adds no wire fields, persistence, or runner activation.
2. Inventory included Vitest project names while terminal IDs used an empty
   project. The native JSON reporter has no project field in the inspected
   [2.1.9 source](https://raw.githubusercontent.com/vitest-dev/vitest/v2.1.9/packages/vitest/src/node/reporters/json.ts),
   [3.2.0 source](https://raw.githubusercontent.com/vitest-dev/vitest/v3.2.0/packages/vitest/src/node/reporters/json.ts),
   and [4.0.0 source](https://raw.githubusercontent.com/vitest-dev/vitest/v4.0.0/packages/vitest/src/node/reporters/json.ts).
   Choose the charter's fail-closed option: any nonempty inventory project name
   refuses with existing `config-unsupported`, including one named project.
   Empty-project inventories and terminal result parsing retain their contracts.
3. `dynamic.Build` explicitly rejects commas in selectors. `SafeRunnerPath` now
   does so too. Discovery uses its existing unsafe-path reason,
   `inventory-unavailable`; execution retains its exact unsafe-selector error.
   No execution errors, goldens, core-freeze outputs, or JSON shapes changed.

## Requirement evidence

| Requirements | Regression fixtures |
|---|---|
| RSP-V0-019 | Updated `TestRSPV0019ServerSharedAndResetDump`: pending sharer and complete collection in both dump orders |
| RSP-V0-019, RSP-V0-013 | `TestRSPV0019NeverJoinedSharerSelectsServerChange`: selector chooses missing-dump test; browser-only control excludes |
| RSP-V0-019 | `TestRSPV0019DumpCollectionPreservesUnknowns`: browser/server uncertainty and inconsistent sharer sets |
| RSP-V0-002, RSP-V0-003 | `TestRSPV0002VitestMultiProjectRefused`: colliding identities, disjoint project files, single named project, admitted major versions |
| RSP-V0-002 | `TestRSPV0002CommaPathRefusedBeforeExecution`: Vitest/Jest/Node discovery and execution reject comma selectors |

## Verification and retained limits

- `TMPDIR=/home/russell/lanes/rspfx-js.tmp script/heavy.sh GOTOOLCHAIN=local go vet ./internal/browsercov`: rc 0.
- `TMPDIR=/home/russell/lanes/rspfx-js.tmp script/heavy.sh GOTOOLCHAIN=local go vet ./internal/jslinecov`: rc 0.
- `gofmt -l` on the four changed Go files: rc 0, no output.
- `script/gen-spec-requirements.sh --write`: rc 0, no generated changes.
- `script/check-line-citations.sh --repin`: rc 0, no moved citations.
- `make -s spec-requirements-check line-citations-check DOC_GATE_GO_TEST='echo NOT_RUN build-only Go tests:' DOC_GATE_LIGHT_GO_TEST='echo NOT_RUN build-only Go tests:'`: rc 0 for static checks; both Go-test recipes explicitly skipped.

The final owner instruction is build-only. Whole-package Go tests, live
qualification, repository gates and selection, independent review, and dogfood
bind/check/seal are NOT_RUN. Pre-change dogfood/CEM evidence is NOT_PRODUCED:
`dogfood-change` enters the binding workflow prohibited for this round. No claim
of passing regressions, independent review, qualification, or completed tickets
is made. The host verifier owns those next steps. No task-store writes or
publication were performed.

Self-development routes: original source/spec inspection used; Corvint context,
affected selection, binding/frontier and recorded outcomes deferred under the
build-only boundary. Learning, external providers and backlog writes are outside
this repair. No self-use or performance measurements were observed.

## Friction and rollback

1. `make spec-requirements` runs an obligation-bound Go test after generation,
   and `make spec-requirements-check` runs Go tests in `internal/specindex` and
   `internal/lrfrepo`. Reproduce by reading their Makefile recipes. To respect
   build-only, generation used the underlying script and check recipes printed
   explicit NOT_RUN notices. The unmodified targets remain for the host.
2. Citation checks report 180 skipped citations whose basenames are not tracked
   at repository root. Reproduce with `make line-citations-check`; this repair
   neither changes those citations nor treats them as validated.

Rollback: revert this repair commit. No stored format migration is required;
runner qualification and enforcement remain unchanged.
