package store_test

import (
	"fmt"
	"os/exec"
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
	if obs.GitCalls > 4 {
		t.Fatalf("%d Git processes for 200 heads, want at most 4", obs.GitCalls)
	}
	for i, r := range obs.Results {
		integrated := i < 20
		if tri(r.Integrated) != fmt.Sprint(integrated) || (!integrated && tri(r.HeadOnRef) != "true") {
			t.Fatalf("query %d: integrated %s on ref %s", i, tri(r.Integrated), tri(r.HeadOnRef))
		}
	}
}
