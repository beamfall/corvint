package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Plan batches bounds (NTP-V0-021, NTP-V0-024). A larger queue of candidates
// refuses LIMIT_EXCEEDED rather than truncating.
const (
	batchDefaultGroupSize = 8
	batchMaxGroupSize     = 64
	batchMaxCandidates    = 256
	batchMaxSharedPaths   = 64
	batchMaxConflictPaths = 16
	batchGitOutputBytes   = 16 << 20
	batchGitTimeout       = 2 * time.Minute
)

// batchPhases are the attempt phases that carry a submitted or ready
// candidate (NTP-V0-020).
var batchPhases = map[string]bool{"BUILT": true, "CHECKING": true, "REVIEWING": true, "READY_FOR_INTEGRATION": true}

// batchCandidate is one ticket's latest candidate attempt as the read saw it.
type batchCandidate struct {
	ticketID, attemptID, base, branch string
	tree                              *string
	// resolved by the dry-merge pass
	baseTree, candTree string
	paths              []string
}

type batchSplit struct {
	ticketID string
	group    int
	paths    []string
}

type batchUngrouped struct {
	ticketID, attemptID, reason, detail string
}

type batchGroup struct {
	members []*batchCandidate
	tree    string
}

// planBatches is the NTP-V0-020 read-only `plan batches` verb. The store
// read is the ordinary lock-free TM-V0-008 read; every Git observation runs
// afterwards against a throw-away object directory, so the repository
// gains no object and no ref (NTP-V0-022).
func planBatches(env Env, args []string) *wire.Result {
	cmd := []string{"plan", "batches"}
	maxSize, sizeSeen := batchDefaultGroupSize, false
	var shared []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--max-group-size":
			if sizeSeen || i+1 >= len(args) {
				return usage(cmd, "--max-group-size needs one value, once")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 || n > batchMaxGroupSize || strconv.Itoa(n) != args[i] {
				return usage(cmd, "--max-group-size must be a decimal integer from 1 to 64")
			}
			maxSize, sizeSeen = n, true
		case "--shared-path":
			if i+1 >= len(args) {
				return usage(cmd, "--shared-path needs one repository-relative path")
			}
			i++
			p, err := wire.ParsePath("--shared-path", args[i])
			if err != nil {
				return usage(cmd, "--shared-path must be a repository-relative path; a trailing / names a directory prefix")
			}
			shared = append(shared, p)
		default:
			return usage(cmd, "unknown plan batches flag")
		}
	}
	if shared = scopePaths(shared); shared == nil {
		shared = []string{}
	}
	if len(shared) > batchMaxSharedPaths {
		return usage(cmd, "at most 64 --shared-path values")
	}
	var cands []*batchCandidate
	var queueID, branch string
	rc, err := withStore(env, func(rc *readCtx) error {
		in, _, err := planInput(rc)
		if err != nil {
			return err
		}
		queueID, branch = rc.store.Queue.QueueID.Raw, rc.store.Queue.IntentBranch
		cands = batchCandidates(in.Tickets, in.Attempts)
		if len(cands) > batchMaxCandidates {
			return wire.Errorf(wire.CodeLimitExceeded, "plan batches", "%d candidates exceed the bound of %d", len(cands), batchMaxCandidates)
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	item, err := batchPlan(rc, queueID, branch, cands, shared, maxSize)
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{item}
	res.MaxNodes = planNodeBound(len(cands))
	return res
}

// batchCandidates is the latest-generation candidate attempt of each OPEN or
// HELD ticket, in plan order: priority, numeric order, ticket ID.
func batchCandidates(inv *ticket.Inventory, attempts map[string]*snapshot.Attempt) []*batchCandidate {
	latest := map[string]*snapshot.Attempt{}
	for _, a := range attempts {
		if !batchPhases[a.Phase] || (a.CandidateTreeOid == nil && a.Branch == "") {
			continue
		}
		if p := latest[a.TicketID.Raw]; p == nil || a.Generation.Uint64() > p.Generation.Uint64() {
			latest[a.TicketID.Raw] = a
		}
	}
	recs := []*ticket.Record{}
	for _, id := range inv.IDs() {
		rec, _ := inv.Get(id)
		if (rec.Status == ticket.StatusOpen || rec.Status == ticket.StatusHeld) && latest[rec.TicketID.Raw] != nil {
			recs = append(recs, rec)
		}
	}
	sort.Slice(recs, func(i, j int) bool {
		a, b := recs[i], recs[j]
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if a.Order.Int() != b.Order.Int() {
			return a.Order.Int() < b.Order.Int()
		}
		return a.TicketID.Raw < b.TicketID.Raw
	})
	out := make([]*batchCandidate, 0, len(recs))
	for _, rec := range recs {
		a := latest[rec.TicketID.Raw]
		out = append(out, &batchCandidate{ticketID: rec.TicketID.Raw, attemptID: a.AttemptID, base: a.BaseCommit, branch: a.Branch, tree: a.CandidateTreeOid})
	}
	return out
}

// batchGit runs Git in the primary worktree with the store's isolated
// environment, every new object redirected into objects, the repository's
// object store as a read-only alternate, a deadline and bounded output. It
// returns stdout and the exit status; a timeout, signal or overflow refuses.
type batchGit struct {
	root, objects, alternate string
}

func (g batchGit) run(args ...string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), batchGitTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, "git", append([]string{"-c", "credential.helper=", "-c", "core.fsmonitor=false"}, args...)...)
	c.Dir = g.root
	c.Env = append(batchGitEnvironment(), "GIT_OBJECT_DIRECTORY="+g.objects, "GIT_ALTERNATE_OBJECT_DIRECTORIES="+g.alternate)
	out := &boundedBuffer{max: batchGitOutputBytes}
	c.Stdout = out
	err := c.Run()
	if out.over {
		return nil, 0, wire.Errorf(wire.CodeLimitExceeded, "git "+args[0], "git output exceeds %d bytes", batchGitOutputBytes)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() >= 1 && ctx.Err() == nil {
		return out.buf.Bytes(), exit.ExitCode(), nil
	}
	if err != nil {
		return nil, 0, wire.Errorf(wire.CodeUnsupported, "git "+args[0], "git observation failed: %v", err)
	}
	return out.buf.Bytes(), 0, nil
}

func batchGitEnvironment() []string {
	environment := []string{}
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment, "LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1", "GCM_INTERACTIVE=never", "GIT_ASKPASS=", "GIT_ALLOW_PROTOCOL=")
}

// boundedBuffer keeps at most max bytes and records an overflow.
type boundedBuffer struct {
	buf  bytes.Buffer
	max  int
	over bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len()+len(p) > b.max {
		b.over = true
		return 0, errors.New("output bound exceeded")
	}
	return b.buf.Write(p)
}

// resolve answers rev^{kind} as an OID, or "" when it does not resolve.
func (g batchGit) resolve(rev, kind string) (string, error) {
	out, code, err := g.run("rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{"+kind+"}")
	if err != nil || code != 0 {
		return "", err
	}
	oid, err := wire.ParseOID("rev-parse", strings.TrimSpace(string(out)))
	if err != nil {
		return "", nil
	}
	return oid, nil
}

// merge is the dry three-way merge of tree onto acc over base. It returns
// the merged tree and the conflicted paths; the merged tree lives only in
// the throw-away object directory.
func (g batchGit) merge(base, acc, tree string) (string, []string, error) {
	out, code, err := g.run("merge-tree", "--write-tree", "-z", "--name-only", "--no-messages", "--merge-base="+base, acc, tree)
	if err != nil {
		return "", nil, err
	}
	if code > 1 {
		return "", nil, wire.Errorf(wire.CodeUnsupported, "git merge-tree", "git merge-tree exited %d", code)
	}
	fields := strings.Split(string(out), "\x00")
	merged, err := wire.ParseOID("merge-tree", fields[0])
	if err != nil {
		return "", nil, wire.Errorf(wire.CodeUnsupported, "git merge-tree", "unexpected merge-tree output")
	}
	if code == 0 {
		return merged, nil, nil
	}
	conflicts := scopePaths(nonEmpty(fields[1:]))
	if len(conflicts) == 0 {
		conflicts = []string{}
	}
	return merged, conflicts, nil
}

func nonEmpty(in []string) []string {
	out := []string{}
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// batchPlan resolves each candidate, groups by path disjointness and confirms
// each join with a dry chained merge (NTP-V0-021..023).
func batchPlan(rc *readCtx, queueID, branch string, cands []*batchCandidate, shared []string, maxSize int) (wire.Value, error) {
	tmp, err := os.MkdirTemp("", "corvint-plan-batches-")
	if err != nil {
		return wire.Value{}, wire.Errorf(wire.CodeUnsupported, "plan batches", "temporary object directory: %v", err)
	}
	defer os.RemoveAll(tmp)
	objects := filepath.Join(tmp, "objects")
	if err := os.Mkdir(objects, 0o700); err != nil {
		return wire.Value{}, wire.Errorf(wire.CodeUnsupported, "plan batches", "temporary object directory: %v", err)
	}
	g := batchGit{root: rc.repo.PrimaryWorktree, objects: objects, alternate: filepath.Join(rc.repo.CommonDir, "objects")}
	ref := "refs/heads/" + branch
	target, err := g.resolve(ref, "commit")
	if err != nil {
		return wire.Value{}, err
	}
	if target == "" {
		return wire.Value{}, wire.Errorf(wire.CodeMissingEvidence, ref, "the intent branch %s does not resolve to a commit", ref)
	}
	targetTree, err := g.resolve(target, "tree")
	if err != nil || targetTree == "" {
		return wire.Value{}, wire.Errorf(wire.CodeMissingEvidence, ref, "the intent branch tree does not resolve")
	}
	var groups []*batchGroup
	var splits []batchSplit
	var ungrouped []batchUngrouped
	for _, c := range cands {
		detail, err := resolveCandidate(g, c)
		if err != nil {
			return wire.Value{}, err
		}
		if detail != "" {
			ungrouped = append(ungrouped, batchUngrouped{c.ticketID, c.attemptID, wire.CodeMissingEvidence, detail})
			continue
		}
		joined := false
		for gi, grp := range groups {
			if len(grp.members) >= maxSize || !batchDisjoint(grp, c, shared) {
				continue
			}
			merged, conflicts, err := g.merge(c.base, grp.tree, c.candTree)
			if err != nil {
				return wire.Value{}, err
			}
			if conflicts != nil {
				splits = append(splits, batchSplit{c.ticketID, gi + 1, conflicts})
				continue
			}
			grp.members, grp.tree, joined = append(grp.members, c), merged, true
			break
		}
		if joined {
			continue
		}
		merged, conflicts, err := g.merge(c.base, targetTree, c.candTree)
		if err != nil {
			return wire.Value{}, err
		}
		if conflicts != nil {
			ungrouped = append(ungrouped, batchUngrouped{c.ticketID, c.attemptID, wire.CodeStaleTree, "the candidate does not merge cleanly onto " + ref + " at " + target + ": " + strings.Join(clip(conflicts), ", ")})
			continue
		}
		groups = append(groups, &batchGroup{members: []*batchCandidate{c}, tree: merged})
	}
	return batchValue(queueID, ref, target, maxSize, shared, groups, splits, ungrouped), nil
}

// resolveCandidate fills the candidate's base tree, candidate tree and changed
// paths. A nonempty detail reports it unresolvable (NTP-V0-024).
func resolveCandidate(g batchGit, c *batchCandidate) (string, error) {
	if _, err := wire.ParseOID("baseCommit", c.base); err != nil {
		return "the attempt records no base commit", nil
	}
	baseTree, err := g.resolve(c.base, "tree")
	if err != nil {
		return "", err
	}
	if baseTree == "" {
		return "base commit " + c.base + " does not resolve in this repository", nil
	}
	candTree := ""
	if c.tree != nil {
		if candTree, err = g.resolve(*c.tree, "tree"); err != nil {
			return "", err
		}
		if candTree == "" {
			return "candidate tree " + *c.tree + " does not resolve in this repository", nil
		}
	} else {
		if candTree, err = g.resolve("refs/heads/"+c.branch, "tree"); err != nil {
			return "", err
		}
		if candTree == "" {
			return "candidate branch refs/heads/" + c.branch + " does not resolve in this repository", nil
		}
	}
	out, code, err := g.run("diff-tree", "-r", "--name-only", "--no-renames", "-z", "--end-of-options", baseTree, candTree)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "changed paths between the base and the candidate are unavailable", nil
	}
	c.baseTree, c.candTree, c.paths = baseTree, candTree, nonEmpty(strings.Split(string(out), "\x00"))
	return "", nil
}

// batchDisjoint holds when no non-shared changed path of c overlaps one of
// the group's: equal paths, or one a directory prefix of the other.
func batchDisjoint(grp *batchGroup, c *batchCandidate, shared []string) bool {
	for _, m := range grp.members {
		for _, p := range c.paths {
			if batchShared(p, shared) {
				continue
			}
			for _, q := range m.paths {
				if p == q || strings.HasPrefix(q, p+"/") || strings.HasPrefix(p, q+"/") {
					return false
				}
			}
		}
	}
	return true
}

func batchShared(p string, shared []string) bool {
	for _, s := range shared {
		if p == s || (strings.HasSuffix(s, "/") && strings.HasPrefix(p, s)) {
			return true
		}
	}
	return false
}

func clip(paths []string) []string {
	if len(paths) > batchMaxConflictPaths {
		return paths[:batchMaxConflictPaths]
	}
	return paths
}

func batchValue(queueID, ref, target string, maxSize int, shared []string, groups []*batchGroup, splits []batchSplit, ungrouped []batchUngrouped) wire.Value {
	gv := []wire.Value{}
	for _, grp := range groups {
		tickets, attempts, paths := []string{}, []string{}, 0
		for _, m := range grp.members {
			tickets, attempts, paths = append(tickets, m.ticketID), append(attempts, m.attemptID), paths+len(m.paths)
		}
		o := wire.NewObject()
		o.Set("ticketIds", wire.Strings(tickets))
		o.Set("attemptIds", wire.Strings(attempts))
		o.Set("changedPaths", countValue(int64(paths)))
		gv = append(gv, wire.ObjectValue(o))
	}
	sv := []wire.Value{}
	for _, s := range splits {
		o := wire.NewObject()
		o.Set("ticketId", wire.String(s.ticketID))
		o.Set("group", countValue(int64(s.group)))
		o.Set("conflictedPaths", wire.Strings(clip(s.paths)))
		o.Set("conflictedTotal", countValue(int64(len(s.paths))))
		sv = append(sv, wire.ObjectValue(o))
	}
	uv := []wire.Value{}
	for _, u := range ungrouped {
		o := wire.NewObject()
		o.Set("ticketId", wire.String(u.ticketID))
		o.Set("attemptId", wire.String(u.attemptID))
		o.Set("reason", wire.String(u.reason))
		o.Set("detail", wire.String(prose(u.detail)))
		uv = append(uv, wire.ObjectValue(o))
	}
	target2 := wire.NewObject()
	target2.Set("ref", wire.String(ref))
	target2.Set("commit", wire.String(target))
	o := wire.NewObject()
	o.Set("profile", wire.String("taskman-plan-batches/0"))
	o.Set("queueId", wire.String(queueID))
	o.Set("target", wire.ObjectValue(target2))
	o.Set("maxGroupSize", countValue(int64(maxSize)))
	o.Set("sharedPaths", wire.Strings(shared))
	o.Set("groups", wire.Array(gv...))
	o.Set("splits", wire.Array(sv...))
	o.Set("ungrouped", wire.Array(uv...))
	o.Set("dryMerge", wire.String("git merge-tree --write-tree into a temporary object directory; no object or ref is added"))
	o.Set("mutationAuthority", wire.Bool(false))
	return wire.ObjectValue(o)
}
