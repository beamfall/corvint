package cli

import (
	"io"
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

// findingFlags are the parsed work finding flags.
type findingFlags struct {
	payload, ingest, supersede, by string
	requestID, issuedAt, role      string
	newDespite                     []string
}

// workFinding is TAW-V0-010..012.
func workFinding(env Env, cmd []string, args []string) *wire.Result {
	fl, _, res := listFlags(cmd, args, []string{"payload", "ingest", "supersede", "by", "request-id", "issued-at", "role"}, []string{"new-despite"}, nil)
	if res != nil {
		return res
	}
	first := func(k string) string {
		if v := fl[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	f := findingFlags{payload: first("payload"), ingest: first("ingest"), supersede: first("supersede"), by: first("by"),
		requestID: first("request-id"), issuedAt: first("issued-at"), role: first("role"), newDespite: fl["new-despite"]}
	if f.role == "" {
		f.role = "OWNER"
	}
	modes := 0
	for _, m := range []string{f.payload, f.ingest, f.supersede} {
		if m != "" {
			modes++
		}
	}
	if modes != 1 || f.requestID == "" {
		return usage(cmd, "pass exactly one of --payload, --ingest or --supersede, and --request-id: "+commandUsage["work finding"])
	}
	if (f.by != "") != (f.supersede != "") {
		return usage(cmd, "--supersede DUP and --by KEEP go together")
	}
	if len(f.newDespite) > 0 && f.payload == "" {
		return usage(cmd, "--new-despite is accepted only with --payload")
	}
	if f.issuedAt != "" {
		if _, err := wire.ParseTimestamp("issuedAt", f.issuedAt); err != nil {
			return errorResult(cmd, err)
		}
	}
	switch {
	case f.supersede != "":
		return findingSupersede(env, cmd, f)
	case f.ingest != "":
		return findingIngest(env, cmd, f)
	}
	raw, err := readFindingSource(env, f.payload)
	if err != nil {
		return errorResult(cmd, err)
	}
	payload, err := wire.ParseInput(raw)
	if err == nil {
		payload, err = mutation.CanonicalPayload(mutation.OpCreate, payload)
	}
	if err != nil {
		return errorResult(cmd, err)
	}
	row, out := fileFinding(env, cmd, f, f.requestID, payload)
	if out != nil {
		return out
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{row}}
}

// readFindingSource reads FILE, or stdin for "-".
func readFindingSource(env Env, src string) ([]byte, error) {
	var r io.Reader
	if src == "-" {
		r = env.Stdin
	} else {
		path := src
		if !strings.HasPrefix(path, "/") {
			path = env.Cwd + "/" + path
		}
		fh, err := os.Open(path)
		if err != nil {
			return nil, wire.Errorf(wire.CodeMalformed, "payload", "%v", err)
		}
		defer fh.Close()
		r = fh
	}
	limit := int64(wire.MaxTicketFileBytes) * 64
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "payload", "finding input larger than %d bytes", limit)
	}
	return data, nil
}

// recordedRequest reports the ticket a request id already committed, so a
// rerun replays instead of ranking against the ticket it created.
func recordedRequest(env Env, requestID string) (string, bool, error) {
	var ticketID string
	var found bool
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		ticketID, found = "", false
		if rc.journalAbsent {
			return nil
		}
		idx := journal.RequestIndex{Reader: journal.Reader{Source: journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.PrimaryWorktree}, QueueID: rc.store.Queue.QueueID, PrimaryWorktree: rc.repo.PrimaryWorktree}}
		_, ok, err := idx.Lookup(requestID)
		if err != nil {
			return err
		}
		found, ticketID = ok, idx.TicketID
		return nil
	})
	_ = rc
	return ticketID, found, err
}

func findingRow(index int, disposition, ticketID, reason string, written bool) wire.Value {
	o := wire.NewObject().Set("disposition", wire.String(disposition)).Set("ticket", wire.String(ticketID)).Set("written", wire.Bool(written))
	if index >= 0 {
		o.Set("index", countValue(int64(index)))
	}
	if reason != "" {
		o.Set("reason", wire.String(reason))
	}
	return wire.ObjectValue(o)
}

func payloadString(v wire.Value, key string) string {
	if v.Kind != wire.KindObject || v.Obj == nil {
		return ""
	}
	if f, ok := v.Obj.Get(key); ok && f.Kind == wire.KindString {
		return f.Str
	}
	return ""
}

// setPayloadString returns a copy of a CREATE payload with key set.
func setPayloadString(v wire.Value, key, s string) (wire.Value, error) {
	obj := wire.NewObject()
	for _, k := range v.Obj.Keys {
		obj.Set(k, v.Obj.Vals[k])
	}
	obj.Set(key, wire.String(s))
	return mutation.CanonicalPayload(mutation.OpCreate, wire.ObjectValue(obj))
}

// fileFinding is TAW-V0-010 for one canonical CREATE payload: the similar
// check first, then attach (ticket recur) or create. A non-nil result is a
// refusal of the whole command.
func fileFinding(env Env, cmd []string, f findingFlags, requestID string, payload wire.Value) (wire.Value, *wire.Result) {
	if id, found, err := recordedRequest(env, requestID); err != nil {
		return wire.Value{}, errorResult(cmd, err)
	} else if found {
		return findingRow(-1, "REPLAYED", id, "request "+requestID+" is already recorded; nothing was written", false), nil
	}
	var dups []similar.Candidate
	var full map[string]string
	named := map[string]bool{}
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		dups = nil
		for k := range named {
			delete(named, k)
		}
		for _, arg := range f.newDespite {
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
			if c.Duplicate() {
				dups = append(dups, c)
			}
		}
		return nil
	})
	if err != nil {
		return wire.Value{}, failure(cmd, rc, err)
	}
	for _, c := range dups {
		id := full[c.Doc.ID]
		if named[id] {
			continue
		}
		// TAW-V0-010: an unnamed duplicate takes the finding as a recurrence.
		context := strings.TrimSpace(payloadString(payload, "title"))
		rargs := []string{c.Doc.ID, "--context", context, "--request-id", requestID, "--role", f.role}
		if f.issuedAt != "" {
			rargs = append(rargs, "--issued-at", f.issuedAt)
		}
		out := ticketRecur(env, rargs)
		if out.Outcome != wire.OutcomeOK {
			out.Command = cmd
			return wire.Value{}, out
		}
		return findingRow(-1, "ATTACHED", id, "similar "+strconv.FormatFloat(c.Similarity, 'f', 2, 64)+" to "+c.Doc.ID+"; recorded as a recurrence", true), nil
	}
	flags := mutateFlags{role: f.role, requestID: requestID, issuedAt: f.issuedAt}
	reason := ""
	if len(named) > 0 {
		var names []string
		for _, c := range dups {
			if id := full[c.Doc.ID]; named[id] {
				names = append(names, c.Doc.ID)
				flags.notDuplicateOf = append(flags.notDuplicateOf, id)
			}
		}
		if len(names) > 0 {
			body := strings.TrimRight(payloadString(payload, "body"), "\n")
			if body != "" {
				body += "\n\n"
			}
			body += "Filed despite similar " + strings.Join(names, ", ")
			var err error
			if payload, err = setPayloadString(payload, "body", body); err != nil {
				return wire.Value{}, errorResult(cmd, err)
			}
			reason = "filed despite similar " + strings.Join(names, ", ")
		}
	}
	actor, err := initActor(f.role)
	if err != nil {
		return wire.Value{}, errorResult(cmd, err)
	}
	out := submitMutation(env, []string{"ticket", "create"}, mutation.OpCreate, actor, flags, payload)
	if out.Outcome != wire.OutcomeOK {
		out.Command = cmd
		return wire.Value{}, out
	}
	created := ""
	if len(out.Items) > 0 {
		if v, ok := out.Items[0].Obj.Get("ticketId"); ok {
			created = v.Str
		}
	}
	return findingRow(-1, "CREATED", created, reason, true), nil
}

// findingIngest is TAW-V0-011: dedupe a lane's batch against itself, then
// file each survivor through TAW-V0-010 with request id R-<index>.
func findingIngest(env Env, cmd []string, f findingFlags) *wire.Result {
	raw, err := readFindingSource(env, f.ingest)
	if err != nil {
		return errorResult(cmd, err)
	}
	v, err := wire.ParseInput(raw)
	if err != nil {
		return errorResult(cmd, err)
	}
	if v.Kind != wire.KindArray || len(v.Arr) == 0 {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "--ingest", "the batch must be a non-empty JSON array of create payloads"))
	}
	date := time.Now().UTC().Format("2006-01-02")
	if f.issuedAt != "" {
		date = f.issuedAt[:10]
	}
	payloads := make([]wire.Value, len(v.Arr))
	for i, e := range v.Arr {
		if payloads[i], err = mutation.CanonicalPayload(mutation.OpCreate, e); err != nil {
			return errorResult(cmd, wire.Annotatef(err, "--ingest/"+strconv.Itoa(i), "finding %d", i))
		}
	}
	// Fold each later duplicate into its earliest survivor as a recurrence
	// line, before any write. The batch is ranked inside the store corpus so
	// its term weights match the store check; only batch docs can match.
	var storeCorpus []similar.Doc
	if rc, err := withInventoryStore(env, func(rc *readCtx) error {
		storeCorpus, _ = storeDocs(rc)
		return nil
	}); err != nil {
		return failure(cmd, rc, err)
	}
	inBatch := func(d *similar.Doc) bool { return strings.HasPrefix(d.ID, "batch:") }
	foldedInto := make([]int, len(payloads))
	var survivors []int
	for i, p := range payloads {
		foldedInto[i] = -1
		docs := append([]similar.Doc(nil), storeCorpus...)
		for _, j := range survivors {
			q := payloadQuery(payloads[j])
			docs = append(docs, similar.Doc{ID: "batch:" + strconv.Itoa(j), Title: q.Title, Status: ticket.StatusOpen, Body: q.Body})
		}
		if len(survivors) > 0 {
			for _, c := range similar.NewIndex(docs).RankQuery(payloadQuery(p), inBatch, 1) {
				if c.Duplicate() {
					foldedInto[i], _ = strconv.Atoi(strings.TrimPrefix(c.Doc.ID, "batch:"))
				}
			}
		}
		if foldedInto[i] < 0 {
			survivors = append(survivors, i)
			continue
		}
		j := foldedInto[i]
		context := strings.TrimSpace(payloadString(p, "title"))
		if why := similar.ValidContext(context); why != "" {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "--ingest/"+strconv.Itoa(i)+"/title", "%s", why))
		}
		body := strings.TrimRight(payloadString(payloads[j], "body"), "\n")
		if body != "" {
			body += "\n\n"
		}
		if payloads[j], err = setPayloadString(payloads[j], "body", body+similar.RecurrenceLine(date, context)); err != nil {
			return errorResult(cmd, err)
		}
	}
	var items []wire.Value
	var warnings []string
	filed := map[int]string{}
	for _, i := range survivors {
		row, out := fileFinding(env, cmd, f, targetRequestID(f.requestID, strconv.Itoa(i), strconv.Itoa(i)), payloads[i])
		if out != nil {
			// Earlier rows stay committed; a rerun with the same request id replays them.
			out.Items = append(items, out.Items...)
			return out
		}
		row.Obj.Set("index", countValue(int64(i)))
		if t, ok := row.Obj.Get("ticket"); ok {
			filed[i] = t.Str
		}
		items = append(items, row)
	}
	for i, j := range foldedInto {
		if j >= 0 {
			items = append(items, findingRow(i, "FOLDED", filed[j], "duplicate of batch finding "+strconv.Itoa(j)+"; recorded as its recurrence line", false))
		}
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: items, Warnings: warnings}
}

// findingSupersede is TAW-V0-012: REFINE supersededBy on DUP, then ARCHIVE
// DUP with a DUPLICATE reason code.
func findingSupersede(env Env, cmd []string, f findingFlags) *wire.Result {
	var dup, keep string
	var dupRec *ticket.Record
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		var err error
		if dup, err = resolveTicketArg(rc, f.supersede); err != nil {
			return err
		}
		if keep, err = resolveTicketArg(rc, f.by); err != nil {
			return err
		}
		rec, ok := rc.store.Inventory.Get(dup)
		if !ok {
			return wire.Errorf(wire.CodeMalformed, "--supersede", "ticket %s is not in the queue", prose(f.supersede))
		}
		if _, ok := rc.store.Inventory.Get(keep); !ok {
			return wire.Errorf(wire.CodeDependencyMissing, "--by", "ticket %s is not in the queue", prose(f.by))
		}
		dupRec = rec
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	if dup == keep {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "--by", "a ticket cannot supersede itself (TAW-V0-012)"))
	}
	already := dupRec.SupersededBy != nil && dupRec.SupersededBy.Raw == keep
	if dupRec.Status == ticket.StatusArchived && already {
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{findingRow(-1, "ALREADY_SUPERSEDED", dup, "superseded by "+keep, false)}}
	}
	if dupRec.Status != ticket.StatusOpen && dupRec.Status != ticket.StatusHeld && dupRec.Status != ticket.StatusDraft {
		return errorResult(cmd, wire.Errorf(wire.CodeTicketState, "--supersede", "ticket %s is %s; only unfinished work is archived as a duplicate", dup, dupRec.Status))
	}
	actor, err := initActor(f.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	if !already {
		payload, err := mutation.CanonicalPayload(mutation.OpRefine, wire.ObjectValue(wire.NewObject().Set("supersededBy", wire.String(keep))))
		if err != nil {
			return errorResult(cmd, err)
		}
		flags := mutateFlags{role: f.role, requestID: targetRequestID(f.requestID, dup, "refine"), target: dup, expected: string(dupRec.Revision), issuedAt: f.issuedAt}
		if out := submitMutation(env, []string{"ticket", "refine"}, mutation.OpRefine, actor, flags, payload); out.Outcome != wire.OutcomeOK {
			out.Command = cmd
			return out
		}
	}
	rev, err := readTargetRevision(env, dup)
	if err != nil {
		return errorResult(cmd, err)
	}
	keepLocal := keep[strings.LastIndex(keep, ":")+1:]
	payload, err := mutation.CanonicalPayload(mutation.OpArchive, wire.ObjectValue(wire.NewObject().Set("reason", wire.String(reasonPrefix("DUPLICATE")+"duplicate of "+keepLocal))))
	if err != nil {
		return errorResult(cmd, err)
	}
	flags := mutateFlags{role: f.role, requestID: targetRequestID(f.requestID, dup, "archive"), target: dup, expected: rev, issuedAt: f.issuedAt}
	if out := submitMutation(env, []string{"ticket", "archive"}, mutation.OpArchive, actor, flags, payload); out.Outcome != wire.OutcomeOK {
		out.Command = cmd
		return out
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{findingRow(-1, "SUPERSEDED", dup, "superseded by "+keep+" and archived as its duplicate", true)}}
}
