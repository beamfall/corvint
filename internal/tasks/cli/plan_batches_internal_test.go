package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// pbRepo is a plain Git repository whose main commit holds a/x, b/y, c/z
// and a ten-line CHANGELOG; it returns the root and the base commit.
func pbRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	pbGit(t, root, nil, "init", "-q", "-b", "main")
	for path, body := range map[string]string{"a/x": "a\n", "b/y": "b\n", "c/z": "c\n", "CHANGELOG": "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pbGit(t, root, nil, "add", "-A")
	pbGit(t, root, nil, "commit", "-q", "-m", "base")
	return root, pbGit(t, root, nil, "rev-parse", "HEAD")
}

func pbGit(t *testing.T, root string, stdin []byte, args ...string) string {
	t.Helper()
	return pbGitEnv(t, root, nil, stdin, args...)
}

func pbGitEnv(t *testing.T, root string, env []string, stdin []byte, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	c.Env = append(os.Environ(), append([]string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid"}, env...)...)
	if stdin != nil {
		c.Stdin = strings.NewReader(string(stdin))
	}
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// pbTree writes the base commit's tree with files replaced and returns it.
func pbTree(t *testing.T, root, base string, files map[string]string) string {
	t.Helper()
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	pbGitEnv(t, root, env, nil, "read-tree", base)
	for path, body := range files {
		blob := pbGit(t, root, []byte(body), "hash-object", "-w", "--stdin")
		pbGitEnv(t, root, env, nil, "update-index", "--add", "--replace", "--cacheinfo", "100644,"+blob+","+path)
	}
	return pbGitEnv(t, root, env, nil, "write-tree")
}

// pbContent snapshots the Git directory, ignoring only object-file mtimes.
func pbContent(t *testing.T, root string) []fixture.Entry {
	t.Helper()
	entries := fixture.TreeSnapshot(t, filepath.Join(root, ".git"))
	for i := range entries {
		if strings.HasPrefix(strings.TrimPrefix(entries[i].Path, "/"), "objects/") && entries[i].Mode.IsRegular() {
			entries[i].MTime = 0
		}
	}
	return entries
}

func pbCand(id, base, tree string) *batchCandidate {
	return &batchCandidate{ticketID: id, attemptID: "att-" + id, base: base, tree: &tree}
}

func pbPlan(t *testing.T, root string, cands []*batchCandidate, shared []string, max int) wire.Value {
	t.Helper()
	rc := &readCtx{repo: &intent.Repository{PrimaryWorktree: root, CommonDir: filepath.Join(root, ".git")}}
	v, err := batchPlan(rc, "Q", "main", cands, shared, max)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func pbGroups(v wire.Value) [][]string {
	gs, _ := v.Obj.Get("groups")
	out := [][]string{}
	for _, g := range gs.Arr {
		ids, _ := g.Obj.Get("ticketIds")
		row := []string{}
		for _, id := range ids.Arr {
			row = append(row, id.Str)
		}
		out = append(out, row)
	}
	return out
}

func pbList(v wire.Value, key string, fields ...string) [][]string {
	xs, _ := v.Obj.Get(key)
	out := [][]string{}
	for _, x := range xs.Arr {
		row := []string{}
		for _, f := range fields {
			s, _ := x.Obj.Get(f)
			if s.Kind == wire.KindArray {
				for _, e := range s.Arr {
					row = append(row, e.Str)
				}
				continue
			}
			row = append(row, s.Str)
		}
		out = append(out, row)
	}
	return out
}

// NTP-V0-020: candidates are the latest-generation BUILT, CHECKING,
// REVIEWING or READY_FOR_INTEGRATION attempt with a tree or branch of an
// OPEN or HELD ticket, in priority, order, ticket ID order.
func TestNTPV0020_CandidatesAreLatestSubmittedOrReadyAttempts(t *testing.T) {
	recs := []*ticket.Record{fixture.Ticket("C1"), fixture.Ticket("C2"), fixture.Ticket("C3"), fixture.Ticket("C4"), fixture.Ticket("C5"), fixture.Ticket("C6")}
	recs[1].Priority = "P0"
	recs[2].Status = ticket.StatusHeld
	recs[3].Status = ticket.StatusCompleted
	queue, err := intent.DecodeQueue(fixture.QueueBytes())
	if err != nil {
		t.Fatal(err)
	}
	inv, err := ticket.NewInventory(queue.QueueID, recs)
	if err != nil {
		t.Fatal(err)
	}
	tree := strings.Repeat("a", 40)
	att := func(rec *ticket.Record, id, phase, gen string, withTree bool, branch string) *snapshot.Attempt {
		a := &snapshot.Attempt{AttemptID: id, TicketID: rec.TicketID, Phase: phase, Generation: wire.Size(gen), BaseCommit: tree, Branch: branch}
		if withTree {
			a.CandidateTreeOid = &tree
		}
		return a
	}
	attempts := map[string]*snapshot.Attempt{}
	for _, a := range []*snapshot.Attempt{
		att(recs[0], "a1-old", "BUILT", "1", true, ""),
		att(recs[0], "a1-new", "REVIEWING", "5", true, ""),
		att(recs[1], "a2", "READY_FOR_INTEGRATION", "2", false, "topic"),
		att(recs[2], "a3", "CHECKING", "3", true, ""),
		att(recs[3], "a4", "BUILT", "4", true, ""),
		att(recs[4], "a5", "RUNNING", "6", true, ""),
		att(recs[5], "a6", "BUILT", "7", false, ""),
	} {
		attempts[a.AttemptID] = a
	}
	got := []string{}
	for _, c := range batchCandidates(inv, attempts) {
		got = append(got, c.ticketID+"="+c.attemptID)
	}
	want := []string{recs[1].TicketID.Raw + "=a2", recs[0].TicketID.Raw + "=a1-new", recs[2].TicketID.Raw + "=a3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidates %v, want %v", got, want)
	}
}

// NTP-V0-021: candidates group by pairwise-disjoint changed paths; an equal
// path or a directory prefix separates them, and a declared shared path is
// exempt from the disjointness test.
func TestNTPV0021_GroupsByDisjointChangedPaths(t *testing.T) {
	grp := &batchGroup{members: []*batchCandidate{{paths: []string{"d/e", "CHANGELOG"}}}}
	for _, c := range []struct {
		paths  []string
		shared []string
		want   bool
	}{
		{[]string{"d"}, nil, false},
		{[]string{"d/e/f"}, nil, false},
		{[]string{"d/ef"}, nil, true},
		{[]string{"CHANGELOG"}, nil, false},
		{[]string{"CHANGELOG"}, []string{"CHANGELOG"}, true},
		{[]string{"d/e"}, []string{"d/"}, true},
		{[]string{"d/e"}, []string{"d"}, false},
	} {
		if got := batchDisjoint(grp, &batchCandidate{paths: c.paths}, c.shared); got != c.want {
			t.Fatalf("disjoint %v shared %v = %v, want %v", c.paths, c.shared, got, c.want)
		}
	}
	root, base := pbRepo(t)
	a := pbTree(t, root, base, map[string]string{"a/x": "A\n"})
	b := pbTree(t, root, base, map[string]string{"b/y": "B\n"})
	a2 := pbTree(t, root, base, map[string]string{"a/x": "A\n", "c/z": "C\n"})
	dir := pbTree(t, root, base, map[string]string{"b/y/deeper": "D\n"})
	v := pbPlan(t, root, []*batchCandidate{pbCand("T1", base, a), pbCand("T2", base, b), pbCand("T3", base, a2), pbCand("T4", base, dir)}, []string{}, 8)
	if got, want := pbGroups(v), [][]string{{"T1", "T2"}, {"T3", "T4"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups %v, want %v", got, want)
	}
	// With a/ shared, T3's a/x no longer separates it from T1; the dry merge
	// of the identical edit is clean, so T3 joins the first group.
	v = pbPlan(t, root, []*batchCandidate{pbCand("T1", base, a), pbCand("T2", base, b), pbCand("T3", base, a2)}, []string{"a/"}, 8)
	if got, want := pbGroups(v), [][]string{{"T1", "T2", "T3"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("shared groups %v, want %v", got, want)
	}
	if total, _ := v.Obj.Get("mutationAuthority"); total.Kind != wire.KindBool || total.Bool {
		t.Fatalf("mutationAuthority %+v", total)
	}
}

// NTP-V0-022: a dry chained merge confirms each join; a conflict on a
// shared path splits the candidate into a later group and is reported; a
// candidate that conflicts with the target tip itself stays ungrouped.
func TestNTPV0022_DryChainedMergeSplitsConflicts(t *testing.T) {
	root, base := pbRepo(t)
	top := pbTree(t, root, base, map[string]string{"a/x": "A\n", "CHANGELOG": "one\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"})
	bottom := pbTree(t, root, base, map[string]string{"b/y": "B\n", "CHANGELOG": "1\n2\n3\n4\n5\n6\n7\n8\n9\nten\n"})
	middle := pbTree(t, root, base, map[string]string{"c/z": "C\n", "CHANGELOG": "1\n2\n3\n4\nfive\n6\n7\n8\n9\n10\n"})
	clash := pbTree(t, root, base, map[string]string{"CHANGELOG": "1\n2\n3\n4\nFIVE\n6\n7\n8\n9\n10\n"})
	v := pbPlan(t, root, []*batchCandidate{pbCand("T1", base, top), pbCand("T2", base, bottom), pbCand("T3", base, middle), pbCand("T4", base, clash)}, []string{"CHANGELOG"}, 8)
	if got, want := pbGroups(v), [][]string{{"T1", "T2", "T3"}, {"T4"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups %v, want %v", got, want)
	}
	if got, want := pbList(v, "splits", "ticketId", "group", "conflictedPaths"), [][]string{{"T4", "1", "CHANGELOG"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("splits %v, want %v", got, want)
	}
	// Advance main past base with a conflicting CHANGELOG line: a candidate
	// built on the old base conflicts with the target and stays ungrouped.
	pbGit(t, root, nil, "update-ref", "refs/heads/main", pbGit(t, root, nil, "commit-tree", clash, "-p", base, "-m", "moved"))
	v = pbPlan(t, root, []*batchCandidate{pbCand("T3", base, middle), pbCand("T1", base, top)}, []string{"CHANGELOG"}, 8)
	if got, want := pbGroups(v), [][]string{{"T1"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("moved groups %v, want %v", got, want)
	}
	if got := pbList(v, "ungrouped", "ticketId", "reason"); !reflect.DeepEqual(got, [][]string{{"T3", wire.CodeStaleTree}}) {
		t.Fatalf("ungrouped %v", got)
	}
}

// NTP-V0-022: the dry merges add no object and no ref to the repository,
// leave its Git directory byte-identical and remove the throw-away object
// directory.
func TestNTPV0022_DryMergeAddsNoObjectsOrRefs(t *testing.T) {
	root, base := pbRepo(t)
	one := pbTree(t, root, base, map[string]string{"a/x": "A\n", "CHANGELOG": "one\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"})
	two := pbTree(t, root, base, map[string]string{"b/y": "B\n", "CHANGELOG": "1\n2\n3\n4\n5\n6\n7\n8\n9\nten\n"})
	three := pbTree(t, root, base, map[string]string{"CHANGELOG": "one-more\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"})
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	objects, refs := pbGit(t, root, nil, "count-objects", "-v"), pbGit(t, root, nil, "for-each-ref")
	gitDir := pbContent(t, root)
	v := pbPlan(t, root, []*batchCandidate{pbCand("T1", base, one), pbCand("T2", base, two), pbCand("T3", base, three)}, []string{"CHANGELOG"}, 8)
	if got := pbGroups(v); len(got) != 2 || len(got[0]) != 2 {
		t.Fatalf("groups %v: want a merged pair and a split", got)
	}
	if got := pbGit(t, root, nil, "count-objects", "-v"); got != objects {
		t.Fatalf("object count changed:\n%s\nwas\n%s", got, objects)
	}
	if got := pbGit(t, root, nil, "for-each-ref"); got != refs {
		t.Fatalf("refs changed:\n%s\nwas\n%s", got, refs)
	}
	// Git freshens the mtime of an existing object that a merge recreates
	// (its gc grace clock); every other byte and timestamp is unchanged.
	if after := pbContent(t, root); !reflect.DeepEqual(after, gitDir) {
		t.Fatalf("the Git directory changed:\n%v\nwas\n%v", after, gitDir)
	}
	if left, err := os.ReadDir(tmp); err != nil || len(left) != 0 {
		t.Fatalf("throw-away directory left behind: %v %v", left, err)
	}
}

// NTP-V0-023: the maximum group size bounds every group.
func TestNTPV0023_MaxGroupSizeBoundsGroups(t *testing.T) {
	root, base := pbRepo(t)
	cands := []*batchCandidate{
		pbCand("T1", base, pbTree(t, root, base, map[string]string{"a/x": "A\n"})),
		pbCand("T2", base, pbTree(t, root, base, map[string]string{"b/y": "B\n"})),
		pbCand("T3", base, pbTree(t, root, base, map[string]string{"c/z": "C\n"})),
	}
	for max, want := range map[int][][]string{1: {{"T1"}, {"T2"}, {"T3"}}, 2: {{"T1", "T2"}, {"T3"}}, 3: {{"T1", "T2", "T3"}}} {
		v := pbPlan(t, root, cands, []string{}, max)
		if got := pbGroups(v); !reflect.DeepEqual(got, want) {
			t.Fatalf("max %d: groups %v, want %v", max, got, want)
		}
		if size, _ := v.Obj.Get("maxGroupSize"); size.Str != string(wire.CountOf(int64(max))) {
			t.Fatalf("maxGroupSize %q", size.Str)
		}
	}
}

// NTP-V0-024: a candidate whose tree, branch or base does not resolve is
// reported in ungrouped with MISSING_EVIDENCE and never grouped; a branch
// candidate without a recorded tree resolves through refs/heads.
func TestNTPV0024_UnresolvableCandidateIsReportedUngrouped(t *testing.T) {
	root, base := pbRepo(t)
	good := pbTree(t, root, base, map[string]string{"a/x": "A\n"})
	topic := pbGit(t, root, nil, "commit-tree", pbTree(t, root, base, map[string]string{"b/y": "B\n"}), "-p", base, "-m", "topic")
	pbGit(t, root, nil, "update-ref", "refs/heads/topic", topic)
	missing := strings.Repeat("e", 40)
	viaBranch := &batchCandidate{ticketID: "T5", attemptID: "att-T5", base: base, branch: "topic"}
	noBranch := &batchCandidate{ticketID: "T6", attemptID: "att-T6", base: base, branch: "gone"}
	v := pbPlan(t, root, []*batchCandidate{pbCand("T1", base, good), pbCand("T2", base, missing), pbCand("T3", missing, good), pbCand("T4", "", good), viaBranch, noBranch}, []string{}, 8)
	if got, want := pbGroups(v), [][]string{{"T1", "T5"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups %v, want %v", got, want)
	}
	got := pbList(v, "ungrouped", "ticketId", "attemptId", "reason")
	want := [][]string{{"T2", "att-T2", wire.CodeMissingEvidence}, {"T3", "att-T3", wire.CodeMissingEvidence}, {"T4", "att-T4", wire.CodeMissingEvidence}, {"T6", "att-T6", wire.CodeMissingEvidence}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ungrouped %v, want %v", got, want)
	}
	// An unresolvable intent branch refuses the whole read.
	rc := &readCtx{repo: &intent.Repository{PrimaryWorktree: root, CommonDir: filepath.Join(root, ".git")}}
	if _, err := batchPlan(rc, "Q", "absent", nil, []string{}, 8); wire.CodeOf(err) != wire.CodeMissingEvidence {
		t.Fatalf("absent intent branch: %v", err)
	}
}

// NTP-V0-021: a shared path is exempt only when both overlapping paths are
// shared; a non-shared path of either candidate collides, in both orders.
func TestNTPV0021_SharedPathCollisionIsSymmetric(t *testing.T) {
	shared := []string{"d/"}
	for _, c := range []struct {
		member, cand []string
		want         bool
	}{
		{[]string{"d"}, []string{"d/e"}, false},
		{[]string{"d/e"}, []string{"d"}, false},
		{[]string{"d/e"}, []string{"d/e"}, true},
		{[]string{"d/e"}, []string{"d/f"}, true},
	} {
		grp := &batchGroup{members: []*batchCandidate{{paths: c.member}}}
		if got := batchDisjoint(grp, &batchCandidate{paths: c.cand}, shared); got != c.want {
			t.Fatalf("member %v candidate %v: disjoint %v, want %v", c.member, c.cand, got, c.want)
		}
	}
}

// NTP-V0-022: a candidate that would join an existing group is still
// checked against the tip: B, built on the old base, merges cleanly onto
// A's identical edit but conflicts with the tip, so it is STALE_TREE.
func TestNTPV0022_JoiningCandidateIsCheckedAgainstTip(t *testing.T) {
	root, base := pbRepo(t)
	line := func(first string) string { return first + "\n2\n3\n4\n5\n6\n7\n8\n9\n10\n" }
	tip := pbGit(t, root, nil, "commit-tree", pbTree(t, root, base, map[string]string{"CHANGELOG": line("b")}), "-p", base, "-m", "tip")
	pbGit(t, root, nil, "update-ref", "refs/heads/main", tip)
	a := pbTree(t, root, tip, map[string]string{"CHANGELOG": line("c")})
	b := pbTree(t, root, base, map[string]string{"CHANGELOG": line("c")})
	v := pbPlan(t, root, []*batchCandidate{pbCand("A", tip, a), pbCand("B", base, b)}, []string{"CHANGELOG"}, 8)
	if got, want := pbGroups(v), [][]string{{"A"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups %v, want %v", got, want)
	}
	if got := pbList(v, "ungrouped", "ticketId", "reason"); !reflect.DeepEqual(got, [][]string{{"B", wire.CodeStaleTree}}) {
		t.Fatalf("ungrouped %v", got)
	}
}

// NTP-V0-022 / NTP-V0-025: repository configuration and attributes never
// reach the dry merge, so a configured merge driver does not run.
func TestNTPV0025_RepositoryMergeDriverDoesNotRun(t *testing.T) {
	root, base := pbRepo(t)
	marker := filepath.Join(t.TempDir(), "driver-ran")
	config, err := os.OpenFile(filepath.Join(root, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.WriteString("[merge \"evil\"]\n\tdriver = touch '" + marker + "'; exit 1\n[core]\n\thooksPath = " + t.TempDir() + "\n"); err != nil {
		t.Fatal(err)
	}
	config.Close()
	fixture.Write(t, filepath.Join(root, ".git", "info", "attributes"), []byte("* merge=evil\n"))
	fixture.Write(t, filepath.Join(root, ".gitattributes"), []byte("* merge=evil\n"))
	one := pbTree(t, root, base, map[string]string{"CHANGELOG": "one\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"})
	two := pbTree(t, root, base, map[string]string{"CHANGELOG": "1\n2\n3\n4\n5\n6\n7\n8\n9\nten\n"})
	v := pbPlan(t, root, []*batchCandidate{pbCand("T1", base, one), pbCand("T2", base, two)}, []string{"CHANGELOG"}, 8)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("the repository's merge driver ran during the read: %v", err)
	}
	if got, want := pbGroups(v), [][]string{{"T1", "T2"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups %v, want %v", got, want)
	}
}

// NTP-V0-024: a missing tree object below the candidate's root tree leaves
// the candidate ungrouped MISSING_EVIDENCE.
func TestNTPV0024_MissingNestedTreeIsUngrouped(t *testing.T) {
	root, base := pbRepo(t)
	nested := pbTree(t, root, base, map[string]string{"n/q": "Q\n"})
	sub := pbGit(t, root, nil, "rev-parse", nested+":n")
	if err := os.Remove(filepath.Join(root, ".git", "objects", sub[:2], sub[2:])); err != nil {
		t.Fatal(err)
	}
	v := pbPlan(t, root, []*batchCandidate{pbCand("T1", base, nested), pbCand("T2", base, pbTree(t, root, base, map[string]string{"a/x": "A\n"}))}, []string{}, 8)
	if got, want := pbGroups(v), [][]string{{"T2"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups %v, want %v", got, want)
	}
	if got := pbList(v, "ungrouped", "ticketId", "reason"); !reflect.DeepEqual(got, [][]string{{"T1", wire.CodeMissingEvidence}}) {
		t.Fatalf("ungrouped %v", got)
	}
}
