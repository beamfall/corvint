# 2026-10-19 V1-1947, V1-1948: environment sources, stand-in languages, precision fixes

Tickets: V1-1947 (GitHub #1199, #1200) and V1-1948, the residuals of V1-1940 (PR #1205).
Requirements: TSA-V0-350..363 in `docs/specs/test-source-analysis-v0.md`. They are agent-drafted
and accepted under the owner's standing auto-accept (2026-10-11).

## Decisions

- **A small YAML reader, not a dependency.**
  - Workflow and compose files are read by a minimal block-YAML reader in
    `internal/testsource/env_contract_sources.go`.
  - It fails closed (`compose-unparsed`, `workflow-unparsed`) on anchors, aliases, tags, merge
    keys, tabs, multiple documents and nested flow collections.
  - Adding a YAML module would widen `go.mod` for an experimental reader; an unparsed file is
    named rather than guessed.
- **The profile's own `compose.NAME` and `.env.NAME` stay unread.** TSA-V0-210 says they are
  listed with `compose-not-read`/`dotenv-not-read`. Reading them would amend an accepted
  requirement, so other compose and dotenv files are read and those two names are left for an
  owner decision.
- **Callers fill in, scripts win.**
  - A workflow step's env reaches the script as its caller environment. A script assignment
    overrides it, except for a self-default such as `PORT=${PORT:-3000}`, which takes the caller's
    value.
  - Callers that disagree, or a `${{ }}` expression, are UNKNOWN with the alternatives listed.
- **Branches are read as branches.** An assignment inside `if`/`case`/a loop/a function, or after
  `&&`/`||`, is branch-dependent: null, with every value it may take. A `{ }` group is not a
  branch. This replaces V1-1940's last-assignment-wins reading.
- **Route fields are bounded by the handler.** JS/TS and Go take the balanced registration call
  plus a named handler's body. Python and Ruby take the indented body. This replaces reading to
  the next registration.
- **`corvint impact` is not extended (NOT_PRODUCED).** It is the core indexed command: it refuses
  `.sh` paths, its receipts are frozen, and its providers attach to the default profile only.
  `corvint tests env --base` already lists the tests a bootstrap offset change affects
  (TSA-V0-215).
- **Script paths are repository-relative.** A relative path in a profile script resolves from the
  repository root, as stand-in launches already did. Only `$(dirname "$0")/` names the script's
  directory.

## Evidence

- Focused tests in `internal/testsource`, run through the host semaphore, pass:
  - `TestEnvContractSources`
  - `TestEnvContractSourceNotes`
  - `TestEnvContractBranchDependent`
  - `TestEnvContractHandlerBounded`
  - `TestEnvContractStandInLanguages`
  - `TestRoleLocatorPerTestMounts`
  - `TestSourceValidityEnv`
  - the existing `TestEnvContract*`, `TestRoleLocator*` and `TestSourceValidity*` tests
- The `cmd/corvint` tests `TestTestsRoleLocatorsAndEnvCLI` and `TestTestsValidityCLI` also pass.
- Twenty mutations, one or more per new rule, were each killed by the named test. They covered:
  - per-test mounts;
  - conditional reading and alternatives;
  - handler bounds;
  - workflow callers, caller conflicts and expressions;
  - dotenv sourcing;
  - compose reading, runner gating and skipping the profile's compose file;
  - Go routes, directory launches, Flask, Rails controllers and WireMock;
  - the language abstention;
  - the validity file filter and test id.
- Real-suite qualification: NOT_RUN.

## Independent review (Codex, one round)

One read-only Codex round on `aa54c0729..5b255f9dc` reported nine findings, all fixed in the
follow-up commit and covered by `TestEnvContractReaderEdges`, with each fix's mutation killed:

1. An empty quoted YAML key panicked: it is now outside the subset.
2. A workflow value such as `$PORT's` recursed without bound: workflow values are always literal,
   and a caller value is evaluated once (`reference-cycle`).
3. A `&&`/`||` list continued on the next line lost its condition: such lines are joined.
4. A compose `env_file` naming the profile's own `.env.NAME` was read: it stays unread.
5. A script reassigning a variable every calling step exports lost its export: it stays passed.
6. An expression-bodied arrow handler took the next function's body: the handler head is bounded.
7. Duplicate flow-mapping keys were accepted: they are outside the subset.
8. Block scalars were rebuilt inexactly: as an `env`/`environment` value they make the file
   unparsed, never guessed.
9. A WireMock mapping without a route did not advance the citation: every request advances it.

The TSA-V0-353, 354, 358, 360 and 361 texts (new in this change) state these readings.
