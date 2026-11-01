package cli

import (
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Orphan lane heads (docs/specs/corvint-tasks-cli-v0.md, TCL-V0-744;
// V1-3856): a lane head that delivered a ticket but was never merged into a
// batch or the integration ref is invisible to dispatch, so the ticket is
// reassigned and rebuilt. `ticket orphan-lanes` lists every lane commit that
// delivers an OPEN ticket and that neither the integration ref nor any
// in-flight batch branch contains.

const (
	profileOrphanLanes = "taskman-orphan-lanes/0"
	defaultBatchGlob   = "claude/batch-*"
	// maxOrphanRows bounds the listed rows; orphanCount stays whole.
	maxOrphanRows = 500
)

var (
	orphanLanesBools  = []string{"json"}
	orphanLanesValues = []string{"ref", "batch", "limit"}
)

func orphanLanesCommand(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "orphan-lanes"}
	fl, _, pos, err := flagsBool(args, orphanLanesBools, orphanLanesValues...)
	if err == nil && len(pos) > 0 {
		err = wire.Errorf(wire.CodeMalformed, "argv", "ticket orphan-lanes takes no ticket ids; it scans every lane head")
	}
	glob := defaultBatchGlob
	if v, ok := fl["batch"]; ok {
		glob = v
	}
	if _, e := path.Match(glob, ""); err == nil && (e != nil || glob == "") {
		err = wire.Errorf(wire.CodeMalformed, "--batch", "--batch takes one branch glob such as claude/batch-*")
	}
	limit := 0
	if err == nil {
		limit, err = parseScanLimit(fl)
	}
	if err != nil {
		return failure(cmd, nil, err)
	}
	var (
		index  *closeoutIndex
		open   map[string]string // ticket ID -> local ID, OPEN only
		branch string
	)
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		index, open = &closeoutIndex{tokens: map[string]string{}, issues: map[string][]string{}}, map[string]string{}
		branch = rc.store.Queue.IntentBranch
		for _, id := range rc.store.Inventory.Sorted() {
			rec, _ := rc.store.Inventory.Get(id)
			index.add(rec)
			if rec.Status == ticket.StatusOpen {
				open[id] = rec.TicketID.Local
			}
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	root := rc.repo.PrimaryWorktree
	ref, refCommit, _, err := closeoutRef(root, branch, fl)
	var heads []store.LaneHead
	if err == nil {
		heads, err = store.LaneHeads(root, refCommit)
	}
	// Every commit an in-flight batch holds beyond the ref is merged work,
	// never an orphan. A batch scan cut at --limit leaves its older commits
	// unknown, so the result is marked truncated.
	inBatch := map[string]bool{}
	var batchRefs, lanes []wire.Value
	batchTruncated := false
	var laneHeads []store.LaneHead
	for _, h := range heads {
		if err != nil {
			break
		}
		short := strings.TrimPrefix(h.Ref, "refs/heads/")
		if ok, _ := path.Match(glob, short); !ok {
			laneHeads = append(laneHeads, h)
			continue
		}
		var scan store.CommitScan
		if scan, err = store.ScanCommits(root, h.Commit, refCommit, limit); err != nil {
			break
		}
		batchTruncated = batchTruncated || scan.Truncated
		for _, c := range scan.Commits {
			inBatch[c.Commit] = true
		}
		batchRefs = append(batchRefs, wire.ObjectValue(wire.NewObject().Set("ref", wire.String(h.Ref)).Set("commit", wire.String(h.Commit)).
			Set("scanTruncated", wire.Bool(scan.Truncated))))
	}
	var rows []wire.Value
	orphans, laneTruncated := 0, false
	seen := map[string]bool{} // commit+ticket, so a commit on two heads is listed once
	for _, h := range laneHeads {
		if err != nil {
			break
		}
		var scan store.CommitScan
		if scan, err = store.ScanCommits(root, h.Commit, refCommit, limit); err != nil {
			break
		}
		laneTruncated = laneTruncated || scan.Truncated
		n := 0
		for _, c := range scan.Commits {
			if inBatch[c.Commit] {
				continue
			}
			for id, kind := range index.commitReferences(c.Message) {
				local, isOpen := open[id]
				if kind != refDelivered || !isOpen || seen[c.Commit+" "+id] {
					continue
				}
				seen[c.Commit+" "+id] = true
				n++
				orphans++
				if len(rows) < maxOrphanRows {
					rows = append(rows, wire.ObjectValue(wire.NewObject().Set("ref", wire.String(h.Ref)).Set("commit", wire.String(c.Commit)).
						Set("ticketId", wire.String(id)).Set("local", wire.String(local)).
						Set("subject", wire.String(truncateUTF8(c.Subject, maxCloseoutSubjectBytes)))))
				}
			}
		}
		lanes = append(lanes, wire.ObjectValue(wire.NewObject().Set("ref", wire.String(h.Ref)).Set("commit", wire.String(h.Commit)).
			Set("orphanCount", count(n)).Set("scanTruncated", wire.Bool(scan.Truncated))))
	}
	if err != nil {
		return failure(cmd, rc, err)
	}
	sortOrphanRows(rows)
	o := wire.NewObject().Set("profile", wire.String(profileOrphanLanes)).Set("ref", wire.String(ref)).Set("refCommit", wire.String(refCommit)).
		Set("batchGlob", wire.String(glob)).Set("batchRefs", wire.Array(batchRefs...)).Set("batchScanTruncated", wire.Bool(batchTruncated)).
		Set("laneHeads", wire.Array(lanes...)).Set("laneScanTruncated", wire.Bool(laneTruncated)).
		Set("orphanCount", count(orphans)).Set("orphans", wire.Array(rows...))
	res := success(cmd, rc)
	res.Items = []wire.Value{wire.ObjectValue(o)}
	if len(batchRefs) == 0 {
		res.Warnings = append(res.Warnings, "no local branch matches the batch glob "+glob+", so in-flight batches are NOT_OBSERVED and a lane commit merged only into a batch this clone lacks is listed as an orphan; fetch or push the batch branches to this clone first (TCL-V0-744)")
	}
	if batchTruncated || laneTruncated {
		res.Warnings = append(res.Warnings, "a lane or batch scan stopped at --limit, so the orphan list is incomplete; raise --limit (TCL-V0-744)")
	}
	return res
}

// sortOrphanRows orders rows by ticket, then ref, then commit.
func sortOrphanRows(rows []wire.Value) {
	key := func(v wire.Value) string {
		g := func(k string) string { s, _ := v.Obj.Get(k); return s.Str }
		return g("ticketId") + "\x00" + g("ref") + "\x00" + g("commit")
	}
	sort.Slice(rows, func(i, j int) bool { return key(rows[i]) < key(rows[j]) })
}
