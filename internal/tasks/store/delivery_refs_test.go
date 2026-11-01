package store_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
)

// dlvRepo is a Git repository on main with one root commit.
func dlvRepo(t *testing.T) (root string, run func(args ...string) string) {
	t.Helper()
	root = t.TempDir()
	run = func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "root")
	return root, run
}

func tri(b *bool) string {
	if b == nil {
		return "nil"
	}
	return fmt.Sprint(*b)
}

// TestTLCV0003_ObserveDeliveries pins the integration ref, branch resolution
// and per-head observations of TLC-V0-003 and TLC-V0-004.
func TestTLCV0003_ObserveDeliveries(t *testing.T) {
	root, git := dlvRepo(t)
	commit := func(branch, msg string) string {
		git("checkout", "-q", branch)
		git("commit", "-q", "--allow-empty", "-m", msg)
		return git("rev-parse", "HEAD")
	}
	git("branch", "lane")
	git("branch", "merged")
	git("branch", "gone")
	git("branch", "rewritten")
	git("branch", "behind")
	onLane := commit("lane", "lane work")
	merged := commit("merged", "merged work")
	gone := commit("gone", "gone work")
	rewritten := commit("rewritten", "rewritten work")
	behind := commit("behind", "behind work")
	commit("behind", "later work")
	git("checkout", "-q", "main")
	git("merge", "-q", "--no-ff", "-m", "merge", "merged")
	git("branch", "-D", "gone")
	git("branch", "-f", "rewritten", "main")
	git("update-ref", "refs/remotes/upstream/remote-only", onLane)
	git("update-ref", "refs/remotes/origin/both", onLane)
	git("update-ref", "refs/remotes/upstream/both", merged)
	mainTip := git("rev-parse", "main")

	qs := []store.DeliveryQuery{
		{Branch: "lane", Head: onLane},
		{Branch: "merged", Head: merged},
		{Branch: "gone", Head: gone},
		{Branch: "rewritten", Head: rewritten},
		{Branch: "behind", Head: behind},
		{Branch: "lane", Head: strings.Repeat("ab", 20)},
		{Branch: "lane", Head: "HEAD"},
		{Branch: "remote-only", Head: onLane},
		{Branch: "both", Head: onLane},
		{Branch: "refs/heads/lane", Head: onLane},
		{Branch: "", Head: onLane},
	}
	obs := store.ObserveDeliveries(root, "main", qs)
	if obs.Failed || obs.IntegrationRef != "refs/heads/main" || obs.IntegrationCommit != mainTip {
		t.Fatalf("observation %+v", obs)
	}
	type row struct{ ref, exists, integrated, onRef string }
	want := []row{
		{"refs/heads/lane", "true", "false", "true"},
		{"refs/heads/merged", "true", "true", "nil"},
		{"", "true", "false", "nil"},
		{"refs/heads/rewritten", "true", "false", "false"},
		{"refs/heads/behind", "true", "false", "true"},
		{"refs/heads/lane", "false", "nil", "nil"},
		{"refs/heads/lane", "false", "nil", "nil"},
		{"refs/remotes/upstream/remote-only", "true", "false", "true"},
		{"refs/remotes/origin/both", "true", "false", "true"},
		{"refs/heads/lane", "true", "false", "true"},
		{"", "true", "false", "nil"},
	}
	for i, r := range obs.Results {
		got := row{r.Ref, tri(r.HeadExists), tri(r.Integrated), tri(r.HeadOnRef)}
		if got != want[i] {
			t.Errorf("query %d %+v: got %+v, want %+v", i, qs[i], got, want[i])
		}
	}

	// origin/<intentBranch> outranks the local branch.
	git("update-ref", "refs/remotes/origin/main", onLane)
	obs = store.ObserveDeliveries(root, "main", qs[:2])
	if obs.IntegrationRef != "refs/remotes/origin/main" || tri(obs.Results[0].Integrated) != "true" || tri(obs.Results[1].Integrated) != "false" {
		t.Fatalf("origin integration ref: %+v", obs)
	}
	// No integration ref: nothing is integrated or walked.
	obs = store.ObserveDeliveries(root, "trunk", qs[:1])
	if obs.Failed || obs.IntegrationRef != "" || obs.Results[0].Integrated != nil || tri(obs.Results[0].HeadExists) != "true" {
		t.Fatalf("missing integration ref: %+v", obs)
	}
	// No repository fails the whole observation.
	if obs := store.ObserveDeliveries(t.TempDir(), "main", qs[:1]); !obs.Failed || len(obs.Results) != 0 {
		t.Fatalf("no repository: %+v", obs)
	}
}

// TestTLCV0006_ObservationIsBatched pins a fixed Git process count however
// many heads are checked.
func TestTLCV0006_ObservationIsBatched(t *testing.T) {
	root, git := dlvRepo(t)
	var qs []store.DeliveryQuery
	for i := 0; i < 10; i++ {
		b := fmt.Sprintf("lane-%d", i)
		git("checkout", "-q", "-b", b, "main")
		for j := 0; j < 20; j++ {
			git("commit", "-q", "--allow-empty", "-m", fmt.Sprintf("%s %d", b, j))
			qs = append(qs, store.DeliveryQuery{Branch: b, Head: git("rev-parse", "HEAD")})
		}
	}
	git("checkout", "-q", "main")
	git("merge", "-q", "--no-ff", "-m", "merge", "lane-0")
	obs := store.ObserveDeliveries(root, "main", qs)
	if obs.Failed || len(obs.Results) != 200 {
		t.Fatalf("observation failed: %+v", obs.Failed)
	}
	if obs.GitCalls > 5 {
		t.Fatalf("%d Git processes for 200 heads, want at most 5", obs.GitCalls)
	}
	for i, r := range obs.Results {
		integrated := i < 20
		if tri(r.Integrated) != fmt.Sprint(integrated) || (!integrated && tri(r.HeadOnRef) != "true") {
			t.Fatalf("query %d: integrated %s on ref %s", i, tri(r.Integrated), tri(r.HeadOnRef))
		}
	}
}

// dlvCommit commits an empty change on branch and returns its object name.
func dlvCommit(git func(...string) string, branch, msg string) string {
	git("checkout", "-q", branch)
	git("commit", "-q", "--allow-empty", "-m", msg)
	return git("rev-parse", "HEAD")
}

// TestTLCV0004_ShallowHistoryIsNotObserved pins that a shallow boundary on
// the integration history never yields a negative reachability: a head the
// grafted walk does not reach is not observed, and one it reaches stays
// integrated.
func TestTLCV0004_ShallowHistoryIsNotObserved(t *testing.T) {
	root, git := dlvRepo(t)
	m1 := dlvCommit(git, "main", "m1")
	m2 := dlvCommit(git, "main", "m2")
	m3 := dlvCommit(git, "main", "m3")
	qs := []store.DeliveryQuery{{Branch: "main", Head: m1}, {Branch: "main", Head: m3}}
	if obs := store.ObserveDeliveries(root, "main", qs); tri(obs.Results[0].Integrated) != "true" {
		t.Fatalf("full history: %+v", obs.Results[0])
	}
	// m2 becomes a shallow boundary: m1 is still an ancestor of main, but
	// the walk can no longer see it.
	if err := os.WriteFile(filepath.Join(root, ".git", "shallow"), []byte(m2+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	obs := store.ObserveDeliveries(root, "main", qs)
	if obs.Failed || obs.Results[0].Integrated != nil || obs.Results[0].HeadOnRef != nil || tri(obs.Results[1].Integrated) != "true" {
		t.Fatalf("shallow history: %+v", obs.Results)
	}
}

// TestTLCV0003_ExactRefInAnyNamespace pins that an explicit refs/ branch
// outside refs/heads and refs/remotes resolves as given, and that a missing
// or pattern-like one stays unresolved without failing the observation.
func TestTLCV0003_ExactRefInAnyNamespace(t *testing.T) {
	root, git := dlvRepo(t)
	git("branch", "lane")
	h1 := dlvCommit(git, "lane", "l1")
	h2 := dlvCommit(git, "lane", "l2")
	git("checkout", "-q", "main")
	git("update-ref", "refs/lanes/delivery", h2)
	git("update-ref", "refs/lanes/delivery-sibling", h2)
	git("branch", "-D", "lane")
	qs := []store.DeliveryQuery{
		{Branch: "refs/lanes/delivery", Head: h1},
		{Branch: "refs/lanes/delivery", Head: h2},
		{Branch: "refs/lanes/missing", Head: h1},
		{Branch: "refs/lanes/*", Head: h1},
	}
	obs := store.ObserveDeliveries(root, "main", qs)
	if obs.Failed {
		t.Fatalf("observation failed: %+v", obs)
	}
	type row struct{ ref, commit, onRef string }
	want := []row{{"refs/lanes/delivery", h2, "true"}, {"refs/lanes/delivery", h2, "true"}, {"", "", "nil"}, {"", "", "nil"}}
	for i, r := range obs.Results {
		if got := (row{r.Ref, r.RefCommit, tri(r.HeadOnRef)}); got != want[i] {
			t.Errorf("query %d %+v: got %+v, want %+v", i, qs[i], got, want[i])
		}
	}
}

// TestTLCV0003_IntegratedHeadIsNotCheckedOnRef pins that an integrated head
// whose branch another query leaves pending keeps headOnRef null instead of
// a false negative from the walk that excludes the integration commit.
func TestTLCV0003_IntegratedHeadIsNotCheckedOnRef(t *testing.T) {
	root, git := dlvRepo(t)
	git("branch", "lane")
	merged := dlvCommit(git, "lane", "merged work")
	git("checkout", "-q", "main")
	git("merge", "-q", "--no-ff", "-m", "merge", "lane")
	later := dlvCommit(git, "lane", "later work")
	dlvCommit(git, "lane", "tip work")
	obs := store.ObserveDeliveries(root, "main", []store.DeliveryQuery{{Branch: "lane", Head: merged}, {Branch: "lane", Head: later}})
	if r := obs.Results[0]; obs.Failed || tri(r.Integrated) != "true" || r.HeadOnRef != nil {
		t.Fatalf("integrated head: %+v", r)
	}
	if r := obs.Results[1]; tri(r.Integrated) != "false" || tri(r.HeadOnRef) != "true" {
		t.Fatalf("pending head: %+v", r)
	}
}

// TestTLCV0006_WalkStartsFromCapturedCommits pins that the branch walk
// starts from the listed commits, not the ref names: three refs at one
// commit share one start, so none needs an ancestry fallback.
func TestTLCV0006_WalkStartsFromCapturedCommits(t *testing.T) {
	root, git := dlvRepo(t)
	git("branch", "x")
	head := dlvCommit(git, "x", "head")
	tip := dlvCommit(git, "x", "tip")
	git("checkout", "-q", "main")
	git("branch", "y", "x")
	git("branch", "z", "x")
	obs := store.ObserveDeliveries(root, "main", []store.DeliveryQuery{{Branch: "x", Head: head}, {Branch: "y", Head: head}, {Branch: "z", Head: head}})
	for i, r := range obs.Results {
		if r.RefCommit != tip || tri(r.HeadOnRef) != "true" {
			t.Fatalf("query %d: %+v", i, r)
		}
	}
	// ref listing, object check, integration walk, shallow check, branch walk
	if obs.GitCalls != 5 {
		t.Fatalf("%d Git processes, want 5 with no ancestry fallback", obs.GitCalls)
	}
}

// TestTLCV0006_ObservationIsDeterministic pins that identical reads give
// identical observations and process counts when walk attribution leaves
// more heads ambiguous than the fallback budget covers.
func TestTLCV0006_ObservationIsDeterministic(t *testing.T) {
	t.Setenv("GIT_AUTHOR_DATE", "2026-11-01T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-11-01T00:00:00Z")
	root, git := dlvRepo(t)
	git("branch", "shared")
	var shared []string
	for i := 0; i < 20; i++ {
		shared = append(shared, dlvCommit(git, "shared", fmt.Sprintf("shared %d", i)))
	}
	git("branch", "x", "shared")
	git("branch", "y", "shared")
	dlvCommit(git, "x", "x tip")
	dlvCommit(git, "y", "y tip")
	git("checkout", "-q", "main")
	var qs []store.DeliveryQuery
	for _, h := range shared {
		qs = append(qs, store.DeliveryQuery{Branch: "x", Head: h}, store.DeliveryQuery{Branch: "y", Head: h})
	}
	first := store.ObserveDeliveries(root, "main", qs)
	if first.Failed {
		t.Fatalf("observation failed")
	}
	for n := 0; n < 8; n++ {
		if again := store.ObserveDeliveries(root, "main", qs); !reflect.DeepEqual(again, first) {
			t.Fatalf("read %d differs from the first (%d and %d Git processes)", n, again.GitCalls, first.GitCalls)
		}
	}
}
