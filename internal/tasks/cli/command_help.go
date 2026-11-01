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
	if result == nil {
		return nil
	}
	treeHelp(cmd, result.Items[0].Obj)
	if verbose {
		return result
	}
	return terseHelp(cmd, result)
}

// terseKeys are the help keys an agent needs to form a valid call.
var terseKeys = []string{"usage", "implemented", "flags", "required", "example", "operation", "payloadKeys", "optionalPayloadKeys"}

func terseHelp(cmd []string, full *wire.Result) *wire.Result {
	o := full.Items[0].Obj
	name := strings.Join(cmd, " ")
	keep := append([]string{}, terseKeys...)
	if isOmitted(name) {
		keep = append(keep, "note")
	}
	if name == "release" {
		// TCD-V0-178 (V1-3200): --reason is call-forming, so its closed codes stay terse.
		keep = append(keep, "reasonCodes")
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
		o.Set("reasonRule", wire.String("--reason takes exactly one of reasonCodes and nothing else; free text refuses MALFORMED naming them. HANDOFF and REVIEW_RETURNED request a stage handoff and must meet handoffPreconditions (otherwise TICKET_STATE, MISSING_EVIDENCE, STALE_POLICY and the other handoffRefusalCodes); every other code only records why the attempt was freed. Omit --reason for a plain release (TCD-V0-178)."))
		o.Set("handoffPreconditions", wire.Strings([]string{
			"HANDOFF requires an external-agent implement, review or integrate stage; REVIEW_RETURNED requires review.",
			"The generation and unexpired lease must match, acceptance must be unchanged, policy must be unchanged or proven compatible, and prospective retry accounting must have no recorded failure or unknown gate.",
			"Without --evidence: submit a candidate first; phase BUILT or CHECKING, scope WITHIN and no pending effects are required.",
			"With --evidence: RUNNING with no candidate, no gate results, scope UNKNOWN and no pending effects; reference uses Identifier grammar (1..128 bytes). Evidence is forbidden for other reasons unless --lane-untouched is supplied, and is forbidden for candidate handoffs.",
			"Under a policy progressSignal HANDOFF_EVIDENCE the reference is the hand-off's progress fingerprint (ATR-V0-080..084): a changed reference counts as progress for loop detection, the doctor and dispatcher backoff; repeat the reference when nothing changed.",
			"Relevant or unproved policy changes fence handoffs with STALE_POLICY. Only policyVersion, holderLiveness, other members' reservations in the allocated pool, and pool members added with their own settings may differ across a fully audited interval; `policy update` lists the live handoffs it fences in handoffFences. Failed/unknown gates remain sticky; no historical refund.",
			"A reference is inert caller evidence, not proof of work or physical cleanup. Release removes the reservation and quarantines an allocated pool; it does not grant completion, review or integration authority.",
			"holderStatus STALE_HOLDER (attempt show, queue status) is advisory input for a coordinator choosing an evidence HANDOFF release; it is not release authority. The release keeps every fence above, and no read, heartbeat, renew or reap releases, frees or fences an attempt because its holder is stale.",
			"--handoff-to implement|review|integrate (optional --handoff-reason) records advisory next-stage intent on a clean HANDOFF or REVIEW_RETURNED generation only; REVIEW_RETURNED accepts only implement and defaults to it. Other combinations refuse MALFORMED. Claims for another stage are not refused.",
		}))
		o.Set("handoffTargets", wire.Strings(intent.StageRoles))
		o.Set("handoffReasonCodes", wire.Strings(snapshot.HandoffReasons))
		o.Set("handoffRefusalCodes", wire.Strings([]string{wire.CodeFenced, wire.CodeStaleTicket, wire.CodeStalePolicy, wire.CodeTicketState, wire.CodeMissingEvidence, wire.CodeMalformed}))
	}
	if lockWaitVerbs[name] {
		o.Set("lockWait", wire.String(lockWaitHelp))
	}
	if redoSettlingVerbs[name] {
		o.Set("redoPending", wire.String(redoPendingHelp))
	}
	if name == "import amend-map" {
		o.Set("note", wire.String("OWNER-only amendment of a mapped queue's import map (CTS-V0-020): one IMPORT_PLAN receipt posts the canonical taskman-import-map/0 file and the queue naming its digest. --expected-map-sha256 names the journaled map you reviewed; another current map refuses REVISION_CONFLICT. Every ticket the old map names stays mapped, a mapped source item keeps its ticket, no ticket is mapped twice, and a newly mapped existing ticket must be that item's import; any conflict refuses DUPLICATE_ID naming each one with nothing written (CTS-V0-021). A queue without a map, a fixture queue or a map of another queue refuses MALFORMED. Re-running the same request ID replays the committed receipt."))
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
	if name == "policy show" {
		o.Set("note", wire.String("Returns the stored policy with policyVersion and policySha256, plus heartbeatTTLSeconds: the effective holder observation TTL that attempt show, queue status and doctor use, from holderLiveness.heartbeatTTLSeconds or 600 when the policy omits it. heartbeatTTLSeconds is not part of the policy bytes or digest (CAL-V0-278)."))
	}
	if name == "attempt heartbeat" {
		o.Set("note", wire.String("Generation-fenced recorded signal. Reads classify it against the policy holderLiveness.heartbeatTTLSeconds (300..86400, default 600 when omitted; policy show reports the effective value). Does not renew the work lease or prove process liveness. A successful current-generation renew records the same signal (CAL-V0-231), so a holder that renews more often than heartbeatTTLSeconds need not also send a heartbeat: a heartbeat recorded at the same instant as a renew writes the same lastHeartbeatAt and changes no holderStatus, and one a moment later only moves the signal by that moment. Send heartbeats only between renews that are further apart than the TTL; freshness follows the last signal, not the lease, so a lease longer than the TTL still reads STALE_HOLDER once the TTL passes without a renew or heartbeat (CAL-V0-277). Use a fresh request ID for each heartbeat; replay never refreshes the timestamp. A committed receipt that head.json does not name yet (REDO_PENDING) does not refuse it: the writer settles that receipt under the store lock, then records the signal, so only a writer still holding the lock delays it (bounded by --lock-wait). A refused heartbeat records no heartbeat signal (a fenced one records only its refusal) and extends no liveness: the holder is classified from the last recorded heartbeat against heartbeatTTLSeconds, so retry the same request ID before the TTL elapses (CAL-V0-222, CAL-V0-223)."))
	}
	if name == "renew" {
		o.Set("note", wire.String("Extends the work lease to now plus --lease-minutes. A successful renew by the current holder at the current generation also records lastHeartbeatAt, so a holder that renews within holderLiveness.heartbeatTTLSeconds is not STALE_HOLDER without a separate attempt heartbeat (CAL-V0-231). Do not pair a heartbeat with each renew: it records the signal again a moment later and is not needed for freshness. A renew-only holder never reads STALE_HOLDER while --lease-minutes*60 is at most heartbeatTTLSeconds; with a longer lease it must renew or heartbeat within the TTL (CAL-V0-277). A renew with any generation other than the attempt's current one, or after the lease expired, is refused REVISION_CONFLICT FENCED and leaves the lease and lastHeartbeatAt unchanged (CAL-V0-009, CAL-V0-230). Use a fresh request ID for each renew: reusing a request ID replays its original committed result, including its original generation, without a write; when the attempt has since moved to another generation the replay also carries an advisory warning naming the current generation (CAL-V0-236)."))
	}
	if name == "pool sweep" {
		o.Set("note", wire.String("Requires explicit timeoutSeconds 1..1800 and OWNER or an explicit OPERATOR policy grant. Replays return the original receipt or PENDING without repeating commands. Private logs can contain command-emitted secrets; explicit archive export includes evidence. FREE records operator-declared reset and verification, not proof of external physical safety."))
	}
	if name == "doctor" {
		o.Set("note", wire.String("Advisory pure read, no lock (TQD-V0-001..012): findings NO_PROGRESS_HANDOFF, REPEAT_REFUSAL, SLOW_LANE_RECOVERY, SETUP_ONLY_PROOF and FALSE_IDLE from at most 4096 receipts within 7 days plus a bounded process walk, each with kind, source, who, detail, remedy, firstSeen, ageSeconds and evidenceSeqs. --plugins DIR runs the directory's executables (at most 4096 entries, 10 s, 64 KiB stdout, a JSON array of {kind,who,detail[,remedy]}, process group killed on exit); a failing plugin is a PLUGIN_FAILED finding and the envelope is untrusted. Only --refresh writes, and only <git common dir>/taskman-doctor/summary.json and its .lock (at most 1 MiB; a linked or foreign-owned cache is refused UNSUPPORTED_FILESYSTEM, a newer one UNSUPPORTED_VERSION). --line prints one plain-text status line from that cache with no store read, or exits 1 when the cache is unavailable. Findings carry no authority."))
	}
	if name == "pool status" {
		o.Set("note", wire.String("Pure read, no lock, no probe and no writes: each configured member's state, current allocation, holder/attempt/generation, quarantine reason with its journal changedSeq, and the last health or cleanup outcome retained with the allocation. The journal records no wall-clock time, so since and observedAt are NOT_OBSERVED, as is any outcome pool state no longer retains. An unknown --pool or --member refuses MALFORMED. A pool that declares allowedSources also shows its defaultSource and allowedSources and, per member, the allocation's requested source and the last identity observation with its class, reported pair, runner wall-clock observedAt and observedSeq, or NOT_OBSERVED (PSR-V0-083)."))
	}
	if name == "claim" || name == "pool acquire" {
		o.Set("detach", wire.String(leaseDetachHelp))
	}
	if name == "claim" {
		o.Set("scope", wire.String("--scope takes every following path until the next flag and may repeat. A ticket that declares QUALIFIED touchPaths or PATH resources keeps that DECLARED scope (CAL-V0-021): a requested path it covers is accepted, and one it does not cover refuses OUT_OF_SCOPE naming each such path, so claim without them and add them with widen (ATR-V0-210). Without declared paths the requested paths are the REQUESTED scope. --scope-from-diff REV (one named ticket; excludes --scope, --whole-repository and --next) derives the scope from the committed change instead of hand-written paths: the rename-free diff between the base tree and REV's tree, with both sides of a rename. Without declared paths it is the DERIVED scope, recorded with derivationSha256 = sha256 of the canonical JSON {profile taskman-diff-scope/0, base, tree, paths}, so submitting REV's tree passes the scope check and gate run and complete close the ticket natively from an INCOMPLETE ticket (TCD-V0-174). A declared scope stays DECLARED, and a changed path it does not cover refuses OUT_OF_SCOPE before any write (TCD-V0-175). An empty diff refuses MALFORMED, more than 256 changed paths LIMIT_EXCEEDED, and an unresolvable REV MALFORMED; none falls back to WHOLE_REPOSITORY. Every claim result carries scope {source, resources}, the effective reserved scope, or null when nothing was reserved (ATR-V0-211)."))
	}
	if name == "claim" || name == "widen" {
		o.Set("admitUnderBarrier", wire.String("--admit-under-barrier passes an OPERATOR admission barrier (corvint-tasks pause) for this one named ticket or attempt, with the OWNER or OPERATOR role, without lifting the barrier: every other claim and widen stays PAUSED. The flag joins the request digest the receipt binds. It never passes an ALL barrier or a CUTOVER, EMERGENCY or DRAIN barrier, and claim --next refuses it (TRM-V0-800, TRM-V0-801). Example: corvint-tasks claim V1-0001 --holder lane --request-id c1 --admit-under-barrier"))
	}
	if name == "pool acquire" {
		o.Set("note", wire.String("Allocates one member to the named live external-agent attempt that holds none, for the attempt's own holder and stage, by the pooled-claim rules: priority yield, --exclude-member and --exclude-authors (review or integrate attempts only), health preparation, and a RESOURCE_COLLISION refusal when no eligible member is free. One allocation per generation: an attempt that returned one cannot acquire again. Generation-fenced; a request-id replay returns the original receipt's allocation (CAL-V0-198..200, CAL-V0-204). --source REV:TREE asks for a member on that source pair: a pool whose allowedSources omit it refuses BLOCKED OUT_OF_SCOPE with SOURCE_NOT_ALLOWED before allocating, and the allocation records the pair (PSR-V0-081)."))
	}
	if name == "pool release" {
		o.Set("note", wire.String("Returns the attempt's exact current allocation early: the member is quarantined until pool cleanup and confirm-safe, exactly as when an attempt ends, and the attempt stays live with no allocation. A shared allocation is refused. Lane-untouched release is unavailable afterwards (CAL-V0-201, CAL-V0-202)."))
	}
	if name == "slots acquire" {
		o.Set("note", wire.String("Takes --count (default 1) shared slots, or with --exclusive the whole capacity, of a policy-declared slots resource for the named live attempt's holder (PSR-V0-032..035). The entry is HELD when it fits, otherwise WAITING; waiters are granted in ticket priority then request order whenever any slot write settles the resource, so a waiter polls with a fresh request ID. An exclusive waiter stops later shared grants until it is granted; --class draws on that class's policy reservation, which no other class can take. A crashed or expired holder is reaped first and an ended or reaped attempt's slots free themselves; there is no member, cleanup or quarantine. A request-id replay returns the original receipt; the slot member is the current state (PSR-V0-037, PSR-V0-038)."))
	}
	if name == "slots release" {
		o.Set("note", wire.String("Drops the attempt's held or waiting entry for --slots and grants waiters; nothing to drop leaves the store unchanged. Passes an admission barrier (PSR-V0-036)."))
	}
	if name == "dispatch" || name == "dispatch status" || name == "dispatch recover-reader" {
		o.Set("readerRecovery", wire.String("A work-state reader whose containment was not proved leaves reader-lifecycle.json, and every dispatch start then refuses QUIESCENCE_UNPROVED; deleting the marker by hand proves nothing. reader-leader.json names the started reader leader's pid and start identity; recover-reader refuses QUIESCENCE_UNPROVED while that process still runs or its identity or record cannot be read, and an exited leader lifts nothing by itself. A reader-lifecycle.json.clearing left by an interrupted removal keeps the quarantine and is attested first. Two recoveries exist (CAL-V0-226..228): (1) a start after a host reboot, when the marker records this machine and an earlier boot, records a HOST_REBOOT attestation under reader-recoveries/ and clears it; a marker from an earlier build, or with no host identity, never qualifies. (2) After confirming the reader and its descendants exited, run `corvint-tasks dispatch recover-reader --program ID --config FILE --reason TEXT --evidence REF --request-id ID`: it refuses RESOURCE_COLLISION while a dispatcher holds the program lock, writes an OPERATOR attestation (reason, evidence reference, marker digest) and only then removes that exact marker. A replay of the request ID returns the record; reusing it for another marker refuses REQUEST_ID_CONFLICT. --reason is prose (1..4096 bytes); --evidence uses Identifier grammar (1..128 bytes). The attestation is the operator's claim, not a physical observation."))
	}
	if name == "pool observe" {
		o.Set("note", wire.String("Records one failed readiness observation of a QUARANTINED member, for a probe or repair that failed outside `health` and `pool cleanup` (PSR-V0-260..262). The detail file (at most 1 MiB) is reduced to a canonical taskman-pool-recovery-detail/0 document: invalid UTF-8 and control characters become '?', every line the secret screen touches becomes [REDACTED], and only the last 2048 bytes are kept; its sha256 is the evidence digest and the document is kept under evidence/. The member's lastHealth or lastCleanup in pool status then reads FAILED with that time and digest. The same kind and digest in a row counts up consecutive; a different failure restarts it at 1, and a passing health or cleanup of that kind clears it. After 3 identical failures, or 24 hours after quarantinedAt was recorded, pool status reports recoveryStalled POOL_MEMBER_RECOVERY_STALLED on the member, and on the pool when every member is stalled; then pool acquire and claim --pool refuse POOL_MEMBER_RECOVERY_STALLED naming the members instead of a plain RESOURCE_COLLISION, and dispatch holds pool-gated launches with that reason (PSR-V0-263..265). A non-QUARANTINED member is refused QUIESCENCE_UNPROVED. Recovery is unchanged: cleanup, recover and confirm-safe."))
	}
	if name == "pool hold" || name == "pool hold renew" || name == "pool hold release" {
		o.Set("note", wire.String("An operator (OWNER or OPERATOR role) holds one FREE pool member for a job outside any dispatched session (PSR-V0-268..272). The hold is recorded in pools.json as a HELD allocation, so claim, pool acquire and dispatch never hand the member out while it stands; pool status and queue status show it with heldAt, expiresAt and expired. --lease-minutes is 5..1440 and renew never moves expiresAt past heldAt plus 1440 minutes (LIMIT_EXCEEDED). An expired hold still occupies the member and cannot be renewed: release it. pool hold release (expired or not, and through an ALL barrier) quarantines the member; run pool cleanup and pool confirm-safe before it is FREE again. --reason is prose of 1..224 bytes."))
	}
	if name == "pool recover" || name == "pool confirm-safe" {
		o.Set("note", wire.String("--reason is free-form prose (1..4096 bytes), not a closed release reason code."))
	}
	if name == "ticket note set" || name == "ticket note clear" {
		o.Set("note", wire.String("Advisory operator prose; never instructions, acceptance or authority. Each note write retains a derived event that receipt audit and redo bind by replaying the transition from audited pre-state (ON-V0-006); claim and claim --next deliver the note pinned by their own admission (ON-V0-007). note set takes several ticket ids with one text (TCD-V0-172): each target gets its own unconditional NOTE_SET with request id <ID>-<local> (a digest suffix when over 64 bytes) and one shared issuedAt, --supersedes and --expected-revision are refused, and the taskman-multi-target/0 item reports each target's outcome; one refusal never blocks the others."))
	}
	if strings.HasPrefix(name, "ticket know-how ") {
		o.Set("note", wire.String("Know-how notes are untrusted agent-authored data, never instructions, acceptance, evidence or authority, and never feed ranking (KHN-V0-007). add pins each --anchor file to its blob in --commit (default HEAD of the working directory) and each --symbol PATH#NAME to the digest of that one declaration's text as the context index's extractor reads it; a retry passes the same --commit and --issued-at. Text, reason, routes, paths and symbols are secret-screened on write. Freshness is computed at read time against the reader's committed HEAD: STALE when a file anchor's blob or a symbol anchor's declaration changed, UNKNOWN when it cannot be resolved (a deleted, renamed, duplicated or unsupported symbol), never stored. reconfirm re-pins a STALE note's own anchors at --commit with provenance, keeping the prior pins in the ledger; a re-pin that changes nothing is refused KNOWHOW_NOT_STALE and an anchor that no longer resolves KNOWHOW_UNRESOLVED. claim and claim --next deliver intersecting notes under a 2 KiB cap; list is a pure read. --role WORKER add is refused unless policy opts in with workerAdd (KHN-V0-021); then the actor must hold the live attempt named by --attempt and --generation on this ticket, every --anchor must lie inside its effects.touchPaths, and --supersedes is refused (KHN-V0-022). For a program spanning repositories, add --repo ALIAS=ROOT pins anchors in the Git top level ROOT and stores them as ALIAS/PATH, so touchPaths and --path match the qualified path (KHN-V0-024, KHN-V0-026); reconfirm of such a note needs the same alias. list and claim take --repo ALIAS=ROOT (repeatable) to resolve a repository note's freshness at that root's HEAD; an unmapped alias leaves its notes UNKNOWN with a warning, never resolved against the working directory (KHN-V0-027)."))
	}
	if strings.HasPrefix(name, "ticket obligations ") {
		o.Set("note", wire.String("A per-ticket obligation ledger (TOL-V0). seed declares the ledger prefix and OPEN obligations {prefix, obligations:[{id, title, core}]}; set changes state (OPEN, PARTIAL, DEFECT, BLOCKED, DEFERRED; PARTIAL is pending and never counts as witnessed), core or review sources with a reason {changes:[{id, state, core, reason, sources?}]}, and core is OWNER-only. sources (TOL-V0-037..046) replaces an obligation's 1..16 review sources: {kind: legacy-case|legacy-spec|manual-case, ref, digest} names a reviewed legacy case (by case ID), legacy spec (by repository path) or manual case with its sha256, and {kind: none, query, searched:[source ids], at} records an empty search; sources are review records, never witness evidence. witness credits ids from one Playwright json-reporter document: only retry-0 results of expected-to-pass tests credit, every match of an id must pass, and the spec file at --commit must hold the id as a whole token; the report is never retained, only its digest. The shipped qualified Playwright version list is 1.63.0 (TOL-V0-126); qualify-version (TOL-V0-060..063) checks a report of the embedded fixture (--print-fixture prints it) and records a passing version in the store-local allow-list, and a report of any other version refuses UNSUPPORTED_VERSION. --test-repository NAME (TOL-V0-064..066) resolves --commit and the source-presence check in a test repository the policy declares under obligations.testRepositories; an undeclared name refuses MALFORMED (OBLIGATION_TEST_REPOSITORY_UNDECLARED). --declared is an OWNER-only vouched witness. A WORKER may witness a report only under a policy obligations.workerWitness opt-in and its own live --attempt and --generation. A test.fail test never credits: an obligation it marks expected-fail (a step title saying expected-fail, a step error naming a defect id, or a title id under a fail annotation naming one) is reported in defectConfirmed with the defect id and error line, and never sets DEFECT; any other obligation it names is reported in mixedExpectedFail with the remedy to move it into its own test. --post-check LOG --post-check-status N names the run's post-check; a non-zero status refuses GATE_FAILED (OBLIGATION_POST_CHECK_FAILED) quoting the log's first actionable line, and nothing from that run is credited. A witness that credits nothing writes nothing (written false). Writes move the ticket revision only; a ledger never satisfies a gate, criterion or completion in V0. show and plan are reads; plan exits 0 only when every core OPEN or PARTIAL obligation is assigned to exactly one planned test, no listed obligation is split, and every id a test lists under its optional rerun member is WITNESSED (reported in reruns); non-core, DEFECT and BLOCKED obligations need no plan entry. witness takes optional provenance --test-commit OID, --environment-tree OID, --evidence-ref TEXT and --test-title TEXT, which show reports on each credited entry's evidence. When some entry records sources, show and plan add sourceReview: stale lists sources whose current digest differs from the reviewed one (legacy-spec at HEAD; legacy-case and manual-case in the corvint-legacy-cases/1 record --legacy-cases FILE names) and unresolved lists those that cannot be read, with the reason; neither changes a state, so a WITNESSED obligation stays WITNESSED. set also takes cites (TOL-V0-079), 1..16 {repository: NAME|null, path} source locations; scope (TOL-V0-080..082) {action: PROPOSE, core, evidence} records a decision with its reason, and {action: ACCEPT, core} applies it, OWNER-only and never by its proposer; under the policy obligations.requireScopeDecision a plain core change is refused APPROVAL_MISSING (OBLIGATION_SCOPE_DECISION_REQUIRED); show adds outOfScope and scopeProposals when any entry records a decision; candidate (TOL-V0-084) {path, startLine, endLine} records a witness candidate awaiting review, cleared by the next witness. boundary (TOL-V0-083) lists each cited repository at HEAD with its runnable test files by naming convention, classifies each pending obligation TESTABLE, NO_TEST_BOUNDARY (NO_TRACKED_CODE or NO_RUNNABLE_TESTS) or UNKNOWN (NO_CITATIONS, REPOSITORY_UNAVAILABLE), and lists accepted out-of-scope obligations that are now TESTABLE as rescopeCandidates. worklist (TOL-V0-086..088) ranks pending obligations whose latest reviewed witness is HELD, for --target or every OPEN or HELD ticket, by --rank priority,nearest,age (default priority,nearest); it excludes a pending candidate, a BLOCKED obligation and a cited example changed or unreadable at HEAD, and lists a NO_FALSIFIABLE_TEXT verdict finding under needsObligationText; --next K returns the head and up to K items citing the same test file. boundary and worklist also report reviewedSubject (TOL-V0-101) for a reviewed entry: its cited test file resolved at the reviewed commit in the review's repository, RESOLVED or UNAVAILABLE with TEST_REPOSITORY_UNRESOLVED, REVIEWED_COMMIT_UNAVAILABLE, SUBJECT_ABSENT_AT_REVIEW, SUBJECT_UNREADABLE_AT_REVIEW or SUBJECT_MISMATCH_AT_REVIEW. A cite naming an undeclared repository refuses MALFORMED (OBLIGATION_CITE_REPOSITORY_UNDECLARED, TOL-V0-102) unless the policy sets obligations.requireDeclaredCites false; obligations.testFilePatterns [{kind, pattern}] extends the boundary's naming set (TOL-V0-103). plan --witnesses (TOL-V0-104..106) adds ledgerWitnesses (each WITNESSED entry's source, testType, commit, testRepository, eventSha256 and status), refuses a reviewed witness stale or unresolved at its staleBasis (the candidate branch commit, else HEAD; TOL-V0-132) as STALE_REVIEW, and names on each UNASSIGNED finding the witness it considered and why it was insufficient (NO_LEDGER_WITNESS, REVIEW_HELD, CANDIDATE_AWAITING_REVIEW). capability (TOL-V0-107..109) answers whether --path, and optionally --contains TEXT inside it, exists at the environment baseline obligations.environmentBaselines declares for the store (\".\") or --repository NAME, naming the baseline ref and commit; it abstains UNKNOWN (ENVIRONMENT_BASELINE_UNDECLARED, ENVIRONMENT_BASELINE_UNRESOLVED, TEST_REPOSITORY_UNRESOLVED, SEARCH_BOUND_EXCEEDED) and never reads HEAD."))
	}
	if name == "preflight" {
		o.Set("note", wire.String("A read of a ticket's obligation ledger against the spec files at --commit (default HEAD), read from Git objects with no checkout (TOL-V0-025..027). Every OPEN, DEFECT or BLOCKED obligation must be named in a test title, test.step title or a <contract>:<id> contract annotation (UNNAMED); a test.fail test must name only expected-fail obligations (MIXED_EXPECTED_FAIL, with file:line); tests in one describe scope that each name one obligation are SPLIT_TESTS unless test.fail or annotated isolated; a spec file whose path holds a line break is UNREADABLE_SPEC_PATH. --path limits the spec files; --plan PATH adds the plan check of the plan document at the commit (PLAN_MISSING when absent); --deep with --gate GATE runs those policy COMMAND gates in a temporary clean worktree of the commit, removed afterwards (DEEP_CHECK_FAILED quotes the first actionable line). Under a policy obligations.requireSourceReview opt-in, an OPEN, DEFECT or BLOCKED obligation without review sources is SOURCE_REVIEW_MISSING and one whose none source lacks a query or a searched source is SOURCE_REVIEW_NONE_UNSEARCHED, each naming the obligation id. Any finding refuses PREFLIGHT_FAILED with each finding's remedy. The source checks are a heuristic scan, not a JavaScript parser. Preflight writes no queue state. --cache (TOL-V0-047..050) keys the report on the ticket record and ledger, the policy, the resolved commit, HEAD, the binary diff of tracked files against HEAD, every untracked file Git does not ignore, the flags and the sha256 of this executable, and replays the report stored for an identical key with cache.status HIT and a PREFLIGHT_CACHE_HIT warning; any change misses (MISS) and stores the new report; --fresh runs the checks and replaces the entry (FRESH); --cache refuses --deep, whose gates are not static; a key that cannot be computed runs the checks with cache.status SKIPPED and the reason. Entries live under the user cache directory (corvint-tasks/preflight), outside every repository and store, at most 256 entries and 7 days old, and are never evidence, a witness or a gate result."))
	}
	if name == "source contains" {
		o.Set("note", wire.String("A pure Git read in the working directory (SCC-V0-001..005): for each commit (at most 64) reports ANCESTOR when it is an ancestor of <rev>; EQUIVALENT with the source commit whose patch-id equals it (a rebase or cherry-pick, as git cherry computes it); ABSENT with PATCH_DIFFERS naming a same-author-and-subject or same-Change-Id source commit whose patch differs (a conflict-resolved rebase), NO_EQUIVALENT, or MERGE_COMMIT (a merge has no patch-id, so only ancestry contains it); or UNKNOWN with MISSING_SOURCE, MISSING_COMMIT, SHALLOW_HISTORY, SCAN_BOUND (more than 10000 source commits outside the commit's history) or GIT_FAILED. UNKNOWN never stands for ABSENT. It never fetches and opens no store."))
	}
	if name == "worktree ensure" {
		o.Set("note", wire.String("Creates or reuses the ticket's worktree in the policy's worktrees.repository on the branch worktrees.branchPattern derives from the ticket's local id alone (TWT-V0-001..007). A new branch starts at worktrees.base; an existing branch is attached only when the base is an ancestor of its tip (PREREQUISITE_UNSATISFIED otherwise). A repeated call returns the same path and branch with created false. The default path is <repository>/.worktrees/<local>; --path and any reused worktree must lie strictly inside the declared repository root (OUT_OF_SCOPE). The result names the repository root and the sha256 of its Git common directory, the base commit, the head, and dirty with the number of uncommitted or untracked entries; ensure never resets, cleans or checks out over them. It writes no Tasks state."))
	}
	if strings.HasPrefix(name, "defects ") {
		o.Set("note", wire.String("The defect register (TDR-V0-001..010): suspected and confirmed PRODUCT or ENVIRONMENT faults found during test work, ids <defects.prefix>-N (default DEF). add and update append one full-state line under the register lock and fsync it; a later line overrides an earlier one and nothing is rewritten. --status is SUSPECTED, CONFIRMED, NOT_A_BUG or DUPLICATE (with --duplicate-of ID); --category PRODUCT or ENVIRONMENT; --severity LOW, MEDIUM, HIGH or CRITICAL. Text is secret-screened (SECRET_DETECTED names the field only). The register is private by default at <git common dir>/taskman-defects/register.jsonl, outside every worktree, push and archive; a policy defects.path places it in the working tree instead. list is a lock-free read that writes nothing. preflight reports DEFECT_UNKNOWN and DEFECT_NOT_A_BUG for register ids cited by obligations or expected-fail annotations, and DEFECT_ENVIRONMENT for a DEFECT or BLOCKED obligation cited only on ENVIRONMENT defects. Defects are untrusted data, never authority."))
	}
	if name == "run-batch" || name == "qualify" {
		o.Set("note", wire.String("Experimental (TOL-V0-028..033, GitHub #714). --config FILE is a closed corvint-tasks-run-config/0 JSON file: testCommand (argv; spec files are appended and PLAYWRIGHT_JSON_OUTPUT_NAME names the report), optional prepCommand, postCheck, captureCommand and staticChecks, runs (1..20), neighbours (NONE or CHANGED_DIRECTORIES), fixturePaths, env (names passed through) and timeoutSeconds. Both verbs run only a clean checkout of --commit (STALE_TREE, DIRTY_WORKTREE) and write their evidence to a new --out DIR/<request-id> (REQUEST_ID_CONFLICT when it exists). With --pool, --attempt and --generation the job acquires a pool member (with its health preparation), runs the capture command before release and releases it; the claim stays the caller's. run-batch runs the named specs once, credits the ledger with the ticket obligations witness rules and writes summary.json giving each obligation its cause (WITNESSED, FAILED with the error line, NOT_REACHED because an earlier step or test failed, NOT_NAMED, NOT_OBSERVED, UNCREDITED with the reason, ALREADY_WITNESSED, DEFECT_CONFIRMED, DEFERRED, OUT_OF_SCOPE); an obligation two earlier summaries in DIR record as FAILED with the same error at the same fixture digest is refused LOOP_DETECTED (OBLIGATION_REPEAT_FAILURE) before any lane is acquired. qualify runs the static checks, the candidate specs N times, the post-check and the specs under the changed directories once, re-runs failing neighbours at --base in a temporary worktree, and writes verdict.json and manifest.json; any retry, candidate failure, post-check failure or neighbour failure the base does not share refuses GATE_FAILED (QUALIFY_NOT_QUALIFIED) naming the reasons. qualify never credits the ledger. The shipped qualified Playwright version list is 1.63.0, so run-batch credits only that version or one an operator recorded with ticket obligations qualify-version."))
	}
	if name == "run-batch" || name == "preflight" {
		v, _ := o.Get("note")
		o.Set("note", wire.String(v.Str+" --refuse-repeat N (TOL-V0-051..055, off by default) keeps a failure-signature ledger under the user cache directory (corvint-tasks/failure-signatures) keyed on the verb, ticket and input digest: after N consecutive failures with the same normalized signature at an unchanged input digest the next run refuses LOOP_DETECTED (FAILURE_SIGNATURE_REPEATED) naming the signature, the count and the digest; a changed input or a pass clears it; --remediation REF admits one more run per distinct REF and first appends an audit line to remediations.jsonl. The ledger is never evidence, a witness or a gate result."))
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
	if name == "ticket closeout-debt" {
		o.Set("note", wire.String("Pure read, no lock and no writes (TCD-V0): OPEN tickets that commits on the integration ref cite by ticket ID or by a linked issue a commit closes, each a candidate with its pending native closeout step, as taskman-closeout-debt/0. --ref defaults to origin/<intentBranch>, else the local intent branch; --limit (default 1000, max 10000) caps the commit scan and a truncated scan is reported. An OPEN ticket whose effective progress names a scanned commit as its head is also a candidate, with citation progress-head (TLF-V0-883). --prs FILE|- (the batch-delivered gh pr list export) adds proposals (TCD-V0-871): each row's proposal is BATCH_CLOSE with the lowest exported MERGED PR whose merge commit was scanned and whose batch-delivered set delivers the ticket, else COMMIT_DELIVERED for a direct-id commit that delivers it, else REVIEW; the item lists proposals (pr, mergeCommit, tickets, the batch-close command) and reviewTickets. It never completes anything. make tasks-reconcile runs it after a merge and retains the report (TCD-V0-872). Exit 0 with no candidates and no proposals, 1 otherwise, 2 on a refusal or error."))
	}
	if name == "ticket delivery-screen" {
		o.Set("note", wire.String("Pure read, no lock and no writes (TCD-V0-159..162, TCD-V0-167..168): for each named ticket, whether an open PR, a merged PR or a commit on the integration ref already references it as delivered, as taskman-delivery-screen/0. --prs FILE is the output of gh pr list --state all --json number,state,title,body,labels; a PR title or body delivers per ticket ID (TCD-V0-730): a Refs, Closes, Fixes, Delivers or Delivered word, or DELIVERED, anywhere in the ID's clause (split at ; and |) delivers it, and a veto (not started, skipped, residual, duplicate, blocked, already, already-delivered or not delivered) mentions only the IDs it governs. A commit line delivers only as a Refs/Closes/Fixes line, a DELIVERED line or a subject scoped to the ticket (its first word, type(ID): scope or final parenthetical); any other mention is CITED (TCD-V0-167). Verdicts: COMPLETED and HELD come from the store before any PR or commit evidence (TCD-V0-170); otherwise IN_FLIGHT, DELIVERED, TRUNCATED (created before the oldest commit of a scan cut at --limit, TCD-V0-168), CITED, MENTIONED, CLEAR, or NOT_OBSERVED when no --prs export was given and nothing on the ref delivers it. Each row carries holds (holdId, reason) and operatorNote, the latest note (NONE, CURRENT, CLEARED or UNAVAILABLE). --prs - reads the export from stdin under the same byte bound (TCD-V0-171). --ref, --since and --limit are those of ticket closeout-debt. --lane-refs also scans every local branch and detached worktree head the screened ref does not contain (TLF-V0-887): a ticket a lane commit delivers is DELIVERED_ON_LANE (after DELIVERED, before TRUNCATED) with laneCommits (ref, commit, subject), a ticket whose effective progress phase is DELIVERED_ON_BRANCH, IN_REVIEW, IN_MERGE_QUEUE or MERGED_AWAITING_CLOSEOUT is IN_FLIGHT, and the item lists laneHeads; a lane scan cut at --limit sets scanTruncated on its laneHeads entry and laneScanTruncated on the item, and a ticket created before that lane's oldest scanned commit is TRUNCATED, never CLEAR; without it the screen is unchanged. Example: corvint-tasks ticket delivery-screen V1-0001 --lane-refs. It never queries GitHub. Exit 0 when every ticket is CLEAR, 1 otherwise, 2 on a refusal or error."))
	}
	if name == "ticket split" {
		o.Set("note", wire.String("Writer (TLF-V0-884, TLF-V0-885): moves the named 1-based acceptance criteria of an OPEN or DRAFT ticket at --expected-revision to a new NATIVE child that copies its classification, effects, gates, dependencies, required roles, pool, execution prerequisites and attached evidence (with each entry's provenance) and carries supersedes: the parent, then removes them from the parent and notes the child in its body. It is an ordered sequence of ordinary mutations under request IDs Q/create, Q/child (only for a parent with execution prerequisites), Q/evidence/I and Q/refine with one issuedAt, each with its own receipt; the parent changes last and refuses STALE_TICKET if the child no longer carries the moved criteria, so an interruption leaves the criteria on both tickets and rerunning with the same --request-id and --issued-at (printed as issuedAt) replays the recorded steps and finishes. A split whose Q/refine is recorded writes nothing; a Q/refine recorded by anything else refuses REQUEST_ID_CONFLICT. A parent body too long for the split note refuses LIMIT_EXCEEDED before any write. A list naming every criterion, a repeat or an out-of-range index refuses. --dry-run prints the child CREATE payload and the parent's remaining criteria and writes nothing. Example: corvint-tasks ticket split V1-3577 --criteria 2,3 --expected-revision 4 --dry-run"))
	}
	if name == "ticket effects-proposals" {
		o.Set("note", wire.String("Pure read, no lock and no writes (TLF-V0-888, TLF-V0-889): for each OPEN ticket whose effects.coverage is UNKNOWN, the repository paths its title, body and acceptance criteria name that exist in the primary worktree (a token with a slash or an extension, or a bare dotfile or extensionless name that exactly matches a root file such as .gitignore or Makefile), each with its source member and the text span that named it (at most 32 per ticket), as taskman-effects-proposals/0 labelled proposal: not authority. A ticket with no candidate is NO_PROPOSAL. A proposal changes nothing: claimability, blockers and planning move only after an operator applies paths with ticket set-effects. Example: corvint-tasks ticket effects-proposals"))
	}
	if name == "ticket batch-delivered" {
		o.Set("note", wire.String("Pure read, no lock and no writes (TCD-V0-163..164): the tickets a merged batch PR delivered, each with its lane PR (a #N the batch title or body names whose exported headRefOid is a commit in the batch merge's ancestry in this repository; any other named PR is listed under excludedPrs as NOT_IN_MERGE_ANCESTRY or HEAD_NOT_OBSERVED and delivers nothing, TCD-V0-179), the batch PR, the main merge commit and the ids of successful CI runs at the merge commit or batch head, as taskman-batch-delivered/0, for a closer to feed to gate run --ci-run and complete. A PR title or body delivers per ticket ID, a veto such as ALREADY-DELIVERED or BLOCKED mentioning only the ID it governs (TCD-V0-730), and the merge's lane commits (every non-merge commit in mergeCommit^1..mergeCommit^2, so a batch of lane merges reads every lane's commits, at most 1000) deliver under the commit rule, with the commit named on the row; laneCommits is NOT_OBSERVED when the merge commit is not fetched (TCD-V0-731). --prs FILE is a gh pr list --json number,state,title,body,headRefName,headRefOid,mergeCommit,labels,mergedAt export holding the batch and its lanes (a MERGED batch without mergeCommit refuses MALFORMED naming this list, TCD-V0-180); --runs FILE a gh run list --json databaseId,headSha,conclusion export, else greenRuns is NOT_OBSERVED. Each PR (lane, then batch) delivers a ticket on a Refs/Closes/Fixes/Delivers or DELIVERED body line, on a body line whose first word is the ticket ID (- **V1-1234**: ...), or through a title scoped to it as a commit subject, so a direct PR with no lanes delivers its own tickets (TCD-V0-870). Tickets only mentioned are listed apart under mentioned. It completes nothing and never queries GitHub. ticket batch-close is the writer over the same exports."))
	}
	if name == "ticket batch-close" {
		o.Set("note", wire.String("Writer. By default (TCD-V0-185..189) it closes every OPEN ticket that ticket batch-delivered lists as delivered by the merged batch through the native lifecycle, as taskman-batch-close/1: it attaches the sha256 of a canonical taskman-batch-merge/0 document (batch PR, PR URL, merge commit, mergedAt, run URL, lane PR, seal, candidate and base commits) as ticket evidence, then claims with --scope-from-diff, submits the candidate tree, runs every required gate (local COMMAND gates in one clean detached worktree of the candidate; a policy ciEvidence gate with --ci-run FILE) and completes at the candidate commit, so the completion is VERIFIED. The candidate is the batch merge commit diffed against its first parent, or with --seal LOCAL=COMMIT[,...] that ticket's sealed lane commit (it must be in the merge's ancestry) diffed against its merge base with the first parent; a declared scope is never widened. The merge commit must be in this clone (else PREREQUISITE_UNSATISFIED, nothing written); the PR URL is the --prs export's url field, else https://github.com/<--repo>/pull/N, else derived from the run URL. A ticket whose required gate cannot run here is refused UNRUNNABLE on its own row with the gate's code before any write and is never closed manually. A failure after the claim releases the attempt and leaves the ticket OPEN; request ids are batch-close-<batchPr>-<local>-a<earlier attempts>-<step>, so a rerun replays a finished step and claims afresh after a release. --manual keeps the TCD-V0-169 writer below unchanged and excludes --seal and --ci-run. With --manual (TCD-V0-169): closes every OPEN ticket that ticket batch-delivered lists as delivered by the merged batch, one COMPLETE_MANUAL each through the same writer as ticket complete-manual, as taskman-batch-close/0. The reason names the lane PR, the batch PR, the merge commit and the CI run URL; evidence is the sha256 of the merge commit string and of the run URL string. The cited run is a successful run at the merge commit (else at the batch head), highest databaseId; its URL is the --runs export's url field (gh run list --json databaseId,headSha,conclusion,url), else https://github.com/<--repo>/actions/runs/<id>. With no green run or no URL it refuses PREREQUISITE_UNSATISFIED and writes nothing. Per-ticket request ids are batch-close-<batchPr>-<local> and issuedAt defaults to the batch mergedAt (else now; pass --issued-at to pin it), so a rerun replays; COMPLETED tickets are skipped as ALREADY_COMPLETED. HELD (TICKET_STATE), other non-OPEN (NOT_OPEN, TICKET_STATE) and live-attempt (ATTEMPT_LIVE) tickets are never closed or reaped, and MENTIONED_ONLY tickets are never closed; each row names its code and recovery. One ticket's refusal never stops the others. --dry-run prints the plan (CLOSE rows) and writes nothing. Exit 0 when every delivered ticket ends COMPLETED (or on a dry run), 1 otherwise."))
	}
	if name == "plan batches" {
		o.Set("note", wire.String("Pure read, no lock and no store writes (NTP-V0-020..025): groups the latest submitted or ready candidate of each OPEN or HELD ticket by pairwise-disjoint changed paths against its base, apart from --shared-path paths, and confirms every join with a dry chained git merge-tree whose objects go to a throw-away object directory that is removed afterwards, so the repository gains no object and no ref. A conflict splits the candidate into a later group; an unresolvable candidate is reported in ungrouped. --max-group-size defaults to 8 (1..64)."))
	}
	if name == "critical-path" {
		o.Set("note", wire.String("Pure read, no lock and no writes: the transitive unsatisfied dependency closure of one ticket (a gate ticket is a ticket) as taskman-critical-path/0, longest chain first, bounded to 256 nodes and 32 chains with truncated and totals. Blocker codes are an open set; facts this reader cannot observe are NOT_OBSERVED, and estimate is always NOT_OBSERVED in v0."))
	}
	if name == "receipt audit" {
		o.Set("note", wire.String("Always the complete audit from receipt 1 (or from the retained anchor of a retired prefix, JRT-V0-005), never resumed from a checkpoint; its item reports anchorSeq (the anchor S it started from, or 0) and retiredThrough (S-1, or 0), and completions {verified, manual}: how many COMPLETED tickets closed through a gated complete and how many through complete-manual (TCD-V0-190); it never reads, creates, replaces or removes the derived checkpoints <state directory>.checkpoint.json (reads) and <state directory>.writer-checkpoint (writers), where the state directory is <git common dir>/taskman. Removing <state directory>.writer-checkpoint forces the next write through the complete audit, which refuses a tampered prefix that a checkpointed writer does not re-read; removing <state directory>.checkpoint.json costs the next read one complete audit. Neither removal changes any journal, intent or archive bytes. --witness-receipts (TOL-V0-138) also checks every WITNESSED obligation whose evidence is a Playwright report digest against the witness receipt store <state directory>/witness-receipts/: a digest with no stored file (RECEIPT_MISSING), a stored file whose bytes no longer hash to it (RECEIPT_MISMATCH), an unreadable one (RECEIPT_UNREADABLE) or a ledger that cannot be folded (LEDGER_UNREADABLE) is a finding marked reviewable=false, and any finding refuses the audit MISSING_EVIDENCE."))
	}
	if name == "receipt put" {
		o.Set("note", wire.String("Stores one regular file of at most 64 MiB in the write-once, content-addressed witness receipt store <state directory>/witness-receipts/<sha256> (TOL-V0-134). An identical file already stored is verified and reported EXISTING; a stored file whose bytes differ from its name is refused, never replaced. It takes no journal lock and writes no receipt; the journal audit and inventory fingerprint ignore the store (TOL-V0-137)."))
	}
	if name == "receipt get" || name == "receipt path" {
		o.Set("note", wire.String("Pure read (TOL-V0-136): resolves a SHA-256 digest in the witness receipt store, re-hashing the stored bytes before answering. get writes exactly the stored bytes to stdout; path reports the file's path and size. A digest with no stored file is MISSING_EVIDENCE and one whose bytes differ is MALFORMED; neither creates the store."))
	}
	if name == "receipt prune" {
		o.Set("note", wire.String("Opt-in maintenance write (TOL-V0-140): removes stored witness receipts that no current obligation ledger entry cites, keeping every cited receipt and every file put in the last 15 minutes so a witness between its put and its ledger write is never raced; it holds the writer lock throughout, so no witness cites a digest it removes, and it also removes temp files a dead put left. A ledger that cannot be folded refuses the prune and removes nothing. --dry-run reports what would be removed and writes nothing. receipt compact and receipt retire never touch the store."))
	}
	if name == "receipt compact" {
		o.Set("note", wire.String("Opt-in maintenance write under the writer lock (CAL-V0-302): after a complete settled audit it rewrites each superseded heartbeat or renew receipt as an elided taskman-receipt/1 that keeps the header, chain digests and request afterimage and drops the attempt record it can rebuild; it never touches ownership or outcome receipts, the head receipt or the latest receipt of an attempt, and it removes both derived checkpoints. --dry-run reports the counts and bytes and writes nothing. Run archive export before every compaction and keep each export: a binary without CAL-V0-303 refuses an elided receipt as UNSUPPORTED_VERSION, and rollback restores each elided receipt from the export taken just before the run that elided it (overlay the exports' receipts/ files oldest first, never replacing a taskman-receipt/0 file with a tombstone), removes both derived checkpoints, then runs the older binary. Compaction shrinks the journal but not the work of a complete audit from receipt 1 (receipt audit, or a lease or pool acquire preparation): that audit restores each elided receipt before validating it, which costs more per receipt than reading a whole one (PSR-V0-114). The fast route avoids that cost: a writer on the CAL-V0-116 writer-checkpoint route (mutate, claim, claim-next, renew, heartbeat, release, pool confirm-safe) and a read that resumes at the read checkpoint validate only the receipts after their checkpoint, so they restore no elided receipt that checkpoint covers; because compaction removes both checkpoints, the next write and the next read each run one complete audit, paying the restore cost, until their checkpoint is rebuilt (PSR-V0-185). It reclaims superseded liveness bodies only: every receipt, chain digest and request afterimage stays in the live journal, so the journal still grows with its history; to bound the journal itself, see `receipt retire` (JRT-V0)."))
	}
	if name == "receipt retire" {
		o.Set("note", wire.String("Operator-invoked, one-way and destructive to the live store (JRT-V0): never run automatically. Under the writer lock, after a complete settled audit with no pending receipt, staging slot, barrier, live attempt, held reservation, pool occupancy, or non-terminal ticket carrying review or escalation references, it exports the store to --archive FILE (a new file outside the state directory, common directory and worktree), verifies that export, binds its digest and the anchor (the head receipt) into <state directory>/RETIRED_PREFIX, then removes receipts before the anchor and both derived checkpoints. Later audits start at the anchor. --dry-run runs every check except the export and writes nothing. An interrupted run leaves RETIRE_INCOMPLETE, which blocks reads and writes until the next retire or unretire rolls it forward (seal present) or back (seal absent). Keep the archive: it is the only copy of the retired receipts and the rollback input."))
	}
	if name == "receipt unretire" {
		o.Set("note", wire.String("Rollback of receipt retire (JRT-V0-011): under the writer lock it checks --archive FILE against the digest and size the seal binds, verifies it, restores every retired receipt it holds, removes the seal and both derived checkpoints, and requires a complete audit from receipt 1."))
	}
	if name == "journal seal" {
		o.Set("note", wire.String("Operator-invoked migration (JSG-V0-013): under the writer lock, after a complete settled audit, it converts an unsegmented store and seals every whole 512-receipt range that leaves at least 512 receipts active into segments/ bound by a taskman-journal-head/1 head; writers then keep sealing. A binary that predates the format refuses the sealed store as UNSUPPORTED_VERSION; journal unseal is the rollback. Refused on a store with a retired prefix. A fault refuses before the seal in progress writes; a stop leaves SEAL_INCOMPLETE for the next writer to settle."))
	}
	if name == "journal unseal" {
		o.Set("note", wire.String("Rollback of journal seal (JSG-V0-013): under the writer lock, after a complete settled audit, it restores every sealed receipt and request afterimage, last segment first, and publishes a taskman-journal-head/0 head; the state directory is then byte-identical to the unsegmented tree. Each segment is its own marker-guarded step, so a stop is settled by the next writer and a rerun continues. On an unsegmented store it writes nothing."))
	}
	if name == "archive verify" {
		o.Set("note", wire.String("Reads FILE, or stdin when FILE is absent or -. Help reads neither."))
	}
	if name == "archive restore" {
		o.Set("note", wire.String("Writer (TRL-V0-870, TRL-V0-871): verifies FILE (an archive export) in full, then rebuilds the state directory <git common dir>/taskman byte-for-byte, which must be absent. Intent files the archive holds must be absent or byte-identical; missing ones are written first, then the state directory is published by one rename. Every refusal (MALFORMED for a damaged or tampered archive, UNSUPPORTED for an existing store or a segmented head, INTENT_DIVERGED for differing intent) writes nothing. An archive from another primary worktree restores with a warning naming `corvint-tasks relocate --from <recorded path>`. Rollback: remove the restored state directory and any intent file it wrote."))
	}
	if name == "criterion-binding capture" {
		o.Set("note", wire.String("Returns a capture and its verification from the current native ticket and attempt. Offline verify consumes the canonical capture object on stdin, not the result envelope."))
	}
	if name == "criterion-binding verify" {
		o.Set("note", wire.String("Reads only canonical capture bytes from stdin; accepts no execution flags. Verification is offline. Help does not read stdin."))
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
}

// lockWaitHelp documents the CAL-V0-111 --lock-wait of every lockWaitVerbs
// command (CAL-V0-216, decision 0518, adds the other preparation-admitted writes).
const lockWaitHelp = "--lock-wait SECONDS (whole seconds, 1..300; anything else refuses MALFORMED) waits up to that long for preparation admission and the store lock instead of the default 30 seconds, for this command only. It is not part of the request: after LOCK_TIMEOUT, resubmit the same request ID with the same arguments; that commits once or replays the committed receipt."

// redoSettlingVerbs are the writers outside lockWaitVerbs whose pre-write
// probe accepts a pending receipt and whose store writer settles it
// (CAL-V0-260, CAL-V0-261); redoPendingHelp is their help (CAL-V0-262).
var redoSettlingVerbs = map[string]bool{
	"pause": true, "unpause": true, "policy update": true, "import": true, "import amend-map": true, "cutover": true, "relocate": true,
	"ticket escalate": true, "ticket answer": true, "run": true, "health": true, "pool cleanup": true,
	"pool operate": true,
}

const redoPendingHelp = "A committed receipt that head.json does not name yet (REDO_PENDING, left by an interrupted writer) does not refuse this command: its writer settles that receipt under the store lock before its own write and reports the completed redo in a warning. A pending receipt the receipt audit does not validate as a redo is refused with the audit's own error. attempt run settles it at its first lease write, before launch; run --attach and every other read never settle one. Over a pending receipt, the policy update handoff fence preview is NOT_OBSERVED (REDO_PENDING) (CAL-V0-260..262)."

// Usage is the single help inventory for each command's supported inputs.
// Omitted verbs deliberately have no invented execution flags.
var commandUsage = map[string]string{
	"criterion-binding capture": "corvint-tasks criterion-binding capture --ticket ID --attempt ID",
	"criterion-binding verify":  "corvint-tasks criterion-binding verify (canonical capture on stdin)",

	"version":                  "corvint-tasks version (alias --version)",
	"ticket list":              "corvint-tasks ticket list [--status S[,S...]] [--offset N] [--limit N] [--summary | --fields KEY[.SUB],...] [--facets | --count]",
	"ticket search":            "corvint-tasks ticket search [--status S] [--kind K] [--priority P] [--owner L] [--milestone L] [--label L] [--text T] [--phase PHASE|NONE] [--offset N] [--limit N] [--summary | --fields KEY[.SUB],...] [--facets | --count]",
	"ticket show":              "corvint-tasks [--root DIR] ticket show <ticketId|local> [--json] [--summary | --fields KEY[.SUB],...]   (output is always JSON, so --json is a no-op; ticket fields such as body and acceptanceCriteria live under record.: items[0].record.acceptanceCriteria, --fields record.body,record.acceptanceCriteria; the revision is items[0].revision; an unpadded token such as QB-1 resolves the one ticket QB-0001; an unknown key is refused naming the valid keys)",
	"ticket blockers":          "corvint-tasks ticket blockers <ticketId|local>",
	"ticket closeout-debt":     "corvint-tasks ticket closeout-debt [--ref REF] [--since REV] [--limit N] [--prs FILE|-]",
	"ticket delivery-screen":   "corvint-tasks ticket delivery-screen <ticketId|local>... [--prs FILE|-] [--ref REF] [--since REV] [--limit N] [--lane-refs]",
	"ticket batch-delivered":   "corvint-tasks ticket batch-delivered --pr N --prs FILE [--runs FILE]",
	"ticket effects-proposals": "corvint-tasks ticket effects-proposals",
	"ticket batch-close":       "corvint-tasks ticket batch-close --pr N --prs FILE --runs FILE [--repo OWNER/NAME] [--seal LOCAL=COMMIT,...] [--ci-run FILE] [--manual] [--dry-run] [--issued-at TS] [--role OWNER|OPERATOR]",
	"ticket export":            "corvint-tasks ticket export [--offset N] [--limit N]",
	"ticket similar":           "corvint-tasks ticket similar (--text TEXT | --payload-stdin) [--status S]... [--label L]... [--limit N] [--digest FILE]   (example: corvint-tasks ticket similar --text 'heavy.sh ignores unstaged files')",
	"ticket digest":            "corvint-tasks ticket digest [--label L]... [--status S]... [--since YYYY-MM-DD] [--format json|text]   (default --label agent-memory --status OPEN; example: corvint-tasks ticket digest --since 2026-10-23 --format text > digest.txt)",
	"ticket recur":             "corvint-tasks ticket recur <ticketId|local> --context TEXT --request-id ID [--issued-at TS] [--role OWNER|OPERATOR]   (example: corvint-tasks ticket recur V1-3211 --context 'lane w4: stale trace store' --request-id r1)",
	"ticket split":             "corvint-tasks ticket split <ticketId|local> --criteria N[,M...] --expected-revision R (--request-id Q | --dry-run) [--issued-at TS] [--role OWNER|OPERATOR]   (example: corvint-tasks ticket split V1-3577 --criteria 2,3 --expected-revision 4 --request-id split-3577)",
	"queue status":             "corvint-tasks queue status [--retries] [--summary | --fields KEY[.SUB],...]",
	"queue snapshot":           "corvint-tasks queue snapshot [--out FILE]   (writes the intent tree as one taskman-intent-snapshot/0 object for a lane host without a journal, which reads it with CORVINT_TASKS_SNAPSHOT=FILE; example: corvint-tasks queue snapshot --out /tmp/queue-snapshot.json)",
	"roadmap":                  "corvint-tasks roadmap [--offset N] [--limit N] [--summary | --fields KEY[.SUB],...] [--facets | --count]",
	"critical-path":            "corvint-tasks critical-path <ticketId|local>",
	"gate list":                "corvint-tasks gate list",
	"gate show":                "corvint-tasks gate show <gateId>",
	"receipt audit":            "corvint-tasks receipt audit [--witness-receipts]",
	"receipt put":              "corvint-tasks receipt put FILE",
	"receipt get":              "corvint-tasks receipt get SHA256",
	"receipt path":             "corvint-tasks receipt path SHA256",
	"receipt prune":            "corvint-tasks receipt prune [--dry-run]",
	"receipt compact":          "corvint-tasks receipt compact [--dry-run]",
	"receipt retire":           "corvint-tasks receipt retire --archive FILE [--dry-run]",
	"receipt unretire":         "corvint-tasks receipt unretire --archive FILE",
	"journal seal":             "corvint-tasks journal seal",
	"journal unseal":           "corvint-tasks journal unseal",
	"reconcile inspect":        "corvint-tasks reconcile inspect <ticketId>",
	"reconcile intent":         "corvint-tasks reconcile intent (--keep-journal | --adopt-file) --request-id ID --target TICKET|RELEASE --file FILE|- [--canonical-sha256 SHA256] [--role OWNER|OPERATOR]",
	"archive export":           "corvint-tasks archive export [--staging DIR]",
	"archive verify":           "corvint-tasks archive verify [FILE|-]",
	"archive restore":          "corvint-tasks archive restore FILE   (example: corvint-tasks archive restore ../queue-export.tar)",
	"init":                     "corvint-tasks init [--role ROLE] [--request-id ID]",
	"pause":                    "corvint-tasks pause --request-id ID [--role OWNER|OPERATOR]",
	"unpause":                  "corvint-tasks unpause --request-id ID [--role OWNER|OPERATOR]",
	"policy show":              "corvint-tasks policy show",
	"policy update":            "corvint-tasks policy update --request-id ID --expected-policy-version N --file PATH [--role OWNER|OPERATOR]",
	"import":                   "corvint-tasks import --file PATH [--role OWNER|OPERATOR]",
	"import amend-map":         "corvint-tasks import amend-map --request-id ID --expected-map-sha256 SHA256 --file PATH [--role OWNER]",
	"cutover":                  "corvint-tasks cutover [--execution] --decision REF [--qualification FILE]",
	"relocate":                 "corvint-tasks relocate --from OLD --request-id ID [--role OWNER|OPERATOR] [--dispatch-config FILE]   (run from the new primary worktree; OLD is the head.primaryWorktree the refusal names)",
	"release list":             "corvint-tasks release list",
	"release show":             "corvint-tasks release show RELEASE",
	"release readiness":        "corvint-tasks release readiness RELEASE",
	"claim":                    "corvint-tasks claim (<ticketId|local> | --next) --holder LABEL --request-id ID [--lease-minutes N] [--branch LABEL] [--base OID] [--scope PATH... | --scope-from-diff REV] [--admit-under-barrier] [--pool ID] [--stage implement|review|integrate] [--exclude-member ID]... [--exclude-authors[=all]] [--share-allocation DIGEST] [--repo ALIAS=ROOT]... [--timing] [--lock-wait SECONDS] [--role ROLE] [--detach [--wait SECONDS]]; claim --attach --request-id ID [--holder LABEL] [--wait SECONDS]",
	"renew":                    "corvint-tasks renew --attempt ID --generation G --request-id ID [--lease-minutes N] [--timing] [--lock-wait SECONDS] [--role ROLE]",
	"release":                  "corvint-tasks release --attempt ID --generation G --request-id ID [--reason CODE] [--evidence LOCAL_REF] [--handoff-to STAGE [--handoff-reason CODE]] [--lane-untouched] [--timing] [--lock-wait SECONDS] [--role ROLE]; release <create|update|candidate|record-gate|promote|list|show|readiness> --help",
	"reap":                     "corvint-tasks reap --request-id ID [--attempt ID --generation G [--lease-expires-at T]] [--role ROLE]",
	"widen":                    "corvint-tasks widen --attempt ID --generation G --request-id ID (--scope PATH... | --whole-repository) [--admit-under-barrier] [--role ROLE]",
	"attempt heartbeat":        "corvint-tasks attempt heartbeat --attempt ID --generation G --request-id ID [--timing] [--lock-wait SECONDS] [--role ROLE]",
	"attempt show":             "corvint-tasks attempt show <attemptId> [--summary | --fields KEY[.SUB],...]",
	"plan preview":             "corvint-tasks plan preview [--pool ID] [--stage implement|review|integrate] [--exclude-member ID]... [--exclude-authors[=all]] [--selected-only] [--summary | --fields KEY[.SUB],...]",
	"submit":                   "corvint-tasks submit --attempt ID --generation G --request-id ID --tree OID [--lock-wait SECONDS] [--role ROLE]",
	"gate run":                 "corvint-tasks gate run --attempt ID --generation G --request-id ID --gate GATE [--worktree DIR | --ci-run FILE] [--role ROLE]",
	"complete":                 "corvint-tasks complete --attempt ID --generation G --request-id ID --commit OID [--role ROLE]",
	"health":                   "corvint-tasks health --member ID [--stage STAGE] --request-id ID [--role ROLE]",
	"pool status":              "corvint-tasks pool status [--pool ID] [--member ID]",
	"doctor":                   "corvint-tasks doctor [--refresh] [--plugins DIR] | corvint-tasks doctor --line",
	"pool sweep":               "corvint-tasks pool sweep --request-id ID --timeout-seconds N [--member ID] [--role OWNER|OPERATOR]",
	"pool cleanup":             "corvint-tasks pool cleanup --member ID --allocation SHA256 --request-id ID [--role ROLE]",
	"pool recover":             "corvint-tasks pool recover --member ID --allocation SHA256 --reason TEXT --request-id ID [--role ROLE]",
	"pool hold":                "corvint-tasks pool hold --pool ID --member ID --holder ID --lease-minutes N --reason TEXT --request-id ID [--role ROLE]",
	"pool hold renew":          "corvint-tasks pool hold renew --member ID --allocation SHA256 --lease-minutes N --request-id ID [--role ROLE]",
	"pool hold release":        "corvint-tasks pool hold release --member ID --allocation SHA256 --request-id ID [--role ROLE]",
	"pool observe":             "corvint-tasks pool observe --member ID --kind health|cleanup --result FAILED --detail-file FILE --request-id ID [--allocation SHA256] [--role ROLE]",
	"pool confirm-safe":        "corvint-tasks pool confirm-safe --member ID --allocation SHA256 --evidence REF --reason TEXT --request-id ID [--lock-wait SECONDS] [--role ROLE]",
	"pool acquire":             "corvint-tasks pool acquire --attempt ID --generation G --pool ID --request-id ID [--source REV:TREE] [--exclude-member ID]... [--exclude-authors[=all]] [--lock-wait SECONDS] [--role ROLE] [--detach [--wait SECONDS]]; pool acquire --attach --request-id ID [--wait SECONDS]",
	"pool release":             "corvint-tasks pool release --attempt ID --generation G --allocation SHA256 --request-id ID [--lock-wait SECONDS] [--role ROLE]",
	"pool operate":             "corvint-tasks pool operate --pool ID --member ID --operation provision|refresh|reset|retarget|teardown --request-id ID [--allocation SHA256] [--source REV:TREE] [--role OWNER|OPERATOR]",
	"pool wait":                "corvint-tasks pool wait --pool ID --ttl-seconds N --request-id ID (--attempt ID --generation G | --ticket ID --holder LABEL) [--lock-wait SECONDS] [--role ROLE]",
	"slots acquire":            "corvint-tasks slots acquire --attempt ID --generation G --slots ID --request-id ID [--count N | --exclusive] [--class LABEL] [--lock-wait SECONDS] [--role ROLE]",
	"slots release":            "corvint-tasks slots release --attempt ID --generation G --slots ID --request-id ID [--lock-wait SECONDS] [--role ROLE]",
	"lane-leader":              "corvint-tasks lane-leader --directory DIR --capsule FILE",
	"pending":                  "corvint-tasks pending",
	"program show":             "corvint-tasks program show",
	"program cleanup":          "corvint-tasks program cleanup --program ID --config FILE",
	"dispatch":                 "corvint-tasks dispatch --program ID --config FILE [--once | --ticks N]; dispatch <status|unpark|upgrade|recover-reader|salvage|context-hook> --help",
	"dispatch context-hook":    "corvint-tasks dispatch context-hook   (Claude Code hook a dispatched worker runs: reads CORVINT_DISPATCH_CONTEXT_FILE and the hook payload on stdin, prints undelivered context notices as hook JSON, always exits 0)",
	"dispatch recover-reader":  "corvint-tasks dispatch recover-reader --program ID --config FILE --reason TEXT --evidence REF --request-id ID",
	"dispatch salvage":         "corvint-tasks dispatch salvage --program ID --config FILE --attempt ATTEMPT   (writes the secret-screened hand-off note of the program worker holding ATTEMPT, running or stopped; never changes the attempt)",
	"dispatch status":          "corvint-tasks dispatch status --program ID --config FILE [--events N] [--worker ID]",
	"dispatch unpark":          "corvint-tasks dispatch unpark --program ID --config FILE --key TICKET|lane:POOL/MEMBER",
	"dispatch upgrade":         "corvint-tasks dispatch upgrade --program ID --config FILE --check NEW_BUILD   (read-only: runs NEW_BUILD's dispatch status on this program and reports ROLLING, DRAIN_REQUIRED, REFUSED or UNKNOWN for a restart on that build); dispatch upgrade --program ID --config FILE --apply NEW_BUILD   (asks the running dispatcher to hand its lock and live workers to NEW_BUILD in place: REQUESTED, REFUSED or UNKNOWN; workers launched earlier finish on their own build)",
	"service install":          "corvint-tasks service install --program ID --config SERVICE_JSON --request-id ID [--replace]",
	"service status":           "corvint-tasks service status --program ID",
	"service uninstall":        "corvint-tasks service uninstall --program ID --request-id ID",
	"service stop":             "corvint-tasks service stop --program ID --request-id ID [--drain]",
	"service resume":           "corvint-tasks service resume --program ID --request-id ID",
	"service run":              "corvint-tasks service run --program ID --manifest FILE   (manager-started foreground main; not for interactive use)",
	"service run-helper":       "corvint-tasks service run-helper --program ID --manifest FILE --helper ID   (manager-started foreground helper wrapper, Linux only; not for interactive use)",
	"config":                   "corvint-tasks config (execution NOT_RUN)",
	"plan batches":             "corvint-tasks plan batches [--max-group-size N] [--shared-path PATH]...",
	"plan record":              "corvint-tasks plan record (execution NOT_RUN)",
	"receipt show":             "corvint-tasks receipt show (execution NOT_RUN)",
	"receipt replay":           "corvint-tasks receipt replay (execution NOT_RUN)",
}

func init() {
	for _, verb := range []string{"create", "update", "candidate", "record-gate", "promote"} {
		commandUsage["release "+verb] = "corvint-tasks release " + verb + " --request-id ID --target RELEASE [--expected-revision N] (--payload JSON | --payload-stdin) [--issued-at TS] [--role ROLE]"
	}
	commandUsage["release membership"] = "corvint-tasks release membership --request-id ID --target RELEASE --expected-revision N (--add TICKET | --remove TICKET)... [--issued-at TS] [--role ROLE]"
	for _, verb := range []string{"run", "admit", "resume", "retry", "drain", "cancel", "answer"} {
		commandUsage[verb] = "corvint-tasks " + verb + " --program ID --config FILE [--role implementer|reviewer|integrator] [--count N] [--ticket ID] [--host codex|claude-code|opencode] [--grant ID] [--question SHA256] [--revision N] [--answer TEXT]"
	}
	commandUsage["ticket note set"] = "corvint-tasks ticket note set <ticketId|local>... --request-id ID (--text TEXT | --text-stdin) [--supersedes N] [--expected-revision N] [--issued-at TS] [--role OWNER|OPERATOR]"
	commandUsage["ticket note clear"] = "corvint-tasks ticket note clear <ticketId|local> --request-id ID [--supersedes N] [--expected-revision N] [--issued-at TS] [--role OWNER|OPERATOR]"
	commandUsage["ticket know-how add"] = "corvint-tasks ticket know-how add <ticketId|local> --request-id ID --expected-revision N (--text TEXT | --text-stdin) (--anchor PATH | --symbol PATH#NAME) [--anchor PATH ...] [--symbol PATH#NAME ...] [--route TOKEN ...] [--commit OID] [--supersedes N --reason TEXT] [--attempt ID] [--generation G] [--evidence-path PATH] [--repo ALIAS=ROOT] [--issued-at TS] [--role OWNER|OPERATOR|WORKER] [--refuse-repeat N [--remediation REF]]"
	commandUsage["ticket know-how retract"] = "corvint-tasks ticket know-how retract <ticketId|local> --request-id ID --expected-revision N --note N --reason TEXT [--issued-at TS] [--role OWNER|OPERATOR]"
	commandUsage["ticket know-how reconfirm"] = "corvint-tasks ticket know-how reconfirm <ticketId|local> --request-id ID --expected-revision N --note N [--commit OID] [--repo ALIAS=ROOT] [--attempt ID] [--generation G] [--issued-at TS] [--role OWNER|OPERATOR]"
	commandUsage["ticket know-how list"] = "corvint-tasks ticket know-how list [--path PATH ...] [--repo ALIAS=ROOT ...] [--ticket <ticketId|local>] [--limit 1..200]"
	commandUsage["ticket obligations seed"] = "corvint-tasks ticket obligations seed --target <ticketId|local> --expected-revision N --request-id ID (--payload JSON | --payload-stdin) [--issued-at TS] [--role OWNER|OPERATOR]"
	commandUsage["ticket obligations set"] = "corvint-tasks ticket obligations set --target <ticketId|local> --expected-revision N --request-id ID (--payload JSON | --payload-stdin) [--issued-at TS] [--role OWNER|OPERATOR]"
	commandUsage["ticket obligations witness"] = "corvint-tasks ticket obligations witness --target <ticketId|local> --expected-revision N --request-id ID --commit OID (--from-playwright-report FILE [--ids ID,...] [--post-check LOG --post-check-status N] [--retain] | --declared ID,... --manifest-sha256 SHA256 --test-id TEXT --reason TEXT | --reviewed ID --review FILE [--exclude-author ID,...]) [--attempt ID --generation G] [--test-commit OID] [--environment-tree OID] [--evidence-ref TEXT] [--test-title TEXT] [--test-repository NAME] [--issued-at TS] [--role OWNER|OPERATOR|WORKER] [--refuse-repeat N [--remediation REF]]"
	commandUsage["ticket obligations show"] = "corvint-tasks ticket obligations show --target <ticketId|local> [--legacy-cases FILE]"
	commandUsage["ticket obligations plan"] = "corvint-tasks ticket obligations plan --target <ticketId|local> --plan FILE [--legacy-cases FILE] [--witnesses]"
	commandUsage["ticket obligations capability"] = "corvint-tasks ticket obligations capability --path PATH [--repository NAME] [--contains TEXT]"
	commandUsage["ticket obligations boundary"] = "corvint-tasks ticket obligations boundary --target <ticketId|local>"
	commandUsage["ticket obligations worklist"] = "corvint-tasks ticket obligations worklist [--target <ticketId|local>] [--rank priority,nearest,age] [--next K]"
	commandUsage["ticket obligations qualify-version"] = "corvint-tasks ticket obligations qualify-version (--print-fixture | --from-playwright-report FILE)"
	commandUsage["preflight"] = "corvint-tasks preflight <ticketId|local> [--commit OID] [--path PREFIX ...] [--plan PATH] [--deep --gate GATE ... | --cache [--fresh]] [--refuse-repeat N [--remediation REF]]"
	commandUsage["source contains"] = "corvint-tasks source contains <rev> <commit> [<commit> ...]"
	commandUsage["worktree ensure"] = "corvint-tasks worktree ensure <ticketId|local> [--path PATH]"
	commandUsage["defects add"] = "corvint-tasks defects add --title TEXT --location TEXT --evidence REF [--severity LOW|MEDIUM|HIGH|CRITICAL] [--status SUSPECTED|CONFIRMED|NOT_A_BUG|DUPLICATE] [--category PRODUCT|ENVIRONMENT] [--duplicate-of ID]"
	commandUsage["defects update"] = "corvint-tasks defects update <defectId> [--title TEXT] [--location TEXT] [--evidence REF] [--severity LOW|MEDIUM|HIGH|CRITICAL] [--status SUSPECTED|CONFIRMED|NOT_A_BUG|DUPLICATE] [--category PRODUCT|ENVIRONMENT] [--duplicate-of ID]"
	commandUsage["defects list"] = "corvint-tasks defects list [--status STATUS] [--category CATEGORY]"
	commandUsage["run-batch"] = "corvint-tasks run-batch <ticketId|local> --config FILE --spec PATH [--spec PATH ...] --out DIR --request-id ID [--commit OID] [--ids ID,...] [--pool POOL --attempt ID --generation G] [--role OWNER|OPERATOR|WORKER] [--refuse-repeat N [--remediation REF]]"
	commandUsage["qualify"] = "corvint-tasks qualify --config FILE --spec PATH [--spec PATH ...] --base REV --out DIR --request-id ID [--commit OID] [--pool POOL --attempt ID --generation G]"
	commandUsage["ticket note show"] = "corvint-tasks ticket note show <ticketId|local>"
	commandUsage["ticket note history"] = "corvint-tasks ticket note history <ticketId|local> [--limit 1..50] [--cursor CURSOR]"
	commandUsage["ticket escalate"] = "corvint-tasks ticket escalate --attempt ID --claim-receipt SEQ|RECEIPT --kind decision|infrastructure|scope|blocked --question TEXT [--options A,B] [--supersedes REQUEST --expected-request-revision N] [--blocked-by TICKET [--gate GATE]] [--expected-revision N] --request-id ID [--role ROLE]"
	commandUsage["ticket answer"] = "corvint-tasks ticket answer --target TICKET --text TEXT [--request REQUEST --expected-request-revision N] [--expected-revision N] --request-id ID [--role ROLE]"
	commandUsage["ticket escalation list"] = "corvint-tasks ticket escalation list [--kind KIND] [--state OPEN|ANSWERED|SUPERSEDED] [--target TICKET] [--program NAME] [--limit 1..50] [--cursor ORIGIN_SHA256]"
	commandUsage["ticket escalation show"] = "corvint-tasks ticket escalation show <ticketId|local> <requestId>"
	commandUsage["ticket escalation history"] = "corvint-tasks ticket escalation history <ticketId|local> <requestId> [--limit 1..50] [--cursor EVENT_SHA256]"
	commandUsage["gate record"] = "corvint-tasks gate record <ticketId|local> --gate GATE (--verdict PASS|RETURN | --from-acceptance REPORT) --subject-receipt SEQ --expected-generation N --expected-revision N --request-id ID [--reason CODE:TEXT] [--candidate-evidence SHA256:BYTES] [--evidence LABEL=SHA256:BYTES] [--reviewer-attempt ID] [--issued-at TS] [--role OWNER|OPERATOR|REVIEWER]"
	commandUsage["gate resubmit"] = "corvint-tasks gate resubmit <ticketId|local> --gate GATE --author-attempt ID --subject-receipt SEQ --expected-generation N --expected-revision N --reason CODE:TEXT --request-id ID [--prior-return SHA256] [--candidate-evidence SHA256:BYTES] [--evidence LABEL=SHA256:BYTES] [--issued-at TS] [--role OWNER|OPERATOR|WORKER] [--refuse-repeat N [--remediation REF]]"
	commandUsage["gate history"] = "corvint-tasks gate history <ticketId|local> --gate GATE [--cursor SHA256] [--limit N]"
	commandUsage["gate state"] = "corvint-tasks gate state <ticketId|local>   (each declared gate's closed external-review work state, read from typed events)"
	commandUsage["gate packet"] = "corvint-tasks gate packet <ticketId|local> [--commit REV] [--source PATH|NAME [--config FILE]] [--max-bytes 4096..262144] [--checkpoint NOTE]"
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
