# Affected and CLI Consistency V0

Owner: Russell Lewis
Date: 2026-11-01
Requirement prefix: `ACX-V0`
Intent status: experimental under the owner's standing lane auto-accept (2026-10-11; lane aff-w12)
Delivery status: experimental
Authoritative inputs: native tickets V1-3583, V1-3327, V1-3578, V1-3726, V1-3727, V1-3045,
V1-2907, V1-3734, V1-3731, V1-3736 and V1-3878; lane aff-w42 (2026-11-02) adds V1-4214, V1-4039,
V1-4008 and the orchestrator's NEW-C1 and NEW-C3 items. These requirements refine the
[affected plan](affected-plan-v0.md) and the command-line surface around it; they live here to
keep that spec's Requirements section under the 256-obligation OCM limit (`maxObligations`).

## Agent digest
- Claim: Affected source walks, worktree path plans, test-file readers, revspec bases and the CLI flag, error and Makefile surface follow one contract.
- Status: experimental under the owner's standing lane auto-accept (2026-10-11; lane aff-w12).
- Exists: `Source.Files` and `ByName` in `internal/liveverify/affected/source.go`; `--worktree` in `cmd/corvint/affected.go`; `selfBinaryNotice` in `cmd/corvint/self_binary.go`.
- Blocked on: nothing.
- Read next: Requirements; Failure modes; Acceptance evidence and traceability.

## Intent and scope

Intent (orchestrator lane request aff-w12, 2026-11-01): close ten friction tickets in
`corvint affected`, test selection and the CLI surface, each with the smallest complete slice that
meets its acceptance criteria. Affected user: an agent or engineer planning tests for a change or
driving Corvint from scripts.

## Requirements

- `ACX-V0-7707`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3736)
  Shipped executables MUST share a process-level CLI failure envelope on stderr: `binary`,
  nonempty `code`, explanatory string `error`, `help`, and `ok:false`. Structured native refusal
  fields MUST remain available, including the original immutable-evidence document in `details`;
  a Tasks native result keeps its profile, codes and warnings as additive fields, and its decoder
  accepts the additive CLI metadata. Successful documents and protocol-only analyzer/MCP/LSP
  traffic MUST retain their owned wire shapes. Invalid CLI input, unknown verbs and unknown help
  topics exit 2; operational refusal exits 1; explicitly invalid native inputs retain exit 2; success exits 0; documented domain advisories retain
  their exit semantics. Unknown verbs and missing tickets MUST carry a code. A bounded
  deterministic spelling suggestion MUST name the nearest actual tree command for `contxt` and
  `tikcet`. Shared conformance tests MUST launch every shipped binary and check error envelope,
  stream and exit status; lint MUST reject a message equal to its code. The standard-library-only
  framing and metadata helpers may cross the Tasks boundary only at the explicitly enumerated
  process/help/version files, and the Tasks source export MUST retain their complete closure.
  Internal outdated-map consumers MUST compare the exact admitted code and message independent
  of additional framing fields; other reasons MUST NOT authorize map replacement. The intentional
  query abstention MUST remain byte-bound under the exact two-rendering QAT-V0-7708 contract;
  arbitrary envelope normalization MUST NOT widen that exemption.
  Tests: `TestCLIErrorContracts_ACXV0859`, `TestCAPV07707NativeEnvelopes`,
  `TestCAPV07707MessageCodeLint`, `TestCAPV07707TasksMissingTicketProcess`,
  `TestACXV07707OutdatedMapEnvelope`, `TestQATV07708SharedCLIEnvelope`.


- **ACX-V0-850:** (experimental under the owner's standing lane auto-accept, 2026-10-11; V1-3583)
  `Source.Files` and `Source.FilesIncluding` SHALL call `accept` with each walked file's
  repository-relative slash path and its base name, on a supplied filesystem, a disk source and a
  context-scoped disk source alike, so a directory-scoped rule such as the Gradle default version
  catalog `gradle/libs.versions.toml` matches. `ByName` SHALL adapt a name-only predicate, and
  every existing name-only caller SHALL keep its selection unchanged.
- **ACX-V0-851:** (experimental, amending AFP-V0-094, under the owner's standing lane
  auto-accept, 2026-10-11; V1-3327) `corvint affected --worktree` with `--path` or `--paths-from`
  SHALL plan the named paths against the working tree, dirty paths and ignored files included,
  instead of refusing `unsupported-affected-mixed-worktree`. Its receipt SHALL carry the
  degradation `worktree-path-plan` and a warning that names HEAD and the dirty count and says the
  plan is worktree-derived and is not pinned evidence. `--worktree` without `--path` or
  `--paths-from`, or with `--require-clean`, `--snapshot` or `--base`, SHALL be an argument
  error. Without `--worktree` the AFP-V0-094 refusal SHALL stay the default. Under `--worktree` a
  directory `--path` SHALL expand from the working tree, untracked and ignored files included
  beside the files HEAD tracks there, and refuse a nested repository; `--by-path` SHALL carry the
  same warning and degradation as the plan.
- **ACX-V0-852:** (experimental, narrowing AFP-V0-021, under the owner's standing lane
  auto-accept, 2026-10-11; V1-3578) For a dirty `_test.go` path that a Go unit owns, another Go
  unit SHALL be a `PATH_LITERAL_READER` only when one of its path tokens names the file itself:
  a component run of the token that matches the path's file name, whole or as the run's
  trailing fragment. A token that names only an ancestor directory of the path SHALL NOT select
  it: Go compiles a `_test.go` file only into its own package's test binary, so a directory
  literal of another package is no read of it. A non-Go unit, a declared read scope
  (AFP-V0-023), a glob reader (AFP-V0-401) and every other dirty path keep their rules.
- **ACX-V0-853:** (experimental, amending AFP-V0-496, under the owner's standing lane
  auto-accept, 2026-10-11; V1-3726) `corvint affected` SHALL advise the focused
  `corvint test --base BASE --dry-run` command only when `.github/cishards/test-commands.json`
  is a regular file at the root, because without it that command falls back to a FULL scope with
  no units. Otherwise the advice SHALL omit it and SHALL carry the unknown entry
  `FOCUSED_TEST_COMMAND_UNDECLARED`, which names the missing declaration and points to the
  declared mandatory checks (the Makefile gate or the AGENTS.md Verify commands) as the next step.
  Compatibility: the change moves one `advice.checks` entry into `advice.unknown` and keeps
  `advice.status` `PLAN_ONLY`. `advice.unknown` is free text outside the CCF-V1-007 (d) register
  and no member, type or CCF-V1-002 identifier changes, so under CCF-V1-006 it is compatible and
  ships no new `affected-plan` profile version; the CCF-V1-002 goldens
  `affected-committed-range.json` and `affected-full-committed-range.json`, whose fixture has no
  declaration, are regenerated in the same change.
- **ACX-V0-854:** (experimental, amending AFP-V0-230, under the owner's standing lane
  auto-accept, 2026-10-11; V1-3727) `corvint affected --base` SHALL also accept a revision such
  as `HEAD~1` or `origin/main`: a value of 1 to 256 printable non-space ASCII characters that is
  not all hex, does not start with `-` or `:` and holds no `..`. It SHALL be resolved once with
  `git rev-parse --verify --quiet --end-of-options VALUE^{commit}` and replaced by the full
  commit id before planning, so `range.base` stays pinned. An all-hex value keeps the AFP-V0-230
  rules. Every value that does not resolve to one commit SHALL be refused with
  `invalid-arguments` carrying the same hint, `git rev-parse --verify 'VALUE^{commit}'`; an
  ambiguous prefix keeps `ambiguous-affected-base` (AFP-V0-232).
- **ACX-V0-855:** (experimental, under the owner's standing lane auto-accept, 2026-10-11;
  V1-3045) When two Python files share one dotted name, as `pkg.py` and `pkg/__init__.py` do,
  the Python plugin SHALL keep both files in the one unit `python:pkg`, with the imports of both,
  so an edit to either has that owner and selects its importers. Neither file SHALL be dropped
  from the graph and become an unowned dirty path.
- **ACX-V0-856:** (experimental, under the owner's standing lane auto-accept, 2026-10-11;
  V1-2907) The `e2e-safe-selection/1` result SHALL carry `frontier_excluded`, one entry per
  `plan.frontierExcluded` plugin with `language`, `units` and the excluded `frontier` entries
  that the profile would read as unbounding the closure had they stayed in `plan.unknown`
  (AFP-V0-560). Entries the profile answers itself are left out, and the member is omitted when
  none is left. It is additive, so the schema string stays `e2e-safe-selection/1`; the decision
  rules are unchanged.
- **ACX-V0-857:** (experimental, under the owner's standing lane auto-accept, 2026-10-11;
  V1-3734) A bare `make` (no target) SHALL run the `help` target, which only prints the targets
  that carry a `## ` description and the effective `BASE`; it SHALL run no build, test or gate.
  The file default SHALL be `BASE ?= origin/main`. `help` SHALL read `BASE` through the
  environment, so a value holding a quote prints as given. Dogfood targets SHALL still refuse a file-origin
  `BASE` (ASD-V0-019), and `make gate` SHALL stay an explicit target.
- **ACX-V0-858:** (experimental, under the owner's standing lane auto-accept, 2026-10-11;
  V1-3731) The verbs whose range end is `--target` (`prove`, `surprise`, `lrf`, `obligations`,
  `ocm`, `dogfood-ocm`) SHALL accept `--head` and `--head=VALUE` as that same option, before any
  verb parser runs; `--target` stays accepted. `--head` given with `--target`, or twice, SHALL be
  refused `invalid-arguments` before any verb parser runs, since some keep the last value. Help
  names only `--target`: `--head` is an undocumented compatibility alias, and the vocabulary test
  pins help without it. `dogfood change|check|seal|batch-check|propose-citations|prechange` SHALL accept
  `--base BASE` in place of the positional BASE; giving both is `duplicate-local-completion-option`.
  A test SHALL enumerate the range and output flags each verb's help names, so a change to that
  vocabulary is a reviewed edit. Renaming the other spellings (`--revision`, `--rev`, `--json`,
  `--summary`) is out of scope here.
- **ACX-V0-859:** (experimental, under the owner's standing lane auto-accept, 2026-10-11;
  V1-3736) An unknown top-level `corvint` command SHALL keep the argparse invalid-choice
  `invalid-arguments` envelope and SHALL append `; did you mean 'X'?`, naming up to three commands
  within edit distance 2 of the typed word, nearest first. The test in `conformance/cli-errors-v0`
  SHALL build every binary that has a command tree in `internal/capcatalog/trees/`, run each one
  with an unknown verb, and check its exit code, output stream and envelope against a declared
  row for that binary. A binary without a row, or a row without a binary, SHALL fail the test.
  Making the binaries' envelopes identical is out of scope here; `ACX-V0-904` unifies the companions.
- **ACX-V0-664:** (experimental, under the owner's standing lane auto-accept, 2026-10-11;
  V1-3736 AC2; amended by ACX-V0-904) Every catalog binary's unknown-verb refusal SHALL carry a
  stable code and name the typed word. Until ACX-V0-904 each refusal kept the stream and exit
  code its row records, and named the
  typed word, on the stream and with the exit code its row already records. Binaries whose refusal
  had no code (`corvint-cem-candidate`, `corvint-cem-experiments`, `corvint-corpus-republish`,
  `corvint-gateway`, `corvint-js-test-provider`, `corvint-ruby-test-provider`,
  `corvint-postmerge-workflow`, `corvint-test`, and the later catalog binary `corvint-update`) SHALL
  print `UNKNOWN_COMMAND`;
  `corvint-postmerge-connect` SHALL refuse an unknown operation as `unknown-operation` before it
  reads any input. The conformance test SHALL fail when a row is silent or unchecked. The
  `corvint-work-queue` row is the one exception: WQO-V0-007 and WQO-V0-030 pin its refusal to the
  exact text `unsupported operation`, so its row checks that text until those requirements change.
  ACX-V0-904 replaces the `UNKNOWN_COMMAND` and `unknown-operation` refusals of the companions with
  the shared `unknown-verb` envelope, which keeps the stable code and the typed word.
- **ACX-V0-750:** (experimental, amending AFP-V0-846 and AFP-V0-871, under the owner's standing
  lane auto-accept, 2026-10-11; V1-4214, NEW-C1) When `corvint affected` judges its own binary
  `self-binary-stale` or `self-binary-unverified`, every output form SHALL carry the notice its
  stderr line prints. The affected-plan/1 receipt (compact and `--full`), the `--summary`
  affected-summary/1 verdict and the `--by-path` affected-by-path/1 receipt SHALL add the member
  `"degradationDetails": {"<code>": "<notice>"}`, and their `--format text` forms SHALL print
  `degraded-detail CODE NOTICE` right after the `degraded CODE` line, so a reader of stdout alone,
  as after `2>&1`, still gets the recovery. The notice is the AFP-V0-846 detail; for a stale
  binary it continues `, or run this checkout's own build without installing it (go run
  ./cmd/corvint)`, and for both codes it ends `; this notice changes no plan, selection or exit
  status: they are this binary's own, and current source may compute them differently`.
  `corvint affected` and `corvint test` SHALL print that notice in their `DEGRADED` lines. A
  binary built at `HEAD`, or one Corvint cannot judge, SHALL add neither the member nor the line,
  so its output is byte-identical to before. The plan, the selection and the exit status are
  unchanged in every case.
- **ACX-V0-751:** (experimental, under the owner's standing lane auto-accept, 2026-10-11;
  V1-4039) Each `corvint affected` or `corvint test` invocation SHALL judge its binary once and
  print at most one `DEGRADED` line, the same line on every invocation at the same `HEAD`, and
  SHALL record nothing about having printed it: read commands keep no such state (product
  invariant 4). A replay that runs the planner N times therefore prints N identical lines; its
  driver dedupes them on the `degradations` member, or merges stdout and stderr so AFP-V0-871
  leaves the line out. Rebuilding, or running this checkout's build with `go run ./cmd/corvint`,
  removes the notice.
- **ACX-V0-752:** (experimental, amending ACX-V0-851, under the owner's standing lane
  auto-accept, 2026-10-11; NEW-C3) `corvint affected --worktree` with `--base` SHALL stay an
  `invalid-arguments` error, decided before the `--path` and `--base` refusal, and its message
  SHALL name the plans that exist: without `--worktree`, `--base BASE` already plans the committed
  range `BASE..HEAD` together with every dirty worktree path (AFP-V0-010), and a named subset
  against the working tree is a `--worktree` path list that also names the range's own paths,
  each rename's source as well as its destination. The recovery command printed in the message
  SHALL include `--no-renames`: `git diff --no-renames --name-only BASE HEAD`.
  Git's default rename detection lists only the destination, which would drop the readers of the
  old path that `--base` plans. The other `--worktree` refusals keep their message without
  `--base`. Planning a worktree-derived path list against a base inside one invocation is a
  non-goal: `--base` alone already covers the range with the working tree, and a second change
  set beside `--path` stays refused (AFP-V0-764).
- **ACX-V0-904:** (experimental, under the owner's standing lane auto-accept, 2026-11-01;
  V1-3878) Every catalog companion binary (every `internal/capcatalog/trees/` binary except
  `corvint`, `corvint-tasks` and `corvint-work-queue`) SHALL refuse a first argument that is an operation word naming
  none of its top-level command-tree nodes or internal entry points with exit 2, nothing on
  stdout and one JSON line on stderr whose fields are exactly `binary`, `code` `unknown-verb`,
  `error` `unknown command "WORD"`, `help` `BINARY --help` and `ok` false, before its own
  dispatch parses anything. `clitree.Help.Serve` applies it after help, and the binaries that
  answer help with `clitree.Answer` (`corvint-cem-candidate`, `corvint-cem-experiments`) or their
  own help (`corvint-test`) call `clitree.RefuseIfUnknownVerb`. Decision: `corvint` keeps the
  argparse-parity `invalid-arguments` envelope (`ACX-V0-859`) and `corvint-tasks` its
  `taskman-command-result/0` `MALFORMED` result on stdout, the Tasks wire contract (CAP-V0-820),
  and `corvint-work-queue` the exact `unsupported operation` text that WQO-V0-007 and WQO-V0-030
  pin (its `serveHelp` leaves an unknown verb to its dispatch), so their rows stay. Non-goals: unifying any other refusal (a bad flag or a missing input keeps
  each binary's own envelope); a verb after preamble options such as `--root PATH`. Failure
  modes: a word a binary dispatches but its tree omits would be refused; the catalog tree check
  keeps dispatched words in the tree, and the conformance rows run every binary. Rollback: restore
  the `!handled` return in `Help.Serve`, drop the three direct calls and the `corvint-work-queue` skip, and restore the rows.

## Non-goals

Pinning a worktree-derived plan as evidence; a repository-wide flag rename. For ACX-V0-750 and
ACX-V0-751: narrowing the AFP-V0-846 / TSP-V0-722 staleness rule (the lane aff-w42 build-log entry
`docs/build-log/2026-11-02-aff-w42.md` measured the narrower rules and records why the rule
stays), rebuilding or replacing a binary, and a once-per-session notice, which would need state a
read command may not write.

## Failure modes

(1) A caller that still compares the first `accept` argument with a base name would match nothing;
the signature change makes every such caller a compile error. (2) A `--worktree` plan can be
read as evidence by a consumer that ignores `degradations`; the warning and the degradation name
it. (3) A Go test that walks a directory holding another package's `_test.go` files and reads
them as data is no longer selected by that directory literal alone on a test-only edit; it is
selected when it declares the read in `.corvint/test-read-scopes.json` (AFP-V0-023), and full CI
still runs it. (4) A decoder that rejects unknown members would reject a stale binary's receipt for
`degradationDetails`; the repository's readers of these receipts (`.github/cishards`,
`tools/corvint-pr-tests`, `internal/companionrelease`, the `jq` filters in
`script/batch-precheck.sh`) are lenient, and a fresh binary's output is unchanged. (5) The notice
never says the plan is current: it says the plan is the binary's own and may differ from what
current source computes, so a stale binary still yields uncertainty, not invented certainty
(product invariant 2).

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| ACX-V0-850 | `Source.Files`, `Source.FilesIncluding` and `ByName` in `internal/liveverify/affected/source.go`; the catalog filters in `internal/liveverify/affected/kotlin/kotlin.go` | `TestSourceFilesOffersRelativePath_V1_3583`; the plugin package tests |
| ACX-V0-851 | `Worktree` in `parseAffectedOptions` and `compileAffectedGraph`, `affectedWorktreeWarning` in `cmd/corvint/affected.go`; `affectedWorktreeFilesUnder` in `cmd/corvint/affected_path_directory.go`; `runAffectedByPath` in `cmd/corvint/affected_by_path.go` | `TestACXV0851WorktreePathPlan`; `TestACXV0851WorktreeDirectoryAndByPath` |
| ACX-V0-852 | `foreignTestFile` and `namesFileName` in `internal/liveverify/affected/readers.go` | `TestTestFileEditSelectsOnlyFileNamingReaders_ACX_V0_852`; the V1-3578 repro counts in `docs/build-log/2026-11-01-aff-w12.md` |
| ACX-V0-854 | `setAffectedBase` in `cmd/corvint/affected.go`, `resolveAffectedAbbrevBase` and `affectedBaseHint` in `cmd/corvint/affected_abbrev_base.go` | `TestACXV0854RevisionBase`; `TestAFPV0230AbbreviatedBaseAndTrailingRoot` |
| ACX-V0-855 | `UnitsSource` in `internal/liveverify/affected/python/python.go` | `TestModuleAndPackageInitShareOneOwner_ACX_V0_855` |
| ACX-V0-856 | `frontierExclusions` and `SelectE2E` in `internal/appflows/selection.go` | `TestE2ESelectionNamesFrontierExclusions_ACXV0856` |
| ACX-V0-853 | `withFocusedNextStep` in `cmd/corvint/affected.go` | `TestAffectedAdviceFocusedNextStep_AFPV0496`; `TestAffectedAdviceOmitsFocusedCommandWithoutDeclaration_ACXV0853`; `TestCoreVerbsEmitTheFrozenProfiles` |
| ACX-V0-857 | `.DEFAULT_GOAL`, `BASE` and `help` in `Makefile` | `TestMakeDefaultGoalIsHelp_ACXV0857` |
| ACX-V0-859 | `suggestChoices` in `cmd/corvint/suggest.go`; `conformance/cli-errors-v0` | `TestUnknownCommandSuggestsNearChoices_ACXV0859`; `TestCLIErrorContracts_ACXV0859` |
| ACX-V0-664 | the unknown-verb refusals in `main.go` of the eight named binaries and `run` in `cmd/corvint-postmerge-connect/main.go`; the coded-row check in `conformance/cli-errors-v0/contract_test.go` | `TestCLIErrorContracts_ACXV0859` |
| ACX-V0-904 | `UnknownVerb`, `RefuseUnknownVerb`, `RefuseIfUnknownVerb` in `internal/clitree/unknownverb.go`; `Help.Serve` in `internal/clitree/help.go`; `serveHelp` in `cmd/corvint-work-queue/help.go`; `conformance/cli-errors-v0` `companion` rows | `TestACXV0904UnknownVerbRefusal`; `TestACXV0904HelpServeRefusesUnknownVerb`; `TestCLIErrorContracts_ACXV0859` |
| ACX-V0-858 | `withHeadAlias` in `cmd/corvint/flag_aliases.go`, `dogfoodFlowArguments` in `cmd/corvint/dogfood_flow.go` | `TestHeadAliasOnTargetVerbs_ACXV0858`; `TestDogfoodFlowBaseOption_ACXV0858`; `TestRangeAndOutputFlagVocabulary_ACXV0858` |
| ACX-V0-750 | `selfBinaryNotice` in `cmd/corvint/self_binary.go`; `degradeStaleAffected` in `cmd/corvint/affected.go`; `writeAffectedDegradations` in `cmd/corvint/affected_text.go`; the `DegradationDetails` members in `affected_compact.go`, `affected_verdict.go` and `affected_by_path.go`; the `corvint-test: DEGRADED` line in `cmd/corvint/test.go` | `TestStaleBinaryNoticeReachesEveryAffectedForm_ACXV0750`; `TestAFPV0825StaleCheckReusesThePlanHEAD` |
| ACX-V0-751 | `degradeStaleAffected` in `cmd/corvint/affected.go`; the stale check in `runTest` in `cmd/corvint/test.go` | `TestStaleBinaryNoticeOncePerInvocation_ACXV0751` |
| ACX-V0-752 | `affectedWorktreeBaseRefusal` and its check in `parseAffectedOptions` in `cmd/corvint/affected.go`; the `--worktree` help in `cmd/corvint/help.go` | `TestWorktreeWithBaseNamesTheAlternative_ACXV0752`; `TestWorktreeWithBaseRecoveryKeepsRenameSources_ACXV0752` (committed `assets/old.txt` → `assets/new.txt`, printed `--no-renames`, recovered paths and old-path reader witness); `TestACXV0851WorktreePathPlan` |

## Rollback

Revert the commits naming these requirements; each is local to its ticket's files and no stored
state depends on them.

| ACX-V0-7707 | `internal/clierror`, `internal/cliconformance`, executable entrypoints and the additive Tasks result decoder | `TestCLIErrorContracts_ACXV0859`, `TestCAPV07707NativeEnvelopes`, `TestCAPV07707MessageCodeLint`, `TestCAPV07707TasksMissingTicketProcess` |

ACX-V0-7707 supersedes the per-binary process-envelope, stream and unknown-verb exit exceptions
of ACX-V0-859, ACX-V0-904 and TCL-V0-859. Their in-process protocol serializers retain their
contracts. The all-binary test now derives its rows from every shipped tree. No new network,
service, Tasks mutation or ranking authority is introduced. Rollback restores the former process
writers and their per-binary conformance rows together; persisted records need no migration.
