package sharedpath

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// cloneOf clones f's repository with every branch as a remote-tracking ref,
// shallow to depth when depth is positive (V1-1726).
func cloneOf(t *testing.T, f *fixture, depth int) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "clone")
	args := []string{"clone", "--quiet", "--no-single-branch", "--no-checkout"}
	if depth > 0 {
		args = append(args, "--depth", strconv.Itoa(depth))
	}
	cmd := exec.Command("git", append(args, "file://"+f.root, dir)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %v %s", err, out)
	}
	shallow := strings.TrimSpace(gitIn(t, dir, "rev-parse", "--is-shallow-repository"))
	if want := strconv.FormatBool(depth > 0); shallow != want {
		t.Fatalf("clone at depth %d is shallow %s, want %s", depth, shallow, want)
	}
	return dir
}

// shallowFixture is main c1..c5 and feature f1 f2 from fork, both editing
// cmd/x.go, so a full clone's dry merge conflicts. Feature commits carry
// older committer times than main's, so the merge-base walk pops commits
// below the merge base before the feature tip.
func shallowFixture(t *testing.T, fork int) *fixture {
	t.Helper()
	f := newFixture(t)
	var mains []string
	for i := 1; i <= 5; i++ {
		mains = append(mains, f.commitWithCommitterDate("main "+strconv.Itoa(i), "2026-03-0"+strconv.Itoa(i)+"T10:00:00Z", map[string]string{
			"cmd/x.go": "main " + strconv.Itoa(i) + "\n", "internal/testutil/a.go": "a " + strconv.Itoa(i) + "\n",
		}))
	}
	f.git("checkout", "-q", "-b", "feature", mains[fork-1])
	f.commitWithCommitterDate("feat: one", "2026-01-01T10:00:00Z", map[string]string{"cmd/x.go": "feature\n"})
	f.commitWithCommitterDate("feat: two", "2026-01-02T10:00:00Z", map[string]string{"cmd/f.go": "f\n"})
	f.git("checkout", "-q", "main")
	return f
}

// SPC-V0-015: in a shallow clone whose merge base of base and head lies
// beyond the shallow boundary, check-shared and behind refuse with
// changes-shared-shallow-merge-base instead of a clean or wrong result; the
// same request on a full clone reports the conflict.
func TestSPCV0015ShallowMergeBaseBeyondBoundaryRefuses(t *testing.T) {
	ctx := context.Background()
	for name, c := range map[string]struct{ fork, depth int }{
		// The clone sees no common commit: main c4 c5, feature f1 f2.
		"no visible merge base": {fork: 1, depth: 2},
		// The clone sees c3 as the merge base, but as a boundary commit
		// whose parents it lacks.
		"boundary merge base": {fork: 3, depth: 3},
	} {
		t.Run(name, func(t *testing.T) {
			f := shallowFixture(t, c.fork)
			full := cloneOf(t, f, 0)
			r, err := Behind(ctx, BehindRequest{Root: full, Base: "origin/main", Head: "origin/feature", Config: testConfig()})
			if err != nil || r.DryMerge.Result != "conflict" {
				t.Fatalf("full clone dry merge = %+v, %v", r.DryMerge, err)
			}
			shallow := cloneOf(t, f, c.depth)
			if _, err := Behind(ctx, BehindRequest{Root: shallow, Base: "origin/main", Head: "origin/feature", Config: testConfig()}); err == nil || err.Error() != CodeShallowMergeBase {
				t.Fatalf("behind in a shallow clone = %v, want %s", err, CodeShallowMergeBase)
			}
			if _, err := Check(ctx, CheckRequest{Root: shallow, Base: "origin/main", Head: "origin/feature", Config: testConfig()}); err == nil || err.Error() != CodeShallowMergeBase {
				t.Fatalf("check-shared in a shallow clone = %v, want %s", err, CodeShallowMergeBase)
			}
		})
	}
}

// SPC-V0-015: a merge base that is base itself is exact, but a side branch of
// the range cut by the shallow boundary still refuses: feature forks from main
// c1, adds f1..f6 and merges main, and a depth-3 clone has main as the merge
// base while f4 is a boundary commit inside main..feature (independent review
// of V1-1726).
func TestSPCV0015ShallowRangeCutByBoundaryRefuses(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	c1 := f.commit("main 1", map[string]string{"cmd/x.go": "x\n"})
	f.commit("main 2", map[string]string{"cmd/y.go": "y\n"})
	f.commit("main 3", map[string]string{"cmd/z.go": "z\n"})
	f.git("checkout", "-q", "-b", "feature", c1)
	for i := 1; i <= 6; i++ {
		f.commit("shared-path: f"+strconv.Itoa(i), map[string]string{"internal/testutil/f" + strconv.Itoa(i) + ".go": "f\n"})
	}
	f.git("-c", "user.useConfigOnly=false", "merge", "-q", "--no-ff", "--no-edit", "main")
	full, shallow := cloneOf(t, f, 0), cloneOf(t, f, 3)
	if base := strings.TrimSpace(gitIn(t, shallow, "merge-base", "origin/main", "origin/feature")); base != strings.TrimSpace(gitIn(t, shallow, "rev-parse", "origin/main")) {
		t.Fatalf("shallow merge base %s is not main", base)
	}
	r, err := Check(ctx, CheckRequest{Root: full, Base: "origin/main", Head: "origin/feature", Config: testConfig()})
	if err != nil || r.Commits != 7 {
		t.Fatalf("full clone check-shared = %d commits, %v", r.Commits, err)
	}
	if _, err := Check(ctx, CheckRequest{Root: shallow, Base: "origin/main", Head: "origin/feature", Config: testConfig()}); err == nil || err.Error() != CodeShallowMergeBase {
		t.Fatalf("check-shared over a cut range = %v, want %s", err, CodeShallowMergeBase)
	}
	if _, err := Behind(ctx, BehindRequest{Root: shallow, Base: "origin/feature", Head: "origin/main", Config: testConfig()}); err == nil || err.Error() != CodeShallowMergeBase {
		t.Fatalf("behind over a cut range = %v, want %s", err, CodeShallowMergeBase)
	}
}

// SPC-V0-015: a shallow criss-cross merge refuses. R deletes cmd/f.go on
// side A (A0 A1) and leaves it on side B (B0 B1); main and feature both merge
// A1 and B1, main restoring cmd/f.go unchanged and feature changing it. Git's
// virtual base merges A1 and B1 through their own merge base R: the full clone
// lacks cmd/f.go there and reports an add/add conflict, while a depth-3 clone,
// cut at A0 and B0, keeps it and merges cleanly. Both visible merge bases keep
// their parents and main...feature is only the two merges, so only the
// several-merge-bases rule refuses (independent review of V1-1726).
func TestSPCV0015ShallowCrissCrossMergeRefuses(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	root := f.commit("root", map[string]string{"cmd/f.go": "r\n"})
	a1 := func() string {
		f.commit("a0", map[string]string{"cmd/f.go": ""})
		return f.commit("a1", map[string]string{"cmd/a.go": "a\n"})
	}()
	f.git("checkout", "-q", "-b", "side", root)
	f.commit("b0", map[string]string{"cmd/b0.go": "b\n"})
	b1 := f.commit("b1", map[string]string{"cmd/b1.go": "b\n"})
	merge := func(branch, content string) {
		f.git("checkout", "-q", "-B", branch, a1)
		f.git("-c", "user.useConfigOnly=false", "merge", "-q", "--no-ff", "--no-commit", b1)
		f.commit("merge "+branch, map[string]string{"cmd/f.go": content})
	}
	merge("main", "r\n")
	merge("feature", "y\n")
	f.git("branch", "-q", "-D", "side")
	full, shallow := cloneOf(t, f, 0), cloneOf(t, f, 3)
	if bases := strings.Fields(gitIn(t, shallow, "merge-base", "--all", "origin/main", "origin/feature")); len(bases) != 2 {
		t.Fatalf("shallow merge bases = %v, want a1 and b1", bases)
	}
	if got := strings.Fields(gitIn(t, shallow, "rev-list", "origin/main...origin/feature")); len(got) != 2 {
		t.Fatalf("shallow main...feature = %v, want the two merges", got)
	}
	r, err := Check(ctx, CheckRequest{Root: full, Base: "origin/main", Head: "origin/feature", Config: testConfig()})
	if err != nil || len(r.Findings) != 1 || r.Findings[0].Code != "shared-path-merge-conflict" || r.Findings[0].Path != "cmd/f.go" {
		t.Fatalf("full clone check-shared = %+v, %v", r.Findings, err)
	}
	if _, err := Check(ctx, CheckRequest{Root: shallow, Base: "origin/main", Head: "origin/feature", Config: testConfig()}); err == nil || err.Error() != CodeShallowMergeBase {
		t.Fatalf("check-shared over a shallow criss-cross = %v, want %s", err, CodeShallowMergeBase)
	}
	if _, err := Behind(ctx, BehindRequest{Root: shallow, Base: "origin/main", Head: "origin/feature", Config: testConfig()}); err == nil || err.Error() != CodeShallowMergeBase {
		t.Fatalf("behind over a shallow criss-cross = %v, want %s", err, CodeShallowMergeBase)
	}
}

// SPC-V0-015: a shallow clone whose merge base keeps its parents gives the
// same behind and check-shared reports as a full clone, with the dry merge
// reading the clone's shallow file; a merge base that is base itself is exact
// even at the boundary.
func TestSPCV0015ShallowMergeBaseInsideBoundaryMatchesFullClone(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	f := shallowFixture(t, 3)
	full, shallow := cloneOf(t, f, 0), cloneOf(t, f, 4)
	if boundary := gitIn(t, shallow, "rev-parse", "origin/main~3"); !strings.Contains(readFile(t, filepath.Join(shallow, ".git", "shallow")), strings.TrimSpace(boundary)) {
		t.Fatalf("main c2 is not a shallow boundary commit")
	}
	behind := func(root, base, head string) BehindReport {
		t.Helper()
		r, err := Behind(ctx, BehindRequest{Root: root, Base: base, Head: head, Now: now, Config: testConfig()})
		if err != nil {
			t.Fatalf("behind in %s: %v", root, err)
		}
		r.BaseRef = BaseObservation{}
		return r
	}
	check := func(root, base, head string) Report {
		t.Helper()
		r, err := Check(ctx, CheckRequest{Root: root, Base: base, Head: head, Config: testConfig()})
		if err != nil {
			t.Fatalf("check-shared in %s: %v", root, err)
		}
		return r
	}
	want, got := behind(full, "origin/main", "origin/feature"), behind(shallow, "origin/main", "origin/feature")
	if want.DryMerge.Result != "conflict" || !reflect.DeepEqual(got, want) {
		t.Fatalf("shallow behind = %+v, full = %+v", got, want)
	}
	if want, got := check(full, "origin/main", "origin/feature"), check(shallow, "origin/main", "origin/feature"); !reflect.DeepEqual(got, want) {
		t.Fatalf("shallow check-shared = %+v, full = %+v", got, want)
	}
	// main c2 is a boundary commit and base here, so the merge base is exact.
	if want, got := check(full, "origin/main~3", "origin/main"), check(shallow, "origin/main~3", "origin/main"); !reflect.DeepEqual(got, want) {
		t.Fatalf("shallow check-shared from a boundary base = %+v, full = %+v", got, want)
	}
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
