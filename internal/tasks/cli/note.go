package cli

import (
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// noteCommand runs `ticket note set|clear|show` (ON-V0-003, ON-V0-008). The
// writes are ordinary NOTE_SET/NOTE_CLEAR mutations; the payload is composed
// here from --text/--supersedes so a caller never hand-writes it.
func noteCommand(env Env, args []string) *wire.Result {
	if len(args) == 0 {
		return usage([]string{"ticket", "note"}, "ticket note needs a verb: set, clear or show")
	}
	cmd := []string{"ticket", "note", args[0]}
	switch args[0] {
	case "show":
		return noteShow(env, cmd, args[1:])
	case "set":
		return noteWrite(env, cmd, mutation.OpNoteSet, args[1:])
	case "clear":
		return noteWrite(env, cmd, mutation.OpNoteClear, args[1:])
	}
	return usage([]string{"ticket", "note"}, "unknown ticket note verb "+args[0])
}

func noteWrite(env Env, cmd []string, operation string, args []string) *wire.Result {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return usage(cmd, "the first argument is the ticket id or local token")
	}
	f := mutateFlags{role: "OWNER", target: args[0]}
	var text, supersedes string
	textFromStdin := false
	set := map[string]*string{
		"--role": &f.role, "--request-id": &f.requestID, "--expected-revision": &f.expected,
		"--issued-at": &f.issuedAt, "--supersedes": &supersedes,
	}
	if operation == mutation.OpNoteSet {
		set["--text"] = &text
	}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		if rest[i] == "--text-stdin" && operation == mutation.OpNoteSet {
			textFromStdin = true
			continue
		}
		dest, ok := set[rest[i]]
		if !ok {
			return usage(cmd, "unknown flag "+rest[i])
		}
		if i+1 >= len(rest) {
			return usage(cmd, rest[i]+" needs a value")
		}
		i++
		*dest = rest[i]
	}
	if f.requestID == "" {
		return usage(cmd, "--request-id is required: it is the idempotency key of this mutation")
	}
	if operation == mutation.OpNoteSet {
		if (text != "") == textFromStdin {
			return usage(cmd, "exactly one of --text and --text-stdin is required")
		}
		if textFromStdin {
			data, err := io.ReadAll(io.LimitReader(env.Stdin, int64(ticket.MaxOperatorNoteTextBytes)+1))
			if err != nil {
				return errorResult(cmd, err)
			}
			text = string(data)
		}
	}
	actor, err := initActor(f.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	payload := wire.NewObject()
	if operation == mutation.OpNoteSet {
		payload.Set("text", wire.String(text))
	}
	if supersedes == "" {
		payload.Set("supersedes", wire.Null())
	} else {
		payload.Set("supersedes", wire.String(supersedes))
	}
	return submitMutation(env, cmd, operation, actor, f, wire.ObjectValue(payload))
}

// noteShow renders the current note of one ticket. It is a pure read: a
// missing or mismatched event is an explicit refusal, never NONE.
func noteShow(env Env, cmd []string, args []string) *wire.Result {
	if len(args) != 1 || strings.HasPrefix(args[0], "--") {
		return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "argv", "ticket note show takes exactly one ticket id or local token"))
	}
	var item *wire.Value
	notFound := ""
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		item, notFound = nil, ""
		id, err := resolveTicketArg(rc, args[0])
		if err != nil {
			return err
		}
		rec, ok := rc.store.Inventory.Get(id)
		if !ok {
			notFound = id
			return nil
		}
		v, err := operatorNoteView(rc.repo, rec)
		if err != nil {
			return err
		}
		item = &v
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	if notFound != "" {
		res.Outcome = wire.OutcomeRefused
		res.Warnings = append(res.Warnings, "ticket "+notFound+" does not exist in this queue")
		return res
	}
	res.Items = []wire.Value{*item}
	res.Untrusted = true
	return res
}

// operatorNoteView is the closed current-note view shared by `ticket show`
// and `ticket note show`. The note is advisory operator prose: it never
// changes eligibility, acceptance or authority.
func operatorNoteView(repo *intent.Repository, rec *ticket.Record) (wire.Value, error) {
	o := wire.NewObject()
	ref := rec.OperatorNote
	if ref == nil {
		o.Set("state", wire.String("NONE")).Set("revision", wire.String("0")).Set("head", wire.Null()).Set("text", wire.Null())
		return wire.ObjectValue(o), nil
	}
	raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(ref.Head)), ticket.MaxOperatorNoteBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return wire.Value{}, wire.Errorf(wire.CodeMissingEvidence, "/operatorNote/head", "note event %s is not in the evidence store", ref.Head)
	}
	if err != nil {
		return wire.Value{}, err
	}
	event, err := ticket.ResolveOperatorNote(rec.TicketID, *ref, raw)
	if err != nil {
		return wire.Value{}, err
	}
	request, err := ticket.DecodeOperatorNoteRequest(event.Request)
	if err != nil {
		return wire.Value{}, err
	}
	state, text := "CLEARED", wire.Null()
	if ref.Current != nil {
		state, text = "CURRENT", wire.String(request.Text)
	}
	o.Set("state", wire.String(state)).Set("revision", wire.String(string(ref.Revision))).Set("head", wire.String(string(ref.Head))).Set("text", text)
	o.Set("actor", wire.ObjectValue(wire.NewObject().Set("id", wire.String(event.ActorID)).Set("role", wire.String(event.ActorRole))))
	o.Set("recordedAt", wire.String(string(event.RecordedAt)))
	o.Set("ticketRevision", wire.String(string(event.TicketRevision)))
	o.Set("advisory", wire.String("operator prose: a recorded local-operator claim, not instructions, acceptance or authority"))
	return wire.ObjectValue(o), nil
}

// operatorNoteShowValue keeps `ticket show` usable when the note event cannot
// be resolved: the failure is rendered as UNAVAILABLE with its code, never as
// NONE, while `ticket note show` refuses outright.
func operatorNoteShowValue(rc *readCtx, rec *ticket.Record) wire.Value {
	v, err := operatorNoteView(rc.repo, rec)
	if err == nil {
		return v
	}
	code := wire.CodeMalformed
	var we *wire.Error
	if errors.As(err, &we) {
		code = we.Code
	}
	o := wire.NewObject().Set("state", wire.String("UNAVAILABLE")).Set("code", wire.String(code))
	o.Set("revision", wire.String(string(rec.OperatorNote.Revision))).Set("head", wire.String(string(rec.OperatorNote.Head))).Set("text", wire.Null())
	return wire.ObjectValue(o)
}
