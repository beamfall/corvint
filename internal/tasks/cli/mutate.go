package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// mutationVerbs maps a `ticket` subcommand to its §3.3 operation. The verb
// name decides the operation; an envelope claiming a different one is
// refused, so a mistyped verb cannot silently run another mutation.
var mutationVerbs = map[string]string{
	"create":           mutation.OpCreate,
	"refine":           mutation.OpRefine,
	"prioritize":       mutation.OpPrioritize,
	"set-dependencies": mutation.OpSetDependencies,
	"set-gates":        mutation.OpSetGates,
	"set-effects":      mutation.OpSetEffects,
	"hold":             mutation.OpHold,
	"release-hold":     mutation.OpReleaseHold,
	"reopen":           mutation.OpReopen,
	"archive":          mutation.OpArchive,
	"restore":          mutation.OpRestore,
	"complete-manual":  mutation.OpCompleteManual,
	"grant-approval":   mutation.OpGrantApproval,
	"revoke-approval":  mutation.OpRevokeApproval,
	"attach-evidence":  mutation.OpAttachEvidence,
}

type mutateFlags struct {
	role, requestID, target, expected, payload string
	// targets is every --target in order; only ticket hold takes more than
	// one (TCD-V0-173), and target stays the last one given.
	targets                          []string
	issuedAt                         string
	payloadFromStdin, help, template bool
	// skeleton is --skeleton, accepted only beside --template
	// (TCT-V0-001); it is not counted in others.
	skeleton bool
	// verbose is --verbose, accepted only beside --help (CAL-V0-170); the
	// in-parser mutation help is always the full text.
	verbose bool
	// batch is --batch, recognized only in a flag position, never as the
	// value of another flag (CAL-V0-106); only ticket refine accepts it.
	batch bool
	// notDuplicateOf is each --not-duplicate-of, accepted only by ticket
	// create (TCL-V0-004).
	notDuplicateOf []string
	// others counts the flags passed besides --help and --template, so
	// --template refuses any flag by presence, not by value.
	others int
}

// mutateCommand runs one ticket mutation against the real journal. The
// payload is the closed §3.3 payload for the verb; everything else in the
// envelope is composed here, so a caller never hand-writes the queue id,
// profile or timestamp.
func mutateCommand(env Env, verb string, args []string) *wire.Result {
	cmd := []string{"ticket", verb}
	operation := mutationVerbs[verb]
	flags, res := parseMutateFlags(cmd, args)
	if res != nil {
		return res
	}
	if flags.batch {
		if operation != mutation.OpRefine {
			return usage(cmd, "unknown flag --batch: only ticket refine has a batch form")
		}
		return refineBatchCommand(env, cmd, flags)
	}
	if flags.help {
		return mutationHelp(cmd, operation)
	}
	if flags.template {
		if operation != mutation.OpCreate {
			return usage(cmd, "--template is available only for ticket create")
		}
		if flags.others != 0 {
			return usage(cmd, "--template takes no other flag")
		}
		if flags.skeleton {
			return createSkeleton(env, cmd)
		}
		return createTemplate(env, cmd)
	}
	if flags.skeleton {
		return usage(cmd, "--skeleton is accepted only with --template")
	}
	if len(flags.targets) > 1 && operation != mutation.OpHold {
		return usage(cmd, "--target names one ticket: only ticket hold and ticket note set take several targets")
	}
	if len(flags.targets) > 1 && flags.expected != "" {
		return usage(cmd, "--expected-revision names one revision: with several --target values hold reads each target's revision itself")
	}
	actor, err := initActor(flags.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	if flags.requestID == "" {
		return usage(cmd, "--request-id is required: it is the idempotency key of this mutation")
	}
	if n := len(flags.requestID); n > wire.MaxRequestIDBytes && !flags.batch && len(flags.targets) <= 1 {
		return errorResult(cmd, wire.Errorf(wire.CodeLimitExceeded, "--request-id",
			"--request-id is %d bytes; it may be at most %d bytes (TCT-V0-991)", n, wire.MaxRequestIDBytes))
	}
	payload, err := readPayload(env, flags)
	if err == nil {
		raw := payload
		payload, err = mutation.CanonicalPayload(operation, payload)
		if err != nil && operation == mutation.OpCreate {
			err = diagnoseCreatePayload(env, raw, err)
		}
	}
	if err != nil {
		return errorResult(cmd, err)
	}
	if operation == mutation.OpCreate {
		if refused := createDuplicateGuard(env, cmd, flags, payload); refused != nil {
			return refused
		}
	}
	if operation == mutation.OpHold && len(flags.targets) > 1 {
		return multiTargetSubmit(env, cmd, operation, actor, flags, flags.targets, payload, true)
	}
	if operation == mutation.OpHold && flags.target != "" && flags.expected == "" {
		if flags.expected, err = readTargetRevision(env, flags.target); err != nil {
			return errorResult(cmd, err)
		}
	}
	return submitMutation(env, cmd, operation, actor, flags, payload)
}

// valuelessTemplateFlag reports whether arg is --template or --skeleton,
// bare or spelled with an =VALUE, and returns the flag name (TCT-V0-990).
func valuelessTemplateFlag(arg string) (string, bool) {
	for _, name := range []string{"--template", "--skeleton"} {
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return name, true
		}
	}
	return "", false
}

// templateValueRefusal is the usage text for a valued --template or
// --skeleton: both are switches, and the ticket kind lives in the payload
// (TCT-V0-990).
func templateValueRefusal(name string) string {
	return name + " takes no value; use ticket create --template for the canonical CREATE payload, or ticket create --template --skeleton for the bare payload, and set the ticket kind in the payload's kind field"
}

func parseMutateFlags(cmd []string, args []string) (mutateFlags, *wire.Result) {
	f := mutateFlags{role: "OWNER"}
	set := map[string]*string{
		"--role": &f.role, "--request-id": &f.requestID,
		"--target": &f.target, "--expected-revision": &f.expected, "--payload": &f.payload,
		"--issued-at": &f.issuedAt,
	}
	for i := 0; i < len(args); i++ {
		if args[i] == "--payload-stdin" {
			f.payloadFromStdin = true
			f.others++
			continue
		}
		if args[i] == "--help" {
			f.help = true
			continue
		}
		if args[i] == "--verbose" {
			f.verbose = true
			continue
		}
		if name, ok := valuelessTemplateFlag(args[i]); ok {
			if strings.Contains(args[i], "=") || (i+1 < len(args) && !strings.HasPrefix(args[i+1], "--")) {
				return f, usage(cmd, templateValueRefusal(name))
			}
			if name == "--template" {
				f.template = true
			} else {
				f.skeleton = true
			}
			continue
		}
		if args[i] == "--batch" {
			f.batch = true
			f.others++
			continue
		}
		if args[i] == "--not-duplicate-of" {
			if cmd[len(cmd)-1] != "create" {
				return f, usage(cmd, "unknown flag --not-duplicate-of: only ticket create runs the duplicate check")
			}
			if i+1 >= len(args) {
				return f, usage(cmd, args[i]+" needs a value")
			}
			i++
			f.notDuplicateOf = append(f.notDuplicateOf, args[i])
			f.others++
			continue
		}
		dest, ok := set[args[i]]
		if !ok {
			return f, usage(cmd, "unknown flag "+args[i])
		}
		if i+1 >= len(args) {
			return f, usage(cmd, args[i]+" needs a value")
		}
		i++
		*dest = args[i]
		if args[i-1] == "--target" && !slices.Contains(f.targets, args[i]) {
			f.targets = append(f.targets, args[i])
		}
		f.others++
	}
	if f.payload != "" && f.payloadFromStdin {
		return f, usage(cmd, "--payload and --payload-stdin are exclusive")
	}
	if f.verbose && !f.help {
		return f, usage(cmd, "--verbose is accepted only with --help")
	}
	return f, nil
}

// readPayload reads the caller's payload as any valid JSON value (V1-0750):
// whitespace, key order and escape form are the caller's choice, and the
// envelope re-encodes the value canonically, so the request digest of a
// pretty-printed payload equals that of its canonical form. Array order is
// kept; mutation payloads sort their set arrays in mutation.CanonicalPayload.
func readPayload(env Env, f mutateFlags) (wire.Value, error) {
	raw := f.payload
	if f.payloadFromStdin {
		data, err := io.ReadAll(io.LimitReader(env.Stdin, int64(wire.MaxTicketFileBytes)+1))
		if err != nil {
			return wire.Value{}, err
		}
		if len(data) > wire.MaxTicketFileBytes {
			return wire.Value{}, wire.Errorf(wire.CodeLimitExceeded, "payload", "payload larger than a ticket file")
		}
		raw = string(data)
	}
	if strings.TrimSpace(raw) == "" {
		return wire.Value{}, wire.Errorf(wire.CodeMalformed, "payload", "no payload: pass a JSON object with --payload or --payload-stdin")
	}
	return wire.ParseInput([]byte(raw))
}

// buildEnvelope composes the closed §3.3 envelope. targetId and
// expectedRevision are null for CREATE, which names no prior record; a note
// operation may also leave expectedRevision null (ON-V0-003).
func buildEnvelope(operation, queueID string, actor mutation.Binding, f mutateFlags, payload wire.Value, now wire.Timestamp) ([]byte, error) {
	target := wire.Null()
	expected := wire.Null()
	if operation == mutation.OpCreate {
		if f.target != "" || f.expected != "" {
			return nil, wire.Errorf(wire.CodeMalformed, "targetId", "CREATE names no target or expected revision")
		}
	} else {
		if f.target == "" || (f.expected == "" && !mutation.IsNoteOperation(operation) && !mutation.IsReviewOperation(operation)) {
			return nil, wire.Errorf(wire.CodeMalformed, "targetId", "%s needs --target and --expected-revision", operation)
		}
		target = wire.String(qualifyTicket(queueID, f.target))
		if f.expected != "" {
			expected = wire.String(f.expected)
		}
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(mutation.Profile))
	o.Set("requestId", wire.String(f.requestID))
	actorValue := wire.NewObject()
	actorValue.Set("id", wire.String(actor.ID))
	actorValue.Set("role", wire.String(actor.Role))
	o.Set("actor", wire.ObjectValue(actorValue))
	o.Set("queueId", wire.String(queueID))
	o.Set("targetId", target)
	o.Set("expectedRevision", expected)
	o.Set("operation", wire.String(operation))
	o.Set("payload", payload)
	o.Set("issuedAt", wire.String(string(now)))
	return wire.EncodeFile(wire.ObjectValue(o)), nil
}

// mutationHelp names one verb's flags and its closed payload keys, sorted as
// the canonical payload spells them, so a caller can compose a valid payload
// without the spec (V1-0329).
func mutationHelp(cmd []string, operation string) *wire.Result {
	o := wire.NewObject()
	o.Set("operation", wire.String(operation))
	o.Set("payloadKeys", wire.Strings(slices.Sorted(slices.Values(mutation.PayloadKeys[operation]))))
	o.Set("requestId", wire.String(fmt.Sprintf("--request-id is the idempotency key: at most %d bytes; a longer one is refused LIMIT_EXCEEDED naming the flag (TCT-V0-991)", wire.MaxRequestIDBytes)))
	target := " [--target TICKET|LOCAL --expected-revision N]"
	if operation == mutation.OpCreate {
		target = ""
		o.Set("localToken", wire.String("Optional payload member localToken chooses the queue-local ID, e.g. \"localToken\":\"BT-002\". It uses the existing LocalToken grammar and refuses collisions (including case-fold collisions). Omit it for automatic allocation. CREATE forbids --target and --expected-revision."))
		o.Set("optionalPayloadKeys", wire.Strings([]string{"localToken"}))
		o.Set("duplicateCheck", wire.String("CREATE first ranks the OPEN tickets against the payload (ticket similar). A match with similarity at least 0.60 over at least 3 distinct terms refuses DUPLICATE_ID naming each match and writes nothing: record a recurrence with ticket recur ID --context TEXT --request-id R, or pass --not-duplicate-of ID for each named ticket to file it anyway. A replayed --request-id skips the check (TCL-V0-004)."))
		o.Set("template", wire.String("corvint-tasks ticket create --template prints a canonical CREATE payload for this queue with each field's type, enum values and nullability; fill title, body and acceptanceCriteria and submit it with --payload-stdin. --template --skeleton prints only the complete payload as items[0] (every key, nulls, empty arrays, the native source; title empty so an unedited submit refuses). A MALFORMED CREATE payload is refused with every missing, unknown and invalid key and its expected shape in one error. It reads only the intent store and writes nothing."))
	}
	if operation == mutation.OpHold {
		target = " [--target TICKET|LOCAL ...] [--expected-revision N]"
		o.Set("multiTarget", wire.String(multiTargetHoldHelp))
		o.Set("optionalPayloadKeys", wire.Strings([]string{"condition"}))
		o.Set("condition", wire.String("Optional machine-checkable condition (SCC-V0-010): {\"kind\":\"COMMIT_CONTAINED\",\"ref\":REF,\"commit\":OID}, {\"kind\":\"PATH_EXISTS\",\"ref\":REF,\"path\":PATH} or {\"kind\":\"SYMBOL_DEFINED\",\"ref\":REF,\"path\":FILE,\"symbol\":IDENTIFIER}; REF is a full refname under refs/. queue status re-evaluates it at every read (holdConditions: UNBLOCKED, BLOCKED or UNKNOWN with the reason) and doctor names met and unknown conditions; a met condition never releases the hold by itself. A commit is contained by ancestry or by a patch-equivalent commit (source contains)."))
	}
	if operation == mutation.OpCreate || operation == mutation.OpRefine {
		o.Set("milestoneRule", wire.String("When policy sets milestones.required true, CREATE without a milestone and REFINE that sets milestone to null refuse VALIDATION_FAILED MILESTONE_REQUIRED (CAL-V0-195); existing records are never rewritten. Absent or false keeps milestone optional."))
	}
	if operation == mutation.OpAttachEvidence {
		o.Set("attachEvidence", wire.String("Records 1..16 sha256 digests (sorted unique; the CLI sorts them) and a nonblank reason of at most 512 bytes on an OPEN native ticket (TEA-V0-001). Revision advances by one; acceptanceRevision, status, gates, attempts and every other record field are unchanged, so attached evidence never satisfies a gate or acceptance criterion. A digest already attached at the current acceptanceRevision refuses DUPLICATE_ID; at most 32 entries per ticket. OWNER by default; OPERATOR only through an explicit policy row. Retry with the same --request-id and --issued-at to replay."))
	}
	if operation == mutation.OpCompleteManual {
		o.Set("completeManual", wire.String(completeManualHelp))
	}
	usageText := "corvint-tasks " + strings.Join(cmd, " ") + " --request-id ID (--payload JSON | --payload-stdin)" + target + " [--issued-at TS] [--role ROLE]"
	if operation == mutation.OpCreate {
		usageText += " [--not-duplicate-of TICKET|LOCAL ...]; corvint-tasks " + strings.Join(cmd, " ") + " --template [--skeleton]"
	}
	if operation == mutation.OpRefine {
		usageText += "; corvint-tasks " + strings.Join(cmd, " ") + " --batch --request-id ID (--payload-stdin | --payload JSON) [--issued-at TS] [--role ROLE]"
		o.Set("batch", wire.String(batchHelp))
	}
	o.Set("usage", wire.String(usageText))
	o.Set("note", wire.String("the payload is a JSON object with exactly these keys; CREATE may add localToken, HOLD may add condition, and REFINE takes a non-empty subset. The CLI canonicalizes it before the request digest (sorted keys, compact separators, literal UTF-8): whitespace, object key order and escape form are free, set arrays such as labels and touchPaths are sorted (duplicates refuse), and ordered arrays such as acceptanceCriteria and dependencies keep the order given; see docs/TASKS-EXTERNAL-AGENTS.md"))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
}

// mutateResult renders one applied mutation. A replay and a fresh commit are
// reported distinctly: both are OK, but only one wrote a receipt.
func mutateResult(cmd []string, report *store.Report) *wire.Result {
	o := wire.NewObject()
	o.Set("outcome", wire.String(report.Outcome.Outcome))
	o.Set("receipt", wire.String(report.Receipt))
	o.Set("replayed", wire.Bool(report.Outcome.Replayed))
	o.Set("ticketId", wire.String(report.Ticket))
	o.Set("releaseId", wire.String(report.Release))
	o.Set("resultingRevision", nullableCount(report.Outcome.ResultingRevision))
	o.Set("resultingAcceptanceRevision", nullableCount(report.Outcome.ResultingAcceptanceRevision))
	o.Set("actorAuthentication", wire.String(report.Coverage.ActorAuthentication))
	o.Set("durability", wire.String(report.Coverage.Durability))
	res := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
	if report.Outcome.Outcome != mutation.OutcomeCompleted {
		res.Outcome = wire.OutcomeRefused
		res.Codes = report.Outcome.Codes
		if report.Detail != "" {
			res.Warnings = append(res.Warnings, prose(report.Detail))
		}
	}
	if report.Redone {
		res.Warnings = append(res.Warnings,
			"a receipt left pending by an interrupted run was completed before this mutation (§5.2 redo)")
	}
	if report.Kind == "NoChange" {
		if len(report.Reaped) == 0 {
			res.Warnings = append(res.Warnings, "the mutation changed nothing; no receipt was written")
		} else {
			res.Warnings = append(res.Warnings, reapSurveyWarning(len(report.Reaped)))
		}
	}
	res.Warnings = append(res.Warnings,
		"the actor binding is a recorded local-operator claim, not an authentication (decision 0003); a real queue still needs the §7.4 cutover record")
	return res
}

func reapSurveyWarning(n int) string {
	if n == 1 {
		return "the reap survey wrote no receipt; 1 per-attempt reap transaction completed"
	}
	return fmt.Sprintf("the reap survey wrote no receipt; %d per-attempt reap transactions completed", n)
}

func nullableCount(c *wire.Count) wire.Value {
	if c == nil {
		return wire.Null()
	}
	return wire.String(string(*c))
}

// submitMutation composes the envelope around an already-read payload and
// commits it through the §5.2 writer; mutateCommand and `ticket note` share it.
func submitMutation(env Env, cmd []string, operation string, actor mutation.Binding, flags mutateFlags, payload wire.Value) *wire.Result {
	return submitMutationContext(writerContext(), env, cmd, operation, actor, flags, payload)
}

// submitMutationContext is submitMutation under a caller's writer context,
// which may carry write-time inputs such as an obligation report check.
func submitMutationContext(ctx context.Context, env Env, cmd []string, operation string, actor mutation.Binding, flags mutateFlags, payload wire.Value) *wire.Result {
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	store0, err := intent.Load(repo.IntentRoot())
	if err != nil {
		return errorResult(cmd, err)
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	// The request digest is the digest of the envelope bytes, and issuedAt is
	// one of them, so a retry replays only when it reproduces the same
	// timestamp. A first issue defaults to now; a retry passes --issued-at.
	issued := now
	if flags.issuedAt != "" {
		if issued, err = wire.ParseTimestamp("issuedAt", flags.issuedAt); err != nil {
			return errorResult(cmd, err)
		}
	}
	envelope, err := buildEnvelope(operation, store0.Queue.QueueID.Raw, actor, flags, payload, issued)
	if err != nil {
		return errorResult(cmd, err)
	}
	var overridden []ticket.Blocker
	if operation == mutation.OpCompleteManual {
		if overridden = failedIntentChecks(store0, flags.target); len(overridden) > 0 && blankReason(payload) {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "/payload/reason", "complete-manual of a ticket whose intent checks FAILED (%s) needs a nonblank reason recording why it is closed despite them (TCD-V0-176)", blockerCodes(overridden)))
		}
	}
	report, err := store.Mutate(ctx, repo, actor, envelope, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	res := mutateResult(cmd, report)
	if w := attemptLiveWarning(env, res, store0.Queue.QueueID.Raw, flags.target); w != "" {
		res.Warnings = append(res.Warnings, w)
	}
	if report.Outcome.Outcome == mutation.OutcomeRequestIDConflict && flags.issuedAt == "" {
		res.Warnings = append(res.Warnings, defaultedIssuedAtConflict(repo, store0, operation, actor, flags, payload, issued))
	}
	if report.Kind == "Transaction" && report.Outcome.Outcome == mutation.OutcomeCompleted {
		if w := unboundedEffectsWarning(store0, operation, payload, report.Ticket); w != "" {
			res.Warnings = append(res.Warnings, w)
		}
		if report.MutationEffects != nil {
			res.Warnings = append(res.Warnings, store.EffectsDirectoryKeyWarnings(env.Cwd, "HEAD", *report.MutationEffects)...)
		}
	}
	if operation == mutation.OpCompleteManual && report.Kind == "Transaction" && report.Outcome.Outcome == mutation.OutcomeCompleted {
		res.Warnings = append(res.Warnings, unmetDependencyWarnings(store0.Inventory, report.Ticket)...)
		if w := overriddenChecksWarning(overridden); w != "" {
			res.Warnings = append(res.Warnings, w)
		}
	}
	return res
}

// completeManualHelp states the complete-manual payload limits the writer
// enforces (TCD-V0-177); they differ from attach-evidence's.
var completeManualHelp = fmt.Sprintf("Payload {\"evidence\":[...],\"reason\":TEXT} on an OPEN ticket. evidence is a JSON array of sha256 digests, each exactly 64 lowercase hex characters; prose, URLs, 40-hex Git commit ids and uppercase hex refuse MALFORMED at /payload/evidence/N. It is a set: the CLI sorts it and a duplicate refuses. It may be empty and has no count cap beyond the %d-byte request envelope. Each digest should identify evidence a reader can find again: a ticket show suggestedEvidence digest (ERG-V0-011), an attach-evidence digest, or the sha256 of a merge commit or CI run URL string as ticket batch-close records. reason is prose of 0..%d bytes (TAB, LF and CR allowed; control and bidi code points refuse); a longer reason refuses LIMIT_EXCEEDED. When the ticket's intent checks FAILED (ticket show intentChecks), reason must be nonblank or the CLI refuses MALFORMED at /payload/reason and writes nothing; a committed close then warns with each overridden check (TCD-V0-176). attach-evidence's 1..16 digests and 512-byte reason are its own limits (TEA-V0-001), not these.", wire.MaxMutationEnvelopeBytes, wire.MaxProseBytes)

// failedIntentChecks is the TCD-V0-176 read of a complete-manual target: the
// blockers of an OPEN ticket whose intent checks FAILED, else nil. A closed or
// unknown target reads nil so a replay or a refused write behaves as before.
func failedIntentChecks(st *intent.Store, target string) []ticket.Blocker {
	id := qualifyTicket(st.Queue.QueueID.Raw, target)
	if rec, ok := st.Inventory.Get(id); !ok || rec.Status != ticket.StatusOpen {
		return nil
	}
	v, ok := st.Inventory.View(id, st.Context())
	if !ok || v.IntentChecks != "FAILED" {
		return nil
	}
	return v.Blockers
}

func blankReason(payload wire.Value) bool {
	if payload.Obj == nil {
		return false
	}
	r, ok := payload.Obj.Get("reason")
	return ok && strings.TrimSpace(r.Str) == ""
}

func blockerCodes(bs []ticket.Blocker) string {
	var codes []string
	for _, b := range bs {
		if !slices.Contains(codes, b.Code) {
			codes = append(codes, b.Code)
		}
	}
	return strings.Join(codes, ", ")
}

// overriddenChecksWarning names the failed intent checks a committed
// complete-manual closed over (TCD-V0-176). DEPENDENCY_UNSATISFIED is left
// to the CAL-V0-186 warning, which names each dependency's status.
func overriddenChecksWarning(bs []ticket.Blocker) string {
	var parts []string
	for _, b := range bs {
		if b.Code == wire.CodeDependencyUnsatisfied {
			continue
		}
		part := b.Code + ": " + b.Detail
		if b.TicketID != "" {
			part = b.Code + " (" + b.TicketID + "): " + b.Detail
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return ""
	}
	return "COMPLETE_MANUAL overrode failed intent checks (TCD-V0-176): " + strings.Join(parts, "; ")
}

// attemptLiveWarning is the ATR-V0-190 detail for an ATTEMPT_LIVE refusal
// whose writer route read no attempt record: a read after the refusal names
// the ticket's live attempt, its holder, holder status and recovery command.
// It is advisory; "" when the refusal is another code or the read fails.
func attemptLiveWarning(env Env, res *wire.Result, queueID, target string) string {
	live := false
	for _, code := range res.Codes {
		live = live || code == wire.CodeAttemptLive
	}
	if !live || target == "" {
		return ""
	}
	detail := ""
	_, err := withStore(env, func(rc *readCtx) error {
		if rc.journalAbsent {
			return nil
		}
		in, _, err := planInput(rc)
		if err != nil {
			return err
		}
		in.Now = wire.Timestamp(time.Now().UTC().Format("2006-01-02T15:04:05Z"))
		if cur := transaction.CurrentAttempt(in, qualifyTicket(queueID, target)); cur != nil {
			detail = "blocking " + cur.Detail()
		}
		return nil
	})
	if err != nil {
		return ""
	}
	return detail
}

// WarningEffectsUnbounded prefixes the CAL-V0-192 warning, so a caller can
// match it without parsing the prose that follows.
const WarningEffectsUnbounded = "EFFECTS_UNBOUNDED"

// unboundedEffectsWarning is the CAL-V0-192 warning for a fresh CREATE,
// REFINE or SET_EFFECTS commit whose resulting effects leave the ticket with
// the WHOLE_REPOSITORY fallback scope, or "" otherwise. CREATE and
// SET_EFFECTS read the effects from the committed payload; REFINE never
// changes effects, so it reads the record loaded before the write.
func unboundedEffectsWarning(st *intent.Store, operation string, payload wire.Value, ticketID string) string {
	var e ticket.Effects
	switch operation {
	case mutation.OpCreate, mutation.OpSetEffects:
		var ok bool
		if e, ok = mutation.PayloadEffects(operation, payload); !ok {
			return ""
		}
	case mutation.OpRefine:
		rec, ok := st.Inventory.Get(ticketID)
		if !ok {
			return ""
		}
		e = rec.Effects
	default:
		return ""
	}
	if !transaction.UnboundedEffects(e) {
		return ""
	}
	return fmt.Sprintf("%s: ticket %s declares no PATH scope under coverage QUALIFIED (coverage %s, %d touchPaths, %d resources), so plan and claim reserve WHOLE_REPOSITORY and it will serialize against every other attempt (policy serialFallback %s); declare its scope with ticket set-effects, or claim a committed change with --scope-from-diff REV (TCD-V0-174)",
		WarningEffectsUnbounded, ticketID, e.Coverage, len(e.TouchPaths), len(e.Resources), st.Policy.SerialFallback)
}

// unmetDependencyWarnings names each COMPLETED-obligation dependency that a
// committed COMPLETE_MANUAL overrode (CAL-V0-186). It reads the intent store
// the command loaded before the write; the receipt and journal are unchanged.
func unmetDependencyWarnings(inv *ticket.Inventory, target string) []string {
	var out []string
	for _, d := range inv.UnmetCompletedDependencies(target) {
		status := d.Status
		if status == "" {
			status = "not found"
		}
		out = append(out, fmt.Sprintf("COMPLETE_MANUAL overrode an unsatisfied dependency: %s (obligation COMPLETED) is %s", d.TicketID, status))
	}
	return out
}
