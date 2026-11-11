package main

import (
	"regexp"
	"slices"
	"strings"
)

var cemHelpActions = map[string]bool{
	"begin": true, "prepare": true, "cite": true, "mark": true,
	"status": true, "verify": true, "report": true,
	"cover": true, "verify-stable": true,
}

// helpSubcommands are the retired oracle's nested argparse choices (GPK-V0-062): a token after the
// command that is neither one of them nor --help is an invalid choice argparse reports before help.
var helpSubcommands = map[string]map[string]bool{
	"cem": cemHelpActions,
}

// helpBooleanFlags are the public command options that take no value, so a --help after one is
// still help. Every other option is scanned as taking one value (GPK-V0-062).
var helpBooleanFlags = map[string]bool{
	"--apply": true, "--check": true, "--dry-run": true, "--enable": true,
	"--full-receipt": true, "--if-stale": true, "--json": true, "--preview": true,
	"--replace": true,
}

// argparseNegativeNumber is argparse's _negative_number_matcher: such a token is a value, not an option.
var argparseNegativeNumber = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`)

func parseHelpInvocation(arguments []string) (string, bool, error) {
	index := 0
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", false, nil
			}
			index += 2
		} else {
			index++
		}
	}
	rest := arguments[index:]
	directTopic := ""
	if len(rest) == 2 && rest[0] == "help" && rest[1] == "--json" {
		return "catalog-json", true, nil
	}
	if len(rest) == 3 && rest[0] == "help" && rest[1] == "--json" { // CAP-V0-950: one command's catalog entry.
		if !slices.Contains(topLevelCommands, rest[2]) {
			return "", true, unknownCatalogCommand(rest[2])
		}
		return catalogCommandHelpPrefix + rest[2], true, nil
	}
	if len(rest) == 2 && rest[0] == "help" {
		directTopic = capabilityOrPublicHelpTopic(rest[1])
		// A nested topic passed as the one argument the refusal lists it as
		// ("docs corpus") resolves like its separate-word form (DCP-V1-057).
		if command, subcommand, spaced := strings.Cut(rest[1], " "); directTopic == "" && spaced {
			directTopic = nestedHelpTopic(command, subcommand)
		}
	} else if len(rest) == 3 && rest[0] == "help" {
		directTopic = cemOrNestedHelpTopic(rest[1], rest[2])
	}
	switch {
	case len(rest) > 0 && (rest[0] == "--help" || rest[0] == "-h"), len(rest) == 1 && rest[0] == "help":
		return "root", true, nil
	case directTopic != "":
		return directTopic, true, nil
	case len(rest) > 1 && rest[0] == "help":
		// `corvint help flows appmap build` names one subcommand of the tree (CAP-V0-042) and
		// prints what `corvint flows appmap build --help` prints (CAP-V0-056).
		if _, ok := findTreeNode(rest[1:]); ok {
			if rest[1] != "help" {
				if topic, requested, err := parseHelpInvocation(append(slices.Clone(rest[1:]), "--help")); requested && err == nil {
					return topic, true, nil
				}
			}
			return nodeHelpPrefix + strings.Join(rest[1:], " "), true, nil
		}
		return "", true, unknownHelpTopic(rest[1:])
	}
	topic := commandHelpTopic(rest)
	if topic == "" || !helpFlagRequested(rest[1:], topic) {
		return "", false, nil
	}
	if node := nodeHelpTopic(rest, topic); node != "" {
		return node, true, nil
	}
	return topic, true, nil
}

// nestedHelpTopics are the two-word `corvint help COMMAND SUBCOMMAND` topics, in the order the
// unknown-topic refusal lists them (DCP-V1-057).
var nestedHelpTopics = [][2]string{}

func nestedHelpTopic(command, subcommand string) string {
	for _, nested := range nestedHelpTopics {
		if nested[0] == command+" "+subcommand {
			return nested[1]
		}
	}
	return ""
}

// unknownHelpTopic refuses `corvint help TOPIC` with argparse's invalid-choice convention, naming
// every valid topic: each public command, then each nested topic (DCP-V1-057). The topic is
// rendered as a Python repr, so a quote or control character in it is escaped.
func unknownHelpTopic(topic []string) error {
	quoted := make([]string, 0, len(topLevelCommands)+len(nestedHelpTopics))
	for _, command := range topLevelCommands {
		quoted = append(quoted, pythonRepr(command))
	}
	for _, nested := range nestedHelpTopics {
		quoted = append(quoted, pythonRepr(nested[0]))
	}
	return argumentError("unknown help topic: " + pythonRepr(strings.Join(topic, " ")) + " (choose from " + strings.Join(quoted, ", ") + ")")
}

// commandHelpTopic names the help topic of a public command invocation, or "" when the command is
// unknown or its nested choice is invalid.
func commandHelpTopic(rest []string) string {
	if len(rest) == 0 {
		return ""
	}
	topic, known := publicHelpTopic(rest[0])
	choices, nested := helpSubcommands[rest[0]]
	switch {
	case !known:
		return ""
	case len(rest) > 2 && rest[0] == "flows" && rest[1] == "appmap" && rest[2] == "build":
		// The build step's own help: manifest, commit rules and the e2e chain (CCE-V0-118).
		return flowsAppmapBuildHelpTopic
	case nested && len(rest) > 1 && rest[1] != "--help" && rest[1] != "-h" && !choices[rest[1]]:
		return ""
	case rest[0] == "cem" && len(rest) > 1 && choices[rest[1]]:
		// One cem action's help, not the whole family's (CEM-V1-340).
		return cemActionHelpPrefix + rest[1]
	}
	return topic
}

// helpFlagRequested applies argparse's left-to-right scan (GPK-V0-062, GPK-V0-064): --help or its
// -h alias before "--" is a help request unless an option that takes a value reaches it first,
// which argparse refuses with "expected one argument" and so never prints help.
func helpFlagRequested(tokens []string, topic string) bool {
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		switch {
		case token == "--":
			return false
		case token == "--help", token == "-h":
			return true
		case !argparseOptionLike(token), strings.Contains(token, "="), helpBooleanFlags[token]:
			continue
		case index+1 < len(tokens) && !argparseOptionLike(tokens[index+1]):
			index++
		default:
			return false
		}
	}
	return false
}

// argparseOptionLike mirrors argparse's _parse_optional classification for tokens it cannot match
// to a declared option: a lone "-", a negative number, or a token containing a space is a value.
func argparseOptionLike(token string) bool {
	return len(token) > 1 && token[0] == '-' && !argparseNegativeNumber.MatchString(token) && !strings.Contains(token, " ")
}

func publicHelpTopic(command string) (string, bool) {
	for _, known := range topLevelCommands {
		if command == known {
			return command, true
		}
	}
	return "", false
}

// helpText is a help topic as printed: a node topic renders one subcommand's own help, and a
// command topic gains its required inputs and example when its text has none (CAP-V0-044).
func helpText(topic string) string {
	if path, ok := strings.CutPrefix(topic, nodeHelpPrefix); ok {
		return nodeHelpText(path)
	}
	if command, ok := strings.CutPrefix(topic, catalogCommandHelpPrefix); ok {
		return catalogCommandJSON(command)
	}
	text := topicText(topic) + helpExampleTrailer(topic)
	if _, hidden := hiddenCommands[topic]; hidden {
		text += hiddenHelpNote
	}
	return text
}

// topicText is a help topic's authored text, the source the command tree derives from.
func topicText(topic string) string {
	if action, ok := strings.CutPrefix(topic, cemActionHelpPrefix); ok {
		return cemActionHelp(action)
	}
	switch topic {
	case flowsAppmapBuildHelpTopic:
		return flowsAppmapBuildHelp()
	case "catalog-json":
		return capabilityCatalogJSON()
	case "docs":
		return docsHelp
	case "root":
		return rootHelp
	case "impact":
		return impactHelp
	case "cem":
		return cemHelp
	case "review":
		return repositoryGuidanceHelp
	case "affected":
		return affectedHelp
	case "flows":
		return flowsHelp
	case "how":
		return howHelp
	case "runs":
		return runsHelp
	case "mcp":
		return mcpHelp
	case "test":
		return testHelp
	case "tests":
		return testsHelp
	case "watch":
		return watchHelp
	case "context":
		return taskContextHelp + cpuProfileHelpNote
	case "instructions":
		return instructionsHelp + cpuProfileHelpNote
	case "index":
		return indexHelp
	case "init", "adopt":
		return activationHelp
	case "test-evidence":
		return testEvidenceHelp
	case "test-validity":
		return testValidityHelp
	case "test-plan":
		return testPlanHelp
	default:
		return capabilityHelpText(topic)
	}
}

// rootHelpMaxBytes bounds root help (CCF-V1-009): it is a verb index, and each verb's usage,
// options and writes live in its own COMMAND --help topic.
const rootHelpMaxBytes = 4096

const rootHelp = `Corvint: proof-carrying context for coding agents

Usage:
  corvint [--root PATH] COMMAND [OPTIONS]
  corvint COMMAND --help | corvint help COMMAND | corvint help --json [COMMAND]|--tree|guide
  corvint --version [--json]

Commands:
  Core (decision 0332; only the modes and profiles CCF-V1 lists are frozen):
    init  alias of adopt
    adopt  bounded repository inventory; --setup lists setup gaps
    index  write the index snapshot
    context  path brief of a file or directory
    impact  path or range impact
    affected  affected-test plan
    cem  Change Evidence Map tool
  Experimental, context and review:
    instructions  instruction load set of a tracked file (IID-V0)
    review  change review: affected plan and branch overlap (RGV-V0)
    docs  source-derived documentation (SDD-V0)
    flows  application flows and assertions (AFU-V0)
    how  how do I GOAL: alias of flows appmap how (SKH-V0)
  Experimental, tests:
    test  affected tests (STR-V0)
    tests  setup, lint, duplicates (TSA-V0)
    watch  rerun the tests a save can affect (LPCV-V0)
    test-plan  test consolidation plan (TCN-V0)
    test-evidence  read retained test dependencies (RSP-V0)
    test-validity  project a live-test receipt (MTV-V0)
    runs  Playwright verdicts (PWR-V0)
  Experimental, agent hosts and local records:
    mcp  local read-only stdio MCP server (AHI)

Global options:
  --root PATH  Repository root (default: current directory).
  --version    Print the version and build (--json: the corvint-version/1 identity).
  --help       Print this help on stdout and exit 0.

Support boundary:
  Experimental verbs carry no stability promise; parentheses name the owning spec.
  Internal and historical verbs stay callable but unlisted (corvint help --json).
  Local-only; no network or telemetry. init, adopt and impact are
  qualified on Darwin and Linux only.
  init, adopt, impact and docs read without mutating repository or trace
  state. impact, docs and index best-effort append a bounded row to
  the self-observation ledger (.corvint/self-observations.jsonl) on an
  unsupported-* failure; a ledger storage failure never alters the response.
  cem begin, prepare, cite and mark write local CEM artifacts; cem report,
  index, and docs flows and maintain write local state (COMMAND --help).
`

// hiddenCommands are the dispatched internal or historical verbs that root help leaves out of
// its verb index (CCF-V1-008, V1-3730). Each stays callable with its own help topic, which says
// it is unlisted, and each value is its catalog line: a gloss and the owning spec prefix.
var hiddenCommands = map[string]string{}

// hiddenHelpNote ends the help topic of each hiddenCommands verb.
const hiddenHelpNote = "\nUnlisted in root help: an internal or historical verb that stays callable.\n"

const testValidityHelp = `Project test-level results through the shared test-validity shape.

Usage:
  corvint [--root PATH] test-validity [--receipt FILE [--route-stubs] | --discover | --static [PATH...]]
      [--producer]
  corvint [--root PATH] test-validity negate --provider ABSOLUTE_FILE --spec S --test T
      [--project P] (--step TITLE | --all-steps) [--max-runs N] [--baseline-repeat N]
      [external-server provider options]

FILE is one corvint-js-test-provider stdout document (at most 4 MiB). The command
recomputes, from its receipt member alone, one corvint-test-validity/0 document:
one projection per test and one run projection, each with five independent
axes (association, hygiene, freshness, execution, strength). The projections
the document already carries are never trusted. No axis is a pass/fail summary,
and a PASSED execution axis states no strength: strength stays NOT_MEASURED
until a mutation run supplies a witness.

Without --receipt there is no test-level evidence: tests is empty and every
run axis is UNSUPPORTED with reason no-input-supplied. --route-stubs makes every
JavaScript test carry routeStubs: a receipt with no route-stub producer record
then states NOT_PRODUCED with reason route-stub-producer-absent, and a profiled
receipt states route-stub-external-profile, instead of omitting the member. An unreadable, oversized,
malformed or receipt-less FILE exits 2 with no stdout. --producer appends one
"producer" member (version, build, commit, commitState; "unknown" or
"unstamped" when not stated) and leaves every other member unchanged.

--discover instead reads the newest completed provider document retained as a
regular *.json file directly under .corvint/test-evidence of the worktree (at
most 256 entries, 16 read attempts, no symlinks). Its bound digests are
compared with the worktree now: a mismatch makes freshness STALE, an identity
that cannot be recomputed makes it UNKNOWN, and only fully matched digests are
CURRENT. With no usable document every run axis stays UNSUPPORTED. For an
RSpec document it also reads each example's spec file whose Git blob still
equals the retained one: hygiene is ELIGIBLE, INELIGIBLE on a skipped, pending,
focused, shared-example-only or memoized-block-expectation finding, or
ABSTAINED when the file or line cannot be read; strength stays NOT_MEASURED
with the example-body expectation count or a described_class stub as its
reason, and sourceRules lists the findings. The command runs no test and never
writes. Experimental.

Every test carries location: its repository-relative file, declaring line and
literal titlePath where the provider or a bound source reading knows them, and
absent naming each missing member with one closed reason (an RSpec shared
example adds includedAt).

--static [PATH...] needs no retained run: it reads the committed HEAD RSpec
(_spec.rb) and JS/TS test files under each repository-relative PATH (default:
the whole tree) and projects one test per example. Hygiene comes from source
(skipped, pending, focused, fixme and assertion-free tests are named in
sourceRules); execution, freshness and strength stay NOT_OBSERVED. It reads Git
objects only and writes nothing. A PATH with no committed test file exits 2.

negate is the only mode that runs tests or writes. It execs the named
corvint-js-test-provider file (never a PATH lookup), which runs the one test
unfaulted --baseline-repeat times (1 to 5, default 1) against an externally
managed application on profile /0 or /1, then one derived network or DOM fault
per step (--step) or one joint run (--all-steps), within --max-runs (baselines
included; default 3 or 2). Retries are 0 and there is one worker. It prints one
corvint-step-negation/0 document, a human summary on stderr (a step with no
derivable fault reads "strength: UNPROVEN (manual control needed)"), and
retains the merged document under .corvint/strength-evidence. It repeats the
test's own side effects on the application and is not a sandbox. An
unqualified runtime tuple, owned server, /2, /3, --keep-reporters, freshness or
watch refuses before any run (exit 2, no stdout), as does a selection that
matches no test or several. Incomplete cleanup or a retention failure exits 1
after the document. Results are diagnostic: no strength axis joins them until
the live matrix is qualified. Experimental.
`

const activationHelp = `Compile one bounded mechanical repository inventory.

Usage:
  corvint [--root PATH] (init | adopt) [--authority-id ID] [--revision REV]
    [--exclude-prefix PATH] [--full-receipt]
  corvint [--root PATH] adopt --setup

init is an alias of adopt: both run the same inventory, and the output's tool
member names the verb that ran. --revision defaults to HEAD. --exclude-prefix is
repeatable. The default output is a bounded sealed summary; --full-receipt emits
the complete canonical receipt. Both activations are read-only and local-only.

Experimental (ADS-V0): adopt --setup takes no other option and reports the
repository's Corvint setup gaps (index snapshot, agent instructions, hooks,
ignored paths, declared test commands), each with its evidence and a suggested
fix. It writes nothing.
`

const affectedHelp = `Compile the affected-test selection plan for the dirty worktree.

Usage:
  corvint [--root PATH] affected [--full | --summary] [--require-clean]
          [--cache DIR | --no-cache] [--format json|text] [--fail-fast-order]
          [--accept-stale-binary]
  corvint [--root PATH] affected [--full] --path PATH [PATH ...] [--path PATH ...]
          [--paths-from FILE] [--worktree] [--cache DIR | --no-cache]
          [--format json|text]
  corvint [--root PATH] affected --by-path --path PATH [PATH ...] [--format json|text]
  corvint [--root PATH] affected [--full] --snapshot RECEIPT
  corvint [--root PATH] affected --snapshot RECEIPT --playwright-config PATH
  corvint [--root PATH] affected [--full | --summary] [--require-clean]
          --base FULL_COMMIT_ID [--format json|text] [--fail-fast-order]
  corvint [--root PATH] affected --base FULL_COMMIT_ID --line-index FILE [--line-test-timeout DURATION] [--line-exec PATH]
  corvint [--root PATH] affected [--base FULL_COMMIT_ID]
          --playwright-config PATH [--playwright-discovery FILE]
          [--playwright-env NAME[=VALUE] ...]
  corvint [--root PATH] affected discovery --playwright-config PATH
          --playwright-list FILE [--playwright-ungrepped-list FILE
          [--playwright-ungrepped-config PATH]]
  corvint [--root PATH] affected cache [--format json|doctor-plugin]
  corvint [--root PATH] affected audit --changes FILE [--accept-stale-binary]
  corvint [--root PATH] affected --ownership-audit
  corvint [--root PATH] affected jobs --base FULL_COMMIT_ID [--declaration PATH]
          [--accept-stale-binary]
  corvint [--root PATH] affected [--base FULL_COMMIT_ID] --provider RECORD
          [--provider RECORD ...] [--provider-command ARGV_JSON ...]
          [--repository ID=DIR ...] [--selection-profile strict|coverage]
  corvint [--root PATH] affected [--base FULL_COMMIT_ID] --provider FILE
          --selection-profile e2e-safe [--playwright-discovery FILE]
          [--repository ID=DIR --repository-base ID=FULL_COMMIT_ID ...]
  corvint [--root PATH] affected [--full] --base FULL_COMMIT_ID
          --appmap-impact REPORT [--appmap-impact REPORT ...]
          [--repository ID=DIR --repository-base ID=FULL_COMMIT_ID ...]

The command reads the Git worktree status, builds the multi-language unit graph
from source text, and writes one affected-plan/1 document to stdout: the
selector's plan (selected units with witnesses and test counts, an exclusion
summary, unknowns, scope) and provider.go.packages, the exact import paths an
operator may copy into a Go live-test provider bundle. plan.excluded is
{count, digest, groups}: groups state each reason, universe and invalidation
once with its count, and digest is the SHA-256 of the full exclusion list.
An advisory check with arguments="provider.go.packages" runs its command with
each of those packages appended, single-quoted.
degradations, present only when it applies, names self-binary-stale (only
with --accept-stale-binary; otherwise that verdict refuses the plan,
AFP-V0-9730): inside its own repository this binary was built from a commit
whose Go source differs from HEAD, so the plan may not be the one current
source gives, or self-binary-unverified when that comparison could not
complete; stderr repeats
it with the rebuild command (AFP-V0-846), except when stdout and stderr are
one pipe or file (2>&1), so the document still parses (AFP-V0-871).
degradationDetails maps each such code to that stderr notice: the rebuild
command, for a stale binary "go run ./cmd/corvint" to use this checkout's own
build without installing it, and that the notice changes no plan, selection or
exit status; --format text adds a "degraded-detail CODE NOTICE" line after
"degraded CODE" (ACX-V0-750). Each invocation prints at most one such stderr
line and keeps no state between runs (ACX-V0-751).
tiers (when anything is selected) ranks every selected unit without changing
the selection: 0 changed packages, 1 direct importers and same-parent
siblings below a top-level directory, 2 the rest; summary is "tier0=N tier1=N tier2=N". A lane
runs tier 0 and 1 locally (at most 15 packages, ./cmd/corvint only when changed
directly) and records tier 2 and anything skipped as NOT_RUN for CI.
summary restates the plan in one object: scope, dirty, selected, excluded and
unknown counts, unknownByReason {REASON: count}, and line, the same as one
text line ("scope=UNKNOWN dirty=5 selected=1 excluded=0 unknown=3
UNOWNED_DIRTY_PATH=3"), so a reader needs neither the plan nor a JSON tool
to learn what was decided; it never changes the selection. dirty counts
plan.dirty, which with --base includes the committed range; worktree and
committed split it, so a clean tree reads worktree=0 (AFP-V0-820).
--format text (AFP-V0-928) writes the same affected-plan/1 plan as lines
instead of JSON: "affected <summary.line> worktree=N revision=SHA [base=SHA]", a
"warning TEXT" line when the plan read unpinned worktree content, one
"selected UNIT tests=N WITNESS <- DIRTY_PATH [via A > B]" line per selection,
one "unknown REASON DETAIL [hint TEXT]" line per plan unknown, the exclusion
count and digest, and one "check KIND COMMAND" line per advice check with the
advisory packages appended. UNOWNED_DIRTY_PATH lines also name the runtime/test,
documentation and output declaration files, including for committed content-only
edits in repositories without the ownership ratchet (AZU-V0-9901).
It cannot be combined with --full or
--playwright-config; --format json (the default) is the JSON document.
Example: corvint affected --base FULL_SHA --format text
--summary (AFP-V0-872) writes a few-line affected-summary/1 verdict instead of
the plan: summary, tiers {counts, summary, tier0, tier1} with the sorted
tier-0 and tier-1 unit names a lane runs locally, range {base, paths count},
degradations and warning; --format text writes it as lines ("affected
<summary.line> ...", "tiers ...", one "tier0 UNIT" or "tier1 UNIT" per unit).
It never changes the selection.
Example: corvint affected --base FULL_SHA --summary
--full writes the affected-plan/0 document instead: every exclusion and every
selected unit's test files. It runs no test, writes no repository state, and never claims
that omitted tests are safe to skip; the provider keeps its own
NO_AFFECTED_SELECTION_PROOF unknown.

--snapshot reads a closed corvint-planning-snapshot/0 JSON receipt with schema,
commitRevision (current HEAD), treeRevision (all source/config bytes),
baseRevision, changedPaths (sorted complete base..commit diff), and
changedPathsSha256 (SHA-256 of canonical JSON changedPaths). It plans from
verified Git blobs in temporary scratch, then removes that scratch. Dirty
worktree bytes are not inputs. The output snapshot scope stays PLAN_ONLY and
accepting=false. Stale, incomplete or mismatched receipts fail closed.
Overlays, symlink/gitlink trees, --base, providers and discovery are unsupported
in this bounded snapshot profile; existing profiles are unchanged.

--base joins the committed tree diff FULL_COMMIT_ID..HEAD (renames listed as
both paths) to the worktree dirty set, so a branch is selected as a whole; the
receipt's range member records the base and exactly the committed paths that
joined. The base is a full commit id, a 7-digit or longer prefix, or a
revision such as HEAD~1 or origin/main; it is resolved once to the full
commit id that range.base records.
plan.dirty is that union ("dirty" means changed since the base), so a clean
worktree still lists its committed range paths there (range.paths names them);
UNOWNED_DIRTY_PATH marks the union's unowned paths from either component.
When a committed range path is not in the worktree dirty set and the worktree
is dirty, the receipt carries a top-level warning naming HEAD and the dirty
count: the plan mixes that commit with mutable worktree content.

--line-index FILE (experimental, GLS-V0-010; requires --base, JSON plan only)
adds lineSelection (affected-line-selection/0): each Go unit plan.selected
names, narrowed by the per-test line coverage index FILE (linecov collect,
linecov-index/4) to whole, tests with an anchored -run pattern, compile
(-run ^$) or exclude. A unit a declaration, new-file, non-Go, test-helper or
other fallback path reaches, through the index's drift to --base too, stays
whole with that category; possible source/data readers stay whole too. Both
sides must preserve declarations and function boundaries. A test without a
complete record is always kept. Unqualified selectors report SHADOW
(unqualified), with proposed decisions only; applying exclusions requires a
matching qualification record and a clean VCS-stamped selector executable.
The selection is FALLBACK, deciding nothing, for an unreadable index, another
module, runner (go env GOFLAGS, CGO_ENABLED, GOEXPERIMENT, -p aside, and the
test runtime: -test.v=test2json and the --line-test-timeout DURATION deadline,
default 10m, the go test -timeout the selected tests run under, and the
--line-exec PATH go test -exec wrapper they run under, none by default) or
ancestry, a dirty worktree, an unknown static scope or an index that binds no
environment; plan is unchanged either way. A test, or a package's in-order
record, observed under an environment the current one does not hold is kept
(environment). It reports counts and loadMillis, selectMillis, staticMillis and
gitMillis, and an exclusion entry (record digest, changed ranges, "any
recorded blob OID changes"). Refused under --full, --summary, --format text,
--by-path, --path, --snapshot, --provider, --playwright-config and
--appmap-impact (unsupported-affected-line-index-profile).
Example: corvint affected --base FULL_SHA --line-index linecov.idx

--require-clean (experimental; not with --snapshot) refuses with exit 2 and
unsupported-affected-mixed-worktree when the worktree status has any entry,
before the range diff, graph walk or provider; a clean worktree is unchanged.

--accept-stale-binary (AFP-V0-9730) keeps the plan of a binary whose Go source
differs from this checkout's HEAD (self-binary-stale, AFP-V0-846) as a
degraded plan: exit 0, the degradations member and the stderr notice. Without
it such a binary exits 2 with error code self-binary-stale and a fix: line,
printing no plan, because its selection may be narrower than this checkout's
own build gives; rebuild it (go install ./cmd/corvint) or run
go run ./cmd/corvint affected ... instead. Any form of affected is refused
alike (--full, --summary, --by-path, --snapshot, --format text, and the
jobs and audit subcommands, whose documents carry the same degradations
member when accepted, AFP-V0-11033). A
self-binary-unverified binary is not refused: it keeps the degradation.
Example: corvint affected --base FULL_SHA --accept-stale-binary.

--fail-fast-order (experimental, AFP-V0-2487; JSON plan only, not with --full,
--summary, --by-path, --format text or --playwright-config) adds an order
member: the selected units that a local corvint test receipt in the Git
directory (corvint/test-run.json, corvint/test-run.N.json) records as FAIL,
BUILD_FAILED or TIMED_OUT come first, each group in plan order. The key, each
receipt read with its sha256, and OBSERVED or NOT_OBSERVED are disclosed. The
selection and every other member are unchanged; without the flag no receipt
is read. Example: corvint affected --base FULL_COMMIT_ID --fail-fast-order

--by-path (experimental, AFP-V0-990; requires --path or --paths-from, not
with --full) builds the graph once and writes one affected-by-path/1 document
instead of the plan: sharedUnboundedReaders lists once the units the union plan
selects as UNBOUNDED_READER (they are selected on any path), and paths gives
each named path's scope, how many of that shared set it selects, its unknowns
and only the units it adds. --format text writes "affected by-path scope=S
paths=N shared=K revision=SHA", one "shared UNIT tests=N WITNESS" line per
shared unit, then per path "path P scope=S shared=J/K additions=M" with its
"  added UNIT tests=N WITNESS" and "  unknown REASON DETAIL" lines. A stale
or unverified binary (AFP-V0-846) is named on stderr, in degradations and
degradationDetails, and on "degraded CODE" and "degraded-detail CODE NOTICE"
text lines (ACX-V0-750).
Example: corvint affected --by-path --path docs/GLOSSARY.md docs/README.md --format text

--path PATH [PATH ...] (experimental; repeatable, up to 4096 paths in all) plans
exactly the named repository-relative paths at HEAD instead of the worktree status or a range,
for example a change list captured elsewhere. It walks reverse dependencies
transitively, as the default form does, so a test two importers away is
selected, and keeps the same scope and unknown vocabulary: a path no language
claims leaves plan.scope UNKNOWN. A path absent at HEAD is planned as a
deleted path. The receipt's path_list member echoes the sorted paths. A dirty
worktree, or an ignored file HEAD does not track that the list names or the
graph reads, refuses with unsupported-affected-mixed-worktree, since the graph
would not be HEAD's; one only a TypeScript unit lists, such as a gitignored
playwright-report/ bundle, is instead excluded from the graph. A worktree whose
only dirty paths are declared documentation (.corvint/documentation-paths.json,
itself clean) that no unit owns plans as a clean one would (AFP-V0-832). One --path
takes every following value up to the next option, so --path a b c and
--path a --path b --path c plan the same bytes. Write --path=VALUE for a name
that begins with -. It does not combine with --base (invalid-arguments).
Under --snapshot, --provider, the Playwright profile (--playwright-config,
--playwright-discovery) or --appmap-impact it refuses with exit 2 and
unsupported-affected-path-profile, naming why that profile plans its own
change set and the recovery: commit the paths and plan with --base <HEAD>.

--worktree (experimental, ACX-V0-851; requires --path or --paths-from, not
with --require-clean, --snapshot or --base) plans the named paths against the
working tree, dirty and ignored files included, instead of refusing a dirty
worktree. The receipt carries the degradation worktree-path-plan and a warning
naming HEAD and the dirty count: the plan is worktree-derived, not pinned
evidence. Without --worktree the refusal stays the default. With --base it
refuses and names the alternatives: --base alone plans BASE..HEAD with every
dirty worktree path, and a named subset against the working tree is a
--worktree path list that adds the range's own paths (ACX-V0-752).
Example: corvint affected --worktree --path internal/foo/foo.go --format text

--paths-from FILE (experimental) adds the --path values listed in FILE, one
per LF-separated line, blank lines skipped; FILE is relative to the root unless
absolute, and is held to the same canonical-path rule and 4096-path bound as
--path. It joins any --path values and takes the same refusals. One call with
many paths builds the graph once and plans their union, so a caller with
several changed files should batch them rather than call once per path.

The graph cache (experimental; never with --snapshot or --playwright-config)
reuses the verified graph between calls on an unchanged checkout. By default it
is corvint/affected under the user cache directory (os.UserCacheDir:
~/Library/Caches on darwin, $XDG_CACHE_HOME or ~/.cache on linux); --cache DIR
names another directory and --no-cache turns the cache off. A default location
that is missing, unwritable or placed where --cache DIR would be refused is not
used and the call runs uncached, with no error. DIR is a directory outside the
repository, outside any .git directory and outside
the repository's Git directory wherever it lives; it must be private to the
user, is created if absent and holds at most one entry per repository root. An entry is
trusted only when it was written by this exact corvint executable for this
root, and every directory the graph walked and every file it read or statted
has the same size, mode, inode, modification and change time;
otherwise, or when the entry is damaged, the graph is built in full and the
entry replaced. Files changed within two seconds of a build are not pinned, and
platforms other than darwin and linux never use an entry. The receipt is
byte-identical with and without the cache, cold, hit or miss; the worktree
status, HEAD and drift checks run on every call, and nothing is written inside
the repository.

affected cache (experimental) reports the default cache's location, state
(PRESENT, ABSENT, UNAVAILABLE or INCOMPLETE), entry count and bytes as one JSON
object, or with --format doctor-plugin as one AFFECTED_CACHE item for a
corvint-tasks doctor plugin; it creates and writes nothing.

affected --ownership-audit (experimental) lists, as one JSON object of profile
affected-ownership-audit/0, each path tracked at HEAD that no plugin, the driver
claimant or a project declaration claims: the paths a plan reports as
UNOWNED_DIRTY_PATH. It refuses a dirty worktree, builds the graph without the
cache, writes nothing, and exits 0 when no path is unowned, 1 when some path
is, and 2 on an error.

affected jobs (experimental, AFP-V0-2340 to AFP-V0-2346) evaluates the
project-owned input declaration of non-test CI jobs (--declaration, default
.corvint/ci-job-inputs.json, corvint-ci-job-inputs/0) over the range
--base..HEAD and the dirty worktree, and writes one corvint-ci-job-plan/0:
per job RUN or SKIP with its reason and witnesses. A job runs when its
workflow file, the declaration, a declared path or directory/, go.mod, go.sum
or the directory of a package its goPackages build on any declared platform
changed, when the affected plan selects one of its goTests or is UNKNOWN, or,
for an exportedTree job, when a changed entry is not a regular blob within the
export bounds. SKIP means no declared input changed (NO_DECLARED_INPUT_CHANGED);
it is not a pass of the job. Exit 0 with a plan; 3, with every job absent, when
the declaration cannot be read (DECLARATION_INVALID), so a caller runs every
job. Example: corvint affected jobs --base "$(git merge-base HEAD origin/main)"

affected audit (experimental) reads a corvint-affected-audit-input/0 FILE
listing changes: id, full base commit id, and kind replay (full head), revert
(full commit id reverted on base) or mutant (diff file applied on base);
junit or playwright-json baseline and changed outcome files relative to FILE,
or none (FILE, outcome, diff and map files must be regular files; a symlink is
refused); optional path_prefix and appmap {map, ledger}. In a private shared
clone it runs affected on every change and appmap-impact where configured,
treats each test whose outcome changed or is new as one that had to run, and writes a corvint-affected-audit/1 report: per
change and selector the verdict (SAFE, MISS, UNKNOWN, NOT_RUN), misses,
recall, precision, fallback and elapsed time; per selector micro recall and
precision and a 95% one-sided Clopper-Pearson bound on the per-change miss
rate. Missing outcomes, a dropped test or a selector error is UNKNOWN, never
safe. It exits 1 on any miss, 3 on any UNKNOWN, 2 on an input error.

advice reads the repository's own declarations - a Makefile gate target and the
AGENTS.md Verify block - and lists them as mandatory checks, followed by one
advisory go test command over the selected packages and the unknown frontier
that qualifies it. A declared command that a sentence of the AGENTS.md Verify
section names in backticks with "unless", "only when" or "only if" is a
conditional check instead, and its reason quotes that sentence. The advice is static: no check is executed, nothing is
derived from an exclusion, and every mandatory check remains required whatever
the advisory list says.

--provider (up to 4 external-evidence records, decoded and verified exactly as
impact does) adds advice.test_selection, an external-test-selection/0 member:
state is narrow-selection-allowed only when every changed path and every
entity it reaches is qualified by a fresh, bound, verified verifies or asserts
relation (covers too under --selection-profile coverage); otherwise it is
full-relevant-suite-required, blocked, or unknown, with every reason listed.
Uncommitted paths never qualify. An external-evidence-provider/2 record may
also relate a test path directly to a changed path (EEP-V2); both sides must
then be bound, fresh, and verified. The member never removes a check, never runs a
test, and is absent when no --provider is given. --repository binds a declared
repository id to a local checkout, as for impact.

--playwright-config selects the separate playwright-affected/0 profile. It
statically expands reached Playwright test files into project-distinct units,
binds project/config/browser/device inputs, and widens to the full relevant
suite on unsupported dynamic config or source reachability. --playwright-discovery
reads a bounded canonical playwright-discovery/0 receipt binding HEAD, config and
source bytes to the complete unfiltered project/file listing. Missing or mismatched
discovery emits no file commands and one complete-config fallbackArgv; a MALFORMED
receipt names its reason and a one-line detail. It executes no config or test and
cannot be combined with --provider. Files Git ignores that HEAD does not track,
such as a playwright-report/ directory, are no graph input and raise no frontier;
a tracked but ignored file and an untracked file Git does not ignore still are.
A dirty path that .corvint/documentation-paths.json declares (AFP-V0-160) is
recorded in plan.declaration (path, sha256, matched paths, state), and each
exclusion names the declaration it relied on. When the config or a source in the
Playwright closure (its tests, hooks and what they import) names a matched path
as a string (an import, a path literal, an ancestor directory), or a naming
file's closure membership is unknown, the declaration is REFUSED: one
playwright:declared-path-contradicted unknown names the declaration, its digest
and the evidence site, and the plan falls back to the full suite (TJAA-V0-101,
AFP-V0-442). A reader outside the closure, such as a standalone scripts/*.mjs
tool, is listed in plan.declaration.outOfClosure instead. A dirty declaration
file that no unit reads is classified CORVINT_DECLARATION (AFP-V0-443).
A dirty path that .corvint/runtime-readers.json declares (AFP-V0-602) is listed
in plan.nonSource as DECLARED_RUNTIME_READERS with the entry's own reason, not
as UNOWNED_DIRTY_PATH; the units that embed, enclose, name or read it are still
selected.
.corvint/playwright-unreached-paths.json (TJAA-V0-106, AFP-V0-874) lists the
source paths the Playwright suite cannot reach, even through the application
under test. Without it, a dirty TypeScript or JavaScript path that reaches no
Playwright test by a static edge raises playwright:out-of-suite-source and the
plan runs the full suite. Schema: a closed JSON object of at most 64 KiB,
{"profile":"corvint-playwright-unreached-paths/0","paths":[...]}, at most 256
strictly ascending repository-relative files or /-ended directories, none under
.corvint/. Example:
  {"profile":"corvint-playwright-unreached-paths/0","paths":["scripts/","tools/seed.ts"]}
Effect: a matched dirty path is excluded, and recorded in plan.declaration, only
when the Playwright closure provably leaves it out; a matched path the closure
may load (its unit is in the closure, it is the config, or membership is
incomplete) refuses the record and raises a second playwright:out-of-suite-source
unknown naming the path. An invalid file is an error, never an empty list.
--playwright-env (repeatable, requires --playwright-config, not with --snapshot)
records one environment variable in plan.environment with its value and SHA-256:
NAME=VALUE is a value the caller supplies, and NAME reads only that variable from
the invoking environment, SET or UNSET. A use.launchOptions.executablePath of the
form process.env.NAME, optionally with ?? or || and a string literal without
escapes, then resolves: UNSET is undefined, so the project records the bundled
browser, and a value records that binary path. The
playwright:browser-executable-unresolved unknown stays when the variable is not
named, the value is empty, or the config loads a module other than
@playwright/test or the path, os, url and fs built-ins, names a module loader or
evaluator such as require, createRequire, module, loadEnvFile, eval or Function,
or declares globalSetup, any of which can set the variable first. No other
variable is read.

affected discovery converts the JSON report of an unfiltered
` + "`playwright test --list --reporter=json`" + ` run (--playwright-list FILE) into that
canonical receipt on stdout, bound to the current HEAD, config bytes and source
digest, with each test path made repository-relative. It refuses a listing that
reports errors, was filtered or sharded, or names another config, and fails on
HEAD or source drift. It runs nothing; the listing stays caller-declared evidence.
--playwright-ungrepped-list FILE adds the same listing taken with every project
grep and grepInvert removed from the config. It is refused like the first and
when it names a pair the config does not select; a grep omission is verified
only when both listings agree outside grep and the configured patterns,
applied to that listing's titles and tags, keep exactly the listed cases.
Otherwise the omission stays unverified.
--playwright-ungrepped-config PATH lets the ungrepped listing name an untracked
wrapper config instead of the bound one, so the tracked config need not be
edited. The wrapper must sit beside the bound config and be exactly this file,
up to comments, whitespace, quote style, semicolons and trailing commas, with
the import naming the bound config's file name with or without its extension:

  import config from './playwright.config';
  export default {
    ...config,
    grep: undefined,
    grepInvert: undefined,
    projects: (config.projects ?? []).map(({ grep, grepInvert, ...project }) => project),
  };

Any other wrapper, one HEAD tracks, or one given without
--playwright-ungrepped-list is refused. The wrapper is kept out of the source
digest; delete it after discovery.

A repository that commits .corvint/flows.json (profile corvint-flows/0: the
flows directory, optional kinds rules assigning unit or integration to path
prefixes, and an optional playwright member naming the tracked config and its
affected discovery receipt) gets one plan for every test kind without any
further option (experimental, APK-V0-001): each selection carries testKind
(unit, integration, e2e or UNKNOWN, never guessed), and plan.e2e lists the E2E
test files the change requires, each naming the flow, variation and changed
path that selected it. The inventory is the bound discovery receipt
(AFU-V1-140), else the flows' test links; a test no flow links is selected as
unlinked; a stale link, global path, unbounded plan, missing receipt or
exceeded bound selects every E2E test with a typed e2e unknown and never
drops one. Example: corvint affected --base FULL_COMMIT_ID --format text.

--selection-profile e2e-safe reads one repository-relative
application-flow-selection-provider/1 FILE (flows directory, runner config,
discovery record, global paths, E2E inventory, per-flow coverage tiers and a
coverage file) and writes an e2e-safe-selection/1 advice.test_selection. It
selects every test whose file changed or that is linked to, reaches, or covers
the change, and omits a test only with a reviewed-links or coverage exclusion
proof, named by digest and stated once in proofs. A global path change, missing
or mismatched discovery, an inventory gap, any unproven exclusion, or more than
16,384 tests or 65,536 paths in a list yields the full relevant suite with
closed e2e-* codes. The strict and coverage profiles are unchanged.
Under e2e-safe, --repository binds an application repository the provider
declares (id and parentless origin commit) to a checkout top level, and
--repository-base pins the commit its changes are read from; both need --base.
Its changed paths join the selection as ID:PATH, coverage records and reviewed
source links may name ID:PATH, and the advice binds each repository's base,
revision and tree. A declared repository left unbound, or a binding the
provider does not declare, yields the full relevant suite with
e2e-repository-unbound.

--appmap-impact (up to 16, experimental ANL-V0-031, ANL-V0-034) reads complete
application-map-impact/0 reports built for --base at HEAD and adds the learned
advice.app_navigation member: per app (several maps aggregate) the reported and
effective verdict, the reasons and the spec files. Uncommitted paths, unknown paths or a spec list over
64 make the effective verdict full-suite-required, which also adds an
APP_NAVIGATION_FULL_SUITE_REQUIRED unknown. It never removes a check or changes
the plan; a trimmed, tampered or mismatched report refuses. In a two-repository
layout (ANL-V0-060..062) a report built in the application repository is
accepted when a declared --repository ID=DIR (relative to --root unless
absolute) is a checkout at the report's head whose --repository-base ID is the
report's base; each row's bindings record the checkout every report was checked
against. An uncommitted path in that checkout, or a path the report counted but
did not place, makes the verdict full-suite-required: an unmapped application
path never reads as "no tests affected". Any other report refuses and names this
layout.

--provider-command ARGV_JSON runs one local provider command and reads its
stdout as a record, exactly as impact does (EEP-TR): a JSON array of strings
whose first element is an absolute executable path, no shell, scrubbed
environment, 10s wall time, 1 MiB stdout, every failure one closed provider
row. It counts toward the same 4-provider bound as --provider.
`

const impactHelp = `Compile experimental impact evidence for index-admitted paths.

Usage:
  corvint [--root PATH] impact [--limit N] [--provider FILE]...
    [--provider-command ARGV_JSON]... [--provider-mcp ARGV_JSON]... [--repository ID=DIR]...
    [--language-profile non-go-syntax-v0] [--require-clean] [--producer] PATH...
  corvint [--root PATH] impact --capabilities
  corvint [--root PATH] impact --base FULL_COMMIT_ID [--limit N]
    [--range-profile expanded-256]
  corvint impact --app-root DIR --test-root ALIAS=ABSOLUTE_ROOT --manifest FILE
    --base COMMIT [--manifest-repo ALIAS] [--at REV] [--test-at REV]
    [--ledger FILE]

Arguments:
  PATH  One or more normalized repository-relative paths whose suffix the
        immutable index admits. By default every path must be tracked at the
        captured revision. For a .go path, the root package included in the
        profile has the module path as its import path. Paths
        admitted but lacking a reverse-import rule emit an explicit coverage
        uncertainty; no language rule is inferred.
        An unadmitted suffix returns unsupported-impact-path-suffix.

Single-root impact reads one --root at its HEAD; there --at and --revision
refuse unsupported-impact-layout (ANL-V0-063). --app-root and --test-root
select two-root impact (ANL-V0-064..067): the diff --base..--at (default HEAD)
is taken in --app-root, tests are read from the --test-root alias at --test-at
or its HEAD, and impact-two-root/0 records both pins. Unknown or ignored app
paths, an uncommitted --app-root or a change selecting no spec file return
full-suite-required with explicit unknown lines, never an empty selection; a
root that moves while impact runs refuses impact-root-drift. Learned, advisory.

Language capabilities (syntax evidence, never complete runtime impact):
  Go          .go                         path/range; package imports (path).
  Ruby        .rb                         path/range; markers; imports unknown;
                                          advisory autoload constant references, two hops.
  JavaScript  .js .jsx .mjs .cjs           path/range; relative/profile/tsconfig imports (path).
  TypeScript  .ts .tsx                    path/range; relative/profile/tsconfig imports (path).
  Swift       .swift                      path; SwiftPM target-module imports and
                                          same-target references (one hop).
  Kotlin      .kt                         path; package imports of declared names and
                                          same-package references; .kts NOT_SUPPORTED.
  C#          .cs                         path; using/static/alias directives and
                                          enclosing-namespace references; .csproj NOT_SUPPORTED.
  Swift, Kotlin and C# rows carry their precision limits as coverage uncertainty;
  range impact and the historical packet budget do not use them.
  The non-go-syntax-v0 profile always reports dynamic-dispatch unknowns;
  unsupported constructs remain unknown even without a lexical dynamic token.

Path receipts over MIXED roots warn with the bound commit, dirty count and up
to 8 dirty paths when requested paths are not all dirty. Mutable content remains excluded. Select
--language-profile non-go-syntax-v0 for versioned structured non-Go unknowns;
a request naming a Ruby path selects it by default.
Non-Go range receipts and MCP paths name that experimental profile automatically.
The path selector cannot be combined with --base.
Other indexed suffixes keep positional evidence and missing-rule uncertainty.
MCP admits the Go/Ruby/JS/TS suffixes above for path requests only.
A syntax-profile path receipt (explicit, or a Ruby path) is the MCP
corvint.impact receipt for the same paths, revision and limit, budgeted only by
--budget-bytes. The historical default profile alone keeps a 5952-byte packet
budget for a path no reverse-import rule names; MCP never applies it.
Reverse-import results stop at direct importers (one hop; Ruby's advisory
autoload rows reach two): a test reached through an intermediate importer is
absent, and a READY receipt is evidence, not a closed test selection.
corvint affected --path PATH [PATH ...] plans a transitive selection at HEAD.

Options:
  --limit N                  Maximum results (default: 10; range: 1-50).
  --require-clean            Experimental: on MIXED path impact, emit an empty
                             NEEDS_WIDENING receipt (exit 0), with reason mixed-worktree.
                             Does not run external providers when abstaining.
                             Unavailable with --base.
  --base FULL_COMMIT_ID      Select the separate committed range profile from
                             an immutable base commit to the captured HEAD.
  --range-profile expanded-256
                             Explicit experimental range capacity of 256 paths;
                             requires --base. All other range bounds and omissions
                             remain unchanged. The default range capacity is 100.
  --provider FILE            Experimental (EEP-V0): attach one external evidence
                             provider record (schema external-evidence-provider/0)
                             to the receipt's separate "external" section. Repeat
                             up to 4 times; a relative FILE resolves against --root;
                             default path profile only. Provider items carry
                             authority "external-provider" and never enter results.
                             Schema external-evidence-provider/1 (EEP-V1) declares
                             repositories by root-commit origin, evaluates freshness
                             per repository, and qualifies every path endpoint.
                             Schema external-evidence-provider/2 (EEP-V2) also
                             composes path-to-path relations into
                             external.path_relations; under /1 they stay unsupported.
  --provider-command ARGV_JSON
                             Experimental (EEP-TR): run one local provider command
                             and read its stdout as a provider record, decoded
                             exactly as a --provider FILE. ARGV_JSON is a JSON array
                             of strings; the first is an absolute executable path.
                             No shell, no PATH lookup; stdin empty; environment
                             PATH, TMPDIR, LANG=C, LC_ALL=C only; working directory
                             --root. Bounds: 10s wall time (process group killed),
                             1 MiB stdout, 64 KiB stderr (never echoed), one record.
                             Counts toward the 4-provider limit.
  --provider-mcp ARGV_JSON   Bounded local MCP 2025-11-25 stdio provider (EEP-MCP).
                             Calls corvint_evidence with empty arguments once;
                             exactly one text content block supplies record bytes.
                             Same argv, environment, 10s and process-group policy
                             as --provider-command; no server requests/notifications.
                             Counts toward the shared 4-provider limit.
  --repository ID=DIR        Experimental (EEP-V1): bind the record repository ID to
                             the local Git checkout at DIR (its top level). Binding
                             holds only when the declared origin is a root commit of
                             DIR's HEAD. Repeat up to 8 times; requires --provider.
  --capabilities             Print the corvint-impact-capabilities/0 listing and
                             exit: suffixes with a reverse-import rule, suffixes
                             admitted without one, those of them with a
                             Swift/Kotlin/C# module reference rule, the
                             per-call limits, derived from the admission
                             tables, and the reverse-import closure depth.
                             Takes no other argument.
  --producer                 Add one "producer" member (version, build, commit,
                             commitState) to the receipt; an unknown member reads
                             "unknown" and an unstamped build reads "unstamped".
                             Every other byte equals the default receipt.
  --                         Treat all remaining arguments as paths.

Requirements:
  Darwin or Linux; a Git repository, with a slash-qualified Go module when a
  .go path is given; no absolute
  or parent-traversal paths; at most 100 paths by default (256 only for the
  explicit expanded range profile) and 1024 characters per path. The
  immutable index admits at most 1,000,000 bytes per source and 128 MiB in total.
  --budget-bytes, unadmitted-suffix, unsupported-platform, and oversized inputs
  are rejected rather than approximated.

The committed range profile requires a clean, stable worktree; derives changed
Go/Ruby/JS/TS paths and exact old/new admitted-source hunk spans itself; binds base/head commit
and tree, status, full-delta and hunk digests, and omissions; admits marker and
ADR relations only from target lines added or replaced by the diff; and rejects
delete, type-change, binary, excluded-source, oversized, or drifting
ranges.

Every profile uses only revision-tracked evidence. All profiles are read-only,
local-only, and emit canonical JSON.
Mixed-worktree freshness is disclosed where applicable.
`

const cemHelp = `Change Evidence Map producer and verifier.

Usage:
  corvint [--root PATH] cem begin --patch PATCH --output MAP [--base REV]
    [--spec cem/1.0]
  corvint [--root PATH] cem prepare --base REV --target REV [--patch CACHE] [--replace]
    [--spec cem/1.0] [--non-regular unknown] [--path-policy base]
  corvint [--root PATH] cem cite --map MAP --hunk SELECTOR --evidence-path PATH
    (--bytes START:END | --lines START:END) --relation RELATION [--output MAP]
    [--replace --expected-hunk-digest DIGEST]
  corvint [--root PATH] cem cite --map MAP --hunk SELECTOR --evidence-path PATH
    (--bytes START:END | --lines START:END) --relation RELATION
    --paired-output DOC --paired-range NAME=BASE..HEAD --paired-root ABS
  corvint [--root PATH] cem mark --map MAP --hunk SELECTOR
    --disposition unknown|mechanical --reason REASON [--output MAP]
    [--expected-hunk-digest DIGEST]
  corvint [--root PATH] cem status --map MAP --expected-base REV --target REV
    --artifacts DIR [OPTIONS]
  corvint [--root PATH] cem status --map MAP --paired DOC --paired-root ABS
  corvint [--root PATH] cem verify --map MAP --expected-base REV --target REV
    --artifacts DIR [OPTIONS]
  corvint [--root PATH] cem report --map MAP --expected-base REV --target REV
    --artifacts DIR [--output REPORT | --preview] [OPTIONS]
  corvint [--root PATH] cem report --map MAP --format json --expected-base REV
    --target REV --artifacts DIR [OPTIONS]
  corvint [--root PATH] cem cover --map MAP --coverprofile PATH --test-run ID [--output MAP] [--states-output PATH]
  corvint [--root PATH] cem verify-stable --repository PATH --map MAP
    --expected-base REV --target REV --artifacts DIR [--trusted-keys FILE]
    [--non-regular unknown]

Actions:
  begin    Build a cem/1.0 producer map from exact out-of-band patch bytes.
  prepare  Derive the canonical base-to-target patch and write or resume the
           cem/1.0 candidate at .corvint/change.cem.json. Commit the candidate,
           then run the returned verification action; it passes --artifacts
           with the absolute repository root.
  cite     Compile a byte or one-based line span of base evidence into the
           normative span and attach it to one hunk with a typed relation.
           --relation accepts: call-site, decision, dependency, implementation,
           incident, specification, test-claim.
           Normative definitions: docs/CHANGE-EVIDENCE-MAP.md#supported.
           With --paired-output, --paired-range and --paired-root (all three,
           additive profile cem-paired-evidence/1, CAX-V0-032) the evidence
           is read at HEAD of the second repository at --paired-root, both
           range ends are resolved to full commit IDs (BASE must be an
           ancestor of HEAD), and the citation is added to the separate
           document DOC bound to the map's change; the map is never written,
           so --output is refused.
           --replace --expected-hunk-digest DIGEST (given together) first drops
           the hunk's earlier citations, only while DIGEST is the hunk's current
           cem-hunk-state/1 digest; a stale digest refuses hunk-digest-mismatch
           and nothing is written, and every other hunk is kept (CEM-V1-694).
  mark     Record an explicit unknown or byte-verifiable mechanical disposition.
           --expected-hunk-digest DIGEST refuses the mark unless DIGEST is the
           hunk's current digest (CEM-V1-695).
  status   Local completion check; verify is the equivalent machine/CI surface;
           report renders the optional human view. All three verify identically.
           report writes $GIT_DIR/corvint/cem-review.md unless --output names
           a repository-relative path or an absolute one outside the repository.
           report --preview returns the same Markdown in the envelope and writes
           nothing (mutates false); it takes neither --output nor --format json.
           status --paired DOC --paired-root ABS (CAX-V0-033) instead checks
           that cem-paired-evidence/1 document: it binds the map, the pinned
           range still resolves, and every citation's blob and span digest
           match at its head; any drift refuses evidence-drift.
  cover    Record a patch coverage witness on every hunk from one local
           coverprofile named by --test-run; the map stays cem/1.0. Any
           language's line coverage works once written in that line format
           ("mode: set" then "PATH:L.C,L.C STATEMENTS COUNT" per span; PATH is
           the hunk path or ends in "/" plus it). The witness is coverage only,
           not an execution or discrimination witness. A hunk whose path the
           profile never names is uncovered in the map and counted apart as
           notInstrumented (with notInstrumentedHunks) in the result.
           --states-output PATH also writes the additive
           cem-coverage-states/1 document (CAX-V0-010): per hunk, in map
           order, covered, uncovered or not-instrumented, bound to the
           written map's SHA-256; the map bytes are unchanged by it.
           Corvint ships no converter from other coverage formats.
  verify-stable
           Verify a cem/1.0 map against the absolute --repository and the
           artifact bytes under --artifacts; prints one
           cem-stable-verification/1 JSON result. --trusted-keys FILE is the
           consumer's absolute corvint-trusted-keys/1 file
           {"profile":"corvint-trusted-keys/1","keys":[{"keyid":K,
           "ed25519PublicKey":B64}]}: only a keyid it names can raise the
           authentication axis to VERIFIED (UNTRUSTED_KEY otherwise). A key
           the map or its artifacts carry is never trusted, and a valid
           signature never raises executionAtCommit.

Verification options:
  --expected-base REV  Independent expected base. Required.
  --target REV         Verification target. Required.
  --patch PATH         Refused: the verifier derives the canonical patch itself.
  --max-unknown N      Policy ceiling on unknown hunks.
  --max-mechanical N   Policy ceiling on mechanical hunks.
  --citation           status adds the map's citation class beside the result;
                       an accepted result always carries its citation block.
  --scaled-operations  status only, opt-in: raise the stable verifier's Git
                       operation bound from the cem/1.0 protocol's fixed 1,024
                       to 1,024 plus 4 per hunk and evidence record; every
                       other bound is unchanged.
  --non-regular unknown
                       prepare, status, verify and verify-stable, opt-in
                       additive profile cem-non-regular/1 (CAX-V0-020): a
                       changed path that is not a regular file (a symbolic
                       link or gitlink) is left
                       out of the mapped patch, never read or followed, and
                       reported in nonRegular beside the result as an explicit
                       unknown that status counts and --max-unknown include.
                       Without it such a path refuses unsupported-tree-mode.
                       The map pins the regular-only patch, so a reader must
                       opt in too; a plain cem/1.0 reader refuses it
                       patch-digest-mismatch. verify-stable then prints
                       {nonRegular, nonRegularProfile, stableVerification}.
  --path-policy base   prepare and status, opt-in additive profile
                       cem-path-policy/1 (CAX-V0-060): read the witness path
                       policy .corvint/witness-paths.json from the base tree
                       only and classify each unknown hunk on a path no
                       extractor reads. An inert rule's hunk is reported
                       declared-inert in pathPolicyHunks, citing the policy
                       file and line, and counted in counts.declaredInert, not
                       against --max-unknown; a data rule's hunk stays unknown.
                       This binary installs no data-reader finder, so a data
                       row reports data-reader-not-produced and names no
                       readers. A code path or a supported or
                       mechanical hunk is never classified, and the map bytes
                       are unchanged. Example: corvint cem status --map
                       .corvint/change.cem.json --expected-base B --target T
                       --artifacts DIR --max-unknown 0 --path-policy base
  --guarding-tests     verify only, opt-in: after an ACCEPT, search the base
                       tests of each uncited Go source hunk's directory (or,
                       when it has none, of up to 16 direct importers) for one
                       that passes at the target and fails with only that hunk
                       reverted in a disposable export; adds the guardingTests
                       report (cem-guarding-tests/0) with citation-plan rows. It
                       sets no tier and writes nothing. Bounds: 300 s per go
                       test run, 1200 s in total. It executes the target's own
                       tests UNCONFINED, with the caller's privileges; a TMPDIR
                       inside the repository is refused. Not with --hunk-revert;
                       status and report refuse it.
  --artifacts DIR      Required: the artifact root the stable verifier reads;
                       status also applies the ceilings to an accepted map.
  --hunk-revert        verify only, opt-in: after an ACCEPT, reverse-apply each
                       test-cited Go source hunk in a disposable export of the
                       target and re-run its named tests; adds the hunkRevert
                       report (cem-hunk-revert/0), and only a consumer-observed
                       failure counts as held; every hunk it did not attempt
                       is listed in notAttempted with its reason. Bounds: 300 s per go test run,
                       1200 s in total (reasons timeout, total-limit). It
                       executes the target's own tests UNCONFINED: no sandbox,
                       with the caller's privileges; use it only on a target
                       you trust. status and report refuse it.

Map paths:
  status and report read --map MAP repository-relative, or as an
  absolute path outside the repository and its Git directories (for example a
  map copied out of the repository); an absolute path inside them is refused,
  so name it repository-relative. The map binds the repository by object IDs;
  the read is bounded, a symlinked map is refused, and only report writes (its
  review file, never over the map). verify also accepts an absolute in-repository
  map. verify-stable accepts either form, resolving a relative --map against
  --repository (CEM-V1-7400). cite, mark and cover read an
  out-of-tree --map under the same rules only with a repository-relative
  --output, because Core writes a map only under the repository's lock and
  never in place outside it (CEM-V1-647); every other action takes MAP
  repository-relative only.
  No action takes a paired change: --range, --range-root and --paired-from
  are refused with the reason, because a cem/1.0 map binds one repository's
  base and target (CEM-V1-648).

Workspace: a CEM cites one repository. Every action but verify-stable
refuses --workspace FILE, or a discovered
--root/.corvint/workspace.json without --no-workspace, as
workspace-unsupported naming the ignored aliases.

cem/1.0 is the only wire. Verification derives the canonical
whole-repository patch itself excluding exactly .corvint/change.cem.json. A
retired cem/0.1, cem/0.2 or cem/0.3 map refuses with unsupported-spec and its
migration fix. All commands are local-only; reports can disclose repository
information and stay local by default.
`

// cpuProfileHelpNote documents the CORVINT_CPUPROFILE and CORVINT_MEMPROFILE env knobs
// (V1-0051, V1-1643, PRB-V0-001..009: runtimeenv.Value resolves the CORVINT_ prefix). They are
// off by default, operator-set only, and honoured by every command at main's single dispatch
// bracket (profile_knobs.go) and by every MCP server's entry point. They write local pprof
// files and never widen what a command reads, returns, or mutates (AGENTS.md invariant 4).
const cpuProfileHelpNote = `
CORVINT_CPUPROFILE=PATH (operator env var, off by default): writes a pprof CPU
profile for this invocation to PATH.
CORVINT_MEMPROFILE=PATH (operator env var, off by default): writes a pprof heap
profile for this invocation to PATH.
Every command and MCP server honours both, one file per process, written on exit
and on SIGINT or SIGTERM; {pid} in PATH becomes the process ID. PATH must lie
outside every Git worktree and Git directory (PRB-V0-013). Diagnostic only;
they do not change what is read, returned, or mutated (corvint help --json lists
the commands and servers).
`

// rootPreambleValue reports whether arguments[index] may be consumed as the value of a bare `--root`
// in the `[--root PATH]` preamble (GPK-V0-064, decision 0181): it must exist and must not be
// option-like under argparseOptionLike. A literal `--` keeps its prior treatment as a value.
func rootPreambleValue(arguments []string, index int) bool {
	return index < len(arguments) && (arguments[index] == "--" || !argparseOptionLike(arguments[index]))
}

const repositoryGuidanceHelp = `Experimental change review: affected plan and local branch overlap.

Usage:
  corvint [--root PATH] review --base FULL_COMMIT_ID [--max-refs N]

Read-only; needs a clean HEAD (any dirt refuses). Review composes advisory
affected advice and local refs/heads overlap hints; max-refs is 1..32.
review.affected is the compact affected-plan/1 document; review.affectedFull
names the argv that prints the full affected-plan/0 plan and that plan's digest.
Missing sources and bounded branch paths remain unknown. No scripts or tests
execute. No index, trace or observation ledger is written. CEM, OCM, frontier
and mandatory test obligations remain open. The inferred features, overview
and feature commands are removed (decision 0597).
review reads one repository: --workspace FILE, or a discovered
--root/.corvint/workspace.json without --no-workspace, refuses as
workspace-unsupported naming the ignored aliases.
`

const testPlanHelp = `Propose a candidate consolidation of declared test variations.

Usage:
  corvint [--root PATH] test-plan consolidate --input FILE [--tests FILE]...
      [--map FILE]... [--revision REV] [--max-steps N] [--format table|json]
  corvint [--root PATH] test-plan check --plan FILE --input FILE [--tests FILE]...
      [--map FILE]... [--revision REV] [--max-steps N]

FILE --input is one test-consolidation-input/0 document (at most 8 MiB). The
planner abstains every variation with a missing anchor, reuses a variation only
when a --tests provider document (at most 8) shows a passing, associated,
eligible e2e test whose bound digests match --revision (default HEAD), classes
duplicates, isolates destructive and conflicting variations, and groups the rest
by context into ordered tests of at most N steps (2..32, default 8). --map (at
most 8 AMAP-V0 maps) validates screens, routes and lineage freshness at the
revision; without it anchor validation is NOT_RUN. Plan authority is always
candidate: it states no test adequacy.

check recomputes the plan and compares it with the one table header and its rows
inside --plan (a Markdown file may surround it). It exits 0 when they match and
the plan is COMPLETE, 1 on test-plan-mismatch, test-plan-incomplete or
test-plan-header-missing, and 2 on invalid arguments or input, with no stdout.
The command reads only its named files and Git, runs no test and never writes.
Experimental.

Prefix lint (ANL-V0-009..011, ANL-V0-035..036):
  corvint [--root PATH] test-plan prefixes --map FILE [--ledger FILE] [--facts FILE]
      [--check [--baseline FILE]] [--revision REV] [--budget N | --full]
Flags specs whose test-sequence prefix (two or more screen transitions) only crosses edges the
--ledger (application-edge-ledger/0) proves and ends on a fresh deep-linkable screen, with the
replacement goto and saved steps. --facts (test-prefix-facts/0) declares read-only, mutating and
assertion-target edges per spec; undeclared effects abstain. A step the map classifies from
page-object source is read-only only when navigation-only and fresh; state-effect and unknown steps
abstain and the sidecar cannot upgrade them. --check exits 1 when a flagged test is not in
--baseline. Learned, read-only, offline.

Page-state grouping (ANL-V0-012..014, ANL-V0-033):
  corvint [--root PATH] test-plan page-state --input FILE [--check --baseline FILE]
Groups non-destructive, state-preserving variations of one input that share app, screen, setup,
user, org and required facts, so one visit can serve several assertions. Destructive, state-changing
or anchor-missing variations are listed as blocked, never merged. --check exits 1 when a group or
blocked variation differs from the --baseline report. Read-only, offline.
`

const breakageHelp = `Inspect an experimental declared cross-repository API relationship map.

Usage:
  corvint [--root PATH] breakage --manifest FILE --api REPO:PATH:SYMBOL
    --repository REPO=CHECKOUT [--repository OTHER=CHECKOUT] [--base FULL_COMMIT]
  corvint [--root PATH] breakage manifest --repository REPO=CHECKOUT[@COMMIT]
    --source REPO:PATH[:START-END] [--source ...] [--provider-skeleton]

Read-only, local and bounded. Explicit repository, source and provider pins select
immutable bytes. Go syntax callers and test references remain distinct from declared
flow/documentation relationships, execution, coverage and behavioral breakage.
Missing, unsupported and ambiguous relationships remain unknown; scope is INCOMPLETE.
No providers or tests are executed and no checkouts are discovered or fetched.
manifest pins HEAD (or @COMMIT), tree, origin and whole-file (or :START-END) blob
spans of the named sources only.
--provider-skeleton emits an empty EEP /3 record whose repositories match the manifest.
With --base, declared_span_change and endpoint_span_changes compare the manifest span and
each EEP /3 endpoint span on the selected source line by line; a selected API without one
resolved Go or JS/TS declaration otherwise reports only whole-file identity.
Provider records may be EEP /1, /2 or /3 (endpoint span/symbol).
Owning experimental contract: BKM-V0.
`
