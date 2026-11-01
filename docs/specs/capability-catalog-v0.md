# Capability catalog V0

Owner: Russell Lewis
Date: 2026-10-20
Requirement prefix: `CAP-V0`
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-2008); CAP-V0-007 to CAP-V0-009 accepted under the same standing auto-accept (V1-2020); CAP-V0-015 to CAP-V0-021 accepted under the same standing auto-accept (beamfall/corvint#1588); CAP-V0-024 to CAP-V0-029 accepted under the same standing auto-accept (beamfall/corvint#1626); CAP-V0-030 to CAP-V0-032 accepted under the same standing auto-accept (beamfall/corvint#1784); CAP-V0-040 to CAP-V0-050 accepted under the same standing auto-accept (V1-2685); CAP-V0-200 experimental under the standing lane auto-accept (2026-10-30; V1-3153); CAP-V0-060 accepted under the same standing auto-accept (V1-3212)
Delivery status: experimental
Authoritative inputs: V1-2008 owner request (2026-10-20) "make sure corvints capabilities are properly documented so agents can fully understand what it can do", and the same-day scope addition for a README capability section; `../../AGENTS.md` invariants 2 and 8; `../AGENT-ROUTES.md`; beamfall/corvint#1588 (2026-10-23), a machine-readable command catalog generated from the command registry and a task-indexed guide shipped with the install.

## Agent digest
- Claim: `llms.txt`, `docs/CAPABILITIES.md`, `corvint help --json`, the guide and per-binary command trees describe every command, flag and MCP tool; drift fails tests.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-2008); CAP-V0-007 to CAP-V0-009 accepted under the same standing auto-accept (V1-2020); CAP-V0-015 to CAP-V0-021 accepted under the same standing auto-accept (beamfall/corvint#1588); CAP-V0-024 to CAP-V0-029 accepted under the same standing auto-accept (beamfall/corvint#1626); CAP-V0-030 to CAP-V0-032 accepted under the same standing auto-accept (beamfall/corvint#1784); CAP-V0-040 to CAP-V0-050 accepted under the same standing auto-accept (V1-2685); CAP-V0-200 experimental under the standing lane auto-accept (2026-10-30; V1-3153); CAP-V0-060 accepted under the same standing auto-accept (V1-3212)/experimental
- Exists: `docs/CAPABILITIES.md`, the README "What Corvint can do" section, `tools/capabilities-check` (its `go test` and `make capabilities-check`), and `internal/capcatalog` (`corvint help --json`, `corvint help guide`, `make capability-catalog`, `make capability-catalog-check`, and the offline Non-capabilities status guard `NONCAPABILITY-STATUS.tsv`), `internal/clitree` with the snapshots under `internal/capcatalog/trees/` and the shrink-only `CLI-RATCHET.tsv`, the root `llms.txt`, and catalog pointers in `AGENTS.md`, `docs/AGENT-ROUTES.md`, `docs/README.md` and the shipped Claude Code, Codex, Gemini CLI, OpenCode and OpenSpec skills.
- Blocked on: the 40 `cmd/` binaries on the CLI ratchet have no command tree yet (`corvint-cem-candidate` and `corvint-cem-experiments` left it under CAP-V0-930, and the ten subcommand companions of CAP-V0-972 under that requirement); flags and subcommands deeper than `VERB SUB` are gated against the command trees, not against the `docs/CAPABILITIES.md` rows, and profile membership and prose claims are not gated (see Failure modes); the guide ships inside the binary rather than as a release-archive member (owner-accepted as shipped with the install, 2026-10-23: a seventh member would break deployed updaters); the drift tests run in the existing Go test shards, and `capability-catalog-check` is not a separate CI step; `corvint-mcp` serves both through `corvint.help` (`MCPV0-100`), the other MCP servers do not; the Pi package text is not routed yet (its qualified adapter tuple pins the package bytes).
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
- `CAP-V0-960`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2422) The
  subcommands CAP-V0-007 checks MUST also include every two-word node path (`VERB SUB`) of the
  committed `internal/capcatalog/trees/corvint.json` command tree, so a subcommand dispatched
  outside the declared tables (for example `flows coverage`) still needs its verb's row to name it.
  A missing or malformed `corvint` tree snapshot is a check error (exit 2).
- `CAP-V0-961`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2422) For a
  command row whose verb has a node in that tree, each backticked span at parenthesis depth zero in
  the row's `Subcommands` sentence (from the word `Subcommands` to the first `. ` at depth zero or
  the cell end) whose first word does not start with `-` names a documented subcommand; one that is
  neither a two-word tree node nor a CAP-V0-007 table entry for the verb is a `subcommand-stale`
  finding, so a subcommand removed from the code cannot stay listed.
- `CAP-V0-962`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2422) The
  CAP-V0-009 match is exact: a listed refusal code MUST equal the whole value of some string
  literal (an owned code declaration such as `const codeX = "the-code"` or a `"the-code"` argument)
  in that source, so a word inside help prose or an unrelated message no longer keeps a removed
  code accepted. This replaces CAP-V0-009's whole-word reading.
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
  `CAP-V0-033` is reserved for this slice and unassigned.
- `CAP-V0-030`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1784, V1-2684)
  The catalog MUST carry a top-level `workflows` member, encoded between `commands` and
  `profiling`. Each entry is a code-declared multi-command chain: `name`, `summary`, `maturity`,
  `specs`, `guide`, `chain` (the IDs of its required steps in order) and `steps`. Each step has
  `order`, `id`, `optional`, `command` and `helpTopic` (both null for a repository state such as a
  file to commit), `summary`, `requires`, `usage`, `requiredFlags`, `flags`, `schemaIds`, `inputs`,
  `commit`, `produces`, `options` and `example`. A step's command may belong to a companion
  binary, which the catalog names without linking or running. The first entry is the
  change-compiled E2E chain (CCE-V0-118): manifest, `corvint flows appmap build`, the committed
  manifest, then `corvint-test e2e` catalog, harvest (optional), obligations, compile, prove and
  shadow. The schema stays `corvint-capability-catalog/0`, because the member is additive
  (the CAP-V0-022 precedent).
- `CAP-V0-031`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1784) Every
  workflow field MUST be derived from the code declaration (`capcatalog.WorkflowSource`) or from
  that step's help text, which is the text the step's `--help` prints. That help is a summary line
  and the `Usage:`, `Inputs:`, `Commit:`, `Produces:`, `Options:` and `Example:` sections.
  `requiredFlags` follows the CAP-V0-022 and CAP-V0-023 rule over the step's usage forms. `flags`
  lists every option in its usage and options, with `--root` kept for a companion command, whose
  own flag it is. `schemaIds` lists every schema its help names. The CAP-V0-018 guide check counts
  a workflow step's schemas as named. Failure mode: a hand-written chain disagrees with the help a
  step prints.
- `CAP-V0-032`: (accepted under the owner's standing auto-accept (2026-10-11); GH-1784) A step's
  `requires` MUST name only earlier steps of the same workflow, every step after the first MUST
  require one, and `order` MUST be the 1-based position, so the declared order is an adoption
  order. Failure mode: an agent reads the chain out of order and runs a step before its inputs
  exist.

- `CAP-V0-040`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685, owner
  request 2026-10-25 "ensure that llms can easily discover all functionality for corvint and its
  tooling, and this stays the case as things are further developed") `corvint`, `corvint-tasks`
  and `corvint-test` MUST each derive one `corvint-command-tree/0` document (`internal/clitree`)
  from the help their binary already prints: `corvint` from every authored help topic,
  `corvint-tasks` from each verb's command help, and `corvint-test` from `commandHelpText`. Each
  node is one subcommand at any depth, with `path`, `command`, `summary`, `usage` (its synopses),
  `flags`, `required`, `example` and `help` (the invocation that prints its help). A node that no
  help synopsis names cannot exist, and a hand-maintained list of commands is not an input.
- `CAP-V0-041`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685)
  `clitree.ScanDirs` MUST parse a binary's dispatch sources (Go AST, no build or execution) and
  return every word a subcommand switch, comparison or table lookup matches at a fixed argument
  position, and every option a flag set declares or an argument loop matches, each with the first
  file and line that matches it, including through a `var` or `:=` alias of an argument or a flag
  set.
- `CAP-V0-042`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685) Every node
  of the three trees MUST answer `--help` and `-h` with its own help and exit 0, reading no stdin
  and opening no repository: the authored topic when one documents exactly that command, otherwise
  the node's rendered help (`clitree.Render`) followed for `corvint` by a pointer to the enclosing
  topic. A group with one subcommand may print that subcommand's topic (`corvint harness --help`).
  The `corvint-test e2e` group and steps print their CCE-V0-118 step help followed by the node's
  `Required:` line (and, for the group, its example). A table-driven test
  over every node of each tree proves it, and `corvint-test`'s runs the built binary.
- `CAP-V0-043`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685) Every word
  and option CAP-V0-041 finds MUST be a node word, a flag of some node, a global flag, or an entry
  of the binary's `commandTreeInternal` list. Each internal entry (a worker the binary starts
  itself, a hook entry point an installed adapter calls, a test fixture, a hook event value, an
  alternative spelling) MUST carry its reason; `clitree.Problems` rejects an entry without a word
  or a reason.
- `CAP-V0-044`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685) Every
  node's help MUST name its required inputs (`Required:`, or `none`) and at least one example.
  `required` lists the inputs every own synopsis names outside optional groups, with their value
  placeholders. A node whose help has no example gets one derived from its first synopsis
  (`clitree.MinimalExample`); `corvint-tasks` command help carries both as `required` and
  `example` members.
- `CAP-V0-045`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685) Each tree
  MUST be committed as `internal/capcatalog/trees/BINARY.json`, byte-equal to what its binary
  prints (`make capability-catalog` regenerates them), and `corvint help --json` MUST link each
  through the additive `commandTrees` member (binary, printing command, snapshot path). Each
  binary's own test fails when its snapshot is stale.
- `CAP-V0-046`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685) `corvint
  help --tree`, `corvint-tasks help --json` and `corvint-test help --json` MUST print the tree,
  and each binary's root help MUST name that command.
- `CAP-V0-047`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685)
  `tools/capabilities-check` MUST fail, with the binary, the word or option and its source line,
  when a snapshot is malformed, names no `cmd/` binary or has a node without a synopsis, example or
  help (`clitree.Problems`), and when CAP-V0-043 finds a dispatched subcommand or option the tree
  lacks. Its MCP tool scan MUST cover every `cmd/` binary whose non-test source answers
  `tools/list` in an interpreted or raw string literal, whatever its name, so a new MCP server's
  tool needs its catalog row.
- `CAP-V0-048`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685) Every other
  `cmd/` binary MUST have a row in `internal/capcatalog/CLI-RATCHET.tsv` (`binary<TAB>reason`), and
  the check MUST fail on a binary with neither a snapshot nor a row, on a row for a binary that now
  has a snapshot or no longer exists, on a row for a binary outside `ratchetBaseline` (the 52
  binaries the ratchet held when introduced), and on more rows than `ratchetCeiling`. A test pins
  the row count to the ceiling, so the ratchet can only shrink and a new binary ships its command
  tree.
- `CAP-V0-049`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685) A root
  `llms.txt` MUST be the single LLM entry point: it links `docs/CAPABILITIES.md`, the three
  command-tree commands and snapshots, `corvint help --json`, `corvint help guide` and the adoption
  guides. `docs/CAPABILITIES.md` MUST describe the command trees, and the shipped skill and adapter
  text keeps routing to it (CAP-V0-005).
- `CAP-V0-050`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685) `AGENTS.md`
  MUST state that new functionality ships with its help and catalog entry and that the
  capabilities check enforces it. `CAP-V0-051` to `CAP-V0-059` are reserved for this slice and
  unassigned.
- `CAP-V0-970`: (experimental under the standing lane auto-accept; V1-3286; amends `CAP-V0-040`)
  The help parser (`clitree.Synopses`) MUST NOT read an example as a synopsis: a line of an
  `Example:` or `Examples:` section (the header line, or the indented block under it up to the
  next unindented line, as `CAP-V0-955` defines the section; batch 1101a, 2026-11-01, combined the
  two V1-3286 implementations) is skipped, and the labelled forms (an inline `usage:` line or
  the indented block under a bare `Usage:`) come before the other synopsis lines, so a node's first
  usage line is the one its help labels wherever an indented example sits, before or after Usage.
  Synopsis groups outside a Usage block (a titled group of indented forms) stay synopses. Failure
  mode: an example written outside an Example section and outside the Usage block is still read
  as a form, after the labelled ones. Evidence: `TestCAPV0970SynopsesTakeTheUsageLineNotAnExample`
  (`internal/clitree`) and the regenerated `internal/capcatalog/trees/corvint.json`, where example
  lines leave the `flows appmap`, `flows appmap build`, `mcp`, `repro` and `repro record` usage
  (and `repro record` loses the `--path` flag its example contributed). Rollback: restore the
  previous `Synopses` and regenerate the snapshots.
- `CAP-V0-060`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3212) When the
  first word of a `corvint-tasks` command is not a verb, the unknown-verb warning MUST name, as
  `corvint-tasks VERB --help`, every implemented verb whose last word is that word (a guessed
  `heartbeat --help` names `attempt heartbeat`), and keep the plain `corvint-tasks help` pointer
  hint when none matches; the outcome stays `ERROR`. A test walks every implemented verb except
  `help`, whose `--help` is the root help (CAL-V0-047), with `--help` and fails on an unknown-verb
  result or help without usage, flags or an example.
- `CAP-V0-950`: (experimental under the standing lane auto-accept; V1-3701; extends `CAP-V0-015`
  and `CAP-V0-045`) `corvint help --json COMMAND`, for any top-level `COMMAND`, MUST print one
  compact, LF-terminated `corvint-capability-command/0` object of at most 5,119 bytes:
  `catalogSchema`, the command's `corvint help --json` entry unchanged except that
  `writeStatements` keeps only the leading sentences that fit, `writeStatementsOmitted` counting
  the rest (which `corvint help COMMAND` and the full catalog still carry), and `fullCatalog`. An
  unknown `COMMAND` exits 2 naming every command. `corvint-tasks help --tree` and `corvint-test help
  --tree` MUST print exactly their `help --json` command tree, so all three binaries accept
  corvint's `help --tree` spelling; `help --json` stays. Non-goals: nested-operation slices, paging
  the full catalog. Falsifier: a command whose slice exceeds the bound, a slice whose entry differs
  from the catalog's other than the omitted statements, or a `--tree` that differs from `--json`.
- `CAP-V0-700`: (experimental under the standing lane auto-accept; V1-3737, V1-3499) Every companion
  binary a release ships beside `corvint` and `corvint-tasks` (`corvint-console`, `corvint-update`,
  `corvint-mcp`, `corvint-docs-mcp`, `corvint-test-validity-mcp`, `corvint-corpus-mcp`,
  `corvint-dashboard-snapshot`, `corvint-web-flows`) MUST answer, through `internal/companioncli`,
  before it parses anything else: `--help`, `-h` and `help` with one usage text starting
  `usage: BINARY` and naming an `Example: BINARY ...`; `help --json` with
  `corvint-companion-help/0` `{binary, version, usage[]}`, the usage lines; `--version` with the one
  line `BINARY VERSION`; and `--version --json` with `corvint-companion-version/0` `{binary,
  version, commit, commitState}` from the binary's own build information (`unknown` without VCS
  stamping). Each form writes stdout only, exits 0, and reads no repository, port or network.
  `corvint-dashboard-snapshot`, which has a command tree (`CAP-V0-972`), answers the three help
  forms and `help --json` from that tree and takes only the two `--version` forms from
  `internal/companioncli`.
  `VERSION` is `0.1.0-experimental` until a companion ships its own release line, so the MCP
  servers' existing `--version` bytes are unchanged. Non-goals: command trees for these binaries
  (`CLI-RATCHET.tsv` keeps them); per-subcommand help. Falsifier: a companion that starts work,
  exits non-zero or writes stderr on one of the five forms.
- `CAP-V0-701`: (experimental under the standing lane auto-accept; V1-3737, V1-3499)
  `corvint-console` MUST refuse any positional argument with `unexpected argument "WORD"` and exit
  2 before it reads a repository or listens; `corvint-update` MUST refuse an action other than
  `check`, `apply` or `rollback` (including none) with its usage on stderr and exit 2 before it reads
  state, and its usage names every action, `--allow-network` and `--component core|tasks`.
- `CAP-V0-820`: (experimental under the standing lane auto-accept (2026-11-01); V1-3902; refines
  `CAP-V0-060`) An unknown verb, at the first word or after any verb group (`ticket`, `queue`,
  `plan`, `policy`, `service`, `slots` and the rest), MUST
  exit nonzero with outcome `ERROR` and the stable code `MALFORMED`, so a caller tells a typo from
  a help request by its codes alone. The envelope's command names the group and the unknown word
  when that word is an identifier (`help` stands in otherwise), and the warning quotes the word and
  suggests the nearest implemented verb by edit distance (at most 2, or a third of the word's
  length when longer; no verb within that distance means no suggestion) after the CAP-V0-060 last-word routes; with neither, it keeps the plain
  `corvint-tasks help` pointer.
- `CAP-V0-200`: (experimental under the standing lane auto-accept (2026-10-30); V1-3153; refines
  `CAP-V0-041` and `CAP-V0-047`) The dispatch scan MUST NOT read as this binary's arguments two
  kinds of argument-named slice. (1) An argument-named `[]string` parameter of a package
  function that never receives the command line: the function is not exported, `main` or
  `init`, is called in the package and named nowhere but as a callee, and every call passes, at
  that position, only a struct field other than `Args` (a recorded invocation such as
  `wholeGoTest(invocation.Argv)`), a literal, an `append` of those, or a caller `[]string`
  parameter, whatever its name, that is not shown to receive the command line, computed to a fixed
  point, so `Run(input []string) { routed(input) }` carries the command line into `routed`; any other argument (a local, a closure
  parameter, a call result) counts as the command line. (2) A slice a function body hands whole,
  sliced, spread or through `append` to `exec.Command`, `exec.CommandContext` or `syscall.Exec`,
  assigns to a `Cmd`'s `Args` field or sets as `Args:` in a composite literal, or binds to a
  `[]string{...}` literal; a single element (`exec.Command("xdg-open", args[1])`) is a string,
  not an argv, and excludes nothing. A name the body binds from `os.Args`, a selector's `Args`,
  an `Args()` call or an `append` of one is never excluded, so a binary that re-executes itself
  is still scanned. A name resolves in its innermost enclosing function, so a closure's
  parameter never hides the same name in the function around it. Non-goals: type-checked data
  flow; following a second name (`a := argv` excludes only `a`). Failure mode: a function that
  dispatches on an argument parameter and also execs that whole parameter hides its dispatch
  words. The scan of the three committed command trees finds the same words and flags as before,
  with `cmd/corvint`'s `wholeGoTest` parameter named `argv` again. Rollback: remove
  `childargv.go` and the `childArgv` predicate, and rename that parameter back to `command`.
- `CAP-V0-930`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685, 2026-10-31)
  `corvint-cem-candidate` and `corvint-cem-experiments` MUST each ship a command tree derived from
  the usage text they print, answer `help --json` with it, keep it committed byte for byte under
  `internal/capcatalog/trees/` (regenerated by `make capability-catalog`), be linked from the
  `commandTrees` member of `corvint help --json`, `llms.txt` and the `docs/CAPABILITIES.md`
  Command trees section, and leave `CLI-RATCHET.tsv`, whose `ratchetCeiling` falls from 52 to 50.
  Each tree's dispatch sources are the binary's own `cmd/` directory, so `tools/capabilities-check`
  fails on a verb or option either binary matches that its tree lacks (CAP-V0-047).
- `CAP-V0-931`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685, 2026-10-31)
  `clitree.Answer` MUST answer, before any flag parsing, `help --json` with the encoded tree,
  `help` alone and a lone `--help` or `-h` with the binary's root help, `help PATH...` with that
  node's `Render`, and any invocation whose leading words name a node and whose arguments carry
  `--help` or `-h` before `--` with that node's `Render`, all with exit 0; `help` with an unknown
  path reports an error naming `BINARY help`. Any other invocation, including an empty argument
  list, is not answered, so the binary's existing dispatch, refusals and exit codes are unchanged.
- `CAP-V0-932`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685, 2026-10-31)
  Each of the two binaries' root help MUST list every node with its summary and name `help --json`,
  and each binary's own test MUST run the binary's `command` entry point in process for every node
  with `--help` and `-h` (exit 0, stdout equal to `Render`, empty stderr) and fail when its committed snapshot is stale.
  Non-goals: command trees for the remaining 50 ratchet binaries (each leaves the ratchet in its
  own change); changing either binary's verbs, flags, refusals or JSON output. Failure modes: the
  usage text states which flags a verb needs, but the single shared flag set of
  `corvint-cem-experiments` still parses every flag on every verb, so a flag the synopsis omits for
  a verb is accepted there and ignored, as before. Rollback: restore the two ratchet rows and the
  ceiling of 52, delete the two snapshots, their `commandTreeRefs`, `llms.txt`, Makefile and
  Command trees entries, the two `help.go` files and `clitree.Answer`.
- `CAP-V0-972`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685, 2026-11-01)
  Every ratchet binary whose dispatch code matches a subcommand word MUST ship a command tree:
  `corvint-corpus-republish`, `corvint-dashboard-snapshot`, `corvint-go-test-provider`,
  `corvint-intake`, `corvint-js-test-provider`, `corvint-postmerge-connect`,
  `corvint-postmerge-metrics`, `corvint-postmerge-workflow`, `corvint-pulse` and
  `corvint-ruby-test-provider` each derive one from a synopsis text in the binary that names every
  subcommand and every option its dispatch code matches, answer `help --json` with it, keep it
  committed byte for byte under `internal/capcatalog/trees/` (regenerated by `make
  capability-catalog`), are linked from the `commandTrees` member of `corvint help --json`,
  `llms.txt` and the `docs/CAPABILITIES.md` Command trees section, and leave `CLI-RATCHET.tsv`,
  whose `ratchetCeiling` falls from 50 to 40. Each tree's dispatch source is the binary's own
  `cmd/` directory, so `tools/capabilities-check` fails on a word or option the binary matches that
  its tree lacks (CAP-V0-047). A synopsis may lead with required options before the subcommand
  word, each an unbracketed `--option VALUE` pair (`corvint-pulse --root ROOT serve --stdio`): the
  builder skips them when it finds the node path, as it skips a bracketed preamble, and lists them
  in the node's `required` and `flags`. A synopsis names only the options its subcommand accepts,
  never prose that reads as options.
- `CAP-V0-973`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685, 2026-11-01)
  `clitree.Help.Serve` MUST answer exactly the invocations `clitree.Answer` answers (CAP-V0-931),
  before the binary's own dispatch, with exit 0, or exit 2 and one stderr line for `help` with an
  unknown path. The root help is the binary's authored usage when it has one, otherwise a
  generated title and synopsis list, followed by every node with its summary and the `help --json`
  pointer. A node's help is its `Render` followed, when the binary has one, by that node's authored
  detail (its flag defaults), so help a binary printed before keeps its content and moves to
  stdout. Every other invocation is not answered, so the binary's dispatch, refusals and exit codes
  are unchanged.
- `CAP-V0-974`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2685, 2026-11-01)
  `clitreetest.CheckCompanion` MUST, in each of the ten binaries' own tests, run the binary's help
  entry point in process for every node with `--help` and `-h` (exit 0, stdout starting with
  `Render`, empty stderr), check that the root help lists every node and names `help --json`, that
  `help` with an unknown path exits 2, and fail when the committed snapshot is stale. Non-goals:
  command trees for the 40 binaries still on the ratchet, which match no subcommand word (16
  analyzer processes, 4 folded shims, 4 MCP servers and 16 single-mode tools whose options alone
  remain ungated by a tree); changing any binary's subcommands, flags, refusals or JSON output.
  Failure modes: a synopsis states which options a subcommand takes, but a binary that parses one
  shared flag set still accepts an option the synopsis omits for that subcommand, as before; a
  subcommand reached only through a computed string is not found by the scan (CAP-V0-041).
  Rollback: restore the ten ratchet rows and the ceiling of 50, and delete the ten snapshots, their
  `commandTreeRefs`, `llms.txt`, Makefile and Command trees entries, the ten `help.go` files,
  `clitree.Help` and `internal/clitree/clitreetest`.
- `CAP-V0-955`: (accepted under the owner's standing auto-accept (2026-11-01); V1-3286; refines
  `CAP-V0-040`) `clitree.Synopses` MUST NOT read an indented line under an example header (an
  unindented line ending in `:` that contains `example`, such as `Example:`, `Examples:` or
  `For example:`) as a synopsis, before or after the Usage block; the example block ends at the next
  unindented line. So an example command never becomes a node's usage, flag or derived example.
  Rollback: drop the example-block skip and regenerate the command trees.
- `CAP-V0-965`: (experimental under the standing lane auto-accept (2026-11-01); V1-3661; refines
  CAP-V0-961) A command row's `Subcommands` sentence MUST have a typed structure: every backticked
  span at parenthesis depth zero in it is exactly one subcommand name (`^[a-z][a-z0-9-]*$`); a
  span holding a flag, an argument or an example is a `subcommand-span-malformed` finding naming
  the verb, the span and the row line, and such detail goes inside parentheses, which the reader
  skips. CAP-V0-961 then reads names rather than relying on the first-word prose convention.
  Rollback: drop `checkSubcommandSpans` and its call; the CAP-V0-961 reader is unchanged.
- `CAP-V0-650`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3730, 2026-11-01)
  The root-help parsers (`capcatalog.Build` and `tools/capabilities-check`) MUST accept the job
  groups of CCF-V1-008: every header after the Core group starts `Experimental`, a line naming
  verbs separated by ` | ` labels each verb, and a Core header after an experimental one fails. The
  registry's `HiddenCommands` verbs MUST appear in the catalog and in the capabilities inventory as
  experimental with their gloss, though root help does not list them. Rollback: drop the family
  split and the hidden-verb merge and regenerate the catalog.
- `CAP-V0-979`: (experimental under the standing lane auto-accept (2026-11-01); V1-3679; refines
  CAP-V0-041 and CAP-V0-043) The CAP-V0-041 scan MUST NOT report a key of an event table (a table
  whose identifier contains `event`, case-insensitively, such as `claudeContextEvents`) as a
  dispatched word, whether an argument indexes the table or is compared with that key
  (`arguments[1] == "session-start"`); such a value needs no `commandTreeInternal` entry. A word an
  argument is compared with that no event table holds is still dispatch. Residual risk: a real
  subcommand spelled like an event key in the same package is not checked. Rollback: drop
  `eventTable` and the event-key filter; event values again need internal entries.

## Boundaries and failure modes

Non-goals: generating `docs/CAPABILITIES.md` from code (the CAP-V0-015 catalog is a separate generated document; the page stays hand-written and checked); checking flags, output schemas or the prose of a row; running any listed command; ranking capabilities; replacing a spec, a help page or
the deep guides the catalog links to.

Failure modes: subcommands are checked to depth two only (`VERB SUB`, CAP-V0-960); deeper nodes
such as `docs corpus build` are left to the command tree (CAP-V0-047); a removed subcommand stays
listed unnoticed when the row names it outside its `Subcommands` sentence (CAP-V0-961); a subcommand word that appears in
some other backticked span of the row counts as listed; MCP profile membership and binary class are prose and unchecked;
the MCP scan counts every `"corvint.<name>"` literal in the scanned source, so a non-tool literal
of that shape would demand a row (none exists today) and a tool name built at run time would be
missed; maturity is checked for commands and README rows only, so a binary or tool row's maturity
can drift from its spec; a refusal code stays accepted while any string literal equals it, even
one that is no longer returned (CAP-V0-962), and a code written in prose without
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

CAP-V0-040 to CAP-V0-050 non-goals: command trees for the binaries on the CLI ratchet (each leaves
it by shipping `help --json` and a snapshot); output schemas, exit codes or flag value types in a
node; generating help text; checking that a node's summary or example is true; a CI step (the
check runs in the existing Go test shards).

CAP-V0-040 to CAP-V0-050 failure modes: the dispatch scan is lexical. It reads a switch,
comparison or lookup only on a value named like an argument slice (`arg*`, `rest`, `tokens`,
`tail`, `remaining`, `words`, `cmd`, or `.Args`), so a subcommand dispatched on a differently named
value, built at run time, or reached through a function value is missed, and a literal compared at
a fixed index that is not a subcommand demands a node or a reasoned internal entry (fail closed).
An option is matched only when a flag set declares it or an argument loop compares it, so an
option parsed by a third-party parser is missed. `required` is read from synopses, so an input the
help calls required only in prose is not listed (as CAP-V0-023). A derived example is the smallest
invocation the first synopsis allows, with placeholders, so it is runnable only after the
placeholders are filled. A node's help is the authored topic when one exists, so a topic that
documents several subcommands answers each of them with its whole text and a pointer, not a
per-subcommand page. The ratchet's binaries are gated only for MCP tools and catalog rows, not for
their subcommands or flags. A help change in any lane changes a snapshot, so a merge that changes
help without regenerating fails CAP-V0-045 until `make capability-catalog` is rerun. `llms.txt` is
checked for the views it names, not for the prose around them.

## Rollback

Delete `docs/CAPABILITIES.md`, `tools/capabilities-check`, the `capabilities-check` Makefile target
and `.PHONY` entry, the `tools/capabilities-check` entry in `.corvint/test-read-scopes.json`, the
README section and the routing lines, then this spec's README row and INDEX entry. No stored state
depends on them. Reverting a skill text change needs its own version bump, because the host-package
version gate refuses a content change at an unchanged version. To roll back only CAP-V0-007 to
CAP-V0-009, remove `parseSubcommands`, `ScanWords`, `checkSubcommands`, `checkRefusals` and the
Experimental maturity case with their tests, and narrow the test read scope back to `internal/mcp/`;
the catalog row edits stay correct. To roll back only CAP-V0-960 to CAP-V0-962, drop `treeSubcommands`, `MergeSubcommands`, `documentedSubcommands` and `checkStaleSubcommands`, restore the whole-word `ScanWords` match, and remove their tests. To roll back only CAP-V0-015 to CAP-V0-021, delete
`internal/capcatalog`, `cmd/corvint/help_catalog.go` and its test, the `capabilityOrPublicHelpTopic`
and `capabilityHelpText` calls and the root usage words in `cmd/corvint/help.go`, the two Makefile targets and `.PHONY`
entries, and the `docs/CAPABILITIES.md` pointer; no stored state depends on them.
To roll back only CAP-V0-030 to CAP-V0-032, delete `internal/capcatalog/workflow.go` and its
test, the `Workflows` fields of `Registry` and `Catalog` and the `workflows` derivation entry,
`cmd/corvint/help_e2e_workflow.go` and its test, the registry line and the `flows appmap build`
help topic in `cmd/corvint/help.go`, then regenerate the catalog. `corvint-test` reads the chain
from the catalog, so CCE-V0-118 is rolled back with it.
To roll back only CAP-V0-040 to CAP-V0-050, delete `internal/clitree`, `internal/capcatalog/trees/`,
`CLI-RATCHET.tsv`, `tools/capabilities-check/trees.go` and `entrypoint.go` with their tests and
calls in `run`, `cmd/corvint/command_tree.go`, `internal/tasks/cli/command_tree.go`, the
`commandHelpText` tree code in `cmd/corvint-test/help.go` and their tests, the `commandTrees`
member and its derivation entry, the `help --tree` and help-trailer calls, the root `llms.txt`
with its read-scope entry, the AGENTS.md rule and the `docs/CAPABILITIES.md` section, then rerun
`make capability-catalog`. No stored state depends on them.

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
| CAP-V0-960 | `treeSubcommands`, `MergeSubcommands` | `TestCAPV0960TreeSubcommandUndocumentedFails`, `TestCAPV0002RepositoryCatalogMatchesInventory` |
| CAP-V0-961 | `documentedSubcommands`, `checkStaleSubcommands` | `TestCAPV0961StaleDocumentedSubcommandFails`, `TestCAPV0961DocumentedSubcommandsParse` |
| CAP-V0-965 | `subcommandSpans`, `checkSubcommandSpans` | `TestCAPV0965SubcommandSpansAreNames` |
| CAP-V0-962 | `ScanLiterals`, `checkRefusals` | `TestCAPV0962RefusalCodeMatchesWholeLiteral`, `TestCAPV0009RemovedRefusalCodeFails` |
| CAP-V0-650 | `capcatalog.Registry.HiddenCommands`, `parseRootCommands`, `parseRootLabel`, `parseHidden`, `parseCommands` | `TestCAPV0002JobGroupedRootHelp`, `TestCAPV0019CommittedCatalogMatchesTheBinary`, `TestHiddenVerbsStayCallableButUnlisted` |
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
| CAP-V0-027 | `internal/capcatalog/AGENT-GUIDE.md` goals 8 to 11, `guideCommandLines` | `TestCAPV0027GuideTeachesDeliveredCapabilities`, `TestCAPV0018ShippedGuideMatchesTheLiveCatalog`; live runs recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-help-guide-noncapabilities.md` |
| CAP-V0-028 | `internal/capcatalog/AGENT-GUIDE.md` Non-capabilities, `NONCAPABILITY-STATUS.tsv` | `TestCAPV0028DeliveredItemsLeftTheNonCapabilities` |
| CAP-V0-029 | `capcatalog.Guide` (embedded), `NONCAPABILITY-STATUS.tsv` row | `TestCAPV0029GuideIsNotAnArchiveMember`, `TestCAPV0017HelpGuideServesTheShippedGuide` |
| CAP-V0-030 | `capcatalog.Workflow`, `Catalog.Workflows`, `cmd/corvint/help_e2e_workflow.go` (`e2eAdoptionWorkflow`), `capabilityRegistry`, `internal/capcatalog/catalog.json` | `TestCAPV0030HelpJSONCarriesTheE2EWorkflow`, `TestCAPV0019CommittedCatalogMatchesTheBinary` |
| CAP-V0-031 | `capcatalog.BuildWorkflow`, `stepSections`, `stepRequiredFlags`, `StepHelp`, `GuideProblems` (workflow schemas) | `TestCAPV0031WorkflowDerivedFromStepHelp`, `TestCAPV0031StepHelpRendersTheStep`, `TestCAPV0018ShippedGuideMatchesTheLiveCatalog` |
| CAP-V0-032 | `e2eAdoptionWorkflow` step `Requires` | `TestCAPV0032ShippedWorkflowRequiresEarlierSteps` |
| CAP-V0-040 | `clitree.Builder`, `cmd/corvint/command_tree.go` (`buildCommandTree`, `helpTopics`), `internal/tasks/cli/command_tree.go`, `cmd/corvint-test/help.go` (`commandHelpText`, `commandTree`) | `TestCAPV0040TreeDerivesFromHelp`, `TestCAPV0042EveryTreeNodeAnswersHelp` (cmd/corvint, internal/tasks/cli), `TestCAPV0042EveryNodeAnswersHelp` (cmd/corvint-test) |
| CAP-V0-041 | `clitree.ScanDirs`, `argumentSlice` | `TestCAPV0041ScanFindsDispatch` |
| CAP-V0-042 | `nodeHelpTopic`, `nodeHelpText`, `parentTopicPointer`, `clitree.Render`, `clitree.Find`, `clitree.LeadingWords`, corvint-test `helpRequest`, tasks command help | `TestCAPV0042FindRenderAndLeadingWords`, `TestCAPV0042EveryTreeNodeAnswersHelp` (cmd/corvint, internal/tasks/cli), `TestCAPV0042EveryNodeAnswersHelp` (built `corvint-test` binary), `TestCCEV0118SubcommandHelp` |
| CAP-V0-043 | `commandTreeInternal` (three binaries), `Tree.Words`, `Tree.AllFlags` | `TestCAPV0043WordsAndFlagsVocabulary`, `TestCAPV0047RepositoryCommandTreesMatchDispatch` |
| CAP-V0-044 | `clitree.MinimalExample`, `helpExampleTrailer`, `treeHelp` | `TestCAPV0040TreeDerivesFromHelp`, `TestCAPV0042EveryTreeNodeAnswersHelp` (cmd/corvint, internal/tasks/cli) |
| CAP-V0-045 | `commandTreeRefs`, `capcatalog.CommandTreeRef`, `internal/capcatalog/trees/*.json`, `make capability-catalog` | `TestCAPV0045CatalogLinksCurrentTreeSnapshots`, `TestCAPV0045TreeSnapshotIsCurrent`, `TestCAPV0046HelpJSONIsTheTree` |
| CAP-V0-046 | `help --tree` topic, `CommandTreeJSON`, corvint-test `helpRequest` and `rootHelp` | `TestCAPV0045CatalogLinksCurrentTreeSnapshots`, `TestCAPV0046HelpJSONIsTheTree`, `TestCAPV0045TreeSnapshotIsCurrent` |
| CAP-V0-047 | `CheckTrees`, `treeDrift`, `clitree.Problems`, `servesMCP` | `TestCAPV0047RepositoryCommandTreesMatchDispatch`, `TestCAPV0047SyntheticDriftFails`, `TestCAPV0047NewMCPServerToolNeedsRow` |
| CAP-V0-048 | `internal/capcatalog/CLI-RATCHET.tsv`, `readRatchet`, `ratchetCeiling`, `ratchetBaseline` | `TestCAPV0048RatchetMatchesCeiling`, `TestCAPV0047SyntheticDriftFails` (ratchet cases) |
| CAP-V0-049 | `llms.txt`, `CheckEntryPoint`, `docs/CAPABILITIES.md` Command trees section | `TestCAPV0049LLMsEntryPointLinksEveryView`, `TestCAPV0005RoutingPointersNameTheCatalog` |
| CAP-V0-050 | `AGENTS.md` Agent routing rule | `TestCAPV0050AgentsRuleNamesHelpCatalogAndGate` |
| CAP-V0-950 | `internal/capcatalog/slice.go` (`Slice`), `cmd/corvint` (`parseHelpInvocation`, `catalogCommandJSON`, root help), `internal/tasks/cli` (`Run`), `cmd/corvint-test` (`helpRequest`) | `TestCAPV0950EveryCommandSlicesUnderTheBudget` (internal/capcatalog), `TestCAPV0950HelpJSONCommandSlice` (cmd/corvint), the `help --tree` cases of `TestCAPV0046HelpJSONIsTheTree` (internal/tasks/cli) and `TestCAPV0045TreeSnapshotIsCurrent` (cmd/corvint-test) |
| CAP-V0-700 | `internal/companioncli` (`Answer`), the `main` of each named companion | `internal/companionrelease/companion_cli_contract_test.go`'s `TestCAPV0700CompanionsAnswerTheSharedContract` (all eight built binaries, five forms, outside a repository); `TestCAPV0700AnswerHandlesOnlyTheContractForms` (internal/companioncli) |
| CAP-V0-701 | `parseOptions` (`cmd/corvint-console`), `main` (`cmd/corvint-update`) | `TestCAPV0701CompanionsRefuseStrayArgumentsFirst` |
| CAP-V0-060 | `unknownVerbHint` (`internal/tasks/cli/cli.go`) | `TestCAPV0060_EveryVerbHelpAndUnknownWordRoutes` (internal/tasks/cli) |
| CAP-V0-820 | `unknownVerb`, `nearestVerbs` (`internal/tasks/cli/unknown_verb.go`) | `TestCAPV0820_UnknownVerbIsCodedAndSuggestsNearest`, `TestCAPV0820_VerbGroupsRefuseUnknownVerb`, `TestCAPV0820_NoSuggestionBeyondThreshold` (internal/tasks/cli) |
| CAP-V0-970 | `clitree.Synopses` (`internal/clitree/clitree.go`), `internal/capcatalog/trees/corvint.json` | `TestCAPV0970SynopsesTakeTheUsageLineNotAnExample` |
| CAP-V0-200 | `internal/clitree/childargv.go` (`childArgvOf`, `nonCommandLineParams`, `childNames`, `commandLine`), `sliceName`; `cmd/corvint/test_whole_reason.go` `wholeGoTest(argv)` | `TestCAPV0047ChildArgvIsNotDispatch` (fails on the base scanner: `flags [--exec-child --no-pager --real-flag -run]`; covers the `wholeGoTest` helper, exec, `CommandContext`, `Cmd.Args`, `syscall.Exec`, `append`, literals, and keeps a forwarded parameter, a handler-table function, a single exec element, a shadowing closure and a re-executed `os.Args`/`flag.Args()` scanned), `TestCAPV0041ScanFindsDispatch`, `TestCAPV0047RepositoryCommandTreesMatchDispatch`; `make capabilities-check` and `make capability-catalog-check` pass |
| CAP-V0-930 | `cmd/corvint-cem-candidate/help.go`, `cmd/corvint-cem-experiments/help.go`, `internal/capcatalog/trees/corvint-cem-{candidate,experiments}.json`, `commandTreeRefs`, `ratchetCeiling` | `TestCAPV0930TreeSnapshotIsCurrent` (both binaries), `TestCAPV0045CatalogLinksCurrentTreeSnapshots`, `TestCAPV0048RatchetMatchesCeiling`, `TestCAPV0047RepositoryCommandTreesMatchDispatch`, `TestCAPV0049LLMsEntryPointLinksEveryView` |
| CAP-V0-931 | `clitree.Answer` | `TestCAPV0931AnswerRoutesHelpOnly` (internal/clitree) |
| CAP-V0-932 | `rootHelp` in both `help.go` files | `TestCAPV0932EveryNodeAnswersHelp` (both binaries) |
| CAP-V0-955 | `internal/clitree/clitree.go` (`Synopses`) | `TestSynopsesSkipsIndentedExamples` (examples after, before and directly above Usage); regenerated `internal/capcatalog/trees/corvint.json` drops the `docs` example lines and the stray `-m` flag |
| CAP-V0-979 | `internal/clitree/scan.go` (`eventTable`, the event-key filter in `scanPackage`) | `TestCAPV0979EventValuesAreNotDispatch` (internal/clitree), `TestCAPV0979EventValuesAreNotDrift` (tools/capabilities-check) |
