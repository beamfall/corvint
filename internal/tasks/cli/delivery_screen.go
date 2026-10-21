package cli

import (
	"bytes"
	"encoding/json"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/store"
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
	verdictMentioned = "MENTIONED"
	verdictClear     = "CLEAR"
)

var (
	// deliveryLead and deliveryTail make a PR-body line a delivery claim
	// (TCD-V0-160): it starts with Refs, Closes, Fixes, Delivers or
	// Delivered, or it says DELIVERED in capitals.
	deliveryLead = regexp.MustCompile(`(?i)^\W*(?:refs|closes|fixes|delivers|delivered)\b`)
	deliveryTail = regexp.MustCompile(`\bDELIVERED\b`)
	// deliveryVeto marks a line that names a ticket it did not deliver.
	deliveryVeto = regexp.MustCompile(`(?i)not[ -]started|not[ -]delivered|skipped|residual|duplicate|blocked|already`)
	// prNumberRef is a `#N` PR reference in a batch title or body.
	prNumberRef = regexp.MustCompile(`#([0-9]{1,9})\b`)
)

// ghPR is one record of `gh pr list --json
// number,state,title,body,headRefName,headRefOid,mergeCommit,labels`.
type ghPR struct {
	Number      int64  `json:"number"`
	State       string `json:"state"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	HeadRefName string `json:"headRefName"`
	HeadRefOid  string `json:"headRefOid"`
	MergeCommit *struct {
		Oid string `json:"oid"`
	} `json:"mergeCommit"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// ghRun is one record of `gh run list --json databaseId,headSha,conclusion`.
type ghRun struct {
	DatabaseID int64  `json:"databaseId"`
	HeadSha    string `json:"headSha"`
	Conclusion string `json:"conclusion"`
}

// readGitHubExport decodes a bounded `gh --json` array file into out.
func readGitHubExport(flag, file string, out any) error {
	raw, err := readBoundedFile(flag, file, maxGitHubExportBytes)
	if err != nil {
		return err
	}
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

// references maps each ticket text names to delivered or mentioned
// (TCD-V0-160). In a PR body only a lead or DELIVERED line delivers; in a
// commit message (anyLine) every line does. A vetoed line only mentions.
// Delivered on any line wins.
func (x *closeoutIndex) references(text string, anyLine bool) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		ids := x.lineTickets(line)
		if len(ids) == 0 {
			continue
		}
		strong := !deliveryVeto.MatchString(line) && (anyLine || deliveryLead.MatchString(line) || deliveryTail.MatchString(line))
		for _, id := range ids {
			if strong {
				out[id] = refDelivered
			} else if out[id] == "" {
				out[id] = refMentioned
			}
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
	prs, commits                     []wire.Value
}

// deliveryScreenCommand is `corvint-tasks ticket delivery-screen`. It takes
// no lock and writes nothing (TCD-V0-159); exit 0 when every ticket is
// CLEAR, 1 otherwise, 2 on a refusal (TCD-V0-162).
func deliveryScreenCommand(env Env, args []string) (*wire.Result, int) {
	cmd := []string{"ticket", "delivery-screen"}
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
		err = readGitHubExport("--prs", file, &prs)
	}
	if err != nil {
		return failure(cmd, nil, err), 0
	}
	var (
		index  *closeoutIndex
		rows   []screenRow
		branch string
	)
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		index = &closeoutIndex{tokens: map[string]string{}, issues: map[string][]string{}}
		rows, branch = nil, rc.store.Queue.IntentBranch
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
				rows = append(rows, screenRow{ticketID: id, local: rec.TicketID.Local, status: rec.Status})
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
	if err != nil {
		return failure(cmd, rc, err), 0
	}
	commitRefs := make([]map[string]string, len(scan.Commits))
	for i, c := range scan.Commits {
		commitRefs[i] = index.references(c.Message, true)
	}
	prRefs := make([]map[string]string, len(prs))
	for i := range prs {
		prRefs[i] = index.references(prs[i].Title+"\n"+prs[i].Body, false)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].ticketID < rows[j].ticketID })
	clear := 0
	for i := range rows {
		r := &rows[i]
		inFlight, delivered, mentioned := false, false, false
		for j, c := range scan.Commits {
			kind := commitRefs[j][r.ticketID]
			if kind == "" {
				continue
			}
			delivered, mentioned = delivered || kind == refDelivered, true
			if len(r.commits) < maxScreenHits {
				o := wire.NewObject()
				o.Set("commit", wire.String(c.Commit))
				o.Set("subject", wire.String(truncateUTF8(c.Subject, maxCloseoutSubjectBytes)))
				o.Set("reference", wire.String(kind))
				r.commits = append(r.commits, wire.ObjectValue(o))
			}
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
				o.Set("number", count(int(p.Number)))
				o.Set("state", wire.String(prose(p.State)))
				o.Set("reference", wire.String(kind))
				o.Set("batch", wire.Bool(p.batch()))
				r.prs = append(r.prs, wire.ObjectValue(o))
			}
		}
		switch {
		case inFlight:
			r.verdict = verdictInFlight
		case delivered:
			r.verdict = verdictDelivered
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
		res.Warnings = append(res.Warnings, "commit scan stopped at --limit "+strconv.Itoa(limit)+"; older references are not reported")
	}
	if !observed {
		res.Warnings = append(res.Warnings, "no --prs export; open and merged PRs NOT_OBSERVED")
	}
	res.Items = []wire.Value{deliveryScreenValue(rows, ref, refCommit, observed, len(prs), len(scan.Commits), scan.Truncated)}
	res.Untrusted = true
	if clear == len(rows) {
		return res, 0
	}
	return res, 1
}

// deliveryScreenValue renders the taskman-delivery-screen/0 item (TCD-V0-161).
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
		tickets = append(tickets, wire.ObjectValue(o))
		human = append(human, r.local+" "+r.verdict+" status="+r.status+" prs="+strconv.Itoa(len(r.prs))+" commits="+strconv.Itoa(len(r.commits)))
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

// batchDeliveredCommand is `corvint-tasks ticket batch-delivered`: the
// tickets a merged batch PR delivered, each with its lane PR, the batch PR,
// the main merge commit and the green CI runs at it (TCD-V0-163). It writes
// nothing and completes nothing; mentioned tickets are listed apart
// (TCD-V0-164).
func batchDeliveredCommand(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "batch-delivered"}
	fl, pos, err := flags(args, "pr", "prs", "runs")
	if err == nil && len(pos) != 0 {
		err = wire.Errorf(wire.CodeMalformed, "argv", "ticket batch-delivered takes no positional argument")
	}
	var number int64
	if err == nil {
		number, err = strconv.ParseInt(fl["pr"], 10, 64)
		if err != nil || number < 1 {
			err = wire.Errorf(wire.CodeMalformed, "--pr", "--pr must be a PR number")
		}
	}
	if _, ok := fl["prs"]; err == nil && !ok {
		err = wire.Errorf(wire.CodeMalformed, "--prs", "ticket batch-delivered needs a --prs export")
	}
	var prs []ghPR
	if err == nil {
		err = readGitHubExport("--prs", fl["prs"], &prs)
	}
	var runs []ghRun
	runsFile, runsObserved := fl["runs"]
	if err == nil && runsObserved {
		err = readGitHubExport("--runs", runsFile, &runs)
	}
	byNumber := map[int64]*ghPR{}
	for i := range prs {
		byNumber[prs[i].Number] = &prs[i]
	}
	batch := byNumber[number]
	switch {
	case err != nil:
	case batch == nil:
		err = wire.Errorf(wire.CodeMalformed, "--pr", "PR #%d is not in the --prs export", number)
	case batch.State != "MERGED" || batch.mergeOid() == "":
		err = wire.Errorf(wire.CodePrerequisiteUnsatisfied, "--pr", "PR #%d is %s, not merged", number, prose(batch.State))
	}
	if err != nil {
		return failure(cmd, nil, err)
	}
	var lanes []int64
	for _, m := range prNumberRef.FindAllStringSubmatch(batch.Title+"\n"+batch.Body, -1) {
		n, _ := strconv.ParseInt(m[1], 10, 64)
		if n != number && byNumber[n] != nil && !slices.Contains(lanes, n) {
			lanes = append(lanes, n)
		}
	}
	slices.Sort(lanes)
	var green []wire.Value
	if runsObserved {
		var ids []int64
		for _, r := range runs {
			if r.Conclusion == "success" && (r.HeadSha == batch.mergeOid() || r.HeadSha == batch.HeadRefOid) && !slices.Contains(ids, r.DatabaseID) {
				ids = append(ids, r.DatabaseID)
			}
		}
		slices.Sort(ids)
		for _, id := range ids {
			green = append(green, wire.String(strconv.FormatInt(id, 10)))
		}
	}
	type hit struct {
		ticketID string
		lane     int64 // 0 = the batch body itself
	}
	var delivered, mentioned []hit
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		index := &closeoutIndex{tokens: map[string]string{}, issues: map[string][]string{}}
		for _, id := range rc.store.Inventory.Sorted() {
			rec, _ := rc.store.Inventory.Get(id)
			index.add(rec)
		}
		delivered, mentioned = nil, nil
		seen := map[string]bool{}
		var weak []hit
		for _, n := range append(slices.Clone(lanes), 0) {
			p := batch
			if n != 0 {
				p = byNumber[n]
			}
			refs := index.references(p.Body, false)
			ids := make([]string, 0, len(refs))
			for id := range refs {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			for _, id := range ids {
				switch {
				case refs[id] == refDelivered && !seen[id]:
					seen[id] = true
					delivered = append(delivered, hit{id, n})
				case refs[id] == refMentioned:
					weak = append(weak, hit{id, n})
				}
			}
		}
		for _, h := range weak {
			if !seen[h.ticketID] {
				seen[h.ticketID] = true
				mentioned = append(mentioned, h)
			}
		}
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
		o.Set("batchPr", count(int(number)))
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
		weakRows = append(weakRows, wire.ObjectValue(o))
	}
	lanesValue := make([]wire.Value, 0, len(lanes))
	for _, n := range lanes {
		lanesValue = append(lanesValue, count(int(n)))
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(profileBatchDelivered))
	o.Set("batchPr", count(int(number)))
	o.Set("mergeCommit", wire.String(batch.mergeOid()))
	o.Set("batchHead", orNull(batch.HeadRefOid))
	o.Set("lanePrs", wire.Array(lanesValue...))
	o.Set("greenRuns", greenValue)
	o.Set("delivered", wire.Array(rows...))
	o.Set("mentioned", wire.Array(weakRows...))
	o.Set("human", wire.Strings(human))
	o.Set("mutationAuthority", wire.Bool(false))
	res := success(cmd, rc)
	if !runsObserved {
		res.Warnings = append(res.Warnings, "no --runs export; CI runs NOT_OBSERVED")
	}
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
