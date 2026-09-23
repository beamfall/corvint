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
	"anchor": true, "provenance": true, "export": true,
}

// helpSubcommands are the retired oracle's nested argparse choices (GPK-V0-062): a token after the
// command that is neither one of them nor --help is an invalid choice argparse reports before help.
var helpSubcommands = map[string]map[string]bool{
	"cem":     cemHelpActions,
	"ocm":     {"prepare": true, "link": true, "mark": true, "status": true, "verify": true, "report": true},
	"harness": {"event": true},
}

// helpBooleanFlags are the public command options that take no value, so a --help after one is
// still help. Every other option is scanned as taking one value (GPK-V0-062).
var helpBooleanFlags = map[string]bool{
	"--apply": true, "--attest": true, "--attest-cem": true, "--attest-cem-v1": true, "--dry-run": true, "--enable": true,
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
	if len(rest) == 2 && rest[0] == "help" {
		directTopic, _ = publicHelpTopic(rest[1])
	}
	switch {
	case len(rest) > 0 && (rest[0] == "--help" || rest[0] == "-h"), len(rest) == 1 && rest[0] == "help":
		return "root", true, nil
	case directTopic != "":
		return directTopic, true, nil
	case len(rest) == 3 && rest[0] == "help" && rest[1] == "harness" && rest[2] == "event":
		return "harness-event", true, nil
	case len(rest) > 0 && rest[0] == "help":
		return "", true, argumentError("unknown help topic")
	}
	topic := commandHelpTopic(rest)
	if topic == "" || !helpFlagRequested(rest[1:]) {
		return "", false, nil
	}
	return topic, true, nil
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
	case len(rest) > 1 && rest[0] == "docs" && rest[1] == "maintain":
		return "docs-maintain"
	case nested && len(rest) > 1 && rest[1] != "--help" && rest[1] != "-h" && !choices[rest[1]]:
		return ""
	}
	return topic
}

// helpFlagRequested applies argparse's left-to-right scan (GPK-V0-062, GPK-V0-064): --help or its
// -h alias before "--" is a help request unless an option that takes a value reaches it first,
// which argparse refuses with "expected one argument" and so never prints help.
func helpFlagRequested(tokens []string) bool {
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
			if command == "harness" {
				return "harness-event", true
			}
			return command, true
		}
	}
	return "", false
}

func helpText(topic string) string {
	switch topic {
	case "docs":
		return docsHelp + "\n" + docsMaintainHelp
	case "docs-maintain":
		return docsMaintainHelp
	case "root":
		return rootHelp
	case "query":
		return fmt.Sprintf(queryHelpFormat, authorityStartPrompt)
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
	default:
		panic("unknown help topic")
	}
}

const rootHelp = `Corvint extraction alpha

Usage:
  corvint [--root PATH] (init | adopt) [--authority-id ID] [--revision REV]
    [--exclude-prefix PATH] [--full-receipt]
  corvint [--root PATH] query --task TASK [--limit N] [--budget-bytes N]
  corvint [--root PATH] feature FEATURE_ID [--limit N] [--budget-bytes N]
  corvint [--root PATH] eval [--goldens FILE] [--trace-fixture FILE]
  corvint [--root PATH] context --task TEXT [--subject PATH] [--limit N]
  corvint [--root PATH] impact [--limit N] [--working-tree-untracked] PATH...
  corvint [--root PATH] impact --base FULL_COMMIT_ID [--limit N]
    [--range-profile expanded-256]
  corvint [--root PATH] harness event OPTIONS
  corvint adapter COMMAND [OPTIONS]
  corvint [--root PATH] dogfood COMMAND OPTIONS
  corvint [--root PATH] dogfood-ocm status --expected-base REV --target REV
  corvint [--root PATH] cem ACTION OPTIONS
  corvint [--root PATH] ocm (status | verify | report) OPTIONS
  corvint [--root PATH] lrf --cem MAP [OPTIONS]
  corvint [--root PATH] frontier --cem MAP --ocm MAP --expected-base REV
    --target REV [OPTIONS]
  corvint [--root PATH] record --task TASK [--opened PATH] --changed PATH
    --verify COMMAND --outcome (passed | failed | blocked)
  corvint [--root PATH] migrate-traces (--dry-run | --apply) [--plan-digest SHA256]
  corvint migration-ratchet --profile FILE
  corvint [--root PATH] observations [--limit N]
  corvint [--root PATH] index [--if-stale]
  corvint [--root PATH] features | overview
  corvint [--root PATH] review --base FULL_COMMIT_ID [--max-refs N]
  corvint [--root PATH] affected
  corvint obligations --cem FILE --impact FILE [--limit N]
  corvint [--root PATH] flows --manifest FILE [--evidence FILE]
  corvint [--root PATH] docs (draft | consume) --source PATH --package DIRECTORY [--task TEXT]
  corvint [--root PATH] batch < REQUEST
  corvint [--root PATH] work observe
  corvint [--root PATH] work propose-wave --envelope PATH --limit N
  corvint [--root PATH] work init --repository NAME --corvint-executable ABSOLUTE_FILE
  corvint [--root PATH] work rebind --corvint-executable ABSOLUTE_FILE
  corvint [--root PATH] work adapter snapshot|details|verify
  corvint [--root PATH] prove --task TEXT [--limit N] [--budget-bytes N]
  corvint [--root PATH] prove [--limit N] [--mutate] PATH...
  corvint [--root PATH] prove --base FULL_COMMIT_ID [--limit N] [--mutate]
  corvint [--root PATH] prove --cem MAP [--expected-base REV] [--target REV] [--patch FILE]
    [--attest | --attest-key PEM] [--attest-cem | --attest-cem-v1]
  corvint [--root PATH] prove --verify-cem-attestation ENVELOPE --attest-public-key PEM
    [--cem MAP]
  corvint [--root PATH] prove --checkpoint FILE
  corvint [--root PATH] prove --export-bundle (--task TEXT [...] | --checkpoint FILE)
  corvint [--root PATH] prove --replay-bundle FILE
  corvint [--root PATH] prove-observe < PROOF
  corvint [--root PATH] witness --base REV [--head REV] [--cem MAP] [--json]
  corvint test-validity [--receipt FILE]
  corvint [--root PATH] COMMAND --help
  corvint help [init|adopt|query|feature|eval|impact|cem|ocm|lrf|frontier|record|migrate-traces|migration-ratchet|observations|affected|obligations|features|overview|review|prove|context|index|batch|docs|depsource|necessity|surprise|answerability|kernel|lease|reads|calibrate|skill-export|dogfood|work|prove-observe|adapter|dogfood-ocm|witness|test-validity|flows]
  corvint help harness [event]
  corvint --version

Commands:
  init           Compile the first mechanical repository inventory.
  adopt          Compile the recoverable mechanical repository inventory.
  query          Compile repository context or the narrow authority-start receipt.
  feature        Compile bounded known or unknown feature context.
  eval           Evaluate a frozen corpus, optionally with an explicit trace-fixture arm.
  context        Compile the task-context packet: the files to read for one task.
  impact         Compile path, untracked-file, or committed-range impact evidence.
  harness event  Compile one supported agent-harness lifecycle event.
  adapter        Run one bounded native host adapter.
  dogfood        Enroll and complete an explicit local evidence workflow.
  dogfood-ocm    Verify the private ordered set of dogfood OCM maps.
  cem            Change Evidence Map producer/verifier: begin, prepare, cite,
                 mark, status, verify, report, and export.
  ocm            Obligation Closure Map producer/verifier: prepare, link, mark,
                 status, verify, and report.
  lrf            Verify repository authority and apply the lexical relevance floor.
  frontier       Compile the deterministic Change Frontier V0 review queue.
  record         Record one explicit local task outcome.
  migrate-traces Plan or apply legacy tree-trace migration.
  migration-ratchet Compare immutable migration-evidence snapshots.
  observations   Read the local self-observation digest; never writes.
  features       Discover experimental inferred feature candidates.
  overview       Compose experimental immutable repository guidance.
  review         Compose experimental range advice and local branch overlaps.
  affected       Compile the affected-test selection plan for the dirty worktree;
                 runs no test and never writes.
  flows          Report experimental application flows, assertion candidates and
                 observed outcomes; record explicitly with the record subcommand.
  obligations    Compose the external-obligations sidecar that joins a CEM's
                 hunks to an impact receipt's external section; never writes.
  prove          Compile the query packet and attach a falsifier verdict to
                 every evidence row; never writes.
  prove-observe  Record one prove document's verdict counts in the local
                 self-observation ledger; the only thing it writes.
  index          Write the committed tree's index snapshot under .corvint/index/
                 for context to read; the only verb that writes there.
  batch          Answer several independent query, context, and impact requests
                 from one loaded index snapshot; never writes.
  work           Validate a repository queue observation or compile a non-operative shadow wave.
  docs           Compile or maintain the source-derived documentation set.
  depsource      Cite pinned third-party Go module source, or abstain; never
                 writes and never fetches. Experimental.
  necessity      Label every task-context packet file necessary, supporting, or
                 unjustified; never writes. Experimental.
  surprise       Report how far the touched set departed from the delivered
                 packet; never writes. Experimental.
  answerability  Report whether the lexical and structural retrieval channels
                 agree; never writes. Experimental.
  kernel         Print the bounded compaction kernel, or verify one from stdin;
                 never writes. Experimental.
  lease          Hold, list, or release a local scope lease over declared paths.
                 Experimental.
  reads          Read the opt-in unplanned-read digest; never writes.
                 Experimental.
  calibrate      Compare recorded packet stances with recorded outcomes; never
                 writes and never applies a threshold. Experimental.
  skill-export   Export admitted learned traces as SKILL.md documents into an
                 operator-named directory; never writes repository or trace state.
                 Experimental.
  witness        Compile the unwitnessed surface of one committed range without
                 mutating repository or trace state.
  test-validity  Project a live-test provider receipt through the shared
                 five-axis test-validity shape; never writes. Experimental.

Global options:
  --root PATH  Repository root (default: current directory). An explicit root is
               made absolute, expands ~ or ~/, and resolves symlinks when
               possible. Query validates all query options before resolving symlinks
               or requiring .git; init, adopt, and impact require .git up front;
               cem validates the repository at its frozen precedence stage.
  --version    Print the Corvint version and build number.
  --help       Print this help on stdout and exit 0.

` + commandMaturityHelp + `Support boundary:
  This binary is the only Corvint runtime and implements only the slices listed
  above; commands marked Experimental carry no stability promise. init, adopt, query, feature, and impact are qualified only on Darwin and
  Linux. All commands are local-only and make no network or telemetry request.

  init, adopt, query, impact, docs, harness, and lrf read without mutating repository or trace
  state. impact, feature, docs, harness event, ocm, lrf, cem, dogfood-ocm, witness,
  index, calibrate, frontier, and record best-effort append a bounded row to the local
  self-observation ledger (.corvint/self-observations.jsonl) on an unsupported-* failure, and
  harness event does so again on a successful session-start, user-prompt, or file-change
  event; a ledger storage failure never alters the response. ocm status and verify are
  otherwise read-only. record writes one atomic local trace row, and migrate-traces --dry-run
  also reads without mutating; --apply rewrites the local trace store. cem begin, prepare, cite,
  and mark write local CEM artifacts (the map and the patch cache), and cem
  report writes the local review report. ocm prepare, link, and mark write local OCM maps, and ocm
  report writes the local OCM review report. cem export writes only the new receipt-bundle
  directory it is given outside the worktree; no other files are touched.
`

const adapterHelp = `Run one bounded native host adapter.

Usage:
  corvint adapter COMMAND [OPTIONS]

Reads one bounded JSON object from stdin and writes one bounded adapter response.
Supported commands are codex, claude-code EVENT, claude-source-handoff, and
source-view. Adapter responses are local-only and carry no repository authority.
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
  corvint [--root PATH] witness --base REV [--head REV] [--cem MAP] [--json]

Builds the committed index, compares the range beginning at --base, and prints
the conservative unwitnessed report. It does not mutate repository or trace
state and never invents a witness.
`

const testValidityHelp = `Project test-level results through the shared test-validity shape.

Usage:
  corvint [--root PATH] test-validity [--receipt FILE | --discover]

FILE is one corvint-js-test-provider stdout document (at most 4 MiB). The command
recomputes, from its receipt member alone, one corvint-test-validity/0 document:
one projection per test and one run projection, each with five independent
axes (association, hygiene, freshness, execution, strength). The projections
the document already carries are never trusted. No axis is a pass/fail summary,
and a PASSED execution axis states no strength: strength stays NOT_MEASURED
until a mutation run supplies a witness.

Without --receipt there is no test-level evidence: tests is empty and every
run axis is UNSUPPORTED with reason no-input-supplied. An unreadable, oversized,
malformed or receipt-less FILE exits 2 with no stdout.

--discover instead reads the newest completed provider document retained as a
regular *.json file directly under .corvint/test-evidence of the worktree (at
most 256 entries, 16 read attempts, no symlinks). Its bound digests are
compared with the worktree now: a mismatch makes freshness STALE, an identity
that cannot be recomputed makes it UNKNOWN, and only fully matched digests are
CURRENT. With no usable document every run axis stays UNSUPPORTED. The command
runs no test and never writes. Experimental.
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
nothing. Malformed command input yields state ERROR with MALFORMED_INPUT.

init writes .corvint/work-queue-policy.json, .corvint/worklist.json and the
executable .corvint/work-queue-adapter for the operator to review and commit; it
refuses when any exists or the explicit Corvint executable is not a safe canonical
external file. rebind updates only that adapter after path, SHA-256, version/build
and source identity change; review and commit it. Observation verifies the binding
and uses no ambient PATH search. adapter prints one document for the qualified
committed worklist.
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
  corvint [--root PATH] eval [--goldens FILE] [--trace-fixture FILE]
  corvint [--root PATH] eval --learn-slot-weights [--goldens FILE] [--admit]
  corvint [--root PATH] eval --reset-slot-weights

Without --goldens, use the repository's .corvint/eval.json or
testing/context-retrieval-goldens.json when available. --trace-fixture adds the
explicit learned-trace comparison arm; ordinary persisted trace replay is not
implemented. The command reads without mutating repository or trace state.
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
`

const activationHelp = `Compile one bounded mechanical repository inventory.

Usage:
  corvint [--root PATH] (init | adopt) [--authority-id ID] [--revision REV]
    [--exclude-prefix PATH] [--full-receipt]

--revision defaults to HEAD. --exclude-prefix is repeatable. The default output
is a bounded sealed summary; --full-receipt emits the complete canonical receipt.
Both activations are read-only and local-only.
`

const recordHelp = `Record one explicit local task outcome.

Usage:
  corvint [--root PATH] record --task TASK [--opened PATH] --changed PATH
    --verify COMMAND --outcome (passed | failed | blocked)

--opened, --changed, and --verify are repeatable; --opened defaults to empty.
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
copies, then removes verified legacy candidates. The command is local-only.
`

const migrationRatchetHelp = `Compare two immutable migration-evidence snapshots.

Usage:
  corvint migration-ratchet --profile FILE

The command writes a deterministic corvint-migration-evidence-ratchet-receipt/1 JSON receipt.
Exit 0 is pass, exit 1 is fail or unknown, and exit 2 is refused input.
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
counting retained hour windows with the newest window.
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
`

const obligationsHelp = `Compose the external-frontier-obligations/0 sidecar for one CEM.

Usage:
  corvint obligations --cem FILE --impact FILE [--limit N]

Reads one cem/0.2 document and one saved corvint impact --provider receipt,
joins every CEM hunk to the receipt's context.external rows by path, and writes
one external-frontier-obligations/0 document to stdout. Each hunk row lists
associations of kind entity, obligation, test, or unknown; every non-unknown
association carries its provider evidence reference, EEP path verification,
and provider freshness, and none states that the record justifies the hunk.
The sidecar binds both inputs by SHA-256 of their raw bytes: binding.cem_sha256
equals the frontier's inputs.cemSha256 for the same CEM file, and every hunk id
equals the CEM hunk id the frontier cites. The verb reads no repository,
verifies no CEM, and changes no frontier state or wire. --limit bounds the
associations per hunk (default 64); omitted rows are counted.

Exit status: 0 with the sidecar on stdout; 2 with an error envelope on stderr
for an argument error, an unreadable or oversized input, a non-cem/0.2 document,
or a non-impact receipt.
`

const affectedHelp = `Compile the affected-test selection plan for the dirty worktree.

Usage:
  corvint [--root PATH] affected
  corvint [--root PATH] affected --snapshot RECEIPT [--playwright-config PATH]
  corvint [--root PATH] affected --base FULL_COMMIT_ID
  corvint [--root PATH] affected [--base FULL_COMMIT_ID]
          --playwright-config PATH [--playwright-discovery FILE]
  corvint [--root PATH] affected [--base FULL_COMMIT_ID] --provider RECORD
          [--provider RECORD ...] [--provider-command ARGV_JSON ...]
          [--repository ID=DIR ...] [--selection-profile strict|coverage]

The command reads the Git worktree status, builds the multi-language unit graph
from source text, and writes one affected-plan/0 document to stdout: the
selector's plan (selected units with witnesses, exclusions, unknowns, scope) and
provider.go.packages, the exact import paths an operator may copy into a Go
live-test provider bundle. It runs no test, writes no repository state, and never claims
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

advice reads the repository's own declarations - a Makefile gate target and the
AGENTS.md Verify block - and lists them as mandatory checks, followed by one
advisory go test command over the selected packages and the unknown frontier
that qualifies it. The advice is static: no check is executed, nothing is
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
discovery emits no file commands and one complete-config fallbackArgv. It executes
no config or test and cannot be combined with --provider.

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
    [--attest | --attest-key PEM] [--attest-cem | --attest-cem-v1]
  corvint [--root PATH] prove --verify-cem-attestation ENVELOPE --attest-public-key PEM
    [--cem MAP]

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
row stays NOT_RUN with the reason. Every cited test runs inside a host
sandbox (macOS sandbox-exec, Linux bwrap) that denies network access, signals
to outside processes, and every write except to a scratch directory emptied
before each run and the invocation's build cache; the exported revision is
read-only to the tests, and each run's process group is killed afterwards. A
run that ends without go test's or pytest's own failure report is an error,
never a kill. A host without a sandbox runs no test and the row stays NOT_RUN. The
repository is never written.

--attest (cem mode) emits an in-toto Statement v1 whose predicate is the
document and whose subjects are the map's sha256 and the target commit.
--attest-key PEM implies --attest and signs the statement into a DSSE envelope
with the Ed25519 PKCS#8 private key at PEM; the key is only ever read, and a
missing or malformed key exits 2 with attest-key-unavailable. Without
--mutate the command runs no test; it never writes.

Experimental, not a stable contract: --attest-cem (cem mode) implies --attest
and prints a second line after the unchanged statement or envelope, a CEM
statement (predicate https://corvint-context.dev/attestation/cem/0) naming MAP by
path, sha256, and size, signed with the same key when --attest-key is given;
--attest-cem-v1 prints cem/v1 instead, also binding baseRevision and patchSha256.
--verify-cem-attestation ENVELOPE checks either offline against the Ed25519 PKIX
key PEM: VERIFIED if --cem MAP matches, else NOT_RUN (cem-bytes-not-supplied).
It exits 2 with attest-public-key-unavailable, attest-envelope-unavailable,
attest-verification-failed, attest-cem-mismatch, or map-unavailable.

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
  --cem MAP           Required CEM 0.1 or 0.2 map.
  --ocm MAP           Optional canonical OCM bound to the CEM and target.
  --patch PATCH       CEM 0.1 only: exact out-of-band patch. Omission uses the
                      per-worktree Git-dir default corvint/change.patch.
  --expected-base REV Independent expected base; required for CEM 0.2.
  --target REV        Optional for CEM 0.1 and required for CEM 0.2.

The command reads maps from bounded regular files, verifies CEM, optional OCM,
patch, revision, blob, and span identities through local Git objects, then emits
one canonical lrf/0 document. Normal results exit 0, including rejected or
abstained edges; aggregate bound results exit 1. Structural, context, I/O, or
interruption failures emit no LRF stdout and exit 2. The command is read-only,
local-only, and always uses the frozen public bounds.
`

const frontierHelp = `Compile the deterministic Change Frontier V0 review queue.

Usage:
  corvint [--root PATH] frontier --cem MAP --ocm MAP --expected-base REV
    --target REV [--command FILE --observation FILE --report FILE] [--json]

Options:
  --cem MAP           Required verified cem/0.2 artifact.
  --ocm MAP           Required canonical ocm/0.1-experimental artifact bound to it.
  --expected-base REV Independent expected base revision; never inferred.
  --target REV        Independent target revision; never inferred.
  --command FILE      Canonical test command, for dynamic test mode.
  --observation FILE  Canonical test observation, for dynamic test mode.
  --report FILE       Raw JUnit report, for dynamic test mode.
  --json              Emit the canonical frontier/0 document instead of the
                      human rendering. Both are renderings of one computation.

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
  corvint [--root PATH] cem prepare --base REV --target REV [--patch CACHE] [--replace]
  corvint [--root PATH] cem cite --map MAP --hunk SELECTOR --evidence-path PATH
    (--bytes START:END | --lines START:END) --relation RELATION [--output MAP]
  corvint [--root PATH] cem mark --map MAP --hunk SELECTOR
    --disposition unknown|mechanical --reason REASON [--output MAP]
  corvint [--root PATH] cem status --map MAP [OPTIONS]
  corvint [--root PATH] cem verify --map MAP [OPTIONS]
  corvint [--root PATH] cem report --map MAP [--output REPORT] [OPTIONS]
  corvint [--root PATH] cem cover --map MAP --coverprofile PATH --test-run ID [--output MAP]
  corvint [--root PATH] cem discriminate --map MAP --target REV [--max-hunks N]
    [--max-mutants N] [--wall-time DURATION] [--output MAP]
  corvint [--root PATH] cem anchor --map MAP [--commit REV]
  corvint [--root PATH] cem provenance --commit REV
  corvint [--root PATH] cem export --map MAP --expected-base REV --target REV
    --output DIR [--witness REPORT]

Actions:
  begin    Build a cem/0.1 candidate from exact out-of-band patch bytes.
  prepare  Derive the canonical base-to-target patch and write or resume the
           cem/0.2 candidate at .corvint/change.cem.json. Commit the candidate,
           then run the returned verification action against HEAD.
  cite     Compile a byte or one-based line span of base evidence into the
           normative span and attach it to one hunk with a typed relation.
  mark     Record an explicit unknown or byte-verifiable mechanical disposition.
  status   Local completion check; verify is the equivalent machine/CI surface;
           report renders the optional human view. All three verify identically.
  cover    Record a patch coverage witness on every hunk from one local Go
           coverprofile named by --test-run; upgrades the map to cem/0.3.
  discriminate
           Mutate the map's changed Go hunks (at most --max-hunks hunks and
           --max-mutants mutants each, within --wall-time; defaults 8, 8, 10m)
           against the _test.go files their test claims cite, and record a
           discriminates / survived / not-run witness pinned to --target and
           the test selection digest. Survivors downgrade the report; the run
           never fails the build.
  anchor   (experimental) Write a pointer to the map committed at HEAD as the
           refs/notes/corvint note of REV (default HEAD); refuses a dirty,
           untracked, or uncommitted map and never replaces a different note.
  provenance  (experimental, read-only) Report REV's Corvint anchor, Git AI
           refs/notes/ai note, and Assisted-by/Agent-Logs-Url trailers as
           untrusted repository-history rows; no URL is fetched.
  export   (read-only) Verify the map as cem verify does and require the
           target to commit it, then copy it, the saved --witness JSON report,
           the dogfood report and the full-gate receipt bound to the same base
           and target into the new absolute directory DIR outside every
           worktree, with a manifest of each receipt's sha256 and
           NOT_RUN/NOT_PRODUCED axes. A receipt that is missing or binds
           another revision is listed as absent. Check it offline with
           script/verify-receipt-bundle.sh DIR.

Verification options:
  --expected-base REV  Independent expected base. Required for cem/0.2.
  --target REV         Verification target. Required for cem/0.2.
  --patch PATH         cem/0.1 only: exact out-of-band patch bytes. A cem/0.2
                       invocation supplying --patch fails invalid-arguments.
  --max-unknown N      Policy ceiling on unknown hunks.
  --max-mechanical N   Policy ceiling on mechanical hunks.

cem/0.2 verification requires --expected-base and --target, derives the
canonical whole-repository patch itself excluding exactly
.corvint/change.cem.json, and reports assurance "canonical". cem/0.1 keeps the
historical explicit/default out-of-band patch behavior with its envelope
warnings. All commands are local-only; reports can disclose repository
information and stay local by default.
`

const ocmHelp = `Prepare, close, or verify an Obligation Closure Map and its bound CEM.

Usage:
  corvint [--root PATH] ocm prepare --target REV --intent PATH [OPTIONS]
  corvint [--root PATH] ocm link --map MAP --obligation ID_OR_ORDINAL
    --hunk ID [--hunk ID...] --test-path PATH --claim SELECTOR
    [--claim SELECTOR...] [OPTIONS]
  corvint [--root PATH] ocm mark --map MAP --obligation ID_OR_ORDINAL
    --reason REASON [--output MAP]
  corvint [--root PATH] ocm status --map MAP [OPTIONS]
  corvint [--root PATH] ocm verify --map MAP [OPTIONS]
  corvint [--root PATH] ocm report --map MAP [--output REPORT] [OPTIONS]

Options:
  --cem PATH          Bound CEM map (default: .corvint/change.cem.json).
  --max-unknown N     Optional non-negative unknown-obligation ceiling.
  --expected-base REV Independent expected base; required for CEM 0.2.
  --target REV        Independent target; required for CEM 0.2.

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

Requirements:
  Darwin or Linux and a Git repository accepted by the bounded Go index. The
  authority-start profile additionally requires one uniquely highest-ranked
  regular root AGENTS.md encoded as UTF-8 with LF line endings, and either no
  local trace directory or a mixed worktree, where trace learning is blocked.
  Repository tasks rank the full eval evidence tables and preserve explicit
  budget, freshness, omission, and uncertainty fields.

The command is read-only and local-only. It emits one canonical JSON receipt on
stdout. Unsupported profiles emit one structured error on stderr and exit 2.

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
    [--provider-command ARGV_JSON]... [--provider-mcp ARGV_JSON]... [--repository ID=DIR]... PATH...
  corvint [--root PATH] impact --working-tree-untracked [--limit N] PATH...
  corvint [--root PATH] impact --base FULL_COMMIT_ID [--limit N]
    [--range-profile expanded-256]

Arguments:
  PATH  One or more normalized repository-relative paths whose suffix the
        immutable index admits. By default every path must be tracked at the
        captured revision. For a .go path, the root package included in the
        profile has the module path as its import path. Paths
        admitted but lacking a reverse-import rule emit an explicit coverage
        uncertainty; no language rule is inferred.
        An unadmitted suffix returns unsupported-impact-path-suffix.

Options:
  --limit N                  Maximum results (default: 10; range: 1-50).
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
Go paths and exact old/new admitted-Go hunk spans itself; binds base/head commit
and tree, status, full-delta and hunk digests, and omissions; admits marker and
ADR relations only from target lines added or replaced by the diff; and rejects
rename, copy, delete, type-change, binary, excluded-Go, oversized, or drifting
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

// cpuProfileHelpNote documents the CPUPROFILE env knob (V1-0051): it is
// off by default, operator-set only, and applies to this command and to
// `context` (taskcontext.go, main.go's harness-event dispatch). It writes a
// local pprof file and never widens what the command reads, returns, or
// mutates (AGENTS.md invariant 4).
const cpuProfileHelpNote = `
CPUPROFILE=PATH (operator env var, off by default): writes a pprof CPU
profile for this invocation to PATH. Diagnostic only; it does not change
what is read, returned, or mutated.
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
  corvint [--root PATH] dogfood cancel [--session-key HASH]

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
`

// rootPreambleValue reports whether arguments[index] may be consumed as the value of a bare `--root`
// in the `[--root PATH]` preamble (GPK-V0-064, decision 0181): it must exist and must not be
// option-like under argparseOptionLike. A literal `--` keeps its prior treatment as a value.
func rootPreambleValue(arguments []string, index int) bool {
	return index < len(arguments) && (arguments[index] == "--" || !argparseOptionLike(arguments[index]))
}

const repositoryGuidanceHelp = `Experimental inferred repository guidance (read-only, clean HEAD only).

Usage:
  corvint [--root PATH] features
  corvint [--root PATH] overview
  corvint [--root PATH] review --base FULL_COMMIT_ID [--max-refs N]

Features are bounded heuristic candidates with immutable blob/line evidence, not
accepted feature IDs. Overview includes inert suggested argv. Review composes
advisory affected advice and local refs/heads overlap hints; max-refs is 1..32.
Missing sources, index data and bounded branch paths remain unknown. No scripts,
tests or suggestions execute. No index, trace or observation ledger is written.
CEM, OCM, frontier and mandatory test obligations remain open.
`

// commandMaturityHelp is the root-help section that names the frozen Core verbs and labels every
// other dispatched verb Experimental with its owning spec prefix (CCF-V1-008).
const commandMaturityHelp = `Command maturity:
  Core (decision 0332, contract CCF-V1 in
  docs/specs/core-compatibility-freeze-v1.md); only the modes and profiles that
  contract lists are frozen:
    init, adopt, index, query, context, impact, affected, prove, cem, ocm,
    frontier, dogfood
  Experimental, no stability promise; the owning spec prefix is in parentheses:
    feature (GPK-V0), eval (REC-V0), lrf (LRF-V0), record (LTPM-V0),
    migrate-traces (LTPM-V0), harness (AHI), work (WQO-V0), adapter (AHI),
    dogfood-ocm (OCM-V0), observations (SOL-V0), obligations (EFO-V0),
    prove-observe (SOL-V0), batch (SBQ-V0), docs (SDD-V0),
    depsource (DSE-V0), necessity (NEC-V0), surprise (TSS-V0),
    answerability (RDS-V0), kernel (CKN-V0), lease (SCL-V0), reads (URE-V0),
    calibrate (OCL-V0), witness (AGW-V0), test-validity (MTV-V0),
    features (RGV-V0), overview (RGV-V0), review (RGV-V0),
    migration-ratchet (MER-V0), flows (AFU-V0), skill-export (LTA-V0)

`
