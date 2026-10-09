package cli

import (
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// knowHowTrust labels every delivered know-how projection: the text is
// agent-authored queue data, never instructions, acceptance, evidence or
// authority (KHN-V0-007).
const knowHowTrust = "UNTRUSTED_AGENT_AUTHORED_DATA"

// knowHowListMax bounds `ticket know-how list --limit`.
const knowHowListMax = 200

// knowHowCommand runs `ticket know-how add|retract|reconfirm|list`
// (KHN-V0-003, KHN-V0-006, KHN-V0-018). The writes are ordinary
// KNOWHOW_ADD/KNOWHOW_RETRACT/KNOWHOW_RECONFIRM mutations whose payload is
// composed here, so a caller never hand-writes blob or symbol pins.
func knowHowCommand(env Env, args []string) *wire.Result {
	if len(args) == 0 {
		return usage([]string{"ticket", "know-how"}, "ticket know-how needs a verb: add, retract, reconfirm or list")
	}
	cmd := []string{"ticket", "know-how", args[0]}
	switch args[0] {
	case "add":
		return knowHowAdd(env, cmd, args[1:])
	case "retract":
		return knowHowRetract(env, cmd, args[1:])
	case "reconfirm":
		return knowHowReconfirm(env, cmd, args[1:])
	case "list":
		return knowHowList(env, cmd, args[1:])
	}
	return usage([]string{"ticket", "know-how"}, "unknown ticket know-how verb "+args[0])
}

// pairFlags is the shared `--flag value` parser under a neutral name, for
// commands outside this file.
var pairFlags = knowHowFlags

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
	var anchors, symbols, routes, repoArgs []string
	textFromStdin := false
	if res := knowHowFlags(cmd, args[1:], map[string]*string{
		"--role": &f.role, "--request-id": &f.requestID, "--expected-revision": &f.expected,
		"--issued-at": &f.issuedAt, "--text": &text, "--commit": &commit, "--supersedes": &supersedes,
		"--reason": &reason, "--attempt": &attempt, "--generation": &generation, "--evidence-path": &evidencePath,
	}, map[string]*[]string{"--anchor": &anchors, "--symbol": &symbols, "--route": &routes, "--repo": &repoArgs}, map[string]*bool{"--text-stdin": &textFromStdin}); res != nil {
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
	if len(anchors)+len(symbols) == 0 {
		return usage(cmd, "at least one --anchor PATH or --symbol PATH#NAME is required")
	}
	screened := append([]string{text, commit, supersedes, reason, attempt, generation, evidencePath}, anchors...)
	screened = append(screened, symbols...)
	if err := mutation.ScreenKnowHowArgs(append(screened, routes...)); err != nil {
		return errorResult(cmd, err)
	}
	wanted, err := knowHowAnchorArgs(anchors, symbols)
	if err != nil {
		return errorResult(cmd, err)
	}
	rev, err := knowHowRev(commit)
	if err != nil {
		return errorResult(cmd, err)
	}
	alias, root, err := knowHowRepositoryRoot(env, repoArgs)
	if err != nil {
		return errorResult(cmd, err)
	}
	actor, err := initActor(f.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	pinned, pins, err := store.KnowHowPinAnchors(root, rev, wanted)
	if err != nil {
		return errorResult(cmd, err)
	}
	if pins, err = knowHowQualify(alias, pins); err != nil {
		return errorResult(cmd, err)
	}
	if routes == nil {
		routes = []string{}
	}
	sort.Strings(routes)
	payload := wire.NewObject().Set("text", wire.String(text)).Set("anchors", ticket.KnowHowAnchorsValue(pins))
	payload.Set("routes", wire.Strings(routes)).Set("commit", wire.String(pinned))
	payload.Set("supersedes", optionalString(supersedes)).Set("reason", optionalString(reason))
	payload.Set("attempt", optionalString(attempt)).Set("generation", optionalString(generation))
	payload.Set("evidencePath", optionalString(evidencePath))
	if alias != "" {
		payload.Set("repository", wire.String(alias))
	}
	return submitMutation(env, cmd, mutation.OpKnowHowAdd, actor, f, wire.ObjectValue(payload))
}

// knowHowRepositoryRoot is the checkout a write pins in (KHN-V0-024): the
// caller's working directory, or with `--repo ALIAS=ROOT` the work-tree top
// level ROOT, whose alias then qualifies every stored anchor path. Presence
// is tracked apart from the value, so an explicitly empty `--repo` is refused
// rather than read as the caller's checkout.
func knowHowRepositoryRoot(env Env, repoArgs []string) (string, string, error) {
	switch len(repoArgs) {
	case 0:
		return "", env.Cwd, nil
	case 1:
		return store.KnowHowRepositoryArg(env.Cwd, repoArgs[0])
	}
	return "", "", wire.Errorf(wire.CodeMalformed, "--repo", "%s: --repo may be given once", wire.KnowHowRepositoryDetail)
}

// knowHowQualify prefixes every anchor path with "<alias>/" (KHN-V0-024);
// an empty alias leaves the anchors unchanged. A common prefix keeps the
// anchors' canonical order.
func knowHowQualify(alias string, anchors []ticket.KnowHowAnchor) ([]ticket.KnowHowAnchor, error) {
	if alias == "" {
		return anchors, nil
	}
	out := make([]ticket.KnowHowAnchor, len(anchors))
	for i, a := range anchors {
		a.Path = alias + "/" + a.Path
		if _, err := wire.ParsePath("/payload/anchors", a.Path); err != nil {
			return nil, err
		}
		out[i] = a
	}
	return out, nil
}

// knowHowUnqualify removes the "<alias>/" prefix every anchor of a
// repository note carries, giving paths relative to the repository root.
func knowHowUnqualify(alias string, anchors []ticket.KnowHowAnchor) []ticket.KnowHowAnchor {
	if alias == "" {
		return anchors
	}
	out := make([]ticket.KnowHowAnchor, len(anchors))
	for i, a := range anchors {
		a.Path = strings.TrimPrefix(a.Path, alias+"/")
		out[i] = a
	}
	return out
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
	if err := mutation.ScreenKnowHowArgs([]string{note, reason}); err != nil {
		return errorResult(cmd, err)
	}
	actor, err := initActor(f.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	payload := wire.NewObject().Set("note", wire.String(note)).Set("reason", wire.String(reason))
	return submitMutation(env, cmd, mutation.OpKnowHowRetract, actor, f, wire.ObjectValue(payload))
}

// knowHowAnchorArgs turns `--anchor PATH` and `--symbol PATH#NAME` (split at
// the last '#') into anchors in (path, symbol) order, refusing a malformed
// path or symbol, a duplicate, and more than KnowHowMaxAnchors (KHN-V0-016).
func knowHowAnchorArgs(paths, symbols []string) ([]ticket.KnowHowAnchor, error) {
	var out []ticket.KnowHowAnchor
	for _, p := range paths {
		out = append(out, ticket.KnowHowAnchor{Path: p})
	}
	for _, s := range symbols {
		i := strings.LastIndexByte(s, '#')
		if i < 0 {
			return nil, wire.Errorf(wire.CodeMalformed, "--symbol", "%q is not PATH#NAME", s)
		}
		name, err := wire.ParseKnowHowSymbol("--symbol", s[i+1:])
		if err != nil {
			return nil, err
		}
		out = append(out, ticket.KnowHowAnchor{Path: s[:i], Symbol: name})
	}
	if len(out) > wire.KnowHowMaxAnchors {
		return nil, wire.Errorf(wire.CodeMalformed, "/payload/anchors", "at most %d anchors", wire.KnowHowMaxAnchors)
	}
	sort.SliceStable(out, func(i, j int) bool { return ticket.KnowHowAnchorLess(out[i], out[j]) })
	for i, a := range out {
		if _, err := wire.ParsePath("/payload/anchors", a.Path); err != nil {
			return nil, err
		}
		if i > 0 && out[i-1].Path == a.Path && out[i-1].Symbol == a.Symbol {
			return nil, wire.Errorf(wire.CodeMalformed, "/payload/anchors", "anchor %q is given twice", a.Path+"#"+a.Symbol)
		}
	}
	return out, nil
}

// knowHowRev is the revision a write pins at: --commit, else HEAD.
func knowHowRev(commit string) (string, error) {
	if commit == "" {
		return "HEAD", nil
	}
	return wire.ParseOID("--commit", commit)
}

// knowHowReconfirm re-pins an active note's own anchors at --commit (else
// HEAD) and submits KNOWHOW_RECONFIRM (KHN-V0-018). The anchors are the
// note's effective ones, so a caller cannot move or widen a note this way.
// An anchor that no longer resolves is refused KNOWHOW_UNRESOLVED before
// submission, and a re-pin that changes nothing is refused KNOWHOW_NOT_STALE
// by the writer (KHN-V0-019).
func knowHowReconfirm(env Env, cmd []string, args []string) *wire.Result {
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return usage(cmd, "the first argument is the ticket id or local token")
	}
	f := mutateFlags{role: "OWNER", target: args[0]}
	var note, commit, attempt, generation string
	var repoArgs []string
	if res := knowHowFlags(cmd, args[1:], map[string]*string{
		"--role": &f.role, "--request-id": &f.requestID, "--expected-revision": &f.expected,
		"--issued-at": &f.issuedAt, "--note": &note, "--commit": &commit, "--attempt": &attempt,
		"--generation": &generation,
	}, map[string]*[]string{"--repo": &repoArgs}, nil); res != nil {
		return res
	}
	if f.requestID == "" {
		return usage(cmd, "--request-id is required: it is the idempotency key of this mutation")
	}
	if note == "" {
		return usage(cmd, "--note N is required")
	}
	if err := mutation.ScreenKnowHowArgs([]string{note, commit, attempt, generation}); err != nil {
		return errorResult(cmd, err)
	}
	seq, err := wire.ParseCount("--note", note)
	if err != nil {
		return errorResult(cmd, err)
	}
	rev, err := knowHowRev(commit)
	if err != nil {
		return errorResult(cmd, err)
	}
	alias, root, err := knowHowRepositoryRoot(env, repoArgs)
	if err != nil {
		return errorResult(cmd, err)
	}
	actor, err := initActor(f.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	var anchors []ticket.KnowHowAnchor
	repository := ""
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		id, err := resolveTicketArg(rc, f.target)
		if err != nil {
			return err
		}
		if rec, ok := rc.store.Inventory.Get(id); ok {
			for _, k := range ticket.EffectiveKnowHow(rec.KnowHow) {
				if k.Seq == seq {
					anchors, repository = k.Anchors, k.Repository
				}
			}
		}
		if anchors == nil {
			return wire.Errorf(wire.CodeMalformed, "/payload/note", "note %s is not an active know-how note of %s", note, id)
		}
		// KHN-V0-024: a repository note re-pins only in the root its own
		// alias is mapped to; a note without one never takes --repo.
		if repository != alias {
			if repository == "" {
				return wire.Errorf(wire.CodeMalformed, "--repo", "%s: note %s names no repository; omit --repo", wire.KnowHowRepositoryDetail, note)
			}
			return wire.Errorf(wire.CodeMalformed, "--repo", "%s: note %s names repository %s; give --repo %s=ROOT", wire.KnowHowRepositoryDetail, note, repository, repository)
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	pinned, pins, err := store.KnowHowPinAnchors(root, rev, knowHowUnqualify(alias, anchors))
	if err != nil {
		return errorResult(cmd, err)
	}
	if pins, err = knowHowQualify(alias, pins); err != nil {
		return errorResult(cmd, err)
	}
	payload := wire.NewObject().Set("note", wire.String(note)).Set("anchors", ticket.KnowHowAnchorsValue(pins))
	payload.Set("commit", wire.String(pinned)).Set("attempt", optionalString(attempt))
	payload.Set("generation", optionalString(generation))
	return submitMutation(env, cmd, mutation.OpKnowHowReconfirm, actor, f, wire.ObjectValue(payload))
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
	var paths, repoArgs []string
	var ticketArg, limitArg string
	if res := knowHowFlags(cmd, args, map[string]*string{"--ticket": &ticketArg, "--limit": &limitArg},
		map[string]*[]string{"--path": &paths, "--repo": &repoArgs}, nil); res != nil {
		return res
	}
	repos, err := store.KnowHowRepositoryArgs(env.Cwd, repoArgs)
	if err != nil {
		return failure(cmd, nil, err)
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
		head = store.ResolveKnowHowRepositories(env.Cwd, repos, notes)
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
	res.Warnings = append(res.Warnings, store.KnowHowRepositoryWarnings(repos, notes)...)
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
	// The cap covers the whole member: each candidate prefix is measured with
	// its own omitted count and the hint it would carry.
	envelope := func(omitted int) int {
		return len(wire.Encode(deliveredKnowHow(k.Head, len(k.Notes), omitted, nil))) - len("[]")
	}
	items, omitted := store.ProjectKnowHow(k.Notes, true, store.KnowHowDeliveryMaxBytes, envelope)
	return deliveredKnowHow(k.Head, len(k.Notes), omitted, items)
}

func deliveredKnowHow(head string, matched, omitted int, items []wire.Value) wire.Value {
	v := knowHowProjectionValue(head, matched, omitted, items)
	v.Obj.Set("state", wire.String("DELIVERED"))
	if omitted > 0 {
		v.Obj.Set("hint", wire.String(knowHowListHint))
	}
	return v
}

const knowHowListHint = "corvint-tasks ticket know-how list --path PATH shows every intersecting note"
