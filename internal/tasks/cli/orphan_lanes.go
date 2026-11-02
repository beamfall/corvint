package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

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
	// The batch branches come from every local branch name, not from
	// LaneHeads, which keeps one ref per commit: a lane branch and a batch
	// branch at the same commit must still count as batched.
	var batchHeads []store.LaneHead
	if err == nil {
		batchHeads, err = orphanBatchHeads(rc.repo.CommonDir, glob)
	}
	// Every commit an in-flight batch holds beyond the ref is merged work,
	// never an orphan. A batch scan cut at --limit leaves its older commits
	// unread, so a candidate it may hold is checked against that batch head
	// with merge-base --is-ancestor; an unanswered check is UNKNOWN, never an
	// orphan.
	inBatch := map[string]bool{}
	var batchRefs, lanes, unknownRows []wire.Value
	batchTruncated := false
	var truncatedBatches []string
	for _, h := range batchHeads {
		if err != nil {
			break
		}
		var scan store.CommitScan
		if scan, err = store.ScanCommits(root, h.Commit, refCommit, limit); err != nil {
			break
		}
		if scan.Truncated {
			batchTruncated = true
			truncatedBatches = append(truncatedBatches, h.Commit)
		}
		for _, c := range scan.Commits {
			inBatch[c.Commit] = true
		}
		batchRefs = append(batchRefs, wire.ObjectValue(wire.NewObject().Set("ref", wire.String(h.Ref)).Set("commit", wire.String(h.Commit)).
			Set("scanTruncated", wire.Bool(scan.Truncated))))
	}
	// batched is true when a truncated batch holds commit, false when none
	// does, and an error when git could not say.
	batched := func(commit string) (bool, error) {
		for _, b := range truncatedBatches {
			if in, e := orphanIsAncestor(rc.repo.CommonDir, commit, b); e != nil || in {
				return in, e
			}
		}
		return false, nil
	}
	var laneHeads []store.LaneHead
	for _, h := range heads {
		if ok, _ := path.Match(glob, strings.TrimPrefix(h.Ref, "refs/heads/")); !ok {
			laneHeads = append(laneHeads, h)
		}
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
				row := wire.NewObject().Set("ref", wire.String(h.Ref)).Set("commit", wire.String(c.Commit)).
					Set("ticketId", wire.String(id)).Set("local", wire.String(local)).
					Set("subject", wire.String(truncateUTF8(c.Subject, maxCloseoutSubjectBytes)))
				if in, e := batched(c.Commit); e != nil {
					if len(unknownRows) < maxOrphanRows {
						unknownRows = append(unknownRows, wire.ObjectValue(row))
					}
					continue
				} else if in {
					continue
				}
				n++
				orphans++
				if len(rows) < maxOrphanRows {
					rows = append(rows, wire.ObjectValue(row))
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
		Set("orphanCount", count(orphans)).Set("orphans", wire.Array(rows...)).Set("unknown", wire.Array(unknownRows...))
	res := success(cmd, rc)
	res.Items = []wire.Value{wire.ObjectValue(o)}
	if len(batchRefs) == 0 {
		res.Warnings = append(res.Warnings, "no local branch matches the batch glob "+glob+", so in-flight batches are NOT_OBSERVED and a lane commit merged only into a batch this clone lacks is listed as an orphan; fetch or push the batch branches to this clone first (TCL-V0-744)")
	}
	if batchTruncated || laneTruncated {
		res.Warnings = append(res.Warnings, "a lane or batch scan stopped at --limit, so the orphan list is incomplete; raise --limit (TCL-V0-744)")
	}
	if len(unknownRows) > 0 {
		res.Warnings = append(res.Warnings, strconv.Itoa(len(unknownRows))+" lane commit(s) are UNKNOWN, not orphans: git could not say whether a truncated batch holds them; raise --limit (TCL-V0-744)")
	}
	return res
}

// orphanBatchHeads is every local branch whose name under refs/heads/
// matches glob, each ref kept even when another branch is at its commit.
func orphanBatchHeads(commonDir, glob string) ([]store.LaneHead, error) {
	out, code, err := orphanGit(commonDir, "for-each-ref", "--format=%(objectname) %(refname)", "refs/heads/")
	if err != nil || code != 0 {
		return nil, wire.Errorf(wire.CodeUnsupported, "git for-each-ref", "listing the local branches failed (exit %d): %v", code, err)
	}
	var heads []store.LaneHead
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		commit, ref, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		if m, _ := path.Match(glob, strings.TrimPrefix(ref, "refs/heads/")); m {
			heads = append(heads, store.LaneHead{Ref: ref, Commit: commit})
		}
	}
	return heads, nil
}

// orphanIsAncestor reports whether commit is reachable from head: git exit 0
// is true, 1 is false, anything else is an error.
func orphanIsAncestor(commonDir, commit, head string) (bool, error) {
	_, code, err := orphanGit(commonDir, "merge-base", "--is-ancestor", "--end-of-options", commit, head)
	switch {
	case err != nil:
		return false, err
	case code == 0:
		return true, nil
	case code == 1:
		return false, nil
	}
	return false, wire.Errorf(wire.CodeUnsupported, "git merge-base", "merge-base --is-ancestor exited %d", code)
}

// orphanGit runs one bounded, read-only git command against the common
// directory with the sanitized plan-batches environment, returning stdout
// and the exit status.
func orphanGit(commonDir string, args ...string) (string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "git", append([]string{"-c", "credential.helper=", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=" + os.DevNull}, args...)...)
	c.Dir = commonDir
	c.Env = append(batchGitEnvironment(), "GIT_DIR="+commonDir)
	c.WaitDelay = time.Second
	out := &boundedBuffer{max: 1 << 20}
	c.Stdout = out
	err := c.Run()
	if out.over {
		return "", 0, wire.Errorf(wire.CodeLimitExceeded, "git "+args[0], "git output exceeds 1 MiB")
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out.buf.String(), exit.ExitCode(), nil
	}
	if err != nil {
		return "", 0, err
	}
	return out.buf.String(), 0, nil
}

// sortOrphanRows orders rows by ticket, then ref, then commit.
func sortOrphanRows(rows []wire.Value) {
	key := func(v wire.Value) string {
		g := func(k string) string { s, _ := v.Obj.Get(k); return s.Str }
		return g("ticketId") + "\x00" + g("ref") + "\x00" + g("commit")
	}
	sort.Slice(rows, func(i, j int) bool { return key(rows[i]) < key(rows[j]) })
}
