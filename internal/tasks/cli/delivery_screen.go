package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Delivery reads (TCD-V0-159..164, V1-2251, V1-2254): which tickets an open
// PR, a merged PR or an integration-ref commit already references as
// delivered, and which tickets a merged batch delivered with its green CI
// runs. GitHub facts come only from operator-exported `gh --json` files;
// the verbs never query a network service, and without an export every
// GitHub fact is NOT_OBSERVED, never CLEAR.
const (
	profileDeliveryScreen = "taskman-delivery-screen/0"
	profileBatchDelivered = "taskman-batch-delivered/0"
	maxGitHubExportBytes  = 32 * wire.MiB
	maxScreenTickets      = 256
	maxScreenHits         = 32

	refDelivered = "delivered"
	refMentioned = "mentioned"

	verdictInFlight  = "IN_FLIGHT"
	verdictDelivered = "DELIVERED"
	// verdictOnLane is a delivery cited only by a local branch or worktree
	// head the screened ref does not contain (TLF-V0-887, --lane-refs).
	verdictOnLane    = "DELIVERED_ON_LANE"
	verdictTruncated = "TRUNCATED"
	verdictCited     = "CITED"
	verdictMentioned = "MENTIONED"
	verdictClear     = "CLEAR"
	// verdictCompleted and verdictHeld come from the store ahead of any PR
	// or commit evidence (TCD-V0-170).
	verdictCompleted = "COMPLETED"
	verdictHeld      = "HELD"
)

var (
	// deliveryLead is a commit line led by a delivery word (TCD-V0-167).
	deliveryLead = regexp.MustCompile(`(?i)^\W*(?:refs|closes|fixes|delivers|delivered)\b`)
	// deliveryWord and deliveryVeto are the per-ID markers of TCD-V0-730:
	// a delivery word, and a phrase that names a ticket it did not deliver.
	// A delivery word inside a veto phrase (`ALREADY-DELIVERED`) is no
	// marker of its own.
	deliveryWord = regexp.MustCompile(`(?i)\b(?:refs|closes|fixes|delivers|delivered)\b`)
	deliveryVeto = regexp.MustCompile(`(?i)(?:already|not)[ -]+delivered|not[ -]+started|skipped|residual|duplicate|blocked|already`)
	// clauseBreak splits a line into the clauses markers stay inside.
	clauseBreak = regexp.MustCompile(`[;|]`)
	// prNumberRef is a `#N` PR reference in a batch title or body.
	prNumberRef = regexp.MustCompile(`#([0-9]{1,9})\b`)
	// subjectScope and subjectTail are the ticket-scoped parts of a commit
	// subject (TCD-V0-167): a conventional-commit scope `type(ID):` and a
	// final parenthetical `(... ID ...)`.
	subjectScope = regexp.MustCompile(`^\s*[A-Za-z]+\(([^()]*)\)!?:`)
	subjectTail  = regexp.MustCompile(`\(([^()]*)\)[\s.]*$`)
)

// ghPR is one record of `gh pr list --json
// number,state,title,body,headRefName,headRefOid,mergeCommit,labels[,mergedAt]`.
type ghPR struct {
	Number      int64  `json:"number"`
	State       string `json:"state"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	HeadRefName string `json:"headRefName"`
	HeadRefOid  string `json:"headRefOid"`
	// MergedAt is the optional `mergedAt` field; batch-close uses it as the
	// default issuedAt so a rerun replays (TCD-V0-169).
	MergedAt string `json:"mergedAt"`
	// URL is the optional `url` field; the gated batch-close binds it as
	// the batch PR's identity (TCD-V0-188).
	URL         string `json:"url"`
	MergeCommit *struct {
		Oid string `json:"oid"`
	} `json:"mergeCommit"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// ghRun is one record of `gh run list --json
// databaseId,headSha,conclusion[,url]`.
type ghRun struct {
	DatabaseID int64  `json:"databaseId"`
	HeadSha    string `json:"headSha"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"url"`
}

// readGitHubExport decodes a bounded `gh --json` array file into out.
func readGitHubExport(flag, file string, out any) error {
	raw, err := readBoundedFile(flag, file, maxGitHubExportBytes)
	if err != nil {
		return err
	}
	return decodeGitHubExport(flag, raw, out)
}

// readGitHubExportArg is readGitHubExport where the file "-" reads the
// export from stdin under the same byte bound (TCD-V0-171).
func readGitHubExportArg(env Env, flag, file string, out any) error {
	if file != "-" {
		return readGitHubExport(flag, commandPath(env.Cwd, file), out)
	}
	if env.Stdin == nil {
		return wire.Errorf(wire.CodeMissingEvidence, flag, "the %s export on stdin is unreadable", flag)
	}
	raw, err := io.ReadAll(io.LimitReader(env.Stdin, int64(maxGitHubExportBytes)+1))
	if err != nil {
		return wire.Errorf(wire.CodeMissingEvidence, flag, "the %s export on stdin is unreadable", flag)
	}
	if len(raw) > maxGitHubExportBytes {
		return wire.Errorf(wire.CodeLimitExceeded, flag, "the %s export on stdin exceeds %d bytes", flag, maxGitHubExportBytes)
	}
	return decodeGitHubExport(flag, raw, out)
}

func decodeGitHubExport(flag string, raw []byte, out any) error {
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(out); err != nil {
		return wire.Errorf(wire.CodeMalformed, flag, "the %s file is not a gh --json array: %s", flag, prose(err.Error()))
	}
	return nil
}

func (p *ghPR) batch() bool {
	if strings.Contains(strings.ToLower(p.Title), "batch") {
		return true
	}
	for _, l := range p.Labels {
		if l.Name == "ci:batch" {
			return true
		}
	}
	return false
}

func (p *ghPR) mergeOid() string {
	if p.MergeCommit == nil {
		return ""
	}
	return p.MergeCommit.Oid
}

// lineTickets returns the indexed tickets one line names by ID.
func (x *closeoutIndex) lineTickets(line string) []string {
	var out []string
	for _, w := range closeoutWord.FindAllString(line, -1) {
		if id, ok := x.tokens[wire.FoldToken(strings.TrimRight(w, ".:"))]; ok && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// refMarker is one delivery word or veto phrase on a line (TCD-V0-730).
type refMarker struct {
	start, end int
	veto       bool
}

// lineMarkers returns the markers of line in order. With anywhere false (a
// commit line, TCD-V0-167) a delivery word other than DELIVERED counts only
// when it leads the line.
func lineMarkers(line string, anywhere bool) []refMarker {
	var out []refMarker
	vetoes := deliveryVeto.FindAllStringIndex(line, -1)
	for _, v := range vetoes {
		out = append(out, refMarker{v[0], v[1], true})
	}
	lead := deliveryLead.FindStringIndex(line)
	for _, w := range deliveryWord.FindAllStringIndex(line, -1) {
		if slices.ContainsFunc(vetoes, func(v []int) bool { return w[0] < v[1] && v[0] < w[1] }) {
			continue
		}
		if anywhere || line[w[0]:w[1]] == "DELIVERED" || (lead != nil && lead[1] == w[1]) {
			out = append(out, refMarker{w[0], w[1], false})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out
}

// idReferences classifies each ticket one line names (TCD-V0-730): per
// clause, an ID's trailing markers (up to the next ID's comma, or the clause
// end) decide first, a veto before a delivery word, then the last marker
// before it; with no marker it is mentioned. vetoed holds every ID a veto
// governs on the line.
func (x *closeoutIndex) idReferences(line string, anywhere bool) (refs map[string]string, vetoed map[string]bool) {
	refs, vetoed = map[string]string{}, map[string]bool{}
	markers := lineMarkers(line, anywhere)
	bounds := []int{0}
	for _, b := range clauseBreak.FindAllStringIndex(line, -1) {
		bounds = append(bounds, b[0])
	}
	bounds = append(bounds, len(line))
	type occ struct {
		start, end int
		id         string
	}
	for c := 0; c+1 < len(bounds); c++ {
		lo, hi := bounds[c], bounds[c+1]
		var occs []occ
		for _, w := range closeoutWord.FindAllStringIndex(line[lo:hi], -1) {
			if id, ok := x.tokens[wire.FoldToken(strings.TrimRight(line[lo+w[0]:lo+w[1]], ".:"))]; ok {
				occs = append(occs, occ{lo + w[0], lo + w[1], id})
			}
		}
		for k, o := range occs {
			trailEnd := hi
			if k+1 < len(occs) {
				trailEnd = occs[k+1].start
				if i := strings.IndexByte(line[o.end:trailEnd], ','); i >= 0 {
					trailEnd = o.end + i
				}
			}
			var leading *refMarker
			trailVeto, trailDeliver := false, false
			for i := range markers {
				m := &markers[i]
				switch {
				case m.start >= lo && m.end <= o.start:
					leading = m
				case m.start >= o.end && m.start < trailEnd:
					trailVeto, trailDeliver = trailVeto || m.veto, trailDeliver || !m.veto
				}
			}
			kind := refMentioned
			switch {
			case trailVeto:
				vetoed[o.id] = true
			case trailDeliver:
				kind = refDelivered
			case leading != nil && leading.veto:
				vetoed[o.id] = true
			case leading != nil:
				kind = refDelivered
			}
			if kind == refDelivered || refs[o.id] == "" {
				refs[o.id] = kind
			}
		}
	}
	return refs, vetoed
}

// references maps each ticket a PR title or body names to delivered or
// mentioned (TCD-V0-160, TCD-V0-730): a delivery word anywhere in an ID's
// clause delivers it unless a veto governs that ID. Delivered on any line
// wins. Commit messages use commitReferences (TCD-V0-167).
func (x *closeoutIndex) references(text string) map[string]string {
	out, _ := x.textReferences(text)
	return out
}

// textReferences is references plus every ID a veto governs on some line.
func (x *closeoutIndex) textReferences(text string) (out map[string]string, vetoed map[string]bool) {
	out, vetoed = map[string]string{}, map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		refs, v := x.idReferences(line, true)
		for id, kind := range refs {
			if kind == refDelivered || out[id] == "" {
				out[id] = kind
			}
		}
		for id := range v {
			vetoed[id] = true
		}
	}
	return out, vetoed
}

// prReferences maps each ticket one PR of a batch read names to delivered or
// mentioned (TCD-V0-870, TCD-V0-730): its body under references, plus every
// body line whose first word is a ticket's ID that no veto on that line
// governs, plus its title read as a commit subject under commitReferences,
// whose scope a per-ID body veto overrides. Delivered wins.
func (x *closeoutIndex) prReferences(p *ghPR) map[string]string {
	out, bodyVetoed := x.textReferences(p.Body)
	for _, line := range strings.Split(p.Body, "\n") {
		lead := strings.TrimLeftFunc(line, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		if lead == "" {
			continue
		}
		_, vetoed := x.idReferences(line, true)
		for _, id := range x.lineTickets(closeoutWord.FindString(lead)) {
			if !vetoed[id] {
				out[id] = refDelivered
			}
		}
	}
	for id, ref := range x.messageReferences(p.Title, true) {
		if ref == refDelivered && bodyVetoed[id] {
			ref = refMentioned
		}
		if ref == refDelivered || out[id] == "" {
			out[id] = ref
		}
	}
	return out
}

// commitReferences maps each ticket a commit message names to delivered or
// mentioned (TCD-V0-167, TCD-V0-730). Per ID, a line delivers when no veto
// governs the ID and either a leading Refs, Closes, Fixes, Delivers or
// Delivered or a DELIVERED in capitals marks it, or it is on the subject and
// the subject is scoped to it: its first word, its conventional-commit scope
// or its final parenthetical names it. Every other naming line only
// mentions. Delivered wins.
func (x *closeoutIndex) commitReferences(message string) map[string]string {
	return x.messageReferences(message, false)
}

// messageReferences is commitReferences with the delivery-word placement of
// idReferences: anywhere in the clause for a PR title (TCD-V0-730), leading
// the line for a commit message.
func (x *closeoutIndex) messageReferences(message string, anywhere bool) map[string]string {
	out := map[string]string{}
	for n, line := range strings.Split(strings.TrimLeft(message, "\n"), "\n") {
		refs, vetoed := x.idReferences(line, anywhere)
		if n == 0 {
			for _, id := range x.subjectTickets(line) {
				if !vetoed[id] {
					refs[id] = refDelivered
				}
			}
		}
		for id, kind := range refs {
			if kind == refDelivered || out[id] == "" {
				out[id] = kind
			}
		}
	}
	return out
}

// subjectTickets returns the tickets a commit subject is scoped to: its
// first word, its conventional-commit scope and its final parenthetical.
func (x *closeoutIndex) subjectTickets(subject string) []string {
	var out []string
	if w := closeoutWord.FindString(subject); w != "" {
		out = append(out, x.lineTickets(w)...)
	}
	for _, re := range []*regexp.Regexp{subjectScope, subjectTail} {
		if m := re.FindStringSubmatch(subject); m != nil {
			out = append(out, x.lineTickets(m[1])...)
		}
	}
	return out
}

// closeoutRef resolves --ref (default origin/<intentBranch>, else the local
// intent branch) and --since as `ticket closeout-debt` does (TCD-V0-001).
func closeoutRef(root, branch string, fl map[string]string) (ref, refCommit, since string, err error) {
	ref = fl["ref"]
	if _, given := fl["ref"]; !given {
		ref = "refs/heads/" + branch
		switch _, e := store.CloseoutRevision(root, "refs/remotes/origin/"+branch); {
		case e == nil:
			ref = "refs/remotes/origin/" + branch
		case wire.CodeOf(e) != wire.CodeMalformed:
			return "", "", "", e
		}
	}
	if refCommit, err = store.CloseoutRevision(root, ref); err != nil {
		return "", "", "", err
	}
	if v, given := fl["since"]; given {
		since, err = store.CloseoutRevision(root, v)
	}
	return ref, refCommit, since, err
}

func parseScanLimit(fl map[string]string) (int, error) {
	v, ok := fl["limit"]
	if !ok {
		return defaultCloseoutScanLimit, nil
	}
	n, e := strconv.Atoi(v)
	if e != nil || n < 1 || n > maxCloseoutScanLimit {
		return 0, wire.Errorf(wire.CodeMalformed, "--limit", "limit must be 1..%d", maxCloseoutScanLimit)
	}
	return n, nil
}

// screenRow is one screened ticket (TCD-V0-161).
type screenRow struct {
	ticketID, local, status, verdict string
	created                          time.Time // zero when unreadable: never TRUNCATED
	prs, commits                     []wire.Value
	lanes                            []wire.Value // --lane-refs only
	// holds and note are the store facts of TCD-V0-170: every hold with its
	// holdId and reason, and the latest operator note.
	holds []ticket.Hold
	note  wire.Value
}

// deliveryScreenCommand is `corvint-tasks ticket delivery-screen`. It takes
// no lock and writes nothing (TCD-V0-159); exit 0 when every ticket is
// CLEAR, 1 otherwise, 2 on a refusal (TCD-V0-162). A COMPLETED or HELD
// ticket takes that verdict from the store before any PR evidence, and each
// row carries the ticket's holds and latest operator note (TCD-V0-170).
func deliveryScreenCommand(env Env, args []string) (*wire.Result, int) {
	cmd := []string{"ticket", "delivery-screen"}
	laneRefs := slices.Contains(args, "--lane-refs")
	// The output is always JSON, so a bare --json is a no-op (TCD-V0-955).
	args = slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == "--lane-refs" || a == "--json" })
	fl, pos, err := flags(args, "prs", "ref", "since", "limit")
	if err == nil && (len(pos) == 0 || len(pos) > maxScreenTickets) {
		err = wire.Errorf(wire.CodeMalformed, "argv", "ticket delivery-screen takes 1..%d ticket ids", maxScreenTickets)
	}
	limit := 0
	if err == nil {
		limit, err = parseScanLimit(fl)
	}
	var prs []ghPR
	file, observed := fl["prs"]
	if err == nil && observed {
		err = readGitHubExportArg(env, "--prs", file, &prs)
	}
	if err != nil {
		return failure(cmd, nil, err), 0
	}
	var (
		index  *closeoutIndex
		rows   []screenRow
		branch string
		phases map[string]string
	)
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		index = &closeoutIndex{tokens: map[string]string{}, issues: map[string][]string{}}
		rows, branch, phases = nil, rc.store.Queue.IntentBranch, map[string]string{}
		for _, id := range rc.store.Inventory.Sorted() {
			rec, _ := rc.store.Inventory.Get(id)
			index.add(rec)
		}
		for _, arg := range pos {
			id, e := resolveTicketArg(rc, arg)
			if e != nil {
				return e
			}
			rec, ok := rc.store.Inventory.Get(id)
			if !ok {
				return wire.Errorf(wire.CodeMalformed, "argv", "ticket %s is not in the queue", prose(arg))
			}
			if !slices.ContainsFunc(rows, func(r screenRow) bool { return r.ticketID == id }) {
				created, _ := time.Parse(time.RFC3339Nano, string(rec.CreatedAt))
				rows = append(rows, screenRow{ticketID: id, local: rec.TicketID.Local, status: rec.Status, created: created, holds: slices.Clone(rec.Holds), note: operatorNoteShowValue(rc, rec)})
				phases[id] = rec.EffectivePhase()
			}
		}
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
	// windowStart is the committer time of the oldest scanned commit when
	// the scan stopped at --limit: a ticket created before it may be cited
	// in history the scan never read (TCD-V0-168). Committer time has whole
	// seconds, so a ticket created within that second counts as before it.
	var windowStart time.Time
	if err == nil && scan.Truncated && len(scan.Commits) > 0 {
		windowStart, err = store.CommitTime(root, scan.Commits[len(scan.Commits)-1].Commit)
	}
	var lanes []laneScan
	if err == nil && laneRefs {
		lanes, err = scanLaneHeads(root, refCommit, limit)
	}
	// laneWindowStart is the latest oldest-scanned-commit time over the lane
	// scans cut at --limit: a ticket created before it may be delivered by a
	// lane commit the scan never read, so it is TRUNCATED, never CLEAR
	// (TLF-V0-887).
	var laneWindowStart time.Time
	laneTruncated := false
	for _, l := range lanes {
		if err != nil || !l.scan.Truncated || len(l.scan.Commits) == 0 {
			continue
		}
		laneTruncated = true
		var at time.Time
		if at, err = store.CommitTime(root, l.scan.Commits[len(l.scan.Commits)-1].Commit); err == nil && at.After(laneWindowStart) {
			laneWindowStart = at
		}
	}
	if err != nil {
		return failure(cmd, rc, err), 0
	}
	commitRefs := make([]map[string]string, len(scan.Commits))
	for i, c := range scan.Commits {
		commitRefs[i] = index.commitReferences(c.Message)
	}
	prRefs := make([]map[string]string, len(prs))
	for i := range prs {
		prRefs[i] = index.prReferences(&prs[i])
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].ticketID < rows[j].ticketID })
	clear := 0
	for i := range rows {
		r := &rows[i]
		inFlight, delivered, cited, mentioned := false, false, false, false
		for j, c := range scan.Commits {
			kind := commitRefs[j][r.ticketID]
			if kind == "" {
				continue
			}
			delivered, cited = delivered || kind == refDelivered, true
			if len(r.commits) < maxScreenHits {
				o := wire.NewObject()
				o.Set("commit", wire.String(c.Commit))
				o.Set("subject", wire.String(truncateUTF8(c.Subject, maxCloseoutSubjectBytes)))
				o.Set("reference", wire.String(kind))
				r.commits = append(r.commits, wire.ObjectValue(o))
			}
		}
		onLane := false
		for _, l := range lanes {
			for _, c := range l.scan.Commits {
				if index.commitReferences(c.Message)[r.ticketID] != refDelivered {
					continue
				}
				onLane = true
				if len(r.lanes) < maxScreenHits {
					o := wire.NewObject()
					o.Set("ref", wire.String(l.ref))
					o.Set("commit", wire.String(c.Commit))
					o.Set("subject", wire.String(truncateUTF8(c.Subject, maxCloseoutSubjectBytes)))
					r.lanes = append(r.lanes, wire.ObjectValue(o))
				}
			}
		}
		if laneRefs && pastInProgress(phases[r.ticketID]) {
			inFlight = true
		}
		order := make([]int, 0)
		for j := range prs {
			if prRefs[j][r.ticketID] != "" {
				order = append(order, j)
			}
		}
		sort.Slice(order, func(a, b int) bool { return prs[order[a]].Number < prs[order[b]].Number })
		for _, j := range order {
			p, kind := &prs[j], prRefs[j][r.ticketID]
			mentioned = true
			if kind == refDelivered {
				inFlight = inFlight || p.State == "OPEN"
				delivered = delivered || p.State == "MERGED"
			}
			if len(r.prs) < maxScreenHits {
				o := wire.NewObject()
				o.Set("number", wire.String(strconv.FormatInt(p.Number, 10)))
				o.Set("state", wire.String(prose(p.State)))
				o.Set("reference", wire.String(kind))
				o.Set("batch", wire.Bool(p.batch()))
				r.prs = append(r.prs, wire.ObjectValue(o))
			}
		}
		switch {
		case r.status == ticket.StatusCompleted:
			r.verdict = verdictCompleted
		case r.status == ticket.StatusHeld || len(r.holds) > 0:
			r.verdict = verdictHeld
		case inFlight:
			r.verdict = verdictInFlight
		case delivered:
			r.verdict = verdictDelivered
		case onLane:
			r.verdict = verdictOnLane
		case !windowStart.IsZero() && r.created.Before(windowStart.Add(time.Second)),
			!laneWindowStart.IsZero() && r.created.Before(laneWindowStart.Add(time.Second)):
			r.verdict = verdictTruncated
		case cited:
			r.verdict = verdictCited
		case !observed:
			r.verdict = notObserved
		case mentioned:
			r.verdict = verdictMentioned
		default:
			r.verdict = verdictClear
			clear++
		}
	}
	res := success(cmd, rc)
	if scan.Truncated {
		res.Warnings = append(res.Warnings, "commit scan stopped at --limit "+strconv.Itoa(limit)+"; older references are not reported, and a ticket created before the oldest scanned commit is TRUNCATED")
	}
	if !observed {
		res.Warnings = append(res.Warnings, "no --prs export; open and merged PRs NOT_OBSERVED")
	}
	for _, l := range lanes {
		if l.scan.Truncated {
			res.Warnings = append(res.Warnings, "lane head "+prose(l.ref)+" scan stopped at --limit "+strconv.Itoa(limit)+"; older lane commits are not reported, and a ticket created before its oldest scanned commit is TRUNCATED")
		}
	}
	item := deliveryScreenValue(rows, ref, refCommit, observed, len(prs), len(scan.Commits), scan.Truncated)
	if laneRefs {
		scanned := make([]wire.Value, 0, len(lanes))
		for _, l := range lanes {
			scanned = append(scanned, wire.ObjectValue(wire.NewObject().Set("ref", wire.String(l.ref)).Set("commit", wire.String(l.commit)).Set("commitsScanned", count(len(l.scan.Commits))).Set("scanTruncated", wire.Bool(l.scan.Truncated))))
		}
		item.Obj.Set("laneHeads", wire.Array(scanned...))
		item.Obj.Set("laneScanTruncated", wire.Bool(laneTruncated))
		if tickets, ok := item.Obj.Get("tickets"); ok {
			for i, t := range tickets.Arr {
				t.Obj.Set("laneCommits", wire.Array(rows[i].lanes...))
			}
		}
	}
	res.Items = []wire.Value{item}
	res.Untrusted = true
	if clear == len(rows) {
		return res, 0
	}
	return res, 1
}

// deliveryScreenValue renders the taskman-delivery-screen/0 item (TCD-V0-161).
// laneScan is one local head's commits that the screened ref lacks.
type laneScan struct {
	ref, commit string
	scan        store.CommitScan
}

// scanLaneHeads reads, for every local branch and detached worktree head not
// contained in refCommit, at most limit of its commits outside refCommit
// (TLF-V0-887).
func scanLaneHeads(root, refCommit string, limit int) ([]laneScan, error) {
	heads, err := store.LaneHeads(root, refCommit)
	if err != nil {
		return nil, err
	}
	var out []laneScan
	for _, h := range heads {
		if store.IsAncestor(root, h.Commit, refCommit) {
			continue
		}
		scan, err := store.ScanCommits(root, h.Commit, refCommit, limit)
		if err != nil {
			return nil, err
		}
		out = append(out, laneScan{ref: h.Ref, commit: h.Commit, scan: scan})
	}
	return out, nil
}

// pastInProgress reports a progress phase that records delivered work not yet
// closed out: on a branch, in review, in the merge queue or merged.
func pastInProgress(phase string) bool {
	switch phase {
	case ticket.PhaseDeliveredOnBranch, ticket.PhaseInReview, ticket.PhaseInMergeQueue, ticket.PhaseMergedAwaitingCloseout:
		return true
	}
	return false
}

func deliveryScreenValue(rows []screenRow, ref, refCommit string, observed bool, prCount, scanned int, truncated bool) wire.Value {
	github := notObserved
	if observed {
		github = "OBSERVED"
	}
	tickets := make([]wire.Value, 0, len(rows))
	human := []string{"delivery-screen " + ref + ": " + strconv.Itoa(scanned) + " commit(s), GitHub " + github}
	for _, r := range rows {
		o := wire.NewObject()
		o.Set("ticketId", wire.String(r.ticketID))
		o.Set("local", wire.String(r.local))
		o.Set("status", wire.String(r.status))
		o.Set("verdict", wire.String(r.verdict))
		o.Set("prs", wire.Array(r.prs...))
		o.Set("commits", wire.Array(r.commits...))
		holds := make([]wire.Value, 0, len(r.holds))
		line := r.local + " " + r.verdict + " status=" + r.status + " prs=" + strconv.Itoa(len(r.prs)) + " commits=" + strconv.Itoa(len(r.commits))
		for _, h := range r.holds {
			holds = append(holds, wire.ObjectValue(wire.NewObject().Set("holdId", wire.String(h.HoldID)).Set("reason", wire.String(h.Reason))))
			line += " hold=" + h.HoldID
		}
		o.Set("holds", wire.Array(holds...))
		o.Set("operatorNote", r.note)
		if st, ok := r.note.Obj.Get("state"); ok && st.Str != "NONE" {
			line += " note=" + st.Str
		}
		tickets = append(tickets, wire.ObjectValue(o))
		human = append(human, line)
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(profileDeliveryScreen))
	o.Set("ref", wire.String(ref))
	o.Set("refCommit", wire.String(refCommit))
	o.Set("commitsScanned", count(scanned))
	o.Set("scanTruncated", wire.Bool(truncated))
	o.Set("github", wire.String(github))
	o.Set("prsExported", count(prCount))
	o.Set("tickets", wire.Array(tickets...))
	o.Set("human", wire.Strings(human))
	o.Set("mutationAuthority", wire.Bool(false))
	return wire.ObjectValue(o)
}

func emitDeliveryScreen(env Env, args []string) int {
	res, code := deliveryScreenCommand(env, args)
	if emit(env.Stdout, res) != 0 {
		return 2
	}
	return code
}

// batchInput is the parsed --pr, --prs and --runs input that `ticket
// batch-delivered` (TCD-V0-163) and `ticket batch-close` (TCD-V0-169) share,
// so both verbs read one export the same way.
type batchInput struct {
	number       int64
	batch        *ghPR
	byNumber     map[int64]*ghPR
	lanes        []int64
	runs         []ghRun
	runsObserved bool
	// green is the sorted successful run IDs at the batch merge commit or
	// the batch head.
	green []int64
	// excluded is every PR the batch text names that is not a lane PR
	// because its head is not proven in the merge's ancestry (TCD-V0-179).
	excluded []excludedLane
	// laneScan is the merge's lane commits (TCD-V0-731); nil when they are
	// NOT_OBSERVED.
	laneScan *store.CommitScan
}

// excludedLane is one PR the batch title or body names that was left out of
// the lane PRs, with why (TCD-V0-179).
type excludedLane struct {
	number int64
	reason string // NOT_IN_MERGE_ANCESTRY | HEAD_NOT_OBSERVED
}

// batchPRFields is the gh pr list --json field list batch-delivered and
// batch-close read (TCD-V0-180).
const batchPRFields = "number,state,title,body,headRefName,headRefOid,mergeCommit,labels,mergedAt"

// Reasons a referenced PR is not a lane PR (TCD-V0-179).
const (
	laneNotInMergeAncestry = "NOT_IN_MERGE_ANCESTRY"
	laneHeadNotObserved    = "HEAD_NOT_OBSERVED"
)

// laneAncestry classifies a referenced PR: "" when its exported head commit
// resolves in root and is an ancestor of the batch merge, else the reason it
// is excluded. An unexported, malformed or unfetched head, or an unfetched
// merge, is HEAD_NOT_OBSERVED: membership is never assumed.
func laneAncestry(root, merge string, p *ghPR) string {
	if _, err := wire.ParseOID("headRefOid", p.HeadRefOid); err != nil {
		return laneHeadNotObserved
	}
	if _, err := wire.ParseOID("mergeCommit", merge); err != nil {
		return laneHeadNotObserved
	}
	if _, err := store.ResolveCommit(root, p.HeadRefOid); err != nil {
		return laneHeadNotObserved
	}
	if _, err := store.ResolveCommit(root, merge); err != nil {
		return laneHeadNotObserved
	}
	if !store.IsAncestor(root, p.HeadRefOid, merge) {
		return laneNotInMergeAncestry
	}
	return ""
}

func (in *batchInput) excludedValue() wire.Value {
	out := make([]wire.Value, 0, len(in.excluded))
	for _, x := range in.excluded {
		out = append(out, wire.ObjectValue(wire.NewObject().Set("pr", count(int(x.number))).Set("reason", wire.String(x.reason))))
	}
	return wire.Array(out...)
}

// excludedWarning names the excluded PRs and the recovery; "" when none.
func (in *batchInput) excludedWarning() string {
	if len(in.excluded) == 0 {
		return ""
	}
	parts := make([]string, 0, len(in.excluded))
	for _, x := range in.excluded {
		parts = append(parts, "#"+strconv.FormatInt(x.number, 10)+" "+x.reason)
	}
	return "the batch names PRs that are not lane PRs, so their bodies deliver nothing (TCD-V0-179): " + strings.Join(parts, ", ") +
		"; a lane PR's headRefOid must be exported and be an ancestor of the merge commit in this repository (git fetch both), or name its tickets in a Refs line of the batch body"
}

// laneCommitsValue is the number of lane commits read, or NOT_OBSERVED
// (TCD-V0-731).
func (in *batchInput) laneCommitsValue() wire.Value {
	if in.laneScan == nil {
		return wire.String(notObserved)
	}
	return count(len(in.laneScan.Commits))
}

// laneWarnings says when the lane commits were not read or were cut short
// (TCD-V0-731).
func (in *batchInput) laneWarnings() []string {
	switch {
	case in.laneScan == nil:
		return []string{"the batch merge commit's lane commits are NOT_OBSERVED (TCD-V0-731): the merge commit must resolve in this repository with two parents (git fetch it); only PR text was read"}
	case in.laneScan.Truncated:
		return []string{"lane commit scan stopped at " + strconv.Itoa(defaultCloseoutScanLimit) + " commits; older lane commits deliver nothing (TCD-V0-731)"}
	}
	return nil
}

// batchHit is one ticket a batch delivered or only mentioned.
type batchHit struct {
	ticketID string
	lane     int64  // 0 = the batch text itself, or a lane commit
	commit   string // the lane commit that is the source, "" for PR text (TCD-V0-731)
}

// parseBatchInput reads the batch inputs of verb from already-parsed flags.
// --prs - reads the PR export from stdin (TCD-V0-171).
func parseBatchInput(env Env, verb string, fl map[string]string, pos []string) (*batchInput, error) {
	in := &batchInput{byNumber: map[int64]*ghPR{}}
	var err error
	if len(pos) != 0 {
		err = wire.Errorf(wire.CodeMalformed, "argv", "ticket %s takes no positional argument", verb)
	}
	if err == nil {
		in.number, err = strconv.ParseInt(fl["pr"], 10, 64)
		if err != nil || in.number < 1 {
			err = wire.Errorf(wire.CodeMalformed, "--pr", "--pr must be a PR number")
		}
	}
	if _, ok := fl["prs"]; err == nil && !ok {
		err = wire.Errorf(wire.CodeMalformed, "--prs", "ticket %s needs a --prs export", verb)
	}
	var prs []ghPR
	if err == nil {
		err = readGitHubExportArg(env, "--prs", fl["prs"], &prs)
	}
	runsFile, runsObserved := fl["runs"]
	in.runsObserved = runsObserved
	if err == nil && runsObserved {
		err = readGitHubExport("--runs", commandPath(env.Cwd, runsFile), &in.runs)
	}
	for i := range prs {
		in.byNumber[prs[i].Number] = &prs[i]
	}
	in.batch = in.byNumber[in.number]
	switch {
	case err != nil:
	case in.batch == nil:
		err = wire.Errorf(wire.CodeMalformed, "--pr", "PR #%d is not in the --prs export", in.number)
	case in.batch.State != "MERGED":
		err = wire.Errorf(wire.CodePrerequisiteUnsatisfied, "--pr", "PR #%d is %s, not merged", in.number, prose(in.batch.State))
	case in.batch.mergeOid() == "":
		err = wire.Errorf(wire.CodeMalformed, "--prs", "PR #%d is MERGED but the --prs export has no mergeCommit for it (TCD-V0-180); export it with gh pr list --state merged --json %s", in.number, batchPRFields)
	}
	if err != nil {
		return nil, err
	}
	var root string
	if repo, e := intent.Resolve(env.Cwd); e == nil {
		root = repo.PrimaryWorktree
	}
	in.deriveLanes(root)
	in.scanLanes(root)
	for _, r := range in.runs {
		if r.Conclusion == "success" && (r.HeadSha == in.batch.mergeOid() || r.HeadSha == in.batch.HeadRefOid) && !slices.Contains(in.green, r.DatabaseID) {
			in.green = append(in.green, r.DatabaseID)
		}
	}
	slices.Sort(in.green)
	return in, nil
}

// scanLanes reads the merge's lane commits (TCD-V0-731); laneScan stays nil
// when root is empty or the merge does not resolve with two parents.
func (in *batchInput) scanLanes(root string) {
	if root == "" {
		return
	}
	if scan, e := store.ScanLaneCommits(root, in.batch.mergeOid(), defaultCloseoutScanLimit); e == nil {
		in.laneScan = &scan
	}
}

// deriveLanes sets the lane PRs: the other exported PRs the batch title or
// body names as #N whose heads root proves in the merge's ancestry; the rest
// are excluded with their reason (TCD-V0-163, TCD-V0-179). An empty root
// proves nothing.
func (in *batchInput) deriveLanes(root string) {
	seen := map[int64]bool{}
	for _, m := range prNumberRef.FindAllStringSubmatch(in.batch.Title+"\n"+in.batch.Body, -1) {
		n, _ := strconv.ParseInt(m[1], 10, 64)
		if n == in.number || in.byNumber[n] == nil || seen[n] {
			continue
		}
		seen[n] = true
		reason := laneHeadNotObserved
		if root != "" {
			reason = laneAncestry(root, in.batch.mergeOid(), in.byNumber[n])
		}
		if reason != "" {
			in.excluded = append(in.excluded, excludedLane{n, reason})
			continue
		}
		in.lanes = append(in.lanes, n)
	}
	slices.Sort(in.lanes)
	slices.SortFunc(in.excluded, func(a, b excludedLane) int { return int(a.number - b.number) })
}

// batchHits indexes the store and returns the tickets the batch delivered
// (a delivery line in a lane PR or the batch PR) and those it only
// mentioned (TCD-V0-163..164, TCD-V0-870). A delivered ticket is never also
// mentioned.
func (in *batchInput) batchHits(rc *readCtx) (delivered, mentioned []batchHit) {
	index := &closeoutIndex{tokens: map[string]string{}, issues: map[string][]string{}}
	for _, id := range rc.store.Inventory.Sorted() {
		rec, _ := rc.store.Inventory.Get(id)
		index.add(rec)
	}
	return in.hits(index)
}

// hits is batchHits over an already-built index.
func (in *batchInput) hits(index *closeoutIndex) (delivered, mentioned []batchHit) {
	seen := map[string]bool{}
	var weak []batchHit
	hit := func(refs map[string]string, n int64, commit string) {
		ids := make([]string, 0, len(refs))
		for id := range refs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			switch {
			case refs[id] == refDelivered && !seen[id]:
				seen[id] = true
				delivered = append(delivered, batchHit{id, n, commit})
			case refs[id] == refMentioned:
				weak = append(weak, batchHit{id, n, commit})
			}
		}
	}
	for _, n := range append(slices.Clone(in.lanes), 0) {
		p := in.batch
		if n != 0 {
			p = in.byNumber[n]
		}
		hit(index.prReferences(p), n, "")
	}
	if in.laneScan != nil {
		for _, c := range in.laneScan.Commits {
			hit(index.commitReferences(c.Message), 0, c.Commit)
		}
	}
	for _, h := range weak {
		if !seen[h.ticketID] {
			seen[h.ticketID] = true
			mentioned = append(mentioned, h)
		}
	}
	return delivered, mentioned
}

// batchDeliveredCommand is `corvint-tasks ticket batch-delivered`: the
// tickets a merged batch PR delivered, each with its lane PR, the batch PR,
// the main merge commit and the green CI runs at it (TCD-V0-163). It writes
// nothing and completes nothing; mentioned tickets are listed apart
// (TCD-V0-164).
func batchDeliveredCommand(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "batch-delivered"}
	fl, pos, err := flags(args, "pr", "prs", "runs")
	var in *batchInput
	if err == nil {
		in, err = parseBatchInput(env, "batch-delivered", fl, pos)
	}
	if err != nil {
		return failure(cmd, nil, err)
	}
	number, batch, lanes, runsObserved := in.number, in.batch, in.lanes, in.runsObserved
	var green []wire.Value
	for _, id := range in.green {
		green = append(green, wire.String(strconv.FormatInt(id, 10)))
	}
	var delivered, mentioned []batchHit
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		delivered, mentioned = in.batchHits(rc)
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	status := func(id string) (string, string) {
		rec, _ := rc.store.Inventory.Get(id)
		return rec.TicketID.Local, rec.Status
	}
	lanePR := func(n int64) wire.Value {
		if n == 0 {
			return wire.Null()
		}
		return count(int(n))
	}
	greenValue := wire.String(notObserved)
	if runsObserved {
		greenValue = wire.Array(green...)
	}
	human := []string{"batch-delivered #" + strconv.FormatInt(number, 10) + " at " + batch.mergeOid() + ": " + strconv.Itoa(len(delivered)) + " delivered, " + strconv.Itoa(len(mentioned)) + " mentioned, green runs " + strconv.Itoa(len(green))}
	if !runsObserved {
		human[0] = strings.TrimSuffix(human[0], strconv.Itoa(len(green))) + notObserved
	}
	rows := make([]wire.Value, 0, len(delivered))
	for _, h := range delivered {
		local, st := status(h.ticketID)
		o := wire.NewObject()
		o.Set("ticketId", wire.String(h.ticketID))
		o.Set("local", wire.String(local))
		o.Set("status", wire.String(st))
		o.Set("lanePr", lanePR(h.lane))
		o.Set("commit", orNull(h.commit))
		o.Set("batchPr", wire.String(strconv.FormatInt(number, 10)))
		o.Set("mergeCommit", wire.String(batch.mergeOid()))
		o.Set("greenRuns", greenValue)
		rows = append(rows, wire.ObjectValue(o))
		human = append(human, local+" "+st+" lane="+lanePRText(h.lane))
	}
	weakRows := make([]wire.Value, 0, len(mentioned))
	for _, h := range mentioned {
		local, st := status(h.ticketID)
		o := wire.NewObject()
		o.Set("ticketId", wire.String(h.ticketID))
		o.Set("local", wire.String(local))
		o.Set("status", wire.String(st))
		o.Set("pr", lanePR(h.lane))
		o.Set("commit", orNull(h.commit))
		weakRows = append(weakRows, wire.ObjectValue(o))
	}
	lanesValue := make([]wire.Value, 0, len(lanes))
	for _, n := range lanes {
		lanesValue = append(lanesValue, count(int(n)))
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(profileBatchDelivered))
	o.Set("batchPr", wire.String(strconv.FormatInt(number, 10)))
	o.Set("mergeCommit", wire.String(batch.mergeOid()))
	o.Set("batchHead", orNull(batch.HeadRefOid))
	o.Set("lanePrs", wire.Array(lanesValue...))
	o.Set("excludedPrs", in.excludedValue())
	o.Set("laneCommits", in.laneCommitsValue())
	o.Set("laneCommitsTruncated", wire.Bool(in.laneScan != nil && in.laneScan.Truncated))
	o.Set("greenRuns", greenValue)
	o.Set("delivered", wire.Array(rows...))
	o.Set("mentioned", wire.Array(weakRows...))
	o.Set("human", wire.Strings(human))
	o.Set("mutationAuthority", wire.Bool(false))
	res := success(cmd, rc)
	if !runsObserved {
		res.Warnings = append(res.Warnings, "no --runs export; CI runs NOT_OBSERVED")
	}
	if w := in.excludedWarning(); w != "" {
		res.Warnings = append(res.Warnings, w)
	}
	res.Warnings = append(res.Warnings, in.laneWarnings()...)
	res.Items = []wire.Value{wire.ObjectValue(o)}
	res.Untrusted = true
	return res
}

func lanePRText(n int64) string {
	if n == 0 {
		return "batch"
	}
	return "#" + strconv.FormatInt(n, 10)
}
