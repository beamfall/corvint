# Corvint extension ecosystem strategy

Date: 2026-08-23  
Status: evidence-backed planning document; not a compatibility or delivery claim  
Authoritative product inputs: `docs/PRODUCT.md`,
`docs/specs/agent-harness-integration-v0.md`

Official host/API references were checked on 2026-08-23. Links are placed beside the claims they
support; they establish host surfaces, not Corvint compatibility.

## Decision

Corvint should ship one local, versioned evidence engine and many deliberately thin delivery
surfaces. Close the experimental MCP read/query floor, complete the native VS Code surface, and put the
Change Frontier into GitHub pull requests next. Stabilize the existing native agent-harness
packages before adding more harnesses. Add another true editor extension only when its native API
can expose value that MCP and a command cannot.

An MCP server, CI wrapper, agent-harness package, and editor extension are different products:

- a **true editor extension** owns editor-native trust, commands, navigation, decorations,
  diagnostics, test presentation, cancellation, and lifecycle disposal;
- a **harness adapter** translates native agent lifecycle events into
  `corvint-harness-event/0` and renders the returned receipt;
- an **MCP bridge** exposes bounded Corvint tools through a common transport but does not observe the
  host lifecycle and is never `FULL` harness support by itself;
- a **PR-check adapter** invokes the same revision-pinned verifier in CI and projects its result
  into a forge's native check surface. It does not become an evidence authority or gain merge
  authority merely because a repository marks the check required.

## Verified delivery truth

The repository currently exposes revision-pinned `query`, `impact`, CEM, OCM, LRF, harness-event,
and explicit outcome commands. The Python shared lifecycle implementation and native packages for
Codex, Claude Code, Gemini CLI, and OpenCode exist, with common-event fixtures. Their published
status is `FALLBACK`; the complete install-to-session matrix and promotion evidence are `NOT_RUN`.
The repository records one real Codex package discovery/install/uninstall observation and five
startup receipts, but those calls are not a latency distribution and do not close `AHI-012`.

Corvint now contains an experimental dependency-free Go `corvint-mcp` server and a proposed exact MCP
`2026-07-28` contract. Its local Go 1.27 unit and compiled-process black-box tests passed, but the
official MCP conformance runner is `NOT_RUN`, no publishable/installable compatibility tuple exists,
and the inherited Git runner's post-start executable pinning remains a promotion blocker. This is
implementation evidence, not delivery or client interoperability. One real local compiled
client-to-compiled-server impact smoke returned `STALE_INDEX` with eight evidence rows; that single
smoke preserves its degraded state and does not promote a packaged VS Code, MCP interoperability,
or host tuple. No Pi or DeepSeek Harness package, JetBrains plugin,
Neovim plugin, Zed extension, Visual Studio VSIX, or GitHub/GitLab/Bitbucket check package is
present. VS Code V0 now has a proposed specification, experimental prototype package/source, and a
hostile-case corpus under `conformance/vscode-extension-v0/`; its owning specification marks
delivery `experimental`. The actual Extension Host matrix remains `NOT_RUN` until a real installed
extension is exercised in both trusted and untrusted profiles. Source presence, static validation,
an oracle driver, or documentation alone is not delivery evidence.

Corvint also lacks an accepted closing Frontier and harness-owned execution authority. Until those
contracts land, every stop/merge projection is advisory, must preserve
`frontier.state: UNAVAILABLE` where applicable, and must not block, continue, approve, merge, or
upgrade authority.

## Shared architecture and reuse boundary

```text
Git revision + worktree + project-owned authority
                     |
                     v
        Corvint local CLI / proof engine
        - repository snapshot and identity
        - query / impact / why / exact expansion
        - CEM / OCM / Frontier / test observations
        - bounds, redaction, receipts, abstention
                     |
        versioned, bounded JSON contracts
          /              |               \
         v               v                v
   MCP read tools   harness event/0   CLI verification reports
         |               |                |
    MCP clients     harness adapters    PR/CI adapters
                         |
              native editor projections
```

Core owns all repository discovery, immutable identities, evidence selection, authority ordering,
uncertainty, redaction, canonicalization, output bounds, receipt identity, and CEM/OCM/Frontier
semantics. Native integrations must not copy the graph, parse source to make their own findings,
reinterpret confidence, infer that evidence was observed, or treat presentation state as proof.

The only reusable native layer should be a small process-and-projection contract:

1. discover an explicitly configured local Corvint executable or a narrowly named candidate;
2. canonicalize and pin the executable path plus file identity and compatible protocol version;
3. execute that exact file without a shell or later `PATH` lookup;
4. pass only the minimum normalized input admitted by the owning protocol;
5. enforce cancellation, deadlines, stdout/stderr limits, strict JSON parsing, path containment,
   text sanitization, and resource disposal;
6. project validated result fields into host-native UI without adding authority; and
7. degrade visibly when trust, permission, compatibility, or evidence is missing.

Do not create a cross-editor UI framework. Share golden logical events, hostile output vectors,
schemas, and black-box expectations; keep the VS Code TypeScript, JetBrains Kotlin, Neovim Lua,
Zed Rust/Wasm, and Visual Studio .NET shells idiomatic and small. No integration may add a daemon,
database, webview, telemetry, source upload, silent binary install/update, or a second evidence
store. Host marketplace updates and Corvint binary discovery/update are separate lifecycles.

## Ranking method

The order below is a product sequence, not a claim about market share. `Coverage` is the fraction
of Corvint's context/edit/review loop the surface can carry. `Unique value` means value unavailable
through a manual Corvint command or MCP alone. `Reach` is a qualitative inference from how many host
surfaces the official mechanism can address; Corvint has no install telemetry or defensible user-count
dataset. `Burden/churn` and `authority risk` are costs, so lower is better. Ratings are
`very high`, `high`, `medium`, or `low` and deliberately avoid false numerical precision.

| Rank | Integration | Kind | Coverage | Unique value | Reach | Burden / API churn | Authority risk | Build decision |
|---:|---|---|---|---|---|---|---|---|
| 1 | MCP universal bridge | thin transport | very high breadth; low lifecycle depth | high | very high | medium | medium | Close the experimental stdio server's promotion blockers first; never label MCP-only support `FULL`. |
| 2 | VS Code | true editor extension | high | very high | very high | medium | medium | Complete V0 now: trust-gated pinned local execution, native views, diagnostics, decorations, and Test Explorer. |
| 3 | GitHub PR checks | thin CI/check adapter | high at review/merge | very high | very high | medium | high | Ship an advisory, SHA-pinned Action/check after the Frontier result contract is consumable; richer App delivery can follow without a hosted Corvint service. |
| 4 | Codex CLI/Desktop | harness adapter | high | very high | high | medium | high | Finish black-box conformance for the existing package before expanding it; keep Codex IDE a separate surface. |
| 5 | Claude Code | harness adapter | high | very high | high | medium | high | Finish the existing plugin matrix; retain explicit degraded behavior when MCP/Frontier authority is unavailable. |
| 6 | Gemini CLI | harness adapter | high | high | high | medium | high | Finish the existing extension matrix and environment-filtering cases before adding capabilities. |
| 7 | GitLab merge-request checks | thin CI/check adapter | high at review/merge | high | high | medium | high | Start with a portable CI job; do not require the Ultimate-only external-status service. |
| 8 | JetBrains IDEs | true editor extension | high | high | high | high | medium | Build after VS Code establishes the editor projection contract; use Trusted Projects and cancellable background execution. |
| 9 | OpenCode | harness adapter | high in principle | high | medium | very high | high | Maintain the existing preview, but pin versions: the official V2 plugin API is beta and hook failure can fail the intercepted operation. |
| 10 | Neovim | true editor plugin | medium | medium | medium | medium | high | Build a lazy Lua client after editor JSON stabilizes; no automatic execution before explicit user trust/configuration. |
| 11 | Pi | harness adapter | high | high | medium | high | very high | Add only after the four existing adapters pass; Pi packages execute with full system access, so pin releases and minimize fields. |
| 12 | Bitbucket PR checks | thin CI/check adapter | high at review/merge | high | medium | high | high | Ship a Pipelines/commit-status wrapper; defer Forge custom merge checks because they add hosted execution and admin enablement. |
| 13 | Zed | MCP/registry adapter first; native extension later | low natively today | medium | medium | high | medium | Publish through the MCP registry; wait for a stable editor UI API before claiming a native Corvint editor experience. |
| 14 | Visual Studio | true editor extension | high | medium | medium | very high | high | Defer until JetBrains; target the out-of-process model only after its required APIs leave preview or are isolated behind version gates. |
| 15 | DeepSeek Harness | harness adapter | high in principle | high | low/unknown | very high | very high | Track, do not promote: the host is a developer preview and its trajectory contains highly sensitive model/tool content Corvint must not ingest. |

### Why MCP is first but insufficient

MCP standardizes tools and structured messages across many agent clients. A local stdio server is
the right Corvint default because Corvint is local-first and does not need HTTP authorization, remote
hosting, or repository upload. MCP `2026-07-28` removes the legacy initialize/session handshake;
every request is self-describing and an optional `server/discover` call reports the implemented
surface ([MCP 2026-07-28 release](https://blog.modelcontextprotocol.io/posts/2026-07-28/)). Codex,
Claude Code, Gemini CLI, OpenCode, and Zed all document MCP connectivity, but connectivity proves
only that a caller can invoke a tool. It does not prove task-start injection, evidence capture,
stop behavior, compaction recovery, or compatibility for a host/version/OS tuple.

The frozen P0 MCP server exposes exactly bounded, read-only `corvint.query`, `corvint.impact`, and
`corvint.status` tools over stdio. It does not yet expose `why`, exact expansion, resources, prompts,
subscriptions, sampling, elicitation, roots, test execution, mutation, `corvint record`, check
approval, or host continuation. Each bridge result binds the repository revision and preserves
degradation. Hosts must still enforce their own approval policy. The VS Code client is stricter:
it pins a configured local `corvint-mcp`, rediscovers and relists on each explicit operation, calls
only query/impact, rejects legacy or interactive flows, and never falls back to the direct CLI.
Codex documents shared local MCP
configuration across Desktop, CLI, and IDE, including stdio and Streamable HTTP, while also making
clear that the IDE does not support Codex plugins
([Codex MCP](https://developers.openai.com/codex/extend/mcp),
[Codex plugins](https://developers.openai.com/codex/plugins)). That makes MCP the widest Codex
floor, not a replacement for lifecycle adapters.

### Why VS Code is the first native editor

VS Code has native APIs for the complete thin presentation layer: Workspace Trust/Restricted Mode,
commands, status bar, TreeViews, diagnostics, editor decorations, cancellation, and the Testing API.
The Testing API publishes discovered tests and results into Test Explorer rather than requiring a
custom UI ([Testing API](https://code.visualstudio.com/api/extension-guides/testing)); Workspace
Trust exists specifically to prevent unintended code execution on opening an untrusted folder
([Workspace Trust](https://code.visualstudio.com/api/extension-guides/workspace-trust)). TreeViews
and status bar items are standard workbench contributions
([extension capabilities](https://code.visualstudio.com/api/extension-capabilities/overview)).

That surface justifies a true extension: evidence/impact/why trees, revision-pinned diagnostics and
display-only decorations, and Corvint live-test observations rendered in Test Explorer. It does not
justify a webview, bundled database, language server, source upload, telemetry, or an editor-owned
test runner. VS Code checks for and installs enabled extension updates through its extension update
mechanism ([Extension Marketplace](https://code.visualstudio.com/docs/configure/extensions/extension-marketplace));
the extension only discovers and pins a separately installed Corvint binary and must never silently
download or replace it. Remote, virtual, and web workspaces remain unsupported or visibly degraded
until their execution and filesystem boundaries pass separate tests; VS Code documents that local
UI and remote workspace extension hosts have different process/filesystem placement
([remote extensions](https://code.visualstudio.com/api/advanced-topics/remote-extensions)).

### PR and merge surfaces

The review boundary is Corvint's most distinctive integration point because the Change Frontier,
CEM drift, intent obligations, and pinned test observations can be checked against the exact head
SHA. Start with repository-owned CI wrappers that invoke the local Corvint binary in the checked-out
workspace and preserve a bounded report artifact. This stays local to the CI job and needs no Corvint
account or hosted indexing service.

- **GitHub:** a reusable Action can report ordinary job status first. If line annotations and the
  Checks tab materially improve review, a GitHub App may create check runs and annotations; the
  Checks API is explicitly for GitHub Apps and requires checks permissions
  ([GitHub Checks API](https://docs.github.com/en/rest/checks),
  [CI checks with a GitHub App](https://docs.github.com/en/apps/creating-github-apps/writing-code-for-a-github-app/building-ci-checks-with-a-github-app)).
  The check must bind the exact commit and never infer merge success.
- **GitLab:** prefer a merge-request pipeline job, which is available across GitLab offerings
  ([GitLab pipelines](https://docs.gitlab.com/ci/pipelines/)). External status checks are an
  Ultimate-tier, webhook/service integration, require the current source `HEAD` SHA, and are
  non-blocking unless a project separately requires them
  ([GitLab status checks](https://docs.gitlab.com/user/project/merge_requests/status_checks/)).
  That is a later projection, not the baseline.
- **Bitbucket:** prefer a pull-request Pipeline plus a commit status
  ([pipeline start conditions](https://support.atlassian.com/bitbucket-cloud/docs/pipeline-start-conditions/),
  [commit statuses](https://developer.atlassian.com/cloud/bitbucket/rest/api-group-commit-statuses/)).
  Required custom merge checks are Forge modules and require workspace-admin enablement
  ([Bitbucket merge checks](https://developer.atlassian.com/platform/forge/manifest-reference/modules/bitbucket-merge-check/));
  that hosted shape conflicts with Corvint's current no-hosted-service scope and is deferred.

All three adapters consume the same machine report and differ only in checkout identity,
permissions, annotations, links, artifacts, and native status rendering. They must not fork CEM,
OCM, or Frontier semantics. Fork pull requests, missing secrets, stale head SHAs, shallow clones,
and absent base commits produce explicit gaps, never a green approximation.

### Agent-harness adapters

The six requested harnesses should share `corvint harness event` and the same logical-event golden;
host packages translate only supported events and render only validated receipts.

- **Codex:** plugins bundle skills, hooks, and MCP, but the IDE extension does not support plugins.
  Current hooks include session, prompt, tool, compaction, stop, and session-end points, with
  project-local hooks gated by trust review
  ([Codex hooks](https://developers.openai.com/codex/hooks),
  [Codex plugins](https://developers.openai.com/codex/plugins)). Keep CLI/Desktop and IDE support
  tuples separate and never read the unstable transcript path.
- **Claude Code:** a plugin can bundle skills, hooks, and MCP servers
  ([Claude plugins](https://code.claude.com/docs/en/plugins)). Its lifecycle hooks are rich, but an
  MCP hook can be unavailable at early session events and expected failures are non-blocking
  ([Claude hooks](https://code.claude.com/docs/en/hooks)). This makes a native package valuable but
  keeps versioned conformance mandatory.
- **Gemini CLI:** extensions package commands, skills, hooks, subagents, and MCP servers
  ([Gemini extensions](https://geminicli.com/docs/extensions/)). Sensitive environment variables
  are filtered unless explicitly declared by the extension
  ([extension reference](https://geminicli.com/docs/extensions/reference/)); Corvint should retain a
  smaller allowlist rather than request broad environment access.
- **OpenCode:** V2 offers session/tool hooks, events, tools, and MCP, but its plugin API is explicitly
  beta and a runtime-hook failure fails the intercepted operation
  ([OpenCode plugins](https://opencode.ai/v2/docs/build/plugins)). Keep beta context injection
  isolated, catch expected Corvint degradation, and do not promote an untested host version.
- **Pi:** extensions expose session, prompt, tool, compaction, and settled-agent events, but packages
  execute arbitrary code with full system access
  ([Pi extensions](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/extensions.md),
  [Pi packages](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md)).
  A Pi adapter can be thin and capable, but its install/trust and interruption tests are release
  blockers, not documentation chores.
- **DeepSeek Harness:** Cordis exposes plugin services/events and the host records an append-only
  trajectory containing prompts, reasoning, tool calls/results, and injections
  ([DeepSeek Harness](https://deepseek.com/harness/en/)). Corvint may append its own receipt handle
  but must not ingest or mirror the trajectory. Developer-preview churn and the privacy boundary
  put this adapter last.

### Remaining true editor extensions

- **JetBrains:** the platform provides Trusted Projects, tool windows, inspections, external
  process execution, progress, and cancellation
  ([Trusted Projects](https://plugins.jetbrains.com/docs/intellij/trusted-projects.html),
  [tool windows](https://plugins.jetbrains.com/docs/intellij/tool-windows.html),
  [execution contexts](https://plugins.jetbrains.com/docs/intellij/execution-contexts.html)). Build
  a Kotlin plugin after VS Code fixes the portable view model and hostile cases; do not start Corvint
  in safe mode.
- **Neovim:** Lua plugins, RPC/job control, diagnostics, extmarks, commands, and floating windows are
  native surfaces
  ([Lua plugins](https://neovim.io/doc/user/lua-plugin/),
  [diagnostics](https://neovim.io/doc/user/diagnostic/),
  [API/extmarks](https://neovim.io/doc/user/api/)). Neovim has no equivalent centralized
  Workspace Trust contract in these APIs, so Corvint execution must remain opt-in per configured
  workspace and every child must be cancellable and reaped.
- **Zed:** current extensions focus on languages, debuggers, themes, snippets, and MCP servers, not
  arbitrary editor panels and diagnostics
  ([Zed extensions](https://zed.dev/docs/extensions)). Process execution is capability-gated
  ([extension capabilities](https://zed.dev/docs/extensions/capabilities)), and Zed says its MCP
  server extension form is being deprecated in favor of the official registry
  ([MCP extensions](https://zed.dev/docs/extensions/mcp-extensions)). Use the MCP registry now;
  revisit a true extension only when stable native UI/navigation APIs exist.
- **Visual Studio:** VSSDK is powerful but in-process and complex; the newer
  `VisualStudio.Extensibility` model is out-of-process, while some relevant APIs remain preview
  ([extensibility models](https://learn.microsoft.com/en-us/visualstudio/extensibility/visualstudio.extensibility/extensibility-models?view=visualstudio),
  [Output window](https://learn.microsoft.com/en-us/visualstudio/extensibility/visualstudio.extensibility/output-window/output-window?view=visualstudio)).
  The platform supports VSIX distribution, Error List, editor change observation, and output/tool
  windows, but its Windows-only test matrix and split API models make it the highest-burden editor
  after Zed's capability gap
  ([shipping VSIX](https://learn.microsoft.com/en-us/visualstudio/extensibility/shipping-visual-studio-extensions?view=vs-2022),
  [editor API](https://learn.microsoft.com/en-us/visualstudio/extensibility/visualstudio.extensibility/editor/editor?view=visualstudio)).

## Build order and promotion gates

1. Freeze the shared read/query/MCP and editor-projection inputs without adding a second graph or
   authority model. Add canonical hostile outputs and receipt fixtures first.
2. Close the experimental local stdio MCP server's Git-executable pinning, publication, official
   conformance, and independent-client gaps. Do not promote any harness from an MCP smoke test.
3. Complete VS Code V0 and run unit/type/lint plus real Extension Host tests in trusted and
   untrusted profiles. Remote/web workspace support remains `NOT_RUN` until separately exercised.
4. Ship an advisory GitHub Action/check against an exact SHA. Promotion to a required merge gate
   waits for an accepted Frontier authority and a false-closure kill test.
5. Close install/session/uninstall, privacy, timeout, compaction, and recursion conformance for the
   existing Codex, Claude Code, Gemini CLI, and OpenCode packages. Promote exact tuples only.
6. Reuse the same CI report for GitLab, then Bitbucket. Add forge-specific richer status surfaces
   only if measured review value exceeds their permission and maintenance cost.
7. Build JetBrains from the stable editor contract, then Neovim. Add Pi after the four existing
   harnesses pass. Track Zed, Visual Studio, and DeepSeek API maturity before implementation.

For every surface publish `(host, surface, host version, adapter version, OS)`, protocol versions,
missing capabilities, permissions, last conformance result, and exact `PASS|FAIL|NOT_RUN` evidence.
One leaked prompt, transcript, environment value, raw tool body, or secret; one silent binary
replacement; one false `FULL`; one unauthorized block/merge/continuation; or one uncancelled child
is a release blocker. Marketplace presence, source code, static validation, and successful install
are not substitutes for the black-box matrix.

## Evidence gaps retained

- Quantitative reachable-user counts and install conversion by host: `NOT_OBSERVED`.
- Independent MCP producer/consumer interoperability for Corvint: `NOT_RUN`; local compiled-server
  vectors pass but do not fill this matrix cell.
- VS Code trusted/untrusted installed Extension Host conformance: `NOT_RUN`.
- VS Code remote, virtual, web, Codespaces, Windows, and Linux matrices: `NOT_RUN`.
- Complete install-to-session conformance for Codex, Claude Code, Gemini CLI, and OpenCode:
  `NOT_RUN`.
- Pi and DeepSeek native package tests: `NOT_RUN`; packages absent.
- GitHub, GitLab, and Bitbucket exact-SHA PR-check fixtures: `NOT_RUN`; adapters absent.
- JetBrains, Neovim, Zed, and Visual Studio native editor matrices: `NOT_RUN`; extensions absent.
- Accepted closing Frontier, merge authority, and false-closure promotion gate: unavailable.

These gaps constrain claims; they do not authorize substituting estimated reach, source inspection,
or a passing transport smoke test for observed compatibility.
