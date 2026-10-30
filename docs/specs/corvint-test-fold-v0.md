# Corvint Test Fold V0

Owner: Russell Lewis
Date: 2026-10-22
Requirement prefix: `CTF-V0`
Intent status: accepted (owner, 2026-10-22)
Delivery status: experimental
Authoritative inputs: owner decision of 2026-10-22 accepting the V1-1667 fold with its own acceptance criteria (recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-22-cce-1022k.md`); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-17-binary-boundary-audit.md` (the fold proposal); `AGENTS.md` invariants 3, 7 and 8; `docs/specs/core-compatibility-freeze-v1.md` (CCF-V1-001, CCF-V1-002); `docs/specs/change-compiled-e2e-v0.md` (Binary boundary).

## Agent digest
- Claim: Test-executing companions fold into `corvint-test` groups, one move each, with output documents byte-identical and every old name still answering.
- Status: accepted (owner, 2026-10-22); experimental; CTF-V0-001..010 accepted under the standing auto-accept (2026-10-11). Groups 1 to 4 (`runner`, `falsify`, `minimize`, `accept`) have moved behind byte-identical goldens; groups 5 and 6 have not.
- Exists: `cmd/corvint-test` groups `e2e` (CCE-V0), `runner`, `falsify`, `minimize` and `accept` (entries `internal/testrunner/cli`, `internal/behaviorfalsify/cli`, `internal/playwrightminimize/cli`, `internal/testacceptance/cli`); the four old companions are shims. Unmoved: core `corvint test-validity negate` and `corvint test`.
- Blocked on: for group 5, an owner decision on where `corvint-test negate` takes its worktree root: core reads the global `--root` and its negate parser refuses `--root` by name, so the moved command must either read only the working directory or gain a root option, and CTF-V0-003 forbids gaining a flag (lane cce2-1022u, 2026-10-22). Group 6 (core `test`) also needs the `make gate-affected` script and the CI shadow step moved first; `.github/` changes are owner-posted.
- Read next: Fold map; Requirements; Rollback.

## User and measurable job

An operator who runs Corvint's test tools today has to know five binary names and two core verbs,
each with its own flag dialect and diagnostic prefix. After the fold, one experimental companion,
`corvint-test`, holds every test-executing command as a group, and core keeps only the frozen
CCF-V1-001 verbs' contracts. The measure is mechanical: every moved command produces the same bytes
under the same profile ID as before, and every old name still answers, either by running the same
code or by naming its replacement.

## Fold map

Each row is one group and one move. Flags, standard input and output documents are unchanged; only
the leading words of the invocation change. The runner group moves first because its listing and
plan documents run offline from a committed request, so its goldens need no browser or toolchain.

| Group | Old invocation | New invocation | Profile IDs kept unchanged | Old name after the move |
|---|---|---|---|---|
| 1 `runner` | `corvint-test-runner plan\|run\|runners\|profiles [...]` | `corvint-test runner plan\|run\|runners\|profiles [...]` | `corvint-test-runner-plan/0`, `corvint-test-runner-receipt/0` | shim |
| 2 `falsify` | `corvint-behavior-falsify plan\|execute\|execute-receipt\|verify-receipt --experimental [...] < input.json` | `corvint-test falsify plan\|execute\|execute-receipt\|verify-receipt --experimental [...] < input.json` | `corvint-browser-behavior-falsification-plan/0`, `corvint-browser-behavior-falsification-report/0`, `corvint-browser-behavior-evidence/1`, `corvint-browser-behavior-evidence-verification/1` and the nested `corvint-browser-behavior-*` and `corvint-behavior-control-perturbation/0` profiles | shim |
| 3 `minimize` | `corvint-playwright-minimize plan\|execute --experimental [--approve-plan sha256:...] < input.json` | `corvint-test minimize plan\|execute --experimental [--approve-plan sha256:...] < input.json` | `corvint-playwright-suite-interaction/0`, `corvint-playwright-suite-interaction-live/0` | shim |
| 4 `accept` | `corvint-tests-accept plan\|accept --plan FILE [...]` and the internal `corvint-tests-accept worker` | `corvint-test accept plan\|accept --plan FILE [...]` and the internal `corvint-test accept worker` | `corvint-new-e2e-request/0`, `/1`, `corvint-new-e2e-assessment/0`, `/1` | shim |
| 5 `negate` | core `corvint test-validity negate --provider ABSOLUTE_FILE --spec S --test T [...]` | `corvint-test negate --provider ABSOLUTE_FILE --spec S --test T [...]` | `corvint-step-negation-refusal/0` and the provider's negation documents | structured refusal in core |
| 6 `run` | core `corvint test --base FULL_COMMIT_ID [...]`, `corvint test shards`, `corvint test verify-shards` | `corvint-test run --base FULL_COMMIT_ID [...]`, `corvint-test run shards`, `corvint-test run verify-shards` | every STR-V0 profile `corvint test` emits today | structured refusal in core, after the gate and CI call sites move |

Not folded in V0:

- `corvint prove --mutate` stays in core. `prove` is a frozen CCF-V1-001 verb, and moving one of its
  modes needs a CCF-V1 amendment the owner has not made.
- `corvint-web-flows` stays separate. It explores an application rather than executing tests; the
  audit named it optional, and folding it needs its own decision.
- The discovery providers (`corvint-js-test-provider`, `corvint-go-test-provider`,
  `corvint-ruby-test-provider`) and `corvint-test-validity-mcp` stay separate. They ship in the
  companion release archive with their own install contracts.

## Requirements

- `CTF-V0-001`: Each group MUST move in its own change, in the order of the fold map, and a change
  MUST NOT move a group before the goldens of `CTF-V0-002` exist at its base. Groups 5 and 6 MUST
  NOT move before groups 1 to 4 have moved.
- `CTF-V0-002`: Before a group moves, its change MUST record goldens of the old invocation:
  standard output, standard error and exit status for every refusal the old command's tests
  exercise, help output, and the output document for each success path that runs offline from a
  committed fixture (group 1 `runners`, `profiles` and `plan`; group 2 `verify-receipt`; group 4
  `plan`). The goldens MUST be written from the old binary only. After the move the same test MUST
  show both the old and the new invocation reproduce each golden: exit status and output documents
  byte for byte; diagnostics and help byte for byte except that the new invocation's name replaces
  the old binary name. Amended for V1-2685 (CAP-V0-042): a new invocation that carries `--help`,
  `-h` or `-help` prints its `corvint-test` command-tree node help and exits 0 instead of the old
  help golden, which the old invocation still reproduces. The only output-document fields exempt from byte identity are those that
  name the executing binary: group 2's `execute-receipt` tool identity (name, revision, executable
  digest) and group 4's build identity. A golden comparison that covers such a field MUST name it
  and compare every other byte. Group 4's `plan` request binds the absolute roots of a fixture
  repository the test commits for each run, so its digest differs per run: the golden holds
  `@DIGEST@` in its place, and the test MUST show that the output carries exactly the digest of
  that run's request.
- `CTF-V0-003`: Every profile ID in the fold map MUST stay unchanged. A moved command MUST NOT gain,
  lose or rename a flag, read a different input, or write to a different place.
- `CTF-V0-004`: After a group moves, its old companion binary MUST be a shim that calls the same
  entry the new group calls, so its standard output, standard error and exit status stay
  byte-identical to the golden. Removing a shim needs its own decision and replaces it with a
  binary that exits 2, writes nothing to standard output, and writes one line naming the
  replacement invocation and the code `command-moved` to standard error.
- `CTF-V0-005`: Once group 5 or 6 moves, the removed core verb or mode MUST return core's existing
  structured error envelope with the code `command-moved` and the replacement invocation, exit
  non-zero, and write no output document, with a test per removed verb. Core MUST keep exactly the
  CCF-V1-001 verbs' contracts; `TestCoreVerbsEmitTheFrozenProfiles` MUST pass unchanged.
- `CTF-V0-006`: A moved command that re-enters its own executable (group 4's `worker`) MUST re-enter
  the same group code under `corvint-test`. A test MUST execute the re-entry through the built
  `corvint-test` binary. Group 2 does not re-enter its own executable: its control hooks are the
  plan's declared commands, each pinned by path and digest, and its own executable is only hashed
  for the receipt tool identity.
- `CTF-V0-007`: A moved group's diagnostics MUST name its new invocation (`corvint-test <group>`)
  wherever the old command named its binary, and MUST NOT use the bare `corvint-test:` prefix that
  core `corvint test` keeps until group 6 moves. The `e2e` group keeps `corvint-test e2e:`. A test
  MUST show the new name in a moved group's usage and no bare `corvint-test:` prefix in its
  sources.
- `CTF-V0-008`: Name matchers MUST key on exact binary names, never on the `corvint-test` prefix. A
  test MUST show that `corvint-test`, `corvint-test-runner` and `corvint-test-validity-mcp` each
  classify as themselves in the companion-release archive classifier and in every other matcher
  the move touches.
- `CTF-V0-009`: Group 6 MUST NOT move until `script/gate-affected-corvint.sh` and the CI shadow step
  invoke `corvint-test run`. The `.github/` change is owner-posted; until it lands, group 6 stays
  in core and its row reads blocked.
- `CTF-V0-010`: The fold MUST NOT add a command, flag, dependency or default-path behaviour to core
  `corvint` beyond the refusals of `CTF-V0-005` (invariant 7), and MUST NOT make core import or
  execute `corvint-test`.

## Failure modes

| Condition | Required observation |
|---|---|
| A golden differs after a move | The move's change fails its focused test; the group does not move |
| A moved command needs a new flag or input | Out of scope for the fold; a separate requirement in the owning spec |
| A re-entry reaches the wrong group | The `CTF-V0-006` test fails |
| The CI call site for group 6 has not landed | Group 6 stays in core; row blocked (`CTF-V0-009`) |
| A matcher uses a `corvint-test` prefix | The `CTF-V0-008` test fails |

## Acceptance evidence

Per group: the golden file `cmd/corvint-test/testdata/fold/<group>.json`, the before and after
comparison test, the shim test at the old name, the re-entry test where one applies, and the
prefix test. Core groups add the `command-moved` refusal test and an unchanged
`TestCoreVerbsEmitTheFrozenProfiles`. Live qualification that a group already has (BBF-V0 live
RSpec, NEA-V0 live Playwright) MUST be rerun through `corvint-test` or recorded `NOT_RUN` with its
command.

## Traceability

| Requirement | Evidence |
|---|---|
| `CTF-V0-001` | Each group's goldens committed before its move: group 1 at `85bc22089`, group 2 at `868affc59`, group 3 at `44cbf177a`, group 4 at `721706587` |
| `CTF-V0-002` | `cmd/corvint-test/fold_test.go` `TestCTFV0002RunnerFoldGoldens` (`testdata/fold/runner.json`, 13 cases) and `TestCTFV0002FalsifyFoldGoldens` (`testdata/fold/falsify.json`, 16 cases including offline `verify-receipt` over a committed receipt) and `TestCTFV0002MinimizeFoldGoldens` (`testdata/fold/minimize.json`, 13 refusal and help cases; its plan needs pinned live evidence) and `TestCTFV0002AcceptFoldGoldens` (`testdata/fold/accept.json`, 19 cases including the offline `plan` over a fixture repository committed per run, its digest checked against that run's request), each recorded from the old binary and run against old and new invocations; the new invocation's help cases compare against its command-tree help (`helpRequest`, CAP-V0-042) |
| `CTF-V0-003` | The same goldens: the `runners`, `profiles`, `plan` (groups 1 and 4) and `verify-receipt` documents compare byte for byte, profile IDs included; the flag sets moved unchanged |
| `CTF-V0-004` | `cmd/corvint-test-runner`, `cmd/corvint-behavior-falsify`, `cmd/corvint-playwright-minimize` and `cmd/corvint-tests-accept` are shims over the shared `cli` entries; their existing tests pass unchanged; the golden tests run the old names |
| `CTF-V0-006` | `cmd/corvint-test/fold_test.go` `TestCTFV0006AcceptWorkerReentersCorvintTest`: an approved `corvint-test accept accept` over the fixture re-enters the built binary as `corvint-test accept worker`, and no run reads `worker-incomplete` (it does when the re-entry is `worker`). Live NEA-V0-002 (`TestNEAV0002ActualBrowserAssessment`, Playwright 1.64.0, Node 22.23.2) was run through the shim and failed identically before (`721706587`) and after the move with a bare `provider-infrastructure` on that host (lane cce2-1022u). On 2026-10-25 (lane cce-live-1025, V1-2431) it PASSES re-entering `corvint-test accept worker` (`CORVINT_ACCEPTANCE_COMPANION=corvint-test`; Playwright 1.64.0, runner Node v24.11.1, headless shell 1248): blocked, rejected, rejected; controls killed, survived, killed; no provider reasons. The 10-22 cause was not recoverable because the reason was bare; each run now names it (NEA-V0-011) |
| `CTF-V0-007` | `cmd/corvint-test/fold_test.go` `TestCTFV0007FoldedGroupBoundary` |
| `CTF-V0-008` | `internal/companionrelease/fold_names_test.go` `TestCTFV0008ExactCompanionNames` |
| `CTF-V0-010` | `TestCTFV0007FoldedGroupBoundary` (core links no group entry) |

`CTF-V0-005` and `CTF-V0-009` have no evidence yet: they bind groups 5 and 6, which have not
moved.

## Non-goals and limits

No behaviour change to any moved command, no new profile version, no change to the frozen core
verbs, no release-archive change, no move of `prove --mutate`, `corvint-web-flows` or the discovery
providers, and no promotion of any `corvint-test` group out of experimental.

## Rollback

Each group moved in its own change, so reverting that change restores the old companion's own
entry; the goldens and the shared entry are harmless to keep. Old names keep working as shims
throughout, so no operator or script needs to migrate back. For groups 5 and 6, reverting restores
the core verb; the CI call-site change of `CTF-V0-009` reverts separately.
