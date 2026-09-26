# Core Compatibility Freeze V1

Owner: Russell Lewis
Date: 2026-09-22
Requirement prefix: `CCF-V1`
Intent status: proposed overall; accepted CCF-V1-006/CCF-V1-007 amendments (decision 0401)
Delivery status: experimental
Authoritative inputs: ticket V1-0007, accepted decision 0332 (the Core set), decision 0358, `AGENTS.md` invariants 1, 2, 4 and 8,
`conformance/cli-parity-v0/manifest.json`, and the owning specs of each Core verb listed in CCF-V1-002.

## Agent digest
- Claim: The twelve Core verbs of decision 0332 keep their command modes, wire profiles, error envelope and state readers compatible from 0.8.1 to 1.0.
- Status: proposed overall; accepted CCF-V1-006/CCF-V1-007 amendments (decision 0401); experimental delivery; the Core set is taken from accepted decision 0332, this contract awaits owner ratification in V1-0001
- Exists: this contract, decision 0358, the root-help `Command maturity:` section (`commandMaturityHelp`), `cmd/corvint/core_freeze_test.go` and its per-mode goldens in `cmd/corvint/testdata/core-freeze/`
- Blocked on: V1-0001 owner ratification of this contract and of the proposed B5, V1-0284 and decision 0398 amendments to CCF-V1-004, and of the proposed V1-0350 register rows and N-1 replay placement in CCF-V1-007; pinned modes for the mutating `cem`, `ocm` and `dogfood` subcommands are NOT_PRODUCED; the exhaustive gate is NOT_RUN
- Read next: Requirements; Breaking-change rule; Traceability

## Intent and scope

Corvint 1.0 needs a stated compatibility boundary so that an agent or CI job written against one
release keeps working on the next. Before this contract the boundary existed only implicitly: byte
parity cases for four verbs, a handful of in-repository readers that match profile strings exactly,
and a help footer saying that commands marked Experimental carry no stability promise, which did not
mark most verbs either way. This contract names the Core verbs, lists what about them is frozen,
states the rule that decides whether a change is breaking, and states how state and profile changes
reach users. It freezes what 0.8.1 already emits; it changes no verb's runtime behaviour or wire
output. Everything not listed stays experimental, so no companion or research profile becomes
frozen by omission.

The N-1 baseline is the published 0.8.1 release (tag `v0.8.1`, commit 0e5d596), per CCF-V1-007.
(Amended 2026-09-25, panel blocker B6; accepted, decision 0401.) The contract was first written against 0.7.0
(tag `v0.7.0`, commit 678c1b1) when `git diff v0.7.0 1894b9e -- cmd/corvint internal` was empty,
but 0.8.0 (516439f) and 0.8.1 shipped it after that premise stopped holding; see CCF-V1-007.

## Requirements

- **CCF-V1-001:** The Core command set MUST be exactly `init`, `adopt`, `index`, `query`, `context`,
  `impact`, `affected`, `prove`, `cem`, `ocm`, `frontier` and `dogfood`. The set is the accepted
  decision 0332 (`docs/decisions/0332-verified-local-workflow-scope-2026-09-22.md:21@17ce43a4`), which
  names Core contracts for these verbs and, for `dogfood`, only its retained local outcome. Project
  authority outranks the ticket's earlier seven-verb assumption (AGENTS.md invariant 3). A change to the
  set changes this requirement and the `frozenCoreVerbs` list in the same change. `feature` and
  `dogfood-ocm` are not Core.
- **CCF-V1-002:** Each frozen Core mode MUST exit 0 and emit one canonical JSON document on stdout
  with `ok=true` and `mutates=false`, unless its row says otherwise, and the identifiers below, byte
  for byte:

  | Verb and mode | Identifier members |
  |---|---|
  | `init` / `adopt`, default and `--full-receipt` | `tool` = the verb; `inventory.profile` = `genesis-inventory/0.1-experimental`; default mode also `inventory.summaryProfile` = `genesis-inventory-summary/0.1-experimental` |
  | `query --task` (repository and authority-start tasks) | `tool=query`; `context.schema_version` = 1; `context.mode` = `query`; the authority-start task has `context.intent.id` = `project-operations` |
  | `context --task` | `tool=context`; top-level `schema_version` = 1 |
  | `impact PATH...` | `tool=impact`; `context.schema_version` = 1; `context.mode` = `impact` |
  | `impact --base FULL_COMMIT_ID` | `context.profile` = `corvint-range-impact/0`; `context.mode` = `range-impact` |
  | `impact --working-tree-untracked PATH...` | `context.profile` = `corvint-working-tree-impact/0` |
  | `affected`, default and `--base FULL_COMMIT_ID` | `tool=affected`; `profile` = `affected-plan/0` |
  | `prove --task`, `prove PATH...`, `prove --base FULL_COMMIT_ID` | `tool=prove`; `profile` = `falsifiable-packet/0`; with `--base`, `packet.profile` = `corvint-range-impact/0` |
  | `index` | `command=index`; `profile` = `corvint-index-snapshot/1`; `mutates=true` (the snapshot write) |
  | `index --if-stale` with a fresh snapshot | `state=fresh`, `mutates=false`; no `ok`, `profile` or `command` member |
  | `cem status` / `cem verify` on a committed map | `tool` = `cem-status` / `cem-verify`; `verification.spec` = `cem/0.2` |
  | `ocm status` / `ocm verify` | `tool` = `ocm-status` / `ocm-verify`; `verification.spec` = `ocm/0.1-experimental` |
  | `frontier --json` | exit 0 for an empty and 1 for an open frontier; `profile` = `frontier/0`; `frontierState`; no `ok` or `mutates` member |
  | `dogfood status` (the retained local outcome) | `tool=dogfood-status`; `profile` = `corvint-local-completion/0`; `claim` = `caller-owned-selected-workflow-only` |

  The genesis identifiers keep their `0.1-experimental` spelling: the bytes are frozen, not the word,
  and the `receiptId` digest domain `atlas-genesis-inventory/0.1-experimental`
  (`internal/genesis/inventory.go:257@744add89`) is frozen with them because changing it changes every receipt id.
  The CEM, OCM and frontier document schemas stay governed by their owning specs (CEM-CB, OCM-V0,
  CF-V0); this contract neither restates nor changes those schemas.
  (proposed, decision 0398; this replaced "pins only the identifiers above".) Each mode row above also
  has one structural golden, `cmd/corvint/testdata/core-freeze/<case>.json`, one file per
  `TestCoreVerbsEmitTheFrozenProfiles` case, captured from the current binary over that case's
  fixture. It holds the full member tree, every array length and every value that is equal across two
  independent fixture builds. A value that differs between those builds is pinned by JSON type only, as
  `"<varies:TYPE>"`: fixture commit ids, temporary paths, and digests over them. A member set, array
  length or type that differs between the builds cannot be frozen and fails generation. The test
  compares each document with its golden. A mismatch is not breaking in itself, because CCF-V1-006
  still decides that, but a compatible change regenerates the golden with `CORVINT_UPDATE_GOLDEN=1` in
  the same change, so review sees every wire difference.
  NOT_PRODUCED: no pinned mode for `cem begin`, `prepare`, `cite`, `mark`, `report`, `cover`,
  `discriminate`, `anchor` and `provenance`; `ocm prepare`, `link`, `mark` and `report`; the frontier
  human rendering and dynamic test mode; and `dogfood begin`, `verify`, `finish`, `review`,
  `handoff` and `cancel`. They are Core verbs' modes without a frozen identifier until a later change
  pins them with a test.
- **CCF-V1-003:** These modes and profiles MUST stay outside the freeze and MUST NOT be advertised as
  Core compatibility: `impact --provider*`, `--repository` and `--range-profile expanded-256`;
  `affected --snapshot`, `--playwright-config`, `--provider*` and `--selection-profile`; `prove --cem`,
  `--attest*`, `--verify-cem-attestation`, `--checkpoint` and `--mutate`; `context` lookup subcommands
  (for example `context authority`); and the profiles `external-evidence-provider/*`,
  `external-test-selection/0`, `playwright-affected/0`, `corvint-planning-snapshot/0`,
  `corvint-checkpoint/0` and in-toto statements. Their owning specs govern them. The verbs
  `native-hook`, `authority-event` and `qualified-event` are undocumented adapter plumbing that
  `runContext` dispatches before the `topLevelCommands` check (`cmd/corvint/main.go:803@e2ed60e2`); they
  are absent from root help and outside the freeze. (proposed, decision 0398) The `cem/0.3` profile and the
  modes that write it, `cem cover`, `cem discriminate` and `cem mark` with a structural reason, are
  experimental and outside the freeze, not among the Core modes listed under CCF-V1-002; they never
  write over the Core sidecar `.corvint/change.cem.json` or their `cem/0.2` input (`CEM-SM-006`).
- **CCF-V1-004:** A Core refusal MUST exit 2 with empty stdout and exactly one stderr JSON line built by
  `emitError` (`cmd/corvint/main.go:1346@40010ccd`): `code`, `error` and `ok=false`, plus the DRC-V0-006
  diagnostic members `subject`, `evidence`, `supported_fixes` and optional `terminal` where the site was
  converted. Frozen code families are `invalid-*` (argument, revision and repository-root validation),
  `unsupported-*` (a well-formed request outside the qualified profile, including
  `unsupported-impact-range`, `unsupported-query-*`, `unsupported-prove-*`, `unsupported-affected-*`,
  `unsupported-working-tree-impact-*`), `repository-*`, `output-failed` and `internal-error`. The
  codeless envelope `{"error","ok":false}` that some index refusals emit (for example `impact` of an
  untracked path) is frozen as it is. `init` / `adopt` with an unusable revision instead print the
  inventory with `ok=false` and `operationalState=INVALID` on stdout and exit 1. Renaming or removing a
  code, adding `code` to the codeless envelope, or moving a refusal between exit classes is breaking.
  (proposed, decision 0398) Adding an optional `code` member to a codeless refusal is compatible, as
  adding an optional member is under CCF-V1-006: `error`, the exit class and the empty stdout stay
  unchanged, and a reader that ignores `code` sees the earlier envelope. This replaces the clause
  above that made adding `code` to the codeless envelope breaking; renaming or removing a code once
  emitted stays breaking. `emitError` therefore emits the `repository-*` and
  `unsupported-git-object-format` codes of a kernel refusal, and `cem` emits `patch-unavailable` and
  `map-unavailable` beside the fixed `cannot read patch` and `cannot read CEM map` text. The two
  pinned `cli-parity-v0` cases this reaches, `cem-begin-unreadable-patch` and `cem-unreadable-map`,
  are recorded as `DR-0041`. A codeless context-index refusal (for example `impact` of an untracked
  path) keeps its envelope.
  (proposed 2026-09-25, panel blocker B5, not accepted) Exactly three Core refusals are exempt from
  the `emitError` envelope, and each keeps its own frozen shape: (a) `frontier` emits the
  `frontier-error/0` document `{"code","profile"}` with no `ok` member, frozen by CF-V0-034 and
  decision 0357, so it is carved out rather than projected; (b) `dogfood` emits
  `{"code","error":{"code","message"},"ok":false}`, where the top-level `code` is the member every
  Core refusal carries and the nested object stays for readers of the earlier envelope, and a
  lifecycle refusal after the policy loaded also writes the `ok=false` policy document on stdout;
  (c) the `init` / `adopt` INVALID inventory above, which also covers a working directory that is not
  a repository root. Every Core verb that reads the repository refuses an omitted `--root` exactly as
  it refuses an explicit one: `invalid-arguments` with `supported_fixes`
  `cli.use-git-repository-root`, the message `not a Git repository: DIR` outside any repository, and
  inside one the message `not the repository root; top level is TOP` with `evidence` `top_level`.
  A repository whose `HEAD` names no commit is refused with `repository-head-unborn`, `subject`
  `repository-state` / `head-commit` and `supported_fixes` `git.create-head-commit`. Before rc.1
  these deliberately add `code` to the formerly codeless unborn-`HEAD` and outside-repository
  `impact` refusals and replace `repository-probe-failed` / `unsupported-prove-history` for a
  non-root working directory; no `cli-parity-v0` case covers those inputs, and the replay is unchanged.
  (proposed 2026-09-26, V1-0284, not accepted) `cem`, `ocm` and `frontier` judge their
  root-relative maps before any repository check, so a non-root working directory gets that verb's
  map refusal, the same for an omitted and an explicit `--root`, rather than `invalid-arguments`.
  No Core read fetches (invariant 7): every Git process `index`, `query`, `context`, `impact` or
  `prove` starts for its reads carries `GIT_NO_LAZY_FETCH=1` and an empty `GIT_ALLOW_PROTOCOL`. Git
  honours the first for direct object reads from 2.45 and for a diff's blob prefetch from 2.46; the
  second refuses every transport on any Git, so a fetch an older Git starts anyway never reaches the
  remote. When the tree or range read of those verbs fails because an object it needs exists only on
  a partial clone's promisor remote, the refusal is `repository-object-unavailable`, the CEM-CB-019
  code for a missing promised object, with `subject` `repository-state` / `promisor-object`, one
  `evidence` pair `object`, and `supported_fixes` `git.fetch-promisor-objects`: the user fetches the
  objects. The named object is a missing promisor object the failed read itself named, in Git's
  error, as the blob whose size it could not report, or as a tree it read; a failure that names none
  keeps its original refusal. Before rc.1 this deliberately replaces the codeless `Git returned an
  invalid blob size` and `Git error: ...` refusals for that input; no `cli-parity-v0` case covers a
  partial clone.
- **CCF-V1-005:** The admission, freshness, omission and abstention members MUST keep their names,
  JSON types and meaning. `query` and path `impact`: `context.state`, `context.freshness.{state, scope,
  revision, mixed_path_count, mixed_paths}`, `context.coverage.{requested_results, included_results,
  omitted_results, critical, critical_missing, uncertainty, within_budget, budget_bytes,
  packet_bytes, authoritative_results, advisory_results}` and `context.exclusions.{count, samples}`;
  `query` also `context.abstention.{active, reason}`, `context.intent` and `context.learning`;
  range `impact` also `context.range` and `context.omissions.{count, samples, sha256}`. `context`:
  `state` and `coverage.{candidates, included_results, omitted_results, critical, critical_missing,
  unexamined, budget_shortage, governance, governance_refused, answerability}`. `init` / `adopt`:
  `inventory.operationalState`, `gaps`, `denominator`, `dirtyState`, `semanticFrontier` and
  `samplesTruncated`. `affected`: `plan.scope`, `plan.unknown`, `plan.excluded`, `advice.status` and
  `provider.go.state` (AFP-V0-003/004). `prove`: `state` and `proof.{counts, proven_results,
  unproven_results, failed_results}` with every row's falsifier verdict (FPK-V0). A frozen
  enumeration may gain a value only under CCF-V1-006. For `index`, `cem`, `ocm`, `frontier` and
  `dogfood status` this contract lists no further members: NOT_PRODUCED; their owning specs
  (IDX-SNAP-V0, CEM-CB, OCM-V0, CF-V0, LCP-V0) govern them. (proposed, decision 0398) The CCF-V1-002
  goldens still pin the names and JSON types of every member these modes emit over the test fixtures.
- **CCF-V1-006:** Breaking-change rule. For output pinned by `conformance/cli-parity-v0/manifest.json`
  (`query`, `impact`, `init`, `adopt`), any stdout or stderr byte change is breaking unless recorded in
  `conformance/divergence-register.md` with a decision. For every other frozen mode: adding an optional
  member is compatible; removing, renaming or retyping a member, changing a CCF-V1-002 identifier value,
  adding a value to a `closed` row of the CCF-V1-007 (d) enumeration register (accepted 2026-09-25, decision 0401;
  panel blocker B6; it replaced "a frozen enumeration a reader must branch on"),
  removing or renaming any registered value, changing the default mode, or making a read mutate state
  is breaking. A breaking change MUST ship a new profile version (for example
  `affected-plan/1`), an N-1 reader per CCF-V1-007, a decision record, and an update to this contract
  and its test in the same change.
- **CCF-V1-007:** (accepted 2026-09-25, decision 0401; panel blocker B6) N-1 and migration policy. N-1
  is the newest release tag on `main`, currently 0.8.1 (tag `v0.8.1`, commit 0e5d596). It is not 0.7.0:
  0.8.0 (516439f) and 0.8.1 both shipped this contract, and between `v0.7.0` and `v0.8.1` 170 files
  under `cmd/corvint` and `internal` changed. One change reached a frozen member. Commit 2f3bfe6
  (V1-0186) added the relation `instruction-routed` to `context`'s `coverage.unexamined`. That
  relation list has twelve entries at `v0.7.0` (line 177 of `git show v0.7.0:internal/contextindex/taskcontext.go`) and thirteen
  in `cmd/corvint/testdata/context-default-wire.golden` at `v0.8.0` and `v0.8.1`. That change shipped
  before this contract was accepted. It is part of the 0.8.1 baseline, and it is compatible under
  (d)'s `open` status. Every Core reader MUST accept the documents and state that N-1 wrote, and
  every state change on the path to 1.0 MUST have a deterministic migration or rebuild:
  (a) derived state, the index snapshot
  `corvint-index-snapshot/1`, treats any format, engine, tree or decode mismatch as a miss and
  rebuilds (IDX-SNAP-V0-003), so no migration exists or is needed; (b) durable local traces
  (`corvint-local-trace/1`) keep the bounded legacy tree-row reader and the explicit, digest-bound
  `migrate-traces --dry-run` / `--apply --plan-digest` migration (LTPM-V0); (c) in-repository readers
  of Core profiles, `prove-observe` (`cmd/corvint/prove_observe.go:90@e92cefdb`), `internal/attest` and
  `internal/companionrelease` core smoke, match the current identifier exactly and MUST accept both N
  and N-1 identifiers in the change that bumps one. The CCF-V1-002 identifiers are unchanged from
  0.8.1: `cmd/corvint/core_freeze_test.go` is byte-identical at `v0.8.1` and at base 489701ca.
  (d) Frozen enumerations are exactly the rows below, keyed by member path and the document's
  `tool`. Each row lists every value that its cited source writes. The
  sources have the same values at this change and at `v0.8.1`, and `git diff v0.8.1` of the cited
  files changes none of these strings. In a `closed` row, a reader may branch exhaustively, so
  adding a value is breaking under CCF-V1-006. In an `open` row, a reader MUST treat an unknown value
  as informational: skip an `unexamined` row with an unknown relation, and decide on
  `abstention.active`, not on the reason. Adding an `open` value is compatible. Removing or renaming a
  value is breaking in either kind of row. Every string a frozen Core mode emits at a registered path
  MUST be a value of its row, so a new value cannot ship without a register change that states its
  status. Each row MUST be reached by at least one frozen mode.

  | Member | `tool` | Values | Status | Source |
  |---|---|---|---|---|
  | `state` | `context` | `READY`, `NO_CANDIDATES` | closed | `internal/contextindex/taskcontext.go:1949@3f21cab9` |
  | `coverage.governance` | `context` | `reserved`, `spec-mentioned`, `unresolved` | closed | `internal/contextindex/taskcontext.go:1714@9847248b` |
  | `coverage.budget_shortage` | `context` | `slots`, `work`, `none` | closed | TCP-V0-011 |
  | `coverage.unexamined[].relation` | `context` | `governing`, `spec-mentioned`, `instruction-routed`, `pair`, `mentioned`, `definition`, `reverse-import`, `reference`, `cochange`, `sibling`, `test`, `lexical`, `documentation` | open | TCP-V0-011 |
  | `coverage.unexamined[].state` | `context` | `examined`, `capped`, `empty-history`, `subject-absent`, `subject-symbols-incomplete`, `not-applicable` | closed | TCP-V0-011 |
  | `context.state` | `query`, `impact` | `READY`, `OUT_OF_SCOPE`, `NEEDS_WIDENING`, `BUDGETED`, `CRITICAL_EVIDENCE_OVERFLOW`, `WORKTREE_EVIDENCE`, `PARTIAL` | closed | `internal/contextindex/receipt.go:41@744935db`, `internal/worktreeimpact/compiler.go:331@11242995` |
  | `context.freshness.state` | `query`, `impact` | `fresh`, `mixed-worktree` | closed | `internal/worktreeimpact/compiler.go:369@f1a395e7` |
  | `context.freshness.scope` | `query`, `impact` | `git`, `git+working-tree` | closed | `internal/worktreeimpact/compiler.go:369@f1a395e7` |
  | `context.abstention.reason` | `query` | `none`, `needs-widening`, `nearest-negative-claim`, `omitted-competing-record`, `below-relevance-floor`, `unindexed-worktree-changes`, `no-relevant-candidates` | open | `internal/contextindex/eval_query.go:386@2e798125` |
  | `inventory.operationalState` | `init`, `adopt` | `COMPLETE`, `PARTIAL`, `INVALID` | closed | `internal/genesis/inventory.go:133@5662da2b` |
  | `inventory.dirtyState` | `init`, `adopt` | `CLEAN`, `DIRTY`, `UNKNOWN` | closed | `internal/genesis/inventory.go:81@4f7a9f91` |
  | `plan.scope` | `affected` | `BOUNDED`, `UNKNOWN` | closed | `internal/liveverify/affected/select.go:55@884d7796` |
  | `advice.status` | `affected` | `PLAN_ONLY` | closed | `cmd/corvint/affected.go:112@7320d4cb` |
  | `provider.go.state` | `affected` | `RUNNABLE`, `EMPTY_SELECTION`, `MODULE_PATH_UNRESOLVED`, `PACKAGE_BOUND_EXCEEDED` | closed | `cmd/corvint/affected.go:131@797e536b` |
  | `state` | `prove` | `READY`, `OUT_OF_SCOPE`, `NEEDS_WIDENING`, `BUDGETED`, `CRITICAL_EVIDENCE_OVERFLOW`, `WORKTREE_EVIDENCE`, `PARTIAL`, `CITED`, `UNPROVEN` | closed | `cmd/corvint/prove.go:1697@b984fed9` |
  | `coverage.answerability.verdict` | `context` | `no-specific-terms`, `relations-answer`, `unsupported-conjunction`, `not-withheld`, `supported` | closed | (proposed, decision 0398) `internal/contextindex/taskcontext.go:2807@c020e4b8` |
  | `context.intent.id` | `query` | `repository`, `project-operations`, `agent-tooling` | open | (proposed, decision 0398) `internal/contextindex/query.go:201@56442382` |
  | `context.intent.confidence` | `query` | `default`, `high` | closed | (proposed, decision 0398) `internal/contextindex/eval_query.go:488@1f684e46` |
  | `context.learning.local_trace_state` | `query` | `absent`, `ready`, `blocked-mixed-worktree` | closed | (proposed, decision 0398) `internal/contextindex/eval_query.go:62@4b83d4fe` |
  | `context.range.status` | `impact` | `CLEAN`, `UNTRACKED-ALLOWED` | closed | (proposed, decision 0398) `internal/contextindex/range_impact.go:227@71c6f8b4`, `internal/contextindex/range_impact.go:236@8fd264ef` |
  | `context.omissions.samples[].reason` | `impact` | `non-Go path outside native Go range profile` | closed | (proposed, decision 0398) `internal/contextindex/range_impact.go:139@7b21b9c6` |
  | `plan.unknown[].reason` | `affected` | `UNINDEXED_SOURCE_PATH`, `UNOWNED_DIRTY_PATH`, `LANGUAGE_FRONTIER`, `NO_SELECTABLE_TEST` | open | (proposed, decision 0398) `internal/liveverify/affected/select.go:39@644f277c` |
  | `plan.excluded[].reason` | `affected` | `NO_DEPENDENCY_PATH_TO_DIRTY_UNIT` | closed | (proposed, decision 0398) `internal/liveverify/affected/select.go:30@93004a33` |
  | `plan.excluded[].invalidation` | `affected` | `NEW_DEPENDENCY_EDGE_OR_DIRTY_PATH` | closed | (proposed, decision 0398) `internal/liveverify/affected/select.go:159@b74c00a2` |
  | `inventory.semanticFrontier[].reason` | `init`, `adopt` | `mechanical-inventory-only`, `declared-source-absent` | closed | (proposed, decision 0398) `internal/genesis/inventory.go:419@9ce6e141`, `internal/genesis/inventory.go:427@dc024f8c` |
  | `inventory.semanticFrontier[].sourceClass` | `init`, `adopt` | `INSTRUCTIONS`, `DOCUMENTATION`, `SPECIFICATION`, `TEST`, `E2E_TEST`, `CI`, `MANIFEST`, `OWNERSHIP`, `RUNBOOK`, `INCIDENT`, `SCHEMA`, `CONFIGURATION`, `CODE`, `OTHER_TEXT` | closed | (proposed, decision 0398) `internal/genesis/classifier.go:14@40a932fe` |
  | `inventory.gaps[].code` | `init`, `adopt` | `binary-asset`, `binary-content`, `blob-budget-exhausted`, `blob-size-unavailable`, `blob-too-large`, `blob-unavailable`, `dirty-worktree`, `entry-budget-exhausted`, `git-budget-exceeded`, `git-input-budget-exceeded`, `git-output-budget-exceeded`, `git-read-failed`, `git-timeout`, `git-unavailable`, `gitlink`, `invalid-activation`, `invalid-authority-id`, `invalid-commit-object`, `invalid-exclusions`, `invalid-git-output-budget`, `invalid-repository`, `invalid-revision`, `invalid-tree-object`, `malformed-blob-batch`, `malformed-tree-entry`, `non-utf8-or-binary`, `receipt-budget-exceeded`, `special-tree-entry`, `symlink`, `unsafe-or-non-utf8-path`, `HISTORY_NOT_SCANNED` | open | (proposed, decision 0398) `internal/genesis/inventory.go:477@a7d278da`, `docs/specs/genesis-backfill.md:265@3cb61d7d` |
  | `packet.mode` | `prove` | `query`, `impact`, `range-impact` | closed | (proposed, decision 0398) `internal/contextindex/range_impact.go:194@c076902a`, `internal/contextindex/receipt.go:549@d6f9a459` |
  | `proof.rows[].falsifier` | `prove` | `history-consistent`, `reference-resolves`, `verifier-accepts`, `test-kills-mutant`, `none` | closed | (proposed, decision 0398) `cmd/corvint/prove.go:42@f81f3f8e` |
  | `proof.rows[].falsified` | `prove` | `PASS`, `FAIL`, `NOT_RUN` | closed | (proposed, decision 0398) `cmd/corvint/prove.go:48@7313a787` |
  | `proof.affected.scope` | `prove` | `BOUNDED`, `UNKNOWN` | closed | (proposed, decision 0398) `cmd/corvint/prove.go:671@f6741c9b`, `internal/liveverify/affected/select.go:55@884d7796` |

  The `graph` relation that the experimental `CORVINT_CONTEXT_GRAPH=on` switch appends
  (`internal/contextindex/ppr.go:43@fd7e5f2f`) is outside the frozen default mode. So is the `prove --cem` state
  `REJECTED` (CCF-V1-003). NOT_PRODUCED: this change registers no enumeration inside
  `coverage.answerability`, `context.intent`, `context.learning`, `context.range`, `plan.unknown`,
  `plan.excluded` or the `prove` rows' falsifier verdicts. Until a later change registers them, CCF-V1-006
  decides a value added there by review. NOT_PRODUCED: no release gate builds the N-1 tag and replays
  the Core modes against this one.
  (proposed, decision 0398; V1-0350) The rows marked proposed register the enumerations inside
  `coverage.answerability`, `context.intent`, `context.learning`, `context.range`, `context.omissions`,
  `plan.unknown`, `plan.excluded`, `inventory.gaps` and `inventory.semanticFrontier`, the `prove` rows'
  falsifier verdicts, `proof.affected.scope` and `packet.mode`. That replaces the first NOT_PRODUCED
  above. `prove` embeds its query or impact packet unchanged as `packet`: the `query` rows govern it
  when `packet.mode` is `query`, and the `impact` rows when it is `impact` or `range-impact`, with
  `context.` read as `packet.`. An `open` row here is one a reader decides beside, not on: on
  `inventory.operationalState` rather than a gap code, on `plan.scope` rather than an unknown reason,
  and on the packet rather than its ranking intent. Outside the register, because no reader branches
  on them: free text (`coverage.answerability.reason`, `context.coverage.uncertainty`,
  `advice.unknown`), kind-prefixed identifiers (`context.coverage.critical` and `critical_missing`),
  enumerations carried as member names (`inventory.denominator`, `proof.counts`), which the
  CCF-V1-002 goldens and the CCF-V1-006 member rule pin, and the result rows (`context.results[]`,
  `results[]`, `proof.rows[]` `kind`, `authority` and `trust`, `plan.selected[].witness`), which
  CCF-V1-005 does not freeze. NOT_PRODUCED: `context.exclusions.samples[].reason` of `query` and
  `impact`, and the `relation` and `trust` of `context`'s `coverage.critical`, `critical_missing` and
  `governance_refused` rows, because no frozen fixture writes one and every row MUST be reached;
  registering them needs a fixture that does. Two changes after 0.8.1 reached members this register
  now covers. Commit 1aa1187c added `omitted-competing-record` to the `open` `context.abstention.reason`
  row, which is compatible; that value is absent from the `v0.8.1` tree. V1-0340 (ec50af2d) removed
  the `plan.excluded[].reason` value `UNINDEXED_DIRTY_GO_PATH_MAY_BE_DELETED_OR_RENAMED`, which 0.8.1
  writes (AFP-V0-012). Both shipped before the register reached their member. Like
  `instruction-routed`, the removal is part of the baseline only once the owner accepts it; otherwise
  V1-0340's rollback restores the value.

  (proposed, decision 0398; V1-0350) N-1 replay. `make core-n1-replay` builds `CORE_N1_TAG` (by
  default the newest release tag without a pre-release suffix reachable from `HEAD^`) from
  `git archive` and runs `TestCoreVerbsEmitTheFrozenProfiles` with `CORVINT_CORE_N1_BINARY` naming that
  binary. Each frozen mode then also runs under the N-1 binary over a fresh fixture. It MUST exit alike
  and carry the same CCF-V1-002 identifiers. Every member path it emits, arrays spread, MUST be emitted
  here with the same JSON type. Every value it writes at a registered path MUST be a value of this
  register. `index --if-stale` on a fresh snapshot is skipped: its setup writes the snapshot with this
  build, and an engine mismatch is a miss by design under (a). The target is opt-in, like
  `companion-release-gate`: release-runbook step 8 runs it, and it is not a `make gate` prerequisite.
  Making it one is an owner decision. This replaces the second NOT_PRODUCED above. A member, value or
  order that no fixture exercises stays unreplayed.
- **CCF-V1-008:** Root help MUST carry a `Command maturity:` section that lists the Core verbs of
  CCF-V1-001 and labels every other dispatched top-level verb `Experimental` with the requirement
  prefix of its owning spec, which `docs/specs/INDEX.json` MUST index. Every dispatched verb is exactly
  one of Core or labelled, and `TestInvalidChoiceNamesEveryDispatchedTopLevelVerb` requires every
  `topLevelCommands` verb in `Commands:`. The only verbs dispatched outside `topLevelCommands` are the
  three CCF-V1-003 plumbing verbs; they MUST stay out of root help, and a fourth such verb fails
  `TestOnlyThePinnedHookPlumbingVerbsBypassRootHelp`.

## Non-goals

- No runtime or wire change to any verb other than the proposed CCF-V1-004 refusal classification of
  2026-09-25 (panel blocker B5), its V1-0284 partial-clone extension and the codes added under
  decision 0398, and no new profile version.
- No freeze of companion, research or experimental verbs, modes or profiles (CCF-V1-003, CCF-V1-008).
- No ratification of the Core boundary: that is the owner's decision in V1-0001.
- No change to the portable proof wire, CEM, OCM or frontier contracts, which have their own owners.
- No platform qualification claim beyond the existing support boundary (Darwin and Linux).

## Failure modes

- A Core identifier changes silently: `TestCoreVerbsEmitTheFrozenProfiles` fails.
- A Core refusal changes exit class, stdout or code family: `TestCoreRefusalsKeepTheFrozenEnvelope` fails.
- A frozen Core mode emits a value outside its CCF-V1-007 (d) register row, or a row is reached by no
  frozen mode: `TestCoreVerbsEmitTheFrozenProfiles` fails through `observeCoreEnumerations`.
- (proposed, decision 0398) The N-1 tag emits a frozen member, identifier or registered value that
  this build lost, retyped or dropped from the register: `make core-n1-replay` fails.
- A refusal drops a code it gained under decision 0398: `TestRepositoryFailureEnvelopeCarriesItsCode`
  or `TestReadFailuresKeepTheFixedTextAndAddTheirCode` fails.
- A Core verb classifies a non-root working directory or an unborn `HEAD` differently from its
  siblings: `TestCoreVerbsRefuseAWorkingDirectoryOutsideTheRootAlike`,
  `TestMapFirstCoreVerbsRefuseANonRootDirectoryAlike` or `TestIndexedCoreVerbsCodeAnUnbornHead` fails.
- (proposed, V1-0284) A Core read reaches a promisor remote, refuses a missing promisor object without
  its code, or names an object the failed read did not: `TestIndexedCoreVerbsCodeAPromisorObjectWithoutFetching`,
  `TestIndexedCoreVerbsRefuseAPromisorFetchGitStartsAnyway` (a Git that ignores `GIT_NO_LAZY_FETCH`)
  or `TestClassifyMissingObjectsNamesOnlyAnObjectTheReadNamed` fails. Residual: readers outside
  these verbs whose Git environment still allows a transport.
- A new verb is dispatched without a maturity label, or a label names an unindexed owner:
  `TestRootHelpLabelsEveryVerbWithMaturityAndOwner` fails.
- A verb is dispatched before the `topLevelCommands` check without being pinned as plumbing:
  `TestOnlyThePinnedHookPlumbingVerbsBypassRootHelp` fails.
- The owner changes the Core set of decision 0332: this contract, its test list and decision 0358
  must change together.
- (proposed, decision 0398) A frozen mode's member is removed, renamed, retyped or added, an array
  changes length, or a fixture-stable value changes, for example the `affected` `plan.graphDigest`
  change between 0.7.0 and 0.8.1: `TestCoreVerbsEmitTheFrozenProfiles` fails against that mode's
  CCF-V1-002 golden. Residual: a value the golden pins by type only, and a member, value or order that
  no test fixture exercises. Only the cli-parity replay (for the four pinned verbs) and each verb's own
  spec tests catch those.

## Acceptance evidence

- `GOTOOLCHAIN=local go test -count=1 -run 'TestCoreVerbsEmitTheFrozenProfiles|TestCoreRefusalsKeepTheFrozenEnvelope|TestCoreVerbsRefuseAWorkingDirectoryOutsideTheRootAlike|TestMapFirstCoreVerbsRefuseANonRootDirectoryAlike|TestIndexedCoreVerbsCodeAnUnbornHead|TestIndexedCoreVerbsCodeAPromisorObjectWithoutFetching|TestIndexedCoreVerbsRefuseAPromisorFetchGitStartsAnyway|TestRootHelpLabelsEveryVerbWithMaturityAndOwner|TestOnlyThePinnedHookPlumbingVerbsBypassRootHelp' ./cmd/corvint`
  and `GOTOOLCHAIN=local go test -count=1 -run 'TestClassifyMissingObjectsNamesOnlyAnObjectTheReadNamed' ./internal/contextindex`.
- The cli-parity replay over the 133-case manifest against a candidate built from this change.
- The cited N-1 and migration tests in Traceability.
- (proposed, decision 0398) `make core-n1-replay` against `v0.8.1`, at each release (runbook step 8).

## Traceability

| Requirement | Evidence |
|---|---|
| CCF-V1-001, CCF-V1-002 | `TestCoreVerbsEmitTheFrozenProfiles` (identifiers, and each mode against its `cmd/corvint/testdata/core-freeze` golden) |
| CCF-V1-003 | `TestOnlyThePinnedHookPlumbingVerbsBypassRootHelp` |
| CCF-V1-004 | `TestCoreRefusalsKeepTheFrozenEnvelope`, `TestConvertedRefusalDiagnostics`, `TestCoreVerbsRefuseAWorkingDirectoryOutsideTheRootAlike`, `TestMapFirstCoreVerbsRefuseANonRootDirectoryAlike`, `TestIndexedCoreVerbsCodeAnUnbornHead`, `TestIndexedCoreVerbsCodeAPromisorObjectWithoutFetching`, `TestIndexedCoreVerbsRefuseAPromisorFetchGitStartsAnyway`, `TestClassifyMissingObjectsNamesOnlyAnObjectTheReadNamed`, `TestRepositoryFailureEnvelopeCarriesItsCode`, `TestReadFailuresKeepTheFixedTextAndAddTheirCode`; cli-parity-v0 replay (`DR-0041`) |
| CCF-V1-005 | `TestCoreVerbsEmitTheFrozenProfiles` (envelope, and member names and types through the goldens); per-verb member tests in AFP-V0, FPK-V0, TCP-V0 and GPK-V0 |
| CCF-V1-006 | cli-parity-v0 replay; `TestCoreVerbsEmitTheFrozenProfiles` |
| CCF-V1-007 (a) | `TestSnapshotRoundTripAppliesDirtyPathsAndMissesOnANewTree`, `TestSectionedSnapshotRefusesACorruptSectionAsAMiss`, `TestIndexIfStaleReceiptsAndFreshSnapshotIsUntouched` |
| CCF-V1-007 (b) | `TestMigrateTracesDryRunMatchesPythonOracleBytes`, `TestMigrateTracesApplyMatchesPythonOracle`, `TestMigrateTracesPlanDigestMismatchWritesNothing`, `TestMigrationCandidateDriftCheckCoversWholePlan`, `TestMigrationQuarantineBindingDetectsReplacement`, `TestPythonOracleMigrationTransform`, `TestReadBoundsTraceReplayWithoutRefusingLargeRepositories`, `TestStoreReadRejectsWholeStoreViolations` |
| CCF-V1-007 (c) | `TestProveObserveRejectsWhatIsNotAProof`, `TestProveObserveRecordsOnlyTheVerdictCounts`, `TestStatementIsByteStableAcrossCalls`, `TestPUBV0024InstalledCoreDiscoveryWorkflows` |
| CCF-V1-007 (d), CCF-V1-006 enumerations | `TestCoreVerbsEmitTheFrozenProfiles` (every registered member it reaches, including the rows of the `prove` `packet`); (proposed, decision 0398) the same test under `make core-n1-replay` (`replayCoreModeN1`) for the N-1 replay |
| CCF-V1-008 | `TestRootHelpLabelsEveryVerbWithMaturityAndOwner`, `TestInvalidChoiceNamesEveryDispatchedTopLevelVerb`, `TestOnlyThePinnedHookPlumbingVerbsBypassRootHelp` |

## Rollback

Revert the change that introduced this contract: the spec, decision 0358, its index rows, the root-help
`Command maturity:` section and `cmd/corvint/core_freeze_test.go`. No runtime, wire or stored state
changes, so rollback needs no migration. The proposed CCF-V1-004 classification of 2026-09-25 rolls back
alone by reverting its change; it writes no stored state. Its V1-0284 extension rolls back the same
way, with its `FIX-REGISTRY.tsv` row and the diagnostic coverage count. The decision 0398 codes roll back the same
way, with `DR-0041` and its two `knownDivergence` declarations.
Reverting only the per-mode goldens (proposed, decision 0398) means deleting `cmd/corvint/testdata/core-freeze/` and
the golden comparison in `TestCoreVerbsEmitTheFrozenProfiles`. That returns the freeze to identifier-only pinning.
Reverting the V1-0350 amendment (proposed, decision 0398) means deleting its register rows, the `prove`
`packet` mapping, `replayCoreModeN1` and the `core-n1-replay` target. That restores the two NOT_PRODUCED
statements of CCF-V1-007; no stored state changes.
