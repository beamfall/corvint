# Core Compatibility Freeze V1

Owner: Russell Lewis
Date: 2026-09-22
Requirement prefix: `CCF-V1`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: ticket V1-0007, accepted decision 0332 (the Core set), decision 0356, `AGENTS.md` invariants 1, 2, 4 and 8,
`conformance/cli-parity-v0/manifest.json`, and the owning specs of each Core verb listed in CCF-V1-002.

## Agent digest
- Claim: The twelve Core verbs of decision 0332 keep their command modes, wire profiles, error envelope and state readers compatible from 0.7.0 to 1.0.
- Status: proposed intent, experimental delivery; the Core set is taken from accepted decision 0332, this contract awaits owner ratification in V1-0001
- Exists: this contract, decision 0356, the root-help `Command maturity:` section (`commandMaturityHelp`), and `cmd/corvint/core_freeze_test.go`
- Blocked on: V1-0001 owner ratification of this contract; pinned modes for the mutating `cem`, `ocm` and `dogfood` subcommands are NOT_PRODUCED; the exhaustive gate is NOT_RUN
- Read next: Requirements; Breaking-change rule; Traceability

## Intent and scope

Corvint 1.0 needs a stated compatibility boundary so that an agent or CI job written against one
release keeps working on the next. Before this contract the boundary existed only implicitly: byte
parity cases for four verbs, a handful of in-repository readers that match profile strings exactly,
and a help footer saying that commands marked Experimental carry no stability promise, which did not
mark most verbs either way. This contract names the Core verbs, lists what about them is frozen,
states the rule that decides whether a change is breaking, and states how state and profile changes
reach users. It freezes what 0.7.0 already emits; it changes no verb's runtime behaviour or wire
output. Everything not listed stays experimental, so no companion or research profile becomes
frozen by omission.

The N-1 baseline is the published 0.7.0 release (tag `v0.7.0`, gated commit 41f2b68). At this
contract's base commit 1894b9e, `git diff v0.7.0 1894b9e -- cmd/corvint internal` is empty, so no Core
state or profile had changed since that baseline; this change adds only root-help text and tests.

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
  CF-V0); this contract pins only the identifiers above and neither restates nor changes those schemas.
  NOT_PRODUCED: no pinned mode for `cem begin`, `prepare`, `cite`, `mark`, `report`, `cover`,
  `discriminate`, `anchor` and `provenance`; `ocm prepare`, `link`, `mark` and `report`; the frontier
  human rendering and dynamic test mode; and `dogfood begin`, `verify`, `finish`, `review` and
  `cancel`. They are Core verbs' modes without a frozen identifier until a later change pins them with
  a test.
- **CCF-V1-003:** These modes and profiles MUST stay outside the freeze and MUST NOT be advertised as
  Core compatibility: `impact --provider*`, `--repository` and `--range-profile expanded-256`;
  `affected --snapshot`, `--playwright-config`, `--provider*` and `--selection-profile`; `prove --cem`,
  `--attest*`, `--verify-cem-attestation`, `--checkpoint` and `--mutate`; `context` lookup subcommands
  (for example `context authority`); and the profiles `external-evidence-provider/*`,
  `external-test-selection/0`, `playwright-affected/0`, `corvint-planning-snapshot/0`,
  `corvint-checkpoint/0` and in-toto statements. Their owning specs govern them. The verbs
  `native-hook`, `authority-event` and `qualified-event` are undocumented adapter plumbing that
  `runContext` dispatches before the `topLevelCommands` check (`cmd/corvint/main.go:790@e2ed60e2`); they
  are absent from root help and outside the freeze.
- **CCF-V1-004:** A Core refusal MUST exit 2 with empty stdout and exactly one stderr JSON line built by
  `emitError` (`cmd/corvint/main.go:1333@40010ccd`): `code`, `error` and `ok=false`, plus the DRC-V0-006
  diagnostic members `subject`, `evidence`, `supported_fixes` and optional `terminal` where the site was
  converted. Frozen code families are `invalid-*` (argument, revision and repository-root validation),
  `unsupported-*` (a well-formed request outside the qualified profile, including
  `unsupported-impact-range`, `unsupported-query-*`, `unsupported-prove-*`, `unsupported-affected-*`,
  `unsupported-working-tree-impact-*`), `repository-*`, `output-failed` and `internal-error`. The
  codeless envelope `{"error","ok":false}` that some index refusals emit (for example `impact` of an
  untracked path) is frozen as it is. `init` / `adopt` with an unusable revision instead print the
  inventory with `ok=false` and `operationalState=INVALID` on stdout and exit 1. Renaming or removing a
  code, adding `code` to the codeless envelope, or moving a refusal between exit classes is breaking.
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
  (IDX-SNAP-V0, CEM-CB, OCM-V0, CF-V0, LCP-V0) govern them.
- **CCF-V1-006:** Breaking-change rule. For output pinned by `conformance/cli-parity-v0/manifest.json`
  (`query`, `impact`, `init`, `adopt`), any stdout or stderr byte change is breaking unless recorded in
  `conformance/divergence-register.md` with a decision. For every other frozen mode: adding an optional
  member is compatible; removing, renaming or retyping a member, changing a CCF-V1-002 identifier value,
  adding a value to a frozen enumeration a reader must branch on, changing the default mode, or making
  a read mutate state is breaking. A breaking change MUST ship a new profile version (for example
  `affected-plan/1`), an N-1 reader per CCF-V1-007, a decision record, and an update to this contract
  and its test in the same change.
- **CCF-V1-007:** N-1 and migration policy. Every Core reader MUST accept the documents and state that
  the previous release (N-1, currently 0.7.0) wrote, and every state change on the path to 1.0 MUST
  have a deterministic migration or rebuild: (a) derived state, the index snapshot
  `corvint-index-snapshot/1`, treats any format, engine, tree or decode mismatch as a miss and
  rebuilds (IDX-SNAP-V0-003), so no migration exists or is needed; (b) durable local traces
  (`corvint-local-trace/1`) keep the bounded legacy tree-row reader and the explicit, digest-bound
  `migrate-traces --dry-run` / `--apply --plan-digest` migration (LTPM-V0); (c) in-repository readers
  of Core profiles, `prove-observe` (`cmd/corvint/prove_observe.go:90@b9e09d8d`), `internal/attest` and
  `internal/companionrelease` core smoke, match the current identifier exactly and MUST accept both N
  and N-1 identifiers in the change that bumps one. Because nothing changed since 0.7.0, the current
  N-1 obligation is satisfied by the unchanged identifiers pinned in CCF-V1-002.
- **CCF-V1-008:** Root help MUST carry a `Command maturity:` section that lists the Core verbs of
  CCF-V1-001 and labels every other dispatched top-level verb `Experimental` with the requirement
  prefix of its owning spec, which `docs/specs/INDEX.json` MUST index. Every dispatched verb is exactly
  one of Core or labelled, and `TestInvalidChoiceNamesEveryDispatchedTopLevelVerb` requires every
  `topLevelCommands` verb in `Commands:`. The only verbs dispatched outside `topLevelCommands` are the
  three CCF-V1-003 plumbing verbs; they MUST stay out of root help, and a fourth such verb fails
  `TestOnlyThePinnedHookPlumbingVerbsBypassRootHelp`.

## Non-goals

- No runtime or wire change to any verb, and no new profile version.
- No freeze of companion, research or experimental verbs, modes or profiles (CCF-V1-003, CCF-V1-008).
- No ratification of the Core boundary: that is the owner's decision in V1-0001.
- No change to the portable proof wire, CEM, OCM or frontier contracts, which have their own owners.
- No platform qualification claim beyond the existing support boundary (Darwin and Linux).

## Failure modes

- A Core identifier changes silently: `TestCoreVerbsEmitTheFrozenProfiles` fails.
- A Core refusal changes exit class, stdout or code family: `TestCoreRefusalsKeepTheFrozenEnvelope` fails.
- A new verb is dispatched without a maturity label, or a label names an unindexed owner:
  `TestRootHelpLabelsEveryVerbWithMaturityAndOwner` fails.
- A verb is dispatched before the `topLevelCommands` check without being pinned as plumbing:
  `TestOnlyThePinnedHookPlumbingVerbsBypassRootHelp` fails.
- The owner changes the Core set of decision 0332: this contract, its test list and decision 0356
  must change together.
- A byte change hides inside a frozen member's content (for example ranking order): only the
  cli-parity replay catches it, for the four pinned verbs; for the other eight Core verbs the contract
  relies on each verb's own spec tests.

## Acceptance evidence

- `GOTOOLCHAIN=local go test -count=1 -run 'TestCoreVerbsEmitTheFrozenProfiles|TestCoreRefusalsKeepTheFrozenEnvelope|TestRootHelpLabelsEveryVerbWithMaturityAndOwner|TestOnlyThePinnedHookPlumbingVerbsBypassRootHelp' ./cmd/corvint`.
- The cli-parity replay over the 133-case manifest against a candidate built from this change.
- The cited N-1 and migration tests in Traceability.

## Traceability

| Requirement | Evidence |
|---|---|
| CCF-V1-001, CCF-V1-002 | `TestCoreVerbsEmitTheFrozenProfiles` |
| CCF-V1-003 | `TestOnlyThePinnedHookPlumbingVerbsBypassRootHelp` |
| CCF-V1-004 | `TestCoreRefusalsKeepTheFrozenEnvelope`, `TestConvertedRefusalDiagnostics` |
| CCF-V1-005 | `TestCoreVerbsEmitTheFrozenProfiles` (envelope); per-verb member tests in AFP-V0, FPK-V0, TCP-V0 and GPK-V0 |
| CCF-V1-006 | cli-parity-v0 replay; `TestCoreVerbsEmitTheFrozenProfiles` |
| CCF-V1-007 (a) | `TestSnapshotRoundTripAppliesDirtyPathsAndMissesOnANewTree`, `TestSectionedSnapshotRefusesACorruptSectionAsAMiss`, `TestIndexIfStaleReceiptsAndFreshSnapshotIsUntouched` |
| CCF-V1-007 (b) | `TestMigrateTracesDryRunMatchesPythonOracleBytes`, `TestMigrateTracesApplyMatchesPythonOracle`, `TestMigrateTracesPlanDigestMismatchWritesNothing`, `TestMigrationCandidateDriftCheckCoversWholePlan`, `TestMigrationQuarantineBindingDetectsReplacement`, `TestPythonOracleMigrationTransform`, `TestReadBoundsTraceReplayWithoutRefusingLargeRepositories`, `TestStoreReadRejectsWholeStoreViolations` |
| CCF-V1-007 (c) | `TestProveObserveRejectsWhatIsNotAProof`, `TestProveObserveRecordsOnlyTheVerdictCounts`, `TestStatementIsByteStableAcrossCalls`, `TestPUBV0024InstalledCoreDiscoveryWorkflows` |
| CCF-V1-008 | `TestRootHelpLabelsEveryVerbWithMaturityAndOwner`, `TestInvalidChoiceNamesEveryDispatchedTopLevelVerb`, `TestOnlyThePinnedHookPlumbingVerbsBypassRootHelp` |

## Rollback

Revert the change that introduced this contract: the spec, decision 0356, its index rows, the root-help
`Command maturity:` section and `cmd/corvint/core_freeze_test.go`. No runtime, wire or stored state
changes, so rollback needs no migration.
