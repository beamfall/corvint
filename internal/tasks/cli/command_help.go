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
//
// A trailing --help or -h returns the terse form (CAL-V0-170): usage,
// implemented, flags and, for mutations, operation and payload keys, plus a
// verboseHelp pointer when anything was left out. `--help --verbose` (either
// order) returns the full help object.
func commandHelp(args []string) *wire.Result {
	isHelp := func(a string) bool { return a == "--help" || a == "-h" }
	n := len(args)
	verbose := n >= 3 && ((args[n-1] == "--verbose" && isHelp(args[n-2])) || (isHelp(args[n-1]) && args[n-2] == "--verbose"))
	cmd := args
	switch {
	case verbose:
		cmd = args[:n-2]
	case n >= 2 && isHelp(args[n-1]):
		cmd = args[:n-1]
	default:
		return nil
	}
	result := fullCommandHelp(cmd)
	if result == nil || verbose {
		return result
	}
	return terseHelp(cmd, result)
}

// terseKeys are the help keys an agent needs to form a valid call.
var terseKeys = []string{"usage", "implemented", "flags", "operation", "payloadKeys", "optionalPayloadKeys"}

func terseHelp(cmd []string, full *wire.Result) *wire.Result {
	o := full.Items[0].Obj
	name := strings.Join(cmd, " ")
	keep := append([]string{}, terseKeys...)
	if isOmitted(name) {
		keep = append(keep, "note")
	}
	terse := pick(full.Items[0], keep...)
	if len(terse.Obj.Keys) < len(o.Keys) {
		terse.Obj.Set("verboseHelp", wire.String("corvint-tasks "+name+" --help --verbose"))
	}
	full.Items[0] = terse
	return full
}

func fullCommandHelp(cmd []string) *wire.Result {
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
			"Relevant or unproved policy changes fence handoffs with STALE_POLICY. Only policyVersion, holderLiveness, other members' reservations in the allocated pool, and pool members added with their own settings may differ across a fully audited interval; `policy update` lists the live handoffs it fences in handoffFences. Failed/unknown gates remain sticky; no historical refund.",
			"A reference is inert caller evidence, not proof of work or physical cleanup. Release removes the reservation and quarantines an allocated pool; it does not grant completion, review or integration authority.",
			"holderStatus STALE_HOLDER (attempt show, queue status) is advisory input for a coordinator choosing an evidence HANDOFF release; it is not release authority. The release keeps every fence above, and no read, heartbeat, renew or reap releases, frees or fences an attempt because its holder is stale.",
			"--handoff-to implement|review|integrate (optional --handoff-reason) records advisory next-stage intent on a clean HANDOFF or REVIEW_RETURNED generation only; REVIEW_RETURNED accepts only implement and defaults to it. Other combinations refuse MALFORMED. Claims for another stage are not refused.",
		}))
		o.Set("handoffTargets", wire.Strings(intent.StageRoles))
		o.Set("handoffReasonCodes", wire.Strings(snapshot.HandoffReasons))
		o.Set("handoffRefusalCodes", wire.Strings([]string{wire.CodeFenced, wire.CodeStaleTicket, wire.CodeStalePolicy, wire.CodeTicketState, wire.CodeMissingEvidence, wire.CodeMalformed}))
	}
	if name == "release" || name == "attempt heartbeat" {
		o.Set("lockWait", wire.String(lockWaitHelp))
	}
	if name == "policy update" {
		o.Set("fileFormat", wire.String("Canonical UTF-8 JSON: sorted object keys, no insignificant whitespace, and exactly one trailing LF."))
		o.Set("versionRule", wire.String("The file policyVersion must equal --expected-policy-version plus one; the flag names the current version."))
	}
	if name == "queue status" || name == "roadmap" {
		o.Set("milestoneCount", wire.String("queue status reports openWithoutMilestone, the count of OPEN tickets with a null milestone; roadmap warns with that count when it is non-zero (CAL-V0-196)."))
	}
	if name == "ticket list" || name == "ticket search" || name == "roadmap" {
		o.Set("facets", wire.String("--facets adds a top-level facets object counting every matched ticket regardless of --offset/--limit: total, byStatus, byPriority, byKind, byExecutionClass, byEligibility (every closed value, zeros included), byNextAction, byBlockerCode (non-terminal tickets, once per code), byMilestone and byLabel (at most 64 most frequent keys, with milestonesOmitted and labelsOmitted), and withoutMilestone. --count returns only that object, with no items and page null, and refuses --offset or --limit (CAL-V0-206, CAL-V0-207)."))
	}
	if name == "release readiness" {
		o.Set("membership", wire.String("The item adds memberCounts (member tickets by status and priority, plus absent member ids) and milestoneDrift: unfinishedNonMembers are DRAFT, OPEN or HELD tickets whose milestone equals the releaseId but are not members, and membersOutsideMilestone are members whose milestone differs; each lists an exact count and at most 100 ids. Membership is never changed. A release without a candidate is BLOCKED on candidate without observing the source tree (CAL-V0-208, CAL-V0-209)."))
	}
	if name == "attempt heartbeat" {
		o.Set("note", wire.String("Generation-fenced recorded signal. Reads classify it against the policy holderLiveness.heartbeatTTLSeconds (300..86400, default 600 when omitted). Does not renew the work lease or prove process liveness. Use a fresh request ID for each heartbeat; replay never refreshes the timestamp."))
	}
	if name == "pool sweep" {
		o.Set("note", wire.String("Requires explicit timeoutSeconds 1..1800 and OWNER or an explicit OPERATOR policy grant. Replays return the original receipt or PENDING without repeating commands. Private logs can contain command-emitted secrets; explicit archive export includes evidence. FREE records operator-declared reset and verification, not proof of external physical safety."))
	}
	if name == "doctor" {
		o.Set("note", wire.String("Advisory pure read, no lock (TQD-V0-001..012): findings NO_PROGRESS_HANDOFF, REPEAT_REFUSAL, SLOW_LANE_RECOVERY, SETUP_ONLY_PROOF and FALSE_IDLE from at most 4096 receipts within 7 days plus a bounded process walk, each with kind, source, who, detail, remedy, firstSeen, ageSeconds and evidenceSeqs. --plugins DIR runs the directory's executables (10 s, 64 KiB stdout, a JSON array of {kind,who,detail[,remedy]}); a failing plugin is a PLUGIN_FAILED finding and the envelope is untrusted. Only --refresh writes, and only <git common dir>/taskman-doctor/summary.json. --line prints one plain-text status line from that cache with no store read, or exits 1 when the cache is unavailable. Findings carry no authority."))
	}
	if name == "pool status" {
		o.Set("note", wire.String("Pure read, no lock, no probe and no writes: each configured member's state, current allocation, holder/attempt/generation, quarantine reason with its journal changedSeq, and the last health or cleanup outcome retained with the allocation. The journal records no wall-clock time, so since and observedAt are NOT_OBSERVED, as is any outcome pool state no longer retains. An unknown --pool or --member refuses MALFORMED."))
	}
	if name == "pool acquire" {
		o.Set("note", wire.String("Allocates one member to the named live external-agent attempt that holds none, for the attempt's own holder and stage, by the pooled-claim rules: priority yield, --exclude-member and --exclude-authors (review or integrate attempts only), health preparation, and a RESOURCE_COLLISION refusal when no eligible member is free. One allocation per generation: an attempt that returned one cannot acquire again. Generation-fenced; a request-id replay returns the original receipt's allocation (CAL-V0-198..200, CAL-V0-204)."))
	}
	if name == "pool release" {
		o.Set("note", wire.String("Returns the attempt's exact current allocation early: the member is quarantined until pool cleanup and confirm-safe, exactly as when an attempt ends, and the attempt stays live with no allocation. A shared allocation is refused. Lane-untouched release is unavailable afterwards (CAL-V0-201, CAL-V0-202)."))
	}
	if name == "pool recover" || name == "pool confirm-safe" {
		o.Set("note", wire.String("--reason is free-form prose (1..4096 bytes), not a closed release reason code."))
	}
	if name == "ticket note set" || name == "ticket note clear" {
		o.Set("note", wire.String("Advisory operator prose; never instructions, acceptance or authority. Each note write retains a derived event that receipt audit and redo bind by replaying the transition from audited pre-state (ON-V0-006); claim and claim --next deliver the note pinned by their own admission (ON-V0-007)."))
	}
	if strings.HasPrefix(name, "ticket know-how ") {
		o.Set("note", wire.String("Know-how notes are untrusted agent-authored data, never instructions, acceptance, evidence or authority, and never feed ranking (KHN-V0-007). add pins each --anchor file to its blob in --commit (default HEAD of the working directory) and each --symbol PATH#NAME to the digest of that one declaration's text as the context index's extractor reads it; a retry passes the same --commit and --issued-at. Text, reason, routes, paths and symbols are secret-screened on write. Freshness is computed at read time against the reader's committed HEAD: STALE when a file anchor's blob or a symbol anchor's declaration changed, UNKNOWN when it cannot be resolved (a deleted, renamed, duplicated or unsupported symbol), never stored. reconfirm re-pins a STALE note's own anchors at --commit with provenance, keeping the prior pins in the ledger; a re-pin that changes nothing is refused KNOWHOW_NOT_STALE and an anchor that no longer resolves KNOWHOW_UNRESOLVED. claim and claim --next deliver intersecting notes under a 2 KiB cap; list is a pure read. --role WORKER add is refused unless policy opts in with workerAdd (KHN-V0-021); then the actor must hold the live attempt named by --attempt and --generation on this ticket, every --anchor must lie inside its effects.touchPaths, and --supersedes is refused (KHN-V0-022). For a program spanning repositories, add --repo ALIAS=ROOT pins anchors in the Git top level ROOT and stores them as ALIAS/PATH, so touchPaths and --path match the qualified path (KHN-V0-024, KHN-V0-026); reconfirm of such a note needs the same alias. list and claim take --repo ALIAS=ROOT (repeatable) to resolve a repository note's freshness at that root's HEAD; an unmapped alias leaves its notes UNKNOWN with a warning, never resolved against the working directory (KHN-V0-027)."))
	}
	if strings.HasPrefix(name, "ticket obligations ") {
		o.Set("note", wire.String("A per-ticket obligation ledger (TOL-V0). seed declares the ledger prefix and OPEN obligations {prefix, obligations:[{id, title, core}]}; set changes state (OPEN, DEFECT, BLOCKED, DEFERRED) or core with a reason {changes:[{id, state, core, reason}]}, and core is OWNER-only. witness credits ids from one Playwright json-reporter document: only retry-0 results of expected-to-pass tests credit, every match of an id must pass, and the spec file at --commit must hold the id as a whole token; the report is never retained, only its digest. The qualified Playwright version list is empty until live qualification, so every report refuses UNSUPPORTED_VERSION. --declared is an OWNER-only vouched witness. A WORKER may witness a report only under a policy obligations.workerWitness opt-in and its own live --attempt and --generation. A witness that credits nothing writes nothing (written false). Writes move the ticket revision only; a ledger never satisfies a gate, criterion or completion in V0. show and plan are reads; plan exits 0 only when every open obligation is assigned to exactly one planned test."))
	}
	if name == "ticket note history" {
		o.Set("note", wire.String("Pure read: newest-first SET and CLEAR events, 20 per page by default (1..50, at most 1 MiB), anchored at the committed head. A truncated page returns an opaque nextCursor bound to its anchor; a missing, cyclic or mismatched event refuses rather than shortening history. Superseded and cleared notes are a record, never current guidance."))
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
	if name == "receipt audit" {
		o.Set("note", wire.String("Always the complete audit from receipt 1, never resumed from a checkpoint; it never reads, creates, replaces or removes the derived checkpoints <state directory>.checkpoint.json (reads) and <state directory>.writer-checkpoint (writers), where the state directory is <git common dir>/taskman. Removing <state directory>.writer-checkpoint forces the next write through the complete audit, which refuses a tampered prefix that a checkpointed writer does not re-read; removing <state directory>.checkpoint.json costs the next read one complete audit. Neither removal changes any journal, intent or archive bytes."))
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

// lockWaitHelp documents the CAL-V0-111 --lock-wait of release and attempt
// heartbeat.
const lockWaitHelp = "--lock-wait SECONDS (whole seconds, 1..300; anything else refuses MALFORMED) waits up to that long for preparation admission and the store lock instead of the default 30 seconds, for this command only. It is not part of the request: after LOCK_TIMEOUT, resubmit the same request ID with the same arguments; that commits once or replays the committed receipt."

// Usage is the single help inventory for each command's supported inputs.
// Omitted verbs deliberately have no invented execution flags.
var commandUsage = map[string]string{
	"criterion-binding capture": "corvint-tasks criterion-binding capture --ticket ID --attempt ID",
	"criterion-binding verify":  "corvint-tasks criterion-binding verify (canonical capture on stdin)",

	"version":            "corvint-tasks version (alias --version)",
	"ticket list":        "corvint-tasks ticket list [--status S[,S...]] [--offset N] [--limit N] [--summary | --fields KEY[.SUB],...] [--facets | --count]",
	"ticket search":      "corvint-tasks ticket search [--status S] [--kind K] [--priority P] [--owner L] [--milestone L] [--label L] [--text T] [--offset N] [--limit N] [--summary | --fields KEY[.SUB],...] [--facets | --count]",
	"ticket show":        "corvint-tasks ticket show <ticketId|local> [--summary | --fields KEY[.SUB],...]",
	"ticket blockers":    "corvint-tasks ticket blockers <ticketId|local>",
	"ticket export":      "corvint-tasks ticket export [--offset N] [--limit N]",
	"queue status":       "corvint-tasks queue status [--retries] [--summary | --fields KEY[.SUB],...]",
	"roadmap":            "corvint-tasks roadmap [--offset N] [--limit N] [--summary | --fields KEY[.SUB],...] [--facets | --count]",
	"critical-path":      "corvint-tasks critical-path <ticketId|local>",
	"gate list":          "corvint-tasks gate list",
	"gate show":          "corvint-tasks gate show <gateId>",
	"receipt audit":      "corvint-tasks receipt audit",
	"reconcile inspect":  "corvint-tasks reconcile inspect <ticketId>",
	"reconcile intent":   "corvint-tasks reconcile intent (--keep-journal | --adopt-file) --request-id ID --target TICKET|RELEASE --file FILE|- [--canonical-sha256 SHA256] [--role OWNER|OPERATOR]",
	"archive export":     "corvint-tasks archive export [--staging DIR]",
	"archive verify":     "corvint-tasks archive verify [FILE|-]",
	"init":               "corvint-tasks init [--role ROLE] [--request-id ID]",
	"pause":              "corvint-tasks pause --request-id ID [--role OWNER|OPERATOR]",
	"unpause":            "corvint-tasks unpause --request-id ID [--role OWNER|OPERATOR]",
	"policy show":        "corvint-tasks policy show",
	"policy update":      "corvint-tasks policy update --request-id ID --expected-policy-version N --file PATH [--role OWNER|OPERATOR]",
	"import":             "corvint-tasks import --file PATH [--role OWNER|OPERATOR]",
	"cutover":            "corvint-tasks cutover [--execution] --decision REF [--qualification FILE]",
	"release list":       "corvint-tasks release list",
	"release show":       "corvint-tasks release show RELEASE",
	"release readiness":  "corvint-tasks release readiness RELEASE",
	"claim":              "corvint-tasks claim (<ticketId|local> | --next) --holder LABEL --request-id ID [--lease-minutes N] [--branch LABEL] [--base OID] [--scope PATH...] [--pool ID] [--stage implement|review|integrate] [--exclude-member ID]... [--exclude-authors[=all]] [--share-allocation DIGEST] [--repo ALIAS=ROOT]... [--timing] [--role ROLE]",
	"renew":              "corvint-tasks renew --attempt ID --generation G --request-id ID [--lease-minutes N] [--timing] [--role ROLE]",
	"release":            "corvint-tasks release --attempt ID --generation G --request-id ID [--reason CODE] [--evidence LOCAL_REF] [--handoff-to STAGE [--handoff-reason CODE]] [--lane-untouched] [--timing] [--lock-wait SECONDS] [--role ROLE]; release <create|update|candidate|record-gate|promote|list|show|readiness> --help",
	"reap":               "corvint-tasks reap --request-id ID [--attempt ID --generation G [--lease-expires-at T]] [--role ROLE]",
	"widen":              "corvint-tasks widen --attempt ID --generation G --request-id ID (--scope PATH... | --whole-repository) [--role ROLE]",
	"attempt heartbeat":  "corvint-tasks attempt heartbeat --attempt ID --generation G --request-id ID [--timing] [--lock-wait SECONDS] [--role ROLE]",
	"attempt show":       "corvint-tasks attempt show <attemptId> [--summary | --fields KEY[.SUB],...]",
	"plan preview":       "corvint-tasks plan preview [--pool ID] [--stage implement|review|integrate] [--exclude-member ID]... [--exclude-authors[=all]] [--selected-only] [--summary | --fields KEY[.SUB],...]",
	"submit":             "corvint-tasks submit --attempt ID --generation G --request-id ID --tree OID [--role ROLE]",
	"gate run":           "corvint-tasks gate run --attempt ID --generation G --request-id ID --gate GATE [--worktree DIR] [--role ROLE]",
	"complete":           "corvint-tasks complete --attempt ID --generation G --request-id ID --commit OID [--role ROLE]",
	"health":             "corvint-tasks health --member ID [--stage STAGE] --request-id ID [--role ROLE]",
	"pool status":        "corvint-tasks pool status [--pool ID] [--member ID]",
	"doctor":             "corvint-tasks doctor [--refresh] [--plugins DIR] | corvint-tasks doctor --line",
	"pool sweep":         "corvint-tasks pool sweep --request-id ID --timeout-seconds N [--member ID] [--role OWNER|OPERATOR]",
	"pool cleanup":       "corvint-tasks pool cleanup --member ID --allocation SHA256 --request-id ID [--role ROLE]",
	"pool recover":       "corvint-tasks pool recover --member ID --allocation SHA256 --reason TEXT --request-id ID [--role ROLE]",
	"pool confirm-safe":  "corvint-tasks pool confirm-safe --member ID --allocation SHA256 --evidence REF --reason TEXT --request-id ID [--role ROLE]",
	"pool acquire":       "corvint-tasks pool acquire --attempt ID --generation G --pool ID --request-id ID [--exclude-member ID]... [--exclude-authors[=all]] [--role ROLE]",
	"pool release":       "corvint-tasks pool release --attempt ID --generation G --allocation SHA256 --request-id ID [--role ROLE]",
	"lane-leader":        "corvint-tasks lane-leader --directory DIR --capsule FILE",
	"pending":            "corvint-tasks pending",
	"program show":       "corvint-tasks program show",
	"dispatch":           "corvint-tasks dispatch --program ID --config FILE [--once | --ticks N]; dispatch <status|unpark> --help",
	"dispatch status":    "corvint-tasks dispatch status --program ID --config FILE [--events N]",
	"dispatch unpark":    "corvint-tasks dispatch unpark --program ID --config FILE --key TICKET|lane:POOL/MEMBER",
	"service install":    "corvint-tasks service install --program ID --config SERVICE_JSON --request-id ID [--replace]",
	"service status":     "corvint-tasks service status --program ID",
	"service uninstall":  "corvint-tasks service uninstall --program ID --request-id ID",
	"service stop":       "corvint-tasks service stop --program ID --request-id ID [--drain]",
	"service resume":     "corvint-tasks service resume --program ID --request-id ID",
	"service run":        "corvint-tasks service run --program ID --manifest FILE   (manager-started foreground main; not for interactive use)",
	"service run-helper": "corvint-tasks service run-helper --program ID --manifest FILE --helper ID   (manager-started foreground helper wrapper, Linux only; not for interactive use)",
	"config":             "corvint-tasks config (execution NOT_RUN)",
	"plan record":        "corvint-tasks plan record (execution NOT_RUN)",
	"receipt show":       "corvint-tasks receipt show (execution NOT_RUN)",
	"receipt replay":     "corvint-tasks receipt replay (execution NOT_RUN)",
	"archive restore":    "corvint-tasks archive restore (execution NOT_RUN)",
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
	commandUsage["ticket know-how add"] = "corvint-tasks ticket know-how add <ticketId|local> --request-id ID --expected-revision N (--text TEXT | --text-stdin) (--anchor PATH | --symbol PATH#NAME) [--anchor PATH ...] [--symbol PATH#NAME ...] [--route TOKEN ...] [--commit OID] [--supersedes N --reason TEXT] [--attempt ID] [--generation G] [--evidence-path PATH] [--repo ALIAS=ROOT] [--issued-at TS] [--role OWNER|OPERATOR|WORKER]"
	commandUsage["ticket know-how retract"] = "corvint-tasks ticket know-how retract <ticketId|local> --request-id ID --expected-revision N --note N --reason TEXT [--issued-at TS] [--role OWNER|OPERATOR]"
	commandUsage["ticket know-how reconfirm"] = "corvint-tasks ticket know-how reconfirm <ticketId|local> --request-id ID --expected-revision N --note N [--commit OID] [--repo ALIAS=ROOT] [--attempt ID] [--generation G] [--issued-at TS] [--role OWNER|OPERATOR]"
	commandUsage["ticket know-how list"] = "corvint-tasks ticket know-how list [--path PATH ...] [--repo ALIAS=ROOT ...] [--ticket <ticketId|local>] [--limit 1..200]"
	commandUsage["ticket obligations seed"] = "corvint-tasks ticket obligations seed --target <ticketId|local> --expected-revision N --request-id ID (--payload JSON | --payload-stdin) [--issued-at TS] [--role OWNER|OPERATOR]"
	commandUsage["ticket obligations set"] = "corvint-tasks ticket obligations set --target <ticketId|local> --expected-revision N --request-id ID (--payload JSON | --payload-stdin) [--issued-at TS] [--role OWNER|OPERATOR]"
	commandUsage["ticket obligations witness"] = "corvint-tasks ticket obligations witness --target <ticketId|local> --expected-revision N --request-id ID --commit OID (--from-playwright-report FILE [--ids ID,...] | --declared ID,... --manifest-sha256 SHA256 --test-id TEXT --reason TEXT) [--attempt ID --generation G] [--issued-at TS] [--role OWNER|OPERATOR|WORKER]"
	commandUsage["ticket obligations show"] = "corvint-tasks ticket obligations show --target <ticketId|local>"
	commandUsage["ticket obligations plan"] = "corvint-tasks ticket obligations plan --target <ticketId|local> --plan FILE"
	commandUsage["ticket note show"] = "corvint-tasks ticket note show <ticketId|local>"
	commandUsage["ticket note history"] = "corvint-tasks ticket note history <ticketId|local> [--limit 1..50] [--cursor CURSOR]"
	commandUsage["ticket escalate"] = "corvint-tasks ticket escalate --attempt ID --claim-receipt SEQ|RECEIPT --kind decision|infrastructure|scope|blocked --question TEXT [--options A,B] [--supersedes REQUEST --expected-request-revision N] [--blocked-by TICKET [--gate GATE]] [--expected-revision N] --request-id ID [--role ROLE]"
	commandUsage["ticket answer"] = "corvint-tasks ticket answer --target TICKET --text TEXT [--request REQUEST --expected-request-revision N] [--expected-revision N] --request-id ID [--role ROLE]"
	commandUsage["ticket escalation list"] = "corvint-tasks ticket escalation list [--kind KIND] [--state OPEN|ANSWERED|SUPERSEDED] [--target TICKET] [--program NAME] [--limit 1..50] [--cursor ORIGIN_SHA256]"
	commandUsage["ticket escalation show"] = "corvint-tasks ticket escalation show <ticketId|local> <requestId>"
	commandUsage["ticket escalation history"] = "corvint-tasks ticket escalation history <ticketId|local> <requestId> [--limit 1..50] [--cursor EVENT_SHA256]"
	commandUsage["gate record"] = "corvint-tasks gate record <ticketId|local> --gate GATE (--verdict PASS|RETURN | --from-acceptance REPORT) --subject-receipt SEQ --expected-generation N --expected-revision N --request-id ID [--reason CODE:TEXT] [--candidate-evidence SHA256:BYTES] [--evidence LABEL=SHA256:BYTES] [--reviewer-attempt ID] [--issued-at TS] [--role OWNER|OPERATOR|REVIEWER]"
	commandUsage["gate resubmit"] = "corvint-tasks gate resubmit <ticketId|local> --gate GATE --author-attempt ID --subject-receipt SEQ --expected-generation N --expected-revision N --reason CODE:TEXT --request-id ID [--prior-return SHA256] [--candidate-evidence SHA256:BYTES] [--evidence LABEL=SHA256:BYTES] [--issued-at TS] [--role OWNER|OPERATOR|WORKER]"
	commandUsage["gate history"] = "corvint-tasks gate history <ticketId|local> --gate GATE [--cursor SHA256] [--limit N]"
	commandUsage["gate state"] = "corvint-tasks gate state <ticketId|local>"
	commandUsage["run"] += "; corvint-tasks run --attempt ID --generation G --timeout SECONDS [--lease-minutes N] [--role ROLE] [--detach] -- COMMAND...; corvint-tasks run --attach --attempt ID [--run RUN] [--wait SECONDS]"
}

func helpFlags(o *wire.Object, key string) []string {
	v, _ := o.Get(key)
	flags := []string{"--help", "-h"}
	for _, word := range strings.Fields(v.Str) {
		word = strings.Trim(word, "[]();")
		if i := strings.IndexAny(word, "[="); i > 0 {
			word = word[:i] // --exclude-authors[=all] names --exclude-authors
		}
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
