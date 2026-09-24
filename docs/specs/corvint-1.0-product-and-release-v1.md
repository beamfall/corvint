# Corvint 1.0 product and release scope V1

Owner: Russell Lewis
Date: 2026-09-22
Requirement prefix: `PRS-V1`
Status: accepted (decision 0373, 2026-09-23)
Intent status: accepted (decision 0373, V1-0001)
Delivery status: not-started
Authoritative inputs: `../../AGENTS.md` invariants 1-8, `public-release-v0.md` (the 0.6 local-workflow
scope, `PUB-V0-001..026` and the 2026-09-16 Core release scope amendment),
`../decisions/0332-verified-local-workflow-scope-2026-09-22.md`, `daily-change-evidence-workflow-v0.md`,
`agent-harness-integration-v0.md`, `../../integrations/compatibility.json`, `../RELEASE-NOTES.md`
(0.7.0 build 46), task-store tickets V1-0001..V1-0021.

## Agent digest
- Claim: Accepted scope: 1.0 Core is the local change-evidence loop and proof wire; companions, host FULL and independent interop leave the Core path (decision 0373).
- Status: accepted (decision 0373, V1-0001) / not-started
- Exists: this spec, decision 0373 with the eleven owner answers, and the prospective amendment section in `public-release-v0.md`; the candidate reader and installer admit a Core-only profile (V1-0125), but no assembler produces one yet.
- Blocked on: V1-0002 queue reconciliation and V1-0007 contract freeze; the untouched repository for V1-0019 and the native linux/amd64 host are not yet named.
- Read next: Decisions the owner must make; Classification of shipped surfaces; Externally dependent gates.

## Human intent

The owner asked for a 1.0 whose stability promise covers one thing that works end to end on the
owner's own machines: the local change-evidence loop. Earlier release scope (`public-release-v0.md`)
bundled optional companions, formal host authority and editor qualification into the release
path. Decision 0332 already cut those from the 0.6 milestone only. Decision 0373 makes that
cut permanent for 1.0, names every shipped surface's classification, and turns the gates that
need third parties into explicit owner choices rather than silent blockers.

Decision 0373 (2026-09-23) accepts every clause. `public-release-v0.md` carries the prospective
1.0 amendment, decision 0332 keeps governing the 0.6 milestone, and the task-store release criteria
are reconciled to this spec. Acceptance of scope is not delivery, qualification or promotion.

## Core definition

Core is the set of surfaces that 1.0 promises to keep stable, N-1 readable and qualified on the Core
platforms. It is exactly:

1. The local change-evidence loop: `init`, `adopt`, `index`, `query`, `context`, `impact`,
   `affected` and `prove`, run by one native Go binary against a local Git repository with no
   account, network, hosted service, database service, embeddings or daemon (invariant 7).
2. The proof wire: the CEM (`cem`), OCM (`ocm`) and change-frontier (`frontier`) profiles frozen
   by V1-0013, with canonical conformance vectors and the in-repo second consumer run by
   `make interop-gate`.
3. The dogfood loop: `dogfood` (and the internal `dogfood-record` and `dogfood-ocm` verbs it runs)
   producing a retained, explicit local outcome bound to a CEM.
4. The three Core jobs accepted by decision 0332: `UC-TASK-ORIENTATION`, `UC-CHANGE-CONSEQUENCE`
   and `UC-EVIDENCE-CARRYING-COMPLETION`. Every other headline job stays literally `UNPROVEN`.
5. The native release artifact, its reproducibility report, `SHA256SUMS`, and the install,
   upgrade, uninstall and recovery lifecycle of `stable-operations-v0.md`.

## Companions

A companion ships separately or alongside Core, carries its own qualification and version, and can
never be a prerequisite for a Core release candidate or a Core claim. Proposed companion list:

- Local admin console (`corvint-console`) and dashboard snapshot (`corvint-dashboard-snapshot`).
- Corvint Tasks (`corvint-tasks`, separate repository) and its roadmap.
- MCP servers: `corvint-mcp`, `corvint-docs-mcp`, `corvint-test-validity-mcp`.
- Test providers: `corvint-js-test-provider`, `corvint-go-test-provider`, and `corvint test-validity`.
- Automatic documentation: `corvint docs` drafting, watch and apply.
- Remote evidence provider (`corvint-remote-provider`): needs network consent, so it is outside the
  default local product by invariant 7.
- Host integrations other than the Core CLI rows in "Supported platforms and hosts": the Gemini
  CLI extension, the OpenCode plugin, the Pi extension, `corvint harness event` and `corvint pi-tool`.
- Formal host FULL or protected authority for any host (see "Externally dependent gates").
- The combined companion bundle `/2` (`corvint-companion-release`, `corvint-public-release-check`)
  and its installed qualification (`PUB-V0-020`, ticket V1-0004).

## Supported platforms and hosts

| Target | Proposed 1.0 label | Evidence today (0.7.0 build 46) |
|---|---|---|
| darwin/arm64 native binary | Core | Archive built twice byte-identically; install lifecycle and hostile matrix passed on darwin/arm64. |
| linux/amd64 native binary | Core, pending native evidence | Archive built and reproducible; install lifecycle `NOT_RUN`. Core label needs a native run (owner question 4). |
| linux/arm64 native binary | FALLBACK | Archive built; lifecycle `NOT_RUN`; 0.6.0 notes record environment-dependent native test failures. |
| darwin/amd64 native binary | FALLBACK | Archive built; lifecycle `NOT_RUN`. |
| windows/amd64 | UNSUPPORTED | Producer emits a zip that is not a qualified target; `PUB-V0-022` keeps Windows out of the candidate. |

FALLBACK means the archive is published and checksummed but carries no platform qualification and
no stability promise beyond Core's wire contracts. A FALLBACK platform can become Core only with its
own retained install-lifecycle and focused-test evidence.

Host rows use the existing tuple `(host, surface, host version, adapter version, OS)`. Every tuple
in `integrations/compatibility.json` is FALLBACK today. Proposed 1.0 host scope:

| Host tuple | Proposed 1.0 label | Reason |
|---|---|---|
| Plain CLI on a Core platform | Core | The loop is fully usable from a shell with no host adapter. |
| Codex CLI via the Codex plugin (`corvint adapter codex`), exact version | Core row at FALLBACK (decision 0373, question 5: yes) | Decision 0332 precedent: local Codex use qualified on an exact version without FULL authority. |
| Claude Code via the Claude Code plugin (`corvint adapter claude-code`), exact version | Core row at FALLBACK (decision 0373, question 5: yes) | Same precedent; installed lifecycle already ran on Claude Code 2.1.267. |
| Gemini CLI, OpenCode, Pi | Companion, FALLBACK | No retained installed-loop evidence that Core could own. |
| Codex Desktop, protected Pi, `/Library/CorvintAuthority` hook trees | post-1.0 | Need formal FULL authority the host API does not currently supply to Core. |
| VS Code extension | deferred | `vscode-extension-v0.md` is deferred. |

## Classification of shipped surfaces

Labels: Core (stability promise), companion (optional, own qualification), experimental (shipped,
no promise, labelled or hidden by V1-0007), deferred (not shipped for 1.0, direction kept),
rejected (will not ship as specified), post-1.0 (intended after 1.0, no 1.0 claim).

| Surface | Label | One-line reason |
|---|---|---|
| `corvint init`, `adopt` | Core | First-run inventory and receipt; loop entry (V1-0008). |
| `corvint index` | Core | Deterministic immutable snapshot every Core read depends on. |
| `corvint query` | Core | Cited retrieval over the pinned snapshot. |
| `corvint context` | Core | Task context packet; `UC-TASK-ORIENTATION`. |
| `corvint impact`, `affected` | Core | Change consequence and verification selection; `UC-CHANGE-CONSEQUENCE`. |
| `corvint prove` | Core | Proof over pinned evidence; `UC-EVIDENCE-CARRYING-COMPLETION`. |
| `corvint cem`, `ocm`, `frontier` | Core | The proof wire to be frozen by V1-0013. |
| `corvint dogfood`, internal `dogfood-record`, `dogfood-ocm` | Core | Retained local outcome of the dogfood loop. |
| `corvint adapter` (codex, claude-code, claude-source-handoff) | Core (decision 0373, question 5: yes) | Thin host entry for the two Core host rows. |
| `corvint --version`, `help` | Core | Identity and discovery of the Core verbs. |
| `corvint docs` (draft, watch, apply, maintain) | companion | Decision 0332 names automatic docs a companion. |
| `corvint test-validity` | companion | Test-provider profile; decision 0332 companion. |
| `corvint harness event`, `pi-tool` | companion | Entry points for non-Core hosts. |
| `corvint witness`, `batch`, `obligations` | experimental | Accepted directions with experimental delivery; not part of the Core loop. |
| `corvint eval`, `lrf`, `calibrate` | experimental | Evaluation and ranking research surfaces. |
| `corvint record`, `migrate-traces`, `migration-ratchet` | experimental | Learning and trace migration stay evaluation-gated (invariant 5). |
| `corvint work`, `observations`, `prove-observe`, internal `dogfood-observe` | experimental | Observation ledgers; never inputs to ranking or authority (invariant 4). |
| `corvint features`, `overview`, `review` | experimental | First-use guidance already labelled experimental (`PUB-V0-017`). |
| `corvint flows` | experimental | Application-flow understanding is an experimental slice (V1-0100). |
| `corvint depsource`, `necessity`, `surprise`, `answerability`, `kernel`, `lease`, `reads`, `skill-export` | experimental | Help text already labels them experimental. |
| `corvint feature` | experimental | Legacy parity verb kept for the Go kernel migration. |
| Internal `authority-event`, `qualified-event`, `native-hook`, `frontier-next`, `plan-fixture`, `source-view`, `docs corpus` | experimental | Protected-authority, planning, source-view and corpus prototypes. |
| `corvint-console`, `corvint-dashboard-snapshot` | companion | Optional operator console with no authority (decision 0081). |
| `corvint-tasks` | companion | Separate repository; roadmap is not a Core claim. |
| `corvint-mcp`, `corvint-docs-mcp`, `corvint-test-validity-mcp` | companion | MCP servers are optional host surfaces. |
| `corvint-js-test-provider`, `corvint-go-test-provider` | companion | Test providers qualify under `/2`, not Core. |
| `corvint-remote-provider` | companion | Network consent places it outside the default local product. |
| `corvint-release-candidate`, `corvint-release-install` | Core | Core candidate assembly and install (`PUB-V0-022..026`). |
| `corvint-companion-release`, `corvint-public-release-check` | companion | Build and qualify the companion bundle `/2`. |
| `corvint-release-gate` | experimental | Self-described experimental offline evidence gate. |
| `corvint-corpus-mcp`, `corvint-behavior-falsify`, `corvint-playwright-minimize`, `corvint-web-flows`, `corvint-pulse`, `corvint-work-queue` | experimental | Prototypes with proposed or experimental specs. |
| `corvint-analyzer-*` external analyzer binaries and `corvint-go-toolchain-receipt` | deferred | Analyzer candidate profiles are deferred; Core analysis is the in-binary kernel. |
| `corvint-analyzer-structured-data` | rejected | Its candidate spec is rejected as specified. |
| Claude Code plugin, Codex plugin trees | Core (decision 0373, question 5: yes) | Carry the hooks for the two Core host rows. |
| Codex `authority-hooks`, `qualified-hooks`, `qualified-direct-hooks` proposed trees | post-1.0 | Require formal host authority. |
| Gemini CLI extension, OpenCode plugin, Pi extension | companion | FALLBACK hosts outside Core. |
| `integrations/pi-protected` | experimental | Protected Pi runtime is an accepted direction with experimental delivery. |
| `extensions/vscode` | deferred | `vscode-extension-v0.md` is deferred. |
| `integrations/testfixture` | none (test-only) | Test fixture tree; never shipped. |
| DeepSeek Harness plugin | post-1.0 | A committed product goal that is not shipped. |
| Hosted service, embeddings, permanent daemon, database service in the default binary | rejected | Invariant 7. |

## Externally dependent gates

Each gate below needs something the owner cannot produce alone today. For each, the disposition
accepted by decision 0373 and the amendment it requires are stated.

### V1-0014: independent proof producers and consumers

- Today: the task-store v1-0 criterion and V1-0021 require that independent interoperability
  passes. Only the in-repo second consumer (`interop/cem01-go`, `make interop-gate`) exists.
- Proposed disposition: post-1.0. Corvint 1.0 MUST NOT claim that CEM, OCM or frontier are
  interoperable. Core keeps the frozen wire, canonical vectors and `make interop-gate`.
- Amendment required: none to `PUB-V0` requirements, which do not require independent producers.
  After acceptance, the coordinator amends the task-store v1-0 release criterion and the V1-0021
  acceptance criterion to replace "independent interoperability passes" with "the frozen wire's
  canonical vectors and in-repo second consumer pass; no interoperability claim". V1-0015, which
  depends on V1-0014, is already COMPLETED; its recorded dependency edge stays and is moot for the
  Core path (the store refuses `set-dependencies` on completed tickets).

### V1-0019: untouched-repository validation

- Today: the three Core workflows must pass on Corvint, Beamfall and at least one untouched
  external repository, with cases frozen before execution.
- Proposed disposition: keep as a Core blocker, in an owner-closable form. The owner selects one
  public repository not used in Corvint development, freezes the cases before execution, and runs
  them. No third party is needed.
- Amendment required: add to `public-release-v0.md` (0.6 local-workflow scope, extended to 1.0)
  one sentence: "The 1.0 Core candidate additionally requires the three Core jobs on one
  owner-selected untouched public repository with cases frozen before execution." The ticket
  type of V1-0019 changes from EXTERNAL to MANUAL.

### Host FULL and authority tuples

- Today: the 2026-09-16 Core release scope amendment says Codex CLI and Desktop, Claude Code,
  Gemini CLI and Pi "still require exact runtime FULL/authority evidence" and that missing
  "required exact runtime FULL" evidence blocks release. V1-0005 and V1-0016 carry this.
- Proposed disposition: move formal FULL and protected authority to post-1.0 (or to a companion
  host profile). Core claims the plain CLI and (question 5: yes) installed Codex CLI and
  Claude Code at FALLBACK on exact versions. Tuples whose host API cannot supply authority stay
  FALLBACK or UNSUPPORTED and do not block Core.
- Amendment required: in `public-release-v0.md`, extend the 0.6 local-workflow scope sentence
  "For that milestone only, it supersedes conflicting prerequisites that require optional
  companions or formal FULL host authority" to cover the 1.0 Core candidate, and state that the
  2026-09-16 clause "still require exact runtime FULL/authority evidence" binds only a companion
  host profile, not the Core release.

### Companion installed qualification (`PUB-V0-020`, V1-0004)

- Today: `PUB-V0-020` and the 2026-09-16 amendment make the `/2` installed run (JS unit and E2E,
  Go foreground, test-validity MCP, docs, Tasks roadmap, named-browser console) Core installed
  qualification.
- Proposed disposition: move to companion. Core installed qualification becomes the native archive
  lifecycle of `stable-operations-v0.md` plus the three Core jobs run on the exact installed bytes.
- Amendment required: `PUB-V0-004`, `-005`, `-006`, `-009`, `-010` and `-020`, `PUB-V0-022`'s
  companion input, and the 2026-09-16 clause "missing core installed ... evidence blocks release"
  bind only the companion profile for 1.0. The existing combined manifest reader and installer do
  not admit a Core-only packet, so a Core-only candidate path is an implementation follow-up under
  V1-0007 or V1-0018.

## Requirements

- `PRS-V1-001`: Corvint 1.0 Core MUST be exactly the surfaces labelled Core in this spec: the local
  change-evidence loop, the CEM/OCM/frontier proof wire, the dogfood loop, the three Core jobs of
  decision 0332, and the native release artifact and its lifecycle.
- `PRS-V1-002`: Only Core surfaces carry the 1.0 stability promise (frozen contracts, N-1 readers
  or deterministic migrations). Companion, experimental, deferred, rejected and post-1.0 surfaces
  MUST NOT be described as stable in release notes, help or product docs.
- `PRS-V1-003`: Every gate on the Core critical path to 1.0 MUST be closable by the owner without a
  third party, or be explicitly kept by an owner answer in "Decisions the owner must make".
- `PRS-V1-004`: Core platforms MUST be darwin/arm64 and linux/amd64 native binaries. A Core
  platform needs retained native install-lifecycle and focused-test evidence on the candidate
  bytes; without it the platform is reported FALLBACK, never Core. Windows MUST stay UNSUPPORTED.
- `PRS-V1-005`: A Core release candidate MUST NOT require companion input. Companions keep their
  own qualification and version and can be released separately.
- `PRS-V1-006`: Core host scope MUST be the plain CLI, plus the exact-version Codex CLI and Claude
  Code rows at FALLBACK when owner question 5 is answered yes. Formal FULL or protected authority for
  any host MUST NOT block Core.
- `PRS-V1-007`: Corvint 1.0 MUST NOT claim CEM, OCM or frontier interoperability unless V1-0014
  passes with at least one non-Corvint producer and two independent consumers.
- `PRS-V1-008`: The 1.0 Core candidate MUST pass the three Core jobs on Corvint, Beamfall and one
  owner-selected untouched public repository, with cases frozen before execution.
- `PRS-V1-009`: Core installed qualification MUST be the native archive lifecycle of
  `stable-operations-v0.md` plus the three Core jobs on the exact installed bytes. It MUST NOT
  depend on the companion bundle `/2`.
- `PRS-V1-010`: Any new shipped verb, binary or integration tree MUST be added to the
  classification table with one label and a one-line reason in the same change that ships it.
- `PRS-V1-011`: This spec amends `public-release-v0.md` prospectively for the 1.0 Core candidate. It
  MUST NOT rewrite historical alpha, 0.5, 0.6 or 0.7 evidence, which keeps its original meaning.
- `PRS-V1-012`: This spec has no authority until a numbered decision records the owner's
  acceptance and answers. Until then `public-release-v0.md`, decision 0332 and the task-store
  release criteria govern.

## Decisions the owner must make

Answered on 2026-09-23: yes to all eleven, recorded with notes in
`../decisions/0373-corvint-1.0-scope-ratified-2026-09-23.md`. The linux/amd64 host (question 4)
and the untouched repository (question 8) are still unnamed. The questions stay as asked.

1. Accept the Core definition (`PRS-V1-001`) and the companion list as written?
2. Accept the classification table as the 1.0 disposition of every shipped surface (`PRS-V1-010`)?
3. Make darwin/arm64 a Core platform and linux/arm64 and darwin/amd64 FALLBACK (`PRS-V1-004`)?
4. Make linux/amd64 the second Core platform, given that you can supply a native linux/amd64 host
   for the lifecycle run? (If no, should linux/arm64, where native runs have occurred, take its
   place?)
5. Keep installed Codex CLI and Claude Code local use at FALLBACK on exact versions as Core host
   rows (`PRS-V1-006`, decision 0332 precedent)?
6. Move formal host FULL and protected authority off the Core path to post-1.0 or a companion
   profile, with the `public-release-v0.md` amendment stated above?
7. Move V1-0014 independent interoperability to post-1.0 and withhold any interoperability claim
   from 1.0 (`PRS-V1-007`)?
8. Keep V1-0019 as a Core blocker in the owner-closable form (one owner-selected untouched public
   repository, cases frozen first) (`PRS-V1-008`)?
9. Move `PUB-V0-020` and V1-0004 (companion `/2` installed qualification) to the companion profile
   (`PRS-V1-009`)?
10. Amend `public-release-v0.md` prospectively rather than supersede it (`PRS-V1-011`)?
11. Record acceptance in a new numbered decision, after which the coordinator amends the task-store
    v1-0 criteria and V1-0019, V1-0021 and V1-0015 to match?

## Non-goals

- Delivering anything. Accepted scope is not qualification, a candidate, or promotion.
- Changing any `PUB-V0` requirement text; the amendment is prose in `public-release-v0.md`, like the
  2026-09-16 Core release scope amendment.
- Promoting any host, platform or companion, or rewriting historical evidence.
- Adding a new spec language, runtime, service or dependency.
- Choosing the untouched repository for V1-0019 or the linux/amd64 host.
- Implementing the Core-only candidate path; that is a follow-up once accepted.

## Failure modes

- A shipped surface missing from the classification table: default to experimental and treat it as
  a `PRS-V1-010` violation to fix before the candidate freeze.
- A Core platform without native evidence at candidate freeze: the platform is reported FALLBACK
  and the release notes say so (`PRS-V1-004`).
- An owner answer of "no" on questions 6, 7 or 9: the current `public-release-v0.md` rule stays a
  Core blocker, and V1-0018 must list it as an open blocker rather than drop it.
- Release notes or help describing a non-Core surface as stable: a `PRS-V1-002` defect.

## Acceptance evidence

- Owner acceptance: `../decisions/0373-corvint-1.0-scope-ratified-2026-09-23.md` answers every
  question above and references this spec. Status: PRODUCED (2026-09-23).
- Document consistency: the focused-docs gate passes with this spec, its `INDEX.json` row, its
  `README.md` row and regenerated `REQUIREMENTS.tsv`.
- Classification completeness: every top-level `corvint` verb, `cmd/` binary, `integrations/` tree and
  `extensions/` tree present at the base commit appears in the classification table. Checked by
  review at acceptance; a mechanical check is a follow-up for V1-0002 or V1-0007.

## Traceability

| Requirement | Evidence | Status |
|---|---|---|
| `PRS-V1-001..012` | Decision 0373 | PRODUCED (2026-09-23) |
| `PRS-V1-010` | Review of this table against the base commit's verbs, binaries and trees | review only |
| `PRS-V1-002` | N-1 upgrade: 0.7.0 archive to 0.8.0 `upgrade-b` and `rollback-a` under `SOP-V0-003` (`stable-operations-v0.md`, V1-0190) | PASS on darwin/arm64, darwin/amd64 (Rosetta 2), linux/arm64 (container); linux/amd64 emulated only |
| `PRS-V1-004` | Native install lifecycle on each Core platform | darwin/arm64 retained for 0.7.0; linux/amd64 `NOT_RUN` |
| `PRS-V1-006` | Host lifecycle qualification, nine cases per tuple (`host-lifecycle-qualification-v1.md`, V1-0016) | PASS for plain CLI, Codex CLI 0.153.2 and Claude Code 2.1.267 on darwin/arm64 with 0.8.0, all FALLBACK; linux `NOT_RUN` |
| `PRS-V1-007` | V1-0014 | `NOT_RUN` |
| `PRS-V1-008` | V1-0019 | `NOT_RUN` |

## Sections not applicable to a scope spec

- Simpler baseline: not applicable; this spec selects scope and does not add a mechanism.
- Trust boundary and resource limits: not applicable; every named surface keeps its owning spec's limits.
- Rollout, compatibility and drift rules: not applicable; the owning contracts and `CCF-V1` carry them.
- Promotion or kill criteria: not applicable; acceptance is the owner decision `PRS-V1-012` records.

## Rollback

Delete this file, its `INDEX.json` entry and its `README.md` row, regenerate `REQUIREMENTS.tsv`,
and remove the "1.0 Core scope amendment" section in `public-release-v0.md` and the pointer sentence
in `../PRODUCT.md`. Decision 0373 and ticket V1-0001 depend on this spec; that decision's rollback
section governs reverting the acceptance.
