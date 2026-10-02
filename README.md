<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/brand/corvint-lockup-dark.svg">
    <img src="assets/brand/corvint-lockup-light.svg" width="520" alt="Corvint">
  </picture>
</p>

<h1 align="center">Corvint</h1>

<p align="center">A <a href="https://github.com/beamfall">Beamfall</a> project.</p>

<p align="center"><strong>Context your agents can cite. Changes your reviewers can check.</strong></p>

<p align="center">
  Corvint is a local-first context compiler for coding agents. Before an edit it hands the agent
  Git-pinned evidence with the reason each item is there. After the edit it binds every diff hunk
  to cited evidence, an explicit unknown, or a verifiable mechanical exception, in a portable
  sidecar that CI checks without an LLM.
</p>

<p align="center">
  <a href="docs/INSTALL.md">Install</a> ·
  <a href="#the-workflow">Workflow</a> ·
  <a href="#status-stated-plainly">Status</a> ·
  <a href="docs/README.md">Documentation</a>
</p>

<p align="center">
  <img alt="Status: 1.0 release candidate" src="https://img.shields.io/badge/status-1.0%20release%20candidate-1971c2">
  <img alt="Architecture: local first" src="https://img.shields.io/badge/architecture-local--first-0b7285">
  <img alt="Runtime: single Go binary" src="https://img.shields.io/badge/runtime-single%20Go%20binary-00add8">
  <img alt="Module dependencies: none" src="https://img.shields.io/badge/module%20dependencies-none-2f9e44">
  <img alt="License: AGPL-3.0-or-later" src="https://img.shields.io/badge/license-AGPL--3.0--or--later-334155">
</p>

**One native Go binary.** No account, hosted service, database, embeddings, or permanent daemon.
`go.mod` declares no module requirements. Read commands do not mutate repository or trace state;
the two bounded exceptions are a local `.corvint/self-observations.jsonl` ledger and, only while
the operator marker `.corvint/unplanned-reads.enabled` exists, `.corvint/unplanned-reads.jsonl` —
neither is a direct input to ranking, evidence, or authority. Learning from these ledgers requires
an explicit operator step and a frozen held-out gate ([AGENTS.md](AGENTS.md) invariant 4). Version
`1.0.0-rc.1` is the release candidate for Corvint 1.0; [what 1.0 promises and what is still being
qualified](#status-stated-plainly).

## Why Corvint

Finding relevant text is only part of a coding task. An agent also needs to know which document
governs the change, which tests constrain it, and what evidence is missing. A reviewer needs to
follow those decisions back to their sources.

Corvint makes that evidence inspectable through context receipts and a map beside the diff.

- **Every result explains itself.** Each item in a packet carries the Git blob it came from, the
  path and line, the authority that admitted it, a confidence level, and a plain-language
  inclusion reason. A reviewer can follow any citation to immutable content.
- **Your repository is the authority.** Agent instructions, accepted decisions, and specs you
  already keep outrank syntax matches, history correlations, and learned traces. Corvint uses
  the documents you have; it introduces no new spec language.
- **It says what it does not know.** Coverage, omissions, uncertainty, and freshness are fields
  in the receipt, and a hunk without cited evidence is an explicit unknown. Task-level retrieval
  abstention is experimental and unqualified; see [Status](#status-stated-plainly).
- **Reviewers get a disposition for every hunk.** A Change Evidence Map (CEM) links each textual
  hunk of a diff to cited evidence, an explicit unknown, or a mechanical exception. The verifier
  checks patch, hunk, blob, and span identity locally, with no LLM, no index, and no shared
  service. The protocol is Apache-2.0, so anyone can implement or verify it.
- **Local and explicit.** The core reads Git and prints JSON. Default evidence flows make no
  network call. `query`, `context`, `impact`, `affected`, and non-executing `prove` modes report
  `mutates: false`, subject to the private-ledger exceptions above. `record` explicitly retains
  task outcomes; learning stays local, bounded, secret-screened, and evaluation-gated.

## Sixty seconds on this repository

You need Git and Go 1.27.1 or later; the full development gate pins exactly `go1.27.1`.
Run this from the root of a Corvint checkout on macOS or Linux:

```console
GOTOOLCHAIN=local go run ./cmd/corvint impact cmd/corvint/main.go --limit 5
```

That asks what a change to the CLI entry point could affect. The receipt names the requested
path, the tests that constrain it, the reason each result was admitted, and what the result limit
left out. No installation or account is needed. Abridged output captured from public commit
`f51f3c9e6fbfc5a6b219692e8bf46e4e43297a36` with its source-built CLI (fields, results, and
evidence records omitted; shown values unchanged). Your checkout determines the hashes and counts:

```json
{
  "context": {
    "results": [
      {
        "id": "cmd/corvint/main.go",
        "kind": "path",
        "evidence": [
          {
            "authority": "git-tree",
            "blob_hash": "3f1a1e9e858b7531c186ad05e42cde01c9d87222",
            "confidence": "authoritative",
            "line": 1,
            "path": "cmd/corvint/main.go",
            "reason": "requested changed path"
          }
        ]
      },
      {
        "id": "cmd/corvint/main_test.go",
        "kind": "test",
        "evidence": [
          {
            "authority": "test-convention",
            "blob_hash": "1b757630da987860fd3b2d321981d8fc6f09dcca",
            "confidence": "high",
            "line": 1,
            "path": "cmd/corvint/main_test.go",
            "reason": "same-package test for cmd/corvint/main.go"
          }
        ]
      }
    ],
    "freshness": {
      "revision": "099c102b26602c83ec91a9f8f7eb7491e0b3f040",
      "scope": "git",
      "state": "fresh"
    },
    "coverage": {
      "included_results": 5,
      "omitted_results": 267,
      "uncertainty": [
        "267 ranked results omitted by result limit"
      ]
    }
  },
  "mutates": false,
  "tool": "impact"
}
```

Install the binary to use Corvint in any supported Git repository:

```console
mkdir -p "$HOME/.local/bin"
GOTOOLCHAIN=local go build -ldflags "-X main.build=$(git rev-list --count --first-parent HEAD)" -o "$HOME/.local/bin/corvint" ./cmd/corvint
export PATH="$HOME/.local/bin:$PATH"
corvint --version
```

Versioned assets are on the [releases page](https://github.com/beamfall/corvint/releases). The
[Core rc.1 release](https://github.com/beamfall/corvint/releases/tag/v1.0.0-rc.1) contains only
`corvint`, with four macOS/Linux archives, `SHA256SUMS` and `verification-report.json`. A matching
native archive needs Git but no Go compiler; verify both the downloaded archive and its internal
checksums before running it. Read the exact platform's qualification, including native versus
emulated runs. Tasks has its own developer release; source companions are not automatically
included in a Core archive. The [installation guide](docs/INSTALL.md) covers checks, first use,
optional tools, upgrades and removal.

## The workflow

| When | Ask Corvint to | What you get back |
|---|---|---|
| Before an edit | Find the context and the likely impact | Git-pinned paths, governing documents, related tests, and the reason each was included |
| While working | Show the evidence boundary | Freshness, coverage, omissions, uncertainty, and explicit abstention |
| At review and in CI | Attach a Change Evidence Map to the diff | A disposition for every textual hunk that CI verifies locally |

| Command | Purpose |
|---|---|
| `corvint query --task "change session revocation" --budget-bytes 8000` | Find task evidence within a byte budget |
| `corvint context --task "change session revocation" --subject src/session.py` | Build a packet around a tracked subject |
| `corvint impact src/session.py` | Inspect the likely impact of a tracked path |
| `corvint affected` | Plan Go test packages for the dirty worktree; run none |
| `corvint prove --task "change session revocation"` | Produce a falsifiable packet with freshness and uncertainty |
| `corvint overview` / `corvint features` | Inspect a clean repository overview or inferred feature candidates |
| `corvint review --base FULL_BASE_SHA --max-refs 8` | Prepare immutable-range test advice and bounded branch-overlap hints |

Replace `src/session.py` with a path tracked in your repository. Go, Python, and JS/TS have
reverse-import impact rules; other indexed languages do not all receive equivalent analysis.
`corvint COMMAND --help` prints each command's contract. To select a repository explicitly, put
`--root /path/to/repo` **before** the command.

The experimental `features`, `overview`, and `review` commands require a clean tree and expose
inferred scope, omissions, and unsupported cases. They do not accept requirements or execute
suggested commands. `review` requires a full ancestor base SHA and does not replace CEM or tests.

## Give the agent a useful next step

A context packet can identify a test counterpart and say why it matters. From the
session-revocation fixture (fields omitted; values unchanged):

```json
{
  "id": "src/test_session.py",
  "kind": "pair",
  "action": "Update this test: it is the subject's test counterpart, so a behaviour change in the subject changes what it must assert.",
  "evidence": [{
    "authority": "test-convention",
    "confidence": "high",
    "blob_hash": "50404716f6d80b4cbfc88c61529c78bd748250c4",
    "reason": "test counterpart of src/session.py"
  }]
}
```

After `src/session.py` is edited, `prove` reports `mixed-worktree` freshness and names the changed
path. Trace recording is blocked for that mixed state. The receipt keeps working changes apart from
committed evidence, so the agent and the reviewer can see the difference.

## Put the evidence beside the diff

A **Change Evidence Map** is a portable sidecar that travels in the repository and verifies with
no LLM, no index, and no shared service. For a committed change, replace `BASE_SHA` with its base
commit and the example citation with a span that exists at that base:

```console
corvint cem prepare --base BASE_SHA --target HEAD
corvint cem cite --map .corvint/change.cem.json --hunk 1 \
  --evidence-path docs/decisions/0001-session-revocation.md --lines 5:5 --relation decision
corvint cem status --map .corvint/change.cem.json \
  --expected-base BASE_SHA --target HEAD --max-unknown 0 --max-mechanical 0
```

For a one-hunk change, `prepare` produces an incomplete worklist; after the citation, `status`
reports `"state": "ready-for-ci"` with a stable span. A larger diff needs a disposition for every
hunk, and the worklist says what remains. The policy above permits no unknowns and no mechanical
exceptions. [CEM in CI](docs/CEM-CI.md) wires that policy into a pipeline.

The `nextActions` argv that `cem prepare` and `ocm prepare` return starts with the command name
`corvint`. Run it with a `corvint` binary in that first position, or build one under that name
(`go build -o corvint ./cmd/corvint`).

The verifier proves structural integrity: patch, hunk, blob, span, and mapping identity. It does
not prove semantic support, causality, test adequacy, or program correctness, and Corvint says so.
The [wire format, schemas, and conformance vectors](docs/CHANGE-EVIDENCE-MAP.md) are Apache-2.0.

## Optional Go LSP evidence

The following CLI/MCP selectors are newer source additions; they are absent from the published
Core rc.1 binary. Build the current source and matching MCP companion, or check your installed
`corvint help context` before using them.

Corvint can use `gopls`, the Go language server, to add definition and reference relationships
to a context packet. This integration is **experimental and opt-in**; it does not change ranked
results or project authority. Install `gopls` explicitly and have your workspace dependencies
available locally, then select a tracked Go file (here, in Corvint’s checkout):

```sh
corvint context --task "Find callers and dependencies" \
  --subject cmd/corvint/main.go --lsp gopls
```

Use `--lsp off` to disable enrichment, including when the legacy environment setting enables it.
For MCP, start `corvint-mcp` with `--tool-profile task-review-lsp` and pass `"lsp": "gopls"` to
`corvint.context`; omitting that argument starts no language server. Other MCP profiles do not
enable LSP. See the [Go LSP guide](docs/LSP.md) for installation, full CLI/MCP examples, limits,
qualification, and troubleshooting. Other language servers are outside this milestone.

## Works where agents work

The host adapters run `corvint` from `PATH`, so install it there first. Core lifecycle adapters report
`FALLBACK` ([compatibility matrix](integrations/README.md)). The optional OpenCode native integration
has a separate exact-tuple qualification; its `integrationSupport: FULL` does not confer execution
or closing Frontier authority ([OpenCode support](integrations/opencode/README.md)). The Codex and Claude Code
adapters are Core host rows; Gemini CLI, OpenCode and Pi are companions:

- [Codex](integrations/codex/README.md), [Claude Code](integrations/claude-code/README.md),
  [Gemini CLI](integrations/gemini-cli/README.md), [OpenCode](integrations/opencode/README.md),
  and [Pi](integrations/pi/README.md) share one bounded lifecycle receipt
  (`corvint help harness event`).
- [VS Code](extensions/vscode/README.md), deferred and not part of 1.0: evidence views plus opt-in
  automatic unit/E2E reruns on editor saves. Install the VSIX and providers, configure the project's toolchains and suite, then
  enable `corvint.liveTests.enabled`. It reruns the configured suite; passing tests do not
  establish test adequacy or authority. [Setup and stop instructions](docs/INSTALL.md#editor-and-test-feedback).
- [MCP](docs/MCP-SERVER.md): `corvint-mcp` is a **companion** local stdio server that exposes
  read-only receipts (`GOTOOLCHAIN=local go build -o ./bin/corvint-mcp ./cmd/corvint-mcp`).
  `corvint-docs-mcp` drafts source-bound documentation and checks a draft against source;
  `corvint-test-validity-mcp` lets agents inspect retained test observations. Both need a
  compatible client and an explicit root; [setup and boundaries](docs/INSTALL.md#agent-tools-and-source-documentation).
- Source-bound documentation: the companion `docs maintain --watch` command refreshes a
  generated page block as eligible source commits land, preserves surrounding prose, and stops on
  outside page edits. It runs explicitly in the foreground on macOS/Linux with time and write
  limits; [preview, apply, and watch](docs/INSTALL.md#agent-tools-and-source-documentation).
- The workflow tools below: tickets, dashboard, console, test providers and release checks.
  Optional Tasks supervision and dispatch start only through explicit operator configuration;
  they have their own qualification and process-lifecycle limits.

Corvint uses the `corvint-*` wire/profile namespace, `corvint.*` MCP tools, and canonical
`.corvint` and `.context-corvint` repository paths. Those are protocol and state contracts and are
versioned independently of the product. The frozen minimum CEM wire is `cem/0.2` with an N-1
`cem/0.1` reader. The separate experimental `cem/0.3` adds structural mechanical reasons and
optional test witnesses; it is not the default Core sidecar or a new stability claim
([profile boundaries](docs/CHANGE-EVIDENCE-MAP.md)).

## The rest of the toolbox

The core CLI works on its own. Separately packaged tools expose test results, tickets, and evidence;
some also run tests, supervise agents or perform explicit local writes. They are qualified separately
from Core and do not inherit its stability promise. Selected Flows, Tasks, documentation and MCP
outcomes are required for stable 1.0 promotion ([expanded scope](docs/specs/corvint-1.0-product-and-release-v1.md#expanded-10-product-scope)).

<details>
<summary>Explore optional companions, deferred editor tooling, and experimental tools</summary>


### Editor: evidence views and live test feedback

The [VS Code extension](extensions/vscode/README.md), deferred from 1.0, runs the same `corvint` binary (or
`corvint-mcp` over stdio) and projects one bounded result into Evidence, Impact and Why views,
diagnostics and decorations. Executables are pinned by identity and revalidated before every run;
suggested verification commands are shown, never executed.

Opt in to `corvint.liveTests.enabled` and the extension becomes a save-triggered test loop:

| Provider | What it runs | Receipt |
|---|---|---|
| `corvint-js-test-provider unit` | The configured Vitest suite | Test identity, pass/fail per test, failure locations as diagnostics |
| `corvint-js-test-provider e2e` | Playwright with an explicit app server, readiness URL and browser | Same shape; test failures separated from infrastructure failures |
| `corvint-go-test-provider` | Go packages, in a preview-only foreground session that reruns on bounded file changes | Discovery, plan, run and observation receipts; imported with **Corvint: Import Test Observation** |

Saves in the declared watch paths rerun the full configured suite; rapid saves coalesce, and a
new save clears the previous result. Completed runs stay in Test Explorer. With
`corvint.liveTests.retainEvidence`, the same completed documents are retained under
`.corvint/test-evidence` where `corvint-test-validity-mcp` lets an agent read them. A green run
is a fact about that run: it does not establish freshness, adequacy or authority, and the
unknown axes stay visible ([contract](docs/specs/js-live-test-provider-v0.md),
[Go provider](docs/specs/go-live-test-provider-v0.md)).

### Framework-aware test selection

The live providers above are only part of the test tooling. `corvint affected` also reads source
and configuration to recognize test units across the following ecosystems. It produces a plan;
it does not launch these runners or establish that omitted tests are safe to skip.

| Ecosystem | Recognized test conventions and runners |
|---|---|
| Go | `go test` packages |
| JavaScript/TypeScript | Vitest, Jest, AVA, Node `node:test`, Bun, Deno, Playwright, Cypress, WebdriverIO, TestCafe, Nightwatch, Detox, Storybook test-runner and Storybook Vitest |
| Python | pytest and unittest file conventions |
| Ruby | RSpec, Minitest/Test::Unit and Rails test conventions |
| Rust | Cargo packages and source test anchors |
| Swift | SwiftPM/Xcode targets, XCTest and Swift Testing |
| Kotlin/Android | JUnit 4/5 and kotlin.test class conventions |
| C#/.NET | VSTest-addressable test methods in SDK-style projects |

Dynamic configuration, ambiguous frameworks, unresolved imports, external runtime/device inputs
and other unsupported cases remain explicit unknowns in the plan. See the
[affected-plan contract](docs/specs/affected-plan-v0.md) and the
[language implementations](internal/liveverify/affected/languages/languages.go) for the exact
bounds. The experimental `corvint prove --mutate` can run bounded Go and pytest mutation checks
when its sandbox and offline prerequisites are available; that is a separate execution path.

### Task manager and work queue

`corvint-tasks` is the local ticket store and roadmap. It is a separate companion binary built
from this repository's `cmd/corvint-tasks` (never a `corvint` subcommand) and owns ticket state:
the console delegates every ticket mutation to it. The store needs no server or account. The
[standalone Tasks developer release](https://github.com/beamfall/corvint/releases/tag/tasks-dev-20260929.2)
provides a macOS Apple silicon archive, checksums and build-verification evidence; its unverified
version and runtime qualification limits remain explicit. Use `corvint-tasks version` and
`corvint-tasks help` to check the installed build and commands. Other targets can be built from
source; build success is not runtime qualification.

Optional [foreground Codex supervision and continuous dispatch](docs/TASKS-SUPERVISION.md) are
operator-started source capabilities. Supervision has scoped local qualification; dispatch and
newer source commands require their own checks and are not all present in that developer archive.
Neither route inherits Core stability or grants publication authority.
Optional named environment pools allocate isolated members with claims and quarantine them until
explicit safe-reuse confirmation; see [external-agent usage](docs/TASKS-EXTERNAL-AGENTS.md#isolated-environment-pools).

`corvint work observe` and `corvint work propose-wave` (also `corvint-work-queue`) read a
queue snapshot and return deterministic shadow proposals: derived path clashes between tickets
and the largest collision-free wave that could run together. The proposals authorize nothing and
carry their inputs' digests, so a second run over the same snapshot reproduces them byte for byte
([Work Queue Observation V0](docs/specs/work-queue-observation-v0.md)).

To adopt the work queue in a repository, run
`corvint work init --repository NAME --corvint-executable "$(command -v corvint)"`, review and
commit the three files it writes under `.corvint/`, and list tickets in `.corvint/worklist.json`
with the paths each one changes. Verification work such as a suite batch, a failure repair, a
test-validity receipt, or a cleanup and retry is an ordinary ticket. Tickets that share a path are
never proposed together; the proposal names the excluded ticket and why. The executable may be in
`~/.local/bin`, `/opt/homebrew/bin`, `/usr/local/bin`, or another safe absolute location: init binds
its path, bytes, version/build and source identity instead of searching ambient `PATH`. After an
upgrade, run `corvint work rebind --corvint-executable "$(command -v corvint)"`, review and commit
the adapter change. See
[INSTALL](docs/INSTALL.md#work-queue-adoption) for the states you get when a step is missing.

### Dashboard and console

`corvint-dashboard-snapshot` compiles one read-only snapshot of what Corvint data exists for a
repository: the revision and authority each artifact represents, what is stale or absent, which
denominators were never observed, and the exact artifact and verifier behind every aggregate.
Its `roadmap` subcommand projects the ticket roadmap the same way. A snapshot is an inspector,
not a scoreboard: an unavailable denominator is reported as unavailable, never as zero
([Local Observability Dashboard V0](docs/specs/local-observability-dashboard-v0.md)).

`corvint-console` serves that snapshot, the ticket board and detail, the specification index and
a code pane on a loopback address you start explicitly:

```sh
corvint-console --repo /absolute/path/to/repo --tasks /path/to/corvint-tasks \
  --snapshot /path/to/corvint-dashboard-snapshot
```

Open `http://127.0.0.1:7777`, stop it with Ctrl-C. It installs nothing, opens no outbound
connection and holds no database ([Local Admin Console V0](docs/specs/local-admin-console-v0.md)).

### Agent-facing servers

Four stdio MCP servers, each bound to one repository root, each read-only. The companion bundle
builder includes `corvint-mcp`, `corvint-docs-mcp` and `corvint-test-validity-mcp`; use a published
bundle only when its exact assets and qualification evidence exist. They are not in the Core rc.1
archive. `corvint-corpus-mcp` is source-only and experimental:

| Server | Tools |
|---|---|
| `corvint-mcp` | `corvint.query`, `corvint.impact` and `corvint.status`: the same bounded context, Go impact and repository-status receipts as the CLI; with `--tool-profile task-review`, `corvint.context` and `corvint.cem.report` also return the task-context packet and a non-publishing CEM report preview |
| `corvint-docs-mcp` | `corvint.docs_draft` returns a source-pinned documentation draft from owner prose and indexed Go declarations; `corvint.docs_consume` rechecks a draft's exact bytes against source |
| `corvint-corpus-mcp` (source-only) | Experimental [revision-pinned documentation corpus](docs/DOCUMENTATION-CORPUS.md); capability-gated read tools over one explicitly supplied local artifact |
| `corvint-test-validity-mcp` | Discovery and projection of retained test evidence in one five-axis shape |

`corvint docs maintain --watch` is the foreground companion to the docs server: it refreshes a
generated page block as eligible source commits land, preserves the surrounding prose, and stops
on any outside edit or when its write and wall-clock limits are reached
([setup](docs/INSTALL.md#agent-tools-and-source-documentation), [protocol](docs/MCP-SERVER.md)).

### Sixteen language analyzers

`cmd/corvint-analyzer-*` are candidate analyzers for Go, Python, JavaScript/TypeScript, Java,
Kotlin/Android, Swift, Objective-C, C/JNI, .NET, Ruby, Rust, shell, SQL, shaders, HTML/CSS and
structured data. Each emits facts with byte spans and evidence digests under a frozen candidate
profile and is admitted to the product only through its own accepted profile ([candidate profiles](docs/specs/analyzer-candidate-profiles.md)).

### Coordination and release checks

- `corvint-pulse`: a bounded local coordinator for snapshot lifecycle state with frozen
  transcripts; leases and deltas are not delivered ([Pulse Snapshot Lease V0](docs/specs/pulse-snapshot-lease-v0.md)).
- `corvint-companion-release` assembles the companion bundle from two clean checkouts and
  writes its manifest and checksums; `corvint-public-release-check` qualifies one retained
  bundle; `corvint-release-gate` is an offline evidence gate; `corvint-go-toolchain-receipt`
  digests a GOROOT tree into a receipt. None publishes anything.

### Flow variation coverage

[Flow variation coverage](docs/FLOW-COVERAGE.md) compares the complete declared documentation
inventory with accepted flow variations and retained original Playwright `/3` receipts. The CLI
can explicitly write a fresh documentation projection; the opt-in MCP `flows` profile is read-only.
Proof binds immutable bytes and declared application identity; deployment attestation remains
outside `/3`. Use the guide's exact input/profile and qualification requirements.

### Experimental editor definition companion

The separate `corvint-lsp` prototype can use an explicitly supplied local gopls binary for
definitions between open Go overlays in one explicitly supplied root:

```sh
go build -o /tmp/corvint-lsp ./cmd/corvint-lsp
/tmp/corvint-lsp --experimental --gopls /absolute/path/to/gopls --root /absolute/project/root
```

The editor must launch the process over stdio and declare the same canonical root. The companion
uses full-text synchronization and UTF-8/16/32 positions; non-open targets return unavailable.
The backend receives offline Go settings, but the executable is trusted local code, not sandboxed.

In a Git root, add `--workspace-drift-guard` to refuse definitions after observed disk, save, branch
or root drift until a new session. The guard hashes the whole workspace within fixed bounds; larger
repositories may exceed its budget. External SDK and caches remain unbound, so this is a partial
experimental guard. Omit the flag to disable it.

This operator-started experiment changes no Core defaults and installs no editor configuration.
It exposes a `corvintDefinitionProbe` experimental marker and accepts direct development `textDocument/definition` requests, but does not advertise the standard definition capability. Automatic editor navigation remains unavailable until an exact client tuple is qualified. See the
[experimental contract](docs/specs/lsp-editor-definition-v0.md) for bounds and rollback.

The optional Go editor companion also accepts experimental `corvint/context` with closed params
`{"textDocument":{"uri":"file:///absolute/root/file.go"},"task":"investigate a requirement","limit":10}`.
The document must be open. Discover the method/schema under `capabilities.experimental.corvintContext`.
The response preserves the task-review Core object, including abstentions, separately from an
unsaved overlay observation (random session ID, decimal-string capture ID, version and SHA-256).
It starts one bounded native context worker, with no second gopls, tests or repository writes.
Point-in-time Git/branch/root and overlay checks fail closed on observed drift. This remains
experimental; exact client tuples and outcome/performance floors are unqualified. See the
[LSP editor context contract](docs/specs/lsp-editor-context-v0.md) for bounds, fixed errors and observation limits.

</details>

## Built to be checked

| | |
|---|---|
| Spec-driven | Every substantive capability has an executable spec with stable requirement IDs in `docs/specs/REQUIREMENTS.tsv`; `go run ./script/spec-coverage-audit` reports test, case, and fixture mentions separately from comments and missing mentions |
| Decision records | Numbered, accepted intent with explicit promotion boundaries in `docs/decisions/` |
| Frozen conformance | Exact receipt and state replay, CEM/LRF/TCQ vectors, and an in-repo second consumer for `cem/0.1` in `interop/cem01-go` |
| Honest disagreements | Every known behavioural disagreement is adjudicated and dated in the [divergence register](conformance/divergence-register.md) |
| Hermetic archives | `make gate` includes `script/go-archive-gate`, which rebuilds the release archives from the committed revision and checks the closed file set ([spec](docs/specs/go-archive-gate-v0.md)) |
| Dogfooded | Substantive Corvint changes must collect context with Corvint and bind the diff to a CEM ([dogfood contract](docs/DOGFOOD.md)) |

## Status, stated plainly

> [!IMPORTANT]
> `1.0.0-rc.1` is the release candidate for Corvint 1.0. The 1.0 stability promise (frozen
> contracts, N-1 readers or deterministic migrations) covers the Core surfaces below and nothing
> else ([1.0 scope](docs/specs/corvint-1.0-product-and-release-v1.md), decision 0373). Stable 1.0
> also requires the expanded Flows, safe navigation, documentation/MCP, full Beamfall roadmap
> takeover by Tasks and automatic documentation outcomes accepted in decision 0426. Those
> requirements do not expand Core's binary boundary or transfer its stability promise to
> companions. The three Core jobs must pass on Corvint, Beamfall and one untouched public
> repository (`PRS-V1-008`), alongside the expanded product qualification. Platform status comes
> from each release's exact assets and evidence. The table distinguishes retained rc.1 evidence
> from the expanded scope; it does not qualify later source changes.

| Surface | 1.0 label | Release evidence and current scope |
|---|---|---|
| `init`, `adopt`, `index`, `query`, `context`, `impact`, `affected`, `prove` | Core | Command, wire and migration contracts frozen; `init`, `adopt` and the deterministic index lifecycle qualified. Receipts carry coverage, omissions and uncertainty as specified. |
| CEM `0.1` / `0.2`, OCM, change frontier | Core | Frozen with canonical conformance vectors. `interop/cem01-go` is an in-repo second consumer for `cem/0.1` only; 1.0 claims no third-party interoperability. |
| Dogfood loop | Core | Substantive Corvint changes are bound to a CEM and sealed with a retained local outcome ([dogfood contract](docs/DOGFOOD.md)). |
| Core jobs | Core | **The rc.1 evaluation failed overall.** Orientation missed critical test files in 3/20 cases on go-chi/chi and 1/20 on Beamfall. Consequence and completion passed on those repositories; the Corvint run aborted before scoring. These results block 1.0 final ([release evidence](docs/RELEASE-NOTES.md#100-rc1-release-candidate)). |
| Native release artifact and install lifecycle | Core | darwin/arm64 and linux/amd64 have retained rc.1 install-lifecycle qualification. darwin/amd64 was tested under Rosetta 2 and linux/arm64 in a container; both remain `FALLBACK`. Windows is unsupported. The candidate is unsigned; publisher identity is `NOT_VERIFIED` ([release evidence](docs/RELEASE-NOTES.md#100-rc1-release-candidate)). |
| Retrieval quality | Core surface, unqualified ranking | Bounded receipts around a named path or subject are the product. Broad task-to-evidence retrieval has not passed held-out evaluation: the retained held-out attempt beat the exact-search baseline on top-5 (0.571 vs 0.343) and met the abstention and latency bars, but returned forbidden results on 7 of 36 must-exclude checks. Do not rely on ranking or abstention. |
| Does CEM help a reviewer? | Not claimed | **Unproven.** A five-pair pilot scored mean missed evidence of 0.90 for control and 0.86 with CEM. It is a pilot, not a held-out outcome study. |
| Performance | Not claimed | No native performance qualification is claimed for rc.1. Earlier measurements compared against the retired Python runtime and do not qualify the Go-only release. |
| Host adapters | Core (Codex, Claude Code), companion (Gemini CLI, OpenCode, Pi) | Core lifecycle receipts report `FALLBACK`; formal `FULL` host authority is post-1.0. OpenCode native integration has separate exact-tuple qualification and no execution authority. The VS Code extension is deferred. |
| Flows, safe navigation, Tasks takeover, documentation and MCP | Required 1.0 product outcomes, independently packaged | Expanded stable-release qualification remains pending under decision 0426; source presence and local gates do not establish whole-product acceptance. |
| MCP servers, `corvint-tasks`, dashboard, console, test providers, docs | Companion | Qualified separately, with no Core stability promise. Selected outcomes above are required for stable 1.0; unrelated consoles and providers remain optional. The console and dashboard present evidence and never hold authority over it. |
| Learned traces, work queue, Pulse, evaluation verbs | Experimental | Shipped without a promise. A learned-path change is admitted only through a pinned two-arm evaluation, and none is qualified for this release. |

These boundaries are backed by committed artifacts: the [1.0 scope](docs/specs/corvint-1.0-product-and-release-v1.md),
the [release notes](docs/RELEASE-NOTES.md), the [specification index](docs/specs/README.md), and
[`benchmarks/`](benchmarks/).

## Read next

- [Documentation and repository layout](docs/README.md): current guides and source organization
- [Product contract](docs/PRODUCT.md): the job, trust boundary, and measurable product loop
- [Architecture](docs/ARCHITECTURE.md): how the pieces fit
- [Agent evidence routes](docs/AGENT-ROUTES.md): task-sized paths to authority, code, tests, and next action
- [Specification index](docs/specs/README.md): accepted intent versus actual delivery state
- [Change Evidence Map](docs/CHANGE-EVIDENCE-MAP.md): the interchange contract
- [CEM in CI](docs/CEM-CI.md): wiring the verifier into a pipeline
- [Dogfood contract](docs/DOGFOOD.md): how Corvint must prove substantive Corvint changes

## Development

For a scoped change, use `corvint affected --base FULL_BASE_SHA`, retain its unknowns, then run
focused tests and the checks required by the owning contract. Follow the [dogfood contract](docs/DOGFOOD.md)
and obtain independent review. Release or repository-wide validation uses the exhaustive gate
below, which includes root tests/vet, interoperability, archive and documentation checks:

```console
test "$(GOTOOLCHAIN=local go env GOVERSION)" = "go1.27.1"
GOTOOLCHAIN=local make gate
script/release-checklist      # read-only; PASS/FAIL/NOT_RUN, exits 0 only when all required rows pass
```

Standalone root or interoperability tests use `go test -count=1 -timeout 30m ./...` and `go vet ./...`
in the corresponding module with `GOTOOLCHAIN=local`. The 30-minute limit is a per-package hang
detector. Do not rerun the same checks before `make gate` unless changed inputs or a failure require it.

If `GOTOOLCHAIN=local go env GOVERSION` reports a different patch version than `go1.27.1` (for
example, a Homebrew `go` formula upgrade changed the linked toolchain), install the exact pinned
version alongside it rather than relinking Homebrew's default, then prepend its `bin` directory to
`PATH` for gate commands only, e.g. `PATH=/opt/homebrew/Cellar/go/1.27.1/bin:$PATH GOTOOLCHAIN=local make gate`
(Intel Homebrew: `/usr/local/Cellar/go/1.27.1/bin`).

Gemini/OpenCode adapter tests use their hosts' Node runtime. Core builds and the native regression
suite do not require Python; optional qualification scripts, such as the live Go LSP campaign,
may require Python 3 and their declared external tools. See [native regression ownership](tests/README.md).

## License

Corvint is free software under the **GNU Affero General Public License v3.0 or later**. The portable
protocol descriptions, schemas, conformance material, examples, and interop implementations are
**Apache-2.0** instead, so anyone can implement the standard, including in proprietary software,
without the copyleft attaching. See [LICENSING.md](LICENSING.md) for the exact path boundary,
[LICENSE](LICENSE) for the AGPL text, [LICENSE-APACHE-2.0](LICENSE-APACHE-2.0) for the Apache text,
and [PROVENANCE.md](PROVENANCE.md).
