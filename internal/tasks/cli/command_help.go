package cli

import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Exact help requests bypass every parser, store read, stdin read and launcher.
// More elaborate existing mutation-help forms continue through their parser;
// a scalar flag value spelled --help is never intercepted here.
func commandHelp(args []string) *wire.Result {
	if len(args) < 2 || (args[len(args)-1] != "--help" && args[len(args)-1] != "-h") {
		return nil
	}
	cmd := args[:len(args)-1]
	name := strings.Join(cmd, " ")
	if len(cmd) == 2 && cmd[0] == "ticket" {
		if operation, ok := mutationVerbs[cmd[1]]; ok {
			result := mutationHelp(cmd, operation)
			result.Items[0].Obj.Set("flags", wire.Strings(helpFlags(result.Items[0].Obj, "usage")))
			return result
		}
	}
	if name == "--version" {
		name = "version"
	}
	usage, ok := commandUsage[name]
	if !ok {
		// Only an actual inventory prefix is a command family. Unknown paths
		// must retain ordinary errors, even when followed by a help flag.
		children := []string{}
		for _, verb := range append(append([]string{}, ReadVerbs...), OmittedVerbs...) {
			if strings.HasPrefix(verb, name+" ") {
				children = append(children, strings.TrimPrefix(verb, name+" "))
			}
		}
		if len(children) == 0 {
			return nil
		}
		sort.Strings(children)
		usage = "corvint-tasks " + name + " <" + strings.Join(children, "|") + ">"
	}
	o := wire.NewObject().Set("usage", wire.String(usage)).Set("implemented", wire.Bool(!isOmitted(name)))
	o.Set("flags", wire.Strings(helpFlags(o, "usage")))
	if isOmitted(name) {
		o.Set("note", wire.String("Help is available; execution remains NOT_RUN. No execution flags are implemented."))
	}
	if name == "release" {
		o.Set("laneUntouched", wire.String("--lane-untouched --evidence LOCAL_REF records an OWNER/OPERATOR attestation: no physical lane access, no lane commands, no physical capability/resource issued or retained, and responsibility for safe reuse. Requires a fresh direct no-health pooled RUNNING generation, exact current policy/config/acceptance and no tracked use, renewal or native Program association. Physical facts and actor authentication remain NOT_OBSERVED; private Dispatcher is not scanned. Only this eligible explicit profile frees its exact occupancy without configured cleanup; ordinary release/handoff retains quarantine. HANDOFF/REVIEW_RETURNED accounting remains independently required; no retry refund is granted."))
		o.Set("reasonCodes", wire.Strings(wire.Codes))
		o.Set("handoffPreconditions", wire.Strings([]string{
			"HANDOFF requires an external-agent implement, review or integrate stage; REVIEW_RETURNED requires review.",
			"The generation and unexpired lease must match, acceptance must be unchanged, policy must be unchanged or proven compatible, and prospective retry accounting must have no recorded failure or unknown gate.",
			"Without --evidence: submit a candidate first; phase BUILT or CHECKING, scope WITHIN and no pending effects are required.",
			"With --evidence: RUNNING with no candidate, no gate results, scope UNKNOWN and no pending effects; reference uses Identifier grammar (1..128 bytes). Evidence is forbidden for other reasons unless --lane-untouched is supplied, and is forbidden for candidate handoffs.",
			"Relevant or unproved policy changes fence handoffs with STALE_POLICY. Only policyVersion and other members' reservations in the allocated pool may differ across a fully audited interval. Failed/unknown gates remain sticky; no historical refund.",
			"A reference is inert caller evidence, not proof of work or physical cleanup. Release removes the reservation and quarantines an allocated pool; it does not grant completion, review or integration authority.",
			"--handoff-to implement|review|integrate (optional --handoff-reason) records advisory next-stage intent on a clean HANDOFF or REVIEW_RETURNED generation only; REVIEW_RETURNED accepts only implement and defaults to it. Other combinations refuse MALFORMED. Claims for another stage are not refused.",
		}))
		o.Set("handoffTargets", wire.Strings(intent.StageRoles))
		o.Set("handoffReasonCodes", wire.Strings(snapshot.HandoffReasons))
		o.Set("handoffRefusalCodes", wire.Strings([]string{wire.CodeFenced, wire.CodeStaleTicket, wire.CodeStalePolicy, wire.CodeTicketState, wire.CodeMissingEvidence, wire.CodeMalformed}))
	}
	if name == "policy update" {
		o.Set("fileFormat", wire.String("Canonical UTF-8 JSON: sorted object keys, no insignificant whitespace, and exactly one trailing LF."))
		o.Set("versionRule", wire.String("The file policyVersion must equal --expected-policy-version plus one; the flag names the current version."))
	}
	if name == "attempt heartbeat" {
		o.Set("note", wire.String("Generation-fenced recorded signal with a 10-minute observation TTL. Does not renew the work lease or prove process liveness. Use a fresh request ID for each heartbeat; replay never refreshes the timestamp."))
	}
	if name == "pool sweep" {
		o.Set("note", wire.String("Requires explicit timeoutSeconds 1..1800 and OWNER or an explicit OPERATOR policy grant. Replays return the original receipt or PENDING without repeating commands. Private logs can contain command-emitted secrets; explicit archive export includes evidence. FREE records operator-declared reset and verification, not proof of external physical safety."))
	}
	if name == "pool recover" || name == "pool confirm-safe" {
		o.Set("note", wire.String("--reason is free-form prose (1..4096 bytes), not a closed release reason code."))
	}
	if name == "ticket note set" || name == "ticket note clear" {
		o.Set("note", wire.String("Advisory operator prose; never instructions, acceptance or authority. Each note write retains a derived event that receipt audit and redo bind by replaying the transition from audited pre-state (ON-V0-006); claim and claim --next deliver the note pinned by their own admission (ON-V0-007)."))
	}
	if name == "ticket escalate" || name == "ticket answer" {
		o.Set("note", wire.String("The worker's escalation question and the owner's answer are untrusted queue data, never instructions, acceptance or authority. ticket escalate reads its ticket, holder, generation and acceptance revision from the claim receipt; the actor must be that holder. ticket answer without --request resolves only the sole same-acceptance OPEN question at commit and otherwise refuses with the open request IDs."))
	}
	if strings.HasPrefix(name, "ticket escalation ") {
		o.Set("note", wire.String("Pure read: writes no ledger and hydrates no evidence. program is UNKNOWN and retryState NOT_OBSERVED until a producer mapping and dispatcher ledger exist; ageSeconds is clockUncertain when the local clock is behind the recorded time."))
	}
	if name == "critical-path" {
		o.Set("note", wire.String("Pure read, no lock and no writes: the transitive unsatisfied dependency closure of one ticket (a gate ticket is a ticket) as taskman-critical-path/0, longest chain first, bounded to 256 nodes and 32 chains with truncated and totals. Blocker codes are an open set; facts this reader cannot observe are NOT_OBSERVED, and estimate is always NOT_OBSERVED in v0."))
	}
	if name == "archive verify" {
		o.Set("note", wire.String("Reads FILE, or stdin when FILE is absent or -. Help reads neither."))
	}
	if name == "criterion-binding capture" {
		o.Set("note", wire.String("Returns a capture and its verification from the current native ticket and attempt. Offline verify consumes the canonical capture object on stdin, not the result envelope."))
	}
	if name == "criterion-binding verify" {
		o.Set("note", wire.String("Reads only canonical capture bytes from stdin; accepts no execution flags. Verification is offline. Help does not read stdin."))
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
}

// Usage is the single help inventory for each command's supported inputs.
// Omitted verbs deliberately have no invented execution flags.
var commandUsage = map[string]string{
	"criterion-binding capture": "corvint-tasks criterion-binding capture --ticket ID --attempt ID",
	"criterion-binding verify":  "corvint-tasks criterion-binding verify (canonical capture on stdin)",

	"version":           "corvint-tasks version (alias --version)",
	"ticket list":       "corvint-tasks ticket list [--offset N] [--limit N]",
	"ticket search":     "corvint-tasks ticket search [--status S] [--kind K] [--priority P] [--owner L] [--milestone L] [--label L] [--text T] [--offset N] [--limit N]",
	"ticket show":       "corvint-tasks ticket show <ticketId|local>",
	"ticket blockers":   "corvint-tasks ticket blockers <ticketId|local>",
	"ticket export":     "corvint-tasks ticket export [--offset N] [--limit N]",
	"queue status":      "corvint-tasks queue status",
	"roadmap":           "corvint-tasks roadmap [--offset N] [--limit N]",
	"critical-path":     "corvint-tasks critical-path <ticketId|local>",
	"gate list":         "corvint-tasks gate list",
	"gate show":         "corvint-tasks gate show <gateId>",
	"receipt audit":     "corvint-tasks receipt audit",
	"reconcile inspect": "corvint-tasks reconcile inspect <ticketId>",
	"reconcile intent":  "corvint-tasks reconcile intent (--keep-journal | --adopt-file) --request-id ID --target TICKET|RELEASE --file FILE|- [--canonical-sha256 SHA256] [--role OWNER|OPERATOR]",
	"archive export":    "corvint-tasks archive export [--staging DIR]",
	"archive verify":    "corvint-tasks archive verify [FILE|-]",
	"init":              "corvint-tasks init [--role ROLE] [--request-id ID]",
	"pause":             "corvint-tasks pause --request-id ID [--role OWNER|OPERATOR]",
	"unpause":           "corvint-tasks unpause --request-id ID [--role OWNER|OPERATOR]",
	"policy show":       "corvint-tasks policy show",
	"policy update":     "corvint-tasks policy update --request-id ID --expected-policy-version N --file PATH [--role OWNER|OPERATOR]",
	"import":            "corvint-tasks import --file PATH [--role OWNER|OPERATOR]",
	"cutover":           "corvint-tasks cutover [--execution] --decision REF [--qualification FILE]",
	"release list":      "corvint-tasks release list",
	"release show":      "corvint-tasks release show RELEASE",
	"release readiness": "corvint-tasks release readiness RELEASE",
	"claim":             "corvint-tasks claim (<ticketId|local> | --next) --holder LABEL --request-id ID [--lease-minutes N] [--branch LABEL] [--base OID] [--scope PATH...] [--pool ID] [--stage implement|review|integrate] [--exclude-member ID]... [--exclude-authors[=all]] [--timing] [--role ROLE]",
	"renew":             "corvint-tasks renew --attempt ID --generation G --request-id ID [--lease-minutes N] [--timing] [--role ROLE]",
	"release":           "corvint-tasks release --attempt ID --generation G --request-id ID [--reason CODE] [--evidence LOCAL_REF] [--handoff-to STAGE [--handoff-reason CODE]] [--lane-untouched] [--timing] [--role ROLE]; release <create|update|candidate|record-gate|promote|list|show|readiness> --help",
	"reap":              "corvint-tasks reap --request-id ID [--attempt ID --generation G] [--role ROLE]",
	"widen":             "corvint-tasks widen --attempt ID --generation G --request-id ID (--scope PATH... | --whole-repository) [--role ROLE]",
	"attempt heartbeat": "corvint-tasks attempt heartbeat --attempt ID --generation G --request-id ID [--timing] [--role ROLE]",
	"attempt show":      "corvint-tasks attempt show <attemptId>",
	"plan preview":      "corvint-tasks plan preview [--pool ID] [--stage implement|review|integrate] [--exclude-member ID]... [--exclude-authors[=all]] [--selected-only]",
	"submit":            "corvint-tasks submit --attempt ID --generation G --request-id ID --tree OID [--role ROLE]",
	"gate run":          "corvint-tasks gate run --attempt ID --generation G --request-id ID --gate GATE [--worktree DIR] [--role ROLE]",
	"complete":          "corvint-tasks complete --attempt ID --generation G --request-id ID --commit OID [--role ROLE]",
	"health":            "corvint-tasks health --member ID [--stage STAGE] --request-id ID [--role ROLE]",
	"pool sweep":        "corvint-tasks pool sweep --request-id ID --timeout-seconds N [--member ID] [--role OWNER|OPERATOR]",
	"pool cleanup":      "corvint-tasks pool cleanup --member ID --allocation SHA256 --request-id ID [--role ROLE]",
	"pool recover":      "corvint-tasks pool recover --member ID --allocation SHA256 --reason TEXT --request-id ID [--role ROLE]",
	"pool confirm-safe": "corvint-tasks pool confirm-safe --member ID --allocation SHA256 --evidence REF --reason TEXT --request-id ID [--role ROLE]",
	"lane-leader":       "corvint-tasks lane-leader --directory DIR --capsule FILE",
	"pending":           "corvint-tasks pending",
	"program show":      "corvint-tasks program show",
	"dispatch":          "corvint-tasks dispatch --program ID --config FILE [--once | --ticks N]; dispatch <status|unpark> --help",
	"dispatch status":   "corvint-tasks dispatch status --program ID --config FILE [--events N]",
	"dispatch unpark":   "corvint-tasks dispatch unpark --program ID --config FILE --key TICKET|lane:POOL/MEMBER",
	"service install":   "corvint-tasks service install --program ID --config SERVICE_JSON --request-id ID [--replace]",
	"service status":    "corvint-tasks service status --program ID",
	"service uninstall": "corvint-tasks service uninstall --program ID --request-id ID",
	"service stop":      "corvint-tasks service stop --program ID --request-id ID [--drain]",
	"service resume":    "corvint-tasks service resume --program ID --request-id ID",
	"service run":       "corvint-tasks service run --program ID --manifest FILE   (manager-started foreground main; not for interactive use)",
	"config":            "corvint-tasks config (execution NOT_RUN)",
	"plan record":       "corvint-tasks plan record (execution NOT_RUN)",
	"receipt show":      "corvint-tasks receipt show (execution NOT_RUN)",
	"receipt replay":    "corvint-tasks receipt replay (execution NOT_RUN)",
	"archive restore":   "corvint-tasks archive restore (execution NOT_RUN)",
}

func init() {
	for _, verb := range []string{"create", "update", "candidate", "record-gate", "promote"} {
		commandUsage["release "+verb] = "corvint-tasks release " + verb + " --request-id ID --target RELEASE [--expected-revision N] (--payload JSON | --payload-stdin) [--issued-at TS] [--role ROLE]"
	}
	for _, verb := range []string{"run", "admit", "resume", "retry", "drain", "cancel", "answer"} {
		commandUsage[verb] = "corvint-tasks " + verb + " --program ID --config FILE [--role implementer|reviewer|integrator] [--count N] [--ticket ID] [--host codex|claude-code|opencode] [--grant FILE] [--question SHA256] [--revision N] [--answer TEXT]"
	}
	commandUsage["ticket note set"] = "corvint-tasks ticket note set <ticketId|local> --request-id ID (--text TEXT | --text-stdin) [--supersedes N] [--expected-revision N] [--issued-at TS] [--role OWNER|OPERATOR]"
	commandUsage["ticket note clear"] = "corvint-tasks ticket note clear <ticketId|local> --request-id ID [--supersedes N] [--expected-revision N] [--issued-at TS] [--role OWNER|OPERATOR]"
	commandUsage["ticket note show"] = "corvint-tasks ticket note show <ticketId|local>"
	commandUsage["ticket escalate"] = "corvint-tasks ticket escalate --attempt ID --claim-receipt SEQ|RECEIPT --kind decision|infrastructure|scope|blocked --question TEXT [--options A,B] [--supersedes REQUEST --expected-request-revision N] [--blocked-by TICKET [--gate GATE]] [--expected-revision N] --request-id ID [--role ROLE]"
	commandUsage["ticket answer"] = "corvint-tasks ticket answer --target TICKET --text TEXT [--request REQUEST --expected-request-revision N] [--expected-revision N] --request-id ID [--role ROLE]"
	commandUsage["ticket escalation list"] = "corvint-tasks ticket escalation list [--kind KIND] [--state OPEN|ANSWERED|SUPERSEDED] [--target TICKET] [--program NAME] [--limit 1..50] [--cursor ORIGIN_SHA256]"
	commandUsage["ticket escalation show"] = "corvint-tasks ticket escalation show <ticketId|local> <requestId>"
	commandUsage["ticket escalation history"] = "corvint-tasks ticket escalation history <ticketId|local> <requestId> [--limit 1..50] [--cursor EVENT_SHA256]"
	commandUsage["gate record"] = "corvint-tasks gate record <ticketId|local> --gate GATE (--verdict PASS|RETURN | --from-acceptance REPORT) --subject-receipt SEQ --expected-generation N --expected-revision N --request-id ID [--reason CODE:TEXT] [--candidate-evidence SHA256:BYTES] [--evidence LABEL=SHA256:BYTES] [--reviewer-attempt ID] [--issued-at TS] [--role OWNER|OPERATOR|REVIEWER]"
	commandUsage["gate resubmit"] = "corvint-tasks gate resubmit <ticketId|local> --gate GATE --author-attempt ID --subject-receipt SEQ --expected-generation N --expected-revision N --reason CODE:TEXT --request-id ID [--prior-return SHA256] [--candidate-evidence SHA256:BYTES] [--evidence LABEL=SHA256:BYTES] [--issued-at TS] [--role OWNER|OPERATOR|WORKER]"
	commandUsage["gate history"] = "corvint-tasks gate history <ticketId|local> --gate GATE [--cursor SHA256] [--limit N]"
	commandUsage["gate state"] = "corvint-tasks gate state <ticketId|local>"
	commandUsage["run"] += "; corvint-tasks run --attempt ID --generation G --timeout SECONDS [--lease-minutes N] [--role ROLE] -- COMMAND..."
}

func helpFlags(o *wire.Object, key string) []string {
	v, _ := o.Get(key)
	flags := []string{"--help", "-h"}
	for _, word := range strings.Fields(v.Str) {
		word = strings.Trim(word, "[]();")
		if strings.HasPrefix(word, "--") && word != "--help" {
			found := false
			for _, f := range flags {
				if f == word {
					found = true
				}
			}
			if !found {
				flags = append(flags, word)
			}
		}
	}
	sort.Strings(flags)
	return flags
}
