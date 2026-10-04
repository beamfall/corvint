package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
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
}

type mutateFlags struct {
	role, requestID, target, expected, payload string
	issuedAt                                   string
	payloadFromStdin, help                     bool
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
	if flags.help {
		return mutationHelp(cmd, operation)
	}
	actor, err := initActor(flags.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	if flags.requestID == "" {
		return usage(cmd, "--request-id is required: it is the idempotency key of this mutation")
	}
	payload, err := readPayload(env, flags)
	if err != nil {
		return errorResult(cmd, err)
	}
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
	report, err := store.Mutate(writerContext(), repo, actor, envelope, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	return mutateResult(cmd, report)
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
			continue
		}
		if args[i] == "--help" {
			f.help = true
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
	}
	if f.payload != "" && f.payloadFromStdin {
		return f, usage(cmd, "--payload and --payload-stdin are exclusive")
	}
	return f, nil
}

func readPayload(env Env, f mutateFlags) (wire.Value, error) {
	return readPayloadWithWhitespace(env, f, false)
}

func readPayloadWithWhitespace(env Env, f mutateFlags, compact bool) (wire.Value, error) {
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
		return wire.Value{}, wire.Errorf(wire.CodeMalformed, "payload", "no payload: pass --payload or --payload-stdin (canonical JSON: keys sorted, no extra whitespace)")
	}
	if compact {
		var buf bytes.Buffer
		if err := json.Compact(&buf, []byte(raw)); err != nil {
			return wire.Value{}, wire.Errorf(wire.CodeMalformed, "payload", "invalid JSON: %v", err)
		}
		raw = buf.String()
	}
	// The payload is a fragment, not a file: supply the framing LF the parser
	// requires. Canonicality is still enforced on the whole envelope.
	if !strings.HasSuffix(raw, "\n") {
		raw += "\n"
	}
	return wire.Parse([]byte(raw))
}

// buildEnvelope composes the closed §3.3 envelope. targetId and
// expectedRevision are null exactly for CREATE, which names no prior record.
func buildEnvelope(operation, queueID string, actor mutation.Binding, f mutateFlags, payload wire.Value, now wire.Timestamp) ([]byte, error) {
	target := wire.Null()
	expected := wire.Null()
	if operation == mutation.OpCreate {
		if f.target != "" || f.expected != "" {
			return nil, wire.Errorf(wire.CodeMalformed, "targetId", "CREATE names no target or expected revision")
		}
	} else {
		if f.target == "" || f.expected == "" {
			return nil, wire.Errorf(wire.CodeMalformed, "targetId", "%s needs --target and --expected-revision", operation)
		}
		target = wire.String(qualifyTicket(queueID, f.target))
		expected = wire.String(f.expected)
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
	target := " [--target TICKET|LOCAL --expected-revision N]"
	if operation == mutation.OpCreate {
		target = ""
		o.Set("localToken", wire.String("Optional payload member localToken chooses the queue-local ID, e.g. \"localToken\":\"BT-002\". It uses the existing LocalToken grammar and refuses collisions (including case-fold collisions). Omit it for automatic allocation. CREATE forbids --target and --expected-revision."))
		o.Set("optionalPayloadKeys", wire.Strings([]string{"localToken"}))
	}
	o.Set("usage", wire.String("corvint-tasks "+strings.Join(cmd, " ")+" --request-id ID (--payload JSON | --payload-stdin)"+target+" [--issued-at TS] [--role ROLE]"))
	o.Set("note", wire.String("the payload is canonical JSON with exactly these keys: sorted object keys, no insignificant whitespace, literal UTF-8 instead of non-ASCII escape forms, and canonical-byte-sorted set arrays such as touchPaths; do not sort ordered arrays such as argv; CREATE may add localToken, and REFINE takes a non-empty subset; see docs/TASKS-EXTERNAL-AGENTS.md"))
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
