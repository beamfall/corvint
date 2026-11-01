# Spec toolchain adapters V0

Owner: Russell Lewis
Date: 2026-10-17
Intent status: accepted
Delivery status: experimental
Authoritative inputs: `AGENTS.md` invariants 2, 3, 6 and 7, decision 0540, decision 0541,
`docs/specs/ocm-intent-forms-v0.md` (`OIF-V0`), `docs/specs/verified-absence-frontier-v0.md`,
`docs/SPEC-TOOLCHAIN-INTEGRATION.md`, `LICENSING.md`

## Agent digest
- Claim: Corvint reads pinned OpenSpec and Spec Kit projects as declared OCM intent forms with native IDs, and CI checks them against upstream.
- Status: accepted intent (owner, 2026-10-17, decision 0540); delivery experimental (partial, `STA-V0-001` to `STA-V0-014` built): the version pins, forms, context classification, host packaging, conformance job, latest-release check and self-development route exist, and `OIF-V0` is accepted (decision 0541), but nothing is advertised as delivered until a `spec-toolchain-conformance` CI run passes (`STA-V0-011`).
- Exists: pins in `interop/spec-toolchains/versions.json`, forms in `internal/lrfrepo/ocm_toolchain_forms.go`, path classification in `internal/contextindex/toolchain_status.go`, packages under `protocol/integrations/{speckit,openspec}/`, fixtures under `interop/spec-toolchains/fixtures/`, `script/spec-toolchain-{conformance,latest}.sh`.
- Blocked on: a passing `spec-toolchain-conformance` CI run (`STA-V0-011`, V1-1697); it passed locally on 2026-10-18.
- Read next: Requirements; Lanes.

## User and job

A team that already writes specs with OpenSpec or Spec Kit wants Corvint to pin, cite and check
those specs without rewriting them in Corvint's grammar (invariant 6). Their agent should get Corvint
context before it implements and a Corvint review after, from inside the toolchain's own workflow.
Corvint's maintainers want CI to fail when either toolchain's current release changes a format
Corvint reads.

## Verified starting state

Upstream, read from the release sources on 2026-10-17:

- OpenSpec 1.14.3. Current truth lives in `openspec/specs/<capability>/spec.md`, where the
  capability may be nested. A change's deltas live in `openspec/changes/<id>/specs/**/spec.md`. An
  archived change moves to `openspec/changes/archive/YYYY-MM-DD-<id>/`.
  - A main spec has `## Purpose` and `## Requirements`. Each requirement is a `### Requirement: <name>`
    block with `####` scenarios.
  - Delta files use `## ADDED|MODIFIED|REMOVED|RENAMED Requirements`.
  - Requirements have no IDs. The case-sensitive name is the identity.
  - Lifecycle is derived, not stored: `tasks.md` checkboxes and archive placement.
  - `validate --json`, `show --json` and `list --json` are documented in upstream
    `docs/agent-contract.md`.
  - There are no hooks. `openspec/config.yaml` `context` and `rules` are injected into agent
    instructions.
- Spec Kit 1.2.1. Features live in `specs/<NNN|YYYYMMDD-HHMMSS>-<slug>/spec.md`, alongside `plan.md`,
  `tasks.md`, `contracts/` and `checklists/`. Project principles are in `.specify/memory/constitution.md`.
  - Functional requirements are `- **FR-001**: ...` lines under `## Requirements`. Success criteria
    are `- **SC-001**: ...` lines under `## Success Criteria`.
  - `[NEEDS CLARIFICATION: ...]` marks open items.
  - Status is a free-text `**Status**: Draft` header line.
  - Extensions declare `extension.yml` (`schema_version: "1.0"`) with `provides.commands` named
    `speckit.<ext-id>.<cmd>`. They also declare `hooks` on `before_`/`after_` events for `specify`,
    `plan`, `tasks` and `implement`, among others. Hooks run as prompts in the agent session.

Corvint at `d35b0ee3`:

- The default OCM reader needs `- \`ID\`: ` or `- **ID:** ` lines with
  `^[A-Z][A-Z0-9-]{2,31}-[0-9]{3}$`. That rejects `FR-001`, which has one letter before the dash,
  and every OpenSpec requirement.
- `OIF-V0` already has a declared-form registry with `excluded` derivations.

## Requirements

- `STA-V0-001`: One manifest, `interop/spec-toolchains/versions.json`, records the supported upstream releases. Each entry has the toolchain name, version, source (npm package or Git tag), commit or tarball SHA-256, and the date checked. The initial entries are OpenSpec `@fission-ai/openspec` 1.14.3 and Spec Kit `github/spec-kit` v1.2.1. Corvint's documentation claims support only for listed versions. Every other version is untested and must be reported as such, never as supported.
- `STA-V0-002`: `ocm prepare --intent-form openspec-spec` reads an OpenSpec main spec. The blob needs exactly one unfenced `## Requirements` line, and the span runs to the next unfenced level-2 heading or the end of the blob, with the `OCM-V0-001` fence rules.
  - Each unfenced line matching `^###\s*Requirement:\s*(.+)$` (case-insensitive keyword) is one requirement. Its native name is the captured text with surrounding blanks trimmed and, as upstream, a closing `#` run removed only when a space or tab precedes it (so `C#` keeps its `#`).
  - The statement runs from the header to the byte before the next unfenced level-3 or level-2 heading, so it includes scenarios.
  - The obligation ID is `OSR-` followed by the first 12 uppercase hex digits of SHA-256 over the name's UTF-8 bytes.
  - A repeated name or ID fails `duplicate-requirement`.
  - Any other unfenced level-3 heading in the span fails `invalid-requirement-item`.
- `STA-V0-003`: `ocm prepare --intent-form openspec-delta` reads one OpenSpec delta file.
  - Requirements come from the `### Requirement:` blocks under every unfenced `## ADDED Requirements` and `## MODIFIED Requirements` heading (case-insensitive, repeatable). They use the `STA-V0-002` statement and ID rules.
  - Blocks under `## REMOVED Requirements` and the `FROM:`/`TO:` pairs under `## RENAMED Requirements` are not obligations. A REMOVED entry is either a `### Requirement: Name` header or upstream's bullet form `` - `### Requirement: Name` `` (backticks optional, V1-1707); any other line under REMOVED names nothing. They are listed in document order in the prepare envelope member `excludedRequirements`, as `{operation, name}` or `{operation, from, to}`.
  - The span is the whole blob.
  - An unpaired RENAMED line, a `FROM:`/`TO:` line that names no requirement, or a requirement block outside the four sections fails `invalid-delta-section`. A blob with no delta section fails `missing-requirements`.
- `STA-V0-004`: Both OpenSpec forms emit the prepare envelope member `nativeNames`, which maps each obligation ID to its native name. The name is also recoverable from the pinned statement. Matching is byte-exact and case-sensitive, as upstream.
- `STA-V0-005`: `ocm prepare --intent-form speckit-spec` reads a Spec Kit `spec.md`.
  - Requirements are the unfenced lines that start `- **FR-NNN**: ` inside exactly one unfenced `## Requirements` section (including its `###` subsections), and the lines that start `- **SC-NNN**: ` inside at most one unfenced `## Success Criteria` section. NNN is three or more digits.
  - The native ID is the obligation ID, unchanged.
  - The statement is the line plus any following lines indented at least two spaces.
  - A line whose statement contains `[NEEDS CLARIFICATION` is not an obligation. It is listed in the envelope member `excludedRequirements` as `{id, reason: "needs-clarification"}`.
  - The header's `**Status**:` text, if present, is reported with surrounding blanks trimmed in the envelope member `nativeStatus`; the member is absent when there is no header.
  - Zero or two `## Requirements` sections fail `invalid-requirements-section`. A repeated ID fails `duplicate-requirement`.
- `STA-V0-006`: The three forms follow `OIF-V0-001` to `OIF-V0-003` and `OIF-V0-009`. They are explicit only, never inferred from path or content, recorded in `intentScope.form`, re-derived on every action, and capped at 256. `OIF-V0-010` applies: LRF and the change-universe surfaces refuse them with `unsupported-intent-form`.
- `STA-V0-007`: The context index classifies documents by pinned path, with no content guessing. `documentAuthority` derives the labels below.

  | Path | Index status | Kind | Native state reported |
  |---|---|---|---|
  | `openspec/specs/**/spec.md` | `current` (OpenSpec's declared current truth) | — | — |
  | `openspec/changes/<id>/**`, not under `archive/` | `proposed` | — | — |
  | `openspec/changes/archive/YYYY-MM-DD-<id>/**` | `archived` | — | archive date |
  | `specs/<feature>/spec.md` with `.specify/` present at the root | `**Status**:` text above the first level-2 heading, lowercased; `draft` when absent | — | header text |
  | `.specify/memory/constitution.md` | — | `instructions` | — |

  "`.specify/` present at the root" means the pinned index holds at least one admitted source whose path starts with `.specify/`. It is never a filesystem probe, so a `.specify/` directory that is untracked, ignored or wholly excluded does not make `specs/<feature>/spec.md` a Spec Kit spec (`specKitPresent` in `internal/contextindex/toolchain_status.go`).

  Each result keeps the native state field so the reader sees the upstream lifecycle, not only Corvint's label. An instructions file under `openspec/changes/` takes the row's status and is never binding.
- `STA-V0-008`: `protocol/integrations/speckit/corvint/` holds a Spec Kit extension (Apache-2.0, `LICENSING.md` boundary).
  - Manifest: `schema_version: "1.0"`, id `corvint`, `extension.effect: read-only` (where Spec Kit validates it), `requires.speckit_version: ">=1.2.1,<2"`, and a `requires.tools` entry for `corvint`.
  - Commands: `speckit.corvint.context`, which runs `corvint context` for the feature's spec and plan paths, and `speckit.corvint.review`, which runs `corvint affected` and `corvint review` over the working diff.
  - Hooks: `before_implement` → context and `after_implement` → review, both `optional: true`.
  - The commands only read, report Corvint's output including every `unknown`/`NOT_PRODUCED`, and never block, edit or commit. `corvint review` refuses a dirty worktree, which is normal after implementing; the command reports that refusal as printed and never commits to satisfy it.
- `STA-V0-009`: `protocol/integrations/openspec/` holds an OpenSpec integration (Apache-2.0).
  - A `config.yaml` snippet that tells the agent to run `corvint context` before `apply` and `corvint review` after it. OpenSpec 1.14.3 injects `rules` only into artifacts and `apply` reads `context` and `operations.apply.guidance`, so the steps appear in `rules.tasks`, `context` and `operations.apply.guidance`.
  - An agent skill file with the same two read-only steps.
  - A README giving the exact `ocm prepare --intent-form` commands for main specs and deltas.
- `STA-V0-010`: `interop/spec-toolchains/fixtures/{openspec,speckit}/` hold minimal projects that pass the pinned upstream validators (`openspec validate --all --strict`; Spec Kit's own templates and `check-prerequisites.sh --json`). They include a nested capability, every delta operation, an archived change, `FR`/`SC` IDs, a clarification marker and a constitution.
- `STA-V0-011`: `make spec-toolchain-conformance` and a CI job of the same name do four things.
  - Install the `STA-V0-001` versions with checksum verification: npm with a committed lockfile whose every entry carries a full SRI SHA-512 (88 base64 characters, 64 bytes; a truncated or empty digest is refused before `npm ci`), and Spec Kit from the pinned commit with `uv` or `pip --require-hashes`.
  - Run the upstream validators on the fixtures.
  - Assert that Corvint's derived obligations equal upstream's view: `openspec show --json` requirement names for each OpenSpec form, and the template-grammar FR/SC ID set for Spec Kit, read with the `STA-V0-005` section rule (FR only under `## Requirements`, SC only under `## Success Criteria`), including a case with FR and SC items under another section that both sides exclude.
  - Install the `STA-V0-008` extension with `specify extension add --dev`.

  Any mismatch fails the job. The job runs on changes to its inputs, the forms and the integrations, not on every PR.
- `STA-V0-012`: `script/spec-toolchain-latest.sh` (operator-run, network) compares `STA-V0-001` with the npm registry and the GitHub latest release. It prints `CURRENT`, `BEHIND <latest>` or `UNAVAILABLE` for each toolchain; of those verdicts only `BEHIND` makes it exit non-zero (1). A setup error, a missing `jq` or `curl` or a manifest that is missing, empty, unparsable or has no complete toolchain entry, prints no verdict and exits 2. Moving to a new release means a manifest edit, a green conformance job and a traced spec update in the same change.
- `STA-V0-013`: `docs/SELF-DEVELOPMENT.md` gains a route for changes to these forms or integrations. The route uses `corvint affected`, the forms' focused tests and `make spec-toolchain-conformance`.
- `STA-V0-014`: A tracked symlink at the root `AGENTS.md` is a Git blob (mode 120000) holding the target path, not an authority document, and no consumer follows it. The authority-start query refuses with `unsupported-query-authority` and a message that names the symlink; Codex governance context reports `AGENTS.md` unavailable. Keep the guide a regular file (or make the symlink the secondary name, for example `CLAUDE.md` pointing at `AGENTS.md`). Resolving a tracked symlink to a tracked regular file in the same tree at the same revision is a post-1.0 idea (V1-1272, V1-1325, V1-2181).

- `STA-V0-965`: (experimental under the standing lane auto-accept, 2026-11-01; V1-3657) `script/spec-toolchain-conformance.sh` MUST select its Python interpreter before any other check: `SPEC_TOOLCHAIN_PYTHON` when set, used as given; otherwise the first of `python3`, `python3.14`, `python3.13`, `python3.12` and `python3.11` on `PATH` whose `sys.version_info` is at least 3.11. When none qualifies (or the override is older) it MUST exit 1 naming each candidate it tried and the `SPEC_TOOLCHAIN_PYTHON=/path/to/python3.11+` override. `--print-python` MUST print the selected interpreter and exit 0, running nothing else, so the selection is testable offline.

## Non-goals

- Bundling Node, Python, `openspec` or `specify` in, or calling them from, the Corvint binary.
- Authoring, editing, archiving or validating specs for the user. Upstream owns that.
- Corvint driving the toolchains' workflows, or any blocking hook.
- Moving Corvint's own `docs/specs/` onto either toolchain (decision 0540, decision 5).
- OpenSpec custom schemas other than the default `spec-driven` artifact shapes, Spec Kit presets that
  rewrite the spec template, and versions not in `STA-V0-001`.
- Spec Kit `tasks.md` (`T001`) or OpenSpec `tasks.md` items as obligations.

## Failure modes

- **Upstream changes a grammar.** The conformance job fails at the bump. Until then the manifest
  claims only the old version (`STA-V0-001`, `STA-V0-012`).
- **A requirement is renamed in OpenSpec.** The renamed requirement gets a new obligation ID. The
  delta form lists the rename in `excludedRequirements`, and the old map returns `intent-scope-mismatch`.
  That is visible staleness, not silent relocation; intent relocation stays the open item named in
  `docs/SPEC-TOOLCHAIN-INTEGRATION.md`.
- **Spec Kit's free-text status.** Any text other than an accepted-like word ranks non-binding
  (`documentAuthority`), so a mislabelled spec loses rank, never gains it.
- **Same-commit citation.** Same-commit spec and code keep the existing CEM two-commit or
  bootstrap-unknown rule.

## Acceptance evidence

- `internal/lrfrepo` tests named `TestSTAV0*` cover `STA-V0-002` to `STA-V0-006` on the
  `STA-V0-010` fixtures and hostile cases. They include default-form byte identity and an
  `ocm prepare`/`status` run on a Git fixture.
  Delivered: `TestSTAV0002_OpenSpecRequirementBlocks`, `TestSTAV0002_OpenSpecSpecRefusals`,
  `TestSTAV0003_OpenSpecDeltaSections`, `TestSTAV0003_OpenSpecDeltaRefusals`,
  `TestSTAV0004_NativeNamesPrepareAndStatus`, `TestSTAV0005_SpecKitRequirementLines`,
  `TestSTAV0005_SpecKitRefusals`, `TestSTAV0006_DeclaredFormsRegistry` and
  `TestSTAV0010_FixturesDerive`; in `cmd/corvint`, `TestSTAV0004_PrepareEnvelopeNativeMembers` and
  `TestSTAV0006_PrepareIntentFormFlag`.
- `internal/contextindex` tests cover `STA-V0-007`: `TestSTAV0007_ToolchainPathClassification` and
  `TestSTAV0007_NonToolchainPathsUnchanged`; `TestSTAV0014_SymlinkedRootGuideIsNotAuthority` covers `STA-V0-014`.
- `protocol/integrations` static tests cover the package shape and read-only commands:
  `TestSTAV0008_SpecKitExtensionManifest` and `TestSTAV0009_OpenSpecIntegration`.
- `internal/lrfrepo` covers `STA-V0-001`: the manifest shape, and the npm lock and hashed Spec Kit
  requirements naming the same releases and checksums.
  Delivered: `TestSTAV0001_VersionsManifest`.
- A passing `spec-toolchain-conformance` CI run covers `STA-V0-008` and `STA-V0-009` against the
  upstream CLIs, `STA-V0-010` and `STA-V0-011`.
  Delivered: `.github/workflows/spec-toolchain-conformance.yml` running `make spec-toolchain-conformance`
  (`script/spec-toolchain-conformance.sh`). It passed locally on 2026-10-18 with OpenSpec 1.14.3 and
  Spec Kit 1.2.1 over seven intent forms, one with out-of-section FR/SC items
  (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-sta-v0-conformance.md`); the first CI run is `NOT_RUN` until the
  workflow reaches GitHub.
- `TestSTAV0965_ConformancePicksAPython311` (`internal/lrfrepo`) covers `STA-V0-965` with stub
  interpreters on `PATH`: an old `python3` is skipped for `python3.12`, the override wins, and a
  host with no qualifying interpreter is refused naming `SPEC_TOOLCHAIN_PYTHON`.
- One `script/spec-toolchain-latest.sh` transcript covers `STA-V0-012`.
  Delivered: the 2026-10-18 transcript (`openspec 1.14.3 CURRENT`, `spec-kit 1.2.1 CURRENT`) in the
  build-log entry, and the offline cases in `script/spec-toolchain-latest_test.sh`
  (`make spec-toolchain-latest-test`).
- The route row in `docs/SELF-DEVELOPMENT.md` covers `STA-V0-013`.
  Delivered: "Change an OpenSpec or Spec Kit intent form or integration".

## Lanes

1. Intent forms and fixtures (`STA-V0-002` to `STA-V0-006`, `STA-V0-010`): `internal/lrfrepo/`,
   `cmd/corvint/ocm.go` help, `interop/spec-toolchains/fixtures/`.
2. Context classification (`STA-V0-007`, `STA-V0-014`): `internal/contextindex/`.
3. Host packaging (`STA-V0-008`, `STA-V0-009`): `protocol/integrations/`.
4. Conformance and versions (`STA-V0-001`, `STA-V0-011` to `STA-V0-013`): `interop/spec-toolchains/`
   (not fixtures), `Makefile`, `.github/workflows/`, `script/`, `docs/SELF-DEVELOPMENT.md`. Starts
   after lane 1's fixtures and forms land.

## Rollback

Roll back `STA-V0-965` by restoring `python=${SPEC_TOOLCHAIN_PYTHON:-python3}` and removing
`--print-python` and its test. Revert the lanes. Maps that declare these forms then fail `invalid-intent-form` on every OCM
action, fail-closed; remove them. The index falls back to `repository-spec` for the paths. Projects
remove the extension with `specify extension remove corvint` and drop the OpenSpec `rules` snippet.
