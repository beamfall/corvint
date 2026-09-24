# Expert Audit Follow-up V0

Owner: Russell Lewis
Date: 2026-09-06
Intent status: accepted bounded repair scope (owner follow-up 2026-09-06)
Delivery status: experimental
Requirement prefix: `EAF-V0`
Authoritative inputs: the owner's instruction to put the expert audit recommendations in place;
`docs/reviews/expert-invention-audit-2026-09-06.md`; existing LTA/FPK/SDD contracts and AT owners.

## Agent digest
- Claim: Audit repairs secret screening, mutation attribution, obligation recovery and native Git-read isolation; experimental outcome gates remain explicit.
- Status: accepted bounded repair scope (owner follow-up 2026-09-06)/experimental
- Exists: bounded repairs, regression witnesses, query recovery guidance, isolated native status and opt-in finite-policy/scope development checks.
- Blocked on: fresh workflow/reviewer outcomes, qualified runtime reuse and the unresolved decision 0009 remain separate gates. Development checks and the passing repair gate do not close them.
- Read next: Requirements; Experiment disposition; Verification and rollback.

## User job and intent boundary

The owner requested implementation of the audit follow-up. Close the reproduced local defects,
preserve work still owed during checkpoint recovery, and give the research recommendations concrete
owners, inputs, controls and stopping conditions. This is one integration slice over existing
capabilities, not another roadmap or a claim to have completed AT-02 through AT-17.

The exact secret-screen and checkpoint amendments are recorded in the owning LTA and FPK specs.
The original audit remains historical evidence, not source authority. The general instruction does
not choose either mutually exclusive authority root in decision 0009. No new root, authenticated
execution claim, Frontier closure rule or mandatory service is introduced here.

## Requirements

- `EAF-V0-001`: Current Go/Python secret screening MUST reject ordinary double-quoted credential
  property assignments under the existing credential vocabulary, screen its existing history
  consumers consistently, and redact observation fields. Quoted values MUST be redacted in full,
  including whitespace and lexical backslash escapes, without decoding encoded keys or values.
  A complete quoted value MUST leave subsequent benign fields unchanged. An opening value quote
  without a closing unescaped quote is malformed input; its redaction boundary is `EAF-V0-010`.
  Stored-v1 validation MUST retain its old
  pattern and byte-compatible row acceptance. Benign quoted fields MUST remain admitted.
- `EAF-V0-002`: Go mutation baseline, mutant and isolated grouped-fallback runs MUST use matching
  JSON/verbose execution mode. An unchanged-source mode-only failure MUST fail baseline admission
  before any mutation is credited; genuine mutant kills and existing failure classifications remain.
- `EAF-V0-003`: Checkpoint output MUST preserve the caller's obligations array exactly and label
  it `caller-reported-unverified`, as amended in FPK-V0-021/023. Partial verification, source drift,
  empty or forged obligation strings MUST NOT silently complete, discard or grant authority to it.
- `EAF-V0-004`: Source-documentation rederivation MUST discover an added admitted importer and
  reject the prior draft even when all prior source blobs are unchanged. Failed extraction MUST
  retain uncertainty and MUST NOT establish absence. This is fresh rederivation, not verdict caching.
- `EAF-V0-005`: Query help MUST describe safe trace-state refusal recovery: retain the original
  failure, task and trace store; consult project authority; use same-task experimental context for
  separate discovery. It MUST NOT suggest deletion, dirtying or rephrasing to evade admission.
- `EAF-V0-006`: The uncertainty perturbation development screen MUST retain the unchanged-task
  padding pair and positive/negative controls. It MUST be explicitly opt-in and report failures as
  promotion blockers; no test expectation or production ranking rule may be weakened to hide one.

- `EAF-V0-007`: Native Corvint and its selected Go provider MUST observe Git status only through
  private metadata, under GPK-V0-009 and the existing isolated-status boundary. This includes the
  two ordinary kernels and status seams in genesis, affected selection, parent verification,
  worksource acquisition/materialization and dashboard authority validation. Configured clean or
  process filters MUST NOT execute, including same-size dirty tracked-file edits, and a configured
  `core.fsmonitor` hook MUST NOT execute even when the caller's Git runner adds no override. A `.git`
  pointer file is read as Git reads it, with every trailing CR and LF stripped, so a CRLF pointer is
  not unsupported metadata. Unsafe or
  unsupported metadata MUST retain `repository-probe-failed` in the ordinary kernels or each other boundary's existing typed
  probe failure; ordinary final-status start, resource and Git failures retain their existing
  error shape, and cancellation retains its existing classification. No failure may retry against live metadata.
  Scratch-parent containment MUST compare filesystem ancestor identities as well as path spelling,
  including case aliases of the worktree and linked-worktree metadata. A naturally terminating
  ancestor walk MUST NOT introduce an arbitrary valid-path depth ceiling.
  The private index MUST preserve the captured index modification time used by Git's racy-entry
  checks, and timestamp drift MUST fail the observation. A qualified caller's scratch parent MUST
  come from its closed execution environment rather than ambient process variables.
  No-follow capture, bounded metadata, before/after drift checks, cleanup and cancellation MUST
  remain. The existing normal-path ceilings of seven Git processes, 500,000 allocated bytes and
  3,600 allocations MUST pass unchanged. A fast recognizer may accept only a strict proven subset
  of safe config and SHA-1 index v2/v3 framing; ambiguous config, SHA-256, v4 or unknown index
  extensions MUST use the existing private Git validation path. Live-index gitlinks and split
  index MUST retain their safe refusal before snapshot loading. Successfully loaded snapshots
  MUST retain committed skipped paths as unframable under SBQ-V0-008; this does not admit a live
  index containing gitlinks. Stored output and snapshot formats remain unchanged; source changes
  require the normal analyzer identity bump. This narrows standalone compatibility with unsafe
  configuration; it does not authorize configuration edits or executions.

- `EAF-V0-008`: The opt-in finite-policy development check MUST bind its independently frozen
  proposed-policy oracle by digest, enumerate all 8,191 nonempty sets of its thirteen reason
  classes and eighteen atomic inputs, and reject missing, duplicate, empty or wrong oracle cells.
  Unknown predicates MUST remain unsupported. The check MUST retain the proposed authority status
  and compare the actual existing Frontier policy, without introducing an adapter or claiming
  general program correctness. If ordinary table tests suffice, stop at those tests.
- `EAF-V0-009`: The opt-in scope development check MUST retain the frozen candidate algorithm
  before independent labels, validate base path/blob/span identity before scoring, and distinguish
  applicable, unrelated, unknown and native-grammar-unsupported labels. Target author citations
  MUST NOT select the independent cohort. Every admitted miss and false positive MUST remain
  visible; unsupported grammar or unobserved reviewer utility MUST NOT count as a successful
  discovery. Native witness output remains the changed-surface baseline, not independent scope
  authority. A contaminated cohort MUST remain invalid even if it supplies useful counterexamples.
- `EAF-V0-010`: Malformed quoted-value redaction boundary. When a credential-vocabulary quoted key
  is followed by an opening value quote and no unescaped `"` occurs anywhere later in the input
  (a later unescaped `"` closes the value lexically under `EAF-V0-001`), the current Go matcher
  (`internal/secretscreen`; decision 0088 retired the Python matcher) MUST redact from the key
  through absolute end of input, including newlines, structural delimiters (`,`, `:`, `}`, `]`)
  and a dangling backslash. End-of-line and next-delimiter
  boundaries are rejected: a secret may itself contain a newline or delimiter, and either boundary
  would re-expose the tail; losing benign trailing text from already-malformed input is the
  conservative failure (invariant 2). Persisted observation fields MUST carry the redacted form.
  Stored-v1 validation MUST NOT change: the v1 matcher does not recognize quoted values, so
  existing rows carrying such text remain accepted.
- `EAF-V0-011`: An `EAF-V0-007` refusal of unsafe or unsupported metadata MUST name its specific
  cause: the ordinary kernels report `repository-probe-failed` with the message
  `Git status cannot safely observe repository metadata: <reason>`. The reason names the refused
  feature, the config key (for example `filter.lfs.process`, `core.attributesFile`,
  `include.*`/`includeIf.*`), the metadata file or directory relative to its Git directory, or the
  applicable byte limit. A filter driver name appears only when it is 1 to 32 characters of
  `[a-z0-9_-]`; any other name is shown as `*`. It MUST NOT carry a config value, file content, or a path outside the
  repository; the one value it may name is the closed `extensions.refStorage=reftable`. What is
  refused does not change, and Git's own metadata-probe failures keep their existing shape. The MCP
  tool-error object stays closed under `MCPV0` and still carries only its sanitized code.
- `EAF-V0-012`: An ancestor directory of the repository or Git directory that the caller can
  search but not read (for example mode `0711` owned by another user), which plain `git status`
  traverses, MUST NOT by itself refuse isolated status. Such a directory has no open handle: the
  reader pins it by its `Lstat` identity, requires a real directory (a symlink is still refused),
  and opens its child by absolute path without following the final component. The status brackets
  MUST re-check every pinned identity, so a replaced search-only ancestor, or a child that no longer
  resolves to the handle that was read, is refused as a replaced metadata directory. This matches
  what Git's own path-based reads observe; a replacement undone entirely between the brackets is
  invisible to both. A metadata file's own directory still needs read access.

## Verified starting state

At `e97d2bc`, quoted JSON credentials were admitted, a mode-only test received a mutant kill, and
checkpoint output omitted obligations. These three regressions failed before the repairs.
The new padding screen demonstrates a separate limitation of the specified TCP-V0-016 rule:
47 to 48 admitted sources raises the rarity cutoff from two to three and can change lexical
`supported`/`READY` without additional task evidence. This is not an implementation violation of
TCP-V0-016; `supported` does not establish behavioral answerability. Changing that rule requires
its own amendment and registered positive/natural-no-gold evaluations.

## Experiment disposition

The executable screen and frozen next-step protocols are in
`benchmarks/expert-audit-followup/README.md`. Existing AT tickets remain the sole execution queue.

| Audit direction | Existing owner | Entry condition and next evidence |
|---|---|---|
| Candidate admission and honest coverage | AT-02/05 | Stage-specific misses under fixed delivered-byte budget; no ranking replacement without admission ablation |
| Irrelevant-file uncertainty | AT-05 | Run the opt-in padding screen; current failure blocks its invariance claim, not the already-declared lexical contract |
| Added-consumer invalidation | AT-12/15 | Fresh-rederivation regressions first; a negative-premise reuse profile needs a declared universe and full-recompute comparison |
| Independent obligation scope challenge | AT-05/17 | Freeze independently supplied base-authority paths and 20 development changes before comparing author-declared scope |
| Finite semantic contract | AT-17 | Freeze an owner-authored finite expected table and independent checker; stop if ordinary table tests are equally useful |
| Question-first activation and reviewer-requested evidence | AT-08/17 | Three unfamiliar-repository screens with competent tools/notes/CI controls, then existing fresh and independent outcome gates |
| Complete cumulative cost | AT-01/08/16 | Host-qualified all-worker measurements; missing telemetry remains unobserved, never zero |
| Completion policy | Decision 0009 and AT-17 | Owner selection remains required; retaining local observations never creates execution authority |

## Non-goals, limits and failure modes

No escaped JSON property decoding or exhaustive secret discovery; no retroactive rewriting of
trace rows; no changed retrieval scoring or held-out access; no automatic completion; no passing
execution reuse; no external participant outreach or hosted profile. The negative-premise and finite
semantic mechanisms remain proposed experiments, not newly delivered runtime capabilities.

Keep existing input, byte, process, allocation and observation bounds. A passing source test does
not establish external usefulness, semantic completeness, independent interoperability or full-task
savings. A failing development experiment is visible evidence, never a default-gate pass.

## Verification and rollback

| Requirement | Implementation / executable witness |
|---|---|
| EAF-V0-001 | `internal/secretscreen`, Python trace matcher; `TestQuotedCredentialsRejectNewRecordsButRetainStoredV1`, shared corpus and history/redaction regressions |
| EAF-V0-002 | `internal/liveverify/mutate/gotest.go`; `TestRunRejectsModeOnlyFailureBeforeMutation`, `TestGroupFallbackRejectsModeOnlyBaseline` and real kill controls |
| EAF-V0-003 | `cmd/corvint/prove_checkpoint.go`; `TestCheckpointObligationsSurvivePartialWorkAndAuthorityDrift`, `TestCheckpointForgedAndEmptyObligationsAreInert` |
| EAF-V0-004 | existing `DraftSources`/`ConsumeDraft`; `TestSourceDraftAddedImporterInvalidatesPriorDraft`, `TestDocsCLIAddedImporterInvalidatesPriorDraft` |
| EAF-V0-005 | `cmd/corvint/help.go`; `TestQueryHelpExposesRepositoryAndFrozenAuthorityProfiles` |
| EAF-V0-007 | native status seams and `internal/gitstatus`; same-size filter, private metadata, format-fallback and unchanged resource regressions; `TestStatusNeverExecutesRepositoryConfiguredFSMonitor`, `TestPrivateStatusAcceptsCRLFWorktreePointers` |
| EAF-V0-006 | opt-in `TestUncertaintyPaddingDevelopmentScreen`; invocation and retained failure in the experiment protocol |
| EAF-V0-008 | opt-in `TestFinitePolicyDevelopmentScreen`; frozen `finite-policy.json`, full finite-domain enumeration and malformed-oracle controls |
| EAF-V0-009 | opt-in scope development check in `internal/lrfrepo/scope_development_test.go`; independently supplied labels, immutable source checks and explicit unsupported outcomes |
| EAF-V0-011 | `internal/gitstatus` refusal reasons; `TestStatusRefusesUnsupportedMetadataBeforeLiveStatus`, `TestStandaloneReadNamesUnsupportedRepositoryFeature`, `TestStandaloneReadsRefuseGitFiltersWithoutMutation` |
| EAF-V0-012 | `internal/gitstatus/read_posix.go` search-only ancestor pinning; `TestStatusReadsThroughSearchOnlyParent` |
| EAF-V0-010 | `internal/secretscreen`, `internal/observations`; `TestUnterminatedQuotedValueRedactsThroughEOF`, `TestAppendRedactsUnterminatedQuotedCredentialPath`, `internal/secretscreen/testdata/parity.json` unterminated cases |

Run focused regressions, the canonical gate, fresh independent review, and post-commit CEM/OCM
checks. Record actual results in BUILD-LOG. AT-06 remains open until its real interrupted-task
comparison; no fixture closes that outcome gate. Rollback reverts this bounded change and retains
the audit and failed experiments. Git-read rollback restores the previous scoped boundary with an analyzer identity bump and reopens
the standalone filter defect; do not describe that rollback as universal read safety.
Removing the fixes restores their bugs and is not a safety
improvement; no historical trace migration is necessary in either direction.
