# Corvint Rebrand V0

Owner: Russell Lewis  
Date: 2026-09-15  
Requirement prefix: `CRB-V0`  
Intent status: accepted  
Delivery status: experimental foundation

Authoritative inputs: the owner's 2026-09-15 choice, “I don't care about dotcom use Corvint”; decision 0295; the accepted CEM canonical-binding contracts.

## Agent digest
- Claim: The prerelease product is named Corvint and its source module is `github.com/Beamfall/corvint`.
- Status: accepted/experimental foundation
- Exists: source, native archive, host/editor, Console/Tasks, MCP/provider, runtime environment, companion inventory and separately reviewed manual brand-asset slices.
- Blocked on: full assembled qualification, installed host state, public URLs and publication ownership; the binary-patch native refusal remains explicit.
- Read next: Current slices and compatibility; User and current state; Requirements.

## Current slices and compatibility

- Current slices: foundation, native archive identity with v2 emission and closed v1 legacy readers,
  source host bundles for Codex, Claude Code, Gemini CLI, and OpenCode, the VS Code editor plus its
  source activity icon, standalone Corvint Console/Tasks discovery and presentation, and standalone
  MCP/live-provider executable presentation with exact MCP client compatibility, runtime/helper
  environment names with retained storage compatibility, and the current closed companion bundle
  inventory (full assembled qualification remains pending). The full brand asset family is separately
  manually qualified; its binary-patch native refusal remains explicit.
- Preserved: frozen wire/profile identifiers, `.corvint/change.cem.json`, canonical `nextActions` executable literal `corvint`, historical evidence, and legal facts.
- Retained compatibility: contractual `.corvint/` and `.context-corvint/` storage, without automatic migration.
- Deferred: public URLs, installed host state, and publication ownership.

## User and current state

The user needs the unreleased product to acquire one coherent public identity before publication. Before this change the Go module coordinate, the native CLI banner, and the Makefile build name carried the previous product identity, and receipt and protocol contracts carried the `corvint-*` namespace; those namespace values are interoperability identities rather than branding.

## Requirements

- `CRB-V0-001`: The current product display name MUST be `Corvint`, with lowercase slug `corvint`.
- `CRB-V0-002`: The root and repository-owned nested Go modules, active Go self-imports, and operational self-module comparisons MUST use the planned local source coordinate `github.com/Beamfall/corvint`. The Git remote remains `Beamfall/corvint` until its separately executed rename.
- `CRB-V0-003`: The main native CLI version output MUST be `Corvint <version> (build <N>)` (amendment 2026-09-19, decision 0314, `PUB-V0-021`), the default local build output MUST be named `corvint`, and current dogfood coordinators MUST validate that exact branded version and a well-formed build number without weakening version equality. The source package MAY remain at `cmd/corvint` during the bounded foundation slice.
- `CRB-V0-004`: Existing public versioned `corvint-*` wire, schema, receipt, and profile identifiers MUST remain byte-compatible until a separately accepted protocol migration replaces them. A private derived index encoding MAY increment its existing schema version when the source-coordinate change invalidates cached package identities.
- `CRB-V0-005`: `.corvint/change.cem.json` MUST remain the canonical CEM path, and canonical CEM/OCM `nextActions` MUST retain the executable literal `corvint` required by `CEM-CB-006`, `CEM-CB-007`, `CEM-CB-009`, and `CEM-CB-016`.
- `CRB-V0-006`: Historical results, receipts, accepted decision facts, third-party notices, provenance, copyright, license text, and immutable Git identities MUST preserve their original names and bytes except for separately reviewed current prose around those facts.
- `CRB-V0-007`: State directories, environment variables, command source directories, plugins, editor IDs, package/publisher identities, current URLs, visual assets, and closed companion bundle manifests/readers MUST change only in later authorized migration slices with explicit compatibility and ownership evidence. Standalone builds from the retained source directories use `Corvint Console` / `corvint-console`, `Corvint Tasks` / `corvint-tasks`, and `corvint-dashboard-snapshot`; the closed release bundle follows CRB-V0-017.
- `CRB-V0-008`: This foundation MUST build offline with Go 1.27.1, print the Corvint version, expose usable help, complete a fixture query, and continue to verify an existing receipt where a practical retained fixture exists.
- `CRB-V0-009`: The source Codex and Claude Code plugin directories, manifests, marketplace
  entries, bundled skills, and hook paths MUST use `corvint` coherently; Gemini CLI's extension,
  command, skill, and hook paths MUST use `corvint` coherently. Shipped hooks MUST invoke the
  `corvint` executable, and no renamed manifest may point at a removed legacy path.
- `CRB-V0-010`: The private, unpublished OpenCode package coordinate MUST be
  `@corvint/opencode`. `CorvintPlugin`, `createCorvintRunner`, `corvintBinary`,
  `CORVINT_BIN`, and `CORVINT_OPENCODE_*` are its only exported and environment names; no
  compatibility alias exists (amendment 2026-09-17).
- `CRB-V0-011`: Every setting has exactly one name (amendment 2026-09-17). An explicitly empty
  executable selector MUST be refused before any child process launches. Explicit neutral options
  retain their existing precedence over ambient values. No read or host load may migrate installed
  settings.
- `CRB-V0-012`: Host package versions MUST advance with renamed shipped content, while existing
  `corvint-*` protocol, envelope, receipt, degradation/error, and `metadata.corvint` identities
  remain unchanged. Static source validation and fixture execution MUST NOT promote `FALLBACK`,
  fill a black-box-tested host version, or replace an existing `NOT_RUN` native conformance fact.
- `CRB-V0-013`: The VS Code source package MUST use private identity `corvint.corvint-vscode`,
  current `corvint.*` command, view, and configuration IDs, current executable names, and Corvint
  presentation. It MUST preserve frozen `corvint.*` MCP tool/profile bytes and the VSC-V0
  configuration trust, pinning, and native-argv behavior. Publisher
  ownership and installed-host qualification remain unverified.
- `CRB-V0-014`: The brand asset family MUST use the corvid silhouette selected by the owner on
  2026-09-19 ("I like the first one") and a custom lowercase `corvint` vector wordmark. Transparent
  dark-ink/light-background and white-ink/dark-background variants MUST share the same geometry;
  universal variants MUST use white artwork on a dark rounded tile. The README MUST select the
  matching light/dark lockup, and the VS Code activity icon MUST remain readable at 24 px and inherit
  the host color. SVG is the source; PNGs are rendered outputs. The artwork is manually qualified
  by SVG parsing, reference checks, PNG format/dimensions, and independent visual review. Native CEM
  does not support the PNG binary patch; this limitation MUST remain explicit.
- `CRB-V0-015`: The standalone Console MUST present as `Corvint Console`, build from retained source
  package `cmd/corvint-console` to `corvint-console`, and default to adjacent `corvint-tasks` and
  `corvint-dashboard-snapshot` before `PATH` names.
  Explicit `--tasks` is primary, absent `--tasks` MAY use `--atm`, equal explicit values MUST pass,
  and empty or conflicting explicit values MUST fail before a child starts. The snapshot roadmap
  command MUST apply the same explicit compatibility rule while continuing to require one of those
  flags and performing no discovery. Existing `corvint-dashboard-snapshot/0`, dashboard error, taskman
  envelope/store, and source-package identities MUST remain unchanged.
- `CRB-V0-016`: Standalone builds from the retained MCP and live-provider source packages MUST be
  named `corvint-mcp`, `corvint-docs-mcp`, `corvint-test-validity-mcp`,
  `corvint-js-test-provider`, and `corvint-go-test-provider`. MCP `serverInfo` MUST use the matching
  current Corvint name and description. The VS Code MCP client MUST accept exactly the complete
  current pair and reject unknown or changing pairs (amendment 2026-09-17). Existing
  versioned `corvint-*` profiles, `corvint.*` tools, producer/retention identities, and
  `--corvint-go-live-authority` MUST remain unchanged.

- `CRB-V0-017`: Current companion writers MUST emit manifest profile `corvint-companion-bundle/1`
  with exactly nine components: `corvint`, `corvint-console`, `corvint-dashboard-snapshot`,
  `corvint-mcp`, `corvint-docs-mcp`, `corvint-test-validity-mcp`, `corvint-js-test-provider`,
  `corvint-go-test-provider`, and `corvint-tasks`. Core module identity MUST be `corvint`;
  Tasks identity MUST be `corvint-tasks`, built from `cmd/corvint-tasks`. Core command source
  directories remain `cmd/corvint-*`. Source/archive/notice/binary and VSIX paths MUST use the
  corresponding closed current identities. An absent manifest profile MUST retain the exact
  legacy nine-component/five-artifact inventory; explicit null, empty, unknown, mixed identities,
  unknown fields and duplicate keys MUST be refused. Retained smoke row IDs, tool names and
  provider receipt/retention profiles MUST remain unchanged. Current VSIX members MUST include
  `dist/src/configuration.js` and `media/corvint.svg`; versioned compatibility MUST preserve
  independently checked source identities, exact members, modes, checksums and legal bytes.
  Profile `/1` has exactly six artifacts: the existing current VSIX and four host package trees,
  plus artifact `pi` of kind `host-package-tree` at `plugins/pi/`, sourced from `integrations/pi/`. Explicit `corvint-companion-bundle/0` MUST retain
  exactly its existing five-artifact current inventory; absence retains the legacy five-artifact
  inventory. Pi adds no executable component. Readers MUST select exact artifact members,
  modes, hashes, notices and source identities by profile and reject mixed inventories. The Pi
  package remains experimental FALLBACK until its own exact native qualification is complete.
  Final full bundle and installed qualification MUST bind the combined final editor/artwork source;
  focused implementation checks MUST NOT claim that qualification.

- `CRB-V0-018`: Runtime settings are read from exactly `CORVINT_CPUPROFILE`,
  `CORVINT_HARNESS_SHARED_OBSERVATION`, `CORVINT_QUERY_SHARED_OBSERVATION`,
  `CORVINT_SNAPSHOT_FORMAT`, `CORVINT_INDEX_SHARDS`, `CORVINT_CONTEXT_TERMS`,
  `CORVINT_CONTEXT_FRAME_RELATION`, `CORVINT_EXPERIMENTAL_COMPACTION_KERNEL`, and
  `CORVINT_BENCH_SNAPSHOT_TRACE`; no counterpart names exist (amendment 2026-09-17). An
  injected adapter environment MUST remain authoritative, including presence of empty values.
  Experimental paths retain their existing opt-in values, status, and defaults. The dogfood
  helpers and CEM example verifier MUST accept `CORVINT_BIN`; explicit neutral selections retain
  existing precedence. The CEM example defaults to `corvint`. Local completion MUST strip the
  `CORVINT_BIN` executable selector from helper child environments. Contract-owned `.corvint/`
  and `.context-corvint/` state, canonical CEM and nextActions, wire/profile names, source
  package paths, test-only controls, and historical experiment inputs remain unchanged. No read
  or host load may migrate installed or repository state. Runtime settings do not authorize a new
  storage root or promote an experimental format.

- `CRB-V0-019`: The current core companion writer MUST emit `corvint-companion-bundle/2` with
  exactly the existing nine Corvint binaries, their two complete verified source archives and
  legal notice groups, and five exact exported host trees: Codex, Claude Code, Gemini CLI,
  OpenCode and Pi. Every host artifact MUST remain labelled `FALLBACK`; exact runtime FULL
  qualification is separate evidence and MUST NOT be inferred from packaging or local completion.
  Profile `/2` MUST omit the VSIX artifact/member, `vsixToolchain` manifest field and assembled
  `VSIX-BUILD-ONLY-NPM.txt` notice. Explicit null or empty toolchain fields MUST also be refused.
  Corresponding source archives MUST remain complete: editor source and source-owned notices
  inside them MUST NOT be deleted. Absent-profile, `/0` and `/1` readers MUST retain their existing
  closed inventory and admission behavior. Unknown/null/empty profiles, duplicate/unknown fields,
  mixed identities or inventory, extra or missing members, and profile-removal downgrade attempts
  MUST refuse. Existing exact source, mode, digest, reproducibility, no-overwrite and atomic
  retention requirements remain. Building `/2` MUST NOT invoke npm, Node or the VSIX builder.

## Non-goals and baseline

The simpler baseline is source-coordinate, version-display, archive, source host-bundle, editor,
and editor activity-icon preparation. This slice does not claim the full rebrand complete, rename
the Git remote, install or publish packages, create redirects, rename general state, qualify an
installed editor host, or assert ownership of npm, editor, domain, or release coordinates.

## Trust, resources, and failures

The local product keeps its existing trust boundary: native Go, local Git evidence, no network dependency, account, daemon, mutable external database, or hosted service. Replacement is limited to active source/module identity. Any changed frozen receipt bytes, unresolved old active import, missing nested-module update, online dependency need, or failed focused proof blocks this slice. Unsupported/bootstrap observations remain `NOT_PRODUCED`; they are not converted into success.

## Acceptance, rollout, rollback, and maintenance

Acceptance requires Go 1.27.1, offline compilation of affected root and nested modules, focused
package tests, `corvint --version`, help, a fixture query, a practical retained receipt
verification, source plugin validation, host-adapter fixtures, and the repository's spec/decision
checks. Host bundle acceptance remains source-only and does not qualify native installed-host
behavior. Rollback reverts each atomic host bundle path and manifest set together; preserved
protocol and history require no rollback. Future rebrand changes must update this spec's
traceability and residue boundary.

Current release wrappers read `CORVINT_*` settings with the same suffixes as the runtime settings
and refuse an explicitly empty executable selector before effects. `corvint-companion-release`
uses `-source-root` (decision 0397 retired `-tasks-root`). Internal installed-harness environment names remain transport
compatibility identities.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| CRB-V0-001, CRB-V0-003 | `cmd/corvint` current help/error renderers and branded help/version tests; `Makefile`; `script/dogfood-{change,check,bind-range}.sh` | focused current CLI presentation, version, help, and coordinator regression proof |
| CRB-V0-002 | root and nested `go.mod`; active Go imports, self-module checks, and Python analyzer offline guard | offline list/build and old-coordinate residue check |
| CRB-V0-004, CRB-V0-005 | unchanged CEM/OCM wire contracts and `.corvint/change.cem.json` | focused existing CEM tests and literal residue inspection |
| CRB-V0-006, CRB-V0-007 | excluded path and identity classes | reviewed diff and residue classification |
| CRB-V0-003, CRB-V0-004 | `conformance/release-artifact-v0/`, `script/go-archive-gate` | host-only archive proof; pre-rename identity rejection tests; five-target production requirement retained |
| CRB-V0-008 | focused root/nested commands, branded public-help assertion, and fresh-process CLI fixture runs | retained local command output and focused package tests |
| CRB-V0-009 | `integrations/{codex,claude-code,gemini-cli}/` | plugin validator, path-resolution assertions, host-adapter fixtures |
| CRB-V0-010, CRB-V0-011 | `integrations/opencode/{package.json,src/index.js,src/runtime.js}` | single-name and empty-selector refusal fixture cases |
| CRB-V0-012 | host manifests, compatibility metadata, unchanged legacy protocol fields | package-version gate, residue review, unchanged FALLBACK/NOT_RUN assertions |
| CRB-V0-013 | `extensions/vscode`, `conformance/vscode-extension-v0` | native executable/MCP/config fixtures, extension check, package listing, conformance runner; installed host remains `NOT_RUN` |
| CRB-V0-014 | `assets/brand/corvint-{lockup,mark}-{light,dark,universal}.{svg,png}`, root `README.md`, and `extensions/vscode/media/corvint.svg` | SVG parse and reference checks, matching geometry across variants, PNG format/dimensions, README theme selection, and independent visual review; PNG binary patches remain unsupported by native CEM |
| CRB-V0-015 | `cmd/corvint-console`, `cmd/corvint-dashboard-snapshot`, `internal/console` | focused compatibility/default tests and the isolated standalone console proof |
| CRB-V0-018 | `internal/runtimeenv`, `cmd/corvint`, `internal/contextindex`, `internal/localcompletion`, dogfood helpers and CEM example | `TestResolveNamespace`, `TestRuntimeEnvironmentCurrentSnapshotSettings`, `TestDogfoodFinishRunsFromBinaryInForeignRepository`, and `script/runtime-environment_test.sh` |
| CRB-V0-016 | three retained MCP command packages, two retained live-provider command packages, and `extensions/vscode/src/mcp.ts` | isolated five-binary handshake proof, MCP conformance, provider regressions, and current/unknown/drift client fixtures |

## Unresolved decisions and promotion gate

Exact release repository rename, registry/package ownership, compatibility removal conditions,
final combined companion bundle and installed editor qualification, and public URLs remain pending. Promotion from
foundation to complete rebrand requires every later leaf, a path-classified residue report, the
full frozen gate, independent review, and explicit publication authorization.

Companion inventory evidence: `TestCurrentAndLegacyClosedBundleProfiles`, `TestCurrentBundleRejectsInvalidProfilesAndMixedIdentities`, and opt-in `TestCurrentExportedTasksAndVSIX` cover CRB-V0-017; final bundle/installed qualification is deferred to the combined source gate.

## Alias retirement amendment (2026-09-17)

Decision 0308 retires the compatibility-alias layer that CRB-V0-010, CRB-V0-011, CRB-V0-013,
CRB-V0-015, CRB-V0-016 and CRB-V0-018 admitted for the prerelease transition. Every runtime
setting, exported symbol, configuration key, executable name and MCP metadata pair has exactly one
name, so absent-only fallback, equal-alias acceptance and alias-conflict refusal no longer exist.
An explicitly empty executable selector is still refused before any child starts. The requirement
texts above are amended in place; the transition wording in decision 0295 is historical.

## Core companion profile amendment (2026-09-16)

The owner deferred VS Code support, upstream merge and installed editor qualification from the
core release. This amendment prospectively supersedes CRB-V0-017's current-writer selection only;
its historical profiles remain readable and all source/provenance/compatibility rules remain.

The `/2` manifest has exactly `profile`, `target`, `goVersion`, `gitVersion`, `notRunTargets`,
`components`, `artifacts`. Their existing value types remain unchanged. Its components are exactly
`corvint`, `corvint-console`, `corvint-dashboard-snapshot`, `corvint-mcp`, `corvint-docs-mcp`,
`corvint-test-validity-mcp`, `corvint-js-test-provider`, `corvint-go-test-provider`, `corvint-tasks`.
Its five artifacts are `codex`, `claude-code`, `gemini-cli`, `opencode`, `pi`, each
`kind=host-package-tree` with the existing exported package path, fields and source identity.
The one-binary native archive remains independent. Rollback withdraws the new writer/release
candidate; it never re-labels `/2` bytes as an old profile or reuses qualification across pins.

Acceptance remains pending: focused `TestCoreBundleProfileRoundTrip`,
`TestCoreBundleRejectsMixedInventory`, `TestCoreBundleRejectsEditorToolchain`,
`TestCoreBundleRejectsProfileDowngrade`, `TestCoreBundleBuildSkipsEditorTools` and retained
absent/zero/one fixtures; then exact frozen-source two-build/installed qualification. These names
are prospective test obligations, not claims that tests or qualifying receipts already exist.
