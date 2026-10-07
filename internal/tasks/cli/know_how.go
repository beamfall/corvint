package cli

import (
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// knowHowTrust labels every delivered know-how projection: the text is
// agent-authored queue data, never instructions, acceptance, evidence or
// authority (KHN-V0-007).
const knowHowTrust = "UNTRUSTED_AGENT_AUTHORED_DATA"

// knowHowListMax bounds `ticket know-how list --limit`.
const knowHowListMax = 200

// knowHowCommand runs `ticket know-how add|retract|list` (KHN-V0-003,
// KHN-V0-006). The writes are ordinary KNOWHOW_ADD/KNOWHOW_RETRACT mutations
// whose payload is composed here, so a caller never hand-writes blob pins.
func knowHowCommand(env Env, args []string) *wire.Result {
	if len(args) == 0 {
		return usage([]string{"ticket", "know-how"}, "ticket know-how needs a verb: add, retract or list")
	}
	cmd := []string{"ticket", "know-how", args[0]}
	switch args[0] {
	case "add":
		return knowHowAdd(env, cmd, args[1:])
	case "retract":
		return knowHowRetract(env, cmd, args[1:])
	case "list":
		return knowHowList(env, cmd, args[1:])
	}
	return usage([]string{"ticket", "know-how"}, "unknown ticket know-how verb "+args[0])
}

// knowHowFlags parses `--flag value` pairs; names in repeat may repeat and
// every other flag may appear once.
func knowHowFlags(cmd, args []string, single map[string]*string, repeat map[string]*[]string, switches map[string]*bool) *wire.Result {
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		name := args[i]
		if b, ok := switches[name]; ok {
			*b = true
			continue
		}
		dest, one := single[name]
		list, many := repeat[name]
		if !one && !many {
			return usage(cmd, "unknown flag "+name)
		}
		if i+1 >= len(args) {
			return usage(cmd, name+" needs a value")
		}
		i++
		if many {
			*list = append(*list, args[i])
			continue
		}
		if seen[name] {
			return usage(cmd, name+" may be given once")
		}
		seen[name] = true
		*dest = args[i]
	}
	return nil
}

func knowHowAdd(env Env, cmd []string, args []string) *wire.Result {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return usage(cmd, "the first argument is the ticket id or local token")
	}
	f := mutateFlags{role: "OWNER", target: args[0]}
	var text, commit, supersedes, reason, attempt, generation, evidencePath string
	var anchors, routes []string
	textFromStdin := false
	if res := knowHowFlags(cmd, args[1:], map[string]*string{
		"--role": &f.role, "--request-id": &f.requestID, "--expected-revision": &f.expected,
		"--issued-at": &f.issuedAt, "--text": &text, "--commit": &commit, "--supersedes": &supersedes,
		"--reason": &reason, "--attempt": &attempt, "--generation": &generation, "--evidence-path": &evidencePath,
	}, map[string]*[]string{"--anchor": &anchors, "--route": &routes}, map[string]*bool{"--text-stdin": &textFromStdin}); res != nil {
		return res
	}
	if f.requestID == "" {
		return usage(cmd, "--request-id is required: it is the idempotency key of this mutation")
	}
	if (text != "") == textFromStdin {
		return usage(cmd, "exactly one of --text and --text-stdin is required")
	}
	if textFromStdin {
		data, err := io.ReadAll(io.LimitReader(env.Stdin, int64(wire.KnowHowMaxTextBytes)+1))
		if err != nil {
			return errorResult(cmd, err)
		}
		text = string(data)
	}
	if len(anchors) == 0 {
		return usage(cmd, "at least one --anchor PATH is required")
	}
	sort.Strings(anchors)
	for _, a := range anchors {
		if _, err := wire.ParsePath("/payload/anchors", a); err != nil {
			return errorResult(cmd, err)
		}
	}
	rev := "HEAD"
	if commit != "" {
		oid, err := wire.ParseOID("--commit", commit)
		if err != nil {
			return errorResult(cmd, err)
		}
		rev = oid
	}
	actor, err := initActor(f.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	pinned, blobs, err := store.KnowHowPins(env.Cwd, rev, anchors)
	if err != nil {
		return errorResult(cmd, err)
	}
	anchorValues := make([]wire.Value, len(anchors))
	for i, a := range anchors {
		anchorValues[i] = wire.ObjectValue(wire.NewObject().Set("blob", wire.String(blobs[i])).Set("path", wire.String(a)))
	}
	if routes == nil {
		routes = []string{}
	}
	sort.Strings(routes)
	payload := wire.NewObject().Set("text", wire.String(text)).Set("anchors", wire.Array(anchorValues...))
	payload.Set("routes", wire.Strings(routes)).Set("commit", wire.String(pinned))
	payload.Set("supersedes", optionalString(supersedes)).Set("reason", optionalString(reason))
	payload.Set("attempt", optionalString(attempt)).Set("generation", optionalString(generation))
	payload.Set("evidencePath", optionalString(evidencePath))
	return submitMutation(env, cmd, mutation.OpKnowHowAdd, actor, f, wire.ObjectValue(payload))
}

func knowHowRetract(env Env, cmd []string, args []string) *wire.Result {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return usage(cmd, "the first argument is the ticket id or local token")
	}
	f := mutateFlags{role: "OWNER", target: args[0]}
	var note, reason string
	if res := knowHowFlags(cmd, args[1:], map[string]*string{
		"--role": &f.role, "--request-id": &f.requestID, "--expected-revision": &f.expected,
		"--issued-at": &f.issuedAt, "--note": &note, "--reason": &reason,
	}, nil, nil); res != nil {
		return res
	}
	if f.requestID == "" {
		return usage(cmd, "--request-id is required: it is the idempotency key of this mutation")
	}
	if note == "" || reason == "" {
		return usage(cmd, "--note N and --reason TEXT are required")
	}
	actor, err := initActor(f.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	payload := wire.NewObject().Set("note", wire.String(note)).Set("reason", wire.String(reason))
	return submitMutation(env, cmd, mutation.OpKnowHowRetract, actor, f, wire.ObjectValue(payload))
}

func optionalString(s string) wire.Value {
	if s == "" {
		return wire.Null()
	}
	return wire.String(s)
}

// knowHowList is the pure read of active notes (KHN-V0-006): filtered by
// anchor path (exact, or every anchor below a trailing-"/" prefix) and by
// home ticket, ordered like claim delivery, with freshness against the
// caller's committed HEAD. It takes no lock and writes nothing.
func knowHowList(env Env, cmd []string, args []string) *wire.Result {
	var paths []string
	var ticketArg, limitArg string
	if res := knowHowFlags(cmd, args, map[string]*string{"--ticket": &ticketArg, "--limit": &limitArg},
		map[string]*[]string{"--path": &paths}, nil); res != nil {
		return res
	}
	limit := 50
	if limitArg != "" {
		n, err := strconv.Atoi(limitArg)
		if err != nil || n < 1 || n > knowHowListMax || strconv.Itoa(n) != limitArg {
			return usage(cmd, "--limit must be 1.."+strconv.Itoa(knowHowListMax))
		}
		limit = n
	}
	for _, p := range paths {
		if _, err := wire.ParsePath("--path", p); err != nil {
			return failure(cmd, nil, err)
		}
	}
	var notes []store.KnowHowNote
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		home := ""
		if ticketArg != "" {
			id, err := resolveTicketArg(rc, ticketArg)
			if err != nil {
				return err
			}
			home = id
		}
		notes = store.SelectKnowHow(rc.store.Inventory, paths, home)
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	head := ""
	if len(notes) > 0 {
		head = store.ResolveKnowHowFreshness(env.Cwd, notes)
		store.SortKnowHow(notes)
	}
	shown := notes
	if len(shown) > limit {
		shown = shown[:limit]
	}
	items := make([]wire.Value, 0, len(shown))
	for _, n := range shown {
		items = append(items, store.KnowHowNoteValue(n, false))
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{knowHowProjectionValue(head, len(notes), len(notes)-len(shown), items)}
	res.Untrusted = true
	return res
}

func knowHowProjectionValue(head string, matched, omitted int, items []wire.Value) wire.Value {
	o := wire.NewObject().Set("trust", wire.String(knowHowTrust)).Set("head", stringOrNull(head))
	o.Set("matched", wire.String(strconv.Itoa(matched))).Set("omitted", wire.String(strconv.Itoa(omitted)))
	return wire.ObjectValue(o.Set("notes", wire.Array(items...)))
}

// claimedKnowHowValue is the `knowHow` member of a claim or claim-next
// result (KHN-V0-006): the compact notes intersecting the claimed ticket's
// touchPaths under the 2 KiB cap, with the count left out and the read that
// shows them all. An unreadable inventory is UNAVAILABLE, never an empty
// list.
func claimedKnowHowValue(k store.ClaimedKnowHow) wire.Value {
	if k.Err != nil {
		o := wire.NewObject().Set("trust", wire.String(knowHowTrust)).Set("state", wire.String("UNAVAILABLE"))
		return wire.ObjectValue(o.Set("code", wire.String(wire.CodeOf(k.Err))).Set("notes", wire.Null()))
	}
	items, omitted := store.ProjectKnowHow(k.Notes, true, store.KnowHowDeliveryMaxBytes)
	v := knowHowProjectionValue(k.Head, len(k.Notes), omitted, items)
	v.Obj.Set("state", wire.String("DELIVERED"))
	if omitted > 0 {
		v.Obj.Set("hint", wire.String("corvint-tasks ticket know-how list --path PATH shows every intersecting note"))
	}
	return v
}
