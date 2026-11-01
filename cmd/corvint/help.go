package main

import (
	"fmt"
	"regexp"
	"strings"
)

const authorityStartPrompt = "Identify the active work queue, required workflow gates, and minimum context needed to safely take the next roadmap ticket"

var cemHelpActions = map[string]bool{
	"begin": true, "prepare": true, "cite": true, "mark": true,
	"status": true, "verify": true, "report": true,
	"cover": true, "discriminate": true,
	"anchor": true, "provenance": true, "export": true, "defer": true, "verify-stable": true, "gate-verdict": true,
	"bundle": true,
}

// helpSubcommands are the retired oracle's nested argparse choices (GPK-V0-062): a token after the
// command that is neither one of them nor --help is an invalid choice argparse reports before help.
var helpSubcommands = map[string]map[string]bool{
	"cem":     cemHelpActions,
	"ocm":     {"prepare": true, "link": true, "mark": true, "status": true, "verify": true, "report": true},
	"harness": {"event": true},
	"step":    {"snapshot": true, "verify": true, "env-check": true},
	"graph":   {"imports": true},
}

// helpBooleanFlags are the public command options that take no value, so a --help after one is
// still help. Every other option is scanned as taking one value (GPK-V0-062).
var helpBooleanFlags = map[string]bool{
	"--apply": true, "--attest": true, "--check": true, "--attest-cem-v2": true, "--attest-cem-v3": true, "--dry-run": true, "--enable": true,
	"--full-receipt": true, "--if-stale": true, "--json": true, "--mutate": true, "--preview": true,
	"--replace": true, "--working-tree-untracked": true,
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
		// `corvint help flows appmap build` names one subcommand of the tree (CAP-V0-042).
		if _, ok := findTreeNode(rest[1:]); ok {
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
var nestedHelpTopics = [][2]string{
	{"harness event", "harness-event"},
	{"docs corpus", "docs-corpus"},
	{"docs flows", "docs-flows"},
	{"docs maintain", "docs-maintain"},
}

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
	case len(rest) > 1 && rest[0] == "docs" && rest[1] == "flows":
		return "docs-flows"
	case len(rest) > 1 && rest[0] == "docs" && rest[1] == "maintain":
		return "docs-maintain"
	case len(rest) > 1 && rest[0] == "docs" && rest[1] == "corpus":
		return corpusHelpTopic(rest[2:])
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
		case !argparseOptionLike(token), strings.Contains(token, "="), helpBooleanFlags[token], topic == "prove" && token == "--attack-tests":
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
			if command == "harness" {
				return "harness-event", true
			}
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
	return topicText(topic) + helpExampleTrailer(topic)
}

// topicText is a help topic's authored text, the source the command tree derives from.
func topicText(topic string) string {
	if action, ok := strings.CutPrefix(topic, cemActionHelpPrefix); ok {
		return cemActionHelp(action)
	}
	if operation, ok := strings.CutPrefix(topic, "docs-corpus-"); ok {
		return docsCorpusOperationHelp(operation)
	}
	switch topic {
	case flowsAppmapBuildHelpTopic:
		return flowsAppmapBuildHelp()
	case "catalog-json":
		return capabilityCatalogJSON()
	case "docs-corpus":
		return docsCorpusHelp()
	case "docs":
		return docsHelp + "\n" + docsMaintainHelp + "\n" + docsFlowsHelp
	case "docs-flows":
		return docsFlowsHelp
	case "docs-maintain":
		return docsMaintainHelp
	case "root":
		return rootHelp
	case "query":
		return fmt.Sprintf(queryHelpFormat, authorityStartPrompt)
	case "delta":
		return deltaHelp
	case "graph":
		return graphImportsHelp
	case "impact":
		return impactHelp
	case "feature":
		return featureHelp
	case "eval":
		return evalHelp
	case "cem":
		return cemHelp
	case "ocm":
		return ocmHelp
	case "lrf":
		return lrfHelp
	case "frontier":
		return frontierHelp
	case "record":
		return recordHelp
	case "migrate-traces":
		return migrateTracesHelp
	case "migration-ratchet":
		return migrationRatchetHelp
	case "observations":
		return observationsHelp
	case "features", "overview", "review":
		return repositoryGuidanceHelp
	case "affected":
		return affectedHelp
	case "obligations":
		return obligationsHelp
	case "flows":
		return flowsHelp
	case "step":
		return stepHelp
	case "changes":
		return changesHelp
	case "runs":
		return runsHelp
	case "repro":
		return reproHelp
	case "mcp":
		return mcpHelp
	case "test":
		return testHelp
	case "tests":
		return testsHelp
	case "breakage":
		return breakageHelp
	case "necessity":
		return necessityHelp
	case "surprise":
		return surpriseHelp
	case "answerability":
		return answerabilityHelp
	case "depsource":
		return depsourceHelp
	case "kernel":
		return kernelHelp
	case "lease":
		return leaseHelp
	case "reads":
		return readsHelp
	case "calibrate":
		return calibrateHelp
	case "skill-export":
		return skillExportHelp
	case "prove":
		return proveHelp
	case "context":
		return taskContextHelp + contextLookupHelp + cpuProfileHelpNote
	case "index":
		return indexHelp
	case "dogfood":
		return localCompletionHelp
	case "batch":
		return batchHelp
	case "init", "adopt":
		return activationHelp
	case "harness-event":
		return harnessEventHelp
	case "work":
		return workHelp
	case "prove-observe":
		return proveObserveHelp
	case "adapter":
		return adapterHelp
	case "dogfood-ocm":
		return dogfoodOCMHelp
	case "witness":
		return witnessHelp
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
  corvint COMMAND --help | corvint help COMMAND | corvint help --json|guide
  corvint --version [--json]

Commands:
  Core (decision 0332; only the modes and profiles CCF-V1 lists are frozen):
    init  first repository inventory
    adopt  recoverable repository inventory
    index  write the index snapshot
    query  repository context or authority receipt
    context  task-context packet: files to read
    impact  path, untracked or range impact
    affected  affected-test plan
    prove  packet with falsifier verdicts
    cem  Change Evidence Map tool
    ocm  Obligation Closure Map tool
    frontier  Change Frontier review queue
    dogfood  evidence: change, check, seal, batch-check
  Experimental, no stability promise; owning spec in parentheses:
    graph  pinned whole-tree import edges and uncertainty (NGI-V0)
    feature  bounded feature context (GPK-V0)
    eval  evaluate a frozen corpus (REC-V0)
    breakage  cross-repository API relations (BKM-V0)
    changes  offline upgrade checklist (UCL1142-V0)
    runs  Playwright verdicts (PWR-V0)
    test  affected tests (STR-V0)
    tests  setup, lint, duplicates (TSA-V0)
    repro  pinned reproduction bundles (RPB-V0)
    harness event  one agent lifecycle event (AHI)
    adapter  one native host adapter (AHI)
    mcp  local read-only stdio MCP server (AHI)
    dogfood-ocm  verify dogfood OCM maps (OCM-V0)
    lrf  lexical relevance floor (LRF-V0)
    record  record one local task outcome (LTPM-V0)
    migrate-traces  plan or apply trace migration (LTPM-V0)
    migration-ratchet  compare migration evidence (MER-V0)
    observations  self-observation digest (SOL-V0)
    features  inferred feature candidates (RGV-V0)
    overview  immutable repository guidance (RGV-V0)
    review  range advice and branch overlaps (RGV-V0)
    flows  application flows and assertions (AFU-V0)
    obligations  CEM external-obligations sidecar (EFO-V0)
    prove-observe  record prove verdict counts (SOL-V0)
    batch  batched query, context, impact (SBQ-V0)
    work  queue observation or shadow wave (WQO-V0)
    docs  source-derived documentation (SDD-V0)
    depsource  cite pinned Go module source (DSE-V0)
    necessity  label packet file necessity (NEC-V0)
    surprise  touched set vs delivered packet (TSS-V0)
    answerability  retrieval channel agreement (RDS-V0)
    kernel  compaction kernel (CKN-V0)
    lease  local scope lease over paths (SCL-V0)
    reads  opt-in unplanned-read digest (URE-V0)
    calibrate  packet stances vs outcomes (OCL-V0)
    skill-export  learned traces as SKILL.md (LTA-V0)
    witness  unwitnessed surface of a range (AGW-V0)
    test-validity  project a live-test receipt (MTV-V0)
    test-plan  test consolidation plan (TCN-V0)
    step  declared authoring scope receipts (ASS-V0)
    delta  source-free record of one change (DLT-V0)

Global options:
  --root PATH  Repository root (default: current directory).
  --version    Print the version and build (--json: the corvint-version/1 identity).
  --help       Print this help on stdout and exit 0.

Support boundary:
  Local-only; no network or telemetry. init, adopt, query, feature and impact are
  qualified on Darwin and Linux only.
  init, adopt, query, impact, docs, harness, and lrf read without mutating
  repository or trace state. impact, feature, docs, harness event, ocm, lrf, cem,
  dogfood-ocm, witness, index, calibrate, frontier, and record best-effort append a
  bounded row to the self-observation ledger (.corvint/self-observations.jsonl)
  on an unsupported-* failure (harness event also on success); a ledger storage
  failure never alters the response. cem begin, prepare, cite and mark write local
  CEM artifacts; cem report, ocm, record, migrate-traces --apply, index, and docs
  flows and maintain write local state (COMMAND --help).
`

const adapterHelp = `Run one bounded native host adapter.

Usage:
  corvint adapter codex
  corvint adapter claude-code EVENT
  corvint adapter claude-source-handoff --packet-out FILE
  corvint adapter source-view [--root PATH] --packet FILE --packet-sha256 SHA256
    [--result N] [--evidence N] [--commit SHA] [--lines START:END] [--requirement ID] [--max-bytes N]
  corvint adapter done-list [--root PATH] [--base FULL_SHA] [--mutate]
  corvint adapter pi EVENT
  corvint adapter pi-tool context|expand|record

Reads one bounded JSON object from stdin and writes one bounded adapter response;
done-list reads no stdin. The installed host hooks invoke these commands. EVENT is
a host hook event such as session-start, pre-compact or post-compact. Adapter
responses are local-only and carry no repository authority.
`

const dogfoodOCMHelp = `Verify the private ordered set of dogfood OCM maps.

Usage:
  corvint [--root PATH] dogfood-ocm status --expected-base REV --target REV

Freshly verifies the bounded intent manifest and every per-scope OCM map against
the same CEM and revisions, then prints the fail-closed aggregate JSON result.
It does not mutate repository or trace state.
`

const witnessHelp = `Compile the unwitnessed surface of one committed range.

Usage:
  corvint [--root PATH] witness --base REV [--head REV] [--cem MAP]
      [--stable-artifact-root DIR] [--evidence FILE]...
      [--evidence-in-repo PATH]... [--json] [--check[=admitted|=candidate]]
  corvint [--root PATH] witness --range app=BASE..HEAD --range tests=BASE..HEAD
      [--range-root NAME=PATH]... [--evidence FILE]... [--evidence-in-repo PATH]...
      [--json] [--check[=admitted|=candidate]]
  corvint [--root PATH] witness --paired-from REV --range-root app=PATH
      [--range-root tests=PATH] [--evidence FILE]... [--evidence-in-repo PATH]...
      [--json] [--check[=admitted|=candidate]]

Builds the committed index, compares the range beginning at --base, and prints
the conservative unwitnessed report. It does not mutate repository or trace
state and never invents a witness. REV is any revision Git resolves to a
commit (a branch, tag, HEAD~3, or an object ID); the report carries the full
commit object ID. A --head other than the checked-out HEAD is a historical
head: its index is built from the Git object store at that commit, the
worktree is not read, and the range reports headBinding COMMIT-PINNED. A head
--base is not an ancestor of, or whose objects are missing, refuses.

--evidence FILE (repeatable, at most 8) reads one corvint-test-validity/0 or
provider document (at most 4 MiB). A Playwright receipt, RSpec provider
document or corvint-go-test-run/0 document (corvint-go-test-provider test-run)
that ran clean on exactly the head commit and tree, with file digests or blob
IDs matching the head blobs, attaches a CALLER_REPORTED witness to each
obligation at its test's anchor, imported, spec or declaring files; such a witness never
closes, and a NOT_RUN obligation stays NOT_RUN. Any other document of a kind
those readers know is an UNBOUND source that attaches nothing; an unreadable or
undecodable one, or valid JSON of an unrecognized provider kind, exits 2. --evidence-in-repo PATH (repeatable; at most 8 with --evidence) reads the
same documents by a path relative to the checkout root: an absolute path, a
.. component, or a symbolic link that resolves outside the checkout exits 2
with the reason named. The summary separates open obligations with a candidate witness from open
obligations with none.

A changed path the range engine refuses on its own (an unsafe name, a
deletion, binary content, a mode change, and similar) is a NOT_RUN obligation
with reason ADMISSION_REFUSED_<RULE> and the rest of the range is analysed;
admission is then PARTIAL. Past the 100-path bound, the paths after the first
100 in path order are each refused alone as PATH_BOUND, and a path whose hunks
would cross the hunk or target-line bound as HUNK_BOUND or TARGET_LINE_BOUND.
The admission packet holds at most 50 ranked results, so in a range of more
than 50 analysable paths an admitted path ranked out of it is a NOT_RUN
obligation with reason ADMISSION_PACKET_BOUND, admission is PARTIAL and its
detail names the omitted paths and the bound; split the range to analyse them.
A repository-level refusal, or a range of more than 1000 changed paths,
refuses the whole range.

Exit 0 prints the report, whatever it says. With --check the report is printed
and the exit is 1 when admission is not COMPUTED or any obligation is not
closed; no V0 authority closes, so this exits 1 for any obligation.
--check=admitted exits 1 only when admission is not COMPUTED, the closure is
neither COMPUTED nor EMPTY, or any obligation is NOT_RUN; --check=candidate also exits 1 when any open obligation
has no candidate witness. Neither mode closes or proves anything. Refused
input exits 2 with no stdout.

A path no extractor reads (Markdown, JSON, YAML, ...) is NOT_RUN as
LANGUAGE_COVERAGE_NONE unless the base tree commits .corvint/witness-paths.json
(profile corvint-witness-paths/0), whose rules classify globs in file order:
an inert path is EXEMPT (DECLARED_INERT) with no test obligation; a data path is
admitted text-only when a reader is declared in the rule or found quoting its
path, and stays NOT_RUN as DATA_READER_MISSING otherwise. Each classified
obligation names the declaring file and line; an invalid policy applies no
rule. Read from the base, a policy never exempts the range that edits it.
For example:
  {"profile":"corvint-witness-paths/0","rules":[
    {"glob":"docs/**/*.md","class":"inert"},
    {"glob":"**/testdata/*.json","class":"data","readers":["pkg/a/a_test.go"]}]}

A cem/1.0 map is applied only when --stable-artifact-root is given and the
stable verifier accepts it against --base and the checked-out HEAD; the report
then uses corvint-witness-stable/0. Any other stable outcome, a cem/1.0 map
without the flag, or the flag over a map that is not cem/1.0 exits 2 with no
stdout.

--range app=BASE..HEAD --range tests=BASE..HEAD compiles an application range
and a tests range as one change (corvint-witness-paired/0), each in --root or
its --range-root checkout. The report names both ranges and sums both; either
range that does not resolve exits 2 naming it. Evidence binds to the tests
head; a Playwright receipt whose application attestation names the app head
commit and tree, clean and unchanged, and whose imported app files match the
app head blobs also cites app obligations as cross-range CALLER_REPORTED
witnesses. --base, --head, --cem and --stable-artifact-root are refused.

A workspace declaration (--workspace FILE, or --root/.corvint/workspace.json)
is not read: --workspace refuses as workspace-unsupported naming the ignored
aliases, and so does a discovered one for a single-range witness unless
--no-workspace; a paired witness names its repositories with --range.

Merged history records a pairing with a trailer on the tests-repository commit,
Paired-Range: <name>=<repo>@<base>..<head>. Only --paired-from REV reads it:
the tests range is REV's only parent to REV (in --root or its --range-root
tests checkout), and the app range is the trailer's <base>..<head> in the
--range-root app checkout, which the caller must name because <repo> is a
label, never a path; without it the trailer is still read and the refusal
names <repo>. The report gains a pairing member naming REV, <repo> and
the trailer. A commit without exactly one parent, no trailer, two, a name
other than app, a value that is not full object IDs, a Paired-Range line
outside the trailer block, an ID the app checkout lacks, or a <base> that is
not an ancestor of <head> (stale) exits 2 with no stdout; --range, --base,
--head, --cem and --stable-artifact-root are refused with it. Explicit
--range flags read no trailer.
`

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

const workHelp = `Validate a repository queue observation or compile a non-operative shadow wave.

Usage:
  corvint [--root PATH] work observe
  corvint [--root PATH] work propose-wave --envelope PATH --limit N
  corvint [--root PATH] work init --repository NAME --corvint-executable ABSOLUTE_FILE
  corvint [--root PATH] work rebind --corvint-executable ABSOLUTE_FILE
  corvint [--root PATH] work adapter snapshot|details|verify

observe and propose-wave print one work-command-result/0 document on stdout.
Neither claims, leases, dispatches, edits, or closes work; a proposal authorizes
nothing. Malformed command input yields state ERROR with MALFORMED_INPUT. Both
need the three adoption files below committed in a clean worktree; otherwise they
yield SOURCE_UNQUALIFIED and stderr names the reason.

init writes .corvint/work-queue-policy.json, .corvint/worklist.json and the
executable .corvint/work-queue-adapter for the operator to review and commit; it
refuses when any exists or the Corvint executable, after resolving symlinks, is
not a safe canonical external file. rebind updates only that adapter after path,
SHA-256, version/build and source identity change; review and commit it.
Observation verifies the binding and uses no ambient PATH search. adapter prints
one document for the qualified committed worklist.
`

const proveObserveHelp = `Record one prove document's verdict counts in the local self-observation ledger.

Usage:
  corvint [--root PATH] prove-observe < PROOF

Reads one falsifiable-packet/0 document from stdin (at most 8 MiB), recounts
proof.rows, and appends one proof row carrying only those counts to
.corvint/self-observations.jsonl. It prints nothing on success. Any other,
oversized, or inconsistent document exits 2 with invalid-proof-document and
leaves the ledger unchanged.
`

const featureHelp = `Compile the implemented experimental feature context slice.

Usage:
  corvint [--root PATH] feature FEATURE_ID [--limit N] [--budget-bytes N]

Known and unknown feature IDs are supported on Darwin and Linux. --limit 1
returns known ledger-only context even with non-Go sources. Larger limits admit
Go candidate ranking; known non-Go ranking is refused with
unsupported-feature-repository. Unknown IDs remain OUT_OF_SCOPE; feature
inventory is unsupported in this explicit-ID command; use experimental features
for inferred discovery. --limit defaults to 10 (range 1-50).
--budget-bytes bounds the complete packet and retains omitted-result counts.
Repository and trace state are read-only; do not substitute a legacy runtime
for a typed unsupported result. See docs/specs/go-production-kernel-migration-v0.md.
`

const evalHelp = `Evaluate a frozen retrieval corpus through the native candidate.

Usage:
  corvint [--root PATH] eval [--goldens FILE] [--trace-fixture FILE] [--exclude-producer NAME]...
  corvint [--root PATH] eval --learn-slot-weights [--goldens FILE] [--admit]
  corvint [--root PATH] eval --reset-slot-weights
  corvint [--root PATH] eval --friction FILE

Without --goldens, use the repository's .corvint/eval.json or
testing/context-retrieval-goldens.json when available. --trace-fixture adds the
explicit learned-trace comparison arm; ordinary persisted trace replay is not
implemented. The command reads without mutating repository or trace state.
The report counts the traces it read by producer (cli, dogfood, pi-tool, or
UNKNOWN for rows written before producer provenance); --exclude-producer NAME
leaves that producer's traces out of the read, repeatable, without changing
the store or the fixture.
Select the owning spec's registered development or frozen evaluation inputs;
never open a sealed holdout or infer promotion from an unregistered run. Preserve
both arms, failures and unknowns. A routine explicit outcome record does not
require a new learned-mechanism evaluation.

--learn-slot-weights reads the local unplanned-read and self-observation ledgers
as negative labels, proposes bounded context slot weights and scores them on the
frozen held-out split. Only with --admit and an improved delta does it write
.context-corvint/slot-weights.json; --reset-slot-weights removes that file and
restores the default slot order. These experimental steps (decision 0368) are
eval's only writes.

Experimental (URE-V0-996..998): --friction FILE takes no other option and
reports deterministic friction metrics (rereads, unplanned_reads,
retried_commands, reverted_edits, first_relevant_read_ms) for each session of
a corvint-friction-sessions/0 record, totals per arm, and precision and recall
for labelled sessions. Missing inputs are NOT_OBSERVED, never 0. It reads only
FILE and writes nothing; the metrics are never a ranking or authority input.
Example: corvint eval --friction internal/frictionmetric/testdata/benchmark.json
`

const activationHelp = `Compile one bounded mechanical repository inventory.

Usage:
  corvint [--root PATH] (init | adopt) [--authority-id ID] [--revision REV]
    [--exclude-prefix PATH] [--full-receipt]
  corvint [--root PATH] adopt --setup

--revision defaults to HEAD. --exclude-prefix is repeatable. The default output
is a bounded sealed summary; --full-receipt emits the complete canonical receipt.
Both activations are read-only and local-only.

Experimental (ADS-V0): adopt --setup takes no other option and reports the
repository's Corvint setup gaps (index snapshot, agent instructions, hooks,
ignored paths), each with its evidence and a suggested fix. It writes nothing.
`

const recordHelp = `Record one explicit local task outcome.

Usage:
  corvint [--root PATH] record --task TASK [--opened PATH] --changed PATH
    (--verify COMMAND | --verify-argv-json JSON_ARRAY) --outcome (passed | failed | blocked)

--opened, --changed, --verify, and --verify-argv-json are repeatable; --opened defaults to empty.
--verify alone retains schema 1. Adding an argv array writes typed schema 2;
arguments retain literal spaces, JSON and operators and are never executed.
Each argv has 1..32 strings, at most 512 characters per argument and 4096
canonical JSON bytes; credentials and control characters refuse. Mixed inputs
share the 50-entry bound. Older binaries cannot read a mixed-version store.
The repository must be clean, every path must be a safe source at HEAD, and
verification commands are secret-screened and shell-bounded. The command
atomically writes one local trace row and is idempotent by trace id.
`

const migrateTracesHelp = `Plan or apply legacy tree-trace migration.

Usage:
  corvint [--root PATH] migrate-traces --dry-run
  corvint [--root PATH] migrate-traces --apply --plan-digest LOWERCASE_SHA256

Exactly one of --dry-run and --apply is required. Dry-run validates the complete
bounded trace store and writes nothing. Apply requires the exact unchanged
dry-run digest, publishes canonical commit traces and byte-preserved quarantine
copies, then removes verified legacy candidates. A trace whose commit still
exists but is no longer reachable from HEAD (after an amend or rebase) is listed
in stranded_revisions and moved, byte-preserved, into the same quarantine. The
command is local-only.
`

const migrationRatchetHelp = `Compare two immutable migration-evidence snapshots.

Usage:
  corvint migration-ratchet --profile FILE [--advisory] [--compare VERDICTS.json --log FILE.jsonl]
    [--log FILE.jsonl] [--label NAME]
  corvint migration-ratchet schema
  corvint migration-ratchet seal (--profile FILE | --snapshot FILE)
  corvint [--root PATH] migration-ratchet span-digest --revision FULL_COMMIT --path PATH --lines START:END

The comparison writes a deterministic corvint-migration-evidence-ratchet-receipt/1 JSON receipt.
Exit 0 is pass, exit 1 is fail or unknown, and exit 2 is refused input.
schema prints every accepted field, record kind, policy acceptance, refusal pattern, the
canonical digest encoding and a sealed example profile that passes.
seal prints the input with every self-digest and canonical order filled; it never changes
content digests, states, links or a migration rule's reviewed artifact bindings.
span-digest prints the Git blob and SHA-256 of one line span at an immutable commit, for a
sub-file test-execution content_sha256. Every form is read-only. Owning contract: MER-V0.

--advisory exits 0 where the ratchet would exit 1; refused input still exits 2. --log appends one
corvint-gate-agreement/0 JSONL record per run, and --compare (needs --log) compares the ratchet's
findings with a corvint-gate-verdicts/0 caller verdict file. Each finding is keyed as a rule on a
test ID: a policy failure CODE:ID as rule CODE on test ID (rule CODE alone without an ID), a
removal as rule removal:KIND on the baseline test ID, a regressing state change as rule
state-regression:KIND on the candidate test ID, and a broken reverse link as rule
broken-reverse-link:KIND on the candidate (else baseline) test ID; an excepted delta abstains.
--label NAME (once) is copied to each record's label, so logs of several callers can be split.
"corvint tests agreement --log FILE" summarises the agreement per rule (GVA-V0).
Example: corvint migration-ratchet --profile p.json --advisory --compare v.json --log /tmp/a.jsonl
`

const observationsHelp = `Read the bounded local self-observation digest.

Usage:
  corvint [--root PATH] observations [--limit N]

The command reads .corvint/self-observations.jsonl and writes a maximum of 120
proposal lines to stdout. It never creates, rotates, or files observations.
When prove-observe has recorded proofs, the digest opens with the
repository's falsification rate (FALSIFICATION) and one FALSIFIER line per
falsifier: judged rows are PASS or FAIL, failed rows are FAIL. Host adapter
degradations are tallied as ADAPTER-DEGRADATION lines by host/event/code,
counting retained hour windows with the newest window; an out-of-project
post-tool target is a correct exclusion and is tallied as ADAPTER-EXCLUSION.
`

const flowsHelp = `Inspect experimental web-flow assertion candidates and runtime observations.

Usage:
  corvint [--root PATH] flows --manifest FILE [--evidence FILE]
  corvint [--root PATH] flows record --manifest FILE --evidence FILE --output FILE

Read-only inspection binds declared tracked source and test files to Git and emits
application-flow-report/0. Static assertions are candidates, not passing tests.
Stale, missing, unsupported and contradicted evidence remains visible. Universal
application completeness is never claimed. Core reads launch no browser or server.

The separate corvint-web-flows companion parses JS/TS and optionally observes an
explicitly trusted local app. Its proposed execution profile remains experimental.
record explicitly writes one screened private evidence file and refuses overwrite;
it does not update ranking or accept inferred intent. No automatic ledger is written.
` + flowsIntentHelp + flowsNavigateHelp + flowsAppmapHelp + flowsAppmapPlanHelp + flowsRequestPathHelp

const obligationsHelp = `Compose the external-frontier-obligations-stable/0 sidecar for one cem/1.0 map.

Usage:
  corvint obligations --cem FILE --impact FILE [--limit N] --repository DIR
      --expected-base REV --target REV --stable-artifact-root DIR
      [--allow-absent-sidecar] [--intent-overlay FILE]

Reads one cem/1.0 map and one saved corvint impact --provider receipt, admits
the map only through the stable verifier's ACCEPT with the exact committed
sidecar, joins every map hunk to the receipt's context.external rows by path,
and writes one external-frontier-obligations-stable/0 document to stdout bound
to the accepted map, patch and target. Each hunk row lists associations of kind
entity, obligation, test, or unknown; every non-unknown association carries its
provider evidence reference, EEP path verification, and provider freshness, and
none states that the record justifies the hunk. Declared criterion links are
listed in their own criteria member and state no satisfaction. --limit bounds
the associations per hunk (default 64); omitted rows are counted.
--allow-absent-sidecar also admits an accepted map that is not committed at
the target (an out-of-tree, read-only review); the document then carries
binding.sidecar ABSENT and binds the map by its digest alone. A mismatched
committed sidecar still refuses.
--intent-overlay FILE (experimental, CEM-V1-642) reads a reviewer-supplied
Markdown intent from an absolute path outside the repository and its Git
directory (a symlinked, missing or oversized file refuses) and adds an advisory
reviewer_overlay member listing its requirement IDs, each UNKNOWN. The overlay
is not project authority and never changes the state, hunks, providers,
criteria, id or exit status.

The retired external-frontier-obligations/0 form is no longer produced: a
retired cem/0.x map refuses with unsupported-spec and its recovery line, and any
other input without the four stable flags refuses with
invalid-obligations-input.

Exit status: 0 with the sidecar on stdout; 2 with an error envelope on stderr
for an argument error, an unreadable or oversized input, a retired cem/0.x map,
a non-cem/1.0 document, a stable verifier refusal, or a non-impact receipt.
`

const affectedHelp = `Compile the affected-test selection plan for the dirty worktree.

Usage:
  corvint [--root PATH] affected [--full] [--require-clean]
          [--cache DIR | --no-cache] [--format json|text]
  corvint [--root PATH] affected [--full] --path PATH [PATH ...] [--path PATH ...]
          [--paths-from FILE] [--cache DIR | --no-cache]
  corvint [--root PATH] affected --by-path --path PATH [PATH ...]
          [--paths-from FILE] [--cache DIR | --no-cache] [--format json|text]
  corvint [--root PATH] affected [--full] --snapshot RECEIPT
  corvint [--root PATH] affected --snapshot RECEIPT --playwright-config PATH
  corvint [--root PATH] affected [--full] [--require-clean]
          --base FULL_COMMIT_ID
  corvint [--root PATH] affected [--base FULL_COMMIT_ID]
          --playwright-config PATH [--playwright-discovery FILE]
          [--playwright-env NAME[=VALUE] ...]
  corvint [--root PATH] affected discovery --playwright-config PATH
          --playwright-list FILE [--playwright-ungrepped-list FILE
          [--playwright-ungrepped-config PATH]]
  corvint [--root PATH] affected cache [--format json|doctor-plugin]
  corvint [--root PATH] affected audit --changes FILE
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
degradations, present only when it applies, names self-binary-stale: inside
its own repository this binary was built from a commit whose Go source differs
from HEAD, so the plan may not be the one current source gives, or
self-binary-unverified when that comparison could not complete; stderr repeats
it with the rebuild command (AFP-V0-846).
tiers (when anything is selected) ranks every selected unit without changing
the selection: 0 changed packages, 1 direct importers and same-parent
siblings below a top-level directory, 2 the rest; summary is "tier0=N tier1=N tier2=N". A lane
runs tier 0 and 1 locally (at most 15 packages, ./cmd/corvint only when changed
directly) and records tier 2 and anything skipped as NOT_RUN for CI.
summary restates the plan in one object: scope, dirty, selected, excluded and
unknown counts, unknownByReason {REASON: count}, and line, the same as one
text line ("scope=UNKNOWN dirty=5 selected=1 excluded=0 unknown=3
UNOWNED_DIRTY_PATH=3"), so a reader needs neither the plan nor a JSON tool
to learn what was decided; it never changes the selection.
--format text (AFP-V0-928) writes the same affected-plan/1 plan as lines
instead of JSON: "affected <summary.line> revision=SHA [base=SHA]", a
"warning TEXT" line when the plan read unpinned worktree content, one
"selected UNIT tests=N WITNESS <- DIRTY_PATH [via A > B]" line per selection,
one "unknown REASON DETAIL [hint TEXT]" line per plan unknown, the exclusion
count and digest, and one "check KIND COMMAND" line per advice check with the
advisory packages appended. It cannot be combined with --full or
--playwright-config; --format json (the default) is the JSON document.
Example: corvint affected --base FULL_SHA --format text
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
joined. The base must be a full commit id present in the repository.
plan.dirty is that union ("dirty" means changed since the base), so a clean
worktree still lists its committed range paths there (range.paths names them);
UNOWNED_DIRTY_PATH marks the union's unowned paths from either component.
When a committed range path is not in the worktree dirty set and the worktree
is dirty, the receipt carries a top-level warning naming HEAD and the dirty
count: the plan mixes that commit with mutable worktree content.

--require-clean (experimental; not with --snapshot) refuses with exit 2 and
unsupported-affected-mixed-worktree when the worktree status has any entry,
before the range diff, graph walk or provider; a clean worktree is unchanged.

--by-path (experimental, AFP-V0-990; requires --path or --paths-from, not
with --full) builds the graph once and writes one affected-by-path/1 document
instead of the plan: sharedUnboundedReaders lists once the units the union plan
selects as UNBOUNDED_READER (they are selected on any path), and paths gives
each named path's scope, how many of that shared set it selects, its unknowns
and only the units it adds. --format text writes "affected by-path scope=S
paths=N shared=K revision=SHA", one "shared UNIT tests=N WITNESS" line per
shared unit, then per path "path P scope=S shared=J/K additions=M" with its
"  added UNIT tests=N WITNESS" and "  unknown REASON DETAIL" lines. A stale
or unverified binary (AFP-V0-846) is named on stderr, in degradations, and on a
"degraded CODE" text line.
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
playwright-report/ bundle, is instead excluded from the graph. One --path
takes every following value up to the next option, so --path a b c and
--path a --path b --path c plan the same bytes. Write --path=VALUE for a name
that begins with -. It does not combine with --base (invalid-arguments).
Under --snapshot, --provider, the Playwright profile (--playwright-config,
--playwright-discovery) or --appmap-impact it refuses with exit 2 and
unsupported-affected-path-profile, naming why that profile plans its own
change set and the recovery: commit the paths and plan with --base <HEAD>.

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

affected audit (experimental) reads a corvint-affected-audit-input/0 FILE
listing changes: id, full base commit id, and kind replay (full head), revert
(full commit id reverted on base) or mutant (diff file applied on base);
junit or playwright-json baseline and changed outcome files relative to FILE,
or none (FILE, outcome, diff and map files must be regular files; a symlink is
refused); optional path_prefix, appmap {map, ledger} and e2e {manifest,
manifest_repo, repos, repo_revisions}, where repos maps an alias to an absolute
tests-repository root and repo_revisions an alias to a full commit id, passed
to e2e-compile as --repo, --manifest-repo and --repo-revision; the report
records each root e2e-compile read and its revision. In a
private shared clone it runs affected on every change, appmap-impact and
e2e-compile (the corvint-test companion) where configured, treats each test whose outcome changed or is new
as one that had to run, and writes a corvint-affected-audit/1 report: per
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

const proveHelp = `Compile the falsifiable context packet for a task, a change, or a CEM map.

Usage:
  corvint [--root PATH] prove --task TEXT [--limit N] [--budget-bytes N]
  corvint [--root PATH] prove [--limit N] [--mutate] PATH...
  corvint [--root PATH] prove --base FULL_COMMIT_ID [--limit N] [--mutate]
  corvint [--root PATH] prove --cem MAP [--expected-base REV] [--target REV] [--patch FILE]
    [--attest | --attest-key PEM]
    [--attest-cem-v2 | --attest-cem-v3 [--artifacts ABS] [--max-unknown N] [--max-mechanical N]
    [--paired-evidence DOC]]
  corvint [--root PATH] prove --verify-cem-attestation ENVELOPE --attest-public-key PEM
    [--cem MAP] [--artifacts ABS] [--target REV]

Checkpoint mode:
  corvint [--root PATH] prove --checkpoint FILE

Read a caller-owned canonical corvint-checkpoint/0 JSON document (at most 256
KiB, 256 handles and 256 critical selectors). Compare handles against the
current Git tree and rehydrate eligible critical selectors by identity. FILE
may be outside the repository. This mode is exclusive with --task, --cem,
--base and --mutate. It writes no repository, index, session or ledger state.
Stored task prose and verification rows are caller-attributed prior observations;
they are echoed unchanged, never executed or promoted to current evidence.
An unchanged handle confirms Git content only, not task correctness or that
anyone read it. Check exit status: a transport failure can leave partial output.

The packet modes compile the same document as query (with --task), impact (with
paths), or cem verify (with --cem), embeds it unchanged, and writes one
falsifiable-packet/0 document to stdout with one verdict per row under
proof.rows. falsifier names the mechanical check that would fail if the row
were wrong: history-consistent for citation checks;
reference-resolves for Go, Python, JavaScript, and TypeScript import rows and
Go and web same-package reference rows; verifier-accepts for supported and
mechanical CEM hunks;
test-kills-mutant for same-package Go test rows; none for advisory history or
an explicit unknown. falsified is PASS, FAIL, or NOT_RUN. Every check asks Git,
a parser over the committed blob, the frozen CEM verifier, or the test itself,
never the index that made the row. The history-consistent claim is:
the cited blob and line still exist at the packet revision; this is a check of the citation, not evidence that the row is relevant
A READY packet whose only falsifier-bearing rows are syntax rows checked by
history-consistent reports state CITED.
reference-resolves parses both sides: an import spec with the claimed path on
the cited line, or an identifier with the claimed name on the cited line plus
a top-level declaration of it in the declaring blob. verifier-accepts passes
when the verifier accepts the map and the hunk's evidence is stable or
relocated at the target; a rejected map fails every claim row and reports
state REJECTED. A dirty path is NOT_RUN. A result is proven only when every
falsifier-bearing row passes; an accepted document with no proven result
reports state UNPROVEN. CEM mode refuses --base and --working-tree-untracked;
the separate committed-range change mode below accepts --base.

Change mode (--base) compiles the same document as impact --base for the
committed range FULL_COMMIT_ID..HEAD and adds one affected-test row per test
the affected-plan selector reaches from the range's changed paths, claiming
that test covers the changed path that reached its unit; proof.affected
carries the plan's graph digest, scope, selected count, and unknown count.
Go _test.go rows and pytest test_*.py or *_test.py rows get test-kills-mutant
when the changed path is source of the same language; every other row gets
none. Each row keeps its own verdict in proof.counts, but the summary counts
affected-test rows per changed path: proof.affected.paths lists, for every
changed path a row claims, the tests whose row passed (covered_by), the FAIL
rows (reached_not_covering), and the NOT_RUN rows (not_run); a path counts
once as proven when covered_by is non-empty, failed when it is empty with a
FAIL row, unproven otherwise. --base with paths or --working-tree-untracked
is refused.

Explicit Go test attack (experimental):
  corvint [--root PATH] prove --base FULL_COMMIT_ID --mutate --attack-tests

Continue through the bounded selected mutant set (at most eight per row), even
when an earlier mutant was killed. Each Go mutation row adds attack counts and
surviving mutant locations bound to the committed blob. COMPLETED means only
that this selected set was judged, never exhaustive mutation or adequate tests.
PARTIAL retains observed counts and survivors when the budget runs out; missing
sandbox, baseline failure, no mutable lines and unsupported languages remain
NOT_RUN with their reasons. The existing falsified verdict keeps its meaning:
a PASS can coexist with survivors. Only changed lines are mutated, with the
same row and invocation deadlines, sandbox and temporary-copy cleanup. This
flag requires --base and --mutate and never runs automatically from a hook.

Named Ruby source (experimental):
  corvint [--root PATH] prove [--base FULL_COMMIT_ID] --mutate
    --mutate-source PATH.rb[:START-END[,START-END...]]
    --mutate-example SPEC_spec.rb:LINE[:LINE...] [--mutate-example ...]

Mutate a named .rb source, changed or not, against the cited RSpec examples
(at most 4 spec files, 16 lines each, 16 spans). Each spec file becomes one
named-source-test row judged like an RSpec row, but every run passes
SPEC:LINE:LINE so only the cited examples run. Mutants stay inside the given
spans, else the whole file under the twelve-mutant cap; the mutation record
names the basis (line_basis caller-spans or whole-file) and the selector
(target). In impact mode the source is added to the paths.

Containerised Ruby runner (experimental):
  corvint [--root PATH] prove ... --mutate --mutate-runner FILE

FILE is a corvint-rbmutate-container/0 declaration: executable and
executableSha256 (the pinned container CLI), execPrefix (its words up to and
including the rspec command, naming {run}), container (where {run} is
mounted) and workspace (an existing host directory the runtime can mount,
neither inside nor containing the repository).
Every Ruby mutation run then goes through the corvint-test-runner rspec
profile instead of the host sandbox: the baseline and each mutant get a fresh
disposable copy of the export under the workspace, removed afterwards, and
each receipt carries its own admitted runner_receipt. A run whose report that
runner does not admit earns no verdict (report_error names why). The container
is the isolation; Corvint does not pin its image or network beyond the prefix.

test-kills-mutant rows stay NOT_RUN unless --mutate is given (impact and
change modes only). Every mutation row of one invocation shares a
thirty-minute budget beside the ten-minute row budget; rows are judged in
document order and a row reached after it is spent is NOT_RUN with the
detail "invocation mutation budget exhausted before this row". --mutate exports the packet revision once into a temporary directory
and judges every test row on that copy: it runs the cited test file as a
baseline, applies up to eight single-operator mutants to the changed file one
at a time, and stops at the first kill: PASS when a
mutant dies, FAIL when every mutant survives, NOT_RUN with a detail line when
nothing was mutable, the ten-minute budget ran out, the baseline failed, or
dependencies were unavailable offline. A Python row is judged the same way by
python3 -m pytest -q -x on the cited test functions, with one-line token
rewrite mutants that Python must first parse; without python3 or pytest the
row stays NOT_RUN with the reason. An RSpec row (a *_spec.rb file claiming a
changed .rb source) is judged by bundle exec rspec, or rspec on PATH, against
up to twelve relational, arithmetic, integer-constant and condition-negation
mutants of the changed lines; every mutant runs, and the row carries a
mutation record with each mutant's verdict and RSpec receipt. Without a
runner, or with a baseline that does not pass, the row stays NOT_RUN with an
UNSUPPORTED witness naming why. Every cited test runs inside a host
sandbox (macOS sandbox-exec, Linux bwrap) that denies network access, signals
to outside processes, and every write except to a scratch directory emptied
before each run and the invocation's build cache; the exported revision is
read-only to the tests, and each run's process group is killed afterwards. A
run that ends without go test's, pytest's or RSpec's own failure report is an error,
never a kill. A host without a sandbox runs no test and the row stays NOT_RUN. The
repository is never written.

--attest (cem mode) emits an in-toto Statement v1 whose predicate is the
document and whose subjects are the map's sha256 and the target commit. A
repository-relative map is named by its path; an absolute map outside the
repository is named external:<sha256 of its bytes>, so the signed bytes carry
no host path (CEM-V1-641).
--attest-key PEM implies --attest and signs the statement into a DSSE envelope
with the Ed25519 PKCS#8 private key at PEM; the key is only ever read, and a
missing or malformed key exits 2 with attest-key-unavailable. Without
--mutate the command runs no test; it never writes.

Experimental, not a stable contract: --attest-cem-v2 (cem mode) implies --attest
and prints a second line after the unchanged statement or envelope, signed with
the same key when --attest-key is given. It takes a cem/1.0 stable map only
(else attest-cem-v2-requires-stable-map), runs the stable verifier with
full-OID --expected-base and --target and the --artifacts root, and only for an accepted map (else attest-cem-v2-not-accepted)
prints cem/v2: subjects are the map then each declared artifact, and the
predicate binds baseRevision, patchSha256 and the target commit and tree.
--max-unknown N and --max-mechanical N are the cem status policy ceilings: an
accepted map over either exits 2 with attest-cem-v2-policy-exceeded and
attests nothing. --attest-cem-v3 is the same with the additive cem/v3
predicate (cem-additive-profiles-v0, CAX-V0-001): it adds counts {total,
supported, unknown, mechanical} and ceilings naming exactly the applied
--max-unknown and --max-mechanical; a cem/v2-only verifier refuses it by
type, and the verifier below compares its counts with --cem MAP.
--paired-evidence DOC (cem/v3 only, CAX-V0-034) also signs pairedRange: the
name, baseRevision and headRevision of a cem-paired-evidence/1 document bound
to the map, and the SHA-256 of its bytes; the verifier reports it.
A verifier PASS row means the cited evidence is present at
the target; it is not a test execution.
The retired cem/0 and cem/v1 statements are no longer produced or verified.
--verify-cem-attestation ENVELOPE checks a cem/v2 envelope offline against the
Ed25519 PKIX key PEM: VERIFIED if --cem MAP matches, else NOT_RUN (cem-bytes-not-supplied);
--artifacts ABS and --target REV check the artifact and target subjects.
It exits 2 with attest-public-key-unavailable, attest-envelope-unavailable,
attest-verification-failed, attest-cem-mismatch, attest-cem-subject-mismatch,
attest-cem-target-mismatch, or map-unavailable.

Experimental failure bundles (decision 0361):
  corvint [--root PATH] prove --export-bundle (--task TEXT [...] | --checkpoint FILE)
  corvint [--root PATH] prove --replay-bundle FILE

--export-bundle prints one historical corvint-failure-bundle/0 JSON document
to stdout, never a file: the exact arguments, HEAD commit, tree and cited blob
ids, the engine version and profile, the checkpoint bytes, and the original
receipt or refusal. It needs a clean worktree and refuses secret-shaped text
(bundle-secret-detected) and bundles over 8 MiB. --replay-bundle reruns it on
this checkout and prints a historical corvint-failure-replay/0 report,
reproduced (exit 0) or diverged (exit 1), or exits 2 with invalid-bundle,
unsupported-version, tampered, missing-input, incompatible-engine,
missing-git-object, drift, or mixed-worktree. Neither is a fresh proof.

proof.ledger is the repository's falsification rate: the share of judged
rows (PASS or FAIL) that failed, over every proof recorded with
prove-observe, which reads a prove document on stdin and appends only its
verdict counts to .corvint/self-observations.jsonl. The block is read from
the ledger, never written by prove, carries no authority, and is absent
until a proof has been recorded; corvint observations prints the same rate
per falsifier.
`

const lrfHelp = `Verify repository authority and apply the lexical relevance floor.

Usage:
  corvint [--root PATH] lrf --cem MAP [--ocm MAP] [--patch PATCH]
    [--expected-base REV] [--target REV]

Options:
  --cem MAP           Required map, read from a bounded regular file.
  --ocm MAP           Optional OCM; not read, since no map is admitted.
  --patch PATCH       Not read, since no map is admitted.
  --expected-base REV Not read, since no map is admitted.
  --target REV        Not read, since no map is admitted.

lrf/0 is frozen over the retired cem/0.x maps and evaluates no cem/1.0 map, so
the command admits no map: a retired cem/0.x map refuses with unsupported-spec
and its recovery line, and a cem/1.0 map refuses with unsupported-spec, both
with no LRF stdout and exit 2. A missing or unreadable --cem also exits 2. The
command is read-only and local-only.
`

const frontierHelp = `Compile the deterministic Change Frontier V0 review queue.

Usage:
  corvint [--root PATH] frontier --cem MAP --ocm MAP --expected-base REV
    --target REV [--artifacts DIR]
    [--command FILE --observation FILE --report FILE] [--json]

Options:
  --cem MAP           Required verified cem/1.0 artifact. A retired cem/0.x
                      map refuses with unsupported-spec (exit 2).
  --ocm MAP           Required canonical ocm/0.1-experimental artifact bound to it.
  --expected-base REV Independent expected base revision; never inferred.
  --target REV        Independent target revision; never inferred.
  --artifacts DIR     Required for a cem/1.0 map, refused with any other: the
                      artifact root the stable verifier reads (no default).
                      A cem/1.0 map yields the frontier-stable-view/0 view,
                      whose state is always NOT_PRODUCED (exit 1).
  --command FILE      Canonical test command, for dynamic test mode.
  --observation FILE  Canonical test observation, for dynamic test mode.
  --report FILE       Raw JUnit report, for dynamic test mode.
  --json              Emit the canonical frontier-stable-view/0 document instead
                      of the human rendering. Both render one computation.

The three dynamic options are all-or-none. Supplying none records test mode
STATIC; supplying all three records DYNAMIC_CALLER_REPORTED, which raises no
authority: caller-reported evidence never closes an item. Any partial
combination is refused.

Exit codes are a result, not a severity: 0 is a valid empty frontier, 1 is a
valid open frontier carrying at least one item, and 2 is operational failure
with no frontier document on stdout and one bounded error object on stderr.
Exit 1 is success.

Each emitted item asserts only that no qualifying relation was accepted for its
named obligation after deterministic verification of the declared inputs and
profiles. It is not proof that evidence does not exist, that code is wrong,
that a test lacks behavioral coverage, that a capability is impossible, or that
the repository was exhaustively searched. The command is read-only, local-only,
and makes no model, network, or service call. It carries no stop decision and
no hook authority.
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
    --target REV --artifacts DIR [--ocm OCM] [OPTIONS]
  corvint [--root PATH] cem cover --map MAP --coverprofile PATH --test-run ID [--output MAP] [--states-output PATH]
  corvint [--root PATH] cem discriminate --map MAP --target REV [--max-hunks N]
    [--max-mutants N] [--wall-time DURATION]
    [--same-change-test SELECTOR=PATH[,SELECTOR=PATH...]] [--output MAP]
  corvint [--root PATH] cem anchor --map MAP [--commit REV] [--artifacts DIR]
  corvint [--root PATH] cem provenance --commit REV [--notes-tip OID] [--verdicts [--verdicts-tip OID]]
  corvint [--root PATH] cem export --map MAP --expected-base REV --target REV
    --artifacts DIR --output DIR
  corvint [--root PATH] cem defer --map MAP --intent PATH --reason TEXT
  corvint [--root PATH] cem verify-stable --repository PATH --map MAP
    --expected-base REV --target REV --artifacts DIR [--trusted-keys FILE]
    [--non-regular unknown]
  corvint cem bundle --request FILE --out DIR

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
           dogfood change prints the digest in its recites-cited-hunk fix line.
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
  discriminate
           Mutate the map's changed Go hunks (at most --max-hunks hunks and
           --max-mutants mutants each, within --wall-time; defaults 8, 8, 10m)
           against the _test.go files their test claims cite, and its Ruby
           hunks against the cited _spec.rb files under RSpec (experimental,
           TCQ-V0-066; bundle exec rspec beside a Gemfile, else rspec on
           PATH), and record a
           discriminates / survived / not-run witness pinned to --target and
           the test selection digest. The review projection shows each
           hunk's state; the run never fails the build. Every arm follows
           one runner contract (experimental, TCQ-V0-067..070): only a
           parsed failing test kills; a mutant that does not build or load,
           or a timeout, is neither. --same-change-test (experimental,
           TCQ-V0-071) adds a test file the change itself adds to that hunk's
           run: it is not evidence and is never cited; only the observed run
           can witness the hunk, and the witness detail names the test.
           SELECTOR is the one-based ordinal or full hunk ID cite --hunk
           takes (TCQ-V0-093).
           The copy reproduces a tracked symlink that stays inside the tree;
           one that would leave it, or that the host cannot create, fails
           the run, and the result then names runnerUnavailable
           sandbox-unpack-symlink (TCQ-V0-092).
           Toolchain prerequisites, never installed: git and the host
           sandbox (bwrap on Linux, sandbox-exec on macOS); go on PATH for
           Go hunks; for Ruby hunks ruby with rspec on PATH, or bundle on
           PATH with the Gemfile's gems already installed. An absent or
           unstartable one leaves its hunks not-run with a detail naming
           the cause (runner-not-found or bundle-unavailable), the probed
           command and its exit status, and the result names
           runnerUnavailable with runnerUnavailableHunks (TCQ-V0-094).
  anchor   (experimental) Write a pointer to the map committed at HEAD as the
           refs/notes/corvint note of REV (default HEAD); refuses a dirty or
           uncommitted map, a map not describing REV's patch, a different note.
           With --artifacts DIR, also verify the map against REV as cem verify
           does and append its verdictSha256 to REV's append-only
           refs/notes/corvint-verdicts ledger; a different verdict for the
           same map and REV is refused, an identical one writes nothing.
  provenance  (experimental, read-only) Report REV's Corvint anchor, Git AI
           note, Assisted-by/Agent-Logs-Url trailers as untrusted rows; with
           --notes-tip OID (or --verdicts-tip OID), rollback when OID is not an
           ancestor of its ref; --verdicts lists REV's verdict ledger records.
  export   (read-only) Verify the cem/1.0 map as cem verify does (with
           --artifacts) and require the target to commit it, then copy the map
           as cem.json and each declared artifact under artifacts/ into the new
           absolute directory DIR outside every worktree, with a
           corvint-cem-stable-bundle/1 manifest. The directory can move and
           still verify with cem verify-stable --map DIR/cem.json
           --artifacts DIR/artifacts.
  defer    (experimental, CEM-V1-643..646) Record that the change defers the
           intent scope PATH, with a one-line --reason of 1..200 bytes, as the
           tracked file .corvint/deferred-intents/<SHA-256>.json the change
           creates (at most 4 intents, merged into the change's one record).
           It names PATH's base blob (or null when absent at the map's base)
           and its obligation count. Commit the record with the change so the
           map's hunks and patchSha256 bind it; it is never a map member.
  bundle   Assemble a cem/1.0 stable bundle for an existing base..target
           without committing the map or writing the repository. --request
           FILE is a cem-stable-plain-assembly/1 request that names the
           repository, both revisions, the cem prepare/cite map (built in a
           disposable clone) and the runner plan and receipt by path and
           SHA-256; --out DIR is a new absolute directory outside the
           repository. It writes DIR/stable.json and DIR/artifacts/, checks
           the map with the stable verifier first, and prints the assembly
           result with its cem verify-stable next action. It takes no --root.
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
  --max-deferred-intents N
                       status and report: admit at most N recorded deferred
                       intent scopes (default 0). An accepted map whose hunks
                       create a .corvint/deferred-intents/ record adds
                       deferredIntents beside the result; a record over the
                       bound or failing its checks exits 1 with state
                       unchanged. verify refuses it.
  --citation           status adds the map's citation class beside the result;
                       an accepted result always carries its citation block.
  --scaled-operations  status only, opt-in: raise the stable verifier's Git
                       operation bound from the cem/1.0 protocol's fixed 1,024
                       to 1,024 plus 4 per hunk and evidence record; every
                       other bound is unchanged. The dogfood flow retries with
                       it after unsupported-resource-limit.
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
                       against --max-unknown; a data rule's hunk stays unknown
                       and names its readers. A code path or a supported or
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
  status, verify and report read --map MAP repository-relative, or as an
  absolute path outside the repository and its Git directories (for example an
  assembled or exported bundle's map); an absolute path inside them is refused,
  so name it repository-relative. The map binds the repository by object IDs;
  the read is bounded, a symlinked map is refused, and only report writes (its
  review file, never over the map). verify-stable reads its --map at any path;
  prove --cem and --verify-cem-attestation --cem read MAP the same way, naming
  an out-of-tree map external:<sha256 of its bytes> in both statements, never
  its host path (CEM-V1-641). cite, mark, cover and discriminate read an
  out-of-tree --map under the same rules only with a repository-relative
  --output, because Core writes a map only under the repository's lock and
  never in place outside it (CEM-V1-647); every other action takes MAP
  repository-relative only. anchor and its
  --artifacts verdict ledger read the map committed at HEAD under MAP, so an
  out-of-tree map must be committed in the product repository before it can
  be anchored; there is no out-of-tree anchor or verdict-ledger mode yet.
  No action takes a paired change: --range, --range-root and --paired-from
  are refused with the reason, because a cem/1.0 map binds one repository's
  base and target; pair the ranges with corvint witness (CEM-V1-648).
  prove --cem and --verify-cem-attestation --cem refuse them with the same
  reason.
  bundle takes absolute clean paths only: --request, --out and every
  request path. Each path's ancestor directories, and the whole repository
  path, are resolved to their real paths first, so a macOS $TMPDIR path
  under /var is read as /private/var,
  and the result and next action name the resolved paths. A final path
  element that is a symlink is still refused (candidate-input), and an
  ancestor that cannot be resolved is refused as bundle-path-unresolved,
  naming the path.

Workspace: a CEM cites one repository. Every action but verify-stable,
gate-verdict and bundle refuses --workspace FILE, or a discovered
--root/.corvint/workspace.json without --no-workspace, as
workspace-unsupported naming the ignored aliases.

cem/1.0 is the only wire. Verification derives the canonical
whole-repository patch itself excluding exactly .corvint/change.cem.json. A
retired cem/0.1, cem/0.2 or cem/0.3 map refuses with unsupported-spec and its
migration fix. All commands are local-only; reports can disclose repository
information and stay local by default.
` + cemGateVerdictHelp

const ocmHelp = `Prepare, close, or verify an Obligation Closure Map and its bound CEM.

Usage:
  corvint [--root PATH] ocm prepare --target REV --intent PATH [OPTIONS]
  corvint [--root PATH] ocm link --map MAP --obligation ID_OR_ORDINAL
    --hunk ID [--hunk ID...] --test-path PATH --claim SELECTOR
    [--claim SELECTOR...] [--accumulate] --expected-base REV --target REV
    [OPTIONS]
  corvint [--root PATH] ocm mark --map MAP --obligation ID_OR_ORDINAL
    --reason REASON [--output MAP]
  corvint [--root PATH] ocm status --map MAP --expected-base REV --target REV
    [OPTIONS]
  corvint [--root PATH] ocm verify --map MAP --expected-base REV --target REV
    [OPTIONS]
  corvint [--root PATH] ocm report --map MAP --expected-base REV --target REV
    [--output REPORT] [OPTIONS]

Options:
  --cem PATH          Bound CEM map (default: .corvint/change.cem.json).
  --intent-form FORM  Experimental prepare intent form: requirements (default),
                      adr-decisions, roadmap-acceptance, openspec-spec,
                      openspec-delta, or speckit-spec.
  --max-unknown N     Optional non-negative unknown-obligation ceiling.
  --expected-base REV Independent expected base; required for the cem/1.0 map.
  --target REV        Independent target; required for the cem/1.0 map.
  --replace           prepare only: replace an existing map that binds a
                      different target, intent or CEM (otherwise refused
                      with map-outdated) or that no longer verifies. The
                      replaced map's links, marks and assessments are
                      discarded, not carried over; copy the map aside first
                      to keep that review.
  --accumulate        link only: union the obligation's earlier hunks and
                      claims with the new ones. Without it a link replaces
                      only the claims on its own --test-path and keeps
                      those on every other test path.

report writes by default to .git/corvint/ocm-review.md, private Git metadata
that never dirties the worktree. An explicit --output must be a normalized,
repository-relative path inside the repository root that names no .git
segment; an absolute path such as /tmp/report.md is refused with
unsupported-ocm-output-path. A report written inside the worktree is a visible
untracked file, which blocks clean-worktree admission (for example frozen
selected-check verification) until it is moved or removed.

The bound CEM must be a cem/1.0 map; a retired cem/0.x map refuses with
unsupported-spec and its recovery line.

prepare, link, and mark publish one source-body-free 0600 local map atomically.
link verifies the OCM and bound CEM before mutation. status and verify are
read-only. report publishes one source-body-free 0600 local report atomically
only after a passing verification and policy verdict.
`

const queryHelpFormat = `Compile experimental repository context or the authority-start receipt.

Usage:
  corvint [--root PATH] query --task TASK [--limit N] [--budget-bytes N]

Parity-qualified prompt (exact bytes):
  %q

Options:
  --task TASK  Required UTF-8 task, 1-8000 characters. Project-operations tasks
               use the authority-start profile: the instruction result, then up
               to three advisory learned paths when the limit leaves room.
               Repository and agent-tooling tasks use the same
               BuildEval -> EvalQuery path as harness user-prompt.
  --limit N    Maximum results (default: 10; range: 1-50).
  --budget-bytes N  Optional packet budget (range: 1024-1000000 bytes).
  --worktree-overlay  Experimental opt-in (DIRTY-CACHE-940..942): read tracked
               dirty paths from the worktree over the HEAD snapshot. Their
               results are advisory, low confidence and cite the worktree blob
               id; freshness scope is git+worktree-overlay. Writes nothing.

Requirements:
  Darwin or Linux and a Git repository accepted by the bounded Go index. The
  authority-start profile additionally requires one uniquely highest-ranked
  regular root AGENTS.md encoded as UTF-8 with LF line endings, and either no
  local trace directory or a mixed worktree, where trace learning is blocked.
  Repository tasks rank the full eval evidence tables and preserve explicit
  budget, freshness, omission, and uncertainty fields.

The command is read-only and local-only. It emits one canonical JSON receipt on
stdout. Unsupported profiles emit one structured error on stderr and exit 2.

Receipt fields:
  revision and freshness.revision are the Git tree id of the captured
  content (HEAD^{tree} for a clean worktree), not a commit id, so they never
  equal learning.history_tip, the commit Git history was read from; the two
  differing is not staleness (GPK-V0-960). Bind evidence by comparing revision
  with a commit's tree (git rev-parse COMMIT^{tree}), then each blob.

Recovery for unsupported-query-trace-state:
  Keep the trace store and original task unchanged. Read the repository's
  governing instructions and its declared evidence routes. For separate file
  discovery, use the same task with:
    corvint [--root PATH] context --task TASK --limit 5
  That experimental packet does not turn the failed authority-start query into
  a success or establish complete evidence. Retain its omissions and uncertainty.
  Never delete traces, dirty the worktree, or rewrite the task to evade admission.
`

const impactHelp = `Compile experimental impact evidence for index-admitted paths.

Usage:
  corvint [--root PATH] impact [--limit N] [--provider FILE]...
    [--provider-command ARGV_JSON]... [--provider-mcp ARGV_JSON]... [--repository ID=DIR]...
    [--language-profile non-go-syntax-v0] [--require-clean] [--producer] PATH...
  corvint [--root PATH] impact --capabilities
  corvint [--root PATH] impact --working-tree-untracked [--limit N] PATH...
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
  The non-go-syntax-v0 profile always reports dynamic-dispatch unknowns;
  unsupported constructs remain unknown even without a lexical dynamic token.

Path receipts over MIXED roots warn with the bound commit, dirty count and up
to 8 dirty paths when requested paths are not all dirty. Mutable content remains excluded. Select
--language-profile non-go-syntax-v0 for versioned structured non-Go unknowns;
a request naming a Ruby path selects it by default.
Non-Go range receipts and MCP paths name that experimental profile automatically.
The path selector cannot be combined with --base or --working-tree-untracked.
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
                             Unavailable with --base or --working-tree-untracked.
  --working-tree-untracked   Explicitly select the separate revision-absent
                             nested-.go evidence profile. It accepts no value;
                             untracked-path impact is implemented for .go files only,
                             and that suffix refusal is checked before repository eligibility.
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
                             admitted without one, the per-call limits, derived
                             from the admission tables, and the reverse-import
                             closure depth. Takes no other argument.
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

The default profile is unchanged and uses only revision-tracked evidence. The
explicit working-tree profile accepts only captured mixed-status paths absent
from that revision; binds contained, single-link regular-file identity and
stable twice-read bytes; caps target files at 1,000,000 bytes and their aggregate
at 64,000,000 bytes; parses Go package/import context; and fails closed on path,
file, status, revision, or read races. Its bytes are observed, not immutable Git
authority. All profiles are read-only, local-only, and emit canonical JSON.
Mixed-worktree freshness is disclosed where applicable.
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

const harnessEventHelp = `Compile one experimental agent-harness lifecycle event.

Usage:
  corvint [--root PATH] harness event --host HOST --host-version VERSION
    --surface SURFACE --adapter-version VERSION --event EVENT --input -
    [--budget-bytes N]

Required options:
  --host HOST               claude-code, codex, gemini-cli, or opencode
  --host-version VERSION    Harness-reported host version token
  --surface SURFACE         Harness surface token
  --adapter-version VERSION Adapter version token
  --event EVENT             file-change, post-tool, session-end, session-start,
                            stop, or user-prompt
  --input -                 Read exactly one bounded JSON object from stdin

Optional:
  --budget-bytes N  Output budget (default: 8000; range: 4096-1000000 bytes).

Input is limited to 131072 bytes. user-prompt, file-change, and compact
session-start additionally build a repository index to compile their context
block; user-prompt shares query's BuildEval -> EvalQuery repository path. The
remaining events do not. The command is read-only apart from that one
bounded local self-observation row, and local-only, and
emits one canonical corvint-harness-event/0 FALLBACK receipt on stdout.
` + cpuProfileHelpNote

const localCompletionHelp = `Coordinate an explicitly enrolled local change workflow.

Usage:
  corvint [--root PATH] dogfood begin --plan FILE [--session-key HASH]
  corvint [--root PATH] dogfood status [--session-key HASH]
  corvint [--root PATH] dogfood verify --check ID [--session-key HASH]
  corvint [--root PATH] dogfood finish [--session-key HASH]
  corvint [--root PATH] dogfood review --report-set DIGEST [--session-key HASH]
  corvint [--root PATH] dogfood cancel [--reason TEXT] [--session-key HASH]
  corvint [--root PATH] dogfood hold --reason TEXT [--session-key HASH]
  corvint [--root PATH] dogfood hold --release-authorization TEXT [--session-key HASH]
  corvint [--root PATH] dogfood export --output FILE [--session-key HASH]
  corvint [--root PATH] dogfood import --input FILE [--session-key HASH]
  corvint [--root PATH] dogfood handoff [--anchors "ANCHOR..."] [--session-key HASH]
  corvint [--root PATH] dogfood handoff --receipt FILE [--session-key HASH]
  corvint [--root PATH] dogfood handoff --task-state FILE [--session-key HASH]
  corvint [--root PATH] dogfood handoff --bundle FILE --session-key HASH
  corvint [--root PATH] dogfood change [--corvint-bin PATH] BASE
  corvint [--root PATH] dogfood check [--base-verifier PATH] [--tree-verifier PATH]
    [--override-verifier PATH] BASE
  corvint [--root PATH] dogfood seal [--base-verifier PATH] [--tree-verifier PATH]
    [--override-verifier PATH] BASE
  corvint [--root PATH] dogfood finish-lane --base BASE --citations FILE
    [--intents FILE] [--verify FILE] [--outcome VALUE] [--corvint-bin PATH]
    [--base-verifier PATH] [--tree-verifier PATH] [--override-verifier PATH]
  corvint [--root PATH] dogfood inputs-check --base BASE --citations FILE
    [--intents FILE] [--verify FILE] [--outcome VALUE] [--corvint-bin PATH]
  corvint [--root PATH] dogfood batch-check [--verifier PATH] BASE
  corvint [--root PATH] dogfood propose-citations [--refresh PLAN] BASE
  corvint [--root PATH] dogfood prechange [--task TEXT] BASE [PATH...]

A plan freezes base, intent paths and selected checks with id, argv, timeoutSeconds
and optional allowCemSidecarOnlyReuse (false by default). Session keys are 64 hex
characters; by default the host session environment is hashed. Only one session
may own an active workflow in a worktree.

verify executes selected argv and retains actual observations. finish validates
bound CEM/OCM evidence and produces reports to inspect. review acknowledges that
exact report set. A subsequent finish materializes the selected-check outcome,
runs the strict dogfood check and records satisfaction only on success.

Status is read-only. Missing evidence remains incomplete. Cancellation is explicit
non-success. Local satisfaction is caller-owned workflow evidence, never execution
attestation or Frontier closure. Native event is a separate bounded adapter profile.

hold records a coordinator's preservation hold on the active enrollment, under
which cancel refuses enrollment-preservation-hold; begin the new base in a
distinct checkout instead, or release it with --release-authorization TEXT, which
is kept as a record (LCP-V0-880, LCP-V0-881).

export writes the enrollment, its session files and the artifacts it names to a
new FILE bound to HEAD's commit and tree, changing nothing; keep FILE outside the
Git directory before a worktree is archived. import restores it into a worktree
at that exact commit and tree with no state for the key, relocating its saved
paths; it never replaces an enrollment or invents satisfaction (LCP-V0-883..885).

handoff is read-only. Without --receipt it emits a receipt naming the session key,
root, bound revision, sorted task anchors, prompt packet sha256 and bytes, and
degradations. With --receipt FILE (that emitted document) it re-resolves the same
packet and prints its exact bytes as packetBase64, or exits 1 reporting the exact
root, revision, enrollment, anchor and packet drift and withholds the recompiled
packet. The receipt grants no authority. --task-state FILE emits a bounded bundle
carrying inert caller scope, decisions, review feedback, unknowns and next actions.
--bundle FILE verifies that bundle using the explicitly selected original session
key and withholds both packet and task state on drift; it never executes next actions.

change, check and seal run the daily path of docs/DOGFOOD.md in the current Git
repository, which needs no Corvint script, VERSION file or source tree. change
writes the private evidence under the Git directory and .corvint/change.cem.json
and .corvint/dogfood-report.json; check verifies the committed binding; seal
checks, then commits the CEM rename to .corvint/changes/COMMIT.cem.json. Steps and
verifiers run as this executable unless an option names another one; this binary
never builds or version-checks one. DOGFOOD_TASK, DOGFOOD_VERIFY,
DOGFOOD_VERIFY_FILE, DOGFOOD_OUTCOME, DOGFOOD_INTENTS_FILE, DOGFOOD_CITATIONS,
DOGFOOD_OCM_LINKS and DOGFOOD_EXCEPTION keep their script meanings.
DOGFOOD_CITATION_REPLACE=HUNK_ID=DIGEST[,...] lets the plan replace the earlier
citations of each named, already cited hunk, and only those; the digest is the one
the citation-plan-recites-cited-hunk fix line prints, so a stale one cites
nothing (DCW-V0-390..392). For check and
seal, DOGFOOD_OUTPUT_CHANGE=REQUIREMENT-ID declares a change whose cem status output
differs from the base verifier's by construction: admitted only when HEAD's
docs/specs/REQUIREMENTS.tsv maps the ID to a spec BASE..HEAD modifies, it runs the
tree verifier in both roles and records base-independence=NOT_PROVED with both
stdout digests, or base=NOT_PRODUCED and its refusal code when the base verifier
refuses the read. The root, the current directory unless --root names one, must be
the worktree top level.

finish-lane runs the bind loop on a committed change: prepare, bind with the
--citations plan, commit .corvint/change.cem.json as "chore: bind change evidence",
rebind, check and seal, one stdout line per step, then every NOT_PRODUCED row and
the check's SUMMARY and verdict. It stops at the first failing step with that
step's own reason and fix line; rerun from a bind commit continues at rebind.
Prepare also cites each plan row against the prepared map without writing it,
so a row cem cite would refuse stops the run before any bind (DCW-V0-335), and
so does an intents manifest or a plan the bind would refuse before any cite:
invalid, past the map's hunks, or citing one span for every hunk (DCW-V0-379). It
never pushes. --citations, --intents, --verify and --outcome take precedence over
DOGFOOD_CITATIONS, DOGFOOD_INTENTS_FILE, DOGFOOD_VERIFY_FILE and DOGFOOD_OUTCOME
(DCW-V0-060..062).

inputs-check runs only finish-lane's input checks and prepare pass, naming the
offending input file and row or line, then removes the prepared map: it binds,
commits and seals nothing. make dogfood-closeout runs it before any bind work
(DCW-V0-380); a finish-lane run with the same base, HEAD, inputs and binary
reuses its prepare pass instead of preparing again (DCW-V0-887).

batch-check is read-only. It checks that every commit in BASE..HEAD belongs to a
sealed change the range adds, verifies each sealed CEM against its own base, and
re-derives each merge result; it reports what it cannot prove (DCW-V0-039).

propose-citations is read-only. It prints, on stdout, a draft citation plan with
one row per hunk of .corvint/change.cem.json still owed a row: a BASE line of a
file the change leaves untouched that names the hunk's path, or an unknown row; it
never cites a file the change adds or modifies (DCW-V0-393), and an unknown row for
a hunk of a modified file names its nearest stable line as a review hint only
(DCW-V0-439). JSON, .agent-evidence/ and one-line files are never drafted, and a
hunk naming a requirement the same change adds stays unknown with that ID, since
same-change intent is not BASE evidence (DCW-V0-440). With no usable map
(missing, unreadable or for another BASE) it drafts one row per hunk of the
canonical BASE..HEAD patch, keyed by full hunk ID, so no probe bind pass is needed
(DCW-V0-350, DCW-V0-394). The draft is labelled PROPOSED on stderr; review it and write
it to the DOGFOOD_CITATIONS file yourself (DCW-V0-150).

--refresh PLAN carries an earlier plan forward after a later edit, amend or
renumber re-hunked it (DCW-V0-442): rows whose hunk ID is current are kept as
written, a row whose hunk was re-hunked is re-keyed to the hunk with the same path
and content (else the same path and BASE range), and every re-keyed or dropped row
is noted on stderr, a re-keyed unknown row's NOTE marked stale for review
(DCW-V0-884); only the remaining hunks are drafted afresh. Without a
citation-plan binding, ordinal rows resolve through the one sealed CEM of
BASE..HEAD whose hunks hold them, so a reviewed plan carries across a reseal
(DCW-V0-885). The bind refuses a
stale hunk-ID row, naming the row, its old ID and the current hunk (DCW-V0-441).
For example, run corvint dogfood propose-citations --refresh plan.tsv BASE > plan.new.

prechange records the agent pre-change receipts before the first edit, while
HEAD's tree is BASE's and no tracked file is modified (DCW-V0-880): context
--task into prechange-context.json (orientation only; it satisfies no slot),
query --task into prechange-query.json and, with PATH, impact PATH... into
prechange-impact.json, all under the Git directory's corvint/, then
prechange-binding.json naming BASE, its tree, the Git directory, the task and
each receipt's digest. --task defaults to DOGFOOD_TASK. It refuses
prechange-after-change once the change began, and prechange-receipt-exists
rather than overwrite a receipt it did not write. Each later dogfood change
notes the bound receipts as OBSERVED, or why one is not (DCW-V0-881).
For example, run corvint dogfood prechange --task "Fix the parser" BASE internal/parse/parse.go.
`

// rootPreambleValue reports whether arguments[index] may be consumed as the value of a bare `--root`
// in the `[--root PATH]` preamble (GPK-V0-064, decision 0181): it must exist and must not be
// option-like under argparseOptionLike. A literal `--` keeps its prior treatment as a value.
func rootPreambleValue(arguments []string, index int) bool {
	return index < len(arguments) && (arguments[index] == "--" || !argparseOptionLike(arguments[index]))
}

const repositoryGuidanceHelp = `Experimental inferred repository guidance (read-only, clean HEAD only;
features and overview degrade to a MIXED unknown when the only dirt is untracked).

Usage:
  corvint [--root PATH] features
  corvint [--root PATH] overview
  corvint [--root PATH] review --base FULL_COMMIT_ID [--max-refs N]

Features are bounded heuristic candidates with immutable blob/line evidence, not
accepted feature IDs. A label that is a canonical feature ID also carries featureId
and featureCall, the inert argv of corvint feature for it with --limit 1, which answers READY only
for a declared feature record. Overview includes inert suggested argv. Review composes
advisory affected advice and local refs/heads overlap hints; max-refs is 1..32.
review.affected is the compact affected-plan/1 document; review.affectedFull
names the argv that prints the full affected-plan/0 plan and that plan's digest.
Missing sources, index data and bounded branch paths remain unknown. No scripts,
tests or suggestions execute. No index, trace or observation ledger is written.
CEM, OCM, frontier and mandatory test obligations remain open.
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

const stepHelp = `Observe experimental authoring step scope locally.

Usage:
  corvint [--root PATH] step snapshot --declaration FILE --host FILE
  corvint [--root PATH] step verify --declaration FILE --host FILE --before FILE
  corvint [--root PATH] step env-check --declaration FILE --host FILE

All input files are bounded regular single-link files outside author/read-only
checkouts and Git authority. A host-owned complete before-state is required for
verify; actual post-state is recaptured. Git is externally pinned by path/hash.
Outputs bind commit/tree, dirty/index/admin state and literal scope findings.
Guards override writes. Optional environment observations contain key/classes
only. No values are read. Exit 0 is PASS, 1 is observed violation, 2 is unavailable,
unsupported or malformed evidence. An incomplete post-state retains completed
component findings with no whole post-state digest. Oversized records refuse.

Native observation supports Darwin/Linux descriptors; qualification is per OS.
This command writes only stdout and fixed errors, never repository/index/trace
state. Host assertions remain unauthenticated. Receipts do not prove confinement,
concurrency, transient writes, ACL/xattr/timestamps or credential isolation, and
never stop a job or authorize execution. Proposed contract: ASS-V0.
`

const deltaHelp = `Experimental immutable change record (corvint-delta/0).

Usage:
  corvint [--root PATH] delta --base FULL_SHA --head FULL_SHA
    [--previous-generation FILE] [--provider FILE] [--checkout ID=PATH]
    [--work-key-pattern PATTERN]

Base and head must be full lowercase commit OIDs. Provider and checkout may repeat;
provider inputs are bounded local records, with no command or network redispatch.
A previous generation is a repository/base-bound docs flows manifest. Work keys
are opaque commit-metadata identifiers; commit prose and source content are excluded.

Prints deterministic canonical JSON without state, object, ref or index writes.
Incomplete coverage retains uncertainty and runFullSuite. Lexical documentation
and assertion denominators do not establish runtime coverage. This profile remains
proposed and experimental pending acceptance and integration evidence.
`
