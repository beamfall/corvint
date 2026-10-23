# Capability catalog V0

Owner: Russell Lewis
Date: 2026-10-20
Requirement prefix: `CAP-V0`
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-2008); CAP-V0-007 to CAP-V0-009 accepted under the same standing auto-accept (V1-2020); CAP-V0-015 to CAP-V0-021 accepted under the same standing auto-accept (beamfall/corvint#1588); CAP-V0-024 to CAP-V0-029 accepted under the same standing auto-accept (beamfall/corvint#1626)
Delivery status: experimental
Authoritative inputs: V1-2008 owner request (2026-10-20) "make sure corvints capabilities are properly documented so agents can fully understand what it can do", and the same-day scope addition for a README capability section; `../../AGENTS.md` invariants 2 and 8; `../AGENT-ROUTES.md`; beamfall/corvint#1588 (2026-10-23), a machine-readable command catalog generated from the command registry and a task-indexed guide shipped with the install.

## Agent digest
- Claim: `docs/CAPABILITIES.md`, `corvint help --json` and `corvint help guide` describe every command; tests fail when any drifts from the binary.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-2008); CAP-V0-007 to CAP-V0-009 accepted under the same standing auto-accept (V1-2020); CAP-V0-015 to CAP-V0-021 accepted under the same standing auto-accept (beamfall/corvint#1588); CAP-V0-024 to CAP-V0-029 accepted under the same standing auto-accept (beamfall/corvint#1626)/experimental
- Exists: `docs/CAPABILITIES.md`, the README "What Corvint can do" section, `tools/capabilities-check` (its `go test` and `make capabilities-check`), and `internal/capcatalog` (`corvint help --json`, `corvint help guide`, `make capability-catalog`, `make capability-catalog-check`, and the offline Non-capabilities status guard `NONCAPABILITY-STATUS.tsv`), and catalog pointers in `AGENTS.md`, `docs/AGENT-ROUTES.md`, `docs/README.md` and the shipped Claude Code, Codex, Gemini CLI, OpenCode and OpenSpec skills.
- Blocked on: subcommands outside the declared tables, flags, profile membership and prose claims are not gated in `docs/CAPABILITIES.md` (see Failure modes); the guide ships inside the binary rather than as a release-archive member (owner-accepted as shipped with the install, 2026-10-23: a seventh member would break deployed updaters); the drift tests run in the existing Go test shards, and `capability-catalog-check` is not a separate CI step; `corvint-mcp` serves both through `corvint.help` (`MCPV0-100`), the other MCP servers do not; the Pi package text is not routed yet (its qualified adapter tuple pins the package bytes).
- Read next: Requirements; Boundaries and failure modes; Traceability.

## User and baseline

An agent that has never seen Corvint must learn what it can ask, how, and how far to trust the
answer. Before this spec the answers were spread over the 589-line README, 50 command help pages,
four MCP servers, 54 companion binaries and more than 200 specs, and some shipped skill text was
older than the code (V1-1271). The baseline is reading `corvint help`, each `COMMAND --help` and
the spec index by hand. The job is one dense page that is checked against the code, so that a row
cannot silently go missing or outlive its command.

## Requirements

- `CAP-V0-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2008)
  `docs/CAPABILITIES.md` MUST open with a "Start here" list of 10 to 15 task-to-command workflows,
  then group reference tables whose rows name the command or tool, the question it answers, when
  to use it and when not, a minimal invocation, its output, its maturity, what it refuses or
  abstains on, and its owning spec as a link. It MUST carry a "Language and path support" section
  for impact, affected and context, including what falls back. Every claim MUST come from the
  current code or help text; maturity MUST be `core` only for the verbs `corvint help` lists as
  Core, `internal` for CI and release tooling, and otherwise the owning spec's delivery status
  (`experimental` unless that spec says `implemented`). Nothing experimental is described as
  delivered or stable.
- `CAP-V0-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2008) The
  catalog MUST hold exactly one gated row for each top-level command listed in the root help's
  `Commands:` block of `cmd/corvint/help.go` or the `topLevelCommands` dispatch list in
  `cmd/corvint/main.go`, each directory under `cmd/` holding a `package main` Go file, and each
  `corvint.*` MCP tool name literal (letters, digits, `_`, `.` and `-`) in non-test Go source
  under `internal/mcp/`
  and `cmd/corvint-*mcp/`. Gated rows live between `<!-- capability-gate:commands -->`,
  `<!-- capability-gate:binaries -->` and `<!-- capability-gate:mcp-tools -->` markers and their
  `<!-- /capability-gate:... -->` closers; a row's name is the first backticked span in its first
  cell, with a leading `corvint ` removed for commands.
- `CAP-V0-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2008)
  `tools/capabilities-check` MUST report a `missing` finding for an inventory item without a row,
  a `stale` finding for a row naming nothing in the inventory, a `duplicate` finding for a second
  row of the same name, a `maturity` finding when a command row's maturity disagrees with the
  root help's Core list, a `dispatch-undocumented` finding for a dispatched verb without a root
  help line, a `dispatch-absent` finding for a root help verb that is not dispatched, and a `broken-link` finding for a relative link in a gated region whose
  target does not exist. Its test MUST run under plain `go test ./...` and MUST prove failure on a
  synthetic omission and a synthetic stale row. `make capabilities-check` runs the same check.
- `CAP-V0-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2008) The root
  `README.md` MUST carry a "What Corvint can do" section near the top whose gated rows
  (`<!-- capability-gate:readme -->`) each give a one-line benefit, a minimal command and a
  maturity label, and link to `docs/CAPABILITIES.md`. Each row's first backticked command MUST
  resolve to a catalog row, and its maturity's first word MUST equal that catalog row's; otherwise
  the check reports `readme-unmapped` or `readme-maturity`.
- `CAP-V0-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2008)
  `AGENTS.md` (one line), the "What capability exists?" row of `docs/AGENT-ROUTES.md`,
  `docs/README.md`, the root README and the shipped agent skills MUST route capability questions
  to `docs/CAPABILITIES.md`. A shipped package whose bytes change MUST carry its version bump and
  mirrors so the host-package version gate (AHI-020) stays green.
- `CAP-V0-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2008) The check
  MUST read only the paths it is given (the catalog, the README, `cmd/corvint/help.go`,
  `cmd/corvint/main.go`, the `cmd/` tree and `internal/mcp/`, plus `os.Stat` of linked targets;
  since V1-2020 also the non-test Go files of `cmd/corvint/` for CAP-V0-007 and the non-test Go
  source under `cmd/` and `internal/`, outside `testdata/`, for CAP-V0-009), run no command, use no
  network and write nothing.
- `CAP-V0-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2020) The check
  MUST read the code-declared subcommand tables of `cmd/corvint` (`helpSubcommands`, a nested map
  whose values are literals or a top-level identifier such as `cemHelpActions`;
  `docsSubcommands` for `docs`; `flowSubcommands` for `flows`, plus every
  `flowSubcommands["NAME"] = ...` registration in a non-test file; `leaseActions` for `lease`) and
  report a `subcommand-undocumented` finding when a declared subcommand is not a word inside any
  backticked span of a catalog row for its verb. A verb without a row is left to the `missing`
  finding. A declared table that is no longer found is a check error (exit 2), so renaming a
  table cannot silently disable the check.
- `CAP-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2020) When the
  root help groups its verbs, a command row whose verb sits under the Experimental header MUST
  have maturity `experimental`; any other maturity is a `maturity` finding, in addition to the
  CAP-V0-003 Core comparison.
- `CAP-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2020) Each
  backticked span in a gated table's "Abstains or refuses on" cell that has the shape of a code
  (kebab-case or SCREAMING_SNAKE_CASE with at least two parts) MUST be a whole word (letters,
  digits, `_` and `-`) of some string literal in non-test Go source under `cmd/` or `internal/`,
  outside `testdata/`; otherwise the check reports `refusal-unknown`, so a code removed or
  renamed in the code cannot stay listed.
- `CAP-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1588) `corvint help --json` MUST print one
  `corvint-capability-catalog/0` JSON document derived only from the binary's command registry:
  the `topLevelCommands` dispatch list (every entry once, in dispatch order), the root help, each
  command's help topics (for `docs`, also its corpus, flows and maintain topics and every corpus
  operation topic) and the code-declared operation tables (`docsSubcommands`, `corpusOperations`,
  `flowSubcommands`, `leaseActions`, `helpSubcommands`). Per command it states purpose, maturity,
  owning spec or freeze contract, operations, nested operations, flags, required flags per usage
  form (`requiredFlags`: the options every `Usage:` line of that form names outside `[ ]` and
  `( )`, keyed by the command plus its operation words; a form with none is omitted; a lower
  bound, see `CAP-V0-022`), schema identifiers,
  stated exit statuses, write class, self-observation ledger listing and write statements; at the
  top level it lists the two-word help topics (`nestedHelpTopics`); and the document names how
  each field is derived. Like every help topic it reads no stdin, opens no
  repository and writes nothing. It also carries the `profiling` member (`corvint-profile-knobs/0`)
  exactly as `PRB-V0-009` defines it.
- `CAP-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1588) A field the registry cannot answer MUST be
  `null` with an `unknowns` entry naming the field and a reason, never a hand-written claim:
  `inputs` and `outputs` always (help does not separate them; `schemaIds` lists every schema the
  help names); `owningSpec` for a Core verb (its `freezeContract` names the root help's freeze
  codes instead) or a verb whose root line names none; `purpose` and `maturity` for a command
  without a root help line; `writes` for a command the root help Support boundary does not
  classify; and `exitCodesMentioned` (an empty list) when the help states no exit status.
- `CAP-V0-017`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1588) `corvint help guide` MUST print the
  task-indexed agent guide embedded in the binary (`internal/capcatalog/AGENT-GUIDE.md`): goals
  mapped to command sequences, each with one runnable example and its output schema, including
  the two-step `corvint affected discovery --playwright-list` then
  `corvint affected --playwright-discovery` workflow, and a Non-capabilities section. Every
  example carries the required options of its usage form (`requiredFlags`), or points to the
  action another command returns and shows its full shape. The catalog binds the guide by SHA-256.
- `CAP-V0-018`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1588) A test MUST fail when the shipped guide
  names a command, operation, nested operation or option that the live catalog does not list for
  that command (or as a global option), names a schema identifier no help topic names, or lists a
  non-capability without a tracking issue (`#N`) or a requirement ID (amended by `CAP-V0-024`;
  a "By design" reason alone no longer passes). The check parses a
  leading `--root PATH` or `--root=PATH` preamble before the command (a `--root` without a path is a
  problem), accepts `corvint help` only with no topic, `--json`, `guide`, a catalogued command or
  a `nestedHelpTopics` entry, and fails an example that omits a required option of its form.
- `CAP-V0-019`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1588) `internal/capcatalog/catalog.json` MUST
  equal the bytes of `corvint help --json`; a test fails with the regeneration command when it
  does not. `make capability-catalog` regenerates it and `make capability-catalog-check` runs the
  CAP-V0-015 to CAP-V0-021 tests.
- `CAP-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1588) The catalog builder MUST read nothing but
  its registry argument and MUST encode the same registry to the same bytes: two-space indented
  JSON in a fixed field order, with sorted map keys, sorted operation, flag, schema-identifier and
  exit-status lists, no HTML escaping and a terminal LF.
- `CAP-V0-021`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1588) Root help MUST name `corvint help --json`
  and `corvint help guide` within its existing three usage lines and its CCF-V1-009 byte bound,
  and the unknown-help-topic refusal MUST keep listing commands and nested topics only.
- `CAP-V0-022`: (owner request, 2026-10-23; amends `CAP-V0-015`) The catalog MUST carry a
  top-level `requiredFlagsCoverage` member, encoded between `derivation` and `globalFlags`, whose
  only value is `partial`, and `derivation.requiredFlagsCoverage` MUST say why: `requiredFlags`
  reads only `Usage:` synopses, and the binary declares no structured required-option table for
  most forms. An option that help prose or the parser requires (one of two alternatives, or one
  required only in some mode) is therefore missing, and an absent form or option does not mean
  none is required. A consumer MUST treat `requiredFlags` as a lower bound. The only code-declared
  required-option table, `corpusOperations` (`docs corpus` operations), already generates those
  forms' usage lines (`DCP-V1-056`), so deriving from it would change no byte. The value can
  become something other than `partial` only in a change that derives every form from a structured
  source and amends this requirement. The schema stays `corvint-capability-catalog/0`, because
  the member is additive.
- `CAP-V0-023`: (owner request, 2026-10-23) The catalog builder MUST NOT derive a required option
  from help prose. A sentence such as "Exactly one of --dry-run and --apply is required" adds
  nothing to `requiredFlags`, and a form whose usage lines are alternatives with no common
  top-level option stays omitted. `CAP-V0-022` is how a consumer learns about that gap.
- `CAP-V0-024`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1626; amends
  `CAP-V0-018`) Every item of the guide's Non-capabilities section MUST name at least one issue
  number (`#N`) or requirement ID (an uppercase prefix and a three-digit number, such as
  `CAP-V0-016` or `MCPV0-100`). A "By design" item MUST name the requirement that excludes the
  capability; the words alone fail the CAP-V0-018 check.
- `CAP-V0-025`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1626)
  `internal/capcatalog/NONCAPABILITY-STATUS.tsv`, embedded in the package, MUST record the status
  of every ID a non-capability names: a header `id`, `state`, `evidence`, then one row per ID with
  non-empty evidence. An issue is `open` or `closed`; a requirement is `undelivered`, `delivered`
  or `excludes` (in force, and the reason the capability is absent by design). A malformed row, an
  unknown state or a repeated ID is a problem, never read as some state. The change that closes a
  named issue or delivers a named requirement MUST update its row in the same change. The check
  reads only this table and the guide: no network access at build or test time.
- `CAP-V0-026`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1626) A test
  MUST fail when a shipped non-capability names an ID that has no status row, an issue recorded
  `closed` or a requirement recorded `delivered`; when a "By design" item names no requirement
  recorded `excludes`; or when any other item names no `open` issue and no `undelivered`
  requirement. `capcatalog.NonCapabilityProblems` returns one line per problem and reads nothing
  but its two arguments. The section parse MUST fail closed: apart from one intro paragraph before
  the first item, every non-blank line MUST be a `- ` item or a two-space continuation directly
  under one, and any other line (another bullet style such as `* `, an indented or numbered item,
  or stray prose) is a problem reported by both `NonCapabilityProblems` and `GuideProblems`, never
  skipped.
- `CAP-V0-027`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1626) The guide's
  Goals MUST carry one runnable example each for static test discovery (`docs corpus
  discover-tests`), per-test coverage ingest (`flows ingest --coverage-artifact` then `flows
  ingest-coverage`), the subject-kind filter and claim hits of `docs corpus search` (`--kind`,
  `--include claims`), ranked claim search (`docs corpus claims --query`) and the delta worklist
  stream (`docs corpus delta --format jsonl`). The CAP-V0-018 check keeps each against the live
  catalog. The `flows ingest-coverage` example MUST pass `--test-key` with a key of the run
  evidence and say the coverage file holds only that test's coverage, because a run with more
  than one record refuses a single-test format without it. A shell redirection inside a quoted
  argument does not end a checked command line.
- `CAP-V0-028`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1626) The seven
  items GH-1626 found delivered (tracked by #1571, #1573, #1578, #1579, #1584, #1585 and #1587, each
  verified against the binary at `7b3a8e55f`) MUST NOT stay listed, and the status table MUST
  record those issues `closed`. The undelivered tsconfig `paths` residual MUST name the open issue
  #1604, not the closed #1578.
- `CAP-V0-029`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1626; owner
  decision 2026-10-23) The guide MUST stay inside the binary (`corvint help guide` and
  `corvint.help`) and MUST NOT be added as a release-archive member, because the archive, its
  verifier, the release-candidate check and the deployed `corvint-update` allow exactly six
  members. Adding one needs its own change with the staged rollout named under the CAP-V0-015 to
  CAP-V0-021 non-goals. Its Non-capability names this requirement, recorded `excludes`.
  `CAP-V0-030` to `CAP-V0-033` are reserved for this slice and unassigned.

## Boundaries and failure modes

Non-goals: generating `docs/CAPABILITIES.md` from code (the CAP-V0-015 catalog is a separate generated document; the page stays hand-written and checked); checking subcommands outside the declared tables, a
row naming a subcommand the code no longer declares, flags, output schemas or the prose of a row; running any listed command; ranking capabilities; replacing a spec, a help page or
the deep guides the catalog links to.

Failure modes: a subcommand dispatched outside the CAP-V0-007 tables (for example `flows coverage`
and `flows record`, or any verb whose subcommands live only in a switch) is not detected, and a
subcommand removed from the code can stay listed in its row; a subcommand word that appears in
some other backticked span of the row counts as listed; MCP profile membership and binary class are prose and unchecked;
the MCP scan counts every `"corvint.<name>"` literal in the scanned source, so a non-tool literal
of that shape would demand a row (none exists today) and a tool name built at run time would be
missed; maturity is checked for commands and README rows only, so a binary or tool row's maturity
can drift from its spec; a refusal code stays accepted while any string literal, including help
prose or an unrelated message, still contains it as a word, and a code written in prose without
backticks or without a separator is not checked; a row naming the right thing with a wrong claim
passes. A catalog edit
that the check rejects is fixed by editing the row, never by widening the gate.

CAP-V0-015 to CAP-V0-021 non-goals: an MCP help resource or prompt, or a help tool on the docs,
corpus or test-validity servers (`corvint-mcp`'s `corvint.help` tool is `MCPV0-100`, which amends
`MCPV0-008` and returns this catalog and guide byte for byte); a separate guide member in the release archive (the archive, its verifier, the release-candidate
check and the deployed `corvint-update` allow exactly six members, so a seventh needs a staged
rollout: tolerant updater and checks first, then the member and its manifest pin); a CI step
(the owner accepted on 2026-10-23 that the drift tests already run in the existing CI Go test
shards, so no `.github` change is needed); per-command input/output separation.

CAP-V0-022 and CAP-V0-023 failure modes: the marker is catalog-wide, so it does not say which
forms are incomplete. Measured at `77a49dfa`: `migrate-traces` lists no required option while its
help says "Exactly one of --dry-run and --apply is required", and `ocm prepare` lists `--intent`
and `--target` while the `ocm` help also calls `--expected-base` required for a `cem/1.0` map. A consumer that ignores unknown
members misses the marker. Rollback: remove the member, its derivation entry, its constant, the
two requirements and their tests, and rerun `make capability-catalog`.

CAP-V0-024 to CAP-V0-029 non-goals: querying GitHub or any network service at build, test or run
time; checking that a named requirement ID exists in `docs/specs/REQUIREMENTS.tsv`; judging whether
a Non-capability's prose is true; listing every undelivered capability.

CAP-V0-024 to CAP-V0-029 failure modes: the status table is caller-maintained evidence. A PR that
closes a named issue or delivers a named requirement without updating its row leaves a stale item
that the test does not catch; an issue closed by hand outside a PR is likewise missed. The
requirement pattern is lexical: a three-digit tracker ID shaped like a requirement would be read as
one and then fail for lack of a status row (fail closed), and a four-digit tracker ID such as
`V1-2458` is not a requirement ID. A delivered capability that no item names is not detected
either way. Rollback: remove `NONCAPABILITY-STATUS.tsv`, `NonCapabilityProblems`,
`NonCapabilityStatus`, `nonCapabilityIDs`, `parseNonCapabilityStatus` and their tests, restore the
"By design" allowance in `GuideProblems`, and rerun `make capability-catalog`; the guide's Goals
additions can stay, since CAP-V0-018 already checks them.

CAP-V0-015 to CAP-V0-021 failure modes: catalog fields are read from help prose, so they are as
complete as the help: a flag or schema the help omits is missing, an exit status stated without
the word "exit" is not listed (the field is named `exitCodesMentioned`), an operation dispatched
only in a switch and absent from usage lines is missing, `writeStatements` copies sentences
without judging them, and `writes` classifies only commands the root Support boundary names.
`local-state` means the command writes local state in at least one mode. A help text change in
any lane changes `catalog.json`, so a merge that changes help without regenerating fails
CAP-V0-019 until `make capability-catalog` is rerun. The guide check verifies names, not claims:
a correct command with a wrong description passes. A closed tracking issue is detected only
through the CAP-V0-025 status table.

## Rollback

Delete `docs/CAPABILITIES.md`, `tools/capabilities-check`, the `capabilities-check` Makefile target
and `.PHONY` entry, the `tools/capabilities-check` entry in `.corvint/test-read-scopes.json`, the
README section and the routing lines, then this spec's README row and INDEX entry. No stored state
depends on them. Reverting a skill text change needs its own version bump, because the host-package
version gate refuses a content change at an unchanged version. To roll back only CAP-V0-007 to
CAP-V0-009, remove `parseSubcommands`, `ScanWords`, `checkSubcommands`, `checkRefusals` and the
Experimental maturity case with their tests, and narrow the test read scope back to `internal/mcp/`;
the catalog row edits stay correct. To roll back only CAP-V0-015 to CAP-V0-021, delete
`internal/capcatalog`, `cmd/corvint/help_catalog.go` and its test, the `capabilityOrPublicHelpTopic`
and `capabilityHelpText` calls and the root usage words in `cmd/corvint/help.go`, the two Makefile targets and `.PHONY`
entries, and the `docs/CAPABILITIES.md` pointer; no stored state depends on them.

## Traceability

| Requirement | Implementation | Focused evidence |
|---|---|---|
| CAP-V0-001 | `docs/CAPABILITIES.md` | `TestCAPV0002RepositoryCatalogMatchesInventory` (structure via gated regions); build log self-check of five rows |
| CAP-V0-002 | `Collect`, `parseCommands`, `parseDispatched`, `scanBinaries`, `scanTools`, `toolNames`, `parseRows` | `TestCAPV0002RepositoryCatalogMatchesInventory`, `TestCAPV0002InventoryCountsMatchCode`, `TestCAPV0002ToolNamesWithDigitsAndHyphens` |
| CAP-V0-003 | `Check`, `checkDispatch`, `tools/capabilities-check/main.go` | `TestCAPV0003SyntheticOmissionAndStaleRowFail`, `TestCAPV0003MaturityAndLinkDriftFail`, `TestCAPV0002DispatchDriftFails` |
| CAP-V0-004 | `checkReadme` | `TestCAPV0004ReadmeRowsMapToCatalog` |
| CAP-V0-005 | routing lines and skill edits | `TestCAPV0005RoutingPointersNameTheCatalog`; `make host-package-versions-check`; `cmd/corvint`'s `TestHostAdapterJavaScriptHosts` (runs `integrations/host-adapters.test.mjs`) |
| CAP-V0-006 | `run`, `Collect`, `Check`, `ScanWords`, `parseSubcommands` | `TestCAPV0006CheckIsReadOnly` (synthetic tree unchanged by path, mode, mtime and digest), `TestCAPV0006CheckImportsNoProcessOrNetwork` |
| CAP-V0-007 | `subcommandTables`, `parseSubcommands`, `literalKeys`, `checkSubcommands` | `TestCAPV0007UndocumentedSubcommandFails`, `TestCAPV0007SubcommandTablesParse`, `TestCAPV0002RepositoryCatalogMatchesInventory` |
| CAP-V0-008 | `parseCommands` experimental set, `Check` maturity switch | `TestCAPV0008ExperimentalMaturityMismatchFails`, `TestCAPV0002GroupedRootHelp` |
| CAP-V0-009 | `ScanWords`, `refusalCode`, `checkRefusals` | `TestCAPV0009RemovedRefusalCodeFails`, `TestCAPV0002RepositoryCatalogMatchesInventory` |
| CAP-V0-015 | `capabilityRegistry`, `capabilityCatalogJSON`, `capcatalog.Build`, `capabilityHelpTopics` | `TestCAPV0015HelpJSONListsEveryDispatchedCommand` |
| CAP-V0-016 | `capcatalog.Build` unknowns | `TestCAPV0016UnderivableFieldsAreUnknownWithReasons`, `TestCAPV0016UnknownFieldsAreNullWithReasons` |
| CAP-V0-017 | `internal/capcatalog/AGENT-GUIDE.md`, `capcatalog.Guide`, `helpText("guide")`, `capcatalog.requiredFlags`, `capcatalog.FormKey` | `TestCAPV0017HelpGuideServesTheShippedGuide`, `TestCAPV0017ShippedGuideDocumentsThePlaywrightTwoStep`, `TestCAPV0017RequiredFlagsAreDerivedFromUsageForms`, `TestCAPV0017GuideExamplesCarryRequiredOptions` |
| CAP-V0-018 | `capcatalog.GuideProblems`, `commandLineProblems`, `helpTopicProblems` | `TestCAPV0018ShippedGuideMatchesTheLiveCatalog`, `TestCAPV0018GuideProblemsRejectsDrift` (preamble, nested help topic and required-option negatives), `TestCAPV0017GuideExamplesCarryRequiredOptions` |
| CAP-V0-019 | `internal/capcatalog/catalog.json`, `capcatalog.Snapshot`, `make capability-catalog`, `make capability-catalog-check` | `TestCAPV0019CommittedCatalogMatchesTheBinary` |
| CAP-V0-020 | `capcatalog.Build`, `capcatalog.Encode` | `TestCAPV0020BuildIsDeterministicAndDerivedFromItsArgument` |
| CAP-V0-021 | `rootHelp` usage line, `unknownHelpTopic` | `TestCAPV0021RootHelpNamesTheCatalogTopicsWithinItsBound`, `TestRootHelpIsACompactVerbIndex` |
| CAP-V0-022 | `capcatalog.Catalog.RequiredFlagsCoverage`, `capcatalog.RequiredFlagsPartial`, `derivation`, `internal/capcatalog/catalog.json` | `TestCAPV0022RequiredFlagsCoverageIsPartial` (value, derivation reason and encoded position), `TestCAPV0022BinaryCatalogMarksRequiredFlagsPartial` (`corvint help --json`), `TestCAPV0019CommittedCatalogMatchesTheBinary` |
| CAP-V0-023 | `capcatalog.requiredFlags` (usage forms only) | `TestCAPV0023ProseRequirementsAreNotDerived` (prose "is required" sentences and alternative usage lines add no required option) |
| CAP-V0-024 | `capcatalog.GuideProblems`, `nonCapabilityIDs`, `requirementPattern` | `TestCAPV0024NonCapabilitiesNameAnIssueOrRequirement`, `TestCAPV0018GuideProblemsRejectsDrift` |
| CAP-V0-025 | `internal/capcatalog/NONCAPABILITY-STATUS.tsv`, `capcatalog.NonCapabilityStatus`, `parseNonCapabilityStatus` | `TestCAPV0025StatusTableIsWellFormed` |
| CAP-V0-026 | `capcatalog.NonCapabilityProblems`, `nonCapabilities`, `plainText` | `TestCAPV0026NonCapabilityProblemsRejectsStaleItems`, `TestCAPV0026ShippedNonCapabilitiesAreCurrent`, `TestCAPV0026UnrecognisedNonCapabilityLinesFailClosed` |
| CAP-V0-027 | `internal/capcatalog/AGENT-GUIDE.md` goals 8 to 11, `guideCommandLines` | `TestCAPV0027GuideTeachesDeliveredCapabilities`, `TestCAPV0018ShippedGuideMatchesTheLiveCatalog`; live runs recorded in `docs/build-log/2026-10-23-help-guide-noncapabilities.md` |
| CAP-V0-028 | `internal/capcatalog/AGENT-GUIDE.md` Non-capabilities, `NONCAPABILITY-STATUS.tsv` | `TestCAPV0028DeliveredItemsLeftTheNonCapabilities` |
| CAP-V0-029 | `capcatalog.Guide` (embedded), `NONCAPABILITY-STATUS.tsv` row | `TestCAPV0029GuideIsNotAnArchiveMember`, `TestCAPV0017HelpGuideServesTheShippedGuide` |
