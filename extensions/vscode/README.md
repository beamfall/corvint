# Corvint Evidence for VS Code

Corvint Evidence is an experimental, local-first VS Code adapter for revision-pinned Corvint context,
impact evidence, inclusion reasons, and explicitly enabled live-test feedback. It uses native
Tree Views, diagnostics, editor decorations, the status bar, commands, and Test Explorer. It has no
webview, telemetry, network client, database, source uploader, updater, terminal, edit action, or
authority to choose tests or change project policy.

This `0.1.0` package is a developer preview with support status `FALLBACK`. Source, compilation, or
a successful install does not make a VS Code/OS/Corvint tuple supported. The current compatibility
matrix and promotion requirements are frozen in
`docs/specs/vscode-extension-v0.md` in the Corvint source repository.

## Runtime dependencies

- VS Code desktop `1.95.0` or newer. Web and virtual workspaces are unsupported.
- A trusted, file-backed workspace on macOS or Linux. Windows execution is unsupported in V0.
- One separately installed Corvint engine in the workspace extension host:
  - `cli` transport: an absolute `corvint` or `corvint` path,
    with the exact V0-admitted version token `0.5.0a4` plus unchanged executable digest/identity
    (the token alone is not capability proof); or
  - `mcp-stdio` transport: an absolute `corvint-mcp` path implementing the experimental
    `docs/specs/mcp-server-2026-07-28-v0.md` contract.
- Git available to the selected engine through the fixed child `PATH`
  `/usr/bin:/bin:/usr/sbin:/sbin`. The extension itself does not search for or pin Git; the
  experimental MCP server's inherited Git pinning gap is retained below.

The VSIX has no npm production dependency or bundled Corvint executable. Node.js 22 and npm are
development/package tooling requirements only; the VS Code desktop extension host supplies the
runtime JavaScript environment.

The extension does not bundle or install a Corvint engine. Remote SSH and dev-container windows need
the executable in the remote workspace extension host, not on the local UI machine. The current MCP
server candidate reports `0.1.0-experimental`; that self-reported version is informational, while
wire conformance and the pinned executable digest establish admission. No packaged/installed VS
Code MCP runtime tuple has passed, and its
inherited Git executable-pinning promotion blocker remains open. MCP runtime evidence is therefore
`NOT_RUN`/`FALLBACK`.

## Use

1. Open one trusted file workspace.
2. Set `Corvint: Transport` to `cli` or `mcp-stdio`.
3. For `cli`, set `Corvint: Executable Path` or run **Corvint: Select and Pin Executable** to inspect
   narrowly named PATH candidates. For MCP, set the absolute **Corvint: MCP Executable Path**; there
   is no MCP PATH discovery.
4. Run **Corvint: Query Evidence** or open a file and run **Corvint: Inspect Active File Impact**.
5. Inspect the Evidence, Impact, and Why views. Diagnostics and decorations are projections of the
   same bounded result, not additional analysis.

**Corvint: Refresh Last Explicit Query** repeats only the last explicit query or impact after trust,
root, transport, and executable identity revalidation. **Corvint: Clear Results** removes the current
in-memory evidence projection. Query and impact remain explicit commands; live-test providers
rerun on saves only after the separate opt-in below. Returned verification suggestions are never executed.

To display a completed Corvint Go live-test observation, configure a contained workspace-relative
`Corvint: Test Observation Path`, then run **Corvint: Import Test Observation**. V0 registers no Test
Run Profile; live execution uses only the separately configured provider. An observation without the shared Corvint semantic verifier is
refused with `UNVERIFIED_IMPORT` and creates no test projection; an incomplete or inconsistent run
cannot show a passing result.

## Automatic unit and E2E feedback

Live tests are off by default. Install the optional Corvint workflow tools and the project's test
dependencies, then configure the workspace. This Vitest example uses project-relative config/test
paths; replace the absolute tool paths and the test-file entry with your own:

```json
{
  "corvint.liveTests.enabled": true,
  "corvint.liveTests.command": [
    "/path/to/corvint/bin/corvint-js-test-provider", "unit",
    "--dir", ".", "--config", "vitest.config.js",
    "--package-json", "package.json", "--lockfile", "package-lock.json",
    "--runner-version", "5.0.0", "--test-file", "unit.test.js"
  ],
  "corvint.liveTests.toolchainPaths": ["/path/to/node/bin"],
  "corvint.liveTests.watchPaths": ["."],
  "corvint.liveTests.retainEvidence": true
}
```

In a trusted workspace the provider runs once, then reruns the full configured suite when you save
an eligible file. Rapid saves coalesce; a new save immediately clears the prior current result.
Completed passes and failures remain in Test Explorer, and reported failure locations become
diagnostics. The extension waits for provider cleanup before retaining a one-shot result. Set
`corvint.liveTests.enabled` to `false` to stop the provider and cancel pending reruns.

The same loop supports `corvint-js-test-provider e2e` with Playwright 1.63.0, an installed browser,
explicit app-server/test argument vectors, readiness URL and app-build directory. The
[interactive fixture](../../conformance/interactive-alpha/README.md) shows the complete unit/E2E
setup and its exact qualification scope. Go uses its separate opt-in foreground session and
caller-pinned configuration; its evidence remains a non-policy preview.

This alpha watches editor saves in declared files/directories, not external filesystem edits.
It excludes `.git`, `.corvint`, `node_modules`, `dist`, `build`, `out` and `coverage`, refuses
symlink escapes, and routes each save to its containing workspace folder. A JS provider must be
named `corvint-js-test-provider` with `unit` or `e2e`; wrapping it in a shell disables save routing
and is refused when retention is enabled. Toolchain directories affect only the live provider;
on macOS a Homebrew Node installation commonly needs `/opt/homebrew/bin` explicitly.

With retention enabled, agents can use the separate `corvint-test-validity-mcp` discovery tool to
inspect the same completed provider documents. Passing execution does not establish freshness,
test adequacy or authority. Unsupported, stale and unmeasured axes remain visible; retained Go
freshness and some JavaScript identity components remain `UNKNOWN`. See the exact installed
qualification record before treating a host/platform tuple as supported.

## Settings

| Setting | Default | Meaning |
|---|---:|---|
| `corvint.transport` | `cli` | Exact local transport: `cli` or `mcp-stdio`; no fallback. |
| `corvint.executablePath` | empty | Absolute `corvint` or `corvint` candidate for the CLI transport. |
| `corvint.mcpExecutablePath` | empty | Absolute `corvint-mcp` candidate for MCP `2026-07-28` stdio. |
| `corvint.testObservationPath` | empty | Contained workspace-relative observation file imported only on command. |
| `corvint.maxOutputBytes` | `262144` | Complete stdout bound; range 4 KiB–1 MiB. |
| `corvint.timeoutMilliseconds` | `15000` | Explicit operation timeout; range 1–30 seconds. |
| `corvint.liveTests.enabled` | `false` | Start the explicitly configured provider in this trusted workspace. |
| `corvint.liveTests.command` | `[]` | Executable and literal argument array; no shell expansion. |
| `corvint.liveTests.retainEvidence` | `false` | Ask a supported provider to retain completed local evidence for MCP discovery. |
| `corvint.liveTests.watchPaths` | `["."]` | Up to 64 relative files/directories whose saves rerun the configured JS suite. |
| `corvint.liveTests.toolchainPaths` | `[]` | Up to 16 absolute directories for the live provider's Node/npm/Go tools. |

All path settings are restricted in untrusted workspaces. Pins and results are memory-only and are
discarded on trust-host reload, root or transport change, clear, deactivation, disable, or uninstall.

## MCP transport

The MCP client is deliberately modern-only and local-only:

- each explicit query/impact starts one pinned `corvint-mcp --root ABSOLUTE_ROOT` process;
- `server/discover` is first and protocol version `2026-07-28` is required on every request;
- no legacy `initialize` fallback, roots capability, HTTP/SSE, sampling, elicitation, prompts,
  resources, subscriptions, progress/logging notifications, catalog cache, restart, replay, or
  transport failover exists;
- every operation validates all three frozen tool definitions, then calls only `corvint.query` or
  `corvint.impact`; and
- MCP query accepts printable ASCII with at least one non-space character, and MCP impact accepts
  only the frozen normalized repository-relative Go-path profile; a CLI-valid Unicode task or
  broader path is rejected before spawn rather than coerced or silently routed to CLI; and
- the canonical text result must exactly duplicate the validated `corvint-mcp-bridge-result/0`
  structured result before its native Corvint receipt is projected.

Cancellation sends `notifications/cancelled`, closes stdin, and then applies bounded direct-child
termination. This is not proof of descendant containment. Any observed residue blocks promotion.

## Trust, privacy, and authority

In Restricted Mode the extension registers inert native UI only. It does not inspect configuration
paths, discover/hash an engine, read the repository or an observation, spawn a process, or preserve
an earlier result. Granting trust starts from empty state. VS Code trust revocation reloads the
extension host; deactivation clears memory and cancels work before the untrusted host starts.

The extension sends nothing to Corvint, its publisher, or a remote service. It stores no evidence,
tasks, paths, pins, history, test output, or receipt in VS Code state or on disk. It never edits
source, changes Git, installs/updates an engine, invokes a shell, or upgrades authority. An explicitly
enabled live provider executes the configured project tests; opt-in evidence retention is owned by
that provider under `.corvint/test-evidence`.
`Ready` means only that the selected trusted root and compatible unchanged local pin exist. A pass
means only pass in the explicitly observed scope, never global correctness or merge readiness.

VS Code Marketplace or administrator policy owns extension installation and updates. The Corvint
engine is separate and never silently changed. Rollback is disable/uninstall or installing a prior
approved VSIX; the separately installed engine and repository are untouched.

## Failure states

The status bar reports `Restricted`, `No CLI`, `Incompatible`, `Ready`, `Busy`, `Degraded`, or
`Unsupported`. Stable failures include trust/root/path denial, missing or incompatible engine,
pin drift, spawn/cancellation/timeout/output failures, invalid or unsupported output/observation,
MCP discovery/protocol/toolset/tool/interaction failures, and observed process residue. A failed
operation clears or visibly disowns the prior result; raw process text is never evidence.

## Build and inspect the developer preview

From `extensions/vscode`:

```console
npm ci
npm run check
npm run package:list
npm run package:vsix
```

`package:list` is the required preflight for unexpected files. `package:vsix` compiles, tests, and
packages without production dependencies. Before any distribution, inspect the generated VSIX,
run the independent hostile conformance suite and trusted/untrusted `@vscode/test-electron` matrix,
audit dependencies/licenses, and obtain owner approval for publisher identity and signing. A
blocked Electron download or unavailable runtime stays `NOT_RUN`. The manifest publisher value is
a developer-preview packaging placeholder, not an approved Marketplace publisher identity.

License: GNU Affero General Public License v3.0 or later; see `LICENSE` in this package.
