package cli

import (
	"container/heap"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Closeout-debt read (TCD-V0-001..009, V1-1615): OPEN tickets that merged
// commits on the integration ref cite, with the native closeout step each one
// still lacks. Every row is a candidate, never a delivery claim.
const (
	profileCloseoutDebt      = "taskman-closeout-debt/0"
	defaultCloseoutScanLimit = 1000
	maxCloseoutScanLimit     = 10000
	maxCloseoutRows          = 2000
	maxCloseoutSubjectBytes  = 512

	citeDirect   = "direct-id"
	citeIssue    = "issue-link"
	citeUmbrella = "umbrella"
	// citeProgressHead marks a row whose OPEN ticket's effective progress
	// names a scanned commit as its head (TLF-V0-883).
	citeProgressHead = "progress-head"

	stepNoAttempt       = "NO_ATTEMPT"
	stepAttemptEnded    = "ATTEMPT_ENDED"
	stepNotSubmitted    = "CLAIMED_NOT_SUBMITTED"
	stepGatesPending    = "SUBMITTED_GATES_PENDING"
	stepGateFailed      = "GATE_FAILED"
	stepCompletePending = "GATES_PASSED_COMPLETE_PENDING"
)

var (
	// closeoutWord is one message word (TCD-V0-004): ticket IDs, local
	// tokens and their full ticket:<authority>:<queue>:<local> form.
	closeoutWord = regexp.MustCompile(`[A-Za-z0-9._:-]+`)
	// closeoutClosing is a GitHub closing keyword naming an issue, with an
	// optional owner/repo qualifier.
	closeoutClosing = regexp.MustCompile(`(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\b:?\s+(?:([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+))?#([0-9]{1,9})\b`)
	// closeoutIssueText and closeoutIssueSlug are the ticket-side issue links.
	closeoutIssueText = regexp.MustCompile(`(?i)\bissue\s+#([0-9]{1,9})\b`)
	closeoutIssueSlug = regexp.MustCompile(`\b([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#([0-9]{1,9})\b`)
)

// closeoutIndex maps citations to the tickets they name, of every status
// (TCD-V0-004); only OPEN ones become rows (TCD-V0-005).
type closeoutIndex struct {
	tokens map[string]string   // folded local or full ID -> ticket ID
	issues map[string][]string // issue number -> ticket IDs, ascending
	origin string              // folded owner/repo of origin, or ""
}

// issueNumber normalizes a decimal issue number ("007" -> "7").
func issueNumber(s string) string {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(n, 10)
}

// ticketIssues is the set of issues a ticket's title or body links.
func (x *closeoutIndex) ticketIssues(rec *ticket.Record) []string {
	text := rec.Title
	if rec.Body != nil {
		text += "\n" + *rec.Body
	}
	seen := map[string]bool{}
	var out []string
	add := func(n string) {
		if n = issueNumber(n); n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, m := range closeoutIssueText.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}
	if x.origin != "" {
		for _, m := range closeoutIssueSlug.FindAllStringSubmatch(text, -1) {
			if wire.FoldToken(m[1]) == x.origin {
				add(m[2])
			}
		}
	}
	return out
}

// add indexes one ticket.
func (x *closeoutIndex) add(rec *ticket.Record) {
	id := rec.TicketID.Raw
	x.tokens[wire.FoldToken(rec.TicketID.Local)] = id
	x.tokens[wire.FoldToken(id)] = id
	for _, n := range x.ticketIssues(rec) {
		x.issues[n] = append(x.issues[n], id)
	}
}

// closeoutCite is one ticket a commit cites and how.
type closeoutCite struct {
	ticketID, kind, issue string
}

// cites returns the tickets one commit message cites (TCD-V0-004), by
// ticket ID. More than one cited ticket makes every citation umbrella.
func (x *closeoutIndex) cites(message string) []closeoutCite {
	byID := map[string]*closeoutCite{}
	for _, w := range closeoutWord.FindAllString(message, -1) {
		w = strings.TrimRight(w, ".:")
		if id, ok := x.tokens[wire.FoldToken(w)]; ok {
			if c := byID[id]; c == nil {
				byID[id] = &closeoutCite{ticketID: id, kind: citeDirect}
			} else {
				c.kind = citeDirect
			}
		}
	}
	for _, m := range closeoutClosing.FindAllStringSubmatch(message, -1) {
		if m[1] != "" && (x.origin == "" || wire.FoldToken(m[1]) != x.origin) {
			continue
		}
		n := issueNumber(m[2])
		for _, id := range x.issues[n] {
			c := byID[id]
			if c == nil {
				c = &closeoutCite{ticketID: id, kind: citeIssue}
				byID[id] = c
			}
			if c.issue == "" {
				c.issue = n
			}
		}
	}
	out := make([]closeoutCite, 0, len(byID))
	for _, c := range byID {
		if len(byID) > 1 {
			c.kind = citeUmbrella
		}
		out = append(out, *c)
	}
	return out
}

// closeoutRow is one candidate (TCD-V0-007).
type closeoutRow struct {
	ticketID, local, commit, subject, kind, issue, step, gate string
}

func closeoutRowLess(a, b closeoutRow) bool {
	if a.ticketID != b.ticketID {
		return a.ticketID < b.ticketID
	}
	if a.commit != b.commit {
		return a.commit < b.commit
	}
	return a.kind < b.kind
}

// closeoutRowHeap is a max-heap in row order: its root is the last row kept.
type closeoutRowHeap []closeoutRow

func (h closeoutRowHeap) Len() int           { return len(h) }
func (h closeoutRowHeap) Less(i, j int) bool { return closeoutRowLess(h[j], h[i]) }
func (h closeoutRowHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *closeoutRowHeap) Push(x any)        { *h = append(*h, x.(closeoutRow)) }
func (h *closeoutRowHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// closeoutRows counts every candidate row but keeps only the first max in
// row order (TCD-V0-007), so a citation fan-out holds at most max rows
// however many it matches.
type closeoutRows struct {
	max, total int
	kept       closeoutRowHeap
}

func (c *closeoutRows) add(r closeoutRow) {
	c.total++
	switch {
	case c.max <= 0:
	case len(c.kept) < c.max:
		heap.Push(&c.kept, r)
	case closeoutRowLess(r, c.kept[0]):
		c.kept[0] = r
		heap.Fix(&c.kept, 0)
	}
}

// sorted returns the kept rows in row order.
func (c *closeoutRows) sorted() []closeoutRow {
	out := append([]closeoutRow(nil), c.kept...)
	sort.Slice(out, func(i, j int) bool { return closeoutRowLess(out[i], out[j]) })
	return out
}

// closeoutStep derives the pending native closeout step of one OPEN ticket
// (TCD-V0-006) from its attempt and its verified gate evidence. gateOf
// reads one gate result by digest.
func closeoutStep(rc *readCtx, rec *ticket.Record, a *snapshot.Attempt, gateOf func(string) (*snapshot.GateResult, error)) (string, string) {
	switch {
	case a == nil:
		return stepNoAttempt, ""
	case !a.Live():
		return stepAttemptEnded, ""
	case a.CandidateTreeOid == nil:
		return stepNotSubmitted, ""
	}
	set := map[string]bool{}
	for _, g := range rc.store.Policy.Gates {
		if g.Required {
			set[g.GateID] = true
		}
	}
	for _, g := range rec.RequiredGates {
		set[g] = true
	}
	required := make([]string, 0, len(set))
	for g := range set {
		required = append(required, g)
	}
	sort.Strings(required)
	results := map[string]*snapshot.GateResult{}
	for _, d := range a.GateResults {
		g, err := gateOf(d)
		if err != nil {
			return notObserved, ""
		}
		results[g.GateID] = g
	}
	pending := ""
	for _, id := range required {
		g := results[id]
		current := g != nil && g.CandidateTreeOid == *a.CandidateTreeOid && g.DefinitionSha256 == rc.store.Policy.GateDefinitionSha256(id) &&
			g.AttemptID == a.AttemptID && g.Generation == a.Generation && g.TicketRevision == a.TicketRevision &&
			g.PolicySha256 == a.PolicySha256 && g.ConfigSha256 == a.ConfigSha256
		switch {
		case current && g.State != "PASSED":
			return stepGateFailed, id
		case !current && pending == "":
			pending = id
		}
	}
	if pending != "" {
		return stepGatesPending, pending
	}
	return stepCompletePending, ""
}

// closeoutAttempts picks each ticket's attempt: the live one, else the one
// that reached its phase last (TCD-V0-006).
func closeoutAttempts(attempts map[string]*snapshot.Attempt) map[string]*snapshot.Attempt {
	out := map[string]*snapshot.Attempt{}
	for _, a := range attempts {
		id := a.TicketID.Raw
		b := out[id]
		switch {
		case b == nil:
		case a.Live() != b.Live():
			if !a.Live() {
				continue
			}
		case a.PhaseSinceSeq.Uint64() != b.PhaseSinceSeq.Uint64():
			if a.PhaseSinceSeq.Uint64() < b.PhaseSinceSeq.Uint64() {
				continue
			}
		case a.AttemptID < b.AttemptID:
			continue
		}
		out[id] = a
	}
	return out
}

// truncateUTF8 cuts s to at most n bytes on a rune boundary.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// closeoutDebtCommand is `corvint-tasks ticket closeout-debt`. It takes no
// lock and writes nothing (TCD-V0-008); the caller maps the result and the
// candidate count to the TCD-V0-009 exit status.
func closeoutDebtCommand(env Env, args []string) (*wire.Result, int) {
	cmd := []string{"ticket", "closeout-debt"}
	fl, pos, err := flags(args, "ref", "since", "limit")
	if err == nil && len(pos) != 0 {
		err = wire.Errorf(wire.CodeMalformed, "argv", "ticket closeout-debt takes no positional argument")
	}
	limit := 0
	if err == nil {
		limit, err = parseScanLimit(fl)
	}
	if err != nil {
		return failure(cmd, nil, err), 0
	}
	var (
		index    *closeoutIndex
		locals   map[string]string            // OPEN tickets only
		heads    map[string][]string          // progress head -> OPEN ticket IDs
		attempts map[string]*snapshot.Attempt // nil = NOT_OBSERVED
		branch   string
	)
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		index = &closeoutIndex{tokens: map[string]string{}, issues: map[string][]string{}, origin: store.OriginSlug(rc.repo.PrimaryWorktree)}
		locals, heads, attempts, branch = map[string]string{}, map[string][]string{}, nil, rc.store.Queue.IntentBranch
		for _, id := range rc.store.Inventory.Sorted() {
			rec, _ := rc.store.Inventory.Get(id)
			index.add(rec)
			if rec.Status == ticket.StatusOpen {
				locals[id] = rec.TicketID.Local
				if p := rec.EffectiveProgress(); p != nil && p.Head != nil {
					heads[*p.Head] = append(heads[*p.Head], id)
				}
			}
		}
		if rc.journalAbsent {
			return nil
		}
		in, _, e := planInput(rc)
		if e != nil {
			return e
		}
		attempts = closeoutAttempts(in.Attempts)
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err), 0
	}
	root := rc.repo.PrimaryWorktree
	ref, refCommit, since, err := closeoutRef(root, branch, fl)
	var scan store.CommitScan
	if err == nil {
		scan, err = store.ScanCommits(root, refCommit, since, limit)
	}
	if err != nil {
		return failure(cmd, rc, err), 0
	}

	src := journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.IntentRoot()}
	gateOf := func(d string) (*snapshot.GateResult, error) {
		raw, err := src.Read("evidence/"+d, snapshot.MaxGateRecordBytes)
		if err != nil {
			return nil, err
		}
		if wire.Sum(raw) != wire.Digest(d) {
			return nil, errors.New("digest mismatch")
		}
		return snapshot.DecodeGateResult(raw)
	}
	collected := closeoutRows{max: maxCloseoutRows}
	for _, c := range scan.Commits {
		subject := truncateUTF8(c.Subject, maxCloseoutSubjectBytes)
		for _, cite := range index.cites(c.Message) {
			if local, open := locals[cite.ticketID]; open {
				collected.add(closeoutRow{ticketID: cite.ticketID, local: local, commit: c.Commit, subject: subject, kind: cite.kind, issue: cite.issue})
			}
		}
		for _, id := range heads[c.Commit] {
			collected.add(closeoutRow{ticketID: id, local: locals[id], commit: c.Commit, subject: subject, kind: citeProgressHead})
		}
	}
	type stepOf struct{ step, gate string }
	steps := map[string]stepOf{}
	rows, total := collected.sorted(), collected.total
	for i := range rows {
		st, ok := steps[rows[i].ticketID]
		if !ok {
			st.step = notObserved
			if attempts != nil {
				rec, _ := rc.store.Inventory.Get(rows[i].ticketID)
				st.step, st.gate = closeoutStep(rc, rec, attempts[rows[i].ticketID], gateOf)
			}
			steps[rows[i].ticketID] = st
		}
		rows[i].step, rows[i].gate = st.step, st.gate
	}
	res := success(cmd, rc)
	if scan.Truncated {
		res.Warnings = append(res.Warnings, "commit scan stopped at --limit "+strconv.Itoa(limit)+"; older citations are not reported")
	}
	res.Items = []wire.Value{closeoutValue(rows, total, ref, refCommit, since, limit, len(scan.Commits), scan.Truncated, index.origin)}
	res.Untrusted = true
	return res, total
}

func orNull(s string) wire.Value {
	if s == "" {
		return wire.Null()
	}
	return wire.String(s)
}

// closeoutValue renders the taskman-closeout-debt/0 item (TCD-V0-007).
func closeoutValue(rows []closeoutRow, total int, ref, refCommit, since string, limit, scanned int, scanTruncated bool, origin string) wire.Value {
	cands := make([]wire.Value, 0, len(rows))
	human := []string{"closeout-debt " + ref + ": " + strconv.Itoa(total) + " candidate(s) in " + strconv.Itoa(scanned) + " commit(s)"}
	for _, r := range rows {
		o := wire.NewObject()
		o.Set("ticketId", wire.String(r.ticketID))
		o.Set("local", wire.String(r.local))
		o.Set("label", wire.String("candidate"))
		o.Set("commit", wire.String(r.commit))
		o.Set("subject", wire.String(r.subject))
		o.Set("citation", wire.String(r.kind))
		o.Set("issue", orNull(r.issue))
		o.Set("closeoutStep", wire.String(r.step))
		o.Set("gate", orNull(r.gate))
		cands = append(cands, wire.ObjectValue(o))
		line := r.local + " candidate " + r.step
		if r.gate != "" {
			line += " (" + r.gate + ")"
		}
		human = append(human, line+" via "+r.kind+" "+r.commit[:min(12, len(r.commit))])
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(profileCloseoutDebt))
	o.Set("ref", wire.String(ref))
	o.Set("refCommit", wire.String(refCommit))
	o.Set("since", orNull(since))
	o.Set("scanLimit", count(limit))
	o.Set("commitsScanned", count(scanned))
	o.Set("scanTruncated", wire.Bool(scanTruncated))
	o.Set("origin", orNull(origin))
	o.Set("candidatesTotal", count(total))
	o.Set("candidatesReturned", count(len(cands)))
	o.Set("candidatesTruncated", wire.Bool(total > len(cands)))
	o.Set("candidates", wire.Array(cands...))
	o.Set("human", wire.Strings(human))
	o.Set("mutationAuthority", wire.Bool(false))
	return wire.ObjectValue(o)
}

// emitCloseoutDebt maps the result to the TCD-V0-009 exit status: 0 for OK
// with no candidate, 1 for OK with candidates, 2 otherwise.
func emitCloseoutDebt(env Env, args []string) int {
	res, candidates := closeoutDebtCommand(env, args)
	if emit(env.Stdout, res) != 0 {
		return 2
	}
	if candidates > 0 {
		return 1
	}
	return 0
}
