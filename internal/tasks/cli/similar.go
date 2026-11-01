package cli

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/similar"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The duplicate check (docs/specs/corvint-tasks-cli-v0.md, TCL-V0-001 to
// TCL-V0-008): ticket similar, ticket digest, ticket recur and the guard
// ticket create runs before it submits.

const (
	similarDefaultLimit = 5
	similarMaxLimit     = 50
	// guardLimit bounds the candidates the create guard inspects.
	guardLimit = 5
)

// listFlags parses `--name VALUE` flags; names in repeat may repeat.
func listFlags(cmd []string, args []string, single []string, repeat []string, bools []string) (map[string][]string, map[string]bool, *wire.Result) {
	values := map[string][]string{}
	set := map[string]bool{}
	kind := map[string]int{}
	for _, n := range single {
		kind["--"+n] = 1
	}
	for _, n := range repeat {
		kind["--"+n] = 2
	}
	for _, n := range bools {
		kind["--"+n] = 3
	}
	for i := 0; i < len(args); i++ {
		k := kind[args[i]]
		name := strings.TrimPrefix(args[i], "--")
		switch {
		case k == 0:
			return nil, nil, usage(cmd, "unknown argument "+args[i])
		case k == 3:
			set[name] = true
			continue
		case i+1 >= len(args):
			return nil, nil, usage(cmd, args[i]+" needs a value")
		case k == 1 && len(values[name]) > 0:
			return nil, nil, usage(cmd, args[i]+" is given more than once")
		}
		i++
		values[name] = append(values[name], args[i])
	}
	return values, set, nil
}

// recordDoc is one ticket as the ranking sees it; ID is the local token, so
// text naming V1-0001 matches the ticket ID field.
func recordDoc(rec *ticket.Record) similar.Doc {
	body := ""
	if rec.Body != nil {
		body = *rec.Body
	}
	text := body
	if len(rec.AcceptanceCriteria) > 0 {
		text += "\n" + strings.Join(rec.AcceptanceCriteria, "\n")
	}
	return similar.Doc{ID: rec.TicketID.Local, Title: rec.Title, Status: rec.Status, Body: text, Labels: rec.Labels, Paths: rec.Effects.TouchPaths, Recurrences: similar.Recurrences(body)}
}

// storeDocs are the store's tickets in ID order, with their full IDs.
func storeDocs(rc *readCtx) ([]similar.Doc, map[string]string) {
	ids := rc.store.Inventory.Sorted()
	docs := make([]similar.Doc, 0, len(ids))
	full := make(map[string]string, len(ids))
	for _, id := range ids {
		rec, _ := rc.store.Inventory.Get(id)
		docs = append(docs, recordDoc(rec))
		full[rec.TicketID.Local] = rec.TicketID.Raw
	}
	return docs, full
}

// payloadHasLabel reports whether a CREATE payload carries label.
func payloadHasLabel(v wire.Value, label string) bool {
	if v.Kind != wire.KindObject || v.Obj == nil {
		return false
	}
	labels, ok := v.Obj.Get("labels")
	if !ok || labels.Kind != wire.KindArray {
		return false
	}
	for _, l := range labels.Arr {
		if l.Kind == wire.KindString && l.Str == label {
			return true
		}
	}
	return false
}

// payloadQuery is a CREATE payload as a query: its title, then its body,
// acceptance criteria, labels and touch paths as the body (TCL-V0-001).
func payloadQuery(v wire.Value) similar.Query {
	if v.Kind != wire.KindObject || v.Obj == nil {
		return similar.Query{}
	}
	str := func(k string) string {
		f, ok := v.Obj.Get(k)
		if !ok || f.Kind != wire.KindString {
			return ""
		}
		return f.Str
	}
	list := func(f wire.Value, ok bool) []string {
		var out []string
		if ok && f.Kind == wire.KindArray {
			for _, e := range f.Arr {
				if e.Kind == wire.KindString {
					out = append(out, e.Str)
				}
			}
		}
		return out
	}
	parts := []string{str("body")}
	parts = append(parts, list(v.Obj.Get("acceptanceCriteria"))...)
	parts = append(parts, list(v.Obj.Get("labels"))...)
	if ef, ok := v.Obj.Get("effects"); ok && ef.Kind == wire.KindObject && ef.Obj != nil {
		parts = append(parts, list(ef.Obj.Get("touchPaths"))...)
	}
	return similar.Query{Title: str("title"), Body: strings.Join(parts, "\n")}
}

func statusFilter(statuses, labels []string) similar.Filter {
	return func(d *similar.Doc) bool {
		if len(statuses) > 0 && !contains(statuses, d.Status) {
			return false
		}
		for _, l := range labels {
			if !contains(d.Labels, l) {
				return false
			}
		}
		return true
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func decimal(x float64) wire.Value { return wire.String(strconv.FormatFloat(x, 'f', 4, 64)) }

// candidateValue is one TCL-V0-002 item.
func candidateValue(c similar.Candidate, full map[string]string) wire.Value {
	id := c.Doc.ID
	if f, ok := full[id]; ok {
		id = f
	}
	ev := make([]wire.Value, 0, len(c.Matches))
	for _, m := range c.Matches {
		o := wire.NewObject().Set("term", wire.String(m.Term)).Set("fields", wire.Strings(m.Fields)).Set("idf", decimal(m.IDF)).Set("tf", count(m.TF))
		ev = append(ev, wire.ObjectValue(o))
	}
	o := wire.NewObject()
	o.Set("ticketId", wire.String(id)).Set("title", wire.String(c.Doc.Title)).Set("status", wire.String(c.Doc.Status))
	o.Set("score", decimal(c.Score)).Set("similarity", decimal(c.Similarity)).Set("duplicate", wire.Bool(c.Duplicate()))
	o.Set("recurrences", count(c.Doc.Recurrences)).Set("reason", wire.String(c.Reason())).Set("evidence", wire.Array(ev...))
	return wire.ObjectValue(o)
}

// ticketSimilar is TCL-V0-001, TCL-V0-002 and TCL-V0-006.
func ticketSimilar(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "similar"}
	fl, set, res := listFlags(cmd, args, []string{"text", "limit", "digest"}, []string{"status", "label"}, []string{"payload-stdin"})
	if res != nil {
		return res
	}
	text, hasText := fl["text"], len(fl["text"]) > 0
	if hasText == set["payload-stdin"] {
		return usage(cmd, "ticket similar takes exactly one of --text TEXT or --payload-stdin")
	}
	limit := similarDefaultLimit
	if l := fl["limit"]; len(l) > 0 {
		n, err := strconv.Atoi(l[0])
		if err != nil || n < 1 || n > similarMaxLimit {
			return usage(cmd, "--limit must be an integer from 1 to 50")
		}
		limit = n
	}
	statuses := fl["status"]
	if len(statuses) == 0 {
		statuses = []string{ticket.StatusOpen}
	}
	var query similar.Query
	if hasText {
		query = similar.Query{Title: text[0]}
	} else {
		payload, err := readPayload(env, mutateFlags{payloadFromStdin: true})
		if err != nil {
			return errorResult(cmd, err)
		}
		query = payloadQuery(payload)
	}
	if strings.TrimSpace(query.Title+query.Body) == "" {
		return usage(cmd, "the query has no text")
	}
	filter := statusFilter(statuses, fl["label"])
	if d := fl["digest"]; len(d) > 0 {
		raw, err := os.ReadFile(commandPath(env.Cwd, d[0]))
		if err != nil {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "--digest", "cannot read digest: %v", err))
		}
		dg, err := similar.ParseDigest(bytes.NewReader(raw))
		if err != nil {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "--digest", "%v", err))
		}
		docs := dg.Docs()
		items := rankItems(similar.NewIndex(docs), query, filter, limit, nil)
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: items, Untrusted: true, Warnings: []string{
			"ranked against digest " + d[0] + " (tree sha256 " + dg.Tree + ", " + strconv.Itoa(len(docs)) + " tickets, title field only); no repository, intent tree or journal was read: report a recurrence by ticket ID to the store's owner",
		}}
	}
	var items []wire.Value
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		docs, full := storeDocs(rc)
		items = rankItems(similar.NewIndex(docs), query, filter, limit, full)
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	out := success(cmd, rc)
	out.Items = items
	out.Untrusted = true
	return out
}

func rankItems(idx *similar.Index, q similar.Query, filter similar.Filter, limit int, full map[string]string) []wire.Value {
	items := []wire.Value{}
	for _, c := range idx.RankQuery(q, filter, limit) {
		items = append(items, candidateValue(c, full))
	}
	return items
}

// ticketDigest is TCL-V0-005.
func ticketDigest(env Env, args []string) int {
	cmd := []string{"ticket", "digest"}
	fl, _, res := listFlags(cmd, args, []string{"format", "since"}, []string{"status", "label"}, nil)
	if res != nil {
		return emit(env.Stdout, res)
	}
	since := ""
	if v := fl["since"]; len(v) > 0 {
		if _, err := time.Parse("2006-01-02", v[0]); err != nil {
			return emit(env.Stdout, usage(cmd, "--since must be a date YYYY-MM-DD"))
		}
		since = v[0]
	}
	format := "json"
	if f := fl["format"]; len(f) > 0 {
		format = f[0]
	}
	if format != "json" && format != "text" {
		return emit(env.Stdout, usage(cmd, "--format must be json or text"))
	}
	statuses, labels := fl["status"], fl["label"]
	if len(statuses) == 0 {
		statuses = []string{ticket.StatusOpen}
	}
	if len(labels) == 0 {
		labels = []string{"agent-memory"}
	}
	filter := statusFilter(statuses, labels)
	var entries []similar.DigestEntry
	var items []wire.Value
	tree := ""
	var notes []string
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		entries, items, notes = nil, nil, nil
		tree = string(rc.store.Tree.Sha256)
		if rc.bundle != nil {
			notes = []string{bundleWarning(rc)}
		}
		for _, id := range rc.store.Inventory.IDs() {
			rec, _ := rc.store.Inventory.Get(id)
			d := recordDoc(rec)
			// RFC 3339 UTC timestamps order as text, so a date prefix compares.
			if !filter(&d) || string(rec.UpdatedAt) < since {
				continue
			}
			entries = append(entries, similar.DigestEntry{ID: d.ID, Title: d.Title, Recurrences: d.Recurrences})
			o := wire.NewObject().Set("ticketId", wire.String(rec.TicketID.Raw)).Set("title", wire.String(d.Title)).Set("recurrences", count(d.Recurrences))
			items = append(items, wire.ObjectValue(o))
		}
		return nil
	})
	if err != nil {
		return emit(env.Stdout, failure(cmd, rc, err))
	}
	if format == "text" {
		if err := similar.WriteDigest(env.Stdout, tree, entries, notes...); err != nil {
			return 1
		}
		return 0
	}
	out := success(cmd, rc)
	if items == nil {
		items = []wire.Value{}
	}
	out.Items = items
	out.Untrusted = true
	return emit(env.Stdout, out)
}

// ticketRecur is TCL-V0-007: one REFINE of body appending a recurrence line.
func ticketRecur(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "recur"}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return usage(cmd, "ticket recur takes a ticket id first: ticket recur ID --context TEXT --request-id R")
	}
	arg := args[0]
	fl, _, res := listFlags(cmd, args[1:], []string{"context", "request-id", "issued-at", "role"}, nil, nil)
	if res != nil {
		return res
	}
	first := func(k string) string {
		if v := fl[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	if first("request-id") == "" {
		return usage(cmd, "--request-id is required: it is the idempotency key of this recurrence")
	}
	context := first("context")
	if why := similar.ValidContext(context); why != "" {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "--context", "%s", why))
	}
	role := first("role")
	if role == "" {
		role = "OWNER"
	}
	actor, err := initActor(role)
	if err != nil {
		return errorResult(cmd, err)
	}
	at := time.Now().UTC()
	if s := first("issued-at"); s != "" {
		if _, err := wire.ParseTimestamp("issuedAt", s); err != nil {
			return errorResult(cmd, err)
		}
		if at, err = time.Parse(time.RFC3339, s); err != nil {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "--issued-at", "%v", err))
		}
	}
	flags := mutateFlags{role: role, requestID: first("request-id"), issuedAt: first("issued-at")}
	var body, recorded string
	var refusal error
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		refusal, recorded = nil, ""
		id, err := resolveTicketArg(rc, arg)
		if err != nil {
			return err
		}
		rec, ok := rc.store.Inventory.Get(id)
		if !ok {
			return wire.Errorf(wire.CodeMalformed, "argv", "ticket %s is not in the queue", prose(arg))
		}
		if !rc.journalAbsent {
			idx := journal.RequestIndex{Reader: journal.Reader{Source: journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.PrimaryWorktree}, QueueID: rc.store.Queue.QueueID, PrimaryWorktree: rc.repo.PrimaryWorktree}}
			if _, found, err := idx.Lookup(flags.requestID); err == nil && found {
				// The index records no operation, so a replay is the same
				// ticket whose body already carries this context's line.
				if idx.TicketID != id || rec.Body == nil || !similar.HasRecurrence(*rec.Body, context) {
					refusal = wire.Errorf(wire.CodeRequestIDConflict, "--request-id", "request %s is already recorded for another mutation; use a new request ID", prose(flags.requestID))
					return nil
				}
				recorded = id
				return nil
			}
		}
		if rec.Status == ticket.StatusCompleted || rec.Status == ticket.StatusArchived {
			refusal = wire.Errorf(wire.CodeTicketState, "ticket", "ticket %s is %s: a recurrence of finished work reopens it first (`corvint-tasks ticket reopen --target %s --request-id R --payload-stdin`), or file a new ticket when the cause differs", id, rec.Status, rec.TicketID.Local)
			return nil
		}
		flags.target, flags.expected = id, string(rec.Revision)
		body = ""
		if rec.Body != nil {
			body = strings.TrimRight(*rec.Body, "\n") + "\n\n"
		}
		body += similar.RecurrenceLine(at.Format("2006-01-02"), context)
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	if recorded != "" {
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Warnings: []string{"request " + flags.requestID + " already recorded a recurrence on " + recorded + "; nothing was written"}}
	}
	if refusal != nil {
		return errorResult(cmd, refusal)
	}
	payload, err := mutation.CanonicalPayload(mutation.OpRefine, wire.ObjectValue(wire.NewObject().Set("body", wire.String(body))))
	if err != nil {
		return errorResult(cmd, err)
	}
	return submitMutation(env, cmd, mutation.OpRefine, actor, flags, payload)
}

// createDuplicateGuard is TCL-V0-004: nil when the create may proceed,
// otherwise the DUPLICATE_ID refusal naming each unnamed duplicate.
func createDuplicateGuard(env Env, cmd []string, flags mutateFlags, payload wire.Value) *wire.Result {
	if !payloadHasLabel(payload, "agent-memory") {
		return nil
	}
	var dups []similar.Candidate
	var full map[string]string
	named := map[string]bool{}
	replay := false
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		dups, replay = nil, false
		for k := range named {
			delete(named, k)
		}
		for _, arg := range flags.notDuplicateOf {
			id, err := resolveTicketArg(rc, arg)
			if err != nil {
				return err
			}
			named[id] = true
		}
		var docs []similar.Doc
		docs, full = storeDocs(rc)
		open := statusFilter([]string{ticket.StatusOpen}, nil)
		for _, c := range similar.NewIndex(docs).RankQuery(payloadQuery(payload), open, guardLimit+len(named)) {
			if c.Duplicate() && !named[full[c.Doc.ID]] {
				dups = append(dups, c)
			}
		}
		if len(dups) > 0 && !rc.journalAbsent {
			// A replay finds its own ticket; it must replay, not refuse.
			idx := journal.RequestIndex{Reader: journal.Reader{Source: journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.PrimaryWorktree}, QueueID: rc.store.Queue.QueueID, PrimaryWorktree: rc.repo.PrimaryWorktree}}
			if _, found, err := idx.Lookup(flags.requestID); err == nil && found {
				replay = true
			}
		}
		return nil
	})
	if wire.CodeOf(err) == wire.CodeRedoPending {
		// Store.Mutate recovers the pending redo; the guard cannot rank yet.
		return nil
	}
	if err != nil {
		return failure(cmd, rc, err)
	}
	if len(dups) == 0 || replay {
		return nil
	}
	items := make([]wire.Value, 0, len(dups))
	names := make([]string, 0, len(dups))
	for _, c := range dups {
		items = append(items, candidateValue(c, full))
		names = append(names, c.Doc.ID+" ("+prose(similar.CleanTitle(c.Doc.Title))+"; "+c.Reason()+")")
	}
	first := dups[0].Doc.ID
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeError, Codes: []string{wire.CodeDuplicateID}, NotRetryable: true, Items: items, Untrusted: true, Warnings: []string{
		prose("the new ticket matches OPEN ticket(s) " + strings.Join(names, "; ") + "; nothing was written. " +
			"For a recurrence run `corvint-tasks ticket recur " + first + " --context TEXT --request-id R`; to file it anyway pass `--not-duplicate-of ID` for each named ticket (TCL-V0-004)"),
	}}
}
