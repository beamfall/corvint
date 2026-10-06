# 2026-10-06: V1-0868 unbounded-reader reasons and read-scope declarations

## Intent

Ticket V1-0868. `corvint affected` selected about 51 test units where 3 or 4 were needed. Rule (d)
UNBOUNDED_READER entries drove 37 to 40 of those, including `cmd/corvint` (hosted estimate 758.7 s).
At base `76f7f2ac`, 34 of the 43 entries in `.corvint/unbounded-readers.json` had no reason.
The ticket asks for three things. Every entry names its unbounded read. Units whose reads can be
declared get AFP-V0-023 declarations, one at a time, each replayed over the five survey pull
requests with zero new misses. The selection counts are reported before and after.

## Change

- New AFP-V0-028 in `docs/specs/affected-plan-v0.md`, with a trace row. Every `units` directory
  needs a `reasons` entry. `tools/unbounded-readers` reports units without one as `unreasoned` and
  fails, naming them, for current and stale units alike. Regression:
  `TestAFPV0028EveryUnitNamesItsUnboundedRead`. It fails on the base checker.
- `.corvint/test-read-scopes.json` gains eight declarations. Main's 125 entries are unchanged.
- `.corvint/unbounded-readers.json` drops those eight units, leaving 35. Each of the 35 now
  carries a reason naming the read and why it cannot be declared.

## Method

Every declaration was measured on Linux, in a `golang:1.27.1` container holding a plain copy of
`76f7f2ac`. Each step used the repository's own `.github/testconfine` wrapper with
`CORVINT_CONFINE_ROOT`.

1. **Proposal.** `strace -f` of the unconfined `go test` lists the files the test process and its
   children open under the repository, outside the package's own directory.
2. **Outcome equality.** The per-test `go test -json` outcomes, unconfined and confined with exactly
   the proposed entries, must be identical, and the package must pass.
3. **Denial check.** A confined run under `strace -e status=failed` must show no refused open under
   the repository. This catches tests that tolerate a denied read and still pass.

Units whose tests fail as root (git dubious ownership, permission tests) were measured as uid 501.
A unit was declared only when all three steps held. Otherwise it stayed in `units` (fail closed).

## Accepted declarations, in replay order

Hosted estimates come from the survey's cost ranking.

| Unit | Entries | Estimate |
|---|---|---|
| `internal/diagnostic` | `cmd/`, `internal/`, two spec files, the coverage count | 10.1 s |
| `tools/compat-trial/build-fixtures` | `go.mod` and three directories | 8.1 s |
| `internal/analyzercap` | `go.mod` and the analyzer directories | 4.9 s |
| `tools/gate-ledger` | three `tools/gate-affected-select` files | 3.2 s |
| `cmd/corvint-postmerge-host-launcher` | none | 1.3 s |
| `internal/postmergeproof` | five postmerge-runtime-v2 schemas | 15.8 s |
| `internal/authoritystore` | none | 1.7 s |
| `internal/dashboard/source` | none | 1.7 s |

`internal/jstestprovider` was declared in the first round and then withdrawn after review. Its
six skips are gated by a flag or an environment variable, not by a repository read. But the
enabled live tests read `conformance/interactive-alpha/fixture/` and build from the repository
root, and no confined run measured them. AFP-V0-028 now requires that a skipped test, when
enabled, reads nothing outside the entries.

A declared root locator still leaves its undeclared dependents unbounded (AFP-V0-023). Several units
keep their entry for that reason as well as for their own reads.

## Replay

Each step was replayed over pull requests 571, 614, 594, 557 and 610 at their merge bases, using a
plain clone under the lane temp directory. The binary was built from `76f7f2ac`. Two change sets
were used: Go files only, and every change except `.corvint/`. "Needed" uses the survey's rule: packages with
tests whose test binary's import closure (`go list -test` at the pull request's head) contains a
package with a changed `.go` file. Read-dependent needs are outside this measure. Each cell shows the number selected, then the number with an UNBOUNDED_READER witness
in parentheses.

| Step | #571 go / all | #614 go / all | #594 go / all | #557 go / all | #610 go / all | misses |
|---|---|---|---|---|---|---|
| main (76f7f2ac) | 79 (18) / 98 (14) | 51 (40) / 71 (30) | 70 (37) / 92 (29) | 52 (37) / 75 (29) | 51 (39) / 71 (30) | 0 |
| + internal/diagnostic | 79 (17) / 98 (13) | 51 (39) / 71 (29) | 70 (36) / 92 (28) | 52 (36) / 75 (28) | 51 (38) / 71 (29) | 0 |
| + tools/compat-trial/build-fixtures | 78 (16) / 97 (12) | 50 (38) / 70 (28) | 69 (35) / 91 (27) | 51 (35) / 74 (27) | 50 (37) / 70 (28) | 0 |
| + internal/analyzercap | 78 (15) / 97 (11) | 49 (37) / 69 (27) | 68 (34) / 90 (26) | 50 (34) / 73 (26) | 49 (36) / 69 (27) | 0 |
| + tools/gate-ledger | 77 (14) / 97 (11) | 48 (36) / 69 (27) | 67 (33) / 90 (26) | 49 (33) / 73 (26) | 48 (35) / 69 (27) | 0 |
| + cmd/corvint-postmerge-host-launcher | 77 (14) / 97 (11) | 47 (35) / 68 (26) | 66 (32) / 89 (25) | 48 (32) / 72 (25) | 47 (34) / 68 (26) | 0 |
| + internal/postmergeproof | 77 (14) / 97 (11) | 46 (34) / 67 (25) | 66 (32) / 89 (25) | 48 (32) / 72 (25) | 46 (33) / 67 (25) | 0 |
| + internal/authoritystore | 77 (14) / 97 (11) | 45 (33) / 66 (24) | 65 (31) / 88 (24) | 47 (31) / 71 (24) | 45 (32) / 66 (24) | 0 |
| + internal/dashboard/source (delivered) | 76 (13) / 96 (10) | 44 (32) / 65 (23) | 64 (30) / 87 (23) | 46 (30) / 70 (23) | 44 (31) / 65 (23) | 0 |
| not adopted: + 5 closure-group units | 76 (12) / 96 (10) | 39 (27) / 61 (19) | 59 (25) / 83 (19) | 41 (25) / 66 (19) | 39 (26) / 61 (19) | 0 |
| needed units | 39 | 4 | 12 | 3 | 4 | |

The base row reproduces the survey's counts. No step dropped a needed unit.

## Units that stay unbounded

Each of the 35 reasons in `.corvint/unbounded-readers.json` is backed by the source and the
container trace. They fall into five groups.

- **Opens the filesystem root `/`.** The wrapper cannot grant `/` without granting the repository.
  - Units: `cmd/corvint`, `cmd/corvint-corpus-mcp`, `conformance/cli-parity-v0`,
    `conformance/dashboard-snapshot-v0`, `conformance/mcp-2026-07-28`,
    `examples/evidence-provider/v0/conformance`, `internal/companionrelease`, `internal/console`,
    `internal/contextindex`, `internal/dashboard/repository`, `internal/flowcoverage`,
    `internal/flowdocs`, `internal/liveverify/affected/golang`, `internal/lspstdio`,
    `internal/mcp/docsbridge`, `internal/postmergeworkflow`, `internal/testrunner`,
    `internal/testrunner/sql`.
  - `cmd/corvint-corpus-mcp`, `internal/lspstdio` and `internal/mcp/docsbridge` pass confined
    with their traced entries declared. They pass only by tolerating refused opens of `/`, which
    the denial check shows (14, 14 and 24). The opening call is not attributed.
  - Sources: `internal/flowdocs/files.go`, `internal/testrunner/execute_unix.go`,
    `internal/testrunner/sql/sql.go`, `internal/cemcandidate/source.go` and
    `internal/stepverify/safeopen/open_unix.go`. Each opens `/` for reading rather than with
    `O_PATH`.
- **Reads `.git`.**
  - Units: `conformance/release-artifact-v0`, `internal/analyzerhtmlcss`,
    `internal/doccorpus/selfcorpus`, `internal/liveverify/affected`,
    `internal/opencodequalification` (a nested build stamps VCS status),
    `tools/cem-interop-runner`.
  - `tools/cem-interop-runner` passed confined, but only because it silently falls back when
    `git rev-parse` is refused. The denial check caught it.
- **Opt-in tests read outside the package.** `internal/jstestprovider` (see above). It also
  locates the root through git.
- **Walks the whole tree.** `internal/tasks` lists 609 directories.
- **Import closure.** These tests run `go list -deps` or `go build` over a binary's import
  closure: `cmd/corvint-analyzer-rust`, `cmd/corvint-analyzer-shader`,
  `internal/analyzerstructured`, `conformance/mcp-test-validity-v0`,
  `internal/liveverify/provider`, `internal/analyzerpython` and `internal/postmergehost`.
  - Five of them passed all three steps with their closure declared: 47 directories for
    `conformance/mcp-test-validity-v0` and about 95 for the others.
  - They are not adopted. Such a declaration must change with every new import into the binary,
    and the confined CI run would fail every pull request that adds one until the declarations
    are updated. That trade-off is the owner's call.
  - The "not adopted" row above shows what they would remove: five more units on four of the pull
    requests (the row is measured on top of the eight delivered declarations).
  - `internal/analyzerpython` still failed three tests with its closure declared.
- **Not measurable here.** `internal/tasks/cli` builds `cmd/corvint-tasks` (33 directories). Its
  unconfined run failed 264 tests in the container, so no confined comparison could prove a
  declaration. `tools/cem-trial` keeps its existing reason.

## Decisions

- **Fail closed.** A unit is declared only with outcome equality and a clean denial check.
- **`cmd/corvint` stays unbounded.** Its trace shows the tests and the built binary opening `/`,
  listing the repository root and reading `.git`. The costliest unit is therefore not declarable
  until the root opens change.
- **Order.** The cost-first order was limited by measurability. The two costliest units after
  `cmd/corvint` (`internal/companionrelease` and `internal/tasks/cli`) are not declarable. The
  eight accepted units were declared one at a time in the order shown, each with its own replay.

## Limits

- Landlock does not mediate `stat` or existence checks. A test that branches on existence alone
  is not caught by either measurement.
- Measurements ran in a Linux container, not on the hosted CI runner. Only Linux test files ran.
  Darwin-only tests are unmeasured.
- A reason is a reviewed statement from the source and one trace. It is not a proof that no
  narrower declaration exists.
- The savings are modest. The eight declared units total about 47 s of hosted estimate per
  selection. `cmd/corvint` (758.7 s), `internal/companionrelease` (243.3 s) and
  `internal/tasks/cli` (184.4 s) stay selected on every Go change.

## NOT_RUN

- `make gate`: not requested; AGENTS.md.
- `-race`.
- A confined run of `internal/tasks/cli` on a hosted runner.

## Review

Codex round 1 raised two P2 findings, and both were repaired.

1. **`internal/jstestprovider`'s empty declaration denied reads made by its opt-in live tests.**
   The declaration was withdrawn, the unit went back into `units` with a reason, and the
   AFP-V0-028 skip clause was tightened. The last two steps were replayed again without it.
2. **The reasons for `cmd/corvint-corpus-mcp`, `internal/lspstdio` and `internal/mcp/docsbridge`
   attributed their `/` opens to `internal/flowdocs` without evidence.** Each was re-measured
   confined, followed by a denial check. The reasons now state the observed tolerated denials and
   say that the opening call is not attributed.

Codex round 2 confirmed both repairs and raised one P3. Two reasons named the wrong operation:
`internal/liveverify/affected/golang` (`groundtruth_test.go:35`) and `internal/liveverify/provider`
(`go list -deps`, not a build). Both were corrected.
