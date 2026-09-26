# Public Release V0

Owner: Russell Lewis  
Date: 2026-09-12  
Requirement prefix: `PUB-V0`  
Intent status: accepted owner scope; implementation details proposed  
Delivery status: not qualified  
Amendments: decision 0167 (build from the staged export; retained bundle archive) amends `PUB-V0-013..015`; decision 0314 adds `PUB-V0-021` (build number); issue 44 adds `PUB-V0-022..026` (closed qualified release candidate); decision 0327 selected `v0.5.0a1`; decision 0328 selected `v0.5.0a2` with the existing unsigned prerelease/no-promotion boundaries and the issue #49 Playwright regression; decision 0329 selects `v0.5.0a3` for the integrated issue #53–#57 rerelease with the same boundaries; decision 0373 adds the "1.0 Core scope amendment" section (owner answers of 2026-09-23); decision 0375 amends `PUB-V0-021` (the build number is provenance, not an ordering or identity key, and is monotonic only along `origin/main`'s first-parent chain since decision 0331).; decision 0384 (V1-0018) widens the `PUB-V0-023` candidate version grammar to admit `MAJOR.MINOR.PATCH-rc.N` and `MAJOR.MINOR.PATCH` beside the retained alpha form.

## Agent digest
- Claim: A public alpha ships the Go CLI, MCP docs, agent/editor unit and E2E test tracking, and an optional dashboard and task manager with a roadmap.
- Status: accepted owner scope; implementation details proposed; not qualified.
- Exists: native CLI archives and experimental console/ticket store; the integrated release is not yet qualified.
- Blocked on: integrated roadmap delivery, optional bundle, automatic docs and test qualification, installed-path checks and publication approval.
- Read next: Human intent; Requirements; Acceptance and rollback.

## 0.6 local-workflow scope

Decision 0332 accepts `daily-change-evidence-workflow-v0.md` for the 0.6 verified-workflow
milestone. For that milestone only, it supersedes conflicting prerequisites that require optional
companions or formal FULL host authority before Core qualification. Local Codex and Claude use
is qualified separately on exact versions. This does not promote any host, waive evidence gates,
or alter historical alpha artifacts. `PUB-V0-001` selects the current candidate version;
0.6 readiness has not been established.

For this Core-only scope, `PUB-V0-022..026` retain their applicable safeguards without
making the optional combined alpha bundle a prerequisite. The existing native archive-gate
archive, reproducibility report and `SHA256SUMS` are the Core artifact inputs; the
candidate packet must retain a closed inventory, source archive and applicable legal
provenance, exact version/build/commit/tree/Go toolchain/target identity, independently
verified reproducibility and checksums, and installed `corvint --version` evidence.
`PUB-V0-022`'s companion input and `PUB-V0-023..024`'s companion identities and rows
apply only to the separate combined alpha profile. Core still requires its own native
Darwin/Linux, full, artifact, installed-host and sealed workflow gates, including all
six evidence classes. `PUB-V0-025..026` retain immutable version/platform candidate
storage, refusal to replace an existing path, and fail-closed verification. Operator-
approved local PATH activation is a separate selection of an installed binary; it
does not replace or mutate retained candidate artifacts. The candidate reader and
installer also admit the separate Core-only profile below (V1-0125); the combined alpha
profile is unchanged.

Freeze product candidate `T` before judged runs: its version/build, commit/tree,
binary, source archive and proof maps identify the exact tested product. Later
evidence-only publication snapshot `E` contains actual reports, receipts and ledger
changes bound to `T`; it does not rebuild or redefine `T`. Give `E` its own scoped
CEM/OCM, integrity and documentation checks, and independent semantic review. Those
checks establish `E`'s publication integrity, not that an `E`-built binary passed
`T`'s exact-target gates. A change to runtime behavior, tests or selected checks,
fixtures, an owning normative requirement, or `T`'s proof maps requires a new candidate
and invalidates affected evidence. A Beamfall receipt keeps the actual Beamfall
`repositoryRevision` and separately binds the evaluated Corvint `T` artifact hashes;
review verifies that join. `T`'s archived `UNPROVEN` ledger claims remain literal.
Only an owner-accepted `E` packet may qualify explicitly named `T`; it cannot silently
rewrite `T`'s archived claims. Exact-packet owner approval remains required before
promotion or publication. This scope remains `NOT_QUALIFIED`.

## 1.0 Core scope amendment (decision 0373, 2026-09-23)

Decision 0373 accepts [`corvint-1.0-product-and-release-v1.md`](corvint-1.0-product-and-release-v1.md)
(`PRS-V1`, ticket V1-0001) and amends this spec prospectively for the 1.0 Core candidate. No
`PUB-V0` requirement text changes, historical evidence keeps its meaning, and nothing here
qualifies or promotes anything.

- The 0.6 local-workflow scope above extends to the 1.0 Core candidate: it supersedes conflicting
  prerequisites that require optional companions or formal FULL host authority before Core
  qualification.
- The 1.0 Core candidate additionally requires the three Core jobs on one owner-selected untouched
  public repository with cases frozen before execution (V1-0019, `PRS-V1-008`).
- The 2026-09-16 clause "still require exact runtime FULL/authority evidence" binds only a
  companion host profile, not the Core release. Installed Codex CLI and Claude Code local use are
  Core host rows at FALLBACK on exact versions (`PRS-V1-006`); tuples whose host API cannot supply
  authority stay FALLBACK or UNSUPPORTED and do not block Core.
- `PUB-V0-004`, `-005`, `-006`, `-009`, `-010`, `-020`, the companion input of `PUB-V0-022` and the
  2026-09-16 clause "missing core installed ... evidence blocks release" bind only the companion
  profile (`PRS-V1-009`). Core installed qualification is the native archive lifecycle of
  `stable-operations-v0.md` plus the three Core jobs on the exact installed bytes. The candidate
  reader and installer admit a Core-only packet (V1-0125, next section), and the assembler produces
  one without companion input (V1-0229).
- Corvint 1.0 makes no interoperability claim for CEM, OCM or frontier (`PRS-V1-007`); V1-0014 is
  post-1.0 and `make interop-gate` with the canonical vectors remains the Core check.

## Core-only candidate reader and installer (V1-0125)

Under `PRS-V1-005`, `internal/releasecandidate` admits a second closed manifest profile,
`corvint-core-release-candidate/0`, beside the unchanged combined
`corvint-qualified-release-candidate/0`. The profile selects a closed inventory; nothing else
changes the reader's fail-closed checks.

- `MANIFEST.json` keeps the same closed fields. `sources` is exactly `[corvint]`. The closed role set
  is four `core-archive`, one each of `core-gate-checksums`, `core-gate-report`, `corvint-source`,
  `qualification-receipt` and `release-notes`. No companion or Corvint Tasks role is admitted.
  Each single-file role is bound to the path the assembler writes (`evidence/core-SHA256SUMS`,
  `evidence/core-verification-report.json`, `source/corvint-src.tar.gz`, `QUALIFICATION.json`,
  `README.md`); a role on any other path is refused.
- Every Core check of the combined profile still applies: the `SHA256SUMS` and asset inventories
  are closed and digest/size exact, the core gate report is an exact PASS reproducibility report
  bound to the manifest commit/tree/toolchain, every listed core archive matches both the gate
  checksums and the report, and the exact host binary passes the isolated `--version` probe. The
  `PUB-V0-023` enumeration bounds are unchanged.
- `QUALIFICATION.json` keeps `corvint-release-qualification/0` and its 28 rows. `core-archive` is
  PASS on all four platforms and every other row is `NOT_RUN`. Every `companion-bundle` row carries
  the evidence text `companion not present`. The reader refuses any other status, so an absent
  companion is never reported or treated as verified.
- `InstallCore` is unchanged: it installs the host core archive only after that verification.
- Assembly (V1-0229): `corvint-release-candidate` without `-companion-dir` writes this profile to
  `corvint-v<version>-core`. It takes no companion, Tasks or npm input. It exports the clean
  `-source-root` HEAD with the companion bundle's verified reader and deterministic source archive
  (`companionrelease.CoreSourceArchive`), refuses unless that commit and tree equal the core gate
  report's, derives the build number from the source root, and retains nothing until the staged
  candidate passes the reader above, including the exact host `--version` probe. With
  `-companion-dir` the combined profile and every companion check are unchanged, and
  `corvint-companion-release` takes only `-source-root` (decision 0397 retired `-tasks-root`).
- Limits: the reader binds the Core-only `source/corvint-src.tar.gz` by digest only; the commit/tree
  binding is an assembler check. Legal provenance and installed-workflow evidence are not carried.
  The version token follows `PUB-V0-023`: since decision 0384 (V1-0018) `1.0.0-rc.N` and `1.0.0`
  assemble and install beside the retained alpha form.

## Human intent

On 2026-09-11 the owner requested work until Corvint is ready for a public GitHub release, then
explicitly selected “Include all three before release” for the dashboard, task manager and
Wallaby.js support. The owner previously chose the Go release path after Rust showed no gain.
The intended first cut is an alpha prerelease, not a stable, FULL or workflow-promotion claim.
The default CLI remains the single native Go product. The console is a separate optional bundle
under decision 0081. Task management means the delivered ticket store and board; autonomous
agent dispatch is not implemented or represented as available. The owner clarified the test
goal as an agent and VS Code extension being able to assess test validity, then explicitly
requested automatic tracking including E2E, automatic docs with MCP, and a roadmap in task
manager. This is interpreted as a Wallaby-style loop, not a commercial Wallaby dependency.
The proposed delivery sequence and acceptance criteria are in
[the integrated product roadmap](../plans/integrated-product-roadmap-2026-09-12.md).

## Requirements

- `PUB-V0-001`: The current version tuple MUST move together to `0.8.1`, including native
  version output, archive smoke expectations, VS Code exact admission and live fixtures.
  Historical benchmark and release evidence MUST retain its original version identities.
- `PUB-V0-002`: The existing native CLI archive gate MUST remain independent. An optional
  companion bundle MUST contain separately built `corvint-console`, `corvint-dashboard-snapshot`
  and `corvint-tasks` binaries plus the separately installable workflow binaries `corvint-mcp`,
  `corvint-docs-mcp`, `corvint-test-validity-mcp`, `corvint-js-test-provider`, and
  `corvint-go-test-provider`, pinned to exact clean Corvint and task-manager commits and trees. The
  default Corvint install remains the independent one-binary archive; no companion becomes a
  default runtime.
- `PUB-V0-003`: Companion build output MUST be outside the source checkouts, include exact
  toolchain/component identities, applicable license and provenance files, archive and binary
  SHA-256 checksums and corresponding source. Two independent builds and archive assemblies
  MUST agree before retaining a candidate. One shared Corvint source archive and notice group MUST
  cover its eight binaries and one task-manager source archive and notice group MUST cover `corvint-tasks`;
  every component manifest reference MUST verify against those exact members. The bundle MUST also
  carry a twice-built, closed-member `corvint-vscode-0.1.0.vsix` and exact exported regular-file
  trees for the Codex, Claude Code, Gemini CLI, and OpenCode host packages, all labelled `FALLBACK`.
  A failed gate MUST NOT label an artifact qualified.
  The preceding `FALLBACK` inventory labels are historical/unqualified and do not satisfy or
  substitute for the current exact-tuple gate.
  The 2026-09-15 owner amendment permits stock OpenCode 1.18.31 only as a `FALLBACK` exception;
  it is not `FULL` and requires no host qualification claim. Every other claimed host/surface
  remains subject to formal exact-tuple `FULL` qualification. This prospective release scope does
  not alter retained historical all-`FULL` goals or their evidence.
- `PUB-V0-004`: The first companion target is macOS arm64. Qualification MUST exercise the
  real extracted binaries: explicit test-store initialization, board, create, detail, edit,
  requirement/code links, evidence dashboard and shutdown. Unsupported administrative/runtime
  verbs MUST remain visibly disabled. Browser and platform claims MUST name actual tested versions.
- `PUB-V0-005`: Every console mutation and external command MUST meet the browser-origin,
  bounded-output and process-lifetime requirements of `LAC-V0-021..024`. Installed-path tests
  MUST verify refusal before effects and no descendants after interruption.
- `PUB-V0-006`: Live test support MUST automatically track supported unit and E2E execution
  and expose matching evidence to the agent and VS Code. Association, executable hygiene,
  freshness, execution result and measured strength MUST remain distinct; passing alone MUST
  NOT imply test adequacy. Unavailable providers, missing authorization, unsupported scope and
  stale or unmeasured test state MUST remain explicit; an example or mock alone is not qualification.
- `PUB-V0-007`: Release notes MUST identify experimental features, qualified platforms,
  default CEM trust in the local Git executable/object database, unmeasured performance and
  unavailable hosted CI. Rust and unqualified Windows artifacts MUST NOT enter the release set.
- `PUB-V0-008`: Final source, CEM and OCM MUST be frozen before canonical gates and exact-target
  artifact qualification. Tagging, pushing, repository visibility, signing and publication remain
  separately approved outward actions; local readiness MUST NOT claim any of them happened.
- `PUB-V0-009`: Automatic documentation MUST maintain a useful source-bound human page on
  eligible changes in an explicitly enabled local session and expose draft/consume through MCP.
  Generated content MUST preserve human authority, citations, unknowns and edit conflicts.
  A read-only draft tool alone MUST NOT be represented as automatic documentation maintenance.
- `PUB-V0-010`: Task manager MUST expose a roadmap of its existing tickets with milestones,
  dependencies, acceptance criteria and visible evidence status. Planning-ticket seeding MUST
  preserve source identity and avoid duplicates without requiring autonomous execution. Full
  import remains under TCP-06; its authority-switch procedure MUST govern any later transition
  from human-owned Markdown to a generated ticket view. A planning import MUST NOT enable dispatch.

- `PUB-V0-011`: `cmd/corvint-companion-release` (package `internal/companionrelease`) MUST implement
  the `PUB-V0-002..004` companion bundle as one command taking one explicit clean checkout root
  (source module `github.com/Beamfall/corvint`, which since decision 0397 also holds the
  `cmd/corvint-tasks` companion) and a single target. It MUST refuse any target other than
  `darwin/arm64` and MUST report `darwin/amd64`, `linux/amd64`, `linux/arm64` and `windows/amd64`
  as `NOT_RUN` in every emitted report. It MUST refuse to start unless the checkout root passes a
  `git status --porcelain` clean-tree check, and MUST refuse an output location nested inside
  the checkout root. Output nesting MUST be judged by directory identity along the
  symlink-resolved output path's ancestry (a not-yet-created output suffix starts the walk at its
  nearest existing ancestor), so a symlink, case or Unicode alias of a root is refused, and a bundle
  name that is not exactly one path element MUST be refused. Before it creates or writes anything
  under it, it MUST refuse a scratch directory that is or lies inside the checkout root, judged
  by directory identity along the symlink-resolved scratch path's ancestry (a not-yet-created
  suffix starts the walk at its nearest existing ancestor), and MUST refuse an existing scratch
  directory that the checkout root is or lies inside, judged by directory identity along the
  symlink-resolved root's ancestry. A directory it creates or reuses directly under the validated
  scratch MUST be refused when an existing entry at that path is not a real directory (a symlink
  or a file), so a planted link cannot redirect a write outside the judged scratch path.
  The wrapper MUST admit a Git linked worktree whose `.git` is a valid file as the checkout root; it
  MUST NOT require `.git` to be a directory. It takes no separate Tasks checkout (decision 0397).
- `PUB-V0-012`: Source used for the bundle's source archives MUST be read twice by independent
  code paths — a `git ls-tree -rz` entry list plus two independently ordered and independently
  parsed `git cat-file --batch` passes — and the two passes MUST agree byte-for-byte before either
  is trusted. Each blob's bytes MUST be re-verified against its own Git object id. The read MUST
  refuse symlinks, submodules, `.git`-path entries (a `.git` segment in any letter case), path
  traversal and duplicate paths, and MUST refuse rather than truncate past 10,000 entries or 128 MiB
  of total exported bytes. The read MUST also refuse an entry set containing two paths equal under
  Unicode simple case folding (`strings.EqualFold` on the full path), or a file path that
  case-folds equal to a directory prefix of another entry's path, before any byte is written to
  disk; the same case-fold refusal MUST run again wherever an assembled archive's member
  inventory is independently re-verified.
- `PUB-V0-013`: Each of the nine bundled binaries (`corvint`, `corvint-console`,
  `corvint-dashboard-snapshot`, `corvint-mcp`, `corvint-docs-mcp`, `corvint-test-validity-mcp`,
  `corvint-js-test-provider`, `corvint-go-test-provider`, `corvint-tasks`) MUST be built from its module's `PUB-V0-012` exported tree,
  written to a fresh scratch directory and re-verified there (exactly the exported file set, each
  file matching its Git object id), never from the live checkout, so an ignored file or a checkout
  change after the clean-tree check cannot reach a shipped binary. Each MUST be built twice,
  independently, with cold, separate build caches, `-trimpath -buildvcs=false`, `CGO_ENABLED=0`, `GOWORK=off`, closed `GOFLAGS`, and
  the exact Go toolchain identity (`go env GOVERSION`) pinned by `requiredGoVersion`; the two
  builds MUST be byte-identical, and the retained binary's own embedded `debug/buildinfo` MUST be
  checked against the pinned Go version and requested `GOOS`/`GOARCH` before it is accepted.
  The VSIX MUST be compiled twice from separate materializations of that same Corvint export. Each
  build MUST install the exact lockfile through `npm ci --ignore-scripts --offline` from a separate
  private clone of the caller-supplied populated cache, verify Node 22.23.2, npm 10.9.8,
  TypeScript 5.9.3 and vsce 3.9.2, and compare the two independently decoded closed member sets.
  The manifest MUST record those versions, executable digests and the exported lockfile digest. A
  separate notice MUST inventory lock-pinned build dependency versions, licenses and integrity
  values while stating that `node_modules` is unshipped and the VSIX has zero runtime dependencies.
  Because vsce writes variable ZIP metadata, the accepted VSIX MUST be a deterministic ZIP rebuilt
  from each agreeing member set; the two canonical bytes MUST agree and be decoded again.
- `PUB-V0-014`: Each component's source archive and the assembled bundle archive MUST each be
  built twice and compared byte-for-byte before either is trusted, and MUST additionally be
  verified by an independent decode pass (a fresh `gzip`/`tar` reader) that checks the exact
  member inventory, modes, contents, the absence of decompressed bytes after the tar terminator,
  and the absence of trailing bytes (including a further gzip member) after the archive's single
  gzip member. The bundle MUST carry, per component, the source commit and tree, the source
  archive's SHA-256, the binary's SHA-256, applicable license/provenance/third-party notices, a
  machine-readable component manifest, and a `SHA256SUMS` file covering every shipped file. The
  bundle's members MUST be rendered without wall-clock input, so the two assemblies can agree. The
  retained bundle is `<bundle-name>/<bundle-name>.tar.gz`, the verified archive, plus
  `<bundle-name>.tar.gz.sha256` holding that archive's SHA-256.
  The four host-package trees MUST be copied only from the exact exported prefixes
  `integrations/codex/`, `integrations/claude-code/`, `integrations/gemini-cli/`, and
  `integrations/opencode/`, with bounded inventories and no symlink, special, unsafe, duplicate, or
  case-fold-colliding path. Their recorded tree digests and every VSIX digest/path/source reference
  MUST verify against the outer archive before its manifest is accepted. OpenCode's external
  dependency and host are not bundled; no raw package is a transformed or qualified marketplace.
- `PUB-V0-015`: An installed smoke test MUST run against the exact extracted bundle bytes before
  the bundle is atomically retained and before any report calls the build qualified: `corvint-tasks`
  version/help/init plus a create-then-refine ticket mutation against a real `.taskman` store, the
  console's board and ticket-detail pages over real loopback HTTP, `corvint-dashboard-snapshot`
  against that store (a Git repository with one commit), two console mutations that MUST be
  refused with 403 for the named reason while every byte under `.taskman` stays unchanged (the
  board's own create form, token included, sent from a foreign Origin; and the same form sent
  same-origin without its token), the detail page's per-ticket controls each rendered, with every
  disabled one carrying its reason, the detail page's own title/body form applied through `/mutate`
  and visible on re-read, the evidence page compiled from the real snapshot binary, the
  requirement/code link pages over a fixture spec committed in that repository (every link pinned
  to the fixture commit and resolving to a rendered target in both directions, and an uncited
  requirement and an absent cited path each rendering a gap with no link), and console
  shutdown with a verified absence of surviving descendant processes. A failed build or failed smoke step MUST retain no output and MUST NOT produce a
  qualified report. Retention MUST be atomic (stage then rename) into a location the command has
  not previously written, and MUST NOT overwrite an existing retained bundle. Browser and UI
  qualification are explicitly out of scope for this command and MUST be named as such in every
  report it emits.
  The extracted workflow smoke MUST additionally check exact versions for the three MCP binaries;
  call stateless `server/discover` with exact 2026-07-28 metadata and list the exact tool catalogues; call `corvint.status`; round-trip an
  `corvint.docs_draft` through `corvint.docs_consume` using its exact returned bytes; and record the
  test-validity catalogue plus the JS and Go provider's existing typed help/usage admissions.
  MCP responses follow `MCPV0-002`'s bounded UTF-8 JSON-object/LF framing and preserve the request
  ID's scalar type; envelope member order and JSON string escaping are not canonicalization gates.
  Actual retained test-validity discovery, JS unit/E2E execution and the Go foreground session stay
  in `PUB-V0-016`'s installed VSIX qualification. Each smoke row MUST bind the assembled bundle
  digest, component digest, source commit/tree and extracted invoked path; that record is retained
  outside the archive it hashes.

- `PUB-V0-016`: The interactive alpha qualification MUST freeze fixture source, dependencies,
runner and editor identities before execution. It MUST exercise automatic unit and E2E
save/fail/fix feedback through an isolated real VS Code host, using Vitest 5.0.0 and Playwright
1.63.0, and the supported Go foreground session on macOS arm64. Completed retained results MUST
be discovered through the actual test-validity MCP and compared with editor evidence. Cancellation
and process cleanup MUST be qualified separately; unretained cancellation is not fabricated for
MCP. Missing real tooling is blocking NOT_RUN. Go preview remains non-policy evidence; this
narrow qualification does not promote LPCV/FULL, remote hosts or test adequacy.
The 2026-09-15 owner amendment excludes human studies from this release qualification. Its
automatic results do not claim measured human usability or comparative speed improvement; the
required correctness, host/editor conformance, performance, memory, cleanup, and security gates
remain unchanged.
The final retained qualification MUST re-verify the unchanged outer archive, checksum and smoke
sidecars before and after execution; materialize its helpers and fixtures only from the retained
Corvint source archive; run with private offline dependency state; and retain its PASS result only
after the real editor/browser, interruption, automatic documentation/MCP and idempotent planning
paths have completed and every observed descendant is absent. Its checker MUST be built by exact
Go 1.27.1 from the clean frozen source revision, refuse source/input/output overlaps before writes,
revalidate pinned Node, npm and Python bytes around child execution, and retain the bounded original
automatic-documentation watch/discovery/list/draft/consume JSON evidence without private paths.
The result binds the archive SHA-256 and frozen Corvint commit/tree. Existing output is never overwritten.

- `PUB-V0-017`: First-use guidance MUST provide the experimental `features`, `overview` and
  `review --base FULL_COMMIT_ID` routes governed by `RGV-V0`. Inferred candidates, source omissions,
  clean-tree admission and incomplete branch overlap MUST stay explicit. Guidance MUST NOT accept
  human intent, execute test advice or claim comparative retrieval superiority.
- `PUB-V0-018`: PR test selection MUST execute only the supported, independently qualified Go
  package plan from trusted immutable tools. Missing or invalid qualification and incomplete or
  unsupported evidence MUST visibly select the full suite. The exact race profile and 200 complete
  frozen historical rows required by AFP govern promotion; full main/release and mandatory static,
  build and interoperability checks remain required. Hosted execution cannot be claimed from local
  macOS observations; JS/E2E execution remains full until its selector is separately qualified.
- `PUB-V0-019`: The shipped witness CLI and release-publication receipt reader MUST ignore Git graft
  ancestry, proving genuine ancestors still qualify and invented ancestors do not. Real repository
  and ambient-graft fixtures MUST retain the original Git/fixture bytes. This repairs immutable
  input interpretation without widening authority or changing the CEM trust boundary.

- `PUB-V0-020`: Core installed qualification for companion profile `/2` MUST freeze fixture
  source/dependencies, archive and source commit/tree, exact executable/runtime identities and
  actual platform/browser versions before execution. It MUST exercise both JS unit/E2E and
  admitted Go foreground sequences across the fixtures, with automatic supported source-save/change
  tracking in an explicitly enabled agent/provider session. A saved fixture change triggers its
  appropriate provider execution,
  failed results are retained and exposed to the agent/MCP, and a saved correction triggers
  replacement results with matching freshness. Each transition MUST be discovered through actual
  test-validity MCP and matched to the exact retained provider result. Manually invoking isolated
  provider commands or replaying prebuilt rows MUST NOT substitute for this automatic path. Association, hygiene, freshness, result and measured strength remain
  distinct; missing authorization/providers and stale/unsupported/unmeasured state remain explicit.
  It MUST exercise enabled automatic documentation watch/apply on a committed eligible change,
  produce a useful bounded page, preserve human authority and edit conflicts, and expose actual
  draft/consume evidence through MCP. It MUST exercise installed Tasks roadmap, dependencies,
  acceptance/evidence state, idempotent planning seed and no-dispatch. It MUST exercise the loopback
  console in a named real browser, with Origin/Host/session refusal before effects and verified
  descendant cleanup on interruption. It MUST verify unchanged archive/checksum/smoke sidecars
  before and after execution, use helpers/fixtures from retained source and private offline
  dependencies, revalidate pinned runtime bytes around children, enforce bounded output and
  exclusive atomic result retention, and refuse overlapping or pre-existing output paths before
  effects. Missing actual tooling/admission is blocking NOT_PRODUCED, never synthetic PASS.
  No editor invocation or result is required. Core evidence does not promote host FULL, Frontier
  authority or test adequacy. The checker MUST be built by exact Go 1.27.1 from the clean frozen
  source revision. Every required row must bind its original bounded evidence and exact fixture,
  archive, source and tool identities; unknowns retain their labels.
- `PUB-V0-021`: Every new Corvint version created on main MUST carry a new build number (owner
  instruction 2026-09-19, decision 0314). The build number is the first-parent commit count of the
  built commit, stamped with `-ldflags "-X main.build=N"` by `make build`, the loose archive gate and
  the archive build, so each merge to main raises it by at least one without a hand-edited file.
  `corvint --version` prints `Corvint <VERSION> (build N)`; an unstamped `go build` reports build
  `0`. The release smoke MUST require the exact stamped number, and the VS Code version probe and
  the dogfood coordinators MUST require the `(build N)` suffix while keeping `VERSION` as the pin.
  The build number is monotonic only along `origin/main`'s first-parent chain, and only since
  decision 0331 restarted that chain's count; it is neither an ordering key nor an identity key
  (decision 0375). `VERSION` orders releases; the released commit plus the installed executable
  digest identify a build. No consumer MAY compare build numbers across releases to order or
  authenticate them; every existing consumer instead checks the stamped number against an exact
  expected value or the `(build N)` shape. A release artifact MUST be built from a commit that is on
  `origin/main`'s first-parent chain at release time, so its stamped number matches what a fresh
  clone of `origin/main` reproduces.
- `PUB-V0-022`: A versioned release candidate MUST be assembled only from the closed seven-file
  core archive-gate output and the closed three-file companion retained output. Its verifier MUST
  require PASS core reproducibility evidence, exact archive checksums, a passing companion retained
  verifier, one matching Corvint commit/tree/toolchain across both inputs, and the exact Corvint and
  Corvint Tasks source archives. Windows MUST remain outside the candidate. A mismatch MUST retain
  no candidate.
- `PUB-V0-023`: The candidate MUST carry a closed machine-readable manifest with the exact version,
  build number, installed `corvint --version` output, Go/Git identities, Corvint and Corvint Tasks
  commits/trees, and every non-manifest asset's path, role, size and SHA-256. The version, the
  assembler `-version` input and the installed store component MUST match exactly one of
  `MAJOR.MINOR.PATCHaN` (retained alpha candidates), `MAJOR.MINOR.PATCH-rc.N` or `MAJOR.MINOR.PATCH`,
  where each numeric part, and the alpha `N`, is a non-negative decimal integer without leading
  zeros and the `-rc.N` `N` is at least 1; any other form, a `v` prefix or `+build` metadata MUST be
  refused (decision 0384, V1-0018).
  A top-level `SHA256SUMS` MUST cover every retained file except itself, including the manifest, qualification
  receipt, core and companion receipts, source archives and release notes. Candidate verification
  MUST bound directory enumeration before materialization: at most 15 regular files, five real
  directories including the root, no directories below those root children, 512 MiB per file and
  1 GiB aggregate retained input. Reads MUST check cancellation and the remaining byte allowance.
  These are input admission bounds, not a process RSS guarantee.
- `PUB-V0-024`: The retained qualification receipt MUST use only `PASS`, `FAIL` and `NOT_RUN`, with
  an explicit row for core archive, companion bundle, exact version identity, affected selection,
  external Playwright receipt discovery, documentation-corpus discovery and work-queue observation
  on macOS amd64/arm64 and Linux amd64/arm64. macOS arm64 MUST PASS every row before assembly.
  Linux amd64 companion and installed workflows MUST remain `NOT_RUN` until executed on Linux;
  cross-build success MUST NOT become installed-platform qualification.
- `PUB-V0-025`: The candidate installer MUST reverify the closed candidate, select only the exact
  host core archive and run its installed `--version` before atomically retaining it under the
  unique `<store>/corvint/<version>/<goos>-<goarch>` path. It MUST refuse an existing path and MUST
  NOT create or change a `current` or `latest` selector. Upgrade, coexistence and rollback are
  explicit selection of immutable version/platform paths, never silent replacement. The store MUST
  be an absolute canonical path with no symlink components, disjoint from the retained candidate
  in either direction. Its existing components MUST be directories. The store name and managed
  child components MUST reject case-fold aliases and parents exceeding 4,096 entries. Candidate
  and installed version probes MUST use an owned process group, private working directory/HOME,
  closed environment, 30-second deadline, one-second shutdown allowance and 4 KiB per output
  stream. Success requires the exact version, observed zero exit and completed group cleanup;
  cancellation before publication MUST retain no installation. Probe output MUST NOT be echoed
  in diagnostics. Store ownership is exclusive and trusted-local: concurrent adversarial renames,
  detached/escaped descendants and power-loss durability are not qualified by these checks.
- `PUB-V0-026`: Candidate assembly or installation MUST fail without retained output when any core
  or companion gate, checksum, source identity/archive, installed workflow, version identity,
  manifest inventory, qualification row, or no-replace promotion check required by
  `PUB-V0-022..025` fails. Local assembly and installation MUST NOT claim or perform tagging,
  pushing, signing, uploading, publication or promotion.

### PUB-V0-020 foreground JS acceptance design

The minimum non-editor JS path is an explicitly enabled product foreground watch mode in
`corvint-js-test-provider unit|e2e`, reusing the existing runner, retained documents and MCP reader.
It requires `--watch RELPATH` (repeatable), `--foreground`, `--experimental`, `--trusted-local`,
`--retain`, and real package.json, lockfile, config and nonempty test-file bindings. One-shot
invocations retain their defaults. Fixed bounds are 128 regular files, 8 MiB total watched bytes,
250 ms content polling/debounce and a 60-minute maximum session; existing run/output bounds remain.
The explicit watched cohort includes every configured binding and rejects missing, symlinked,
replaced or escaping inputs visibly. This supports in-place file saves; editors that save by
atomic file replacement receive an explicit refusal and require a new session. The original
worktree and working-directory identities are rechecked before observing or starting a run.
Polling is not an atomic filesystem transaction. No service, automatic installation or authority
is introduced.

An initial run and each settled source change use the real provider. Superseded work is cancelled
and its cleanup awaited before replacement; changed-cohort completion cannot publish late green.
Only the original completed provider JSON documents are emitted and retained; no pending receipt
is manufactured. The product stdout is a stream of complete existing JSON values. Lifecycle and
infrastructure failures remain visible and bounded; interruption leaves no owned descendant.
Polling is scheduling evidence, not source-cohort freshness qualification. Real package/lock
bindings preserve the consumer's explicit UNKNOWN when their identity is not reverified; watch
mode MUST NOT manufacture a digest or promote UNKNOWN to CURRENT. During a pending source-only
edit, actual MCP discovery MUST show the previous exact document only with that explicit unknown
freshness. Qualification records this limitation and verifies original retained/document/MCP
consistency through initial-pass, save-fail and save-fix for both JS modes. Unit actual-path proof
precedes E2E/checker expansion. The existing JS experiment specification remains prose-only.

### Core installed receipt and execution contract (PUB-V0-020)

#### CLI and admission

Add `--qualification core|editor`, default `editor` for historical CLI compatibility; unknown values
fail before effects. Core accepts only bundle/2; editor refuses /2 before scratch. Core requires the
existing bundle-dir, source-root, scratch, output, npm-cache, browser-cache, node, python,
python-sha256, expected-commit and expected-tree flags, plus `--node-sha256`, `--npm-sha256`,
`--go-authority-bundle`, `--go-authority-sha256`. npm identity is the resolved adjacent npm-cli.js;
no arbitrary executable command flag. All hashes canonical lowercase SHA256; Git IDs validated by
actual immutable source/object format. Source-root must be clean and match built checker revision.
The supplied Go attachment must name the exact selected scratch fixture and installed verifier,
admit its actual Go toolchain, scope and temporary/cache roots, and be accepted by the existing
provider. Missing authority stays blocking NOT_PRODUCED. No fixture/fake authority, mock provider,
recorded provider documents or summary rows can be passed as checker inputs. A caller-measured
experimental trusted-local attachment is only that explicit tier, never protected host FULL.
Before effects: existing path/overlap/exclusive-output checks plus frozen archive/checksum/smoke
sidecars, nonsymlink runtime/cache roots, exact Go attachment bytes and required cache contents.
A fresh exclusive lock covers the invocation; stale/existing locks refuse, never get removed blindly.

`make public-release-check` (`script/public-release-check`) selects the profile through
`CORVINT_PUBLIC_RELEASE_QUALIFICATION`: unset means `editor`; `core` passes `-qualification core`
and requires `CORVINT_NODE_SHA256`, `CORVINT_NPM_SHA256`, `CORVINT_GO_AUTHORITY_BUNDLE` (an
absolute regular file that must not overlap scratch or result) and `CORVINT_GO_AUTHORITY_SHA256`,
forwarded as the four core flags. `editor` refuses any of the four; an empty or unknown selection
exits 2 before any build, scratch or result effect.

#### Result envelope and evidence encoding

New closed profile `corvint-public-release-core-installed/0` has exactly:
`profile,status,bundleSha256,sourceCommit,sourceTree,nodeSha256,npmSha256,pythonSha256,identity,
providers,console,roadmap,docs,stages`.
`status` only PASS; no report file on failure, cleanup failure or NOT_PRODUCED. No editorHost/VSIX/FULL
fields. `identity` is one InstalledEvidence with name `identity`; providers/console/roadmap/docs are
arrays of InstalledEvidence (`name,sha256,raw` exactly), all names below required once, no extras.
`raw` is a JSON string containing original bounded UTF-8 document bytes, never a parsed/reencoded
replacement; sha256 hashes those bytes. Existing native shapes are validated by their actual reader
or matching closed profile; reject duplicate JSON keys, wrong profiles and unknown required variants.
`stages` contains unique core-only `{name,stdoutSha256,stderrSha256,stdout,stderr}` for the
actual fixed stages below. Each stream is its exact bounded UTF-8 string with its own SHA256.
Report max8MiB, combined stage streams1MiB; truncation fails. Top runtime/identity digests must agree.

#### Exact witness names and assertions

Providers, for each prefix `js-unit`, `js-e2e`, `go` (24 rows total):
`PREFIX-initial-provider`, `PREFIX-initial-mcp`, `PREFIX-pending-fail-mcp`, `PREFIX-fail-provider`,
`PREFIX-fail-mcp`, `PREFIX-pending-fix-mcp`, `PREFIX-fix-provider`, `PREFIX-fix-mcp`.
Additionally `test-validity-discover`, `test-validity-list`, `provider-transitions`,
`provider-interruption` (28 provider rows total).
Provider rows are original existing JS output or Go completed session event. MCP rows are original
JSON-RPC responses selecting that exact retained file. Initial/fail/fix execution must be actual
PASSED/FAILED/PASSED, nonempty tests, no infrastructure/cancellation/omissions; Go sequence increases.
Pending MCP must select the prior exact document and cannot claim CURRENT after an unmeasured edit.
UNKNOWN freshness, NOT_MEASURED strength and explicit preview tier are preserved, never adequacy.

`provider-transitions`: new closed `corvint-core-provider-transitions/0` with exactly
`profile,status,sessions`; exactly three sessions with fields `kind,commandSha256,fixtureBeforeSha256,
fixtureFailedSha256,fixtureFixedSha256,initialSha256,failedSha256,fixedSha256,pendingFailSha256,
pendingFixSha256,retainedBytesMatch,supersededSuccessAbsent,supersededProviderRaw,supersededMcpRaw`. Kind is unit/e2e/go. The raw driver
produces this only while running each installed product session once and making real source saves.
The supersededSuccessAbsent assertion additionally requires an actual edit during active execution,
an observed newer generation and prior-run cleanup. JS withholds the superseded completion. Go
MUST preserve its native retained stale negative (LPCV-V0-055), requiring STALE freshness and
non-success run/test projections, followed by a higher-sequence correctly bound replacement PASS.
The two superseded Raw strings hold the exact Go stale event and MCP response (empty for JS);
they and the transition stream are retained in actual stage bytes. No old current/success result
may qualify. Idle edits or TERM/INT alone do not prove this boundary.
`provider-interruption`: new closed `corvint-core-provider-interruption/0` with `profile,status,runs`;
six runs (unit/e2e/go × TERM/INT), each `kind,signal,exitCode,observedIdentities,remainingIdentities,
cancelledReceiptPublished`. Identity `{pid,startTime,observation}` uses actual platform birth data;
observation explicitly labels seconds-resolution ps when that is the available witness. No kernel
admission claim. Nonempty observed identities, remaining empty, cancelled receipt false required.

Docs exact names: `watch-receipt.json,mcp-discover.json,mcp-list.json,mcp-draft.json,mcp-consume.json,
conflict-receipt.json`. First five unchanged original helper outputs. New conflict profile
`corvint-core-docs-conflict/0` exactly `profile,status,humanBeforeSha256,humanAfterSha256,
eligibleCommit,refusalExitCode,refusalReceipt`; preserve exact native WatchReceipt response string. Its closed fields are profile,cycles,writes,
stopped_reason,complete,summaries,omitted_summaries; profile corvint-docmaintain-watch/0,
stopped_reason maintenance-conflict, refusalExitCode2, writes unchanged after human edit. Each summary has
commit,tree,source_digest,page_digest,status. Human digests equal, eligible new commit real.
Source: internal/docmaintain/watch.go:21 and cmd/corvint/docs_maintain.go:213.

Roadmap exact names: `tasks-seed-first,tasks-seed-replay,tasks-roadmap,tasks-ticket-details,
tasks-no-dispatch`. Seed script currently emits text, so seed rows use new closed
`corvint-core-tasks-seed/0`: `profile,phase,sourceSha256,storeBeforeSha256,storeAfterSha256,stdout`;
stdout is exact original script output, phase first/replay; replay store digest equality mandatory.
Roadmap preserves actual taskman-command-result/0 with closed envelope fields profile,command,
outcome,codes,snapshot,mutation,items,page,untrusted,warnings. Require OK, mutation null,
page.truncated false, exactly11 immutable IDs in expected milestone/order. Native roadmap items:
milestone,ticketId,title,status,kind,priority,order,owner,requiredGates,gateResults,eligibility,nextAction.
Details use new closed corvint-core-tasks-details/0 with profile,responses; exactly11 original
bounded `ticket show ID` response strings in stable ID order. Validate original embedded ticket
records against seed source acceptanceCriteria/dependencies/requirementRefs/source digests; preserve
NOT_OBSERVED gate/evidence/attempt axes. Existing native wire/record schemas govern nested shapes.
No-dispatch uses new closed `corvint-core-tasks-no-dispatch/0` exactly
`profile,queueStatusRaw,attemptNotRunRaw,stateBeforeSha256,stateAfterSha256,laneArtifactsAbsent`.
Both Raw fields are exact original command-result/0 strings. queue status must report fixture true,
executionCutover false,11 tickets and attempts/publication NOT_OBSERVED. `attempt` must return
NOT_RUN without mutation. State inventories must match around that command, and no attempts,
effects or worktrees lane directories may exist. Ticket records must be MANUAL. This proves the
bounded fixture did not dispatch; it never turns absent native observation into a fabricated
admission/dispatch count or production execution guarantee. Source: Tasks063 internal/cli/cli.go
queueStatus/omitted verbs, internal/store/store.go genesisDirectories, internal/wire/result.go.


Console exact name `browser`; separately versioned `corvint-core-console-proof/0`, leaving historical
corvint-installed-roadmap-proof/0 unchanged. Exact fields `profile,status,browserName,browserVersion,
browserExecutableSha256,tickets,forms,bodySha256,sourceDigest,originRefusal,hostRefusal,sessionRefusal,
storeBeforeSha256,storeAfterSha256,observedIdentities,remainingIdentities`. Chromium real navigation,
11 tickets/zero mutation forms, valid detail/evidence pages, actual forbidden Origin/Host/missing and
invalid session requests before effects. All refusal booleans true, store digests equal, no remaining
identities. Session refusal uses the actual POST /mutate form-token check: missing token and invalid token
each return403 `invalid form token; reload this page` before any handler. Foreign Origin and Host
return403 at the outer boundary. No token requirement is imposed on read-only pages. Requests and
native status/body originals retained inside stage output. Source: internal/console/server.go:138-183.

#### Identity row and stages

Identity raw profile `corvint-core-installed-identity/0`, exact fields `profile,archiveSha256,
checksumsSha256,smokeSha256,sourceCommit,sourceTree,tasksCommit,binaries,runtimes,fixtures,
npmCacheSha256,browserCacheSha256,goAuthoritySha256,platform,unchangedAfter`.
Binary rows exactly nine manifest paths with `{name,sha256}`; runtimes exactly node/npm/python/browser/
git/go with `{name,sha256,version}`; fixture rows exactly js-unit/js-e2e/go/docs/planning with
`{name,treeSha256,packageSha256,lockSha256,configSha256,testSha256}`, using explicit empty only when
that artifact does not exist for that fixture type. `platform` exactly `{os,arch}`; unchangedAfter
true only after final rehash. Digests over bounded canonical path/content inventories, no scratch path
leak. Initial fixture rows describe frozen inputs; mutations are bound by transition/docs/seed rows.
Rehash archive/sidecars/runtimes and immutable fixture members before/after every child; mutable
fixture files may change only at explicit observed transitions. Private cache source never mutates.
Stages exactly `offline-dependencies,providers,provider-interruption,docs,planning,console`;
setup/cleanup errors fail their enclosing stage. No unattached or prebuilt witness accepted.
Successful scratch removal follows final validation and exclusive atomic publication; any cleanup
failure withdraws that invocation's own result. Failed execution retires all owned processes but
preserves its owned scratch evidence and emits no PASS report. Historical editor stages stay unchanged.

#### Reused helper interfaces and order

1. Existing VerifiedRetainedBundle extraction/identity, admission, copyBoundedTree, runtime hash,
   procgroup bounds, exclusive atomic output. Separate RunCoreInstalledQualification/options/report.
2. New retained-source `conformance/interactive-alpha/core-provider-proof.mjs` calls installed
   watch CLI and existing Go session, reuses transport-neutral mcp-client.cjs behavior (move no editor
   production code). JS exact flags/output already proven; Go attachment supplied externally.
3. Existing Python watch_mcp_proof.py interface `--corvint BIN --docs-mcp BIN --output-root DIR`;
   add explicit core profile option/extra conflict witness while preserving historical output/default.
4. Existing seed-planning-store.sh with installed Tasks plus existing original roadmap browser driver;
   add separate core mode/shape and actual refusal/descendant checks, preserve old profile0.
5. Freeze all helpers/source/CEM before final canonical and cold bundle. Installed run only against
   that frozen retained bundle and same clean checker. First missing authority/tool remains visible
   NOT_PRODUCED; never report native completion just because serialization tests pass.

#### Required negative controls

Mixed profiles/editor/core, missing/unknown options, bad expected identity, missing/extra/duplicate names,
noncanonical hashes/raw mismatch, unknown/extra closed fields, omitted actual browser, manual/replayed
provider results, stale execution or false CURRENT/adequacy, Go authority absent/mismatch, late stale
publication, invalid file/path/cache/tool drift, dependency network fallback, docs conflict overwrite,
Tasks replay mutation/dispatch/admission, console refusal-after-effects, surviving original birth
identity, overflow/timeout, archive/sidecar drift, output overlap/preexistence/concurrency and partial
cleanup: no PASS output. Existing UNKNOWN/preview states alone are not failure where PUB020 allows.

### Companion inventory compatibility (CRB-V0-017)

PUB-V0-002/003/013/015 name the current writer inventory under manifest profile
`corvint-companion-bundle/1`. Its eight core components use module `corvint`, shared source
`source/corvint-src.tar.gz` and `notices/corvint/`; Tasks uses module `corvint-tasks`,
`source/corvint-tasks-src.tar.gz` and `notices/corvint-tasks/`. Since decision 0397 that Tasks
archive is the standalone subset of the same recorded Corvint commit (`go.mod`, the notices,
`cmd/corvint-tasks/**` and `internal/tasks/**`), and the Tasks binary carries the same
`-X main.build` stamp as `corvint`. Binaries are exactly `bin/<name>`.
The six artifacts are `corvint-vscode` at `extensions/corvint-vscode-0.1.0.vsix` and the
unchanged four host names at `plugins/{codex,claude-code,gemini-cli,opencode}/`, plus
`pi` at `plugins/pi/`. Explicit `corvint-companion-bundle/0` retains exactly the prior
five-artifact current inventory without Pi; all nine component identities remain unchanged.

An absent profile exclusively selects the historical inventory: `corvint`, `corvint-console`,
`corvint-dashboard-snapshot`, `corvint-mcp`, `corvint-docs-mcp`, `corvint-test-validity-mcp`,
`corvint-js-test-provider`, `corvint-go-test-provider` use module `corvint`; `atm` uses
`corvint-taskman`. Their exact source/notice paths retain those module names and VSIX remains
`corvint-vscode` at `extensions/corvint-vscode-0.1.0.vsix`, with the same four host artifacts.
No current writer emits this legacy form. Explicit null/empty/unknown profile and any mixed
identity MUST fail; no name-based inference is permitted. All three forms retain every strict
JSON, source-identity, archive, checksum, notice and smoke-binding check. Smoke step names,
provider receipt/retention profiles and MCP `corvint.*` tool names remain byte-compatible.
The current VSIX build uses the planned local Beamfall/corvint source coordinate without
asserting publication. Rollback restores the legacy writer only with its exact legacy source;
it cannot relabel current binaries as a valid historical inventory.

## Non-goals and failure modes

No service installation, shared database, hosted dashboard, automatic account/license creation,
real task dispatch, third-party executable redistribution or promotion claim is implied.
A stale target, cross-site mutation, silent provider fallback, escaped child, unbounded output,
missing source/license, digest mismatch or failed required check blocks readiness.
GitHub CI billing is an external prerequisite for hosted checks, never a test pass.

## Acceptance and rollback

Use the existing native gate, independent review and strict Corvint local completion workflow,
plus the optional bundle's reproducibility and real installed-path evidence. The task-manager
source receives its own canonical verification and independent review. U4/GP performance
qualifications remain `NOT_RUN` unless measured; no timing benefit is inferred from packaging.
Rollback before publication is removal of the optional binaries and retention of the preceding
native Go artifact; no ticket-store format or persisted-state migration is introduced.

`PUB-V0-002`/`PUB-V0-003`'s optional-bundle evidence is `script/corvint-companion-release-gate`
(opt-in `make companion-release-gate`, deliberately not a `go-archive-gate`/`gate` prerequisite:
it rebuilds four binaries twice each and is far too slow for the gate). The script is a thin
wrapper, matching `script/go-archive-gate`'s own convention: it sets up an isolated environment
and invokes `cmd/corvint-companion-release` once for `darwin/arm64` on its own checkout, which
since decision 0397 also holds `cmd/corvint-tasks` (it no longer clones a separate Tasks
checkout or reads `CORVINT_TASKMAN_REPO`) — the
double-build/double-archive-assembly-must-agree requirement is enforced inside that command
itself (`internal/companionrelease`, `buildComponentTwice` and `buildTarGzTwice`), per
`PUB-V0-011..015`. A refused or failed run retains no output and prints no `qualified` claim
(`companionrelease.Run` returns before `retainBundle` on any build, archive, or smoke failure).
A wrapper signalled by `HUP`, `INT` or `TERM` exits 129, 130 or 143 after removing its private
parent, never 0 (decision 0248).
As of 2026-09-12 this has been exercised against a real `corvint-taskman` clone; see
`docs/BUILD-LOG.md` for the run's outcome. The smoke's refused-mutation steps are the loopback-HTTP
half of `PUB-V0-005`'s installed-path refusal-before-effects check. Its control, edit and evidence
and requirement/code link steps are the loopback-HTTP half of `PUB-V0-004`. Browser qualification
with named versions stays a separate, not-yet-exercised step that this command declines to claim.

## Traceability

| Requirement | Implementation | Test |
|---|---|---|
| PUB-V0-001 | `VERSION`, `cmd/corvint/main.go`, `conformance/release-artifact-v0/manifest.json`, `extensions/vscode/src/executable.ts`, `conformance/vscode-extension-v0/cases.json` | `TestPublicReleaseVersionTupleMovesTogether`, `TestGoOnlySourceAndVersion`; historical evidence keeps its own identity: `TestHistoricalBenchmarkEvidenceRetainsOriginalVersionIdentity` |
| PUB-V0-002 | `Makefile` (`gate`/`companion-release-gate` targets), `internal/companionrelease` | archive-gate independence across every target and recipe `gate` reaches: `TestGateExcludesCompanionReleaseGate`; pinned-commit companion binaries: `TestValidateTargetRefusesUnsupported`, `TestRequireCleanTreeRefusesDirtyCheckout` (PUB-V0-011), `TestVerifyBuildInfoRequiresPinnedClosedBuild` (PUB-V0-013); measured end-to-end: the 2026-09-12 companion gate run in `docs/BUILD-LOG.md` |
| PUB-V0-003 | `internal/companionrelease/companionrelease.go` | `TestRunFailuresRetainNoOutputAndNoQualifiedReport` (a refused gate, a smoke error and a failed smoke step each return no report and retain nothing); the remaining clauses are covered by the PUB-V0-011..015 rows and the measured gate run |
| PUB-V0-004 | `internal/companionrelease/smoke.go`, `internal/console/links.go` | `TestCheckConsolePagesRequiresReasonedControlsAppliedEditAndEvidence` (the installed smoke's control, edit and evidence checks), `TestCheckConsoleLinksRequiresPinnedResolvingLinksAndExplicitGaps` (its requirement/code link checks); named platform claim, browser qualification not yet exercised: `TestReleaseNotesPlatformAndBrowserClaimsNameTestedVersionsOrDeferQualification` |
| PUB-V0-005 | `internal/console/server.go`, `internal/companionrelease/smoke.go` | `TestConsoleBoundary` (unit, LAC-V0-021), `TestCheckMutationRefusalsRequiresRefusalWithoutStoreEffect` (the installed smoke's refusal check); interruption: `TestRunCapturedCleansUpDescendantsOnCancel` |
| PUB-V0-006 | `internal/testvalidity`, `internal/jstestprovider`, `internal/testvaliditydoc` (`Discover`), `conformance/mcp-test-validity-v0` | shape: `TestProjectMatchesSharedVectors`, `TestSharedVectorsCoverRequiredCases`; reach: `TestVectorsAndReadOnly`, `TestToolCatalogueIsExactlyOneReadOnlyTool`; automatic consumption at experimental tier (decision 0202, `LPCV-V0-053..054`, `MTV-V0-009`): `TestDiscoverSelectsNewestRetainedEvidence`, `TestDiscoverRefusesSymlinkedEvidence`, `TestDiscoverUnmatchedIdentityIsNeverCurrent`, `TestDiscoverWithoutEvidenceIsUnsupported`, `TestTestValidityDiscoveryMatchesMCPDocument`; opt-in producer retention (decision 0218, `LPCV-V0-055`): `corvint-js-test-provider --retain` and `corvint-go-test-provider session --retain` write their stdout document into `.corvint/test-evidence`, bounded to 32 and atomic, `TestEmitRetainsStdoutBytesPrunesAndRefusesSymlink`, `TestSessionRetainsCompletedEventBytesPrunesAndRefusesSymlink`, `TestTestValidityDiscoverSelectsProducerRetainedDocument`; VS Code opt-in retention (decision 0230, `VSC-V0-070`): `corvint.liveTests.retainEvidence`, default `false`, inserts `--retain` after a supported provider subcommand and refuses any other command, `withEvidenceRetention passes --retain only on explicit opt-in to a retaining provider subcommand (VSC-V0-070)`; not met: retention stays opt-in rather than automatic, the extension-to-discovery path is unit-tested only, Go results stay preview (`GLTP-V0-048`), and the real-provider Electron run with retention and a qualified Go session matrix are `NOT_RUN` — `docs/plans/pub-v0-006-reach-gap-2026-09-12.md` records the remaining gap |
| PUB-V0-007 | `docs/RELEASE-NOTES.md`, `docs/RELEASE-NOTES-alpha.md` | `TestReleaseNotesExcludeRustAndWindowsFromReleaseSet` (Rust and unqualified Windows stay out of the release set), `TestReleaseNotesPlatformAndBrowserClaimsNameTestedVersionsOrDeferQualification` (qualified platforms), `TestReleaseNotesPublicationReasonMatchesReleaseChecklist` (the readiness paragraph states the checklist's own publication `NOT_RUN` reason), `TestReleaseNotesCEMTrustCitationLandsOnTrustRoots` (the CEM Git-trust disclosure's line citation still lands on the roots-of-trust statement); the wording of the experimental-feature, CEM Git-trust, unmeasured-performance and hosted-CI disclosures is reviewed by hand |
| PUB-V0-008 | `script/release-checklist` | `script/release-checklist_test.sh` (tag/publication/promotion report `NOT_RUN` by default, so local readiness never claims they happened); freezing source/CEM/OCM before canonical gates is an unautomated human step, not a test — `docs/plans/public-release-v0-gap-2026-09-12.md`'s release checklist items 2-3 |
| PUB-V0-009 | `internal/docmaintain`, `cmd/corvint-docs-mcp` | `TestApplyDetectsConcurrentEditAndRefusesWithNoWrite`, `TestUnauthorizedApplyRefuses` (human authority, edit conflicts), `TestServeToolsListAndCallRoundTripsDocsDraftAndConsume` (MCP draft/consume round trip); the release notes list this surface as experimental, so a read-only draft tool alone is not claimed to be automatic maintenance; `TestApplyRefusesWhenPageExistenceChanged` (page existence is part of the conflict boundary), `TestZeroMaxWritesDefaultsToSelectorCount`, `TestSelectorCommentBreakoutRefused` (`-->` or `"` in the package selector), `TestParseArgumentsIsClosed` (empty `--root`), `TestToolsListRefusesUnknownKeyAndNonEmptyCursor` |
| PUB-V0-010 | `internal/console` (`/roadmap`) | `TestRoadmapGroupingAndBlockers`; `atm roadmap`, planning-ticket seeding and duplicate avoidance live in the separate `corvint-taskman` repository and are not exercised by this repository's tests |
| PUB-V0-011 | `internal/companionrelease/target.go`, `internal/companionrelease/clean.go`, `internal/companionrelease/companionrelease.go`, `internal/gitstatus/status.go`, `script/corvint-companion-release-gate` | Existing path/cleanliness tests; `script/corvint-companion-release-gate_test.sh` proves a real linked worktree reaches the release command while preserving the clean clone boundary. |
| PUB-V0-012 | `internal/companionrelease/export.go`, `internal/companionrelease/verify.go` | `TestListTreeEntriesRefusesSymlinkAndSubmodule`, `TestListTreeEntriesRefusesDuplicatePaths`, `TestListTreeEntriesRefusesCaseFoldCollisions`, `TestVerifyTarGzRefusesCaseFoldedMembers`, `TestExportSourceRefusesOversizedEntryCount`, `TestExportSourceRefusesTotalBytesBound`, `TestVerifyBlobDigestRefusesMismatch`, `TestRefuseUnsafePath`, `TestExportSourceRefusesDisagreeingPasses`, `TestBatchBlobsRefusesTrailingData` (data after the last requested object in either independent `cat-file --batch` pass is refused, not silently dropped from the byte-for-byte agreement check); `TestExportSourceAdmitsCountsAtTheirBounds` (exactly 10,000 entries and exactly 128 MiB are admitted), `TestBatchBlobsRefusesWrongRecordTerminator` |
| PUB-V0-013 | `internal/companionrelease/build.go`, `internal/companionrelease/vsix.go`, `internal/companionrelease/npm_notices.go`, `internal/companionrelease/companionrelease.go` | Existing staged-export/double-Go-build tests plus `TestCanonicalVSIXRequiresClosedAgreeingMembers` and `TestBuildOnlyNPMNoticeIsExplicitlyUnshipped`; final exact-source nine-binary/VSIX build remains the retained companion gate. |
| PUB-V0-014 | `internal/companionrelease/archive.go`, `internal/companionrelease/verify.go`, `internal/companionrelease/manifest.go`, `internal/companionrelease/packages.go`, `internal/companionrelease/companionrelease.go` | Existing archive tests plus shared-source/manifest-reference and exact exported host-package-tree tests in `internal/companionrelease`. |
| PUB-V0-015 | `internal/companionrelease/smoke.go`, `internal/companionrelease/workflow_smoke.go`, `internal/companionrelease/retain.go`, `internal/companionrelease/companionrelease.go`, `internal/companionrelease/proc.go` | Existing installed smoke/process tests plus `TestMCPDiscoveryRequiresExactProtocolAndIdentity` and `TestRetainedSmokeReportBindsArchiveOutsideArchive`; the retained exact-binary run remains the final companion gate. |
| PUB-V0-016 | `extensions/vscode/test/installed`, `conformance/interactive-alpha`, `internal/companionrelease/installed.go`, `cmd/corvint-public-release-check`, `script/public-release-check` | retained verifier, no-overwrite and wrapper refusal tests (`script/public-release-check_test.sh`); the final exact retained installed run remains required. |
| PUB-V0-020 | `internal/companionrelease/core_installed.go`, `internal/companionrelease/core_evidence.go`, `conformance/interactive-alpha`, `cmd/corvint-public-release-check`, `script/public-release-check` | `TestCoreOptionsRemainExplicitAndClosed`, `TestCoreClosedStageRequiresExactNonNullableFields`, `TestCoreReaderRejectsHistoricalAndMixedShapes`, `TestCoreNativeWitnessReplay`; `script/public-release-check_test.sh` (core selection pass-through, editor refusal of core settings, unknown/empty selection, missing core setting, relative/non-regular/overlapping Go authority refused before effects); the retained installed runs are recorded in `docs/BUILD-LOG.md`. |
| PUB-V0-021 | `cmd/corvint/main.go` (`build`), `Makefile` (`build`), `conformance/release-artifact-v0/build.go` (`buildNumber`, `buildArguments`), `conformance/release-artifact-v0/archive_run.go`, `extensions/vscode/src/executable.ts`, `script/dogfood-change.sh`, `script/dogfood-check.sh`, `script/dogfood-bind-range.sh` | `TestSmokeTestExecutesRealSubprocessAndDetectsFailures` (missing and wrong build numbers fail), `TestGoOnlySourceAndVersion` (unstamped build 0), `TestCorvintHostArchivePartialProof` (extracted archive smoke requires the exact first-parent count), `version probe requires the build number (VSC-V0-007 PUB-V0-021)` |
| PUB-V0-022 | `internal/releasecandidate` (`verifyCore`, `verifyCoreArchiveBinary`, `Assemble`), `internal/companionrelease/retained.go` | `TestPUBV0022VerifyCoreRequiresClosedReproducibleChecksummedSet`, `TestPUBV0022CoreArchiveVerifierRejectsNonArchiveBytes`; the real dry run consumes both retained gates at exact commits |
| PUB-V0-023 | `internal/releasecandidate` (`Manifest`, `assetsFor`, `renderChecksums`, `Verify`) | `TestPUBV0023ClosedManifestRunsIsolatedHostProbe`, `TestPUBV0026CandidateInventoryBounds`, `TestPUBV0023CandidateVersionGrammar`, `TestPUBV0023ReleaseCandidateRC1AssemblesVerifiesAndInstalls`; real candidate verification closes the manifest/checksum inventory |
| PUB-V0-024 | `internal/companionrelease/core_smoke.go`, `internal/companionrelease/smoke.go`, `internal/releasecandidate` (`buildQualification`, `validateQualification`) | `TestPUBV0024InstalledCoreDiscoveryWorkflows`, `TestInitSmokeRepoPinsIntentBranch`; retained-bundle tests require the five new `/2` steps while preserving legacy profiles |
| PUB-V0-025 | `internal/releasecandidate/install.go`, `cmd/corvint-release-install` | `TestPUBV0025VersionedInstallCoexistsAndNeverReplaces`, `TestPUBV0025PromotionNeverReplacesExistingCandidate`, `TestPUBV0025RecoveryLifecycle`, `TestPUBV0025HostileStore` |
| PUB-V0-026 | `cmd/corvint-release-candidate`, `cmd/corvint-release-install`, `internal/releasecandidate` | `TestPUBV0026FailedInputRetainsNoCandidate`, `TestPUBV0026CandidateVerifierRejectsChecksumDrift`, `TestPUBV0026ScratchAndOutputCannotOverlapInputs`, `TestPUBV0026ProbeFailureCleansInstall`, `TestPUBV0026InterruptedInstallReapsDescendant`, `TestPUBV0026CancelledInstallRetainsNothing`; completed staging is reverified before promotion, command errors precede retention and neither command has a publication operation |

## Core release scope amendment (2026-09-16)

The owner's later direction defers stock VS Code support/FULL, upstream PR merge/delivery and
editor qualification. They are nonblocking for this core release; existing failures and NOT_RUN
remain historical evidence. No human study is required. Codex CLI and Desktop separately, Claude
Code, Gemini CLI and Pi still require exact runtime FULL/authority evidence; stock OpenCode remains
FALLBACK. Accepted qualified retrieval and final exact-source release gates remain mandatory.

PUB-V0-003/013/014 now dispatch the new core inventory through CRB-V0-019. The absent, `/0`, `/1`
closed readers and corresponding older build evidence retain their original meaning. PUB-V0-006
requires actual core provider-to-agent/MCP evidence; its editor presentation is deferred
under PUB-V0-016. PUB-V0-015 still requires the complete extracted-binary smoke and is insufficient for
core installed qualification: the actual retained JS unit/E2E and Go execution/discovery obligation
is PUB-V0-020 for `/2`. PUB-V0-016 is preserved intact as the historical deferred editor requirement;
its results MUST NOT be rewritten as passing core evidence. PUB-V0-004/005/009/010 remain binding.

### Core build and installed command boundary

`cmd/corvint-companion-release` writes `/2` by default; no writer profile-selection flag is added.
Existing source/tasks aliases, target, scratch, output-parent and bundle-name arguments retain
admission. `--npm-cache` remains an accepted compatibility argument but `/2` neither resolves nor
uses it. Retained readers, not new writer switches, preserve old inventories.

`cmd/corvint-public-release-check --qualification core` selects only `/2` and requires the existing
`--bundle-dir`, `--source-root`, `--scratch`, `--output`, `--npm-cache`, `--browser-cache`, `--node`,
`--python`, `--python-sha256`, `--expected-commit`, `--expected-tree` plus independently frozen
`--node-sha256` and `--npm-sha256`. The existing adjacent npm CLI resolution remains, with canonical
path/hash checks; runtime/dependency pins remain Node v22.23.2, npm10.9.8, Vitest5.0.0 and
Playwright1.63.0. Browser identity is measured and bound by the retained fixture runner. The
existing omitted `--qualification` value selects the historical editor path; explicit `editor`
selects that same path. Empty/unknown selection, core/old-bundle or editor/core-bundle pairing,
missing core digests and cross-profile result shapes refuse before execution. Protected Go
provider attachment is still required by its own contract; this CLI cannot mint it or silently
fall back to a mock session.

Core result profile is `corvint-public-release-core-installed/0`, with exactly `profile`, `status`,
`bundleSha256`, `sourceCommit`, `sourceTree`, `nodeSha256`, `npmSha256`, `pythonSha256`, `providers`,
`console`, `roadmap`, `docs`, `stages`. Existing digest/string encodings, `InstalledEvidence`
(name/sha256/raw) and `InstalledStage` (name/outputSha256) shapes remain. `status` is `PASS` only
when every required operation succeeds. `providers`, `console`, `roadmap` and `docs` are arrays
of bounded original `InstalledEvidence`; they cannot be empty. Their closed admitted evidence
profiles and mandatory named witnesses must be reviewed against the actual reused fixtures before
checker implementation. No generic arbitrary-JSON evidence may qualify a result. `editorHost`
is forbidden. Historical `corvint-public-release-installed/0` shape and meaning remain unchanged.

Acceptance is pending: exact profile/mixed-shape/digest/refusal-before-write tests, actual retained
provider/docs/Tasks/browser runs, cancellation/no-descendant regression, independent receipt review
and final release-checklist rows. An explicit deferred editor row remains nonblocking without being
marked PASS; missing core installed or required exact runtime FULL/retrieval evidence blocks release.
Rollback removes `/2` publication eligibility and the core checker path, preserves old reader
behavior and all failed/NOT_PRODUCED receipts, and never downgrades or promotes retained evidence.
