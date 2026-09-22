# Corvint VS Code Extension V0 conformance

This directory is an independent, dependency-free black-box conformance harness for the Corvint
VS Code extension. It does not import extension internals. A host driver exercises the installed
extension through public VS Code surfaces, then emits one bounded observation for each input case.
The runner validates the observations against `cases.json` and cross-case security invariants.

The suite covers `VSC-V0-001..076`, including the optional MCP 2026-07-28 stdio client. A pass from
`fixtures/oracle-driver.mjs` proves only that the runner recognizes the frozen observation contract
and detects selected weakenings; it is not Corvint extension evidence. Product status remains
`NOT_RUN` until an actual `@vscode/test-electron` host driver supplies observations for distinct
trusted and untrusted profiles. MCP runtime evidence separately remains `NOT_RUN` until that driver
uses a publishable, digest-pinned `corvint-mcp` implementing the frozen MCPV0 contract.

## Run

```console
node --test conformance/vscode-extension-v0/run.test.mjs
node conformance/vscode-extension-v0/run.mjs \
  --driver /absolute/path/to/vscode-host-driver.mjs
```

The runner invokes the driver directly with `process.execPath` for `.mjs`, `.cjs`, and `.js`
drivers, or directly as an executable otherwise. It never uses a shell. Each invocation receives
one case as JSON on stdin and must emit exactly one JSON object on stdout. Driver stderr is evidence
only on failure and is bounded. The default runner deadline is 5 seconds; product-side automatic
Corvint operations remain bounded to 2 seconds by the cases.

## Host-driver contract

The output profile is `corvint-vscode-host-observation/0`:

```json
{
  "profile": "corvint-vscode-host-observation/0",
  "caseId": "VSC-CONF-TRUST-001",
  "result": "denied",
  "executions": [],
  "limits": {
    "timeoutMs": 2000,
    "stdoutBytes": 0,
    "stderrBytes": 0,
    "terminated": false,
    "terminationReason": null
  },
  "ui": {
    "status": "Restricted",
    "trees": { "evidence": [], "impact": [], "why": [] },
    "diagnostics": [],
    "decorations": [],
    "tests": [],
    "output": ["Corvint execution disabled: workspace is not trusted"]
  },
  "effects": {
    "reads": [],
    "network": [],
    "telemetry": [],
    "storage": [],
    "writes": [],
    "authorityChanges": [],
    "installs": []
  },
  "residue": {
    "processes": 0,
    "watchers": 0,
    "diagnostics": 0,
    "decorations": 0,
    "tests": 0,
    "views": 0,
    "statusBar": 0,
    "storage": 0
  }
}
```

MCP observations additionally retain the selected transport, pinned executable SHA-256, raw parsed
request/result transcript, one-page tool catalogue, process lifecycle, framing counters, and
cancellation ordering. The conformance runner independently validates modern-only
`2026-07-28` metadata, `server/discover` before `tools/list`, the exact no-cursor MCPV0 registry,
closed root-free query/impact arguments, canonical text/`structuredContent` duplication, and the
direct nested context receipt. A server-reported version is bounded inert display/debug identity;
protocol, exact tools, and the pinned executable digest establish compatibility.

An execution observation has literal `executable`, `argv`, `shell`, `pathLookup`, and optional
`identity`. `shell` and `pathLookup` must always be false. The pinned cases additionally require
all executions to retain the same literal executable and identity after the fixture mutates PATH
or replaces a discovery candidate.

`effects.reads` records extension-initiated file/configuration/repository reads. It may be non-empty
for trusted discovery, but must be empty in the untrusted profile. That profile must not resolve or
open the configured path, inspect PATH candidates, stat a repository, or spawn a process.

The driver must observe public host state, child-process calls, filesystem writes, network attempts,
and extension lifecycle disposal. It must not derive a passing observation from private extension
functions. Trusted and untrusted cases require separate VS Code profiles and user-data directories:
VS Code does not allow tests to grant or revoke Workspace Trust programmatically.

## Result interpretation

- `PASS`: every selected vector and cross-case invariant passed against the installed extension.
- `FAIL`: the extension was exercised and at least one observable contract failed.
- `NOT_RUN`: no actual VS Code host driver was supplied, Electron could not start, or a required
  trusted/untrusted profile was unavailable. `NOT_RUN` is never promoted to `PASS`.

## Retained runtime dependencies

The repository has no complete per-case host driver in this conformance directory. The extension
lockfile pins `@vscode/test-electron` 3.1.0 and the installed lifecycle runner pins VS Code 1.137.0.
The complete trusted/untrusted conformance matrix remains `NOT_RUN`. Closing it requires two fresh
isolated user-data profiles,
a compiled or packaged extension, and a public-host-surface driver that emits this observation
profile without importing extension internals.

The MCP runtime row additionally requires a publishable local `corvint-mcp` executable, selected by
an absolute `corvint.mcpExecutablePath`, whose bytes and filesystem identity can be pinned. Server
source, Go unit tests, direct server conformance, or a CLI pass do not satisfy the client row.

Disable/uninstall checks require zero live process, watcher, diagnostic, decoration, test,
TreeView/status-bar, and extension-owned storage residue. Git/index/trace artifacts created by an
explicit Corvint CLI operation are Corvint Core state and outside the extension-owned residue set; the
driver must distinguish them from extension storage rather than deleting them.

## Live-provider vectors (058–076)

`exercise-live-provider` describes ordered public configuration/save/workspace actions and fixture
provider IO. `/fixture` denotes the driver's isolated temporary fixture directory; canonicalize it
in reported locations. The driver must never read `expect` to produce an observation. Scripted
provider records are inputs, not observations. Cases cover submitted snapshot states, real
diagnostics, retained completion, generation replacement, cleanup and configured argv/PATH.

The optional `liveTests` object extends `corvint-vscode-host-observation/0` without changing old
observations. It has exactly `trusted`, `checkpoints` and `events`; `trusted` is the observed public
`workspace.isTrusted` value and must match the requested profile. Checkpoints use the ordered capture labels
and copied `liveTestSnapshot(root)` phase/generation/items/digest/identity/omission values, plus
real diagnostic locations (zero-based lines) and observed TestRun message/output strings where
available. `snapshotAbsent:true` records an actual undefined export result. The checker enforces
all expected fields, bounds and closed members; generation advances and 250 ms debounce intervals
are checked independently. Diagnostic paths are fixture-relative. Events record only the event
kinds/labels selected by each vector, in observed order with monotonic `atMs`; `argv` includes the
executable, `env` projects the explicitly requested environment keys. Group probes record actual
OS group absence, never just signal delivery. Empty event selection is not proof of no processes;
pre-spawn refusal cases separately require zero executions.

Operations: `activate`, `configure` (resource settings), `stdout` (exact fixture bytes), `exit`,
`spawn-descendant` (fixture child), `save` (real document save), `capture`, `probe-group`,
`await-spawn`, `await-stdin-open` and `await-stdin-eof`. The in-flight supersession
fixture arms exact late bytes with `arm-termination-output`, records their actual emission using
`await-termination-output`, and issues `save-burst` without awaiting each replacement. Its old
provider is still alive at the first save; group absence must precede replacement spawn. Capture waits for the preceding operation's
observable completion unless it explicitly captures pending state. Saves in one burst are issued
before awaiting replacement. Drivers must use bounded deadlines and clean every owned process.

The existing installed lifecycle harness is `extensions/vscode/test/installed/`; it exercises
packaged real providers and exports/diagnostics/MCP through public surfaces. Its legacy environment
variable names and wire profiles stay compatible when their values point to Corvint artifacts.
It is not yet a complete per-case conformance driver. In particular, the stable public VS Code API
does not expose a reader for TestRun messages/output; snapshot states alone cannot prove advisory
text or exact TestRun call cardinality. Such vectors remain `NOT_RUN` unless genuine host/UI
observations exist. Do not add private production instrumentation to manufacture that evidence.

Oracle-driver live vectors return explicit `NOT_RUN`. Positive and mutated synthetic observations
in runner tests prove only checker discrimination. A `NOT_RUN` observation must contain a reason
and must omit `liveTests`; fabricated values accompanying abstention are rejected. The scratch
trusted development-host proof demonstrated one failed JS fixture record, matching snapshot digest,
real diagnostic and absent process group; it does not qualify a VSIX, real providers, an untrusted
host, or the complete suite. The full installed matrix and mandatory canonical gate run on the
final integrated release source, separately from focused leaf checks.
