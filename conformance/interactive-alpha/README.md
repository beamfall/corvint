# Interactive alpha installed-host fixture

This frozen fixture qualifies the opt-in VS Code save lifecycle against a real extension host,
Vitest 5.0.3, Playwright 1.64.0, a real browser, the supported Go foreground session, and the
retained provider evidence reader. The harness copies the fixture into a private temporary Git repository;
the checked-in fixture is never edited by a qualification run.

Required extension development dependency: `@vscode/test-electron` 3.1.0.

```sh
CORVINT_RELEASE_ARCHIVE=/absolute/path/corvint-release.tar.gz \
CORVINT_RELEASE_EXTRACTED_ROOT=/absolute/path/extracted-release \
CORVINT_VSIX=/absolute/path/extracted-release/corvint-vscode.vsix \
CORVINT_JS_PROVIDER=/absolute/path/extracted-release/corvint-js-test-provider \
CORVINT_GO_PROVIDER=/absolute/path/extracted-release/corvint-go-test-provider \
node extensions/vscode/test/installed/run.mjs
```

The two binary paths and VSIX must resolve inside `CORVINT_RELEASE_EXTRACTED_ROOT`; source-tree
builds are refused. The runner installs that VSIX into the isolated extension directory and uses a
separate minimal development extension only as the test driver. It pins
`/Applications/Visual Studio Code.app/Contents/MacOS/Code` to version 1.137.0 and
SHA-256 `72f6b27260b64923278468d94a0ac83f0ce32038803e0b6c05072eddf7d046a9`. It constructs the
caller attachment only from freshly measured executable, Git, Go, fixture, cache, scope, and limit
facts. The browser identity is the pinned Chromium headless shell under the explicit browser cache,
matching Playwright’s default headless execution; full Chrome’s launcher is not used for that identity.
The foreground session validates those pins but remains a non-policy preview: this path does
not invoke the production parent or produce an authority verdict, capability, plan, or attestation.
The harness does not synthesize any of those objects. It creates fresh VS Code user-data and
extensions directories, writes a frozen profile that disables telemetry and extension/editor
updates, launches with updates and telemetry disabled and Settings Sync explicitly off, and never
uses an editor or browser download fallback. Fixture dependencies install from the existing npm
cache with lifecycle scripts disabled through the npm CLI tied to the pinned Node installation;
that CLI's version and digest are checked and retained with the other tool identities. Missing or
mismatched pinned tools are failures. The harness
also fails closed on snapshot export, diagnostics, retained-evidence disagreement, residue, profile mutation, or
identity drift. `--interrupt-witness` runs only the separate host-interruption cleanup witness; it
does not create retained cancellation evidence.

The harness's own node:test suite, `core-process.test.mjs` and `retained-evidence.test.mjs`, runs with `make interactive-alpha-test`
(part of `make typescript-test`, the declared TypeScript family) and needs node alone; its browser
cases skip without `CORVINT_TEST_PLAYWRIGHT`. The installed-host proofs beside it,
`core-provider-proof.mjs` and `extensions/vscode/test/installed/run.mjs`, are declared binary
consumers of the two live test providers in
`.corvint/test-binary-consumers.json`, bound to the flows under `flows/` by `.corvint/e2e-flows.json`
(AFP-V0-762, AFU-V1-1655). `core-planning-proof.mjs` and `roadmap-proof.mjs` exec binaries decision
0594 extracted or cut and have no runner.

Every edit under `fixture/` selects two installed qualification targets. The portable Node
regression exercises the launcher seams with stubs. For the installed results, run
`make interactive-alpha-installed-editor` with the release environment above and
`make interactive-alpha-installed-core` with the installed binaries in `CORVINT_PROVIDER_BIN_DIR`,
a fresh work directory in `CORVINT_CORE_WORK`, its matching `CORVINT_GO_AUTHORITY_BUNDLE`
and a fresh evidence directory in `CORVINT_CORE_OUTPUT`. The core target runs the sessions driver.
Both qualifications remain outstanding until their installed drivers succeed.

The local reader admits only unprofiled JS unit/E2E receipts and
`corvint-go-live-session-event/0` pass/fail/stale events. It cross-checks observations
against emitted projections, rejects unknown forms and omitted tests, preserves stale
negatives and never establishes current-input freshness from retained bytes. This is
provider/editor parity evidence, not MCP transport qualification. Legacy report field
names (`mcp`, `testValidityMcp`, `supersededMcpRaw`) and capture basenames remain for
compatibility: they now refer to the local reader and original retained provider bytes.
No synthetic MCP response is emitted. The `testValidityMcp` identity pins the reader
source file; provider identities still pin the installed release binaries.
