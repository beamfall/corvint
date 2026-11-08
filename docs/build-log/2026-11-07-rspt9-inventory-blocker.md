# V1-5111: node-test inventory contract blocker

Lane: rspt9. Inspected checkout: `89372490721bb960ff9261a930c94f8514a2056e`.
Disposition: PARTIAL; implementation stopped under the charter's explicit instruction to stop
and record a question when the accepted contract cannot be implemented without inventing behavior.
No runner implementation, qualification, or delivery claim is made.

The charter requires inventory through each runner's list mode for vitest, jest, `node --test`
and Playwright. RSP-V0-002 requires every test identity and its source path, and makes missing
or unparseable inventory FULL. On this host `/usr/bin/node --version` reports `v22.22.1`;
`node --help` exposes test filtering and reporters but no test-list mode. These were help/version
reads, not test execution. The current official Node command-line documentation likewise does
not document a test-list mode. This is an external runner capability blocker for the charter's
literal discovery requirement, not evidence that a different discovery design is impossible.

Node permits subtests to be registered inside a parent test's body. Consequently, inferring an
inventory from a run with all test bodies filtered out cannot establish that all test identities
have been enumerated. A FULL discovery execution or a deliberately coarser inventory would be
a different discovery contract and is not silently substituted here. Source scanning likewise
does not prove the identities of dynamically registered tests.

Original references inspected on 2026-11-07:

- [Node command-line options](https://nodejs.org/api/cli.html#--test).
- [Node subtests](https://nodejs.org/api/test.html#subtests).
- [Node name filtering](https://nodejs.org/api/test.html#filtering-tests-by-name).
- [Jest collection mode](https://jestjs.io/docs/cli#--collecttests) describes per-test collection;
  [Jest listTests](https://jestjs.io/docs/cli#--listtests) lists files. Treating the latter as
  complete per-test discovery would also be unsound. Jest collection support was not exercised.

Question for integration: should node-test use a specified FULL discovery execution and reconcile
its reported identities, remain explicitly inventory-absent/FULL until a native discovery mode
exists, or target another explicitly specified discovery mechanism? No choice is adopted here.

RSP-V0-018, RSP-V0-019 and RSP-V0-052, the RSP-V0-002..005 runner pieces, and RSP-V0-040 step 5
remain undelivered by this lane. Existing runner behavior and spec traceability are unchanged.
Runner tests, live qualification, independent review, and CEM binding/check/seal are NOT_RUN: the final
build-only charter forbids those executions and delegates integration validation. No qualification
record or requirement-to-test mapping was produced. The ticket remains open; no Tasks writes occur.

The actual checkout equals the charter's integration base above. The earlier common instructions
also name `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`; neither base was used to bind a CEM or
run test selection. The final build-only instruction overrides the earlier request to run
dogfood-change at start and dogfood-check at finish, since those would bind/run prohibited work.

Retained local help/version evidence lives outside the repository at
`/home/russell/lanes/rspt9-evidence/`. SHA-256 digests:

- `node-help.txt`: `ab464311ea045c7d1287bc56f590dbccadbd5f7c6e5960e4d260b07d911cda0f`.
- `node-version.txt`: `d757d43c53951704e32c04b9b1e6b4cc65713b659e72fcdbb02f5200675c7fd8`.

Check friction: the explicitly allowed `make -s spec-requirements-check line-citations-check`
invokes an existing `internal/specindex` Go test through `script/heavy.sh` as a prerequisite.
It also invokes focused `internal/lrfrepo` tests. Both reported PASS; the Make command returned
0, including the line-citation check. No Go test command was directly invoked, and no runner
package was tested. No spec definitions or existing line citations changed, so regeneration and
repinning were unnecessary.
