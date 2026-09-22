# Interactive alpha installed-host fixture

This frozen fixture qualifies the opt-in VS Code save lifecycle against a real extension host,
Vitest 5.0.0, Playwright 1.63.0, a real browser, the supported Go foreground session, and the
read-only test-validity MCP. The harness copies the fixture into a private temporary Git repository;
the checked-in fixture is never edited by a qualification run.

Required extension development dependency: `@vscode/test-electron` 3.1.0.

```sh
CORVINT_RELEASE_ARCHIVE=/absolute/path/corvint-release.tar.gz \
CORVINT_RELEASE_EXTRACTED_ROOT=/absolute/path/extracted-release \
CORVINT_VSIX=/absolute/path/extracted-release/corvint-vscode.vsix \
CORVINT_JS_PROVIDER=/absolute/path/extracted-release/corvint-js-test-provider \
CORVINT_GO_PROVIDER=/absolute/path/extracted-release/corvint-go-test-provider \
CORVINT_TEST_VALIDITY_MCP=/absolute/path/extracted-release/corvint-test-validity-mcp \
node extensions/vscode/test/installed/run.mjs
```

The three binary paths and VSIX must resolve inside `CORVINT_RELEASE_EXTRACTED_ROOT`; source-tree
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
also fails closed on snapshot export, diagnostics, MCP disagreement, residue, profile mutation, or
identity drift. `--interrupt-witness` runs only the separate host-interruption cleanup witness; it
does not create retained cancellation evidence.
