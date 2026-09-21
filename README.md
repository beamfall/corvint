<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/brand/corvint-lockup-dark.svg">
    <img src="assets/brand/corvint-lockup-light.svg" width="520" alt="Corvint">
  </picture>
</p>

<p align="center"><strong>Context your agents can cite. Changes your reviewers can check.</strong></p>

<p align="center">
  Corvint is a local-first context compiler for coding agents. Before an edit it hands the agent
  Git-pinned evidence with the reason each item is there. After the edit it binds every diff hunk
  to cited evidence, an explicit unknown, or a verifiable mechanical exception, in a portable
  sidecar that CI checks without an LLM.
</p>

<p align="center">
  <img alt="Status: extraction alpha" src="https://img.shields.io/badge/status-extraction%20alpha-d9a441">
  <img alt="Architecture: local first" src="https://img.shields.io/badge/architecture-local--first-0b7285">
  <img alt="Runtime: single Go binary" src="https://img.shields.io/badge/runtime-single%20Go%20binary-00add8">
  <img alt="Module dependencies: none" src="https://img.shields.io/badge/module%20dependencies-none-2f9e44">
  <img alt="License: AGPL-3.0-or-later" src="https://img.shields.io/badge/license-AGPL--3.0--or--later-334155">
</p>

**One native Go binary.** No account, hosted service, database, embeddings, or permanent daemon.
`go.mod` declares no module requirements. Read commands change nothing. Version `0.5.0a2` is an
experimental alpha; [what works today and what is still an open gate](#status-stated-plainly).

## Why Corvint

Coding agents do not run out of tokens. They run out of evidence. A grep or an embedding search
returns text; the agent still has to guess which file governs the change, which test constrains
it, and what it never saw. The reviewer then inherits a diff with no trail back to why.

Corvint replaces the guess with a receipt and the opaque diff with a map.

- **Every result explains itself.** Each item in a packet carries the Git blob it came from, the
  path and line, the authority that admitted it, a confidence level, and a plain-language
  inclusion reason. A reviewer can follow any citation to immutable content.
- **Your repository is the authority.** Agent instructions, accepted decisions, and specs you
  already keep outrank syntax matches, history correlations, and learned traces. Corvint uses
  the documents you have; it introduces no new spec language.
- **It says what it does not know.** Coverage, omissions, uncertainty, freshness, and explicit
  abstention are fields in the receipt, not an afterthought. Missing evidence produces an
  unknown, never invented certainty.
- **Reviewers get a disposition for every hunk.** A Change Evidence Map (CEM) links each textual
  hunk of a diff to cited evidence, an explicit unknown, or a mechanical exception. The verifier
  checks patch, hunk, blob, and span identity locally, with no LLM, no index, and no shared
  service. The protocol is Apache-2.0, so anyone can implement or verify it.
- **Nothing to run, nothing to trust.** The core reads Git and prints JSON. Default evidence
  flows make no network call. `query`, `context`, `impact`, `affected`, and `prove` report
  `mutates: false`; learning happens only on an explicit `record`, stays local, bounded, and
  secret-screened, and cannot change ranking without passing a pinned evaluation gate.

## Sixty seconds on this repository

You need Git and Go 1.27 (`go.mod` requires 1.27.1; the full gate pins exactly `go1.27.1`).
From a checkout on Darwin or Linux:

```console
GOTOOLCHAIN=local go run ./cmd/corvint impact cmd/corvint/main.go --limit 5
```

That asks what a change to the CLI entry point could affect. The receipt names the requested
path, the tests that constrain it, the reason each result was admitted, and what the result limit
left out. No installation or account is needed. Abridged output from a `0.4.0a4` checkout (fields,
results, and evidence records omitted; shown values unchanged):

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
            "confidence": "authoritative",
            "blob_hash": "b6a95d5864bcbaaebb4832e8eea9f0d4966478ea",
            "line": 1,
            "reason": "requested changed path"
          }
        ]
      },
      {
        "id": "cmd/corvint/harness_context_test.go",
        "kind": "test",
        "evidence": [
          {
            "authority": "test-marker",
            "confidence": "high",
            "blob_hash": "4ea3efb9bc13016ec38f4df1d6a4f1ee363366a2",
            "line": 30,
            "reason": "same-package test carries feature:session-revocation"
          },
          {
            "authority": "test-convention",
            "confidence": "medium",
            "blob_hash": "4ea3efb9bc13016ec38f4df1d6a4f1ee363366a2",
            "line": 1,
            "reason": "same-package test for cmd/corvint/main.go"
          }
        ]
      }
    ],
    "freshness": {
      "scope": "git",
      "state": "fresh"
    },
    "coverage": {
      "included_results": 5,
      "omitted_results": 175,
      "uncertainty": [
        "175 ranked results omitted by result limit"
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

When versioned release assets are available, the native archive whose attached qualification
evidence names your platform needs Git but no Go compiler. The [installation
guide](docs/INSTALL.md) covers archive checks, first use, optional workflow tools, upgrades, and
removal.

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

## Works where agents work

The host adapters run `corvint` from `PATH`, so install it there first. Every adapter is a
developer preview reporting `FALLBACK`, not `FULL` ([compatibility matrix](integrations/README.md)):

- [Codex](integrations/codex/README.md), [Claude Code](integrations/claude-code/README.md),
  [Gemini CLI](integrations/gemini-cli/README.md), [OpenCode](integrations/opencode/README.md),
  and [Pi](integrations/pi/README.md) share one bounded lifecycle receipt
  (`corvint help harness event`).
- [VS Code](extensions/vscode/README.md): evidence views plus opt-in automatic unit/E2E reruns on
  editor saves. Install the VSIX and providers, configure the project's toolchains and suite, then
  enable `corvint.liveTests.enabled`. It reruns the configured suite; passing tests do not
  establish test adequacy or authority. [Setup and stop instructions](docs/INSTALL.md#editor-and-test-feedback).
- [MCP](docs/MCP-SERVER.md): `corvint-mcp` is an **experimental** local stdio server that exposes
  read-only receipts (`GOTOOLCHAIN=local go build -o ./bin/corvint-mcp ./cmd/corvint-mcp`).
  `corvint-docs-mcp` drafts source-bound documentation and checks a draft against source;
  `corvint-test-validity-mcp` lets agents inspect retained test observations. Both need a
  compatible client and an explicit root; [setup and boundaries](docs/INSTALL.md#agent-tools-and-source-documentation).
- Source-bound documentation: the experimental `docs maintain --watch` command refreshes a
  generated page block as eligible source commits land, preserves surrounding prose, and stops on
  outside page edits. It runs explicitly in the foreground on macOS/Linux with time and write
  limits; [preview, apply, and watch](docs/INSTALL.md#agent-tools-and-source-documentation).
- The optional workflow tools below: tickets, dashboard, console, test providers and release
  checks. None is a hosted service and none dispatches agents.

Corvint uses the `corvint-*` wire/profile namespace, `corvint.*` MCP tools, and canonical
`.corvint` and `.context-corvint` repository paths. Those are protocol and state contracts and are
versioned independently of the product.

## The rest of the toolbox

The CLI is the product. Around it, this repository ships the tools that make a working session
observable: what tests just said, what work is queued, what evidence exists and how fresh it is.
Every one of them is local, explicit and read-through; none holds authority over the repository.

### Editor: evidence views and live test feedback

The [VS Code extension](extensions/vscode/README.md) runs the same `corvint` binary (or
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

### Task manager and work queue

`corvint-tasks` is the local ticket store and roadmap. It lives in its own source module,
is built into the companion bundle beside `corvint`, and owns ticket state: the console delegates
every ticket mutation to it. No server, no account, no agent dispatch.

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

Three stdio MCP servers, each bound to one repository root, each read-only:

| Server | Tools |
|---|---|
| `corvint-mcp` | `corvint.query`, `corvint.impact` and `corvint.status`: the same bounded context, Go impact and repository-status receipts as the CLI |
| `corvint-docs-mcp` | `corvint.docs_draft` writes source-pinned documentation from owner prose and indexed Go declarations; `corvint.docs_consume` rechecks a draft's exact bytes against source |
| `corvint-corpus-mcp` | Experimental [revision-pinned documentation corpus](docs/DOCUMENTATION-CORPUS.md); capability-gated read tools over one explicitly supplied local artifact |
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

## Built to be checked

| | |
|---|---|
| Spec-driven | Every substantive capability has an executable spec with stable requirement IDs in `docs/specs/REQUIREMENTS.tsv`; `go run ./script/spec-coverage-audit` reports test, case, and fixture mentions separately from comments and missing mentions |
| Decision records | Numbered, accepted intent with explicit promotion boundaries in `docs/decisions/` |
| Frozen conformance | Exact receipt and state replay, CEM/LRF/TCQ vectors, and an independent Go interoperability consumer in `interop/cem01-go` |
| Honest disagreements | Every known behavioural disagreement is adjudicated and dated in the [divergence register](conformance/divergence-register.md) |
| Hermetic archives | `make gate` includes `script/go-archive-gate`, which rebuilds the release archives from the committed revision and checks the closed file set ([spec](docs/specs/go-archive-gate-v0.md)) |
| Dogfooded | Substantive Corvint changes must collect context with Corvint and bind the diff to a CEM ([dogfood contract](docs/DOGFOOD.md)) |

## Status, stated plainly

> [!IMPORTANT]
> Corvint is an extraction alpha (`Corvint 0.5.0a2`). Availability and tested platform status come
> from the exact versioned release assets and their attached qualification evidence.

| Surface | Current state |
|---|---|
| Receipts, coverage, abstention | Experimental, the delivery state recorded in the [specification index](docs/specs/README.md); the shapes above are what the binary emits today. |
| `context`, `affected`, `prove`, `index` | Experimental under their specs: Task-Context Packet, Affected Plan, Falsifiable Packet, and Index Snapshot V0. |
| Retrieval quality | **Experimental and unqualified.** Bounded receipts around a named path or subject are the product today. Broad task-to-evidence retrieval has not passed held-out evaluation: the latest held-out attempt beat the exact-search baseline on top-5 (0.571 vs 0.343) and met the abstention and latency bars, but returned forbidden results on 7 of 36 must-exclude checks. Do not rely on ranking or abstention. |
| CEM `0.1` / `0.2` | Experimental: reference producer, verifier, schemas, and conformance vectors. Independent third-party interoperability is still unproven. |
| Does CEM help a reviewer? | **Unproven.** A five-pair pilot scored mean missed evidence of 0.90 for control and 0.86 with CEM. It is a pilot, not a held-out outcome study. |
| Performance | Unmeasured for the current Go-only revision. Earlier measurements compared against the retired Python runtime and do not qualify this one. |
| Host adapters, VS Code, MCP | Developer preview or experimental, reporting `FALLBACK`. |
| Tickets, work queue, dashboard, console, test providers, Pulse | Experimental under their specs; the console and dashboard present evidence and never hold authority over it. |
| Learned traces | Experimental and advisory; a learned-path change is admitted only through a pinned two-arm evaluation, and none is qualified for this release. |

These boundaries are backed by committed artifacts: the [release notes](docs/RELEASE-NOTES-alpha.md),
the [specification index](docs/specs/README.md), and [`benchmarks/`](benchmarks/).

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

```console
test "$(GOTOOLCHAIN=local go env GOVERSION)" = "go1.27.1"
GOTOOLCHAIN=local go test -count=1 -timeout 30m ./...
GOTOOLCHAIN=local go vet ./...
(cd interop/cem01-go && GOTOOLCHAIN=local go test -count=1 -timeout 30m ./... && GOTOOLCHAIN=local go vet ./...)
make gate
script/release-checklist      # read-only; seven rows, PASS/FAIL/NOT_RUN, exits 0 only when all pass
```

If `GOTOOLCHAIN=local go env GOVERSION` reports a different patch version than `go1.27.1` (for
example, a Homebrew `go` formula upgrade changed the linked toolchain), install the exact pinned
version alongside it rather than relinking Homebrew's default, then prepend its `bin` directory to
`PATH` for gate commands only, e.g. `PATH=/opt/homebrew/Cellar/go/1.27.1/bin:$PATH GOTOOLCHAIN=local make gate`
(Intel Homebrew: `/usr/local/Cellar/go/1.27.1/bin`).

Gemini/OpenCode adapter tests use their hosts' Node runtime. Corvint itself and its developer tools
do not require Python. See [native regression ownership](tests/README.md).

## License

Corvint is free software under the **GNU Affero General Public License v3.0 or later**. The portable
protocol descriptions, schemas, conformance material, examples, and interop implementations are
**Apache-2.0** instead, so anyone can implement the standard, including in proprietary software,
without the copyleft attaching. See [LICENSING.md](LICENSING.md) for the exact path boundary,
[LICENSE](LICENSE) for the AGPL text, [LICENSE-APACHE-2.0](LICENSE-APACHE-2.0) for the Apache text,
and [PROVENANCE.md](PROVENANCE.md).
